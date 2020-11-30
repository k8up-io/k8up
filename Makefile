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

CRD_SPEC_VERSION ?= v1

CRD_FILE ?= k8up-crd.yaml
CRD_FILE_LEGACY ?= k8up-crd-legacy.yaml
CRD_ROOT_DIR ?= config/crd/apiextensions.k8s.io

TESTBIN_DIR ?= ./testbin/bin
KIND_BIN ?= $(TESTBIN_DIR)/kind
KIND_VERSION ?= 0.9.0
KIND_KUBECONFIG ?= ./testbin/kind-kubeconfig
KIND_NODE_VERSION ?= v1.18.8
KIND_CLUSTER ?= k8up-$(KIND_NODE_VERSION)
KIND_KUBECTL_ARGS ?= --validate=true

antora_preview_cmd ?= docker run --rm --publish 35729:35729 --publish 2020:2020 --volume "${PWD}":/preview/antora docker.io/vshn/antora-preview:2.3.4 --style=syn --antora=docs

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v3

all: build

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Run tests (see https://sdk.operatorframework.io/docs/building-operators/golang/references/envtest-setup)
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin

$(TESTBIN_DIR):
	mkdir -p $(TESTBIN_DIR)

# Run integration tests with envtest
# See https://storage.googleapis.com/kubebuilder-tools/ for list of supported K8s versions
# No, there's no 1.18 support, so we're going for 1.19
integration_test: export ENVTEST_K8S_VERSION = 1.19.2
integration_test: generate fmt vet $(TESTBIN_DIR)
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -tags=integration -v ./... -coverprofile cover.out

# Build manager binary
build: generate fmt vet
	go build -o k8up main.go

dist: generate fmt vet
	goreleaser release --snapshot --rm-dist --skip-sign

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./main.go

# Install CRDs into a cluster
install: generate
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: generate
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
generate:
	@CRD_ROOT_DIR="$(CRD_ROOT_DIR)" go generate -tags=generate generate.go
	@rm config/*.yaml

# Generate CRD to file
crd: generate
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 > $(CRD_FILE)
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1beta1 > $(CRD_FILE_LEGACY)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: fmt vet
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

# Build the docker image
docker-build: build
	docker build . -t $(DOCKER_IMG) -t $(QUAY_IMG)

# Push the docker image
docker-push:
	docker push $(DOCKER_IMG)
	docker push $(QUAY_IMG)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: generate
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

docs-serve:
	$(antora_preview_cmd)

e2e_test: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e_test: setup_e2e_test build
	@echo "TODO: Add actual e2e tests!"

setup_e2e_test: export KUBECONFIG = $(KIND_KUBECONFIG)
setup_e2e_test: $(KIND_BIN)
	@kubectl config use-context kind-$(KIND_CLUSTER)
	@$(KUSTOMIZE) build $(CRD_ROOT_DIR)/$(CRD_SPEC_VERSION) | kubectl apply $(KIND_KUBECTL_ARGS) -f -

run_kind: export KUBECONFIG = $(KIND_KUBECONFIG)
run_kind: setup_e2e_test
	go run ./main.go

$(KIND_BIN): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_BIN): $(TESTBIN_DIR)
	curl -Lo $(KIND_BIN) "https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$$(uname)-amd64"
	@chmod +x $(KIND_BIN)
	$(KIND_BIN) create cluster --name $(KIND_CLUSTER) --image kindest/node:$(KIND_NODE_VERSION)
	@kubectl version
	@kubectl cluster-info

clean: export KUBECONFIG = $(KIND_KUBECONFIG)
clean:
	$(KIND_BIN) delete cluster --name $(KIND_CLUSTER) || true
	rm -r testbin/ dist/ bin/ cover.out k8up || true
