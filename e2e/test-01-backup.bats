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

	apply definitions/backup
	try "at most 10 times every 1s to get backup named 'k8up-k8up-backup' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get job named 'k8up-k8up-backup' and verify that '.status.active' is '1'"

	wait_until backup/k8up-k8up-backup completed

	run restic snapshots 2>/dev/null

	echo "Restic output: '${output}'"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run restic dump latest "/data/subject-pvc/${expected_filename}"

	echo "---BEGIN actual ${expected_filename}---"
	echo "${output}"
	echo "---END---"

	[ "${output}" = "${expected_content}" ]
}
