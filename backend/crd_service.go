package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// CRDItem représente une CustomResourceDefinition exposée à l'UI
type CRDItem struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Group    string `json:"group"`
	Version  string `json:"version"`
	Scope    string `json:"scope"`
	Provider string `json:"provider"`
}

// CRDSchemaResponse contient le schéma JSON nettoyé prêt pour le générateur de formulaire
type CRDSchemaResponse struct {
	CRDName    string                 `json:"crdName"`
	Kind       string                 `json:"kind"`
	APIVersion string                 `json:"apiVersion"`
	Schema     map[string]interface{} `json:"schema"`
}

type crdNotFoundError struct {
	name string
}

func (err *crdNotFoundError) Error() string {
	return fmt.Sprintf("CRD introuvable: %s", err.name)
}

// listCRDs liste les CRDs du cluster et les filtre optionnellement par provider ou groupe d'API
func listCRDs(ctx context.Context, providerFilter, groupFilter string) ([]CRDItem, error) {
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := k8sClient.List(ctx, crdList); err != nil {
		return nil, fmt.Errorf("erreur lors du listage des CRDs: %w", err)
	}

	items := make([]CRDItem, 0, len(crdList.Items))
	for _, crd := range crdList.Items {
		group := crd.Spec.Group
		kind := crd.Spec.Names.Kind
		crdName := crd.Name

		// Exclure les CRDs cert-manager (non pertinentes pour la configuration des providers CAPI)
		if strings.HasSuffix(group, "cert-manager.io") {
			continue
		}

		// Filtrage par groupe si spécifié
		if groupFilter != "" && !strings.EqualFold(group, groupFilter) {
			continue
		}

		// Trouver la version active (storage=true ou première servie)
		version := getActiveCRDVersion(&crd)
		provider := detectProviderFromCRD(crdName, group, kind, crd.GetLabels(), crd.GetAnnotations())

		// Filtrage par provider si spécifié
		if providerFilter != "" && !strings.EqualFold(provider, providerFilter) {
			continue
		}

		items = append(items, CRDItem{
			Name:     crdName,
			Kind:     kind,
			Group:    group,
			Version:  version,
			Scope:    string(crd.Spec.Scope),
			Provider: provider,
		})
	}

	// Tri alphabétique par nom
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items, nil
}

// getCRDSchema extrait et normalise le sous-schéma 'spec' d'une CRD
func getCRDSchema(ctx context.Context, crdName, requestedVersion string) (*CRDSchemaResponse, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	key := types.NamespacedName{Name: crdName}
	if err := k8sClient.Get(ctx, key, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &crdNotFoundError{name: crdName}
		}
		return nil, fmt.Errorf("erreur lors de la récupération de la CRD %s: %w", crdName, err)
	}

	// Trouver la version demandée ou la version active
	var targetVersion *apiextensionsv1.CustomResourceDefinitionVersion
	for i, v := range crd.Spec.Versions {
		if requestedVersion != "" && v.Name == requestedVersion {
			targetVersion = &crd.Spec.Versions[i]
			break
		}
		if requestedVersion == "" && v.Storage {
			targetVersion = &crd.Spec.Versions[i]
		}
	}

	// Si aucune version de stockage n'a été marquée et pas de version demandée, prendre la première servie
	if targetVersion == nil && len(crd.Spec.Versions) > 0 {
		for i, v := range crd.Spec.Versions {
			if v.Served {
				targetVersion = &crd.Spec.Versions[i]
				break
			}
		}
		if targetVersion == nil {
			targetVersion = &crd.Spec.Versions[0]
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("aucune version trouvée pour la CRD %s", crdName)
	}

	if targetVersion.Schema == nil || targetVersion.Schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("la CRD %s ne contient pas de openAPIV3Schema pour la version %s", crdName, targetVersion.Name)
	}

	// Conversion du schéma OpenAPIV3Schema en map générique pour manipulation
	rawJSON, err := json.Marshal(targetVersion.Schema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la sérialisation du schéma de la CRD: %w", err)
	}

	var rootSchema map[string]interface{}
	if err := json.Unmarshal(rawJSON, &rootSchema); err != nil {
		return nil, fmt.Errorf("erreur lors de la désérialisation du schéma de la CRD: %w", err)
	}

	// Extraction du sous-schéma 'spec' s'il existe
	specSchema := extractSpecSubSchema(rootSchema)

	apiVersion := fmt.Sprintf("%s/%s", crd.Spec.Group, targetVersion.Name)

	return &CRDSchemaResponse{
		CRDName:    crd.Name,
		Kind:       crd.Spec.Names.Kind,
		APIVersion: apiVersion,
		Schema:     specSchema,
	}, nil
}

// extractSpecSubSchema extrait 'properties.spec' et nettoie les attributs non configurables
func extractSpecSubSchema(rootSchema map[string]interface{}) map[string]interface{} {
	props, ok := rootSchema["properties"].(map[string]interface{})
	if !ok {
		return rootSchema
	}

	spec, hasSpec := props["spec"].(map[string]interface{})
	if !hasSpec {
		// Pas de 'spec', retourner le schéma complet (sans 'status')
		delete(props, "status")
		delete(props, "metadata")
		removeReadOnlyProperties(rootSchema)
		return rootSchema
	}

	// Si 'description' n'est pas renseignée sur 'spec', récupérer celle du parent si disponible
	if _, ok := spec["description"]; !ok {
		if rootDesc, ok := rootSchema["description"].(string); ok {
			spec["description"] = rootDesc
		}
	}

	removeReadOnlyProperties(spec)

	return spec
}

func removeReadOnlyProperties(schema map[string]interface{}) {
	properties, ok := schema["properties"].(map[string]interface{})
	if ok {
		removed := make(map[string]bool)
		for name, rawProperty := range properties {
			property, ok := rawProperty.(map[string]interface{})
			if !ok {
				continue
			}
			if readOnly, _ := property["readOnly"].(bool); readOnly || name == "status" {
				delete(properties, name)
				removed[name] = true
				continue
			}
			removeReadOnlyProperties(property)
		}
		if required, ok := schema["required"].([]interface{}); ok {
			filtered := make([]interface{}, 0, len(required))
			for _, name := range required {
				if key, ok := name.(string); !ok || !removed[key] {
					filtered = append(filtered, name)
				}
			}
			if len(filtered) == 0 {
				delete(schema, "required")
			} else {
				schema["required"] = filtered
			}
		}
	}
	if items, ok := schema["items"].(map[string]interface{}); ok {
		removeReadOnlyProperties(items)
	}
}

// getActiveCRDVersion retourne le nom de la version active de stockage
func getActiveCRDVersion(crd *apiextensionsv1.CustomResourceDefinition) string {
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			return v.Name
		}
	}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			return v.Name
		}
	}
	if len(crd.Spec.Versions) > 0 {
		return crd.Spec.Versions[0].Name
	}
	return "v1"
}

// detectProviderFromCRD détermine le provider CAPI parent à partir des métadonnées de la CRD
func detectProviderFromCRD(crdName, group, kind string, labels, annotations map[string]string) string {
	normalize := func(p string) string {
		// Les providers kubeadm/talos existent en control plane ET en bootstrap : le catalogue distingue le second par un suffixe.
		if bootstrapName := strings.TrimPrefix(p, "bootstrap-"); bootstrapName != p && (bootstrapName == "kubeadm" || bootstrapName == "talos") {
			return bootstrapName + "-bootstrap"
		}
		p = strings.TrimPrefix(p, "infrastructure-")
		p = strings.TrimPrefix(p, "control-plane-")
		p = strings.TrimPrefix(p, "bootstrap-")
		p = strings.TrimPrefix(p, "core-")
		return p
	}

	// 1. Vérification dans les annotations ou labels standards de clusterctl
	if val, ok := labels["cluster.x-k8s.io/provider"]; ok && val != "" {
		return normalize(val)
	}
	if val, ok := annotations["clusterctl.cluster.x-k8s.io/provider"]; ok && val != "" {
		return normalize(val)
	}

	// 2. Vérification par groupe CAPI standard
	lowerName := strings.ToLower(crdName)
	lowerKind := strings.ToLower(kind)

	switch {
	// Infrastructure
	case strings.Contains(lowerName, "aws") || strings.Contains(lowerKind, "aws"):
		return "aws"
	case strings.Contains(lowerName, "azure") || strings.Contains(lowerKind, "azure"):
		return "azure"
	case strings.Contains(lowerName, "gcp") || strings.Contains(lowerKind, "gcp"):
		return "gcp"
	case strings.Contains(lowerName, "vsphere") || strings.Contains(lowerKind, "vsphere"):
		return "vsphere"
	case strings.Contains(lowerName, "openstack") || strings.Contains(lowerKind, "openstack"):
		return "openstack"
	case strings.Contains(lowerName, "proxmox") || strings.Contains(lowerKind, "proxmox"):
		return "proxmox"
	case strings.Contains(lowerName, "digitalocean") || strings.Contains(lowerKind, "digitalocean"):
		return "digitalocean"
	case strings.Contains(lowerName, "hetzner") || strings.Contains(lowerKind, "hetzner"):
		return "hetzner"
	case strings.Contains(lowerName, "docker") || strings.Contains(lowerKind, "docker"):
		return "docker"

	// Control Plane & Bootstrap
	case strings.Contains(lowerName, "kubeadmconfig") || strings.Contains(lowerKind, "kubeadmconfig"):
		return "kubeadm-bootstrap"
	case strings.Contains(lowerName, "kubeadmcontrolplane") || strings.Contains(lowerKind, "kubeadmcontrolplane"):
		return "kubeadm"
	case strings.Contains(lowerName, "talosconfig") || strings.Contains(lowerKind, "talosconfig"):
		return "talos-bootstrap"
	case strings.Contains(lowerName, "taloscontrolplane") || strings.Contains(lowerKind, "taloscontrolplane"):
		return "talos"

	// Core CAPI
	case group == "cluster.x-k8s.io":
		return "cluster-api"
	}

	// Fallback : extraire le premier mot avant "clusters" ou "machines"
	parts := strings.Split(crdName, ".")
	if len(parts) > 0 {
		prefix := parts[0]
		for _, suffix := range []string{"clusters", "machines", "machinetemplates", "configs", "controlplanes"} {
			if strings.HasSuffix(prefix, suffix) {
				return strings.TrimSuffix(prefix, suffix)
			}
		}
	}

	return "unknown"
}
