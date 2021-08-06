IMG_TAG ?= latest

CURDIR ?= $(shell pwd)
BIN_FILENAME ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/k8up

integrationtest_dir ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/.integration-test
integrationtest_dir_created = $(integrationtest_dir)/.created
e2etest_dir ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/.integration-test
e2etest_dir_created = $(e2etest_dir)/.created

CRD_FILE ?= k8up-crd.yaml
CRD_FILE_LEGACY ?= k8up-crd-legacy.yaml
CRD_ROOT_DIR ?= config/crd/apiextensions.k8s.io
CRD_SPEC_VERSION ?= v1
CRD_DOCS_REF_PATH ?= docs/modules/ROOT/pages/references/api-reference.adoc

KIND_NODE_VERSION ?= v1.20.0
KIND ?= go run sigs.k8s.io/kind
KIND_KUBECONFIG ?= $(CURDIR)/$(e2etest_dir)/kind-kubeconfig-$(KIND_NODE_VERSION)
KIND_CLUSTER ?= k8up-$(KIND_NODE_VERSION)
KIND_KUBECTL_ARGS ?= --validate=true

ENABLE_LEADER_ELECTION ?= false

SHASUM ?= $(shell command -v sha1sum > /dev/null && echo "sha1sum" || echo "shasum -a1")
E2E_TAG ?= e2e_$(shell $(SHASUM) $(BIN_FILENAME) | cut -b-8)
E2E_REPO ?= local.dev/k8up
K8UP_E2E_IMG = $(E2E_REPO)/k8up:$(E2E_TAG)
WRESTIC_E2E_IMG = $(E2E_REPO)/wrestic:$(E2E_TAG)

BATS_FILES ?= .

KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v3

# Image URL to use all building/pushing image targets
K8UP_DOCKER_IMG ?= docker.io/vshn/k8up:$(IMG_TAG)
K8UP_QUAY_IMG ?= quay.io/vshn/k8up:$(IMG_TAG)
WRESTIC_DOCKER_IMG ?= docker.io/vshn/wrestic:$(IMG_TAG)
WRESTIC_QUAY_IMG ?= quay.io/vshn/wrestic:$(IMG_TAG)

# Operator Integration Test
ENVTEST_ADDITIONAL_FLAGS ?= --bin-dir "$(integrationtest_dir)"
INTEGRATION_TEST_DEBUG_OUTPUT ?= false
setup-envtest ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest $(ENVTEST_ADDITIONAL_FLAGS)
