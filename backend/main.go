package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- Structures (Models) ---

type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// NOUVEAUX CHAMPS
	Category      string `json:"category"` // 'infrastructure', 'controlplane', 'bootstrap', 'core'
	Version       string `json:"version"`
	RepositoryURL string `json:"repositoryURL,omitempty"`
}

type Cluster struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	K8sVersion   string `json:"k8sVersion"`
	Nodes        int    `json:"nodes"`
	ControlPlane string `json:"controlPlane"` // Ex: "3 x m5.xlarge"
	Created      string `json:"created"`
	// NOUVEAUX CHAMPS
	Namespace              string `json:"namespace"`
	InfrastructureProvider string `json:"infrastructureProvider"`
	ControlPlaneProvider   string `json:"controlPlaneProvider"`
	BootstrapProvider      string `json:"bootstrapProvider"`
}

// --- Données pour la création (Payload JSON) ---

type CreateClusterPayload struct {
	Name              string `json:"name"`
	K8sVersion        string `json:"k8sVersion"`
	ControlPlaneNodes int    `json:"controlPlaneNodes"`
	ControlPlaneType  string `json:"controlPlaneType"`
	WorkerNodes       int    `json:"workerNodes"`
	WorkerType        string `json:"workerType"`
	// NOUVEAUX CHAMPS
	InfrastructureProvider string `json:"infrastructureProvider"`
	ControlPlaneProvider   string `json:"controlPlaneProvider"`
	BootstrapProvider      string `json:"bootstrapProvider"`
}

type CreateProviderPayload struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Version       string `json:"version"`
	RepositoryURL string `json:"repositoryURL"`
}

// --- Base de données simulée (In-Memory) ---
var (
	mockProviders  = make(map[string]Provider)
	mockClusters   = make(map[string]Cluster)
	providersMutex = &sync.RWMutex{}
	clustersMutex  = &sync.RWMutex{}
)

func init() {
	rand.Seed(time.Now().UnixNano())
	initMockData()
}

func initMockData() {
	providers := []Provider{
		// Infrastructure Providers
		{ID: "aws", Name: "Amazon Web Services", Category: "infrastructure", Version: "v2.4.1", RepositoryURL: "github.com/kubernetes-sigs/cluster-api-provider-aws"},
		{ID: "azure", Name: "Microsoft Azure", Category: "infrastructure", Version: "v1.11.0", RepositoryURL: "github.com/kubernetes-sigs/cluster-api-provider-azure"},
		{ID: "gcp", Name: "Google Cloud Platform", Category: "infrastructure", Version: "v1.8.0", RepositoryURL: "github.com/kubernetes-sigs/cluster-api-provider-gcp"},
		{ID: "vsphere", Name: "VMware vSphere", Category: "infrastructure", Version: "v1.10.2", RepositoryURL: "github.com/kubernetes-sigs/cluster-api-provider-vsphere"},

		// ControlPlane Providers
		{ID: "kubeadm", Name: "Kubeadm Control Plane", Category: "controlplane", Version: "v1.7.0", RepositoryURL: "github.com/kubernetes-sigs/cluster-api"},
		{ID: "talos", Name: "Talos Control Plane", Category: "controlplane", Version: "v0.6.0", RepositoryURL: "github.com/siderolabs/cluster-api-provider-talos"},

		// Bootstrap Providers
		{ID: "kubeadm-bootstrap", Name: "Kubeadm Bootstrap", Category: "bootstrap", Version: "v1.7.0", RepositoryURL: "github.com/kubernetes-sigs/cluster-api"},
		{ID: "talos-bootstrap", Name: "Talos Bootstrap", Category: "bootstrap", Version: "v0.6.0", RepositoryURL: "github.com/siderolabs/cluster-api-provider-talos"},

		// Core
		{ID: "cluster-api", Name: "Cluster API (Core)", Category: "core", Version: "v1.7.0", RepositoryURL: "github.com/kubernetes-sigs/cluster-api"},
	}
	providersMutex.Lock()
	for _, p := range providers {
		mockProviders[p.ID] = p
	}
	providersMutex.Unlock()

	clusters := []Cluster{
		{
			ID: "prod-eu-1", Name: "prod-cluster-eu-west-1", Status: "En cours", K8sVersion: "v1.29.3", Nodes: 52, ControlPlane: "3 x m5.xlarge", Created: "Il y a 2 mois",
			Namespace: "capi-prod-cluster-eu-west-1", InfrastructureProvider: "aws", ControlPlaneProvider: "kubeadm", BootstrapProvider: "kubeadm-bootstrap",
		},
		{
			ID: "staging-us-1", Name: "staging-cluster-us-east-1", Status: "Provisionnement", K8sVersion: "v1.30.0", Nodes: 8, ControlPlane: "3 x Standard_D4s_v3", Created: "Il y a 2 heures",
			Namespace: "capi-staging-cluster-us-east-1", InfrastructureProvider: "azure", ControlPlaneProvider: "talos", BootstrapProvider: "talos-bootstrap",
		},
	}
	clustersMutex.Lock()
	for _, c := range clusters {
		mockClusters[c.ID] = c
	}
	clustersMutex.Unlock()
}

// --- Main Function ---
func main() {
	mux := http.NewServeMux()

	// --- Routes API ---
	mux.HandleFunc("/api/clusters", handleClusters)     // GET (all), POST (create)
	mux.HandleFunc("/api/clusters/", handleClusterByID) // GET (one), DELETE (one)
	mux.HandleFunc("/api/providers", handleProviders)   // GET (all), POST (create)

	// Middleware pour CORS
	handler := corsMiddleware(mux)

	log.Println("Backend CAPI (v2) démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

// --- Handlers ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Ou une origine spécifique
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Handlers /api/clusters ---
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
	case "GET":
		getCluster(w, r, id)
	case "DELETE":
		deleteCluster(w, r, id)
	default:
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
	}
}

// --- Handlers /api/providers ---
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

// --- Logique "Backend" (CRUD) ---

func getClusters(w http.ResponseWriter, r *http.Request) {
	clustersMutex.RLock()
	defer clustersMutex.RUnlock()

	var clusterList []Cluster
	for _, c := range mockClusters {
		clusterList = append(clusterList, c)
	}

	// Tri par date de création (simulé)
	sort.Slice(clusterList, func(i, j int) bool {
		return clusterList[i].Created < clusterList[j].Created // Simulation simple
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusterList)
}

func getCluster(w http.ResponseWriter, r *http.Request, id string) {
	clustersMutex.RLock()
	defer clustersMutex.RUnlock()

	cluster, ok := mockClusters[id]
	if !ok {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster)
}

func createCluster(w http.ResponseWriter, r *http.Request) {
	var payload CreateClusterPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Payload JSON invalide", http.StatusBadRequest)
		return
	}

	// --- VRAIE IMPLÉMENTATION ---
	// C'est ici que vous utiliseriez client-go.
	// 1. Créer le Namespace: "capi-" + payload.Name
	// 2. Créer l'objet Cluster (avec le namespace)
	// 3. Créer l'objet InfraCluster (ex: AWSCluster)
	// 4. Créer l'objet ControlPlane (ex: TalosControlPlane)
	// 5. Créer les MachineDeployments
	// --- (Simulation) ---

	clustersMutex.Lock()
	defer clustersMutex.Unlock()

	newID := payload.Name
	if _, exists := mockClusters[newID]; exists {
		newID = payload.Name + "-" + time.Now().Format("060102")
	}

	newCluster := Cluster{
		ID:           newID,
		Name:         payload.Name,
		Status:       "Provisionnement",
		K8sVersion:   payload.K8sVersion,
		Nodes:        payload.WorkerNodes,
		ControlPlane: payload.ControlPlaneType, // Simplifié
		Created:      "À l'instant",
		// NOUVEAUX CHAMPS
		Namespace:              "capi-" + payload.Name, // Logique de namespace
		InfrastructureProvider: payload.InfrastructureProvider,
		ControlPlaneProvider:   payload.ControlPlaneProvider,
		BootstrapProvider:      payload.BootstrapProvider,
	}

	mockClusters[newID] = newCluster

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newCluster)
}

func deleteCluster(w http.ResponseWriter, r *http.Request, id string) {
	// --- VRAIE IMPLÉMENTATION ---
	// client.ClusterV1beta1().Clusters(namespace).Delete(...)
	// --- (Simulation) ---
	clustersMutex.Lock()
	defer clustersMutex.Unlock()

	if _, ok := mockClusters[id]; !ok {
		http.Error(w, "Cluster non trouvé", http.StatusNotFound)
		return
	}
	delete(mockClusters, id)

	w.WriteHeader(http.StatusNoContent)
}

func getProviders(w http.ResponseWriter, r *http.Request) {
	providersMutex.RLock()
	defer providersMutex.RUnlock()

	var providerList []Provider
	for _, p := range mockProviders {
		providerList = append(providerList, p)
	}

	// Tri par nom
	sort.Slice(providerList, func(i, j int) bool {
		return providerList[i].Name < providerList[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerList)
}

// createProvider - NOUVELLE FONCTION
func createProvider(w http.ResponseWriter, r *http.Request) {
	var payload CreateProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Payload JSON invalide", http.StatusBadRequest)
		return
	}

	if payload.Name == "" || payload.Category == "" || payload.Version == "" {
		http.Error(w, "Champs manquants", http.StatusBadRequest)
		return
	}

	// --- VRAIE IMPLÉMENTATION ---
	// Ici, vous pourriez essayer de valider l'URL du repo,
	// ou même de lancer 'clusterctl init' pour ce provider.
	// --- (Simulation) ---

	providersMutex.Lock()
	defer providersMutex.Unlock()

	newID := strings.ToLower(strings.ReplaceAll(payload.Name, " ", "-"))
	if _, exists := mockProviders[newID]; exists {
		newID = newID + "-" + time.Now().Format("06")
	}

	newProvider := Provider{
		ID:            newID,
		Name:          payload.Name,
		Category:      payload.Category,
		Version:       payload.Version,
		RepositoryURL: payload.RepositoryURL,
	}

	mockProviders[newID] = newProvider

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newProvider)
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	// --- VRAIE IMPLÉMENTATION ---
	// client.ClusterV1beta1().Providers().Delete(...)
	// --- (Simulation) ---
	http.Error(w, "Fonction non implémentée", http.StatusNotImplemented)
}
