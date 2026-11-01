package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type manifestKey struct {
	group, kind, name string
}

func keyForManifest(obj *unstructured.Unstructured) manifestKey {
	gv, _ := schema.ParseGroupVersion(obj.GetAPIVersion())
	return manifestKey{gv.Group, obj.GetKind(), obj.GetName()}
}

// referencesInSpec follows Kubernetes-style references, not arbitrary name fields.
func referencesInSpec(value interface{}, namespace string, refs map[manifestKey]bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		group, _ := v["apiGroup"].(string)
		if apiVersion, ok := v["apiVersion"].(string); ok {
			if gv, err := schema.ParseGroupVersion(apiVersion); err == nil {
				group = gv.Group
			}
		}
		kind, _ := v["kind"].(string)
		name, _ := v["name"].(string)
		refNamespace, _ := v["namespace"].(string)
		if isCAPIGroup(group) && kind != "" && name != "" && (refNamespace == "" || refNamespace == namespace) {
			refs[manifestKey{group, kind, name}] = true
		}
		for _, child := range v {
			referencesInSpec(child, namespace, refs)
		}
	case []interface{}:
		for _, child := range v {
			referencesInSpec(child, namespace, refs)
		}
	}
}

func selectClusterManifests(objects []unstructured.Unstructured, name, uid string) []unstructured.Unstructured {
	selected := make(map[manifestKey]bool)
	selected[manifestKey{coreGroup, "Cluster", name}] = true
	for i := range objects {
		obj := &objects[i]
		clusterName, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterName")
		if obj.GetLabels()["cluster.x-k8s.io/cluster-name"] == name || clusterName == name {
			selected[keyForManifest(obj)] = true
		}
	}
	for changed := true; changed; {
		changed = false
		refs := make(map[manifestKey]bool)
		owners := map[string]bool{uid: uid != ""}
		for i := range objects {
			obj := &objects[i]
			if selected[keyForManifest(obj)] {
				if obj.GetUID() != "" {
					owners[string(obj.GetUID())] = true
				}
				referencesInSpec(obj.Object["spec"], obj.GetNamespace(), refs)
				if keyForManifest(obj).group == coreGroup && obj.GetKind() == "Cluster" {
					className, _, _ := unstructured.NestedString(obj.Object, "spec", "topology", "classRef", "name")
					classNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "topology", "classRef", "namespace")
					if className == "" {
						className, _, _ = unstructured.NestedString(obj.Object, "spec", "topology", "class")
					}
					if className != "" && (classNamespace == "" || classNamespace == obj.GetNamespace()) {
						refs[manifestKey{coreGroup, "ClusterClass", className}] = true
					}
				}
			}
		}
		for i := range objects {
			obj := &objects[i]
			key := keyForManifest(obj)
			if selected[key] {
				continue
			}
			linked := refs[key]
			for _, owner := range obj.GetOwnerReferences() {
				if owners[string(owner.UID)] {
					linked = true
				}
			}
			if linked {
				selected[key] = true
				changed = true
			}
		}
	}
	result := make([]unstructured.Unstructured, 0)
	for i := range objects {
		if selected[keyForManifest(&objects[i])] {
			obj := objects[i].DeepCopy()
			obj.SetManagedFields(nil)
			result = append(result, *obj)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := keyForManifest(&result[i]), keyForManifest(&result[j])
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.group != b.group {
			return a.group < b.group
		}
		return a.name < b.name
	})
	return result
}

func getClusterManifests(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	cluster, err := findClusterByUID(ctx, id)
	if err != nil {
		log.Printf("Lecture du cluster %s : %v", id, err)
		http.Error(w, "Erreur lors de la lecture du cluster", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}
	crds := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := k8sClient.List(ctx, crds); err != nil {
		log.Printf("Lecture des CRD : %v", err)
		http.Error(w, "Erreur lors de la lecture des CRD", http.StatusInternalServerError)
		return
	}
	objects := make([]unstructured.Unstructured, 0)
	warnings := make([]string, 0)
	for i := range crds.Items {
		crd := &crds.Items[i]
		if crd.Spec.Scope != apiextensionsv1.NamespaceScoped || !isCAPIGroup(crd.Spec.Group) || crd.Spec.Group == "clusterctl.cluster.x-k8s.io" {
			continue
		}
		version := ""
		for _, v := range crd.Spec.Versions {
			if v.Served && (version == "" || v.Storage) {
				version = v.Name
			}
		}
		if version == "" {
			continue
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: crd.Spec.Group, Version: version, Kind: crd.Spec.Names.Kind + "List"})
		if err := k8sClient.List(ctx, list, client.InNamespace(cluster.Namespace)); err != nil {
			log.Printf("Lecture des manifests %s dans %s : %v", crd.Name, cluster.Namespace, err)
			warnings = append(warnings, fmt.Sprintf("%s : %v", crd.Name, err))
			continue
		}
		for i := range list.Items {
			list.Items[i].SetGroupVersionKind(schema.GroupVersionKind{Group: crd.Spec.Group, Version: version, Kind: crd.Spec.Names.Kind})
		}
		objects = append(objects, list.Items...)
	}
	sort.Strings(warnings)
	manifests := selectClusterManifests(objects, cluster.Name, string(cluster.UID))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"manifests": manifests, "warnings": warnings})
}
