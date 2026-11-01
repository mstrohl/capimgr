package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSelectClusterManifests(t *testing.T) {
	obj := func(apiVersion, kind, name string, spec map[string]interface{}) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": apiVersion, "kind": kind,
			"metadata": map[string]interface{}{"name": name, "namespace": "shared"},
			"spec":     spec,
		}}
	}
	cluster := obj(coreGroup+"/v1beta2", "Cluster", "demo", map[string]interface{}{
		"infrastructureRef": map[string]interface{}{"apiGroup": infrastructureGroup, "kind": "DockerCluster", "name": "infra"},
		"otherRef":          map[string]interface{}{"apiGroup": infrastructureGroup, "kind": "DockerCluster", "name": "foreign", "namespace": "elsewhere"},
		"topology":          map[string]interface{}{"classRef": map[string]interface{}{"name": "class"}},
	})
	class := obj(coreGroup+"/v1beta2", "ClusterClass", "class", nil)
	infra := obj(infrastructureGroup+"/v1beta2", "DockerCluster", "infra", map[string]interface{}{
		"template": map[string]interface{}{"apiVersion": infrastructureGroup + "/v1beta1", "kind": "DockerMachineTemplate", "name": "template"},
	})
	template := obj(infrastructureGroup+"/v1beta2", "DockerMachineTemplate", "template", nil)
	deployment := obj(coreGroup+"/v1beta2", "MachineDeployment", "workers", map[string]interface{}{"clusterName": "demo"})
	owned := obj(coreGroup+"/v1beta2", "MachineSet", "owned", nil)
	deployment.SetUID("md-uid")
	owned.SetOwnerReferences([]metav1.OwnerReference{{UID: "md-uid"}})
	other := obj(coreGroup+"/v1beta2", "Cluster", "other", nil)
	foreign := obj(infrastructureGroup+"/v1beta2", "DockerCluster", "foreign", nil)
	result := selectClusterManifests([]unstructured.Unstructured{cluster, class, infra, template, deployment, owned, other, foreign}, "demo", "cluster-uid")
	got := map[string]bool{}
	for _, item := range result {
		got[item.GetName()] = true
	}
	for _, name := range []string{"demo", "class", "infra", "template", "workers", "owned"} {
		if !got[name] {
			t.Errorf("missing linked object %s", name)
		}
	}
	if got["other"] || got["foreign"] {
		t.Errorf("included unrelated objects: %v", got)
	}
}

func TestClusterManifestsUnknownCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	previous := k8sClient
	k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	t.Cleanup(func() { k8sClient = previous })
	response := httptest.NewRecorder()
	handleClusterByID(response, httptest.NewRequest(http.MethodGet, "/api/clusters/missing/manifests", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
}
