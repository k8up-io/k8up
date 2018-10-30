# Dev Environment
You'll need:

* Minishift or Minikube
* golang installed :) (everything is tested with 1.10.1)
* dep installed
* Your favorite IDE (with a golang plugin)
* docker
* make

## Generate kubernetes code
If you make changes to the CRD struct you'll need to run code generation. This can be done with make:

```
cd /project/root
make generate
```

This creates the client folder and deepcopy functions for the structs. This needs to be run on a local docker instance so it can mount the code to the container.

## Run the operator in dev mode

```
cd /to/go/project
minishift start
oc login -u system:admin # default developer account doesn't have the rights to create a crd
#The operator has the be run at least once before to create the CRD
go run cmd/operator/*.go -development
#Add a demo backupworker (adjust the variables to your liking first)
kubectl apply -f manifest-examples/baas.yaml
#Add a demo PVC if necessary
kubectl apply -f manifest-examples/pvc.yaml
```

## Build and push the Restic container
The container has to exist on the registry in order for the operator to find the correct one.

```
minishift start
oc login -u developer
eval $(minishift docker-env)
docker login -u developer -p $(oc whoami -t) $(minishift openshift registry)
docker build -t $(minishift openshift registry)/myproject/baas:0.0.1 .
docker push $(minishift openshift registry)/myproject/baas:0.0.1
```

## Installation
All required definitions for the installation are located at `manifest/install/`:

```bash
kubectl apply -f manifest/install/
```

Please be aware that these manifests are intended for dev and as examples. They are not the official way to install the operator in production. For this we provide a helm chart at https://github.com/appuio/charts.
You may need to adjust the namespaces in the manifests. There are various other examples under `manifest/examples/`.

Please see the example resource here in the readme for an explanation of the various settings.

# Examples
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

The Restic repository password and the credentials for S3 need to be saved to OpenShift secrets:
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
  password: YXNkZg==
```

The values of the secrets need to be in base64. To convert a string to base64 use:
```bash
echo -n "p@ssw0rd" | base64
```

## Make consistent backups
The Operator also supports setting commands that ensure consistency, you just need to set the `appuio.ch/backupcommand` annotation on the pods.

```yaml
<SNIP>
template:
    metadata:
      labels:
        app: mariadb
      annotations:
        appuio.ch/backupcommand: mysqldump -uroot -psecure --all-databases
    spec:
      containers:
        - env:
            - name: MYSQL_ROOT_PASSWORD
<SNIP>
```

## Configuration
Various things can be configured via environment variables:
* `BACKUP_IMAGE` URL for the restic image, default: `172.30.1.1:5000/myproject/restic`
* `BACKUP_ANNOTATION` the annotation to be used for filtering, default: `appuio.ch/backup`
* `BACKUP_CHECKSCHEDULE` the default check schedule, default: `0 0 * * 0`
* `BACKUP_PODFILTER` the filter used to find the backup pods, default: `backupPod=true`
* `BACKUP_DATAPATH` where the PVCs should get mounted in the container, default `/data`
* `BACKUP_JOBNAME` names for the backup job objects in OpenShift, default: `backupjob`
* `BACKUP_PODNAME` names for the backup pod objects in OpenShift, default: `backupjob-pod`
* `BACKUP_RESTARTPOLICY` set the RestartPolicy for the backup jobs. According to the [docs](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/) this should be `OnFailure` for jobs that terminate, default: `OnFailure`
* `BACKUP_METRICBIND` set the bind address for the prometheus endpoint, default: `:8080`
* `BACKUP_PROMURL` set the operator wide default prometheus push gateway, default `http://127.0.0.1/`
* `BACKUP_BACKUPCOMMANDANNOTATION` set the annotation name where the backup commands are stored, default `appuio.ch/backupcommand`
* `BACKUP_PODEXECROLENAME` set the rolename that should be used for pod command execution, default `pod-executor`
* `BACKUP_PODEXECACCOUNTNAME` set the service account name that should be used for the pod command execution, default: `pod-executor`
* `BACKUP_GLOBALACCESSKEYID` set the S3 access key id to be used globaly
* `BACKUP_GLOBALSECRETACCESSKEY` set the S3 secret access key to be used globaly
* `BACKUP_GLOBALREPOPASSWORD` set the restic repository password to be used globaly
* `BACKUP_GLOBALKEEPJOBS` set the count of jobs to keep globally
* `BACKUP_GLOBALS3ENDPOINT` set the S3 endpoint to be used globally
* `BACKUP_GLOBALS3BUCKET` set the S3 bucket to be used globally
* `BACKUP_GLOBALSTATSURL` set the URL for wrestic to post additional metrics gloablly, default `""`
* `BACKUP_GLOBALRESTORES3BUCKET` set the global restore S3 bucket for restores
* `BACKUP_GLOBALRESTORES3ENDPOINT` set the global restore S3 endpoint for the restores (needs the scheme [http/https]
* `BACKUP_GLOBALRESTORES3ACCESKEYID` set the global resotre S3 accessKeyID for restores
* `BACKUP_GLOBALRESTORES3SECRETACCESSKEY` set the global restore S3 SecretAccessKey for restores

You only need to adjust `BACKUP_IMAGE` everything else can be left default.

# Description of the various CRDs

## Schedule CRD
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

## Restore CRD
It's now possible to define various restore jobs. Currently these kinds of restores are supported:

* To a PVC
* To S3 as tar.gz

Example for a restore to a PVC:

```yaml
apiVersion: appuio.ch/v1alpha1
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

## Archive CRD
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

## Backup CRD
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

## Check CRD
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

## Prune CRD
This will trigger a single prune run and delete the snapshots according to the defined retention rules. This one needs to run exclusively on the repository. No other jobs must run on the same repository while this one is still running.

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

# Manual restore
To manually restore you'll need:
* Linux machine with restic https://github.com/restic/restic
* Fuse

Let's take the `backend` part of the above example resource:
```yaml
backend:
  password: asdf # The restic encryption password
  s3: # Self explaining
    endpoint: http://10.144.1.133:9000
      bucket: baas
      username: 8U0UDNYPNUDTUS1LIAF3
      password: ip3cdrkXcHmH4S7if7erKPNoxDn27V0vrg6CHHem
```
**Note:** future versions may move the credentials to the Kubernetes/OpenShift secrets store.

You can use these key/value pairs to configure restic:

```bash
export RESTIC_REPOSITORY=s3:http://10.144.1.133:9000/baas
export RESTIC_PASSWORD=asdf
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
5ed64a2d  2018-06-08 09:18:34  macbook-vshn.local              /Users/simonbeck/go/src/git.vshn.net/vshn/baas/vendor
----------------------------------------------------------------------
1 snapshots

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
