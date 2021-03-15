#!/bin/bash

export MINIO_NAMESPACE=${MINIO_NAMESPACE-minio}

errcho() {
	echo >&2 "${@}"
}

timestamp() {
	date +%s
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
	clear_pv_data
}

teardown() {
	cp -r /tmp/detik debug || true
}

clear_pv_data() {
	rm -rfv ./debug/data/pvc-subject
	mkdir -p ./debug/data/pvc-subject
}

kustomize() {
	go run sigs.k8s.io/kustomize/kustomize/v3 "${@}"
}

restic() {
	kubectl run "wrestic-$(timestamp)" \
		--rm \
		--attach \
		--restart Never \
		--wait \
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
	local minio_access_key minio_secret_key minio_url
	minio_access_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.accesskey}" | base64 --decode)
	minio_secret_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.secretkey}" | base64 --decode)
	minio_url=http://${minio_access_key}:${minio_secret_key}@minio.minio.svc.cluster.local:9000

	kubectl run "minio-$(timestamp)" \
		--rm \
		--attach \
		--stdin \
		--restart Never \
		--wait=true \
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
	if [ "${#}" != 3 ]; then
		errcho "$0 Expected 3 arguments, got ${#}."
		exit 1
	fi

	local file var_name var_value
	file=${1}
	var_name=${2}
	var_value=${3}

	sed -i \
		-e "s|\$${var_name}|${var_value}|" \
		"${file}"
}

prepare() {
	DEFINITION_DIR=${1}
	mkdir -p "debug/${DEFINITION_DIR}"

	local target_file
	target_file="debug/${DEFINITION_DIR}/main.yml"

	kustomize build "${DEFINITION_DIR}" -o "${target_file}"

	replace_in_file "${target_file}" E2E_IMAGE "'${E2E_IMAGE}'"
	replace_in_file "${target_file}" WRESTIC_IMAGE "'${WRESTIC_IMAGE}'"
	replace_in_file "${target_file}" ID "$(id -u)"
	replace_in_file "${target_file}" BACKUP_ENABLE_LEADER_ELECTION "'${BACKUP_ENABLE_LEADER_ELECTION}'"
	replace_in_file "${target_file}" BACKUP_FILE_NAME "${BACKUP_FILE_NAME}"
	replace_in_file "${target_file}" BACKUP_FILE_CONTENT "${BACKUP_FILE_CONTENT}"
}

apply() {
	prepare "${@}"
	kubectl apply -f "debug/${1}/main.yml"
}

given_a_clean_ns() {
	kubectl delete namespace "${DETIK_CLIENT_NAMESPACE}" --ignore-not-found
	kubectl delete pv subject-pv --ignore-not-found
	clear_pv_data
	kubectl create namespace "${DETIK_CLIENT_NAMESPACE}"
	echo "✅  The namespace '${DETIK_CLIENT_NAMESPACE}' is ready."
}

given_a_subject() {
	if [ "${#}" != 2 ]; then
		errcho "$0 Expected 2 arguments, got ${#}."
		exit 1
	fi

	export BACKUP_FILE_NAME=$1
	export BACKUP_FILE_CONTENT=$2

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

given_an_existing_backup() {
	if [ "${#}" != 2 ]; then
		errcho "$0 Expected 2 arguments, got ${#}."
		exit 1
	fi

	local backup_file_name backup_file_content
	backup_file_name=${1}
	backup_file_content=${2}
	given_a_subject "${backup_file_name}" "${backup_file_content}"

	apply definitions/backup
	wait_until backup/k8up-k8up-backup completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Backup named 'k8up-k8up-backup'"

	run restic dump latest "/data/subject-pvc/${backup_file_name}"
	# shellcheck disable=SC2154
	[ "${backup_file_content}" = "${output}" ]

	echo "✅  An existing backup is ready"
}

wait_until() {
	local object condition ns
	object=${1}
	condition=${2}
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	echo "Waiting for '${object}' in namespace '${ns}' to become '${condition}' ..."
	kubectl -n "${ns}" wait --timeout 1m --for "condition=${condition}" "${object}"
}

expect_file_in_container() {
	if [ "${#}" != 4 ]; then
		errcho "$0 Expected 4 arguments, got ${#}."
		exit 1
	fi

	local pod container expected_file expected_content
	pod=${1}
	container=${2}
	expected_file=${3}
	expected_content=${4}

	commands=(
		"ls -la \"$(dirname "${expected_file}")\""
		"test -f \"${expected_file}\""
		"cat \"${expected_file}\""
		"test \"${expected_content}\" \"=\" \"\$(cat \"${expected_file}\")\" "
	)

	echo "Testing if file '${expected_file}' contains '${expected_content}' in container '${container}' of pod '${pod}':"

	for cmd in "${commands[@]}"; do
		echo "> by running the command \`sh -c '${cmd}'\`."
		kubectl exec \
			"${pod}" \
			--container "${container}" \
			--stdin \
			--namespace "${DETIK_CLIENT_NAMESPACE}" \
			-- sh -c "${cmd}"
		echo '↩'
	done
}
