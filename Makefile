# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:
.DEFAULT_GOAL := help

PROJECT_ROOT_DIR = .
include Makefile.vars.mk tools/tools.mk
include Makefile.restic-integration.mk
# Chart-related
-include charts/charts.mk

e2e_make := $(MAKE) -C e2e
go_build ?= go build -o $(BIN_FILENAME) $(K8UP_MAIN_GO)

all: build ## Invokes the build target

.PHONY: test
test: ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: integration-test
# operator module {
# See https://storage.googleapis.com/kubebuilder-tools/ for list of supported K8s versions
integration-test: export ENVTEST_K8S_VERSION = 1.22.x
integration-test: export KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT = $(INTEGRATION_TEST_DEBUG_OUTPUT)
# }
# restic module {
integration-test: export RESTIC_BINARY = $(restic_path)
integration-test: export RESTIC_PASSWORD = $(restic_password)
integration-test: export RESTIC_REPOSITORY = s3:http://$(minio_address)/test
integration-test: export AWS_ACCESS_KEY_ID = $(minio_root_user)
integration-test: export AWS_SECRET_ACCESS_KEY = $(minio_root_password)
integration-test: export RESTORE_S3ENDPOINT = http://$(minio_address)/restore
integration-test: export RESTORE_ACCESSKEYID = $(minio_root_user)
integration-test: export RESTORE_SECRETACCESSKEY = $(minio_root_password)
integration-test: export BACKUP_DIR = $(backup_dir)
integration-test: export RESTORE_DIR = $(restore_dir)
integration-test: export STATS_URL = $(stats_url)
# }
integration-test: generate $(integrationtest_dir_created) restic-integration-test-setup $(SETUP_ENVTEST_BIN) ## Run integration tests with envtest
	$(SETUP_ENVTEST_BIN) $(ENVTEST_ADDITIONAL_FLAGS) use '$(ENVTEST_K8S_VERSION)!'
	export KUBEBUILDER_ASSETS="$$($(SETUP_ENVTEST_BIN) $(ENVTEST_ADDITIONAL_FLAGS) use -i -p path '$(ENVTEST_K8S_VERSION)!')"; \
		env | grep KUBEBUILDER; \
		go test -tags=integration -coverprofile cover.out  ./...

.PHONY: build
build: generate fmt vet $(BIN_FILENAME) docs-update-usage ## Build manager binary

.PHONY: run
run: export BACKUP_ENABLE_LEADER_ELECTION = $(ENABLE_LEADER_ELECTION)
run: export K8UP_DEBUG = true
run: export BACKUP_OPERATOR_NAMESPACE = default
run: fmt vet ## Run against the configured Kubernetes cluster in ~/.kube/config. Use ARGS to pass arguments to the command, e.g. `make run ARGS="--help"`
	go run $(K8UP_MAIN_GO) $(ARGS) $(CMD) $(CMD_ARGS)

.PHONY: run-operator
run-operator: CMD := operator
run-operator: run  ## Run the operator module against the configured Kubernetes cluster in ~/.kube/config. Use ARGS to pass arguments to the command, e.g. `make run-operator ARGS="--debug" CMD_ARGS="--help"`

.PHONY: run-restic
run-restic: CMD := restic
run-restic: run  ## Run the restic module. Use ARGS to pass arguments to the command, e.g. `make run-restic ARGS="--debug" CMD_ARGS="--check"`

.PHONY: install
install: export KUBECONFIG = $(KIND_KUBECONFIG)
install: generate kind-setup ## Install CRDs into a cluster
	kubectl apply $(KIND_KUBECTL_ARGS) -f $(CRD_ROOT_DIR)/v1

.PHONY: uninstall
uninstall: export KUBECONFIG = $(KIND_KUBECONFIG)
uninstall: generate kind-setup ## Uninstall CRDs from a cluster
	kubectl delete -f $(CRD_ROOT_DIR)/v1

deploy_args =

.PHONY: deploy
deploy: export KUBECONFIG = $(KIND_KUBECONFIG)
deploy: kind-load-image install ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	helm upgrade --install k8up charts/k8up \
		--create-namespace \
		--namespace k8up-system \
		--set podAnnotations.imagesha="$(shell docker image inspect $(K8UP_E2E_IMG) | jq -r '.[].Id')" \
		--set image.pullPolicy=IfNotPresent \
		--set image.registry=$(E2E_REGISTRY) \
		--set image.repository=$(E2E_REPO) \
		--set image.tag=$(E2E_TAG) \
		--set k8up.backupImage.repository=$(E2E_REGISTRY)/$(E2E_REPO) \
		--set k8up.backupImage.tag=$(E2E_TAG) \
		--wait $(deploy_args)

.PHONY: generate
generate: ## Generate manifests e.g. CRD, RBAC etc.
	# Generate code
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=".github/boilerplate.go.txt" paths="./..."
	# Generate CRDs
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=$(CRD_ROOT_DIR)/v1 crd:crdVersions=v1

.PHONY: crd
crd: generate ## Generate CRD to file
	@cat $(CRD_ROOT_DIR)/v1/*.yaml | yq > $(CRD_FILE)

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: lint
lint: generate fmt vet docs-generate ## Invokes the fmt and vet targets
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

.PHONY: docker-build
docker-build: $(BIN_FILENAME) ## Build the docker image
	docker build . \
		--tag $(K8UP_QUAY_IMG) \
		--tag $(K8UP_GHCR_IMG) \
		--tag $(K8UP_E2E_IMG)

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(K8UP_QUAY_IMG)
	docker push $(K8UP_GHCR_IMG)

clean: export KUBECONFIG = $(KIND_KUBECONFIG)
clean: restic-integration-test-clean e2e-clean docs-clean ## Cleans up the generated resources
# setup-envtest removes write permission from the files it generates, so they have to be restored in order to delete the directory
	chmod +rwx -R -f $(integrationtest_dir) || true

	rm -rf $(e2etest_dir) $(integrationtest_dir) dist/ bin/ cover.out $(BIN_FILENAME) $(WORK_DIR)

.PHONY: release-prepare
release-prepare: crd ## Prepares artifacts for releases

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

###
### Assets
###

# Build the binary without running generators
.PHONY: $(BIN_FILENAME)
$(BIN_FILENAME): export CGO_ENABLED = 0
$(BIN_FILENAME): export GOOS = $(K8UP_GOOS)
$(BIN_FILENAME): export GOARCH = $(K8UP_GOARCH)
$(BIN_FILENAME):
	$(go_build)

$(integrationtest_dir_created):
	mkdir -p $(integrationtest_dir)
# a marker file must be created, because the date of the
# directory may update when content in it is created/updated,
# which would cause a rebuild / re-initialization of dependants
	@touch $(integrationtest_dir_created)

###
### KIND
###

.PHONY: kind-setup
kind-setup: ## Creates a kind instance if one does not exist yet.
	@$(e2e_make) kind-setup

.PHONY: kind-clean
kind-clean: ## Removes the kind instance if it exists.
	@$(e2e_make) kind-clean

.PHONY: kind-run
kind-run: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-run: kind-setup kind-minio install run-operator ## Runs the operator on the local host but configured for the kind cluster

kind-load-image: docker-build
	$(e2e_make) kind-load-image

.PHONY: kind-minio
kind-minio: $(minio_sentinel)

$(minio_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(minio_sentinel): kind-setup
	kubectl apply -f $(SAMPLES_ROOT_DIR)/deployments/minio.yaml
	@touch $@
###
### E2E Test
###

.PHONY: e2e-test
e2e-test: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-test: export BATS_FILES := $(BATS_FILES)
e2e-test: e2e-setup docker-build install ## Run the e2e tests
	@$(e2e_make) test

.PHONY: e2e-setup
e2e-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-setup: ## Run the e2e setup
	@$(e2e_make) setup

.PHONY: e2e-clean-setup
e2e-clean-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-clean-setup: ## Clean the e2e setup (e.g. to rerun the e2e-setup)
	@$(e2e_make) clean-setup

.PHONY: e2e-clean
e2e-clean: ## Remove all e2e-related resources (incl. all e2e Docker images)
	@$(e2e_make) clean

###
### Documentation
###

include ./docs/docs.mk
