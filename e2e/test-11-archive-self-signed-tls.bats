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

@test "Given an existing Restic repository, When creating a Archive (mTLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_mtls_backup "${expected_filename}" "${expected_content}"
	given_a_clean_archive archive

	# Archive
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/archive/s3-mtls-archive-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Archive named 'k8up-s3-mtls-archive-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-archive-mtls' and verify that '.status.active' is '1'"

	wait_until archive/k8up-s3-mtls-archive-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Archive named 'k8up-s3-mtls-archive-mtls'"

	run restic list snapshots

	echo "---BEGIN total restic snapshots output---"
	total_snapshots=$(echo -e "${output}" | wc -l)
	echo "${total_snapshots}"
	echo "---END---"

	run mc ls minio/archive

	echo "---BEGIN total archives output---"
	total_archives=$(echo -n -e "${output}" | wc -l)
	echo "${total_archives}"
	echo "---END---"

	[ "$total_snapshots" -eq "$total_archives" ]
}
