package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	coreGroup           = "cluster.x-k8s.io"
	infrastructureGroup = "infrastructure.cluster.x-k8s.io"
	controlPlaneGroup   = "controlplane.cluster.x-k8s.io"
	bootstrapGroup      = "bootstrap.cluster.x-k8s.io"

	maxManifests = 50
)

// ManifestDescriptor décrit une ressource à fournir pour créer un cluster.
type ManifestDescriptor struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required"`
	CRDName     string                 `json:"crdName"`
	Group       string                 `json:"group"`
	Kind        string                 `json:"kind"`
	Version     string                 `json:"version"`
	Object      map[string]interface{} `json:"object"`
}

// BlueprintResponse liste les manifests obligatoires (préremplis) et les manifests optionnels ajoutables.
type BlueprintResponse struct {
	Namespace string               `json:"namespace"`
	Required  []ManifestDescriptor `json:"required"`
	Optional  []ManifestDescriptor `json:"optional"`
}

type crdInfo struct {
	Name       string
	Group      string
	Kind       string
	Version    string
	Provider   string
	Namespaced bool
}

func (c crdInfo) apiVersion() string { return c.Group + "/" + c.Version }

func loadCRDInfos(ctx context.Context) ([]crdInfo, error) {
	list := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := k8sClient.List(ctx, list); err != nil {
		return nil, err
	}
	infos := make([]crdInfo, 0, len(list.Items))
	for i := range list.Items {
		crd := &list.Items[i]
		infos = append(infos, crdInfo{
			Name:       crd.Name,
			Group:      crd.Spec.Group,
			Kind:       crd.Spec.Names.Kind,
			Version:    getActiveCRDVersion(crd),
			Provider:   detectProviderFromCRD(crd.Name, crd.Spec.Group, crd.Spec.Names.Kind, crd.GetLabels(), crd.GetAnnotations()),
			Namespaced: crd.Spec.Scope == apiextensionsv1.NamespaceScoped,
		})
	}
	return infos, nil
}

// sameProvider compare deux identifiants de provider (le suffixe "-bootstrap" du catalogue est ignoré).
func sameProvider(a, b string) bool {
	return strings.TrimSuffix(a, "-bootstrap") == strings.TrimSuffix(b, "-bootstrap")
}

func findCRD(infos []crdInfo, group, providerID string, match func(kind string) bool) (crdInfo, bool) {
	var best crdInfo
	found := false
	for _, info := range infos {
		if info.Group != group || !info.Namespaced || !sameProvider(info.Provider, providerID) || !match(info.Kind) {
			continue
		}
		// Un provider peut exposer plusieurs variantes (ex. DockerCluster et DevCluster) : on préfère celle qui porte son nom.
		prefers := strings.HasPrefix(strings.ToLower(info.Kind), strings.ToLower(strings.TrimSuffix(providerID, "-bootstrap")))
		bestPrefers := strings.HasPrefix(strings.ToLower(best.Kind), strings.ToLower(strings.TrimSuffix(providerID, "-bootstrap")))
		if !found || (prefers && !bestPrefers) || (prefers == bestPrefers && info.Kind < best.Kind) {
			best, found = info, true
		}
	}
	return best, found
}

func findCoreCRD(infos []crdInfo, kind string) (crdInfo, bool) {
	for _, info := range infos {
		if info.Group == coreGroup && info.Kind == kind {
			return info, true
		}
	}
	return crdInfo{}, false
}

// objectRef construit une référence d'objet selon le contrat de l'API Cluster (apiGroup en v1beta2, apiVersion avant).
func objectRef(target crdInfo, name string, groupStyle bool) map[string]interface{} {
	if groupStyle {
		return map[string]interface{}{"apiGroup": target.Group, "kind": target.Kind, "name": name}
	}
	return map[string]interface{}{"apiVersion": target.apiVersion(), "kind": target.Kind, "name": name}
}

func newObject(target crdInfo, namespace, name string, spec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": target.apiVersion(),
		"kind":       target.Kind,
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
		"spec":       spec,
	}
}

func isKind(suffix string, excluded ...string) func(string) bool {
	return func(kind string) bool {
		if !strings.HasSuffix(kind, suffix) {
			return false
		}
		for _, bad := range excluded {
			if strings.HasSuffix(kind, bad) {
				return false
			}
		}
		return true
	}
}

type blueprintRequest struct {
	Name, Namespace, KubernetesVersion string
	Infrastructure, ControlPlane       string
	Bootstrap                          string
	CPReplicas, WorkerReplicas         int64
}

// buildBlueprint déduit, depuis les CRDs réellement installées, les manifests nécessaires à un cluster.
func buildBlueprint(infos []crdInfo, req blueprintRequest) (*BlueprintResponse, error) {
	cluster, ok := findCoreCRD(infos, "Cluster")
	if !ok {
		return nil, fmt.Errorf("Cluster API is not installed (Cluster CRD missing)")
	}
	machineDeployment, ok := findCoreCRD(infos, "MachineDeployment")
	if !ok {
		return nil, fmt.Errorf("MachineDeployment CRD missing")
	}

	infra, okInfra := findCRD(infos, infrastructureGroup, req.Infrastructure, isKind("Cluster", "ClusterTemplate"))
	template, okTemplate := findCRD(infos, infrastructureGroup, req.Infrastructure, isKind("MachineTemplate"))
	controlPlane, okCP := findCRD(infos, controlPlaneGroup, req.ControlPlane, isKind("ControlPlane"))
	bootstrap, okBootstrap := findCRD(infos, bootstrapGroup, req.Bootstrap, isKind("ConfigTemplate"))

	var missing []string
	if !okInfra {
		missing = append(missing, "infrastructure cluster for provider "+req.Infrastructure)
	}
	if !okTemplate {
		missing = append(missing, "MachineTemplate for provider "+req.Infrastructure)
	}
	if !okCP {
		missing = append(missing, "ControlPlane for provider "+req.ControlPlane)
	}
	if !okBootstrap {
		missing = append(missing, "ConfigTemplate for bootstrap provider "+req.Bootstrap)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("CRDs not found (are the providers installed?): %s", strings.Join(missing, ", "))
	}

	groupStyle := cluster.Version == "v1beta2"
	name, ns := req.Name, req.Namespace
	cpName := name + "-control-plane"
	mdName := name + "-md-0"
	labels := map[string]interface{}{"cluster.x-k8s.io/cluster-name": name}

	clusterObj := newObject(cluster, ns, name, map[string]interface{}{
		"clusterNetwork": map[string]interface{}{
			"pods":          map[string]interface{}{"cidrBlocks": []interface{}{"192.168.0.0/16"}},
			"services":      map[string]interface{}{"cidrBlocks": []interface{}{"10.128.0.0/12"}},
			"serviceDomain": "cluster.local",
		},
		"controlPlaneRef":   objectRef(controlPlane, cpName, groupStyle),
		"infrastructureRef": objectRef(infra, name, groupStyle),
	})
	clusterObj["metadata"].(map[string]interface{})["labels"] = labels

	var cpSpec map[string]interface{}
	if controlPlane.Kind == "TalosControlPlane" {
		cpSpec = map[string]interface{}{
			"version":  req.KubernetesVersion,
			"replicas": req.CPReplicas,
			"infrastructureTemplate": map[string]interface{}{
				"apiVersion": template.apiVersion(), "kind": template.Kind, "name": cpName,
			},
			"controlPlaneConfig": map[string]interface{}{
				"controlplane": map[string]interface{}{"generateType": "controlplane"},
			},
		}
	} else {
		machineTemplateRef := objectRef(template, cpName, groupStyle)
		machineTemplate := map[string]interface{}{"infrastructureRef": machineTemplateRef}
		if groupStyle {
			machineTemplate = map[string]interface{}{"spec": map[string]interface{}{"infrastructureRef": machineTemplateRef}}
		}
		cpSpec = map[string]interface{}{
			"version":         req.KubernetesVersion,
			"replicas":        req.CPReplicas,
			"machineTemplate": machineTemplate,
		}
		if controlPlane.Kind == "KubeadmControlPlane" {
			cpSpec["kubeadmConfigSpec"] = map[string]interface{}{}
		}
	}

	mdTemplateLabels := map[string]interface{}{"cluster.x-k8s.io/cluster-name": name, "cluster.x-k8s.io/deployment-name": mdName}
	mdSpec := map[string]interface{}{
		"clusterName": name,
		"replicas":    req.WorkerReplicas,
		"selector":    map[string]interface{}{"matchLabels": mdTemplateLabels},
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{"labels": mdTemplateLabels},
			"spec": map[string]interface{}{
				"clusterName":       name,
				"version":           req.KubernetesVersion,
				"bootstrap":         map[string]interface{}{"configRef": objectRef(bootstrap, mdName, groupStyle)},
				"infrastructureRef": objectRef(template, mdName, groupStyle),
			},
		},
	}

	bootstrapSpec := map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{}}}
	if bootstrap.Kind == "KubeadmConfigTemplate" {
		bootstrapSpec["template"] = map[string]interface{}{"spec": map[string]interface{}{"format": "cloud-config"}}
	}
	machineTemplateSpec := map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{}}}

	descriptor := func(id, title, description string, info crdInfo, object map[string]interface{}) ManifestDescriptor {
		return ManifestDescriptor{ID: id, Title: title, Description: description, Required: true, CRDName: info.Name,
			Group: info.Group, Kind: info.Kind, Version: info.Version, Object: object}
	}
	required := []ManifestDescriptor{
		descriptor("cluster", "Cluster", "Root resource: network and references to the control plane and infrastructure.", cluster, clusterObj),
		descriptor("infrastructure-cluster", infra.Kind, "Cluster infrastructure ("+req.Infrastructure+").", infra, newObject(infra, ns, name, map[string]interface{}{})),
		descriptor("control-plane", controlPlane.Kind, "Control plane ("+req.ControlPlane+").", controlPlane, newObject(controlPlane, ns, cpName, cpSpec)),
		descriptor("control-plane-machine-template", template.Kind+" (control plane)", "Control plane machine template.", template, newObject(template, ns, cpName, machineTemplateSpec)),
		descriptor("workers", "MachineDeployment (workers)", "Worker node pool.", machineDeployment, newObject(machineDeployment, ns, mdName, mdSpec)),
		descriptor("workers-machine-template", template.Kind+" (workers)", "Worker machine template.", template, newObject(template, ns, mdName, machineTemplateSpec)),
		descriptor("workers-bootstrap", bootstrap.Kind+" (workers)", "Worker bootstrap configuration ("+req.Bootstrap+").", bootstrap, newObject(bootstrap, ns, mdName, bootstrapSpec)),
	}

	used := map[string]bool{}
	for _, d := range required {
		used[d.Group+"/"+d.Kind] = true
	}
	var optional []ManifestDescriptor
	for _, info := range infos {
		if !info.Namespaced || !isCAPIGroup(info.Group) || strings.HasSuffix(info.Kind, "List") {
			continue
		}
		// Les ressources déjà proposées en obligatoire restent ajoutables en plusieurs exemplaires (ex. second pool de workers)
		spec := map[string]interface{}{}
		optional = append(optional, ManifestDescriptor{
			ID: strings.ToLower(info.Kind), Title: info.Kind, Description: info.Group,
			CRDName: info.Name, Group: info.Group, Kind: info.Kind, Version: info.Version,
			Object: newObject(info, ns, name+"-"+strings.ToLower(info.Kind), spec),
		})
	}
	sort.Slice(optional, func(i, j int) bool { return optional[i].Kind < optional[j].Kind })

	return &BlueprintResponse{Namespace: ns, Required: required, Optional: optional}, nil
}

func isCAPIGroup(group string) bool {
	return group == coreGroup || strings.HasSuffix(group, "."+coreGroup)
}

func queryInt(r *http.Request, key string, fallback int64) int64 {
	if value, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64); err == nil {
		return value
	}
	return fallback
}

// getClusterBlueprint gère GET /api/clusters/blueprint.
func getClusterBlueprint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := blueprintRequest{
		Name:              q.Get("name"),
		KubernetesVersion: strings.TrimSpace(q.Get("kubernetesVersion")),
		Infrastructure:    q.Get("infrastructureProvider"),
		ControlPlane:      q.Get("controlPlaneProvider"),
		Bootstrap:         q.Get("bootstrapProvider"),
		CPReplicas:        queryInt(r, "controlPlaneReplicas", 3),
		WorkerReplicas:    queryInt(r, "workerReplicas", 3),
	}
	if problems := validation.IsDNS1123Label(req.Name); len(problems) > 0 || len(req.Name) > 58 {
		http.Error(w, "The cluster name must be a valid DNS label and allow for the capi- namespace prefix", http.StatusBadRequest)
		return
	}
	if req.KubernetesVersion == "" || req.Infrastructure == "" || req.ControlPlane == "" || req.Bootstrap == "" {
		http.Error(w, "Kubernetes version and providers (infrastructure, control plane, bootstrap) are required", http.StatusBadRequest)
		return
	}
	if req.CPReplicas < 1 || req.WorkerReplicas < 0 || req.WorkerReplicas > 500 {
		http.Error(w, "The control plane must have at least one replica and workers must have between 0 and 500", http.StatusBadRequest)
		return
	}
	req.Namespace = "capi-" + req.Name

	infos, err := loadCRDInfos(r.Context())
	if err != nil {
		log.Printf("Erreur K8s (List CRDs): %v", err)
		http.Error(w, "Failed to read CRDs", http.StatusInternalServerError)
		return
	}
	blueprint, err := buildBlueprint(infos, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(blueprint)
}

// prepareManifests valide les manifests fournis et force leur namespace. Seules les ressources du groupe Cluster API sont acceptées.
func prepareManifests(payload CreateClusterPayload, namespace string) ([]*unstructured.Unstructured, error) {
	if len(payload.Manifests) == 0 || len(payload.Manifests) > maxManifests {
		return nil, fmt.Errorf("the number of manifests must be between 1 and %d", maxManifests)
	}
	objects := make([]*unstructured.Unstructured, 0, len(payload.Manifests))
	seen := map[string]bool{}
	clusters := 0
	for index, raw := range payload.Manifests {
		obj := &unstructured.Unstructured{Object: raw}
		gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
		if err != nil || gv.Version == "" || obj.GetKind() == "" {
			return nil, fmt.Errorf("manifest %d: apiVersion and kind are required", index+1)
		}
		if !isCAPIGroup(gv.Group) {
			return nil, fmt.Errorf("manifest %d (%s): only Cluster API resources (*.cluster.x-k8s.io) are allowed", index+1, obj.GetKind())
		}
		if problems := validation.IsDNS1123Subdomain(obj.GetName()); len(problems) > 0 {
			return nil, fmt.Errorf("manifest %d (%s): invalid name %q", index+1, obj.GetKind(), obj.GetName())
		}
		key := gv.Group + "/" + obj.GetKind() + "/" + obj.GetName()
		if seen[key] {
			return nil, fmt.Errorf("duplicate manifest: %s %s", obj.GetKind(), obj.GetName())
		}
		seen[key] = true
		if gv.Group == coreGroup && obj.GetKind() == "Cluster" {
			clusters++
			if obj.GetName() != payload.Name {
				return nil, fmt.Errorf("the Cluster must be named %q", payload.Name)
			}
		}
		unstructured.RemoveNestedField(obj.Object, "status")
		obj.SetNamespace(namespace)
		obj.SetUID("")
		obj.SetResourceVersion("")
		obj.SetOwnerReferences(nil)
		obj.SetFinalizers(nil)
		objects = append(objects, obj)
	}
	if clusters != 1 {
		return nil, fmt.Errorf("exactly one Cluster manifest is required")
	}
	// Le Cluster est créé en dernier : ses références existent déjà lorsque le contrôleur le réconcilie.
	sort.SliceStable(objects, func(i, j int) bool {
		return !isClusterObject(objects[i]) && isClusterObject(objects[j])
	})
	return objects, nil
}

func isClusterObject(obj *unstructured.Unstructured) bool {
	return obj.GroupVersionKind().Group == coreGroup && obj.GetKind() == "Cluster"
}

// createCluster gère POST /api/clusters : crée le namespace puis l'ensemble des manifests, avec annulation en cas d'échec.
func createCluster(w http.ResponseWriter, r *http.Request) {
	var payload CreateClusterPayload
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if problems := validation.IsDNS1123Label(payload.Name); len(problems) > 0 || len(payload.Name) > 58 {
		http.Error(w, "The cluster name must be a valid DNS label and allow for the capi- namespace prefix", http.StatusBadRequest)
		return
	}
	namespaceName := "capi-" + payload.Name
	objects, err := prepareManifests(payload, namespaceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	namespaceCreated := false
	if err := k8sClient.Create(ctx, ns); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Printf("Erreur K8s (Create Namespace): %v", err)
			http.Error(w, "Failed to create namespace", http.StatusInternalServerError)
			return
		}
		log.Printf("Namespace %s existe déjà, réutilisation.", namespaceName)
	} else {
		namespaceCreated = true
	}

	var created []*unstructured.Unstructured
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			if err := k8sClient.Delete(ctx, created[i]); err != nil && !apierrors.IsNotFound(err) {
				log.Printf("Erreur de nettoyage %s/%s: %v", created[i].GetKind(), created[i].GetName(), err)
			}
		}
		if namespaceCreated {
			if err := k8sClient.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
				log.Printf("Erreur de nettoyage du namespace %s: %v", namespaceName, err)
			}
		}
	}
	for _, obj := range objects {
		if err := k8sClient.Create(ctx, obj); err != nil {
			log.Printf("Erreur K8s (Create %s %s): %v", obj.GetKind(), obj.GetName(), err)
			rollback()
			status := http.StatusInternalServerError
			if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
				status = http.StatusUnprocessableEntity
			}
			http.Error(w, fmt.Sprintf("%s %s : %v", obj.GetKind(), obj.GetName(), err), status)
			return
		}
		created = append(created, obj)
	}

	response := Cluster{
		Name:              payload.Name,
		Namespace:         namespaceName,
		KubernetesVersion: payload.KubernetesVersion,
		Status:            "Pending",
	}
	for _, obj := range created {
		switch gv := obj.GroupVersionKind(); {
		case isClusterObject(obj):
			response.ID = string(obj.GetUID())
			if kind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind"); kind != "" {
				response.InfrastructureProvider = kind
			}
			if kind, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "kind"); kind != "" {
				response.ControlPlaneProvider = kind
			}
		case gv.Group == bootstrapGroup && strings.HasSuffix(obj.GetKind(), "ConfigTemplate") && response.BootstrapProvider == "":
			response.BootstrapProvider = obj.GetKind()
		}
	}
	log.Printf("Cluster CAPI %s créé (%d manifests).", payload.Name, len(created))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

var _ client.Client = nil
