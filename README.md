![Build status](https://api.travis-ci.com/vshn/k8up.svg?branch=master)

<img src="https://raw.githubusercontent.com/vshn/k8up/master/docs/images/logo.png" width="150">

# Overview

K8up is a backup operator that will handle PVC and app backups on a k8s/OpenShift cluster.

Just create a `schedule` object in the namespace you’d like to backup. It’s that easy. K8up takes care of the rest. It also provides a Prometheus endpoint for monitoring.

K8up is currently under heavy development and far from feature complete. But it should already be stable enough for production use.

# Documentation

The docs are WIP: https://vshn.github.io/k8up/

# Dev Environment
You'll need:

* Minishift or Minikube
* golang installed :) (everything is tested with 1.11.3)
* dep installed
* Your favorite IDE (with a golang plugin)
* docker
* make

## Generate kubernetes code
If you make changes to the CRD structs you'll need to run code generation. This can be done with make:

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
docker build -t $(minishift openshift registry)/myproject/baas:0.0.1 .
docker save $(minishift openshift registry)/myproject/baas:0.0.1 > tmp.tar
eval $(minishift docker-env)
docker load -i tmp.tar
rm tmp.tar
docker login -u developer -p $(oc whoami -t) $(minishift openshift registry)
docker push $(minishift openshift registry)/myproject/baas:0.0.1
```

# Docker tags

(AMD64/x86 arch only)

* latest: master branch
* dev: dev branch (useful for pre-releases)
* versioned: tagged git releases
