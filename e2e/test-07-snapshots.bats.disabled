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

@test "Given apps in two namespaces, When creating a backup of both apps, Then expect one snapshot object per namespace" {
	expected_content="expected content: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_a_clean_s3_storage
	given_a_subject "${expected_filename}" "${expected_content}"

	# second subject in another namespace
	ns2=e2e-second-subject
	kubectl delete ns "${ns2}" || true
	kubectl create ns "${ns2}"
	echo "create second PVC"
	yq e '.metadata.namespace='\"${ns2}\"'' definitions/pv/pvc.yaml | kubectl apply -f -
	echo "create second subject"
	yq e '.spec.template.spec.containers[0].securityContext.runAsUser='$(id -u)' | .spec.template.spec.containers[0].env[1].value="test" | .metadata.namespace='\"${ns2}\"'' definitions/subject/deployment.yaml | kubectl apply -f -
	echo "create second credentials"
	yq e '.metadata.namespace='\"${ns2}\"'' definitions/secrets/secrets.yaml | kubectl apply -f -

	# backup in first namespace
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='"$(id -u)"'' definitions/backup/backup.yaml | kubectl apply -f -

	# backup in second namespace
	echo "create second backup"
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)' | .metadata.namespace='\"${ns2}\"'' definitions/backup/backup.yaml | kubectl apply -f -

	wait_until backup/k8up-backup completed

	verify_snapshot_count 1 "${DETIK_CLIENT_NAMESPACE}"

	kubectl -n "${ns2}" wait --timeout 2m --for "condition=completed" backup/k8up-backup

	verify_snapshot_count 1 "${ns2}"

}

@test "Given backups in two repositories, When backing up in same namespace, Then expect two snapshot objects" {
	expected_content="expected content: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_a_clean_s3_storage
	given_a_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.runAsUser='"$(id -u)"'' definitions/backup/backup.yaml | kubectl apply -f -

	# backup in second repo
	echo "create second backup"
	yq e '.spec.podSecurityContext.runAsUser='$(id -u)' | .metadata.name="k8up-backup-second" | .spec.backend.s3.bucket="second"' definitions/backup/backup.yaml | kubectl apply -f -

	wait_until backup/k8up-backup completed
	wait_until backup/k8up-backup-second completed

	verify_snapshot_count 2 "${DETIK_CLIENT_NAMESPACE}"

}
