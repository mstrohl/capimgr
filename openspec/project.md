# Cluster API Manager (capimgr) - Project Overview

## 1. Description & Vision
**capimgr** est une console d'administration et de gestion web dédiée à **Kubernetes Cluster API (CAPI)**.
Elle permet aux équipes d'infrastructure et plateformes de :
1. **Gérer l'écosystème de providers CAPI à la demande** : installer, mettre à jour, configurer et désinstaller des providers d'infrastructure (AWS, Azure, GCP, vSphere, Proxmox, OpenStack, Hetzner, etc.), de control plane (Kubeadm, Talos, RKE2, etc.) et de bootstrap.
2. **Gérer le cycle de vie complet des clusters enfants (workload clusters)** : création, supervision de l'état/conditions, passage à l'échelle (scale-up / scale-down des nœuds workers et control plane), montée de version Kubernetes, téléchargement du `kubeconfig` et suppression ordonnée.
3. **Générer dynamiquement des formulaires à partir des CRDs des providers** : exploiter les schémas OpenAPI v3 intégrés dans les `CustomResourceDefinition` (CRD) installées dans le cluster de management pour présenter des formulaires interactifs, typés et validés sans nécessiter de développement frontend spécifique par provider.

---

## 2. Pile Technologique (Tech Stack)

### Backend
- **Langage** : Go (1.22+)
- **Framework & K8s SDK** :
  - `sigs.k8s.io/controller-runtime` (Client K8s typé & dynamique)
  - `sigs.k8s.io/cluster-api/api/v1beta1` (Définitions d'API CAPI)
  - `k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1` (Introspection CRD & OpenAPIV3Schema)
  - `k8s.io/client-go` (Gestion des secrets, kubeconfig, jobs)
- **Outil d'automatisation CAPI** : `clusterctl` CLI intégré dans l'image de conteneur d'exécution et orchestré via des K8s `Batch/v1 Jobs`.
- **API Protocol** : REST HTTP JSON avec spécification OpenAPI 3.0 (`backend/openapi.yaml`).

### Frontend
- **Langage & Framework** : React 18, Vite 7
- **Styling** : Tailwind CSS
- **Moteur de formulaire dynamique** : Intégration de schémas JSON Schema / OpenAPI v3 pour la génération réactive de formulaires (composants imbriqués avec validation de types, valeurs par défaut, énumérations et bascule Formulaire <-> YAML).
- **Communication** : Client API REST asynchrone avec gestion d'état et notifications toast.

### Déploiement & Infrastructure K8s
- **Cluster cible** : Cluster de management Kubernetes hébergeant CAPI.
- **Namespaces dédiés** :
  - `capi-dashboard-system` : hébergement des Pods backend, frontend et des Jobs `clusterctl`.
  - `capi-providers-system` : installation standard des contrôleurs de providers CAPI.
- **RBAC** : ServiceAccount avec droits ciblés pour l'API backend et SA privilégié (`clusterctl-installer-sa`) pour les Jobs d'installation.

---

## 3. Architecture Globale

```
                      +---------------------------------------+
                      |             Utilisateur               |
                      +---------------------------------------+
                                          |
                                          v (HTTP/S)
                      +---------------------------------------+
                      |         Frontend (React / Vite)       |
                      | - Dashboard clusters & providers      |
                      | - Moteur de formulaire dynamique CRD  |
                      | - Visualiseur de statut & logs Jobs   |
                      +---------------------------------------+
                                          |
                                          v (REST API :8080)
                      +---------------------------------------+
                      |          Backend (Go / Mux)           |
                      | - Gestionnaire Clusters & Kubeconfig  |
                      | - Extracteur OpenAPIV3Schema des CRDs |
                      | - Orchestrateur Jobs clusterctl       |
                      +---------------------------------------+
                             |                     |
                             v                     v
              +--------------------------+  +-------------------------------+
              | Kubernetes API Server    |  | K8s Jobs (clusterctl)         |
              | - CustomResourceDefs     |  | - clusterctl init             |
              | - Clusters (cluster.x-k8s) | - clusterctl delete           |
              | - Secrets (kubeconfig)   |  | - Logs d'exécution            |
              +--------------------------+  +-------------------------------+
```

---

## 4. Conventions du Projet
- **Spécifications** : Formalisées sous `openspec/` selon les normes OpenSpec (SDD) : exigences RFC 2119 (`MUST`, `SHOULD`, `SHALL`) et scénarios `GIVEN / WHEN / THEN`.
- **API Design** : Tout nouvel endpoint backend doit être documenté dans `backend/openapi.yaml`.
- **Extensibilité des providers** : Aucun provider ne doit nécessiter de code durci (hardcoded) dans le frontend ; toute interaction avec les CRDs s'appuie sur la découverte dynamique K8s.
