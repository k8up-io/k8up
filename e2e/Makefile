clean_targets += e2e-clean

uname_s := $(shell uname -s)
ifeq ($(uname_s),Linux)
	xargs := xargs --no-run-if-empty
else
	xargs := xargs
endif

bats := node_modules/bats/bin/bats
bats_args ?=

.PHONY: e2e-test
e2e-test: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-test: export E2E_IMAGE = $(K8UP_E2E_IMG)
e2e-test: export IMG_REGISTRY = $(E2E_REGISTRY)
e2e-test: export IMG_TAG = $(E2E_TAG)
e2e-test: export IMG_REPO = $(E2E_REPO)
e2e-test: e2e-setup install kind-load-image ## Run the E2E tests
	@cd ./e2e && $(bats) $(bats_args) $(BATS_FILES)

.PHONY: kind-load-image
kind-load-image: kind-setup docker-build ## Load the e2e container image onto e2e cluster
	@$(KIND) load docker-image --name $(KIND_CLUSTER) $(K8UP_E2E_IMG)

.PHONY: e2e-setup
e2e-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-setup: chart-prepare
e2e-setup: e2e/node_modules kind-setup | $(e2etest_dir) ## Run the e2e setup

.PHONY: clean
e2e-clean: kind-clean ## Remove all e2e-related resources (incl. all e2e Docker images)
	docker images --filter "reference=$(E2E_REGISTRY)/$(E2E_REPO)" --format "{{.Repository }}:{{ .Tag }}" | $(xargs) docker rmi || true
	rm -rf e2e/debug e2e/node_modules $(e2etest_dir)

###
### Artifacts
###

$(e2etest_dir):
	mkdir -p $(e2etest_dir)

e2e/node_modules:
	@npm --prefix ./e2e install
