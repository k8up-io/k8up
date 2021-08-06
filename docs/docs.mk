pages   := $(shell find . -type f -name '*.adoc')
out_dir := ./_public

docker_cmd  ?= docker
docker_opts ?= --rm --tty --user "$$(id -u)"

antora_cmd  ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/antora vshn/antora:2.3.3
antora_opts ?= --cache-dir=.cache/antora

asciidoctor_cmd  ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/documents/ asciidoctor/docker-asciidoctor asciidoctor
asciidoctor_pdf_cmd  ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/documents/ vshn/asciidoctor-pdf:1.8.1 --attribute toclevels=1
asciidoctor_epub3_cmd  ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/documents/ vshn/asciidoctor-epub3:1.8.1 --attribute toclevels=1
asciidoctor_opts ?= --destination-dir=$(out_dir)
asciidoctor_kindle_opts ?= --attribute ebook-format=kf8

vale_cmd ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}"/src/modules/ROOT/pages:/pages vshn/vale:2.6.1 --minAlertLevel=error /pages
hunspell_cmd ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}":/spell vshn/hunspell:1.7.0 -d en,vshn -l -H _public/**/**/*.html
htmltest_cmd ?= $(docker_cmd) run $(docker_opts) --volume "$${PWD}"/_public:/test wjdp/htmltest:v0.12.0
preview_cmd ?= $(docker_cmd) run --rm --publish 35729:35729 --publish 2020:2020 --volume "${PWD}":/preview/antora vshn/antora-preview:2.3.8 --antora=docs --style=k8up

.PHONY: docs-all
docs-all: docs-html docs-pdf ## Generate HTML and PDF docs
docs-documents: docs-pdf docs-manpage docs-kindle docs-epub ## Generate downloadable docs

# This will clean the Antora Artifacts, not the npm artifacts
.PHONY: docs-clean
docs-clean: ## Cleans Antora artifacts
	rm -rf $(out_dir) '?' .cache

.PHONY: docs-check
docs-check: ## Runs vale against the docs to check style
	$(vale_cmd)

.PHONY: docs-syntax
docs-syntax: docs-html ## Runs hunspell against the docs
	$(hunspell_cmd)

.PHONY: docs-htmltest
docs-htmltest: docs-html docs-pdf docs-epub docs-kindle docs-manpage ## Runs htmltest against the docs
	$(htmltest_cmd)

.PHONY: docs-preview
docs-preview: ## Start documentation preview at http://localhost:2020 with Live Reload
	$(preview_cmd)

.PHONY: docs-html-open
docs-html-open: docs-html ## Start documentation preview at http://localhost:2020 with Live Reload and open in browser
	$(shell xdg-open _public/index.html)

.PHONY: docs-html docs-pdf docs-manpage docs-epub docs-kindle
docs-html:    $(out_dir)/index.html ## Generate HTML version of documentation with Antora, output at ./_public/
docs-pdf:     $(out_dir)/k8up.pdf ## Generate PDF version of documentation with Antora, output at ./_public/
docs-manpage: $(out_dir)/k8up.1 ## Generate Manpage version of documentation with Antora, output at ./_public/
docs-epub:    $(out_dir)/k8up.epub ## Generate epub version of documentation with Antora, output at ./_public/
docs-kindle:  $(out_dir)/k8up-kf8.epub ## Generate Kindle version of documentation with Antora, output at ./_public/

$(out_dir)/index.html: playbook.yml $(pages)
	$(antora_cmd) $(antora_opts) $<

$(out_dir)/%.1: docs/%.adoc $(pages)
	$(asciidoctor_cmd) --backend=manpage --attribute doctype=manpage $(asciidoctor_opts) $<

$(out_dir)/%.pdf: docs/%.adoc $(pages)
	$(asciidoctor_pdf_cmd) $(asciidoctor_opts) $<

$(out_dir)/%.epub: docs/%.adoc $(pages)
	$(asciidoctor_epub3_cmd) $(asciidoctor_opts) $<

$(out_dir)/%-kf8.epub: docs/%.adoc $(pages)
	$(asciidoctor_epub3_cmd) $(asciidoctor_kindle_opts) $(asciidoctor_opts) $<
