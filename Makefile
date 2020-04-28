CODE_GENERATOR_IMAGE := slok/kube-code-generator:v1.11.2
DIRECTORY := $(PWD)
CODE_GENERATOR_PACKAGE := github.com/vshn/k8up

pages   := $(shell find . -type f -name '*.adoc')
out_dir := ./_archive
web_dir := ./_public

docker_cmd  ?= docker
docker_opts ?= --rm --tty --user "$$(id -u)"

antora_cmd  ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/antora vshn/antora:2.3.0
antora_opts ?= --cache-dir=.cache/antora

vale_cmd ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}"/docs/modules/ROOT/pages:/pages vshn/vale:2.1.1 --minAlertLevel=error /pages

generate:
	docker run --rm -it \
	-v $(DIRECTORY):/go/src/$(CODE_GENERATOR_PACKAGE) \
	-e PROJECT_PACKAGE=$(CODE_GENERATOR_PACKAGE) \
	-e CLIENT_GENERATOR_OUT=$(CODE_GENERATOR_PACKAGE)/client/k8s \
	-e APIS_ROOT=$(CODE_GENERATOR_PACKAGE)/apis \
	-e GROUPS_VERSION="backup:v1alpha1" \
	-e GENERATION_TARGETS="deepcopy,client" \
	$(CODE_GENERATOR_IMAGE)

UNAME := $(shell uname)
ifeq ($(UNAME), Linux)
	OS = linux-x64
	OPEN = xdg-open
endif
ifeq ($(UNAME), Darwin)
	OS = darwin-x64
	OPEN = open
endif

# This will clean the Antora Artifacts, nothing else
.PHONY: clean
clean:
	rm -rf $(out_dir) $(web_dir)

.PHONY: open
open: $(web_dir)/index.html
	-$(OPEN) $<

.PHONY: html
html:    $(web_dir)/index.html

$(web_dir)/index.html: playbook.yml $(pages)
	$(antora_cmd) $(antora_opts) $<

.PHONY: check
check:
	$(vale_cmd)


