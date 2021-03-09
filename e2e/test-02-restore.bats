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
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	given_a_subject
	given_an_existing_backup

	apply definitions/restore
	try "at most 10 times every 1s to get restore named 'k8up-k8up-restore' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get job named 'k8up-k8up-restore' and verify that '.status.active' is '1'"

	wait_until restore/k8up-k8up-restore completed

	# shellcheck disable=SC2016
	kubectl exec \
		deploy/subject-deployment \
		--container "subject-container" \
		--stdin \
		--namespace "${DETIK_CLIENT_NAMESPACE}" \
		-- \
			sh -c 'ls -la /data && test -f /data/expectation.txt && cat /data/expectation.txt && echo test "MagicString" "=" "$(</data/expectation.txt)"'
}
