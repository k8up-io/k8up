#!/bin/bash

export MINIO_NAMESPACE=${MINIO_NAMESPACE-minio}

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
		--image "${WRESTIC_IMAGE}" \
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

mc() {
	minio_access_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.accesskey}" | base64 --decode)
	minio_secret_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.secretkey}" | base64 --decode)
	minio_url=http://${minio_access_key}:${minio_secret_key}@minio.minio.svc.cluster.local:9000
	kubectl run minio \
		--rm \
		--attach \
		--stdin \
		--restart Never \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${MINIO_IMAGE-minio/mc:latest}" \
		--env "MC_HOST_s3=${minio_url}" \
		--pod-running-timeout 10s \
		--quiet=true \
		--command -- \
			mc \
			"${@}"
}

replace_in_file() {
	VAR_NAME=${1}
	VAR_VALUE=${2}
	FILE=${3}

	sed -i \
		-e "s|\$${VAR_NAME}|${VAR_VALUE}|" \
		"${FILE}"
}

prepare() {
	DEFINITION_DIR=${1}
	mkdir -p "debug/${DEFINITION_DIR}"
	kustomize build "${DEFINITION_DIR}" -o "debug/${DEFINITION_DIR}/main.yml"

	replace_in_file E2E_IMAGE "'${E2E_IMAGE}'" "debug/${DEFINITION_DIR}/main.yml"
	replace_in_file WRESTIC_IMAGE "'${WRESTIC_IMAGE}'" "debug/${DEFINITION_DIR}/main.yml"
	replace_in_file ID "$(id -u)" "debug/${DEFINITION_DIR}/main.yml"
	replace_in_file BACKUP_ENABLE_LEADER_ELECTION "'${BACKUP_ENABLE_LEADER_ELECTION}'" "debug/${DEFINITION_DIR}/main.yml"
}

apply() {
	prepare "${@}"
	kubectl apply -f "debug/${1}/main.yml"
}

given_a_clean_ns() {
	kubectl delete namespace "${DETIK_CLIENT_NAMESPACE}" --ignore-not-found
	kubectl delete pv subject-pv --ignore-not-found
	kubectl create namespace "${DETIK_CLIENT_NAMESPACE}" || true
	echo "✅  The namespace '${DETIK_CLIENT_NAMESPACE}' is ready."
}

given_a_subject() {
	apply definitions/subject
	echo "✅  The subject is ready"
}

given_s3_storage() {
	helm repo add minio https://helm.min.io/
	helm repo update
	helm upgrade --install minio \
		--values definitions/minio/helm.yaml \
		--create-namespace \
		--namespace "${MINIO_NAMESPACE}" \
		minio/minio

	echo "✅  S3 Storage is ready"
}

given_a_running_operator() {
	apply definitions/k8up

	NAMESPACE=k8up-system \
		wait_until deployment/k8up-operator available
	echo "✅  A running operator is ready"
}

wait_until() {
	object=${1}
	condition=${2}
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}
	echo "Waiting for '${object}' in namespace '${ns}' to become '${condition}' ..."
	kubectl -n "${ns}" wait --timeout 1m --for "condition=${condition}" "${object}"
}
