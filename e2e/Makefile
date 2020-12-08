
# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

install_bats:
	@npm install

run_bats: export KUBECONFIG = $(KIND_KUBECONFIG)
run_bats:
	@mkdir -p debug || true
	@node_modules/.bin/bats .

clean:
	rm -r debug node_modules || true
