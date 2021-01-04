
# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

BATS := node_modules/.bin/bats

.PHONY: install_bats
install_bats: $(BATS)

.PHONY: run_bats
run_bats: export KUBECONFIG = $(KIND_KUBECONFIG)
run_bats: install_bats
	@mkdir -p debug || true
	@$(BATS) .

.PHONY: clean
clean:
	rm -r debug node_modules || true

$(BATS): node_modules

node_modules:
	@npm install
