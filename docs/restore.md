# Restore

It is possible to tell the operator to perform restores either to a PVC or an S3 bucket.

For this you can create a restore object:

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Restore
metadata:
  name: restore-test

spec:
  repoPasswordSecretRef:
    name: backup-repo
    key: password
  s3:
    endpoint: http://localhost:9000
    bucket: restore
    accessKeyIDSecretRef:
      name: backup-credentials
      key: username
    secretAccessKeySecretRef:
      name: backup-credentials
      key: password
  backend:
    s3:
      endpoint: http://localhost:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
```

This will trigger a one time job to restore the latest snapshot to S3.

## Manual restore via Restic

To manually restore you'll need:
* Linux machine with [restic](https://github.com/restic/restic)
* Fuse (Optional for mounting)

Let's take this `backend` example from a schedule:
```yaml
backend:
  s3:
    endpoint: http://localhost:9000
    bucket: baas
    accessKeyIDSecretRef:
      name: backup-credentials
      key: username
    secretAccessKeySecretRef:
      name: backup-credentials
      key: password
```

You'll need the credentials from the secrets as well as the encryption key. With that information you can configure restic:

```bash
export RESTIC_REPOSITORY=s3:http://localhost/baas
export RESTIC_PASSWORD=p@assword
export AWS_ACCESS_KEY_ID=8U0UDNYPNUDTUS1LIAF3
export AWS_SECRET_ACCESS_KEY=ip3cdrkXcHmH4S7if7erKPNoxDn27V0vrg6CHHem
```
Now you can use Restic to browse and restore snapshots:

```bash
# List snapshots
restic snapshots
repository dec6d66c opened successfully, password is correct
ID        Date                 Host                Tags        Directory
----------------------------------------------------------------------
5ed64a2d  2018-06-08 09:18:34  macbook-vshn.local              /data
----------------------------------------------------------------------
1 snapshots

restic restore 5ed64a2d --target /restore

# Or mount the repository for convenient restores
restic mount ~/Desktop/mount
repository dec6d66c opened successfully, password is correct
Now serving the repository at /Users/simonbeck/Desktop/mount/
Dont forget to umount after quitting!

ll ~/Desktop/mount
total 0
dr-xr-xr-x  1 simonbeck  staff    0 Jun  8 09:21 .
drwx------+ 6 simonbeck  staff  192 Jun  8 09:15 ..
dr-xr-xr-x  1 simonbeck  staff    0 Jun  8 09:21 hosts
dr-xr-xr-x  1 simonbeck  staff    0 Jun  8 09:21 ids
dr-xr-xr-x  1 simonbeck  staff    0 Jun  8 09:21 snapshots
dr-xr-xr-x  1 simonbeck  staff    0 Jun  8 09:21 tags
```

Here you can browse all backups by host, ids, snapshots or tags.
