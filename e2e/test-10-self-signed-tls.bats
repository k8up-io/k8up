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

### Start backup section

@test "Given a PVC, When creating a Backup (TLS) of an app, Then expect Restic repository - using self-signed issuer" {
	expected_content="expected content for tls: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_a_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup-tls.yaml | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup-tls' and verify that '.status.started' is 'true'"
	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup-tls' '.status.active' 1 true

	wait_until backup/k8up-backup-tls completed

	run restic snapshots

	echo "---BEGIN restic snapshots output---"
	echo "${output}"
	echo "---END---"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run get_latest_snap

	run restic dump "${output}" "/data/subject-pvc/${expected_filename}"

	echo "---BEGIN actual ${expected_filename}---"
	echo "${output}"
	echo "---END---"

	[ "${output}" = "${expected_content}" ]
}

@test "Given a PVC, When creating a Backup (mTLS) of an app, Then expect Restic repository - using self-signed issuer" {
	expected_content="expected content for mtls: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_a_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup-mtls' and verify that '.status.started' is 'true'"
	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup-mtls' '.status.active' 1 true

	wait_until backup/k8up-backup-mtls completed

	run restic snapshots

	echo "---BEGIN restic snapshots output---"
	echo "${output}"
	echo "---END---"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run get_latest_snap

	run restic dump "${output}" "/data/subject-pvc/${expected_filename}"

	echo "---BEGIN actual ${expected_filename}---"
	echo "${output}"
	echo "---END---"

	[ "${output}" = "${expected_content}" ]
}

@test "Given a PVC, When creating a Backup (mTLS with env) of an app, Then expect Restic repository - using self-signed issuer" {
	expected_content="expected content for mtls: $(timestamp)"
	expected_filename="expected_filename.txt"

	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_a_subject "${expected_filename}" "${expected_content}"

	kubectl apply -f definitions/secrets
	kubectl apply -f definitions/backup/config-mtls-env.yaml
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/backup/backup-mtls-env.yaml | kubectl apply -f -

	try "at most 10 times every 5s to get backup named 'k8up-backup-mtls-env' and verify that '.status.started' is 'true'"
	verify_object_value_by_label job 'k8up.io/owned-by=backup_k8up-backup-mtls-env' '.status.active' 1 true

	wait_until backup/k8up-backup-mtls-env completed

	run restic snapshots

	echo "---BEGIN restic snapshots output---"
	echo "${output}"
	echo "---END---"

	echo -n "Number of Snapshots >= 1? "
	jq -e 'length >= 1' <<< "${output}"          # Ensure that there was actually a backup created

	run get_latest_snap

	run restic dump "${output}" "/data/subject-pvc/${expected_filename}"

	echo "---BEGIN actual ${expected_filename}---"
	echo "${output}"
	echo "---END---"

	[ "${output}" = "${expected_content}" ]
}

### End backup section

### Start restore to pvc section

@test "Given an existing Restic repository, When creating a Restore (TLS), Then Restore to PVC - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Delete and create new subject
	new_content="New content for tls: $(timestamp)"
	new_filename="new_file.txt"
	given_a_clean_ns
	give_self_signed_issuer
	given_a_subject "${new_filename}" "${new_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/restore-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-restore-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-restore-tls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-restore-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-restore-tls'"

	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${new_filename}" "${new_content}"
}

@test "Given an existing Restic repository, When creating a Restore (mTLS), Then Restore to PVC - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Delete and create new subject
	new_content="New content for mtls: $(timestamp)"
	new_filename="new_file.txt"
	given_a_clean_ns
	give_self_signed_issuer
	given_a_subject "${new_filename}" "${new_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/restore-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-restore-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-restore-mtls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-restore-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-restore-mtls'"

	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
	expect_file_in_container 'deploy/subject-deployment' 'subject-container' "/data/${new_filename}" "${new_content}"
}

### End restore to pvc section

### Start restore to s3 section

@test "Given an existing Restic repository, When creating a Restore (TLS), Then Restore to S3 (TLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-tls-restore-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-tls-restore-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-tls-restore-tls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-tls-restore-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-tls-restore-tls'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}

@test "Given an existing Restic repository, When creating a Restore (mTLS), Then Restore to S3 (TLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-tls-restore-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-tls-restore-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-tls-restore-mtls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-tls-restore-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-tls-restore-mtls'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}

@test "Given an existing Restic repository, When creating a Restore (TLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-mtls-restore-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-mtls-restore-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-restore-tls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-mtls-restore-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-mtls-restore-tls'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}

@test "Given an existing Restic repository, When creating a Restore (mTLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-mtls-restore-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-mtls-restore-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-restore-mtls' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-mtls-restore-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-mtls-restore-mtls'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}

@test "Given an existing Restic repository, When creating a Restore (mTLS with env), Then Restore to S3 (mTLS with env) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Restore
	kubectl apply -f definitions/secrets
	kubectl apply -f definitions/restore/config-mtls-env.yaml
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/restore/s3-mtls-restore-mtls-env.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Restore named 'k8up-s3-mtls-restore-mtls-env' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-restore-mtls-env' and verify that '.status.active' is '1'"

	wait_until restore/k8up-s3-mtls-restore-mtls-env completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Restore named 'k8up-s3-mtls-restore-mtls-env'"

	expect_dl_file_in_container 'deploy/subject-dl-deployment' 'subject-container' "/data/${expected_filename}" "${expected_content}"
}

### End restore to s3 section

### Start archive to s3 section

@test "Given an existing Restic repository, When creating a Archive (TLS), Then Restore to S3 (TLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"
	given_a_clean_archive archive

	# Archive
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/archive/s3-tls-archive-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Archive named 'k8up-s3-tls-archive-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-tls-archive-tls' and verify that '.status.active' is '1'"

	wait_until archive/k8up-s3-tls-archive-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Archive named 'k8up-s3-tls-archive-tls'"

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

@test "Given an existing Restic repository, When creating a Archive (mTLS), Then Restore to S3 (TLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"
	given_a_clean_archive archive

	# Archive
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/archive/s3-tls-archive-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Archive named 'k8up-s3-tls-archive-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-tls-archive-mtls' and verify that '.status.active' is '1'"

	wait_until archive/k8up-s3-tls-archive-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Archive named 'k8up-s3-tls-archive-mtls'"

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

@test "Given an existing Restic repository, When creating a Archive (TLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"
	given_a_clean_archive archive

	# Archive
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/archive/s3-mtls-archive-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Archive named 'k8up-s3-mtls-archive-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-archive-tls' and verify that '.status.active' is '1'"

	wait_until archive/k8up-s3-mtls-archive-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Archive named 'k8up-s3-mtls-archive-tls'"

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

@test "Given an existing Restic repository, When creating a Archive (mTLS), Then Restore to S3 (mTLS) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"
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

@test "Given an existing Restic repository, When creating a Archive (mTLS with env), Then Restore to S3 (mTLS with env) - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"
	given_a_clean_archive archive

	# Archive
	kubectl apply -f definitions/secrets
	kubectl apply -f definitions/archive/config-mtls-env.yaml
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/archive/s3-mtls-archive-mtls-env.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Archive named 'k8up-s3-mtls-archive-mtls-env' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-s3-mtls-archive-mtls-env' and verify that '.status.active' is '1'"

	wait_until archive/k8up-s3-mtls-archive-mtls-env completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Archive named 'k8up-s3-mtls-archive-mtls-env'"

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

### End archive to s3 section

### Start check section

@test "Given a PVC, When creating a Check (TLS) of an app, Then expect Restic repository - using self-signed issuer" {
	# Backup
	expected_content="Old content for tls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Check
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/check/check-tls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Check named 'k8up-check-tls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-check-tls' and verify that '.status.active' is '1'"

	wait_until check/k8up-check-tls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Check named 'k8up-check-tls'"
}

@test "Given a PVC, When creating a Check (mTLS) of an app, Then expect Restic repository - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Check
	kubectl apply -f definitions/secrets
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/check/check-mtls.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Check named 'k8up-check-mtls' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-check-mtls' and verify that '.status.active' is '1'"

	wait_until check/k8up-check-mtls completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Check named 'k8up-check-mtls'"
}

@test "Given a PVC, When creating a Check (mTLS with env) of an app, Then expect Restic repository - using self-signed issuer" {
	# Backup
	expected_content="Old content for mtls: $(timestamp)"
	expected_filename="old_file.txt"
	given_a_running_operator
	given_a_clean_ns
	given_s3_storage
	give_self_signed_issuer
	given_an_existing_backup "${expected_filename}" "${expected_content}"

	# Check
	kubectl apply -f definitions/secrets
	kubectl apply -f definitions/check/config-mtls-env.yaml
	yq e '.spec.podSecurityContext.fsGroup='$(id -u)' | .spec.podSecurityContext.runAsUser='$(id -u)'' definitions/check/check-mtls-env.yaml | kubectl apply -f -

	try "at most 10 times every 1s to get Check named 'k8up-check-mtls-env' and verify that '.status.started' is 'true'"
	try "at most 10 times every 1s to get Job named 'k8up-check-mtls-env' and verify that '.status.active' is '1'"

	wait_until check/k8up-check-mtls-env completed
	verify "'.status.conditions[?(@.type==\"Completed\")].reason' is 'Succeeded' for Check named 'k8up-check-mtls-env'"
}

### End check section
