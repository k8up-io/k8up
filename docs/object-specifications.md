# Object Specifications

The K8up operator includes various CRDs which get added to the cluster. Here We'll explain them in more detail.

## Schedule
With the schedule CRD it is possible to put all other CRDs on a schedule.

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Schedule
metadata:
  name: schedule-test
spec:
  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
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
    schedule: '0 * * * *'
    restoreMethod:
      s3:
        endpoint: http://10.144.1.224:9000
        bucket: restoremini
        accessKeyIDSecretRef:
          name: backup-credentials
          key: username
        secretAccessKeySecretRef:
          name: backup-credentials
          key: password
  backup:
    schedule: '* * * * *'
    keepJobs: 4
    promURL: http://10.144.1.224:9000
  check:
    schedule: '*/5 * * * *'
    promURL: http://10.144.1.224:9000
  prune:
    schedule: '*/2 * * * *'
    retention:
      keepLast: 5
      keepDaily: 14
```

### Settings

  * `archive`: see archive for further explaination
  * `backend`: see backend for further explanaition
  * `check`: see check for further explanaition
  * `prune`: see prune for further explanaition

## Restore
It's now possible to define various restore jobs. Currently these kinds of restores are supported:

* To a PVC
* To S3 as tar.gz

Example for a restore to a PVC:

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Restore
metadata:
  name: restore-test

spec:
  repoPasswordSecretRef:
    name: backup-repo
    key: password
  restoreMethod:
    folder:
      claimName: restore

  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
```

This will restore the latest snapshot from `http://10.144.1.224:9000` to the PVC with the name `restore`.

### Settings

TODO:

## Archive
The archive CRD will take the latest snapshots from each namespace/project in the repository. Thus you should only run one schedule per repository for archival as there's a chance that you'll archive snapshots more than once.

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Archive
metadata:
  name: archive-test
spec:
  repoPasswordSecretRef:
    name: backup-repo
    key: password
  restoreMethod:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: restoremini
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
```

### Settings

TODO:

## Backup
This will trigger a single backup.

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Backup
metadata:
  name: baas-test
spec:
  keepJobs: 4
  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
  retention:
    keepLast: 5
    keepDaily: 14
  promURL: http://10.144.1.224:9000
  repoPasswordSecretRef:
    name: backup-repo
    key: password
```

### Settings

TODO:

## Check
This will trigger a single check run on the repository.

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Check
metadata:
  name: check-test
spec:
  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
  promURL: http://10.144.1.224:9000
  repoPasswordSecretRef:
    name: backup-repo
    key: password

```

### Settings

TODO:

## Prune
This will trigger a single prune run and delete the snapshots according to the defined retention rules. This one needs to run exclusively on the repository. No other jobs must run on the same repository while this one is still running. The operator ensures that the prune will run exclusively on the repository.

```yaml
apiVersion: backup.appuio.ch/v1alpha1
kind: Prune
metadata:
  name: prune-test
spec:
  retention:
    keepLast: 5
    keepDaily: 14
  backend:
    s3:
      endpoint: http://10.144.1.224:9000
      bucket: baas
      accessKeyIDSecretRef:
        name: backup-credentials
        key: username
      secretAccessKeySecretRef:
        name: backup-credentials
        key: password
      repoPasswordSecretRef:
        name: backup-repo
        key: password
  promURL: http://10.144.1.224:9000
```

### Settings

TODO:

## Backend

TODO:

### Settings

TODO:
