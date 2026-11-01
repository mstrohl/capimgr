# Liste des Tâches d'Implémentation (Tasks) : capimgr

## Phase 1 : Moteur d'Introspection des CRDs & Schémas JSON (Backend)
- [x] 1.1 Créer le service `crd_service.go` pour lister les CRDs via le client `apiextensionsv1`.
- [x] 1.2 Implémenter l'extracteur et le nettoyeur de schéma OpenAPI v3 (`spec.versions[].schema.openAPIV3Schema`) pour ne retenir que les propriétés configurables sous `spec`.
- [x] 1.3 Exposer les routes HTTP :
  - `GET /api/crds` (liste des CRDs avec filtrage par provider/groupe d'API).
  - `GET /api/crds/{name}/schema` (schéma JSON prêt à l'emploi avec métadonnées).
- [x] 1.4 Ajouter des tests unitaires pour la sérialisation et le nettoyage du schéma JSON.

## Phase 2 : Gestion Dynamique et Suivi des Providers (Backend & K8s)
- [x] 2.1 Mettre à jour `provider_service.go` pour corréler dynamiquement les providers avec l'ensemble des CRDs associées (et non une seule CRD codée en dur).
- [x] 2.2 Implémenter le service de suivi de Jobs (`job_service.go`) :
  - `GET /api/jobs/{id}/status` pour interroger l'état du Job K8s (`batchv1.Job`).
  - `GET /api/jobs/{id}/logs` pour lire les logs du Pod exécutant `clusterctl`.
- [x] 2.3 Mettre à jour `rbac.yaml` pour autoriser la lecture des Pods et des Logs dans `capi-dashboard-system`.
- [x] 2.4 Tester l'installation asynchrone d'un provider avec remontée des logs.

## Phase 3 : Gestion Complète du Cycle de Vie des Clusters Enfants (Backend)
- [ ] 3.1 Étendre le modèle `Cluster` pour intégrer les conditions réelles CAPI (`ControlPlaneReady`, `InfrastructureReady`, etc.).
- [ ] 3.2 Implémenter l'endpoint `GET /api/clusters/{id}` renvoyant la structure détaillée.
- [ ] 3.3 Implémenter l'endpoint `GET /api/clusters/{id}/kubeconfig` :
  - Lecture du Secret `<cluster-name>-kubeconfig` dans le namespace du cluster.
  - Extraction de la clé `value` et encodage/renvoi direct au format YAML/texte.
- [ ] 3.4 Implémenter l'endpoint `GET /api/clusters/{id}/machines` pour lister les `Machine` et `MachineDeployment` associées.
- [ ] 3.5 Implémenter l'endpoint `PATCH /api/clusters/{id}/scale` pour ajuster le nombre de répliques des pools de nœuds workers.

## Phase 4 : Moteur de Formulaires Dynamiques pour CRDs (Frontend)
- [ ] 4.1 Concevoir le composant réutilisable `CRDFormRenderer` :
  - Rendu des types primitifs (`string`, `number`, `boolean`).
  - Rendu des menus déroulants pour les champs énumérés (`enum`).
  - Rendu des sections imbriquées pour les `object`.
  - Rendu des tableaux de puces ou de sous-formulaires pour les `array`.
- [ ] 4.2 Intégrer un validateur client basé sur les contraintes du schéma (`required`, `pattern`, `minimum`, `maximum`).
- [ ] 4.3 Développer la vue miroir bidirectionnelle (Formulaire visuel <---> Éditeur YAML) avec synchronisation en temps réel.
- [ ] 4.4 Intégrer la sélection de CRD lors de la création d'infrastructure ou de personnalisation de cluster.

## Phase 5 : Refonte et Modernisation de l'Interface UI (Frontend)
- [ ] 5.1 Restructurer la navigation et ajouter un visualiseur de détails de cluster à onglets :
  - Onglet 1 : Vue d'ensemble & Conditions CAPI.
  - Onglet 2 : Liste des Nœuds & Machines.
  - Onglet 3 : Accès & Kubeconfig (copie en 1 clic et téléchargement du fichier).
  - Onglet 4 : Gestion des Répliques & Échelle (Scale).
- [ ] 5.2 Mettre à jour le Catalogue des Providers :
  - Modal de sélection de version disponible pour l'installation.
  - Fenêtre modale de logs en direct lors de l'exécution du Job `clusterctl`.
  - Liste des CRDs actives associées à chaque provider installé.
- [ ] 5.3 Moderniser les notifications (toasts animés pour les opérations asynchrones).

## Phase 6 : Documentation, OpenAPI & Validation
- [ ] 6.1 Mettre à jour `backend/openapi.yaml` avec l'ensemble des nouveaux endpoints et schémas de payload.
- [ ] 6.2 Valider le déploiement sur cluster Kubernetes avec les manifestes `manifests.yaml` et `rbac.yaml`.
- [ ] 6.3 Archiver les spécifications OpenSpec dans `openspec/specs/` une fois validées.
