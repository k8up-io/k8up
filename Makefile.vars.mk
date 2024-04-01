IMG_TAG ?= latest

K8UP_MAIN_GO ?= cmd/k8up/main.go
K8UP_GOOS ?= linux
K8UP_GOARCH ?= amd64

CURDIR ?= $(shell pwd)
BIN_FILENAME ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/k8up
WORK_DIR = $(CURDIR)/.work

integrationtest_dir ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/.integration-test
e2etest_dir ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/.e2e-test

go_bin ?= $(PWD)/.work/bin
$(go_bin):
	@mkdir -p $@

golangci_bin = $(go_bin)/golangci-lint

CRD_FILE ?= k8up-crd.yaml
CRD_ROOT_DIR ?= config/crd/apiextensions.k8s.io
CRD_DOCS_REF_PATH ?= docs/modules/ROOT/pages/references/api-reference.adoc

SAMPLES_ROOT_DIR ?= config/samples
minio_sentinel = $(e2etest_dir)/minio_sentinel

KIND_NODE_VERSION ?= v1.26.6
KIND_KUBECONFIG ?= $(e2etest_dir)/kind-kubeconfig-$(KIND_NODE_VERSION)
KIND_CLUSTER ?= k8up-$(KIND_NODE_VERSION)
KIND_KUBECTL_ARGS ?= --validate=true

ENABLE_LEADER_ELECTION ?= false

E2E_TAG ?= e2e
E2E_REGISTRY = local.dev
E2E_REPO ?= k8up-io/k8up
K8UP_E2E_IMG = $(E2E_REGISTRY)/$(E2E_REPO):$(E2E_TAG)

BATS_FILES ?= .

# Image URL to use all building/pushing image targets
K8UP_GHCR_IMG ?= ghcr.io/k8up-io/k8up:$(IMG_TAG)
K8UP_QUAY_IMG ?= quay.io/k8up-io/k8up:$(IMG_TAG)

# Operator Integration Test
ENVTEST_ADDITIONAL_FLAGS ?= --bin-dir "$(go_bin)"
INTEGRATION_TEST_DEBUG_OUTPUT ?= false
# See https://storage.googleapis.com/kubebuilder-tools/ for list of supported K8s versions
ENVTEST_K8S_VERSION = 1.26.x
