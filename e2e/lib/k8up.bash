#!/bin/bash

export MINIO_NAMESPACE=${MINIO_NAMESPACE-minio}

directory=$(dirname "${BASH_SOURCE[0]}")
source "$directory/detik.bash"

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
		errcho "$0 expected 2 arguments, got ${#} (${FUNCNAME[1]})."
		exit 1
	fi

	if [ "${1}" != "${2}" ]; then
		errcho "Expected ${1} arguments, got ${2} (${FUNCNAME[1]})."
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

# We're not using `kubectl run --attach` here, to get the output of the pod.
# It's very unreliable unfortunately. So running the pod, waiting and getting the
# log output is a lot less prone for race conditions.
restic() {
	podname="restic-$(timestamp)"
	kubectl run "$podname" \
		--restart Never \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${E2E_IMAGE}" \
		--env "AWS_ACCESS_KEY_ID=minioadmin" \
		--env "AWS_SECRET_ACCESS_KEY=minioadmin" \
		--env "RESTIC_PASSWORD=myreposecret" \
		--pod-running-timeout 60s \
		--quiet \
		--command -- \
		restic \
		--no-cache \
		--repo "s3:http://minio.minio.svc.cluster.local:9000/backup" \
		"${@}" \
		--json > /dev/null
	kubectl wait --for jsonpath='{.status.phase}'=Succeeded pod "$podname" -n "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" --timeout=2m > /dev/null
	kubectl -n "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" logs "$podname"
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
	kubectl delete pvc subject-pvc --ignore-not-found
	clear_pv_data
	kubectl create namespace "${DETIK_CLIENT_NAMESPACE}"
	echo "✅  The namespace '${DETIK_CLIENT_NAMESPACE}' is ready."
}

given_a_subject() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	kubectl apply -f definitions/pv/pvc.yaml
	yq e '.spec.template.spec.containers[0].securityContext.runAsUser='$(id -u)' | .spec.template.spec.containers[0].env[0].value=strenv(BACKUP_FILE_CONTENT) | .spec.template.spec.containers[0].env[1].value=strenv(BACKUP_FILE_NAME)' definitions/subject/deployment.yaml | kubectl apply -f -

	echo "✅  The subject is ready"
}

given_an_annotated_subject() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	kubectl apply -f definitions/pv/pvc.yaml
	yq e '.spec.template.spec.containers[1].securityContext.runAsUser='$(id -u)' | .spec.template.spec.containers[1].env[0].value=strenv(BACKUP_FILE_CONTENT) | .spec.template.spec.containers[1].env[1].value=strenv(BACKUP_FILE_NAME)' definitions/annotated-subject/deployment.yaml | kubectl apply -f -

	echo "✅  The annotated subject is ready"
}

given_an_annotated_subject_pod() {
  require_args 2 ${#}

  export BACKUP_FILE_NAME=${1}
  export BACKUP_FILE_CONTENT=${2}

  yq e '.spec.containers[1].securityContext.runAsUser='$(id -u)' | .spec.containers[1].env[0].value=strenv(BACKUP_FILE_CONTENT) | .spec.containers[1].env[1].value=strenv(BACKUP_FILE_NAME)' definitions/annotated-subject/pod.yaml | kubectl apply -f -

  echo "✅  The annotated subject pod is ready"
}

given_a_rwo_pvc_subject_in_worker_node() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	yq e 'with(select(document_index == 1) .spec.template.spec; .containers[0].securityContext.runAsUser='$(id -u)' | .containers[0].env[0].value=strenv(BACKUP_FILE_CONTENT) | .containers[0].env[1].value=strenv(BACKUP_FILE_NAME))' definitions/pvc-rwo-subject/worker.yaml | kubectl apply -f -

	echo "✅  The pvc rwo worker subject is ready"
}

given_a_rwo_pvc_subject_in_controlplane_node() {
	require_args 2 ${#}

	export BACKUP_FILE_NAME=${1}
	export BACKUP_FILE_CONTENT=${2}

	yq e 'with(select(document_index == 1) .spec.template.spec; .containers[0].securityContext.runAsUser='$(id -u)' | .containers[0].env[0].value=strenv(BACKUP_FILE_CONTENT) | .containers[0].env[1].value=strenv(BACKUP_FILE_NAME))' definitions/pvc-rwo-subject/controlplane.yaml | kubectl apply -f -

	echo "✅  The pvc rwo controlplane subject is ready"
}

given_s3_storage() {
	# Speed this step up
	(helm -n "${MINIO_NAMESPACE}" list | grep minio > /dev/null) && return
	helm repo add minio https://charts.min.io/ --force-update
	helm repo update
	helm upgrade --install minio \
		--values definitions/minio/helm.yaml \
		--create-namespace \
		--namespace "${MINIO_NAMESPACE}" \
		minio/minio

	echo "✅  S3 Storage is ready"
}

given_a_clean_s3_storage() {
	# uninstalling an then installing the helmchart unfortunatelly hangs ong GH actions
	given_s3_storage
	kubectl -n "${MINIO_NAMESPACE}" scale deployment minio  --replicas 0

	kubectl -n "${MINIO_NAMESPACE}" delete pvc minio
	yq e '.metadata.namespace='\"${MINIO_NAMESPACE}\"'' definitions/minio/pvc.yaml | kubectl apply -f -

	kubectl -n "${MINIO_NAMESPACE}" scale deployment minio  --replicas 1

	echo "✅  S3 Storage cleaned"
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

	helm uninstall -n k8up-system k8up || true

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

verify_object_value_by_label() {
	require_args 5 ${#}

	resource=${1}
	labelSelector=${2}
	property=${3}
	expected_value=${4}
	should_return=${5}

	result=$(get_resource_value_by_label "$resource" "$labelSelector" "$property")

	code=$(verify_result "$result" "$expected_value")
	if [[ "$should_return" == "true" ]]; then
		return "$code"
	else
		echo "$code"
	fi
}

get_resource_value_by_label() {
	require_args 3 ${#}

	resource=${1}
	labelSelector=${2}
	property=${3}
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	query=$(build_k8s_request "$property")
	result=$(eval kubectl --namespace "${ns}" get "${resource}" -l "${labelSelector}" "$query" --no-headers)

	# Debug?
	detik_debug "-----DETIK:begin-----"
	detik_debug "$BATS_TEST_FILENAME"
	detik_debug "$BATS_TEST_DESCRIPTION"
	detik_debug ""
	detik_debug "Client query:"
	detik_debug "kubectl --namespace ${ns} get ${resource} -l ${labelSelector} $query --no-headers"
	detik_debug ""
	detik_debug "Result:"
	detik_debug "$result"
	detik_debug "-----DETIK:end-----"
	detik_debug ""

	# Is the result empty?
	if [[ "$result" == "" ]]; then
		echo "No resource of type '$resource' was found with the labelSelector '$labelSelector'."
	fi

	echo "$result"
}

# verify values of pods created by jobs
verify_job_pod_values() {
	labelSelector=${1}
	shift
	property=${1}
	shift
	expected_values="${*}"

	jobs=$(get_resource_value_by_label job "$labelSelector" "")
	IFS=$'\n'
	invalid=0
	valid=0

	for want in $expected_values; do
		for line in $jobs; do
			ret=$(verify_object_value_by_label pod "job-name=${line}" "$property" "$want" false)
			if [[ "$ret" == "0" ]]; then
				valid=$((valid + 1))
			else
				invalid=$((invalid + 1))
			fi
		done
	done

	if [[ "$valid" == "0" ]]; then
		return 102
	fi

	return 0
}

# copied from detik.bash by detik to an own function
verify_result() {
	require_args 2 ${#}

	result=${1}
	expected_value=${2}

	IFS=$'\n'
	invalid=0
	valid=0
	for line in $result; do
		# Keep the second column (property to verify)
		value=$(echo "$line" | awk '{ print $2 }')
		element=$(echo "$line" | awk '{ print $1 }')

		# Compare with an exact value (case insensitive)
		value=$(to_lower_case "$value")
		expected_value=$(to_lower_case "$expected_value")
		if [[ "$value" != "$expected_value" ]]; then
			detik_debug "Current value for $element is $value..."
			invalid=$((invalid + 1))
		else
			detik_debug "$element has the right value ($value)."
			valid=$((valid + 1))
		fi
	done

	if [[ "$valid" == "0" ]]; then
		invalid=102
	fi

	echo $invalid
}

wait_until() {
	require_args 2 ${#}

	local object condition ns
	object=${1}
	condition=${2}
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	echo "Waiting for '${object}' in namespace '${ns}' to become '${condition}' ..."
	kubectl -n "${ns}" wait --timeout 2m --for "condition=${condition}" "${object}"
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

get_latest_snap() {
	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	kubectl -n "${ns}" get snapshots -ojson | jq -r '.items | sort_by(.spec.date) | reverse | .[0].spec.id '
}

get_latest_snap_by_path() {
	require_args 1 ${#}

	ns=${NAMESPACE=${DETIK_CLIENT_NAMESPACE}}

	kubectl -n "${ns}" get snapshots -ojson | jq --arg path "$1" -r '[.items | sort_by(.spec.date) | reverse | .[] | select(.spec.paths[0]==$path)] | .[0].spec.id'
}

verify_snapshot_count() {
	require_args 2 ${#}

	ns=${2}

	echo "looking for ${1} snapshots"

	[ "$(kubectl -n "${ns}" get snapshots -ojson | jq -r '.items | length')" = "${1}" ]
}
