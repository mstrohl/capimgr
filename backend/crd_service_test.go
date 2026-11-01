package main

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestDetectProviderFromCRD(t *testing.T) {
	tests := []struct {
		name        string
		crdName     string
		group       string
		kind        string
		labels      map[string]string
		annotations map[string]string
		expected    string
	}{
		{
			name:     "AWS Cluster via name",
			crdName:  "awsclusters.infrastructure.cluster.x-k8s.io",
			group:    "infrastructure.cluster.x-k8s.io",
			kind:     "AWSCluster",
			expected: "aws",
		},
		{
			name:     "Azure Machine via kind",
			crdName:  "azuremachines.infrastructure.cluster.x-k8s.io",
			group:    "infrastructure.cluster.x-k8s.io",
			kind:     "AzureMachine",
			expected: "azure",
		},
		{
			name:     "Proxmox via name",
			crdName:  "proxmoxclusters.infrastructure.cluster.x-k8s.io",
			group:    "infrastructure.cluster.x-k8s.io",
			kind:     "ProxmoxCluster",
			expected: "proxmox",
		},
		{
			name:     "Talos Control Plane",
			crdName:  "taloscontrolplanes.controlplane.cluster.x-k8s.io",
			group:    "controlplane.cluster.x-k8s.io",
			kind:     "TalosControlPlane",
			expected: "talos",
		},
		{
			name:     "Kubeadm Bootstrap Config",
			crdName:  "kubeadmconfigs.bootstrap.cluster.x-k8s.io",
			group:    "bootstrap.cluster.x-k8s.io",
			kind:     "KubeadmConfig",
			expected: "kubeadm-bootstrap",
		},
		{
			name:    "Override via label",
			crdName: "customclusters.infra.custom.io",
			group:   "infra.custom.io",
			kind:    "CustomCluster",
			labels: map[string]string{
				"cluster.x-k8s.io/provider": "my-custom-provider",
			},
			expected: "my-custom-provider",
		},
		{
			name:     "Core Cluster-API",
			crdName:  "clusters.cluster.x-k8s.io",
			group:    "cluster.x-k8s.io",
			kind:     "Cluster",
			expected: "cluster-api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectProviderFromCRD(tt.crdName, tt.group, tt.kind, tt.labels, tt.annotations)
			if result != tt.expected {
				t.Errorf("detectProviderFromCRD() = %v, attendu %v", result, tt.expected)
			}
		})
	}
}

func TestExtractSpecSubSchema(t *testing.T) {
	rootSchema := map[string]interface{}{
		"description": "Schéma racine du Cluster",
		"type":        "object",
		"properties": map[string]interface{}{
			"metadata": map[string]interface{}{"type": "object"},
			"status": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ready": map[string]interface{}{"type": "boolean"},
				},
			},
			"spec": map[string]interface{}{
				"type": "object",
				"required": []interface{}{
					"region",
				},
				"properties": map[string]interface{}{
					"region": map[string]interface{}{
						"type":        "string",
						"description": "Région de déploiement",
						"default":     "eu-west-3",
					},
					"status": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	spec := extractSpecSubSchema(rootSchema)

	if spec == nil {
		t.Fatalf("extractSpecSubSchema a renvoyé nil")
	}

	props, ok := spec["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Le champ 'properties' est manquant dans 'spec'")
	}

	if _, exists := props["region"]; !exists {
		t.Errorf("La propriété 'region' devrait être présente dans 'spec'")
	}

	if _, exists := props["status"]; exists {
		t.Errorf("Le champ 'status' résiduel aurait dû être supprimé")
	}

	if desc, ok := spec["description"].(string); !ok || desc != "Schéma racine du Cluster" {
		t.Errorf("La description parente aurait dû être héritée, obtenu %v", spec["description"])
	}
}

func TestGetActiveCRDVersion(t *testing.T) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: "v1alpha1", Storage: false, Served: true},
				{Name: "v1beta1", Storage: true, Served: true},
				{Name: "v1beta2", Storage: false, Served: true},
			},
		},
	}

	activeVer := getActiveCRDVersion(crd)
	if activeVer != "v1beta1" {
		t.Errorf("getActiveCRDVersion() = %v, attendu v1beta1", activeVer)
	}
}
