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

@test "Given an existing Restic repository, When creating a Restore (mTLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_mtls_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-mtls-restore-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-mtls-restore-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-restore-mtls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-mtls-restore-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-mtls-restore-mtls'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}
