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

@test "Annotated app, When creating a backup, Then expect Error" {
	expected_content="expected content: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_a_broken_annotated_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup.yaml | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup' and verify that '.status.started' is 'true'"
	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup' '.status.active' 1 true

	wait_for_until_jsonpath backup/k8up-backup 2m 'jsonpath={.status.conditions[?(@.type=="Completed")].reason}=Failed'

}
