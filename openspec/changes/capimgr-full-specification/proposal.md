# Proposition : Plateforme Complète Cluster API Manager (capimgr)

## 1. Contexte & Problématique

Le projet **capimgr** vise à fournir une interface graphique intuitive et complète pour opérer **Cluster API (CAPI)**.

Dans son état initial :
1. **Providers figés et non découvrables** : Le catalogue est en partie codé en dur dans le backend (`providerCatalog`, `providerCRDMap`, `providerKindMap`). L'ajout d'un nouveau provider nécessite des modifications dans le code source Go et le code frontend.
2. **Cycle de vie incomplet des clusters enfants** : La création actuelle génère un cluster CAPI minimaliste reposant sur une hypothétique `ClusterClass: default`. Les opérations essentielles (suivi des conditions, récupération du `kubeconfig` pour se connecter au cluster enfant, redimensionnement des nœuds, détails des machines, suppression avec dépendances) ne sont pas implémentées.
3. **Absence de configuration dynamique des CRDs** : Chaque provider CAPI (AWS, Azure, GCP, vSphere, Proxmox, Hetzner, Talos, etc.) introduit ses propres Custom Resource Definitions (CRDs) avec des paramètres spécifiques (ex: `AWSCluster.spec.region`, `VSphereCluster.spec.server`, `ProxmoxMachineTemplate.spec...`). Actuellement, l'interface propose un formulaire figé avec des options spécifiques à AWS, ce qui empêche d'utiliser pleinement les autres providers.

## 2. Objectifs du Projet

- **P1 - Découverte et Gestion Dynamique des Providers** :
  - Support de l'ensemble des providers supportés par `clusterctl` ainsi que des dépôts personnalisés via ConfigMap.
  - Possibilité d'installer, mettre à jour (choix de la version), inspecter et désinstaller n'importe quel provider.
  - Détection automatique et temps réel des CRDs introduites par chaque provider.
- **P2 - Gestion Complète des Clusters Enfants (Workload Clusters)** :
  - Déploiement de clusters enfants soit via `ClusterClass` & `Topology`, soit via génération de manifestes multi-ressources.
  - Tableau de bord de santé du cluster : statut des phases, conditions détaillées (`ControlPlaneReady`, `InfrastructureReady`), inventaire des machines / `MachineDeployments`.
  - Récupération sécurisée du fichier `kubeconfig` du cluster enfant pour permettre l'administration directe.
  - Mise à l'échelle (scale-up / scale-down) des nœuds et modification de configuration.
- **P3 - Moteur de Formulaires Dynamiques pour CRDs** :
  - Endpoint backend extrayant le schéma OpenAPI v3 (`openAPIV3Schema`) de n'importe quelle CRD installée.
  - Filtrage intelligent des champs système/lecture seule (status, metadata interne) pour se concentrer sur les champs configurables (`spec`).
  - Composant frontend réactif générant des formulaires avec validation en temps réel, valeurs par défaut, énumérations, infobulles descriptives et bascule interactive avec un éditeur YAML.

## 3. Périmètre (Scope)

### Dans le périmètre (In Scope)
- Extension du backend Go pour introspecter l'API `apiextensions.k8s.io/v1` et servir les schémas JSON Schema.
- Nouveaux endpoints pour :
  - `/api/crds` et `/api/crds/{name}/schema`
  - `/api/clusters/{id}/kubeconfig`
  - `/api/clusters/{id}/nodes` et `/api/clusters/{id}/scale`
  - `/api/jobs/{id}/status` et `/api/jobs/{id}/logs` (suivi de l'installation des providers)
- Refactorisation du frontend React pour intégrer un visualiseur de clusters avec onglets (Aperçu, Nœuds, Configuration CRD, Kubeconfig, Événements).
- Composant dynamique de formulaire de CRD permettant d'instancier ou modifier une ressource de provider.
- Documentation OpenAPI 3.0 exhaustive.

### Hors périmètre (Out of Scope pour cette proposition)
- Gestion multi-tenancy avec authentification OIDC/Keycloak complète (sera traitée dans une évolution ultérieure ; l'accès actuel s'appuie sur le RBAC du cluster).
- Sauvegarde / restauration automatique via Velero (peut être intégré ultérieurement).

## 4. Bénéfices Attendus
- **Autonomie totale** : Déployer n'importe quel nouveau provider CAPI sans retoucher une seule ligne de code frontend ou backend.
- **Expérience utilisateur fluide** : Remplacer l'édition manuelle de fichiers YAML complexes par des formulaires guidés et validés.
- **Observabilité des clusters enfants** : Suivre l'avancement du provisionnement en temps réel depuis l'UI sans recourir à `kubectl`.
