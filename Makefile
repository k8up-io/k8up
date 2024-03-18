# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E -h '^[^#].+\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# extensible array of targets. Modules can add target to this variable for the all-in-one target.
clean_targets := build-clean

PROJECT_ROOT_DIR = .
include Makefile.vars.mk
include Makefile.restic-integration.mk envtest/integration.mk
# Chart-related
-include charts/charts.mk
# Documentation
-include ./docs/docs.mk
# KIND
-include e2e/kind.mk
# E2E tests
-include e2e/Makefile

go_build ?= go build -o $(BIN_FILENAME) $(K8UP_MAIN_GO)

.PHONY: test
test: ## Run tests
	go test ./... -coverprofile cover.out

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
	kubectl apply $(KIND_KUBECTL_ARGS) -f $(CRD_ROOT_DIR)/v1 --server-side

.PHONY: uninstall
uninstall: export KUBECONFIG = $(KIND_KUBECONFIG)
uninstall: generate kind-setup ## Uninstall CRDs from a cluster
	kubectl delete -f $(CRD_ROOT_DIR)/v1

deploy_args =

.PHONY: deploy
deploy: export KUBECONFIG = $(KIND_KUBECONFIG)
deploy: kind-load-image install ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	helm upgrade --install k8up ./charts/k8up \
		--create-namespace \
		--namespace k8up-system \
		--set podAnnotations.imagesha="$(shell docker image inspect $(K8UP_E2E_IMG) | jq -r '.[].Id')" \
		--set image.pullPolicy=IfNotPresent \
		--set image.registry=$(E2E_REGISTRY) \
		--set image.repository=$(E2E_REPO) \
		--set image.tag=$(E2E_TAG) \
		--values ./e2e/definitions/operator/deploy.yaml \
		--wait $(deploy_args)

.PHONY: generate
generate: ## Generate manifests e.g. CRD, RBAC etc.
	# Generate code
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=".github/boilerplate.go.txt" paths="./..."
	# Generate CRDs
	go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=$(CRD_ROOT_DIR)/v1 crd:crdVersions=v1

.PHONY: crd
crd: generate ## Generate CRD to file
	@yq $(CRD_ROOT_DIR)/v1/*.yaml > $(CRD_FILE)

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: lint
lint: fmt vet golangci-lint ## Invokes all linting targets
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

.PHONY: golangci-lint
golangci-lint: $(golangci_bin) ## Run golangci linters
	$(golangci_bin) run --timeout 5m --out-format colored-line-number ./...

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

build-clean:
	rm -rf dist/ bin/ cover.out $(BIN_FILENAME) $(WORK_DIR) $(CRD_FILE)

clean: $(clean_targets) ## Cleans up all the locally generated resources

.PHONY: release-prepare
release-prepare: crd ## Prepares artifacts for releases

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

$(integrationtest_dir):
	mkdir -p $(integrationtest_dir)

.PHONY: kind-run
kind-run: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-run: kind-setup kind-minio install run-operator ## Runs the operator on the local host but configured for the kind cluster

.PHONY: kind-minio
kind-minio: $(minio_sentinel)

$(minio_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(minio_sentinel): kind-setup
	kubectl apply -f $(SAMPLES_ROOT_DIR)/deployments/minio.yaml
	@touch $@

$(golangci_bin): | $(go_bin)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go_bin)"
