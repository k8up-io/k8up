# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:

PROJECT_ROOT_DIR = ..
include ../Makefile.vars.mk
include kind.mk

uname_s := $(shell uname -s)
ifeq ($(uname_s),Linux)
	xargs := xargs --no-run-if-empty
else
	xargs := xargs
endif

bats := node_modules/.bin/bats

.DEFAULT_GOAL := help

.PHONY: test
test: export KUBECONFIG = $(KIND_KUBECONFIG)
test: export E2E_IMAGE = $(E2E_IMG)
test: setup ## Run the E2E tests
	@mkdir -p debug
	@$(KIND) load docker-image --name $(KIND_CLUSTER) $(E2E_IMG)
	@$(bats) .

.PHONY: setup
setup: export KUBECONFIG = $(KIND_KUBECONFIG)
setup: $(bats) kind-setup ## Run the e2e setup

.PHONY: clean-setup
clean-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
clean-setup: kind-clean ## Clean the e2e setup (e.g. to rerun the setup)

.PHONY: clean
clean: clean-setup ## Remove all e2e-related resources (incl. all e2e Docker images)
	docker images --filter "reference=$(E2E_REPO)" --format "{{.Repository }}:{{ .Tag }}" | $(xargs) docker rmi || true
	rm -r debug node_modules || true

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

###
### Artifacts
###

$(bats):
	@npm install
