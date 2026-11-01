package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path" // Ajouté pour parser les URLs
	"sort"
	"strings"
	"sync"
	"time"

	// --- Imports K8s étendus ---
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// --- Structures (Models) ---

type Provider struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Version       string   `json:"version"`                 // Sera peuplé dynamiquement
	RepositoryURL string   `json:"repositoryURL,omitempty"` // Sera peuplé dynamiquement
	Installed     bool     `json:"installed"`
	Status        string   `json:"status"` // active | archived | unknown
	CRDs          []string `json:"crds,omitempty"`
}

type Cluster struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Namespace              string `json:"namespace"`
	KubernetesVersion      string `json:"kubernetesVersion"`
	Status                 string `json:"status"`
	Nodes                  int    `json:"nodes"`
	InfrastructureProvider string `json:"infrastructureProvider"`
	ControlPlaneProvider   string `json:"controlPlaneProvider"`
	BootstrapProvider      string `json:"bootstrapProvider"`
	ControlPlaneReady      bool   `json:"controlPlaneReady"`
	InfrastructureReady    bool   `json:"infrastructureReady"`
}

// CreateClusterPayload décrit les manifests (ressources CAPI) à créer pour un cluster.
type CreateClusterPayload struct {
	Name              string                   `json:"name"`
	KubernetesVersion string                   `json:"kubernetesVersion"`
	Manifests         []map[string]interface{} `json:"manifests"`
}

type ScaleClusterPayload struct {
	WorkerReplicas *int64 `json:"workerReplicas"`
}

type KubeconfigResponse struct {
	ClusterName    string `json:"clusterName"`
	ServerEndpoint string `json:"serverEndpoint"`
	RawKubeconfig  string `json:"rawKubeconfig"`
}

type CreateProviderPayload struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	// Version et URL ne sont plus nécessaires ici, car le catalogue est dynamique
	// Mais on les garde pour la compatibilité de l'ajout manuel
	Version       string `json:"version"`
	RepositoryURL string `json:"repositoryURL"`
}

// NOUVEAU: Payload pour l'installation d'un provider
type InstallProviderPayload struct {
	Version string            `json:"version,omitempty"` // Optionnel: "v1.2.3"
	Env     map[string]string `json:"env,omitempty"`     // Variables d'environnement optionnelles pour le provider
}

// --- Client K8s et Catalogue ---

var (
	k8sClient    client.Client
	k8sClientset kubernetes.Interface

	// MODIFIÉ: Ce n'est plus un "mock", c'est le catalogue de base
	providerCatalog = make(map[string]Provider)
	providersMutex  = &sync.RWMutex{}
	// providerCRDMap mappe un ID de provider au nom de son CRD principal
	providerCRDMap map[string]string
)

func init() {
	// MODIFIÉ: Initialise le catalogue de base
	initProviderCatalog()

	// Initialise le map de détection CRD
	providerCRDMap = map[string]string{
		"cluster-api":       "clusters.cluster.x-k8s.io",
		"aws":               "awsclusters.infrastructure.cluster.x-k8s.io",
		"azure":             "azureclusters.infrastructure.cluster.x-k8s.io",
		"gcp":               "gcpclusters.infrastructure.cluster.x-k8s.io",
		"vsphere":           "vsphereclusters.infrastructure.cluster.x-k8s.io",
		"openstack":         "openstackclusters.infrastructure.cluster.x-k8s.io",
		"proxmox":           "proxmoxclusters.infrastructure.cluster.x-k8s.io",
		"digitalocean":      "digitaloceanclusters.infrastructure.cluster.x-k8s.io",
		"hetzner":           "hetznerclusters.infrastructure.cluster.x-k8s.io",
		"kubeadm":           "kubeadmcontrolplanes.controlplane.cluster.x-k8s.io",
		"talos":             "taloscontrolplanes.controlplane.cluster.x-k8s.io",
		"kubeadm-bootstrap": "kubeadmconfigs.bootstrap.cluster.x-k8s.io",
		"talos-bootstrap":   "talosconfigs.bootstrap.cluster.x-k8s.io",
	}
}

// MODIFIÉ: initProviderCatalog initialise le catalogue statique (sans versions)
func initProviderCatalog() {
	providersMutex.Lock()
	providerCatalog = map[string]Provider{
		// Core
		"cluster-api": {ID: "cluster-api", Name: "Cluster API (Core)", Category: "core"},
		// Infrastructure
		"aws":          {ID: "aws", Name: "AWS", Category: "infrastructure"},
		"azure":        {ID: "azure", Name: "Azure", Category: "infrastructure"},
		"gcp":          {ID: "gcp", Name: "Google Cloud", Category: "infrastructure"},
		"vsphere":      {ID: "vsphere", Name: "vSphere", Category: "infrastructure"},
		"openstack":    {ID: "openstack", Name: "OpenStack", Category: "infrastructure"},
		"proxmox":      {ID: "proxmox", Name: "Proxmox", Category: "infrastructure"},
		"digitalocean": {ID: "digitalocean", Name: "DigitalOcean", Category: "infrastructure"},
		"hetzner":      {ID: "hetzner", Name: "Hetzner", Category: "infrastructure"},
		// Control Plane
		"kubeadm": {ID: "kubeadm", Name: "Kubeadm", Category: "controlplane"},
		"talos":   {ID: "talos", Name: "Talos", Category: "controlplane"},
		// Bootstrap
		"kubeadm-bootstrap": {ID: "kubeadm-bootstrap", Name: "Kubeadm (Bootstrap)", Category: "bootstrap"},
		"talos-bootstrap":   {ID: "talos-bootstrap", Name: "Talos (Bootstrap)", Category: "bootstrap"},
	}
	providersMutex.Unlock()
}

// --- Main Function (inchangée) ---
func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Erreur GetConfig: %v", err)
	}

	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = clusterv1.AddToScheme(sch)
	_ = apiextensionsv1.AddToScheme(sch) // Pour lister les CRDs
	_ = batchv1.AddToScheme(sch)         // Pour créer les Jobs

	k8sClient, err = client.New(cfg, client.Options{Scheme: sch})
	if err != nil {
		log.Fatalf("Erreur client.New: %v", err)
	}

	k8sClientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("Avertissement: Impossible d'initialiser k8sClientset (logs indisponibles): %v", err)
	}

	log.Println("Démarrage du CAPI Dashboard Backend sur :8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/clusters", handleClusters)
	mux.HandleFunc("/api/clusters/", handleClusterByID)
	mux.HandleFunc("/api/providers", handleProviders)
	mux.HandleFunc("/api/providers/", handleProviderByID)
	mux.HandleFunc("/api/clusterctl/providers", handleProviderAvailable)
	mux.HandleFunc("/api/crds", handleCRDs)
	mux.HandleFunc("/api/crds/", handleCRDByName)
	mux.HandleFunc("/api/jobs/", handleJobs)

	registerOpenAPIRoutes(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Démarrage du CAPI Dashboard Backend sur :%s", port)

	handler := corsMiddleware(mux)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Erreur ListenAndServe: %v", err)
	}
}

func registerOpenAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "openapi.yaml")
	})
	mux.Handle("/api/docs/", http.StripPrefix("/api/docs/", http.FileServer(http.Dir("swagger-ui"))))
}

// --- Handlers (inchangés, sauf handleProviderByID) ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleClusters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getClusters(w, r)
	case "POST":
		createCluster(w, r)
	default:
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
	}
}

func handleClusterByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/clusters/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Identifiant de cluster manquant", http.StatusBadRequest)
		return
	}

	if parts[0] == "blueprint" && len(parts) == 1 && r.Method == http.MethodGet {
		getClusterBlueprint(w, r)
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			getClusterDetail(w, r, id)
		case http.MethodDelete:
			deleteCluster(w, r, id)
		default:
			http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	switch {
	case parts[1] == "kubeconfig" && r.Method == http.MethodGet:
		getClusterKubeconfig(w, r, id)
	case parts[1] == "machines" && r.Method == http.MethodGet:
		getClusterMachines(w, r, id)
	case parts[1] == "manifests" && r.Method == http.MethodGet:
		getClusterManifests(w, r, id)
	case parts[1] == "scale" && r.Method == http.MethodPatch:
		scaleCluster(w, r, id)
	default:
		http.Error(w, "Route non trouvée ou méthode non autorisée", http.StatusNotFound)
	}
}

func handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getProviders(w, r)
	case "POST":
		createProvider(w, r)
	default:
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
	}
}

// MODIFIÉ: Gère /api/providers/{id} (DELETE) et /api/providers/{id}/install (POST)
func handleProviderByID(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/")
	id := pathParts[0]

	// Route: /api/providers/{id}/install
	if len(pathParts) == 2 && pathParts[1] == "install" {
		if r.Method == "POST" {
			// MODIFIÉ: Accepte un payload
			installProvider(w, r, id)
			return
		}
	}

	// Route: /api/providers/{id}/versions
	if len(pathParts) == 2 && pathParts[1] == "versions" && r.Method == "GET" {
		getProviderVersions(w, r, id)
		return
	}

	// Route: /api/providers/{id}
	if len(pathParts) == 1 {
		switch r.Method {
		case "DELETE":
			deleteProvider(w, r, id)
			return
		}
	}

	http.Error(w, "Route non trouvée ou méthode non autorisée", http.StatusNotFound)
}

func handleProviderAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}
	repositories, err := getClusterctlRepositories()
	if err != nil {
		log.Printf("Erreur lors de la lecture des dépôts clusterctl: %v", err)
		http.Error(w, "Erreur lors de la lecture des dépôts clusterctl", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(repositories)
}

// handleCRDs gère GET /api/crds (?provider=... &group=...)
func handleCRDs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}
	providerFilter := r.URL.Query().Get("provider")
	groupFilter := r.URL.Query().Get("group")

	crds, err := listCRDs(r.Context(), providerFilter, groupFilter)
	if err != nil {
		log.Printf("Erreur listCRDs: %v", err)
		http.Error(w, fmt.Sprintf("Erreur lors de la lecture des CRDs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(crds)
}

// handleCRDByName gère GET /api/crds/{name}/schema (?version=...)
func handleCRDByName(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/crds/"), "/")
	if len(pathParts) < 1 || pathParts[0] == "" {
		http.Error(w, "Nom de CRD manquant", http.StatusBadRequest)
		return
	}

	crdName := pathParts[0]
	if len(pathParts) == 2 && pathParts[1] == "schema" {
		if r.Method != "GET" {
			http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
			return
		}
		requestedVersion := r.URL.Query().Get("version")
		schemaResp, err := getCRDSchema(r.Context(), crdName, requestedVersion)
		if err != nil {
			log.Printf("Erreur getCRDSchema (%s): %v", crdName, err)
			var notFound *crdNotFoundError
			if errors.As(err, &notFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schemaResp)
		return
	}

	http.Error(w, "Route non trouvée", http.StatusNotFound)
}

// handleJobs gère GET /api/jobs/{id}/status et GET /api/jobs/{id}/logs
func handleJobs(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/jobs/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" {
		http.Error(w, "Route job non valide", http.StatusBadRequest)
		return
	}

	jobID := pathParts[0]
	action := pathParts[1]

	if r.Method != "GET" {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	switch action {
	case "status":
		status, err := getJobStatus(r.Context(), jobID)
		if err != nil {
			log.Printf("Erreur getJobStatus (%s): %v", jobID, err)
			var notFound *jobNotFoundError
			if errors.As(err, &notFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	case "logs":
		logs, err := getJobLogs(r.Context(), jobID)
		if err != nil {
			log.Printf("Erreur getJobLogs (%s): %v", jobID, err)
			var notReady *jobLogsNotReadyError
			if errors.As(err, &notReady) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(logs))
	default:
		http.Error(w, "Action inconnue (utilisez 'status' ou 'logs')", http.StatusNotFound)
	}
}

// --- Logique "Backend" (Clusters - inchangée) ---

// getClusters (inchangé)
func getClusters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clusterListK8s := &clusterv1.ClusterList{}
	if err := k8sClient.List(ctx, clusterListK8s, client.InNamespace("")); err != nil {
		log.Printf("Erreur K8s (List Clusters): %v", err)
		http.Error(w, "Erreur lors de la lecture des clusters", http.StatusInternalServerError)
		return
	}

	// Mapper les objets K8s en objets UI
	clusterListUI := make([]Cluster, len(clusterListK8s.Items))
	for i, c := range clusterListK8s.Items {
		status := "Pending"
		if c.Status.Phase != "" {
			status = c.Status.Phase
		}

		k8sVersion := ""
		if c.Spec.Topology != nil {
			k8sVersion = c.Spec.Topology.Version
		}

		nodes, err := countNodes(ctx, c)
		if err != nil {
			log.Printf("Erreur K8s (List Machines pour %s/%s): %v", c.Namespace, c.Name, err)
			http.Error(w, "Erreur lors de la lecture des machines des clusters", http.StatusInternalServerError)
			return
		}
		bootstrapProvider, err := bootstrapProviderFromMachines(ctx, c)
		if err != nil {
			log.Printf("Erreur K8s (List bootstrap refs pour %s/%s): %v", c.Namespace, c.Name, err)
			http.Error(w, "Erreur lors de la lecture des providers des clusters", http.StatusInternalServerError)
			return
		}

		clusterListUI[i] = Cluster{
			ID:                     string(c.UID), // Utiliser l'UID K8s
			Name:                   c.Name,
			Namespace:              c.Namespace,
			KubernetesVersion:      k8sVersion,
			Status:                 status,
			Nodes:                  nodes,
			InfrastructureProvider: objectReferenceKind(c.Spec.InfrastructureRef),
			ControlPlaneProvider:   objectReferenceKind(c.Spec.ControlPlaneRef),
			BootstrapProvider:      bootstrapProvider,
			ControlPlaneReady:      c.Status.ControlPlaneReady,
			InfrastructureReady:    c.Status.InfrastructureReady,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusterListUI)
}

func countNodes(ctx context.Context, cluster clusterv1.Cluster) (int, error) {
	machines := &unstructured.UnstructuredList{}
	machines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   clusterv1.GroupVersion.Group,
		Version: clusterv1.GroupVersion.Version,
		Kind:    "MachineList",
	})
	if err := k8sClient.List(ctx, machines,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{"cluster.x-k8s.io/cluster-name": cluster.Name},
	); err != nil {
		return 0, err
	}
	return len(machines.Items), nil
}

func findClusterByUID(ctx context.Context, id string) (*clusterv1.Cluster, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := k8sClient.List(ctx, clusterList); err != nil {
		return nil, fmt.Errorf("erreur lors du listage des clusters: %w", err)
	}
	for i := range clusterList.Items {
		if string(clusterList.Items[i].UID) == id {
			return &clusterList.Items[i], nil
		}
	}
	return nil, nil
}

func getMachineDeployments(ctx context.Context, cluster clusterv1.Cluster) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   clusterv1.GroupVersion.Group,
		Version: clusterv1.GroupVersion.Version,
		Kind:    "MachineDeploymentList",
	})
	if err := k8sClient.List(ctx, list,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{"cluster.x-k8s.io/cluster-name": cluster.Name},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func getClusterDetail(w http.ResponseWriter, r *http.Request, id string) {
	cluster, err := findClusterByUID(r.Context(), id)
	if err != nil {
		log.Printf("Erreur lors de la recherche du cluster %s: %v", id, err)
		http.Error(w, "Erreur lors de la lecture du cluster", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	machineDeployments, err := getMachineDeployments(r.Context(), *cluster)
	if err != nil {
		log.Printf("Erreur lors de la lecture des MachineDeployments du cluster %s: %v", cluster.Name, err)
		http.Error(w, "Erreur lors de la lecture des MachineDeployments", http.StatusInternalServerError)
		return
	}
	deployments := make([]map[string]interface{}, 0, len(machineDeployments))
	for _, md := range machineDeployments {
		desired, _, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
		ready, _, _ := unstructured.NestedInt64(md.Object, "status", "readyReplicas")
		deployments = append(deployments, map[string]interface{}{
			"name":          md.GetName(),
			"replicas":      desired,
			"readyReplicas": ready,
		})
	}

	nodes, err := countNodes(r.Context(), *cluster)
	if err != nil {
		log.Printf("Erreur lors du comptage des machines du cluster %s: %v", cluster.Name, err)
		http.Error(w, "Erreur lors de la lecture des machines", http.StatusInternalServerError)
		return
	}
	bootstrapProvider, err := bootstrapProviderFromMachines(r.Context(), *cluster)
	if err != nil {
		log.Printf("Erreur lors de la lecture du provider bootstrap du cluster %s: %v", cluster.Name, err)
		http.Error(w, "Erreur lors de la lecture des providers du cluster", http.StatusInternalServerError)
		return
	}
	version := ""
	if cluster.Spec.Topology != nil {
		version = cluster.Spec.Topology.Version
	}
	status := string(cluster.Status.Phase)
	if status == "" {
		status = "Pending"
	}

	w.Header().Set("Content-Type", "application/json")
	conditions := make([]interface{}, 0, len(cluster.Status.Conditions))
	for _, condition := range cluster.Status.Conditions {
		conditions = append(conditions, condition)
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                     string(cluster.UID),
		"name":                   cluster.Name,
		"namespace":              cluster.Namespace,
		"kubernetesVersion":      version,
		"status":                 status,
		"nodes":                  nodes,
		"controlPlaneReady":      cluster.Status.ControlPlaneReady,
		"infrastructureReady":    cluster.Status.InfrastructureReady,
		"infrastructureProvider": objectReferenceKind(cluster.Spec.InfrastructureRef),
		"controlPlaneProvider":   objectReferenceKind(cluster.Spec.ControlPlaneRef),
		"bootstrapProvider":      bootstrapProvider,
		"conditions":             conditions,
		"controlPlaneRef":        cluster.Spec.ControlPlaneRef,
		"infrastructureRef":      cluster.Spec.InfrastructureRef,
		"machineDeployments":     deployments,
	})
}

func objectReferenceKind(reference *corev1.ObjectReference) string {
	if reference == nil {
		return ""
	}
	return reference.Kind
}

func getClusterMachines(w http.ResponseWriter, r *http.Request, id string) {
	cluster, err := findClusterByUID(r.Context(), id)
	if err != nil {
		log.Printf("Erreur lors de la recherche du cluster %s: %v", id, err)
		http.Error(w, "Erreur lors de la lecture du cluster", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	machines := &unstructured.UnstructuredList{}
	machines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   clusterv1.GroupVersion.Group,
		Version: clusterv1.GroupVersion.Version,
		Kind:    "MachineList",
	})
	if err := k8sClient.List(r.Context(), machines,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{"cluster.x-k8s.io/cluster-name": cluster.Name},
	); err != nil {
		log.Printf("Erreur lors de la lecture des machines du cluster %s: %v", cluster.Name, err)
		http.Error(w, "Erreur lors de la lecture des machines", http.StatusInternalServerError)
		return
	}

	items := make([]map[string]interface{}, 0, len(machines.Items))
	for _, machine := range machines.Items {
		phase, _, _ := unstructured.NestedString(machine.Object, "status", "phase")
		version, _, _ := unstructured.NestedString(machine.Object, "spec", "version")
		providerID, _, _ := unstructured.NestedString(machine.Object, "spec", "providerID")
		internalIP, externalIP := machineIPs(machine.Object)
		role := "worker"
		if _, isControlPlane := machine.GetLabels()["cluster.x-k8s.io/control-plane"]; isControlPlane {
			role = "control-plane"
		}
		if nodeName, found, _ := unstructured.NestedString(machine.Object, "status", "nodeRef", "name"); found && nodeName != "" {
			node := &corev1.Node{}
			if err := k8sClient.Get(r.Context(), client.ObjectKey{Name: nodeName}, node); err == nil {
				for _, address := range node.Status.Addresses {
					switch address.Type {
					case corev1.NodeInternalIP:
						internalIP = address.Address
					case corev1.NodeExternalIP:
						externalIP = address.Address
					}
				}
			} else if !apierrors.IsNotFound(err) {
				log.Printf("Erreur lors de la lecture du Node %s: %v", nodeName, err)
				http.Error(w, "Erreur lors de la lecture des adresses des machines", http.StatusInternalServerError)
				return
			}
		}
		items = append(items, map[string]interface{}{
			"name":              machine.GetName(),
			"phase":             phase,
			"version":           version,
			"internalIP":        internalIP,
			"externalIP":        externalIP,
			"ipAddress":         firstNonEmpty(externalIP, internalIP),
			"providerID":        providerID,
			"role":              role,
			"creationTimestamp": machine.GetCreationTimestamp(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func machineIPs(machine map[string]interface{}) (string, string) {
	var internalIP, externalIP string
	addresses, _, _ := unstructured.NestedSlice(machine, "status", "addresses")
	for _, rawAddress := range addresses {
		address, ok := rawAddress.(map[string]interface{})
		if !ok {
			continue
		}
		typeName, _ := address["type"].(string)
		value, _ := address["address"].(string)
		switch typeName {
		case "InternalIP":
			internalIP = value
		case "ExternalIP":
			externalIP = value
		}
	}
	return internalIP, externalIP
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func bootstrapProviderFromMachines(ctx context.Context, cluster clusterv1.Cluster) (string, error) {
	machines := &unstructured.UnstructuredList{}
	machines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   clusterv1.GroupVersion.Group,
		Version: clusterv1.GroupVersion.Version,
		Kind:    "MachineList",
	})
	if err := k8sClient.List(ctx, machines,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{"cluster.x-k8s.io/cluster-name": cluster.Name},
	); err != nil {
		return "", err
	}
	for _, machine := range machines.Items {
		kind, _, _ := unstructured.NestedString(machine.Object, "spec", "bootstrap", "configRef", "kind")
		if kind != "" {
			return kind, nil
		}
	}
	return "", nil
}

func scaleCluster(w http.ResponseWriter, r *http.Request, id string) {
	var payload ScaleClusterPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Payload JSON invalide", http.StatusBadRequest)
		return
	}
	if payload.WorkerReplicas == nil {
		http.Error(w, "workerReplicas est obligatoire", http.StatusBadRequest)
		return
	}
	if *payload.WorkerReplicas < 0 || *payload.WorkerReplicas > 500 {
		http.Error(w, "workerReplicas doit être compris entre 0 et 500", http.StatusBadRequest)
		return
	}

	cluster, err := findClusterByUID(r.Context(), id)
	if err != nil {
		log.Printf("Erreur lors de la recherche du cluster %s: %v", id, err)
		http.Error(w, "Erreur lors de la lecture du cluster", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}
	deployments, err := getMachineDeployments(r.Context(), *cluster)
	if err != nil {
		log.Printf("Erreur lors de la lecture des MachineDeployments du cluster %s: %v", cluster.Name, err)
		http.Error(w, "Erreur lors de la lecture des MachineDeployments", http.StatusInternalServerError)
		return
	}
	if len(deployments) == 0 {
		http.Error(w, "Aucun MachineDeployment worker n'est disponible pour ce cluster", http.StatusConflict)
		return
	}

	for i := range deployments {
		original := deployments[i].DeepCopy()
		if err := unstructured.SetNestedField(deployments[i].Object, *payload.WorkerReplicas, "spec", "replicas"); err != nil {
			log.Printf("Impossible de préparer le scaling de %s: %v", deployments[i].GetName(), err)
			http.Error(w, "Impossible de préparer la mise à l'échelle", http.StatusInternalServerError)
			return
		}
		if err := k8sClient.Patch(r.Context(), &deployments[i], client.MergeFrom(original)); err != nil {
			log.Printf("Erreur lors du scaling du MachineDeployment %s: %v", deployments[i].GetName(), err)
			http.Error(w, "Erreur lors de la mise à l'échelle", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Réplicas des workers mis à jour.",
		"workerReplicas": *payload.WorkerReplicas,
	})
}

func getClusterKubeconfig(w http.ResponseWriter, r *http.Request, id string) {
	cluster, err := findClusterByUID(r.Context(), id)
	if err != nil {
		log.Printf("Erreur lors de la recherche du cluster %s: %v", id, err)
		http.Error(w, "Erreur lors de la lecture du cluster", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	secret := &corev1.Secret{}
	secretName := cluster.Name + "-kubeconfig"
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: cluster.Namespace, Name: secretName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "Le kubeconfig n'est pas encore disponible : le control plane est en cours d'initialisation.", http.StatusConflict)
			return
		}
		log.Printf("Erreur lors de la lecture du Secret %s/%s: %v", cluster.Namespace, secretName, err)
		http.Error(w, "Erreur lors de la lecture du kubeconfig", http.StatusInternalServerError)
		return
	}
	rawKubeconfig, ok := secret.Data["value"]
	if !ok || len(rawKubeconfig) == 0 {
		http.Error(w, "Le kubeconfig n'est pas encore disponible : le control plane est en cours d'initialisation.", http.StatusConflict)
		return
	}

	serverEndpoint, err := kubeconfigServerEndpoint(rawKubeconfig)
	if err != nil {
		log.Printf("Impossible de lire l'endpoint depuis le kubeconfig %s/%s: %v", cluster.Namespace, secretName, err)
		http.Error(w, "Le kubeconfig du cluster est invalide", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.kubeconfig"`, cluster.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	_ = json.NewEncoder(w).Encode(KubeconfigResponse{
		ClusterName:    cluster.Name,
		ServerEndpoint: serverEndpoint,
		RawKubeconfig:  string(rawKubeconfig),
	})
}

func kubeconfigServerEndpoint(raw []byte) (string, error) {
	var config struct {
		Clusters []struct {
			Cluster struct {
				Server string `json:"server"`
			} `json:"cluster"`
		} `json:"clusters"`
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return "", err
	}
	if len(config.Clusters) == 0 || config.Clusters[0].Cluster.Server == "" {
		return "", fmt.Errorf("endpoint du serveur absent")
	}
	return config.Clusters[0].Cluster.Server, nil
}

// deleteCluster (inchangé)
func deleteCluster(w http.ResponseWriter, r *http.Request, id string) {
	clusterToDelete, err := findClusterByUID(r.Context(), id)
	if err != nil {
		log.Printf("Erreur lors de la recherche du cluster %s: %v", id, err)
		http.Error(w, "Impossible de lire les clusters", http.StatusInternalServerError)
		return
	}
	if clusterToDelete == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	// 3. Supprimer le cluster CAPI
	// (Le CAPI controller s'occupera de supprimer le namespace si l'annotation est présente)
	if err := k8sClient.Delete(r.Context(), clusterToDelete); err != nil {
		log.Printf("Erreur K8s (Delete Cluster): %v", err)
		http.Error(w, "Erreur lors de la suppression du cluster", http.StatusInternalServerError)
		return
	}
	log.Printf("Cluster CAPI %s supprimé.", clusterToDelete.Name)

	w.WriteHeader(http.StatusNoContent)
}

// --- Logique "Catalogue Provider" (MODIFIÉE) ---

// extractVersionFromURL extrait proprement la version (ex: v1.9.0 ou latest) d'une URL de release
func extractVersionFromURL(url string) string {
	cleanURL := strings.TrimSuffix(url, "/")
	base := path.Base(cleanURL)
	if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
		dir := path.Dir(cleanURL)
		base = path.Base(dir)
	}
	if base == "" || base == "." || base == "/" {
		return "latest"
	}
	return base
}

// typeToCategory mappe le type CAPI (ex. InfrastructureProvider) en catégorie normalisée
func typeToCategory(providerType string) string {
	switch providerType {
	case "InfrastructureProvider":
		return "infrastructure"
	case "ControlPlaneProvider":
		return "controlplane"
	case "BootstrapProvider":
		return "bootstrap"
	case "CoreProvider":
		return "core"
	case "IPAMProvider":
		return "ipam"
	case "AddonProvider":
		return "addon"
	default:
		return strings.ToLower(strings.TrimSuffix(providerType, "Provider"))
	}
}

// NOUVEAU: Exécute 'clusterctl config repositories' et parse la sortie
// Renvoie map[providerID] -> map["version"|"url"|"category"]
func getClusterctlRepositories() (map[string]map[string]string, error) {
	cmd := exec.Command("clusterctl", "config", "repositories")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de clusterctl: %v, stderr: %s", err, stderr.String())
	}

	repoMap := make(map[string]map[string]string)
	scanner := bufio.NewScanner(&out)
	scanner.Scan() // Skip header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// S'attend à une sortie comme: NAME  TYPE  URL  FILE
		if len(fields) >= 3 {
			id := fields[0]
			provType := fields[1]
			url := fields[2]
			version := extractVersionFromURL(url)
			category := typeToCategory(provType)

			repoMap[id] = map[string]string{
				"url":      url,
				"version":  version,
				"category": category,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture des dépôts clusterctl: %w", err)
	}
	return repoMap, nil
}

// getInstalledProviderCRDs détecte dynamiquement les CRDs rattachées à chaque provider
func getInstalledProviderCRDs(ctx context.Context) (map[string][]string, error) {
	crdListK8s := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := k8sClient.List(ctx, crdListK8s); err != nil {
		log.Printf("Erreur K8s (List CRDs): %v", err)
		return nil, err
	}

	providerCRDs := make(map[string][]string)
	for _, crd := range crdListK8s.Items {
		providerID := detectProviderFromCRD(crd.Name, crd.Spec.Group, crd.Spec.Names.Kind, crd.GetLabels(), crd.GetAnnotations())
		if providerID != "unknown" {
			providerCRDs[providerID] = append(providerCRDs[providerID], crd.Name)
		}

		// Compatibilité avec le mapping statique providerCRDMap
		for id, crdName := range providerCRDMap {
			if crd.Name == crdName {
				found := false
				for _, existing := range providerCRDs[id] {
					if existing == crd.Name {
						found = true
						break
					}
				}
				if !found {
					providerCRDs[id] = append(providerCRDs[id], crd.Name)
				}
			}
		}
	}
	return providerCRDs, nil
}

// getProviders (MODIFIÉ)
// Fusionne les données du catalogue statique, de clusterctl, et des CRDs
func getProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Obtenir les données dynamiques de 'clusterctl config repositories'
	repoData, err := getClusterctlRepositories()
	if err != nil {
		log.Printf("Erreur getClusterctlRepositories: %v", err)
		http.Error(w, "Erreur lors de la lecture de la configuration clusterctl", http.StatusInternalServerError)
		return
	}

	// 2. Obtenir le statut d'installation et la liste des CRDs
	installedCRDMap, err := getInstalledProviderCRDs(ctx)
	if err != nil {
		http.Error(w, "Erreur lors de la lecture des CRDs", http.StatusInternalServerError)
		return
	}

	// 3. Fusionner avec le catalogue statique
	providersMutex.RLock()
	defer providersMutex.RUnlock()

	seen := make(map[string]bool)
	providerList := make([]Provider, 0, len(providerCatalog)+len(repoData))
	for _, p := range providerCatalog {
		crds := installedCRDMap[p.ID]
		p.Installed = len(crds) > 0
		p.CRDs = crds

		// Enrichir avec les données de clusterctl
		if data, ok := repoData[p.ID]; ok {
			p.RepositoryURL = data["url"]
			p.Version = data["version"] // C'est la version par défaut
		}

		seen[p.ID] = true
		providerList = append(providerList, p)
	}

	// Ajouter dynamiquement tous les autres providers découverts par clusterctl
	for id, data := range repoData {
		if !seen[id] {
			crds := installedCRDMap[id]
			category := data["category"]
			if category == "" {
				category = "infrastructure"
			}
			displayName := strings.ToUpper(id[:1]) + id[1:]
			p := Provider{
				ID:            id,
				Name:          displayName,
				Category:      category,
				Version:       data["version"],
				RepositoryURL: data["url"],
				Installed:     len(crds) > 0,
				CRDs:          crds,
			}
			seen[id] = true
			providerList = append(providerList, p)
		}
	}

	// 4. Statut (actif/archivé) déduit des dépôts GitHub des providers
	repos := make(map[string]string, len(providerList))
	for _, p := range providerList {
		if repo, ok := githubRepoFromURL(p.RepositoryURL); ok {
			repos[p.ID] = repo
		}
	}
	statuses := providerStatuses(ctx, repos)
	for i := range providerList {
		if status, ok := statuses[providerList[i].ID]; ok {
			providerList[i].Status = status
		} else {
			providerList[i].Status = providerStatusUnknown
		}
	}

	// 5. Trier et renvoyer
	sort.Slice(providerList, func(i, j int) bool {
		return providerList[i].Name < providerList[j].Name
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerList)
}

// createProvider (Inchangé - modifie le catalogue statique)
func createProvider(w http.ResponseWriter, r *http.Request) {
	var payload CreateProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		http.Error(w, "Le nom du provider est obligatoire", http.StatusBadRequest)
		return
	}
	switch payload.Category {
	case "infrastructure", "controlplane", "bootstrap", "core":
	default:
		http.Error(w, "Catégorie de provider invalide", http.StatusBadRequest)
		return
	}
	providerID := strings.ToLower(payload.Name)
	if problems := validation.IsDNS1123Label(providerID); len(problems) > 0 {
		http.Error(w, "Le nom du provider doit également être un identifiant DNS valide", http.StatusBadRequest)
		return
	}

	newProvider := Provider{
		ID:       providerID,
		Name:     payload.Name,
		Category: payload.Category,
		// Version et URL sont optionnels, clusterctl les trouvera
		Version:       payload.Version,
		RepositoryURL: payload.RepositoryURL,
		Installed:     false,
	}

	providersMutex.Lock()
	providerCatalog[newProvider.ID] = newProvider
	providersMutex.Unlock()
	log.Printf("Provider %s ajouté au catalogue de base.", newProvider.ID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newProvider)
}

// deleteProvider (MODIFIÉ - Gère ?includeCRD=true)
func deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// NOUVEAU: Lire le paramètre de requête pour --include-crd
	includeCRD := r.URL.Query().Get("includeCRD") == "true"
	if includeCRD {
		// Le frontend doit afficher cette mise en garde AVANT l'appel,
		// mais nous l'ajoutons au log du backend pour plus de sécurité.
		log.Printf("ATTENTION: Lancement de la suppression de %s avec l'option --include-crd. Tous les objets personnalisés (clusters, machines, etc.) liés à ce provider seront supprimés.", id)
	}

	// 1. Récupérer les infos (catégorie) depuis notre catalogue statique
	providersMutex.RLock()
	providerInfo, ok := providerCatalog[id]
	providersMutex.RUnlock()
	if !ok {
		repositories, err := getClusterctlRepositories()
		if err != nil {
			log.Printf("Erreur lors de la découverte du provider %s: %v", id, err)
			http.Error(w, "Impossible de découvrir ce provider", http.StatusInternalServerError)
			return
		}
		repository, found := repositories[id]
		if !found {
			http.Error(w, "Provider non trouvé", http.StatusNotFound)
			return
		}
		providerInfo = Provider{ID: id, Name: id, Category: repository["category"]}
	}
	if problems := validation.IsDNS1123Label(id); len(problems) > 0 {
		http.Error(w, "Identifiant de provider invalide", http.StatusBadRequest)
		return
	}

	// 2. Obtenir la commande de suppression (en passant le flag includeCRD)
	deleteCmdArgs := getProviderDeleteCommand(providerInfo.Category, id, includeCRD)
	if deleteCmdArgs == nil {
		http.Error(w, "Catégorie de provider non supprimable ou inconnue", http.StatusBadRequest)
		return
	}
	// 3. Définir le Job Kubernetes pour exécuter la commande
	jobName := fmt.Sprintf("delete-%s-%d", id, time.Now().Unix())
	automountToken := true
	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "capi-dashboard-system", // Doit être le même namespace que l'installer
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName:           "clusterctl-installer-sa", // Utilise le même SA
					AutomountServiceAccountToken: &automountToken,
					Containers: []corev1.Container{
						{
							Name:            "uninstaller",
							Image:           clusterctlImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"clusterctl"},
							Args:            deleteCmdArgs,
							Env: append([]corev1.EnvVar{
								{
									Name:  "KUBERNETES_SERVICE_HOST",
									Value: "kubernetes.default.svc.cluster.local",
								},
								{
									Name:  "KUBERNETES_SERVICE_PORT",
									Value: "443",
								},
							}, githubTokenEnv()...),
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	// 4. Créer le Job
	if err := k8sClient.Create(ctx, job); err != nil {
		log.Printf("Erreur K8s (Create Job Delete): %v", err)
		http.Error(w, "Erreur lors de la création du Job de suppression", http.StatusInternalServerError)
		return
	}

	log.Printf("Job %s créé pour supprimer le provider %s (IncludeCRD: %t)", jobName, id, includeCRD)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "Suppression du provider initiée.", "jobName": jobName})
}

// installProvider (MODIFIÉ)
// Accepte un payload pour une version optionnelle
func installProvider(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	if problems := validation.IsDNS1123Label(id); len(problems) > 0 {
		http.Error(w, "Identifiant de provider invalide", http.StatusBadRequest)
		return
	}

	// 1. Décoder le payload optionnel
	var payload InstallProviderPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	// Permettre un corps vide (EOF est ok si ContentLength == 0)
	if err != nil && err != io.EOF {
		if r.ContentLength > 0 {
			http.Error(w, fmt.Sprintf("Erreur de décodage du payload: %v", err), http.StatusBadRequest)
			return
		}
	}
	for name := range payload.Env {
		if problems := validation.IsEnvVarName(name); len(problems) > 0 {
			http.Error(w, fmt.Sprintf("Nom de variable d'environnement invalide %q: %s", name, strings.Join(problems, ", ")), http.StatusBadRequest)
			return
		}
		switch name {
		case "KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT", "GOPROXY", "GITHUB_TOKEN":
			http.Error(w, fmt.Sprintf("La variable d'environnement %q est réservée", name), http.StatusBadRequest)
			return
		}
	}
	versionToInstall := payload.Version
	if versionToInstall != "" && !isValidProviderVersion(versionToInstall) {
		http.Error(w, "Version invalide (format attendu: v1.2.3)", http.StatusBadRequest)
		return
	}

	// 2. Si la version n'est pas spécifiée, trouver la version par défaut
	if versionToInstall == "" {
		log.Printf("Aucune version spécifiée pour %s, recherche de la version par défaut via clusterctl...", id)
		repoData, err := getClusterctlRepositories()
		if err != nil {
			log.Printf("Erreur getClusterctlRepositories: %v", err)
			http.Error(w, "Erreur lors de la lecture de la configuration clusterctl", http.StatusInternalServerError)
			return
		}
		data, ok := repoData[id]
		if !ok {
			http.Error(w, "Provider non trouvé dans la configuration clusterctl", http.StatusNotFound)
			return
		}
		versionToInstall = data["version"]
		log.Printf("Version par défaut trouvée: %s", versionToInstall)
	} else {
		log.Printf("Installation du provider %s avec la version manuelle: %s", id, versionToInstall)
	}

	// 3. Récupérer les infos (catégorie) depuis notre catalogue statique
	// Si non trouvé dans le catalogue, on tente de déterminer la catégorie via clusterctl
	providersMutex.RLock()
	providerInfo, ok := providerCatalog[id]
	providersMutex.RUnlock()
	if !ok {
		// Fallback: chercher la catégorie via clusterctl repositories
		repoData, repoErr := getClusterctlRepositories()
		if repoErr == nil {
			if data, found := repoData[id]; found {
				providerInfo = Provider{
					ID:       id,
					Name:     id,
					Category: data["category"],
				}
				ok = true
			}
		}
		if !ok {
			http.Error(w, "Provider non trouvé dans le catalogue interne ni dans clusterctl", http.StatusNotFound)
			return
		}
	}

	// Un provider archivé ne peut plus être installé (la désinstallation reste possible)
	if repoData, repoErr := getClusterctlRepositories(); repoErr == nil {
		if repo, found := githubRepoFromURL(repoData[id]["url"]); found && fetchGithubRepoStatus(ctx, repo) == providerStatusArchived {
			http.Error(w, "Ce provider est archivé et ne peut plus être installé", http.StatusConflict)
			return
		}
	}

	// 4. Obtenir la commande d'installation
	installCmdArgs := getProviderInstallCommand(providerInfo.Category, id, versionToInstall)
	if installCmdArgs == nil {
		http.Error(w, "Catégorie de provider non installable ou inconnue", http.StatusBadRequest)
		return
	}
	fullCommandString := "clusterctl " + strings.Join(installCmdArgs, " ")
	//fullCommandString := "clusterctl config repositories && " + strings.Join(installCmdArgs, " ")
	log.Printf("Commande clusterctl appliquée: %s", fullCommandString)

	// 5. Définir le Job Kubernetes (inchangé par rapport à votre version corrigée)
	jobName := fmt.Sprintf("install-%s-%d", id, time.Now().Unix())
	automountToken := true
	// Le ConfigMap n'est requis que pour les providers personnalisés : clusterctl connaît déjà les providers officiels
	optionalConfig := true
	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "capi-dashboard-system",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName:           "clusterctl-installer-sa",
					AutomountServiceAccountToken: &automountToken,
					Containers: []corev1.Container{
						{
							Name:            "installer",
							Image:           clusterctlImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"clusterctl"},
							Args:            installCmdArgs,
							Env: func() []corev1.EnvVar {
								envVars := []corev1.EnvVar{
									{
										Name:  "KUBERNETES_SERVICE_HOST",
										Value: "kubernetes.default.svc.cluster.local",
									},
									{
										Name:  "KUBERNETES_SERVICE_PORT",
										Value: "443",
									},
									{
										Name:  "GOPROXY",
										Value: "off",
									},
								}
								envVars = append(envVars, githubTokenEnv()...)
								// Injecter les variables d'env du payload (ex: AWS_B64ENCODED_CREDENTIALS)
								for k, v := range payload.Env {
									envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
								}
								return envVars
							}(),
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "clusterctl-config-volume",
								MountPath: "/root/.cluster-api",
							}},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{
						Name: "clusterctl-config-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "clusterctl-config"},
								Optional:             &optionalConfig,
							},
						},
					}},
				},
			},
		},
	}

	// 6. Créer le Job
	if err := k8sClient.Create(ctx, job); err != nil {
		log.Printf("Erreur K8s (Create Job): %v", err)
		http.Error(w, "Erreur lors de la création du Job d'installation", http.StatusInternalServerError)
		return
	}

	log.Printf("Job %s créé pour installer le provider %s:%s", jobName, id, versionToInstall)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "Installation du provider initiée.", "jobName": jobName})
}

// MODIFIÉ: Construit la commande pour 'clusterctl delete' avec le flag optionnel
func getProviderDeleteCommand(category string, providerID string, includeCRD bool) []string {
	var flag string
	switch category {
	case "infrastructure":
		flag = "--infrastructure"
	case "controlplane":
		flag = "--control-plane"
	case "bootstrap":
		flag = "--bootstrap"
	case "core":
		flag = "--core"
	default:
		return nil // Catégorie non supportée
	}

	// La commande de base
	cmd := []string{"delete", flag, providerID}

	// NOUVEAU: Ajouter le flag si demandé
	if includeCRD {
		cmd = append(cmd, "--include-crd")
	}

	return cmd
}

// MODIFIÉ: Signature mise à jour pour accepter la catégorie, l'ID et la version
func getProviderInstallCommand(category string, providerID string, version string) []string {
	var flag string
	switch category {
	case "infrastructure":
		flag = "--infrastructure"
	case "controlplane":
		flag = "--control-plane"
	case "bootstrap":
		flag = "--bootstrap"
	case "core":
		flag = "--core"
	default:
		return nil // Catégorie non supportée
	}
	// Construit la commande avec l'ID et la version résolue
	// FIXÉ: chaque argument doit être un élément séparé du slice (pas une seule string "init -n ...")
	if version == "" || version == "latest" {
		return []string{"init", flag, providerID}
	}
	return []string{"init", flag, fmt.Sprintf("%s:%s", providerID, version)}
}

// clusterctlImage retourne l'image des Jobs clusterctl (CLUSTERCTL_IMAGE, sinon l'image par défaut).
func clusterctlImage() string {
	if image := os.Getenv("CLUSTERCTL_IMAGE"); image != "" {
		return image
	}
	return "192.168.1.9:5000/capimgr-clusterctl:latest"
}

// githubTokenEnv transmet aux Jobs clusterctl le token GitHub (Secret GITHUB_TOKEN_SECRET, clé "token"),
// afin d'éviter la limite de 60 requêtes/h de l'API publique. Le token n'est jamais copié en clair dans le Job.
func githubTokenEnv() []corev1.EnvVar {
	secretName := os.Getenv("GITHUB_TOKEN_SECRET")
	if secretName == "" {
		return nil
	}
	optional := true
	return []corev1.EnvVar{{
		Name: "GITHUB_TOKEN",
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "token",
			Optional:             &optional,
		}},
	}}
}
