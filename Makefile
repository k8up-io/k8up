# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Current Operator version
VERSION ?= $(shell date +%Y-%m-%d_%H-%M-%S)
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

BIN_FILENAME ?= k8up

CRD_SPEC_VERSION ?= v1

CRD_FILE ?= k8up-crd.yaml
CRD_FILE_LEGACY ?= k8up-crd-legacy.yaml
CRD_ROOT_DIR ?= config/crd/apiextensions.k8s.io

TESTBIN_DIR ?= ./testbin/bin
KIND_BIN ?= $(TESTBIN_DIR)/kind
KIND_VERSION ?= 0.9.0
KIND_NODE_VERSION ?= v1.18.8
KIND_KUBECONFIG ?= ./testbin/kind-kubeconfig-$(KIND_NODE_VERSION)
KIND_CLUSTER ?= k8up-$(KIND_NODE_VERSION)
KIND_KUBECTL_ARGS ?= --validate=true

SETUP_KIND := testbin/.kind_setup_complete
E2E_TAG ?= e2e_$(VERSION)
E2E_REPO ?= local.dev/k8up/e2e

ENABLE_LEADER_ELECTION ?= false

# Image URL to use all building/pushing image targets
DOCKER_IMG ?= docker.io/vshn/k8up:$(IMG_TAG)
QUAY_IMG ?= quay.io/vshn/k8up:$(IMG_TAG)
E2E_IMG := $(E2E_REPO):$(E2E_TAG)

build_cmd ?= CGO_ENABLED=0 go build -o $(BIN_FILENAME) main.go

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v3
KUSTOMIZE_BUILD_CRD ?= $(KUSTOMIZE) build $(CRD_ROOT_DIR)/$(CRD_SPEC_VERSION)

all: build ## Invokes the build target

test: fmt vet ## Run tests
	go test ./... -coverprofile cover.out

# Run tests (see https://sdk.operatorframework.io/docs/building-operators/golang/references/envtest-setup)
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin

$(TESTBIN_DIR):
	mkdir -p $(TESTBIN_DIR)

# See https://storage.googleapis.com/kubebuilder-tools/ for list of supported K8s versions
# No, there's no 1.18 support, so we're going for 1.19
integration_test: export ENVTEST_K8S_VERSION = 1.19.2
integration_test: generate fmt vet $(TESTBIN_DIR) ## Run integration tests with envtest
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -tags=integration -v ./... -coverprofile cover.out

build: generate fmt vet ## Build manager binary
	$(build_cmd)

dist: generate fmt vet ## Generates a release
	goreleaser release --snapshot --rm-dist --skip-sign

run: export BACKUP_ENABLE_LEADER_ELECTION = $(ENABLE_LEADER_ELECTION)
run: fmt vet ## Run against the configured Kubernetes cluster in ~/.kube/config
	go run ./main.go

install: generate ## Install CRDs into a cluster
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 | kubectl apply -f -

uninstall: generate ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 | kubectl delete -f -

deploy: generate ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

generate: ## Generate manifests e.g. CRD, RBAC etc.
	@CRD_ROOT_DIR="$(CRD_ROOT_DIR)" go generate -tags=generate generate.go
	@rm config/*.yaml

crd: generate ## Generate CRD to file
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1 > $(CRD_FILE)
	$(KUSTOMIZE) build $(CRD_ROOT_DIR)/v1beta1 > $(CRD_FILE_LEGACY)

fmt: ## Run go fmt against code
	go fmt ./...

vet: ## Run go vet against code
	go vet ./...

lint: fmt vet ## Invokes the fmt and vet targets
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

# Build the binary without running generators
.PHONY: $(BIN_FILENAME)
$(BIN_FILENAME):
	$(build_cmd)

docker-build: export GOOS = linux
docker-build: $(BIN_FILENAME) ## Build the docker image
	docker build . -t $(DOCKER_IMG) -t $(QUAY_IMG) -t $(E2E_IMG)

docker-push: ## Push the docker image
	docker push $(DOCKER_IMG)
	docker push $(QUAY_IMG)

.PHONY: bundle
bundle: generate ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

clean: export KUBECONFIG = $(KIND_KUBECONFIG)
clean: clean_e2e clean_kind ## Cleans up the generated resources
	rm -r testbin/ dist/ bin/ cover.out $(BIN_FILENAME) || true

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

###
### KIND
###

.PHONY: rund_kind
run_kind: export KUBECONFIG = $(KIND_KUBECONFIG)
run_kind: export BACKUP_ENABLE_LEADER_ELECTION = $(ENABLE_LEADER_ELECTION)
run_kind: $(SETUP_KIND) ## Runs the operator in kind
	go run ./main.go

$(KIND_BIN): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_BIN): $(TESTBIN_DIR)
	curl -Lo $(KIND_BIN) "https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$$(uname)-amd64"
	@chmod +x $(KIND_BIN)

$(KIND_KUBECONFIG): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_KUBECONFIG): $(KIND_BIN)
	$(KIND_BIN) create cluster --name $(KIND_CLUSTER) --image kindest/node:$(KIND_NODE_VERSION)
	@kubectl version
	@kubectl cluster-info

$(SETUP_KIND): export KUBECONFIG = $(KIND_KUBECONFIG)
$(SETUP_KIND): $(KIND_KUBECONFIG)
	@kubectl config use-context kind-$(KIND_CLUSTER)
	@$(KUSTOMIZE_BUILD_CRD) | kubectl apply $(KIND_KUBECTL_ARGS) -f -
	@touch $(SETUP_KIND)

.PHONY: clean_kind
clean_kind: export KUBECONFIG = $(KIND_KUBECONFIG)
clean_kind: ## Remove the KIND Cluster
	$(KIND_BIN) delete cluster --name $(KIND_CLUSTER) || true
	@rm $(SETUP_KIND) || true

###
### E2E Test
###

e2e_test: export E2E_IMAGE = $(E2E_IMG)
e2e_test: setup_e2e_test $(KIND_KUBECONFIG) ## Runs the e2e test suite
	$(MAKE) -C e2e run_bats -e KUBECONFIG=../$(KIND_KUBECONFIG)

.PHONY: setup_e2e_test
setup_e2e_test: $(SETUP_KIND) docker-build ## Run the e2e setup
	$(MAKE) -C e2e install_bats
	@$(KIND_BIN) load docker-image --name $(KIND_CLUSTER) $(E2E_IMG)

.PHONY: clean_e2e_setup
clean_e2e_setup: export KUBECONFIG = $(KIND_KUBECONFIG)
clean_e2e_setup: ## Clean the e2e setup (e.g. to rerun the setup_e2e_test)
	kubectl delete ns k8up-system --ignore-not-found --force --grace-period=0 || true
	@$(KUSTOMIZE_BUILD_CRD) | kubectl delete -f - || true

.PHONY: clean_e2e
clean_e2e: clean_e2e_setup
	docker images --filter "reference=$(E2E_REPO)" --format "{{.Repository }}:{{ .Tag }}" | xargs --no-run-if-empty docker rmi || true
	$(MAKE) -C e2e clean

###
### Documentation
###

include ./docs/docs.mk
