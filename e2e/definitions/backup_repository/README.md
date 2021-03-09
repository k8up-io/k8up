# A restic Backup

This folder contains a complete restic backup.
`restic snapshot` would show it like this:

```bash
$ restic snapshots
ID        Time                 Host              Tags        Paths
------------------------------------------------------------------------------
fcf582cd  2021-03-09 15:48:32  k8up-e2e-subject              /data/subject-pvc
------------------------------------------------------------------------------
1 snapshots
```

## Re-create the backup

To create the backup, I ran the test `test-01-backup.bats`.
But I wanted to have the string `MagicString` as content of the file `/data/expectation.txt` in the `subject` container,
which is actually the `/expectation.txt` file in the `subject-pv`.
So I changed the deployment of the subject:

```patch
--- a/e2e/definitions/subject/deployment.yaml
+++ b/e2e/definitions/subject/deployment.yaml
@@ -21,7 +21,7 @@ spec:
         - sh
         - -c
         - |
-          date +%s | tee /data/expectation.txt && \
+          echo "MagicString" | tee /data/expectation.txt && \
           ls -la /data && \
           echo 'test file written, sleeping now' && \
           sleep infinity
```

Only then I ran the test `test-01-backup.bats`.

Afterwards, when the test was successful,
I used minio's `mc` command to copy the content from the S3 storage to the local file system.
To do that yourself,
**`cd` to the root of the project**,
then copy and paste the following snippet into your `bash` or `zsh` compatible shell:

```bash
# First, `cd` to the root of the project!
cd /home/vshn/src/k8up

export WRESTIC_IMAGE=quay.io/vshn/wrestic:v0.1.9
export MINIO_NAMESPACE=minio
export KUBECONFIG="$(pwd)/testbin/bin/kind-kubeconfig-v1.20.0"
export E2E_IMAGE="local.dev/k8up/e2e:e2e_$(sha1sum $(pwd)/k8up | cut -b-8)"; echo "$E2E_IMAGE"
timestamp() {
  date +%s
}
restic() {
	kubectl run "wrestic-$(timestamp)" \
		--rm \
		--attach \
		--restart Never \
		--wait \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${WRESTIC_IMAGE}" \
		--env "AWS_ACCESS_KEY_ID=myaccesskey" \
		--env "AWS_SECRET_KEY=mysecretkey" \
		--env "RESTIC_PASSWORD=myreposecret" \
		--pod-running-timeout 10s \
		--quiet=true \
		--command -- \
			restic \
			--no-cache \
			-r "s3:http://minio.minio.svc.cluster.local:9000/backup" \
			"${@}"
}
mc() {
	minio_access_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.accesskey}" | base64 --decode)
	minio_secret_key=$(kubectl -n "${MINIO_NAMESPACE}" get secret minio -o jsonpath="{.data.secretkey}" | base64 --decode)
	minio_url=http://${minio_access_key}:${minio_secret_key}@minio.minio.svc.cluster.local:9000
	kubectl run "minio-$(timestamp)" \
		--rm \
		--attach \
		--stdin \
		--restart Never \
		--wait=true \
		--namespace "${DETIK_CLIENT_NAMESPACE-"k8up-system"}" \
		--image "${MINIO_IMAGE-minio/mc:latest}" \
		--env "MC_HOST_s3=${minio_url}" \
		--pod-running-timeout 10s \
		--quiet=true \
		--command -- \
			mc \
			"${@}"
}
mc_cat() {
    file="$1"
    echo "Copying 's3/backup/${file}' to '$(pwd)/e2e/definitions/backup_repository/${file}'"
    mc cat "s3/backup/${file}" > "e2e/definitions/backup_repository/${file}"
}
```

Now you can browse the S3 backup bucket like so:

```bash
$ mc ls -r s3/backup
[2021-03-09 20:13:39 UTC]   155B config
[2021-03-09 20:13:51 UTC] 1.2KiB data/04/047185cfd10e1022ddbb70cfa95733fc35bd132911f2c677f151130c3de2d002
[2021-03-09 20:13:42 UTC]   468B index/b86075dc8b4313e0694c4be1db1db4bc67b4e81001fb91869326f5a54c476ab7
[2021-03-09 20:13:45 UTC]   457B keys/e3bf7b2ec06eabb49295c31c1fe1de85a2382c00bb40db58d102bdea8c55363a
[2021-03-09 20:13:48 UTC]   223B snapshots/fcf582cd2f972ea11e34bbd17491cd4d581f66d344916d7d398b548a10ae365d
```

To export the files to this folder, `backup_repository`, make sure you're in the root of the project.
Then you can use the `mc_cat` function:

```bash
$ # First, `cd` to the root of the project!
$ cd /home/vshn/src/k8up
$ mc_cat config
Copying 'config' to '/home/vshn/src/k8up/e2e/definitions/backup_repository/config
```

Do this for every file in the repository.
Make sure that the folders in `e2e/definitions/backup_repository/data/` already exist before using `mc_cat`.
