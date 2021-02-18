#!/bin/bash

setup() {
	debug "-- $BATS_TEST_DESCRIPTION"
	debug "-- $(date)"
	debug ""
	debug ""
}

setup_file() {
	reset_debug
}

teardown() {
	cp -r /tmp/detik debug || true
}

kustomize() {
	go run sigs.k8s.io/kustomize/kustomize/v3 "${@}"
}

prepare() {
	mkdir -p "debug/${1}"
	kustomize build "${1}" -o "debug/${1}/main.yml"
	sed -i -e "s|\$E2E_IMAGE|'${E2E_IMAGE}'|" "debug/${1}/main.yml"
}

apply() {
	prepare "${@}"
	kubectl apply -f "debug/${1}/main.yml"
}
