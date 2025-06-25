#!/usr/bin/env bats

load "lib/utils"
load "lib/detik"
load "lib/k8up"

# shellcheck disable=SC2034
DETIK_CLIENT_NAME="kubectl"
# shellcheck disable=SC2034
DETIK_CLIENT_NAMESPACE="k8up-e2e-subject"
# shellcheck disable=SC2034
DEBUG_DETIK="true"

@test "Given a PVC, When creating a Backup of an app, Then expect Restic repository" {
	expected_content="expected content: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_a_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	kubectl apply -f definitions/backup/podconfig.yaml
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup.yaml | \
	yq e '.spec.podConfigRef.name="podconfig"' - | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup' and verify that '.status.started' is 'true'"
	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup' '.status.active' 1 true

	wait_until backup/k8up-backup completed

	run restic snapshots

	echo "---BEGIN restic snapshots output---"
	echo "${output}"
	echo "---END---"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run get_latest_snap

	run restic dump "${output}" "/data/subject-pvc/${expected_filename}"

	echo "---BEGIN actual ${expected_filename}---"
	echo "${output}"
	echo "---END---"

	echo "${output} = ${expected_content}"
	[ "${output}" = "${expected_content}" ]

	# Check podConfig merging behaviour
	annotation="$(kubectl -n "${DETIK_CLIENT_NAMESPACE}" get podConfig podconfig -ojson | jq -r '.metadata.annotations.test')"
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .metadata.annotations.test "$annotation"

	name="$(kubectl -n "${DETIK_CLIENT_NAMESPACE}" get podConfig podconfig -ojson | jq -r '.spec.template.spec.containers[0].env[0].name')"
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .spec.containers[0].env[0].name "$name"

	secCont="$(kubectl -n "${DETIK_CLIENT_NAMESPACE}" get podConfig podconfig -ojson | jq -r '.spec.template.spec.containers[0].securityContext.allowPrivilegeEscalation')"
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .spec.containers[0].securityContext.allowPrivilegeEscalation "${secCont}"

	secCont="$(kubectl -n "${DETIK_CLIENT_NAMESPACE}" get podConfig podconfig -ojson | jq -r '.spec.template.spec.containers[0].volumeMounts[0].mountPath')"
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .spec.containers[0].volumeMounts[0].mountPath "${secCont}"

	secCont="$(kubectl -n "${DETIK_CLIENT_NAMESPACE}" get podConfig podconfig -ojson | jq -r '.spec.template.spec.volumes[0].name')"
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .spec.volumes[0].name "${secCont}"
}
