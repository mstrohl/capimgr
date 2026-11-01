REGISTRY     ?= 192.168.1.9:5000
TAG          ?= latest
KIND_CLUSTER ?= kind
NAMESPACE    ?= capi-dashboard-system
RELEASE      ?= capimgr

BACKEND_IMAGE  := $(REGISTRY)/capimgr-backend:$(TAG)
FRONTEND_IMAGE := $(REGISTRY)/capimgr-frontend:$(TAG)
CLUSTERCTL_IMAGE := $(REGISTRY)/capimgr-clusterctl:$(TAG)
CLUSTERCTL_VERSION ?= v1.11.3

.PHONY: build push build-clusterctl push-clusterctl kind-registry deploy undeploy all port-forward

build:
	docker build -f backend/backend.Dockerfile -t $(BACKEND_IMAGE) backend
	docker build -f frontend/frontend.Dockerfile -t $(FRONTEND_IMAGE) frontend
	$(MAKE) build-clusterctl

build-clusterctl:
	docker build -f clusterctl/clusterctl.Dockerfile --build-arg CLUSTERCTL_VERSION=$(CLUSTERCTL_VERSION) -t $(CLUSTERCTL_IMAGE) clusterctl

push-clusterctl: build-clusterctl
	docker push $(CLUSTERCTL_IMAGE)

push: build
	docker push $(BACKEND_IMAGE)
	docker push $(FRONTEND_IMAGE)
	docker push $(CLUSTERCTL_IMAGE)

# Autorise les noeuds kind à tirer depuis la registry HTTP (sans TLS)
kind-registry:
	@for node in $$(kind get nodes --name $(KIND_CLUSTER)); do \
	  docker exec $$node mkdir -p /etc/containerd/certs.d/$(REGISTRY); \
	  printf 'server = "http://$(REGISTRY)"\n\n[host."http://$(REGISTRY)"]\n  capabilities = ["pull", "resolve"]\n  skip_verify = true\n' \
	    | docker exec -i $$node tee /etc/containerd/certs.d/$(REGISTRY)/hosts.toml >/dev/null; \
	done

deploy: kind-registry
	helm upgrade --install $(RELEASE) deploy/helm/capimgr \
	  --kube-context kind-$(KIND_CLUSTER) \
	  --namespace $(NAMESPACE) --create-namespace \
	  --set image.registry=$(REGISTRY) --set image.tag=$(TAG) \
	  $(if $(GITHUB_TOKEN),--set githubToken=$(GITHUB_TOKEN),) \
	  --wait --timeout 3m
	kubectl --context kind-$(KIND_CLUSTER) -n $(NAMESPACE) rollout restart deploy/capimgr-backend deploy/capimgr-frontend

undeploy:
	helm uninstall $(RELEASE) --kube-context kind-$(KIND_CLUSTER) --namespace $(NAMESPACE)

all: push deploy

port-forward:
	kubectl --context kind-$(KIND_CLUSTER) -n $(NAMESPACE) port-forward svc/capimgr-frontend 8080:80
