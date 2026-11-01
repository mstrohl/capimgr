package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

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
}

type CreateClusterPayload struct {
	Name                   string `json:"name"`
	KubernetesVersion      string `json:"kubernetesVersion"`
	ControlPlaneReplicas   int    `json:"controlPlaneReplicas"` // Non utilisé pour l'instant, mais bon à avoir
	WorkerReplicas         int    `json:"workerReplicas"`       // Non utilisé
	InfrastructureProvider string `json:"infrastructureProvider"`
	ControlPlaneProvider   string `json:"controlPlaneProvider"`
	BootstrapProvider      string `json:"bootstrapProvider"`
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
	// providerKindMap mappe un ID de provider à l'ApiVersion/Kind qu'il gère
	providerKindMap map[string]map[string]string

	// providerCRDMap mappe un ID de provider au nom de son CRD principal
	providerCRDMap map[string]string
)

func init() {
	// MODIFIÉ: Initialise le catalogue de base
	initProviderCatalog()

	// Initialise le map ApiVersion/Kind pour la création de Cluster
	providerKindMap = map[string]map[string]string{
		// Infrastructure
		"aws": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2", // Note: CAPI utilise v1beta2 pour AWS
			"kind":       "AWSCluster",
		},
		"azure": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "AzureCluster",
		},
		"gcp": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "GCPCluster",
		},
		"vsphere": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "VSphereCluster",
		},
		"openstack": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1", // v1alpha7 est aussi commun
			"kind":       "OpenStackCluster",
		},
		"proxmox": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
			"kind":       "ProxmoxCluster",
		},
		"digitalocean": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "DigitalOceanCluster",
		},
		"hetzner": {
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "HetznerCluster",
		},
		// Control Plane
		"kubeadm": {
			"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
			"kind":       "KubeadmControlPlane",
		},
		"talos": {
			"apiVersion": "controlplane.cluster.x-k8s.io/v1alpha2",
			"kind":       "TalosControlPlane",
		},
		// Bootstrap
		"kubeadm-bootstrap": {
			"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
			"kind":       "KubeadmConfig",
		},
		"talos-bootstrap": {
			"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
			"kind":       "TalosConfig",
		},
	}

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

	// Swagger & OpenAPI
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "openapi.yaml")
	})

	//// --- Swagger UI Routes ---
	//// Sert le fichier openapi.yaml
	//mux.HandleFunc("/openapi", func(w http.ResponseWriter, r *http.Request) {
	//	http.ServeFile(w, r, "openapi.yaml")
	//})
	//// Sert l'interface Swagger UI
	//fs := http.FileServer(http.Dir("./swagger-ui"))
	//mux.Handle("/", fs) // Assurez-vous que c'est bien le dernier handle pour ne pas écraser les autres

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

// --- Handlers (inchangés, sauf handleProviderByID) ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
	id := strings.TrimPrefix(r.URL.Path, "/api/clusters/")
	switch r.Method {
	case "DELETE":
		deleteCluster(w, r, id)
	default:
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
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
	switch r.Method {
	case "GET":
		getClusterctlRepositories()
	default:
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
	}
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
			http.Error(w, err.Error(), http.StatusNotFound)
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
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	case "logs":
		logs, err := getJobLogs(r.Context(), jobID)
		if err != nil {
			log.Printf("Erreur getJobLogs (%s): %v", jobID, err)
			http.Error(w, err.Error(), http.StatusNotFound)
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
	ctx := context.Background()
	clusterListK8s := &clusterv1.ClusterList{}
	if err := k8sClient.List(ctx, clusterListK8s, client.InNamespace("")); err != nil {
		log.Printf("Erreur K8s (List Clusters): %v", err)
		http.Error(w, "Erreur lors de la lecture des clusters", http.StatusInternalServerError)
		return
	}

	// Mapper les objets K8s en objets UI
	clusterListUI := make([]Cluster, len(clusterListK8s.Items))
	for i, c := range clusterListK8s.Items {
		status := "Provisionnement"
		if c.Status.Phase != "" {
			status = c.Status.Phase
		}
		if !c.Status.ControlPlaneReady {
			status = "Control Plane non prêt"
		}

		k8sVersion := ""
		if c.Spec.Topology != nil {
			k8sVersion = c.Spec.Topology.Version
		}

		clusterListUI[i] = Cluster{
			ID:                     string(c.UID), // Utiliser l'UID K8s
			Name:                   c.Name,
			Namespace:              c.Namespace,
			KubernetesVersion:      k8sVersion,
			Status:                 status,
			Nodes:                  countNodes(c),
			InfrastructureProvider: c.Spec.InfrastructureRef.Kind,
			ControlPlaneProvider:   c.Spec.ControlPlaneRef.Kind,
			BootstrapProvider:      "Inconnu", // CAPI ne stocke pas le bootstrap ref ici
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusterListUI)
}

// Simule le comptage des noeuds
func countNodes(cluster clusterv1.Cluster) int {
	// Logique réelle: lister les 'Machine' CAPI pour ce cluster
	// Pour l'instant, on simule
	if cluster.Status.Phase == "Provisioned" {
		return 3 // 1 CP, 2 Workers (simulé)
	}
	return 0
}

// createCluster (inchangé)
func createCluster(w http.ResponseWriter, r *http.Request) {
	var payload CreateClusterPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := context.Background()

	// 1. Définir le Namespace
	namespaceName := "capi-" + payload.Name
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}

	// 2. Créer le Namespace
	if err := k8sClient.Create(ctx, ns); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Printf("Erreur K8s (Create Namespace): %v", err)
			http.Error(w, "Erreur lors de la création du namespace", http.StatusInternalServerError)
			return
		}
		log.Printf("Namespace %s existe déjà, réutilisation.", namespaceName)
	} else {
		log.Printf("Namespace %s créé.", namespaceName)
	}

	// 3. Définir l'objet Cluster CAPI
	// Note: C'est un cluster "Topology-Aware" basique
	newCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      payload.Name,
			Namespace: namespaceName,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   "default", // L'utilisateur doit créer un ClusterClass 'default'
				Version: payload.KubernetesVersion,
				// Les Replica counts vont dans le ClusterClass
			},
			InfrastructureRef: getRefFromPayload(payload.InfrastructureProvider, payload.Name),
			ControlPlaneRef:   getRefFromPayload(payload.ControlPlaneProvider, payload.Name),
		},
	}

	// 4. Créer l'objet Cluster
	if err := k8sClient.Create(ctx, newCluster); err != nil {
		log.Printf("Erreur K8s (Create Cluster): %v", err)
		http.Error(w, "Erreur lors de la création du cluster", http.StatusInternalServerError)
		return
	}
	log.Printf("Cluster CAPI %s créé.", newCluster.Name)

	// 5. Renvoyer la nouvelle ressource (simplifiée)
	newClusterUI := Cluster{
		ID:                     string(newCluster.UID),
		Name:                   newCluster.Name,
		Namespace:              newCluster.Namespace,
		KubernetesVersion:      newCluster.Spec.Topology.Version,
		Status:                 "En attente",
		Nodes:                  0,
		InfrastructureProvider: newCluster.Spec.InfrastructureRef.Kind,
		ControlPlaneProvider:   newCluster.Spec.ControlPlaneRef.Kind,
		BootstrapProvider:      payload.BootstrapProvider, // On triche un peu
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newClusterUI)
}

// Helper pour traduire l'ID du payload en ObjectReference CAPI
func getRefFromPayload(providerID string, clusterName string) *corev1.ObjectReference {
	spec, ok := providerKindMap[providerID]
	if !ok {
		// Fallback
		return &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Unknown",
			Name:       clusterName,
		}
	}
	return &corev1.ObjectReference{
		APIVersion: spec["apiVersion"],
		Kind:       spec["kind"],
		Name:       clusterName, // CAPI attend que l'objet Infra/CP ait le même nom
	}
}

// deleteCluster (inchangé)
func deleteCluster(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()

	// 1. Lister tous les clusters dans tous les namespaces
	clusterListK8s := &clusterv1.ClusterList{}
	if err := k8sClient.List(ctx, clusterListK8s, client.InNamespace("")); err != nil {
		http.Error(w, "Impossible de lister les clusters", http.StatusInternalServerError)
		return
	}

	// 2. Trouver le cluster par son UID
	var clusterToDelete *clusterv1.Cluster
	for i, c := range clusterListK8s.Items {
		if string(c.UID) == id {
			clusterToDelete = &clusterListK8s.Items[i]
			break
		}
	}

	if clusterToDelete == nil {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	// 3. Supprimer le cluster CAPI
	// (Le CAPI controller s'occupera de supprimer le namespace si l'annotation est présente)
	if err := k8sClient.Delete(ctx, clusterToDelete); err != nil {
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
	ctx := context.Background()

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
	var providerList []Provider
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

	// 4. Trier et renvoyer
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

	newProvider := Provider{
		ID:       strings.ToLower(payload.Name),
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
	ctx := context.Background()

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
		http.Error(w, "Provider non trouvé dans le catalogue interne", http.StatusNotFound)
		return
	}

	// 2. Obtenir la commande de suppression (en passant le flag includeCRD)
	deleteCmdArgs := getProviderDeleteCommand(providerInfo.Category, id, includeCRD)
	if deleteCmdArgs == nil {
		http.Error(w, "Catégorie de provider non supprimable ou inconnue", http.StatusBadRequest)
		return
	}
	fullCommandString := "clusterctl " + strings.Join(deleteCmdArgs, " ")

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
							Image:           "smartdeploy/capimgr-backend:clusterctl", // Utilise la même image
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sh", "-c", fullCommandString},
							Env: []corev1.EnvVar{
								{
									Name:  "KUBERNETES_SERVICE_HOST",
									Value: "kubernetes.default.svc.cluster.local",
								},
								{
									Name:  "KUBERNETES_SERVICE_PORT",
									Value: "443",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
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
	ctx := context.Background()

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
	versionToInstall := payload.Version

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
					Category: typeToCategory(data["type"]),
				}
				ok = true
			}
		}
		if !ok {
			http.Error(w, "Provider non trouvé dans le catalogue interne ni dans clusterctl", http.StatusNotFound)
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
							Image:           "smartdeploy/capimgr-backend:clusterctl",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sh", "-c", fullCommandString},
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
								// Injecter les variables d'env du payload (ex: AWS_B64ENCODED_CREDENTIALS)
								for k, v := range payload.Env {
									envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
								}
								return envVars
							}(),
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
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
