MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.SUFFIXES:

# git tag of the K8up version to fetch
# defaults to tag specified in values.yaml, as this gets maintained by renovate
APP_VERSION ?= $(shell grep -E "^  tag: " values.yaml | cut -d " " -f 4 | sed s/\"//g)

SED_BIN := sed

REPLACE_RBAC_LABEL := {{- include "k8up.staticLabels" . | nindent 4 }}
REPLACE_NAME_PREFIX := {{ include "k8up.fullname" . }}-
REPLACE_SA_NAME := {{ include "k8up.serviceAccountName" . }}

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = "(: ).*?## "}; {gsub(/\\:/,":",$$1)}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: rbac
rbac: templates/rbac.yaml ## Update the RBAC templates by building them from Kustomize

.PHONY: templates/rbac.yaml
templates/rbac.yaml:
	$(SED_BIN) -i -e 's/?ref=.*/?ref=$(APP_VERSION)/g' rbac-kustomize/kustomization.yaml
	kustomize build rbac-kustomize -o $@
	$(SED_BIN) -i -e '1i{{- if .Values.rbac.create -}}' $@
	$(SED_BIN) -i -e 's/replace: LABELS/$(REPLACE_RBAC_LABEL)/g' $@
	$(SED_BIN) -i -e 's/PREFIX-/$(REPLACE_NAME_PREFIX)/g' $@
	$(SED_BIN) -i -e 's/name: default/name: $(REPLACE_SA_NAME)/g' $@
	$(SED_BIN) -i -e '$$a{{- end }}' $@

.PHONY: appVersion
appVersion: ## Replaces "appVersion" in Chart.yaml with 'image.tag' from values.yaml
	$(SED_BIN) -i -e 's/^appVersion.*/appVersion: $(APP_VERSION)/' Chart.yaml

.PHONY: update
update: rbac appVersion ## Updates all templates

#
# "Interface" for parent Makefile
#
prepare: update ## Prepare helm chart
