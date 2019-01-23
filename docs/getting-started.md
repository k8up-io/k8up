# Getting Started

## Backup credentials

First create credentials for the backup repository.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: backup-credentials
  namespace: myproject
type: Opaque
data:
  username: OFUwVUROWVBOVURUVVMxTElBRjM=
  password: aXAzY2Rya1hjSG1INFM3aWY3ZXJLUE5veERuMjdWMHZyZzZDSEhlbQ==

---

apiVersion: v1
kind: Secret
metadata:
  name: backup-repo
  namespace: myproject
type: Opaque
data:
  password: cEBzc3cwcmQ=
```

The values of the secrets need to be in base64. To convert a string to base64 use:
```bash
echo -n "p@ssw0rd" | base64
```

**Attention:** Please store that password somewhere save. This is the encryption password for Restic. Without it you'll lose access to the backup permanently.

## Operator installation

The most convenient way to install K8up is via [helm](https://helm.sh/):

```bash
helm repo add appuio https://charts.appuio.ch

export TILLER_NAMESPACE=tiller
helm install appuio/baas-operator -n K8up --namespace k8up-operator

# Also install our prometheus instance
helm install appuio/prometheus -n K8up --namespace k8up-operator
```

For advanced cofniguration of the operator please see TODO: link.

## Enable backup schedule in a namespace

Now you'll only need to create a custom object in the namespaces you'd like to backup:

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Schedule
metadata:
  name: schedule-test
spec:
  backend:
    s3:
      endpoint: http://localhost:9000
      bucket: backups
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
      repoPasswordSecretRef:
        name: backup-repo
        key: password
  archive:
    schedule: '0 0 1 * *'
    restoreMethod:
      s3:
        endpoint: http://localhost:9000
        bucket: archive
        accessKeyIDSecretRef:
          name: backup-credentials
          key: username
        secretAccessKeySecretRef:
          name: backup-credentials
          key: password
  backup:
    schedule: '0 3 * * *'
    keepJobs: 4
    promURL: http://localhost:9000
  check:
    schedule: '0 1 * * 1'
    promURL: http://localhost:9000
  prune:
    schedule: '0 1 * * 0'
    retention:
      keepLast: 5
      keepDaily: 14
```
This will instruct the operator to do backups each day at 03:00 AM. As well as monthly prune and check jobs for repository maintenance. It will also archive the latest snapshots to an S3 bucket once each week.

Feel free to adjust the schedules to your liking. For figuring out the crontab syntax, we recommend to get help from [crontab.guru](https://crontab.guru).

!!! Note
    * You can always check the state and configuration of your backup by using `oc -n mynamespace describe schedule`
    * By default all PVCs are stored in backup. By adding the annotation `appuio.ch/backup=false` to a PVC object it will get excluded from backup.

## Application aware backups

It’s possible to define annotations on pods with backup commands. These backup commands should create an application aware backup and stream it to stdout.


Define an annotation on pod:

```yaml
<SNIP>
template:
  metadata:
    labels:
      app: mariadb
    annotations:
      appuio.ch/backupcommand: mysqldump -uroot -psecure --all-databases
<SNIP>
```

With this annotation the operator will trigger that command inside the the container and capture the stdout to a backup.

Tested with:

* MariaDB
* MongoDB
* tar to stdout

But it should work with any command that has the ability to output the backup to stdout.
