#!/bin/bash

export MINIO_NAMESPACE=${MINIO_NAMESPACE-minio}

errcho() {
	echo >&2 "${@}"
}

if [ -z "${E2E_IMAGE}" ]; then
	errcho "The environment variable 'E2E_IMAGE' is undefined or empty."
	exit 1
fi

timestamp() {
	date +%s
}

require_args() {
	if [ "${#}" != 2 ]; then
		errcho "$0 expected 2 arguments, got ${#}."
		exit 1
	fi

	if [ "${1}" != "${2}" ]; then
		errcho "Expected ${1} arguments, got ${2}."
		exit 1
	fi
}

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
		--timeout 3s \
		--quiet \
		--command -- \
		restic \
		--no-cache \
		--repo "s3:http://minio.minio.svc.cluster.local:9000/backup" \
		"${@}" \
		--json
}

replace_in_file() {
	require_args 3 ${#}

	local file var_name var_value
	file=${1}
	var_name=${2}
	var_value=${3}

	sed -i \
		-e "s|\$${var_name}|${var_value}|" \
		"${file}"
}

prepare() {
	require_args 1 ${#}

	local definition_dir target_dir target_file
	definition_dir=${1}
	target_dir="debug/${definition_dir}"
	target_file="${target_dir}/main.yml"

	mkdir -p "${target_dir}"
	kustomize build "${definition_dir}" -o "${target_file}"

	replace_in_file "${target_file}" E2E_IMAGE "'${E2E_IMAGE}'"
	replace_in_file "${target_file}" WRESTIC_IMAGE "'${WRESTIC_IMAGE}'"
	replace_in_file "${target_file}" ID "$(id -u)"
	replace_in_file "${target_file}" BACKUP_ENABLE_LEADER_ELECTION "'${BACKUP_ENABLE_LEADER_ELECTION}'"
	replace_in_file "${target_file}" BACKUP_FILE_NAME "${BACKUP_FILE_NAME}"
	replace_in_file "${target_file}" BACKUP_FILE_CONTENT "${BACKUP_FILE_CONTENT}"
}

apply() {
	require_args 1 ${#}

	prepare "${1}"
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
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	apply definitions/subject
	echo "✅  The subject is ready"
}

given_an_annotated_subject() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	apply definitions/annotated-subject
	echo "✅  The annotated subject is ready"
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
	apply definitions/operator

	NAMESPACE=k8up-system \
		wait_until deployment/k8up-operator available
	echo "✅  A running operator is ready"
}

given_an_existing_backup() {
	require_args 2 ${#}

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
	require_args 2 ${#}

	local object condition ns
	object=${1}
	condition=${2}
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	echo "Waiting for '${object}' in namespace '${ns}' to become '${condition}' ..."
	kubectl -n "${ns}" wait --timeout 1m --for "condition=${condition}" "${object}"
}

expect_file_in_container() {
	require_args 4 ${#}

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
