# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:

PROJECT_ROOT_DIR = ..
include ../Makefile.vars.mk ../tools/tools.mk
include kind.mk

uname_s := $(shell uname -s)
ifeq ($(uname_s),Linux)
	xargs := xargs --no-run-if-empty
else
	xargs := xargs
endif

bats := node_modules/bats/bin/bats
bats_args ?=

.DEFAULT_GOAL := help

.PHONY: test
test: export KUBECONFIG = $(KIND_KUBECONFIG)
test: export E2E_IMAGE = $(K8UP_E2E_IMG)
test: export IMG_REGISTRY = $(E2E_REGISTRY)
test: export IMG_TAG = $(E2E_TAG)
test: export IMG_REPO = $(E2E_REPO)
test: setup kind-load-image ## Run the E2E tests
	@$(bats) $(bats_args) $(BATS_FILES)

.PHONY: kind-load-image
kind-load-image: kind-setup ## Load the e2e container image onto e2e cluster
	@$(KIND) load docker-image --name $(KIND_CLUSTER) $(K8UP_E2E_IMG)

.PHONY: setup
setup: export KUBECONFIG = $(KIND_KUBECONFIG)
setup: $(bats) $(e2etest_dir_created) kind-setup ## Run the e2e setup

.PHONY: clean-setup
clean-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
clean-setup: kind-clean ## Clean the e2e setup (e.g. to rerun the setup)

.PHONY: clean
clean: clean-setup ## Remove all e2e-related resources (incl. all e2e Docker images)
	docker images --filter "reference=$(E2E_REGISTRY)/$(E2E_REPO)" --format "{{.Repository }}:{{ .Tag }}" | $(xargs) docker rmi || true
	rm -rf debug node_modules || true

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

###
### Artifacts
###

$(e2etest_dir_created):
	mkdir -p $(e2etest_dir)
# a marker file must be created, because the date of the
# directory may update when content in it is created/updated,
# which would cause a rebuild / re-initialization of dependants
	@touch $@

$(bats):
	@npm install
