# Spécification : Gestion des Fournisseurs CAPI (Provider Management)

## But (Purpose)
Cette spécification définit les exigences et le comportement du système pour la découverte, l'installation, la supervision et la suppression dynamique des providers Cluster API (Infrastructure, Control Plane, Bootstrap et Core).

## Exigences (Requirements)

### Requirement: Découverte Dynamique des Providers
Le système SHALL agréger et retourner la liste complète des providers disponibles dans le catalogue Cluster API ainsi que les providers configurés dans les dépôts locaux ou distants.

#### Scenario: Récupération de la liste des providers
- **GIVEN** un cluster de management opérationnel
- **WHEN** un client appelle `GET /api/providers`
- **THEN** le système retourne un tableau JSON contenant chaque provider avec son identifiant, son nom convivial, sa catégorie (`infrastructure`, `controlplane`, `bootstrap`, `core`), son statut d'installation (`installed: true|false`), et sa version par défaut.

#### Scenario: Détection dynamique de l'état d'installation
- **GIVEN** un provider dont les CRDs correspondantes ont été enregistrées dans le cluster
- **WHEN** le système évalue l'état d'installation
- **THEN** le champ `installed` du provider est positionné à `true` et la liste des CRDs actives associées est rattachée.

---

### Requirement: Installation Asynchrone de Provider avec Versionnage
Le système SHALL permettre l'installation d'un provider via un Job Kubernetes asynchrone en exécutant `clusterctl init`, en permettant à l'utilisateur de spécifier ou non une version.

#### Scenario: Installation avec version par défaut
- **GIVEN** un provider disponible mais non installé
- **WHEN** un client appelle `POST /api/providers/{id}/install` avec un corps vide
- **THEN** le système crée un Job Kubernetes `install-{id}-{timestamp}` dans le namespace `capi-dashboard-system` exécutant `clusterctl init` avec la version recommandée
- **THEN** le système répond avec le code HTTP `202 Accepted` et le nom du Job.

#### Scenario: Installation avec version explicite
- **GIVEN** un utilisateur souhaitant installer une version spécifique (ex: `v2.4.0`)
- **WHEN** un client appelle `POST /api/providers/{id}/install` avec le payload `{"version": "v2.4.0"}`
- **THEN** le système lance le Job avec l'argument `{id}:v2.4.0`.

---

### Requirement: Suivi en Temps Réel de l'Installation
Le système SHALL exposer l'état et les journaux (logs) du Job d'installation pour informer l'utilisateur de la progression.

#### Scenario: Consultation de l'état du Job
- **GIVEN** un Job d'installation en cours ou terminé
- **WHEN** un client appelle `GET /api/jobs/{id}/status`
- **THEN** le système retourne l'état (`Pending`, `Running`, `Succeeded`, `Failed`) et l'heure de début/fin.

#### Scenario: Consultation des journaux du Job
- **GIVEN** un Job dont le Pod associé produit des logs
- **WHEN** un client appelle `GET /api/jobs/{id}/logs`
- **THEN** le système retourne le flux textuel des logs du conteneur d'installation.

---

### Requirement: Désinstallation Sécurisée d'un Provider
Le système SHALL permettre la désinstallation d'un provider via un Job `clusterctl delete`, avec option explicite pour la purge des CRDs.

#### Scenario: Désinstallation standard
- **GIVEN** un provider installé sans ressource orpheline
- **WHEN** un client appelle `DELETE /api/providers/{id}` sans paramètre `includeCRD`
- **THEN** le système lance un Job exécutant `clusterctl delete --{category} {id}` en préservant les CRDs.

#### Scenario: Désinstallation avec purge des CRDs
- **GIVEN** un utilisateur souhaitant supprimer complètement un provider et ses CRDs
- **WHEN** un client appelle `DELETE /api/providers/{id}?includeCRD=true`
- **THEN** le système lance le Job avec le drapeau `--include-crd` et consigne un avertissement dans les logs d'audit.
