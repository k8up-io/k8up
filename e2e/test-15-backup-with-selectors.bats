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

@test "Given multiple labeled&non-labeled PVCs and PreBackupPods, When creating a Backup with selectors, Then expect Snapshot resources only for resources with matching labels" {
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_a_clean_s3_storage

	backup_file_name="testfile"
	backup_file_content="hello"

	kubectl apply -f definitions/pv/pvc.yaml
	kubectl apply -f definitions/pv/pvcs-matching-labels.yaml
	kubectl apply -f definitions/prebackup/prebackup-match-labels.yaml
	kubectl apply -f definitions/prebackup/prebackup-no-labels.yaml

	given_a_subject "${backup_file_name}" "${backup_file_content}"
	given_a_subject "${backup_file_name}" "${backup_file_content}" subject-pvc-specific-value-1
	given_a_subject "${backup_file_name}" "${backup_file_content}" subject-pvc-specific-value-2
	given_a_subject "${backup_file_name}" "${backup_file_content}" subject-pvc-label-exists

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup-selectors.yaml | \
	kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup-selectors' and verify that '.status.started' is 'true'"

	wait_until backup/k8up-backup-selectors completed

	verify_snapshot_count 5 "${DETIK_CLIENT_NAMESPACE}"
	run get_latest_snap_by_path /data/subject-pvc-specific-value-2
	run get_latest_snap_by_path k8up-e2e-subject-specific-label-value-1
	run get_latest_snap_by_path data/subject-pvc-label-exists
	run get_latest_snap_by_path data/subject-pvc-specific-value-1
	run get_latest_snap_by_path k8up-e2e-subject-arbitrary-label-value

}
