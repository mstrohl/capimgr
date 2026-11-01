# Spécification : Cycle de Vie des Clusters Enfants (Cluster Lifecycle)

## But (Purpose)
Cette spécification définit les exigences et le comportement du système pour le provisionnement, l'inspection de l'état de santé, le redimensionnement et la suppression des clusters Kubernetes enfants gérés par Cluster API.

## Exigences (Requirements)

### Requirement: Liste et État Global des Clusters Enfants
Le système SHALL lister tous les objets `Cluster` du groupe d'API `cluster.x-k8s.io` sur l'ensemble des namespaces avec leur phase et leurs métriques de base.

#### Scenario: Lister tous les clusters
- **GIVEN** un ou plusieurs clusters CAPI existants dans le cluster de management
- **WHEN** un client appelle `GET /api/clusters`
- **THEN** le système retourne un tableau d'objets `Cluster` incluant l'identifiant (UID), le nom, le namespace, la version Kubernetes, la phase CAPI (`Provisioning`, `Provisioned`, `Deleting`, etc.), le nombre de nœuds réels et les types de providers associés.

---

### Requirement: Consultation Détaillée et Conditions de Santé
Le système SHALL fournir une vue détaillée d'un cluster incluant ses conditions de santé (`ControlPlaneReady`, `InfrastructureReady`, etc.) et la liste des machines.

#### Scenario: Obtenir les détails d'un cluster
- **GIVEN** un cluster enfant identifié par son UID
- **WHEN** un client appelle `GET /api/clusters/{id}`
- **THEN** le système retourne l'objet complet avec :
  - Les conditions CAPI (type, statut `True`/`False`/`Unknown`, raison, message, dernière transition)
  - Les références vers le Control Plane et l'Infrastructure
  - La liste des MachineDeployments associés.

#### Scenario: Cluster non trouvé
- **GIVEN** un identifiant UID inexistant
- **WHEN** un client appelle `GET /api/clusters/{id}`
- **THEN** le système retourne un code HTTP `404 Not Found`.

---

### Requirement: Provisionnement de Cluster Enfant
Le système SHALL permettre la création d'un cluster enfant dans un namespace dédié en initialisant les ressources CAPI nécessaires.

#### Scenario: Création réussie d'un cluster
- **GIVEN** des providers d'infrastructure et de control plane installés
- **WHEN** un client envoie une requête `POST /api/clusters` avec le nom, la version Kubernetes, les références de providers et les spécifications de nœuds
- **THEN** le système crée le namespace `capi-{nom}` s'il n'existe pas
- **THEN** le système crée la ressource `Cluster` CAPI associée
- **THEN** le système répond avec le code HTTP `201 Created` et l'objet cluster initialisé.

---

### Requirement: Redimensionnement des Nœuds (Scaling)
Le système SHALL permettre d'augmenter ou réduire le nombre de répliques des pools de nœuds workers d'un cluster.

#### Scenario: Modification du nombre de workers
- **GIVEN** un cluster déployé avec un `MachineDeployment` actif
- **WHEN** un client appelle `PATCH /api/clusters/{id}/scale` avec `{"workerReplicas": 5}`
- **THEN** le système met à jour le champ `spec.replicas` du `MachineDeployment` correspondant
- **THEN** le système retourne le code HTTP `200 OK` avec les répliques mises à jour.

---

### Requirement: Suppression Ordonnée de Cluster Enfant
Le système SHALL permettre la suppression d'un cluster en déléguant le nettoyage des machines physiques/virtuelles au contrôleur CAPI sous-jacent.

#### Scenario: Suppression déclenchée
- **GIVEN** un cluster actif
- **WHEN** un client envoie une requête `DELETE /api/clusters/{id}`
- **THEN** le système supprime l'objet `Cluster` K8s
- **THEN** le contrôleur CAPI initie la suppression en cascade des machines et de l'infrastructure
- **THEN** le système répond avec le code HTTP `204 No Content`.
