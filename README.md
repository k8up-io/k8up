![Build status](https://api.travis-ci.com/vshn/k8up.svg?branch=rewrite)

<img src="https://raw.githubusercontent.com/vshn/k8up/master/docs/images/logo.png" width="150">

# Overview

K8up is a backup operator that will handle PVC and app backups on a k8s/OpenShift cluster.

Just create a `schedule` and a `credentials` object in the namespace you’d like to backup. It’s that easy. K8up takes care of the rest. It also provides a Prometheus endpoint for monitoring.

K8up is currently under heavy development and far from feature complete. But it should already be stable enough for production use.

# Documentation

The documentation is published here: https://k8up.io/

# Contributing

K8up is an operator written using the [Operator SDK](https://sdk.operatorframework.io/docs).

You'll need:

- A running kubernetes cluster (minishift, minikube, k3s, ... you name it)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and [kustomize](https://kubernetes-sigs.github.io/kustomize/installation/)
- golang installed :) (everything is tested with 1.13)
- Your favorite IDE (with a golang plugin)
- docker
- make

## Generate kubernetes code

If you make changes to the CRD structs you'll need to run code generation. This can be done with make:

```
make manifests
```

## Install CRDs

CRDs can be either installed on the cluster by running `make install` or using `kubectl apply -k config/crd`.

Currently there's an issue using [`make install`](https://github.com/kubernetes-sigs/kubebuilder/issues/1544) related to how the CRDs are specified.
Therefore settle to the second approach for now.

## Run the operator

You can run the operator/manager in three different ways:

1. as a docker image (see [quickstart](https://sdk.operatorframework.io/docs/building-operators/golang/quickstart/))
2. using `make run`
3. using a configuration of your favorite IDE (see below for VSCode example)

Example VSCode run configuration:

```
{
  // Use IntelliSense to learn about possible attributes.
  // Hover to view descriptions of existing attributes.
  // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/main.go",
      "env": {
        "BACKUP_IMAGE": "vshn/wrestic:v0.2.0",
        "BACKUP_GLOBALS3ENDPOINT": "http://somewhere.example.org",
        "BACKUP_GLOBALS3BUCKET": "somebucket",
        "BACKUP_GLOBALSECRETACCESSKEY": "replacewithaccesskey",
        "BACKUP_GLOBALACCESSKEYID": "replacewithkeyid",
        "BACKUP_GLOBALREPOPASSWORD": "somepassword"
      },
      "args": []
    }
  ]
}
```

Best is if you have [minio](https://min.io/download) installed somewhere to be able to setup the needed env values. It needs to be reachable from within your dev cluster.

## Example configurations

There are a number of example configurations in [`config/samples`](config/samples). Apply them using `kubectl apply -f config/samples/somesample.yaml`
