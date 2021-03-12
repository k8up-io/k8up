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

@test "verify a restore" {
	# Backup
	expected_content="Old content: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Delete and create new subject
	new_content="New content: $(timestamp)"
	new_filename="new_file.txt"
	given_a_clean_ns
	given_a_subject "${new_filename}" "${new_content}"

	# Restore
	apply definitions/restore
	try "at most 10 times every 1s to get restore named 'k8up-k8up-restore' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get job named 'k8up-k8up-restore' and verify that '.status.active' is '1'"
	wait_until restore/k8up-k8up-restore completed

	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${new_filename}" "${new_content}"
}
