# Spécification : Formulaires Dynamiques pour CRDs (Dynamic CRD Forms)

## But (Purpose)
Cette spécification définit le mécanisme d'introspection des schémas de `CustomResourceDefinition` (CRD) et de génération automatique de formulaires interactifs côté interface utilisateur, permettant la configuration de n'importe quelle ressource introduite par les providers sans développement ad-hoc.

## Exigences (Requirements)

### Requirement: Introspection et Liste des CRDs Installées
Le système SHALL répertorier l'ensemble des CRDs installées sur le cluster de management et permettre leur filtrage par provider ou groupe d'API (`infrastructure.cluster.x-k8s.io`, `controlplane.cluster.x-k8s.io`, `bootstrap.cluster.x-k8s.io`).

#### Scenario: Lister les CRDs disponibles
- **GIVEN** des providers installés disposant de CRDs actives
- **WHEN** un client appelle `GET /api/crds`
- **THEN** le système retourne un tableau d'objets décrivant chaque CRD :
  - `name` (ex: `awsclusters.infrastructure.cluster.x-k8s.io`)
  - `kind` (ex: `AWSCluster`)
  - `group` (ex: `infrastructure.cluster.x-k8s.io`)
  - `version` (version stockée ou préférée, ex: `v1beta2`)
  - `scope` (`Namespaced` ou `Cluster`)
  - `provider` (identifiant du provider parent déduit).

#### Scenario: Filtrer les CRDs par provider
- **GIVEN** plusieurs providers installés dans le cluster
- **WHEN** un client appelle `GET /api/crds?provider=aws`
- **THEN** le système ne retourne que les CRDs appartenant au provider spécifié.

---

### Requirement: Extraction et Normalisation du Schéma OpenAPI v3
Le système SHALL extraire le sous-schéma OpenAPI v3 correspondant à la spécification configurable (`spec`) de la CRD et le convertir en un format JSON Schema exploitable par le moteur de rendu de formulaire.

#### Scenario: Récupération du schéma de configuration
- **GIVEN** une CRD valide présente sur le cluster
- **WHEN** un client appelle `GET /api/crds/{name}/schema`
- **THEN** le système localise la version active de la CRD dans `spec.versions`
- **THEN** le système extrait `schema.openAPIV3Schema.properties.spec`
- **THEN** le système exclut les champs en lecture seule et les attributs internes de statut
- **THEN** le système retourne le JSON Schema avec les types, énumérations (`enum`), descriptions, valeurs par défaut (`default`) et contraintes de validation (`required`, `minimum`, `maximum`, `pattern`).

#### Scenario: CRD inexistante
- **GIVEN** un nom de CRD non présent sur le cluster
- **WHEN** un client appelle `GET /api/crds/{name}/schema`
- **THEN** le système retourne un code HTTP `404 Not Found`.

---

### Requirement: Rendu Visuel du Formulaire Dynamique
L'interface utilisateur SHALL générer des composants de formulaire appropriés pour chaque propriété du schéma JSON reçu.

#### Scenario: Champs de types primitifs
- **GIVEN** une propriété de type `string`, `integer` ou `boolean`
- **WHEN** le composant `CRDFormRenderer` traite le schéma
- **THEN** il génère un champ texte pour `string`, un champ numérique pour `integer`, et un interrupteur à bascule pour `boolean`.

#### Scenario: Champs à choix restreints (Enum)
- **GIVEN** une propriété comportant une liste `enum: ["us-east-1", "eu-west-1", "eu-west-3"]`
- **WHEN** le composant génère le champ
- **THEN** il rend un composant de sélection déroulant avec ces options exactes.

#### Scenario: Objets et structures imbriquées
- **GIVEN** une propriété de type `object` avec ses propres sous-propriétés
- **WHEN** le composant génère le champ
- **THEN** il crée un bloc visuel repliable (Accordéon / Carte) regroupant les sous-champs avec son titre et sa description d'aide.

#### Scenario: Tableaux de valeurs et listes d'objets
- **GIVEN** une propriété de type `array`
- **WHEN** le composant génère le champ
- **THEN** il offre une interface dynamique d'ajout/suppression d'éléments (`[ + Ajouter ]`, `[ Supprimer ]`).

---

### Requirement: Synchronisation Bidirectionnelle Formulaire <-> YAML
L'interface utilisateur SHALL maintenir une synchronisation en temps réel entre les valeurs saisies dans le formulaire dynamique et la représentation YAML standard de la ressource Kubernetes.

#### Scenario: Saisie dans le formulaire répercutée dans le YAML
- **GIVEN** l'utilisateur renseigne un champ dans le formulaire dynamique
- **WHEN** la valeur est modifiée
- **THEN** l'onglet éditeur YAML affiche instantanément le document Kubernetes mis à jour avec `apiVersion`, `kind`, `metadata` et la nouvelle valeur dans `spec`.

#### Scenario: Édition directe dans le YAML répercutée dans le formulaire
- **GIVEN** l'utilisateur colle ou modifie directement le texte dans l'éditeur YAML
- **WHEN** la syntaxe YAML est valide
- **THEN** les champs du formulaire dynamique se mettent à jour pour refléter les nouvelles valeurs.
