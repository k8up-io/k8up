# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMG_TAG ?= latest

# Image URL to use all building/pushing image targets
DOCKER_IMG ?= docker.io/vshn/k8up:$(IMG_TAG)
QUAY_IMG ?= quay.io/vshn/k8up:$(IMG_TAG)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

CRD_FILE ?= k8up-crd.yaml

TESTBIN_DIR ?= ./testbin/bin
KIND_BIN ?= $(TESTBIN_DIR)/kind
KIND_VERSION ?= 0.9.0
KIND_KUBECONFIG ?= ./testbin/kind-kubeconfig
KIND_CLUSTER ?= k8up

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

all: build

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Run tests (see https://sdk.operatorframework.io/docs/building-operators/golang/references/envtest-setup)
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin

$(TESTBIN_DIR):
	mkdir -p $(TESTBIN_DIR)

integration_test: export ENVTEST_K8S_VERSION = 1.19.0
integration_test: generate fmt vet manifests $(TESTBIN_DIR)
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -tags=integration -v ./... -coverprofile cover.out

# Build manager binary
build: generate fmt vet
	go build -o k8up main.go

dist: generate fmt vet
	goreleaser release --snapshot --rm-dist --skip-sign

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@go run hack/patch-crd-compatibility.go config/crd/bases/backup.appuio.ch_prebackuppods.yaml

# Generate CRD to file
crd: manifests kustomize
	$(KUSTOMIZE) build config/crd > $(CRD_FILE)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: fmt vet
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: build
	docker build . -t $(DOCKER_IMG) -t $(QUAY_IMG)

# Push the docker image
docker-push:
	docker push $(DOCKER_IMG)
	docker push $(QUAY_IMG)

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

setup_e2e_test: export KUBECONFIG = $(KIND_KUBECONFIG)
setup_e2e_test: $(KIND_BIN) manifests
	@kubectl config use-context kind-$(KIND_CLUSTER)
	@kubectl apply -k config/crd

run_kind: export KUBECONFIG = $(KIND_KUBECONFIG)
run_kind: setup_e2e_test
	go run ./main.go

$(KIND_BIN): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_BIN): $(TESTBIN_DIR)
	curl -Lo $(KIND_BIN) "https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$$(uname)-amd64"
	chmod +x $(KIND_BIN)
	$(KIND_BIN) create cluster --name $(KIND_CLUSTER)
	kubectl cluster-info

clean: export KUBECONFIG = $(KIND_KUBECONFIG)
clean:
	$(KIND_BIN) delete cluster --name $(KIND_CLUSTER) || true
	rm -r testbin/ dist/ bin/ cover.out k8up || true
