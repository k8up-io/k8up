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

@test "Creating a Backup of an annotated pod" {
    expected_content="expected content: $(timestamp)"
    expected_filename="expected_filename.txt"

    given_a_running_operator
    given_a_clean_ns
    given_s3_storage
    given_an_annotated_subject_pod "${expected_filename}" "${expected_content}"

    kubectl apply -f definitions/secrets
    yq e '.spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup.yaml | kubectl apply -f -

    try "at most 10 times every 5s to get backup named 'k8up-backup' and verify that '.status.started' is 'true'"
    verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup' '.status.active' 1 true

    wait_until backup/k8up-backup completed

    run restic snapshots

    echo "---BEGIN restic snapshots output---"
    echo "${output}" | jq .
    echo "---END---"

    echo -n "Number of Snapshots >= 1? "
    jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

    run get_latest_snap_by_path /k8up-e2e-subject-subject-container.txt

    run restic dump --path /k8up-e2e-subject-subject-container.txt "${output}" k8up-e2e-subject-subject-container.txt

    echo "---BEGIN actual /k8up-e2e-subject-subject-container.txt---"
    echo "${output}"
    echo "---END---"

    echo "${output} = ${expected_content}"
    [ "${output}" = "${expected_content}" ]
}
