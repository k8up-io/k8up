MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.SUFFIXES:

rbac_gen_src = ../../config/rbac/role.yaml
rbac_gen_tgt = templates/operator-clusterrole.yaml

ifeq ($(shell uname -s),Darwin)
	sed := gsed -i
else
	sed := sed -i
endif

$(rbac_gen_tgt):
	@cp $(rbac_gen_src) $@
	@yq -i e '.metadata.name="{{ include \"k8up.fullname\" . }}-manager", del(.metadata.creationTimestamp)' $@
	@yq -i e '.metadata.labels.replace="LABELS"' $@
	@$(sed) -e 's/replace: LABELS/{{- include "k8up.labels" . | nindent 4 }}/g' $@
	@$(sed) -e '1s/^/{{- if .Values.rbac.create -}}\n/' $@
	@$(sed) -e '$$a{{- end -}}' $@

.PHONY: prepare
prepare: $(rbac_gen_tgt) ## Helmify generated artifacts

.PHONY: clean
clean: ## Clean generated artifacts
	rm -rf $(rbac_gen_tgt)
