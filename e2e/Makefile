
# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

.PHONY: install_bats
install_bats: node_modules/.bin/bats

.PHONY: run_bats
run_bats: export KUBECONFIG = $(KIND_KUBECONFIG)
run_bats:
	@mkdir -p debug || true
	@node_modules/.bin/bats .

clean:
	rm -r debug node_modules || true

node_modules/.bin/bats: node_modules

node_modules:
	@npm install
