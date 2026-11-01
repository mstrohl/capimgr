package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// --- Structures (Models) ---

type Provider struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Version       string `json:"version"`
	RepositoryURL string `json:"repositoryURL,omitempty"`
	Installed     bool   `json:"installed"`
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
	Name          string `json:"name"`
	Category      string `json:"category"`
	Version       string `json:"version"`
	RepositoryURL string `json:"repositoryURL"`
}

// --- Client K8s et Catalogue Mock ---

var (
	k8sClient client.Client

	mockProviders  = make(map[string]Provider)
	providersMutex = &sync.RWMutex{}
	// providerKindMap mappe un ID de provider à l'ApiVersion/Kind qu'il gère
	providerKindMap map[string]map[string]string

	// providerCRDMap mappe un ID de provider au nom de son CRD principal
	providerCRDMap map[string]string
)

func init() {
	initMockProviders()

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

// initMockProviders initialise le catalogue statique des providers
//func initMockProviders() {
//	providersMutex.Lock()
//	mockProviders = map[string]Provider{
//		// Core
//		"cluster-api": {ID: "cluster-api", Name: "Cluster API (Core)", Category: "core", Version: "v1.7.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api"},
//		// Infrastructure
//		"aws":          {ID: "aws", Name: "AWS", Category: "infrastructure", Version: "v2.4.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-aws"},
//		"azure":        {ID: "azure", Name: "Azure", Category: "infrastructure", Version: "v1.11.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-azure"},
//		"gcp":          {ID: "gcp", Name: "Google Cloud", Category: "infrastructure", Version: "v1.7.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-gcp"},
//		"vsphere":      {ID: "vsphere", Name: "vSphere", Category: "infrastructure", Version: "v1.9.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-vsphere"},
//		"openstack":    {ID: "openstack", Name: "OpenStack", Category: "infrastructure", Version: "v0.10.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-openstack"},
//		"proxmox":      {ID: "proxmox", Name: "Proxmox", Category: "infrastructure", Version: "v0.6.0", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-proxmox"},
//		"digitalocean": {ID: "digitalocean", Name: "DigitalOcean", Category: "infrastructure", Version: "v0.9.1", RepositoryURL: "https://github.com/kubernetes-sigs/cluster-api-provider-digitalocean"},
//		"hetzner":      {ID: "hetzner", Name: "Hetzner", Category: "infrastructure", Version: "v1.3.0", RepositoryURL: "https://github.com/syself/cluster-api-provider-hetzner"},
//		// Control Plane
//		"kubeadm": {ID: "kubeadm", Name: "Kubeadm", Category: "controlplane", Version: "v1.7.0", RepositoryURL: "https-::github.com:kubernetes-sigs:cluster-api:tree:main:controlplane:kubeadm"},
//		"talos":   {ID: "talos", Name: "Talos", Category: "controlplane", Version: "v0.6.0", RepositoryURL: "https://github.com/siderolabs/cluster-api-provider-talos"},
//		// Bootstrap
//		"kubeadm-bootstrap": {ID: "kubeadm-bootstrap", Name: "Kubeadm (Bootstrap)", Category: "bootstrap", Version: "v1.7.0", RepositoryURL: "https:://github.com/kubernetes-sigs/cluster-api/tree/main/bootstrap/kubeadm"},
//		"talos-bootstrap":   {ID: "talos-bootstrap", Name: "Talos (Bootstrap)", Category: "bootstrap", Version: "v0.6.0", RepositoryURL: "https://github.com/siderolabs/cluster-api-provider-talos"},
//	}
//	providersMutex.Unlock()
//}

// --- Main Function (Modifiée) ---
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

	log.Println("Démarrage du CAPI Dashboard Backend sur :8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/clusters", handleClusters)
	mux.HandleFunc("/api/clusters/", handleClusterByID)
	mux.HandleFunc("/api/providers", handleProviders)
	mux.HandleFunc("/api/providers/", handleProviderByID)

	handler := corsMiddleware(mux)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Erreur ListenAndServe: %v", err)
	}
}

// --- Handlers (inchangés, ils routent vers la logique) ---

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

// --- Logique "Backend" (Fonctionnelle) ---

// getClusters (MODIFIÉ)
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
			// ** CORRECTION APPLIQUÉE ICI **
			// c.Spec.Topology.Version est un 'string', pas un '*string'
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

// createCluster (MODIFIÉ)
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

// deleteCluster (MODIFIÉ)
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

// getProviders (MODIFIÉ)
func getProviders(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// 1. Lister tous les CRDs installés sur le cluster
	crdListK8s := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := k8sClient.List(ctx, crdListK8s); err != nil {
		log.Printf("Erreur K8s (List CRDs): %v", err)
		http.Error(w, "Erreur lors de la lecture des CRDs", http.StatusInternalServerError)
		return
	}

	// 2. Créer un Set des noms de CRDs pour une recherche rapide
	installedCRDs := make(map[string]bool)
	for _, crd := range crdListK8s.Items {
		installedCRDs[crd.Name] = true
	}

	// 3. Parcourir le catalogue 'mock' et le mettre à jour
	providersMutex.RLock()
	defer providersMutex.RUnlock()

	var providerList []Provider
	for _, p := range mockProviders {
		// Vérifier si le CRD correspondant est dans notre Set
		if crdName, ok := providerCRDMap[p.ID]; ok {
			if _, installed := installedCRDs[crdName]; installed {
				p.Installed = true
			} else {
				p.Installed = false
			}
		}
		providerList = append(providerList, p)
	}

	// 4. Trier et renvoyer
	sort.Slice(providerList, func(i, j int) bool {
		return providerList[i].Name < providerList[j].Name
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerList)
}

// createProvider (Inchangé - utilise les mocks)
func createProvider(w http.ResponseWriter, r *http.Request) {
	var payload CreateProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newProvider := Provider{
		ID:            strings.ToLower(payload.Name),
		Name:          payload.Name,
		Category:      payload.Category,
		Version:       payload.Version,
		RepositoryURL: payload.RepositoryURL,
		Installed:     false,
	}

	providersMutex.Lock()
	mockProviders[newProvider.ID] = newProvider
	providersMutex.Unlock()
	log.Printf("Provider %s ajouté au catalogue mock.", newProvider.ID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newProvider)
}

// deleteProvider (Inchangé - utilise les mocks)
func deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	providersMutex.Lock()
	if _, ok := mockProviders[id]; !ok {
		providersMutex.Unlock()
		http.Error(w, "Provider non trouvé", http.StatusNotFound)
		return
	}
	delete(mockProviders, id)
	providersMutex.Unlock()
	log.Printf("Provider %s supprimé du catalogue mock.", id)

	w.WriteHeader(http.StatusNoContent)
}

// installProvider (CORRIGÉ)
// Crée un Job Kubernetes pour exécuter 'clusterctl init' pour un provider donné.
// La correction s'assure que le Job utilise le token du Service Account spécifié.
func installProvider(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()

	// 1. Récupérer les infos du provider depuis le catalogue
	providersMutex.RLock()
	provider, ok := mockProviders[id]
	providersMutex.RUnlock()
	if !ok {
		http.Error(w, "Provider non trouvé dans le catalogue", http.StatusNotFound)
		return
	}

	// 2. Obtenir la commande d'installation pour clusterctl
	installCmdArgs := getProviderInstallCommand(provider)
	if installCmdArgs == nil {
		http.Error(w, "Catégorie de provider non installable ou inconnue", http.StatusBadRequest)
		return
	}

	// NOUVEAU: Construire la commande complète pour le shell
	// On combine "clusterctl" avec ses arguments (ex: "clusterctl init --core cluster-api:v1.7.0")
	fullCommandString := "clusterctl " + strings.Join(installCmdArgs, " ")

	// 3. Définir le Job Kubernetes
	jobName := fmt.Sprintf("install-%s-%d", id, time.Now().Unix())

	// Pointeur vers 'true' pour la configuration du pod.
	// C'est la manière idiomatique de définir un *bool en Go pour les specs K8s.
	automountToken := true

	// Le nombre de tentatives avant de marquer le job comme échoué. 0 signifie aucune nouvelle tentative.
	backoffLimit := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			// Il est recommandé de lancer ces jobs dans un namespace dédié à l'outillage.
			Namespace: "capi-dashboard-system",
		},
		Spec: batchv1.JobSpec{
			// Ne pas réessayer le job en cas d'échec, pour faciliter le débogage.
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					// Assigner le Service Account qui a les permissions nécessaires (ex: cluster-admin).
					ServiceAccountName: "clusterctl-installer-sa",

					// --- CORRECTION 1: Forcer le montage du token ---
					// Garantit que le token du Service Account est monté dans le pod,
					// même si le Service Account lui-même est configuré pour ne pas le faire par défaut.
					AutomountServiceAccountToken: &automountToken,

					Containers: []corev1.Container{
						{
							Name:  "installer",
							Image: "smartdeploy/capimgr-backend:clusterctl",

							// --- CORRECTION 2: Utiliser 'Args' au lieu de 'Command' ---
							// L'image 'clusterctl' a un ENTRYPOINT qui est "/clusterctl".
							// Pour passer des paramètres à cet entrypoint, il faut utiliser le champ 'Args'.
							// Le champ 'Command' remplacerait l'entrypoint, ce qui n'est pas ce que nous voulons.
							//Args: installCmdArgs,
							Command: []string{"/bin/sh", "-c", fullCommandString},

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
							// Le montage du volume pour la configuration de clusterctl reste pertinent.
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "clusterctl-config-volume",
									MountPath: "/root/.cluster-api", // clusterctl cherche sa config ici
								},
							},
						},
					},
					// Le pod doit être arrêté après l'échec ou le succès du job.
					RestartPolicy: corev1.RestartPolicyOnFailure,
					// Définir le volume qui montera le ConfigMap de configuration de clusterctl.
					Volumes: []corev1.Volume{
						{
							Name: "clusterctl-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "clusterctl-config", // Le nom du ConfigMap à créer
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// 4. Créer le Job sur le cluster Kubernetes
	if err := k8sClient.Create(ctx, job); err != nil {
		log.Printf("Erreur K8s (Create Job): %v", err)
		http.Error(w, "Erreur lors de la création du Job d'installation", http.StatusInternalServerError)
		return
	}

	log.Printf("Job %s créé pour installer le provider %s", jobName, id)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "Installation du provider initiée.", "jobName": jobName})
}

// getProviderInstallCommand est conservé tel quel, car il est déjà correct.
func getProviderInstallCommand(provider Provider) []string {
	var flag string
	switch provider.Category {
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
	// La commande correcte pour 'clusterctl init' est `init {flag} {provider_name}:{version}`
	// Ici, nous laissons clusterctl utiliser la version par défaut définie dans son ConfigMap.
	return []string{"init", flag, fmt.Sprintf("%s:%s", provider.ID, provider.Version)}
}
