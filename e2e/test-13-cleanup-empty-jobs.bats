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

@test "Given empty namespace, When creating multiple backups, Then expect cleanup" {
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)' | .metadata.name="first-backup"' definitions/backup/backup.yaml | kubectl apply -f -

	yq e '.spec.podSecurityContext.runAsUser='$(id -u)' | .metadata.name="second-backup"' definitions/backup/backup.yaml | kubectl apply -f -

	kubectl -n "$DETIK_CLIENT_NAMESPACE" annotate backup/second-backup reconcile=now

	wait_for_until_jsonpath backup/second-backup 5m 'jsonpath={.status.conditions[?(@.type=="Scrubbed")].message}="Deleted 1 resources"'

}
