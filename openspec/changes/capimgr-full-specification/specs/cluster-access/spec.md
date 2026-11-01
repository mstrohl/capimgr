# Spécification : Accès et Connectivité aux Clusters Enfants (Cluster Access)

## But (Purpose)
Cette spécification définit les exigences et le comportement du système pour l'accès aux clusters enfants déployés, notamment l'extraction sécurisée du fichier `kubeconfig`, la visualisation des machines et la gestion des certificats d'accès.

## Exigences (Requirements)

### Requirement: Récupération Sécurisée du Kubeconfig Enfant
Le système SHALL permettre l'extraction du secret contenant le fichier `kubeconfig` généré automatiquement par Cluster API pour chaque cluster enfant prêt.

#### Scenario: Kubeconfig disponible
- **GIVEN** un cluster enfant ayant terminé l'initialisation de son control plane
- **WHEN** un client appelle `GET /api/clusters/{id}/kubeconfig`
- **THEN** le système recherche le Secret nommé `{cluster-name}-kubeconfig` dans le namespace du cluster
- **THEN** le système extrait la clé `value` contenant la configuration `kubeconfig`
- **THEN** le système retourne un payload JSON avec le contenu textuel brut du kubeconfig ainsi que les métadonnées (nom de cluster, endpoint du serveur API K8s)
- **THEN** l'en-tête de réponse permet également un téléchargement direct sous forme de fichier `.kubeconfig`.

#### Scenario: Cluster en cours de provisionnement (Control plane non prêt)
- **GIVEN** un cluster enfant dont le secret kubeconfig n'a pas encore été généré par CAPI
- **WHEN** un client appelle `GET /api/clusters/{id}/kubeconfig`
- **THEN** le système retourne un code HTTP `409 Conflict` ou `404 Not Found` avec un message explicatif : `"Le kubeconfig n'est pas encore disponible : le control plane est en cours d'initialisation."`.

---

### Requirement: Inventaire Détaillé des Machines du Cluster
Le système SHALL fournir l'inventaire complet des instances physiques ou virtuelles (`Machine` et `MachineDeployment`) rattachées au cluster enfant.

#### Scenario: Lister les machines du cluster
- **GIVEN** un cluster enfant avec des machines déployées
- **WHEN** un client appelle `GET /api/clusters/{id}/machines`
- **THEN** le système liste toutes les ressources `Machine` CAPI associées à ce cluster
- **THEN** chaque élément contient le nom, la phase (`Running`, `Provisioning`, `Failed`), la version de Kubernetes, l'adresse IP interne/externe, le rôle (`control-plane` ou `worker`), et la date de démarrage.

---

### Requirement: Téléchargement et Copie en 1 Clic (UI)
L'interface utilisateur SHALL proposer des actions ergonomiques pour exploiter immédiatement le `kubeconfig`.

#### Scenario: Copie dans le presse-papier
- **GIVEN** l'utilisateur consultant l'onglet Kubeconfig d'un cluster
- **WHEN** l'utilisateur clique sur `"Copier dans le presse-papier"`
- **THEN** le texte complet du kubeconfig est copié et une notification de confirmation s'affiche.

#### Scenario: Téléchargement de fichier
- **GIVEN** l'utilisateur consultant l'onglet Kubeconfig
- **WHEN** l'utilisateur clique sur `"Télécharger le fichier kubeconfig"`
- **THEN** le navigateur déclenche le téléchargement d'un fichier nommé `{cluster-name}.kubeconfig`.
