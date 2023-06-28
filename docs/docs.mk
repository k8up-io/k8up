# Determine whether to use podman
#
# podman currently fails when executing in GitHub actions on Ubuntu LTS 20.04,
# so we never use podman if GITHUB_ACTIONS==true.
use_podman := $(shell command -v podman 2>&1 >/dev/null; p="$$?"; \
		if [ "$${GITHUB_ACTIONS}" != "true" ]; then echo "$$p"; else echo 1; fi)

ifeq ($(use_podman),0)
	engine_cmd  ?= podman
	engine_opts ?= --rm --userns=keep-id
else
	engine_cmd  ?= docker
	engine_opts ?= --rm --user "$$(id -u)"
endif

use_go := $(shell command -v go 2>&1 >/dev/null; echo "$$?")

ifeq ($(use_go),0)
	go_cmd ?= go
else
	go_cmd ?= $(engine_cmd) run $(engine_opts) -e GOCACHE=/k8up/.cache -w /k8up --volume "$${PWD}:/k8up" golang:1.20 go
endif

orphans_cmd ?= $(engine_cmd) run $(engine_opts) --volume "$${PWD}:/antora" ghcr.io/vshn/antora-nav-orphans-checker:1.1 -antoraPath /antora/docs
vale_cmd ?= $(engine_cmd) run $(engine_opts) --volume "$${PWD}"/docs/modules/ROOT/pages:/pages --workdir /pages ghcr.io/vshn/vale:2.15.5 --minAlertLevel=error .
preview_cmd ?= $(engine_cmd) run --rm --publish 35729:35729 --publish 2020:2020 --volume "${PWD}":/preview/antora ghcr.io/vshn/antora-preview:3.1.2.3 --antora=docs --style=k8up

.PHONY: docs-check
docs-check: ## Runs vale against the docs to check style
	$(orphans_cmd)
	$(vale_cmd)

.PHONY: docs-preview
docs-preview: ## Start documentation preview at http://localhost:2020 with Live Reload
	$(preview_cmd)

.PHONY: docs-generate
docs-generate: docs-update-usage docs-generate-api

docs_usage_dir ?= docs/modules/ROOT/examples/usage
.PHONY: docs-update-usage
docs-update-usage: ## Generates dumps from `k8up --help`, which are then included as part of the docs
	$(go_cmd) run $(K8UP_MAIN_GO) --help > "$(docs_usage_dir)/k8up.txt"
	$(go_cmd) run $(K8UP_MAIN_GO) restic --help > "$(docs_usage_dir)/restic.txt"
	$(go_cmd) run $(K8UP_MAIN_GO) operator --help > "$(docs_usage_dir)/operator.txt"

## CRD API doc generator
.PHONY: docs-generate-api
docs-generate-api:  ## Generates API reference documentation
	$(go_cmd) run github.com/elastic/crd-ref-docs@latest --source-path=api/v1 --config=docs/api-gen-config.yaml --renderer=asciidoctor --templates-dir=docs/api-templates --output-path=$(CRD_DOCS_REF_PATH)

