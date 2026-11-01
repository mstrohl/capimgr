package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testInfos() []crdInfo {
	return []crdInfo{
		{Name: "clusters.cluster.x-k8s.io", Group: coreGroup, Kind: "Cluster", Version: "v1beta2", Provider: "core", Namespaced: true},
		{Name: "machinedeployments.cluster.x-k8s.io", Group: coreGroup, Kind: "MachineDeployment", Version: "v1beta2", Provider: "core", Namespaced: true},
		{Name: "machinehealthchecks.cluster.x-k8s.io", Group: coreGroup, Kind: "MachineHealthCheck", Version: "v1beta2", Provider: "core", Namespaced: true},
		{Name: "dockerclusters.infrastructure.cluster.x-k8s.io", Group: infrastructureGroup, Kind: "DockerCluster", Version: "v1beta2", Provider: "docker", Namespaced: true},
		{Name: "dockermachinetemplates.infrastructure.cluster.x-k8s.io", Group: infrastructureGroup, Kind: "DockerMachineTemplate", Version: "v1beta2", Provider: "docker", Namespaced: true},
		{Name: "kubeadmcontrolplanes.controlplane.cluster.x-k8s.io", Group: controlPlaneGroup, Kind: "KubeadmControlPlane", Version: "v1beta2", Provider: "kubeadm", Namespaced: true},
		{Name: "kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io", Group: bootstrapGroup, Kind: "KubeadmConfigTemplate", Version: "v1beta2", Provider: "kubeadm", Namespaced: true},
	}
}

func TestBuildBlueprintResolvesDockerProvider(t *testing.T) {
	bp, err := buildBlueprint(testInfos(), blueprintRequest{
		Name: "demo", Namespace: "capi-demo", KubernetesVersion: "v1.30.0",
		Infrastructure: "docker", ControlPlane: "kubeadm", Bootstrap: "kubeadm-bootstrap",
		CPReplicas: 1, WorkerReplicas: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Required) != 7 {
		t.Fatalf("required = %d, want 7", len(bp.Required))
	}
	if len(bp.Optional) == 0 {
		t.Fatal("optional manifests expected")
	}
}

func TestBuildBlueprintReportsMissingCRDs(t *testing.T) {
	_, err := buildBlueprint(testInfos(), blueprintRequest{
		Name: "demo", Namespace: "capi-demo", KubernetesVersion: "v1.30.0",
		Infrastructure: "aws", ControlPlane: "kubeadm", Bootstrap: "kubeadm-bootstrap",
	})
	if err == nil {
		t.Fatal("expected error for uninstalled provider")
	}
}

func TestCreateClusterCreatesManifests(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	previousClient := k8sClient
	k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	t.Cleanup(func() { k8sClient = previousClient })

	bp, err := buildBlueprint(testInfos(), blueprintRequest{
		Name: "demo", Namespace: "capi-demo", KubernetesVersion: "v1.30.0",
		Infrastructure: "docker", ControlPlane: "kubeadm", Bootstrap: "kubeadm-bootstrap",
		CPReplicas: 1, WorkerReplicas: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := CreateClusterPayload{Name: "demo", KubernetesVersion: "v1.30.0"}
	for _, d := range bp.Required {
		payload.Manifests = append(payload.Manifests, d.Object)
	}
	body, _ := json.Marshal(payload)
	response := httptest.NewRecorder()
	createCluster(response, httptest.NewRequest(http.MethodPost, "/api/clusters", bytes.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	ns := &corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "capi-demo"}, ns); err != nil {
		t.Fatal(err)
	}
}

func TestCreateClusterRejectsForeignGroups(t *testing.T) {
	payload := CreateClusterPayload{Name: "demo", Manifests: []map[string]interface{}{
		{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": "x"}},
	}}
	if _, err := prepareManifests(payload, "capi-demo"); err == nil {
		t.Fatal("expected rejection of non-CAPI resource")
	}
}

func TestGetClusterKubeconfigReturnsJSONAndDownloadHeader(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "capi-test-cluster",
			UID:       types.UID("cluster-uid"),
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-kubeconfig",
			Namespace: "capi-test-cluster",
		},
		Data: map[string][]byte{
			"value": []byte("apiVersion: v1\nclusters:\n- cluster:\n    server: https://api.example.test:6443\n  name: test\n"),
		},
	}

	previousClient := k8sClient
	k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, secret).Build()
	t.Cleanup(func() { k8sClient = previousClient })

	request := httptest.NewRequest(http.MethodGet, "/api/clusters/cluster-uid/kubeconfig", nil)
	response := httptest.NewRecorder()
	getClusterKubeconfig(response, request, "cluster-uid")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="test-cluster.kubeconfig"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	var result KubeconfigResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.ServerEndpoint != "https://api.example.test:6443" {
		t.Errorf("serverEndpoint = %q", result.ServerEndpoint)
	}
	if result.ClusterName != "test-cluster" {
		t.Errorf("clusterName = %q", result.ClusterName)
	}
}

func TestGetClusterKubeconfigReturnsConflictBeforeSecretExists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "capi-test-cluster",
			UID:       types.UID("cluster-uid"),
		},
	}

	previousClient := k8sClient
	k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	t.Cleanup(func() { k8sClient = previousClient })

	request := httptest.NewRequest(http.MethodGet, "/api/clusters/cluster-uid/kubeconfig", nil)
	response := httptest.NewRecorder()
	getClusterKubeconfig(response, request, "cluster-uid")

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestMachineIPs(t *testing.T) {
	machine := map[string]interface{}{
		"status": map[string]interface{}{
			"addresses": []interface{}{
				map[string]interface{}{"type": "InternalIP", "address": "10.0.0.5"},
				map[string]interface{}{"type": "ExternalIP", "address": "203.0.113.5"},
			},
		},
	}
	internalIP, externalIP := machineIPs(machine)
	if internalIP != "10.0.0.5" || externalIP != "203.0.113.5" {
		t.Fatalf("machineIPs() = (%q, %q)", internalIP, externalIP)
	}
}

func TestProviderCommandsHonorVersionAndCRDPurge(t *testing.T) {
	installArgs := getProviderInstallCommand("infrastructure", "aws", "v2.4.0")
	wantInstall := []string{"init", "--infrastructure", "aws:v2.4.0"}
	if len(installArgs) != len(wantInstall) {
		t.Fatalf("install args = %v, want %v", installArgs, wantInstall)
	}
	for i := range wantInstall {
		if installArgs[i] != wantInstall[i] {
			t.Fatalf("install args = %v, want %v", installArgs, wantInstall)
		}
	}

	deleteArgs := getProviderDeleteCommand("infrastructure", "aws", true)
	wantDelete := []string{"delete", "--infrastructure", "aws", "--include-crd"}
	if len(deleteArgs) != len(wantDelete) {
		t.Fatalf("delete args = %v, want %v", deleteArgs, wantDelete)
	}
	for i := range wantDelete {
		if deleteArgs[i] != wantDelete[i] {
			t.Fatalf("delete args = %v, want %v", deleteArgs, wantDelete)
		}
	}
}
