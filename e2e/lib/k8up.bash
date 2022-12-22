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

# Runs before each test file
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

restic() {
	kubectl run "restic-$(timestamp)" \
		--rm \
		--attach \
		--restart Never \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${E2E_IMAGE}" \
		--env "AWS_ACCESS_KEY_ID=myaccesskey" \
		--env "AWS_SECRET_ACCESS_KEY=mysecretkey" \
		--env "RESTIC_PASSWORD=myreposecret" \
		--pod-running-timeout 10s \
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
	cp -r "definitions" "debug/definitions"


	replace_in_file "${target_file}" E2E_IMAGE "'${E2E_IMAGE}'"
	replace_in_file "${target_file}" ID "$(id -u)"
	replace_in_file "${target_file}" BACKUP_FILE_NAME "${BACKUP_FILE_NAME}"
	replace_in_file "${target_file}" BACKUP_FILE_CONTENT "${BACKUP_FILE_CONTENT}"
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

	kubectl apply -f definitions/pv
	yq e '.spec.template.spec.containers[0].securityContext.runAsUser='$(id -u)' | .spec.template.spec.containers[0].env[0].value=strenv(BACKUP_FILE_CONTENT) | .spec.template.spec.containers[0].env[1].value=strenv(BACKUP_FILE_NAME)' definitions/subject/deployment.yaml | kubectl apply -f -

	echo "✅  The subject is ready"
}

given_an_annotated_subject() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	kubectl apply -f definitions/pv
	yq e '.spec.template.spec.containers[0].securityContext.runAsUser='$(id -u)' | .spec.template.spec.containers[0].env[0].value=strenv(BACKUP_FILE_CONTENT) | .spec.template.spec.containers[0].env[1].value=strenv(BACKUP_FILE_NAME)' definitions/annotated-subject/deployment.yaml | kubectl apply -f -

	echo "✅  The annotated subject is ready"
}

given_s3_storage() {
	helm repo add minio https://helm.min.io/ --force-update
	helm repo update
	helm upgrade --install minio \
		--values definitions/minio/helm.yaml \
		--create-namespace \
		--namespace "${MINIO_NAMESPACE}" \
		minio/minio

	echo "✅  S3 Storage is ready"
}

given_a_running_operator() {
	values_src="definitions/operator/values.yaml"
	values_tgt="debug/definitions/operator/values.yaml"
	mkdir -p "$(dirname ${values_tgt})"
	cp "${values_src}" "${values_tgt}"

	replace_in_file ${values_tgt} IMAGE_SHA "$(docker image inspect ${E2E_IMAGE} | jq -r '.[].Id')"
	replace_in_file ${values_tgt} E2E_REGISTRY "${IMG_REGISTRY}"
	replace_in_file ${values_tgt} E2E_REPO "${IMG_REPO}"
	replace_in_file ${values_tgt} E2E_TAG "${IMG_TAG}"

	helm upgrade --install k8up ../charts/k8up \
		--create-namespace \
		--namespace k8up-system \
		--values "${values_tgt}" \
		--wait

	echo "✅  A running operator is ready"
}

given_an_existing_backup() {
	require_args 2 ${#}

	local backup_file_name backup_file_content
	backup_file_name=${1}
	backup_file_content=${2}
	given_a_subject "${backup_file_name}" "${backup_file_content}"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup.yaml | kubectl apply -f -

	wait_until backup/k8up-backup completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Backup named 'k8up-backup'"

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
