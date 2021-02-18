#!/bin/bash

export WRESTIC_IMAGE=${WRESTIC_IMAGE-quay.io/vshn/wrestic}

errcho() {
	>&2 echo "${@}"
}

if [ -z "${E2E_IMAGE}" ]; then
	errcho "The environment variable 'E2E_IMAGE' is undefined or empty."
	exit 1
fi

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

restic() {
	kubectl run wrestic \
		--rm \
		--attach \
		--restart Never \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${WRESTIC_IMAGE-quay.io/vshn/wrestic}" \
		--env "AWS_ACCESS_KEY_ID=myaccesskey" \
		--env "AWS_SECRET_KEY=mysecretkey" \
		--env "RESTIC_PASSWORD=myreposecret" \
		--pod-running-timeout 10s \
		--quiet=true \
		--command -- \
			restic \
			--no-cache \
			-r "s3:http://minio.minio.svc.cluster.local:9000/backup" \
			"${@}" \
			--json
}

prepare() {
	mkdir -p "debug/${1}"
	kustomize build "${1}" -o "debug/${1}/main.yml"
	sed -i \
		-e "s|\$E2E_IMAGE|'${E2E_IMAGE}'|" \
		"debug/${1}/main.yml"
	sed -i \
		-e "s|\$WRESTIC_IMAGE|'${WRESTIC_IMAGE}'|" \
		"debug/${1}/main.yml"
}

apply() {
	prepare "${@}"
	kubectl apply -f "debug/${1}/main.yml"
}

given_a_subject() {
	kubectl create namespace k8up-e2e-subject || true
	apply definitions/subject
}

given_s3_storage() {
	# TODO HELM REPO
	helm install minio \
		--values definitions/minio/helm.yaml \
		--create-namespace \
		--namespace minio \
		minio/minio
}

given_running_operator() {
	apply definitions/k8up
}
