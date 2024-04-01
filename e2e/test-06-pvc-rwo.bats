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

@test "Given two RWO PVCs, When creating a Backup of an app, Then expect Restic repository" {
	reset_debug

	expected_content="expected content: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_a_rwo_pvc_subject_in_worker_node "${expected_filename}-worker" "${expected_content}-worker"
	given_a_rwo_pvc_subject_in_controlplane_node "${expected_filename}-controlplane" "${expected_content}-controlplane"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup.yaml | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup' and verify that '.status.started' is 'true'"

	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup' '.status.active' 1 true
	verify_job_pod_values 'k8up.io/owned-by=backup_k8up-backup' .spec.nodeName k8up-v1.26.6-control-plane k8up-v1.26.6-worker

	wait_until backup/k8up-backup completed

	run restic snapshots

	echo "---BEGIN restic snapshots output---"
	echo "${output}"
	echo "---END---"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run get_latest_snap_by_path /data/pvc-rwo-subject-pvc-worker

	run restic dump "${output}" --path /data/pvc-rwo-subject-pvc-worker "/data/pvc-rwo-subject-pvc-worker/${expected_filename}-worker"

	echo "---BEGIN actual ${expected_filename}-worker---"
	echo "${output}"
	echo "---END---"

	echo "${output} = ${expected_content}-worker"
	[ "${output}" = "${expected_content}-worker" ]

	run get_latest_snap_by_path /data/pvc-rwo-subject-pvc-controlplane

	run restic dump "${output}" --path /data/pvc-rwo-subject-pvc-controlplane "/data/pvc-rwo-subject-pvc-controlplane/${expected_filename}-controlplane"

	echo "---BEGIN actual ${expected_filename}-controlplane---"
	echo "${output}"
	echo "---END---"

	echo "${output} = ${expected_content}-controlplane"
	[ "${output}" = "${expected_content}-controlplane" ]
}
