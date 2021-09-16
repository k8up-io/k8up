[![Build](https://img.shields.io/github/workflow/status/vshn/k8up/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/vshn/k8up)
![Kubernetes version](https://img.shields.io/badge/k8s-v1.20-blue)
[![Version](https://img.shields.io/github/v/release/vshn/k8up)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/vshn/k8up)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/k8up/total)][releases]
[![Docker image](https://img.shields.io/docker/pulls/vshn/k8up)][dockerhub]
[![License](https://img.shields.io/github/license/vshn/k8up)][license]

![K8up logo](docs/modules/ROOT/assets/images/k8up-logo.svg "K8up")

# K8up Backup Operator

K8up is a Kubernetes backup operator based on [Restic](https://restic.readthedocs.io) that will handle PVC and application backups on a Kubernetes or OpenShift cluster.

Just create a `schedule` and a `credentials` object in the namespace you’d like to backup.
It’s that easy. K8up takes care of the rest. It also provides a Prometheus endpoint for monitoring.

## Documentation

The documentation is written in AsciiDoc and published with Antora to [k8up.io](https://k8up.io/).
It's source is available in the `docs/` directory.

## Contributing

K8up is written using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

You'll need:

- A running Kubernetes cluster (minishift, minikube, k3s, ... you name it)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and [kustomize](https://kubernetes-sigs.github.io/kustomize/installation/)
- Go development environment
- Your favorite IDE (with a Go plugin)
- Docker
- make
  
To run the end-to-end test (e.g. `make e2e-test`), you additionally need:

- `helm` (version 3)
- `jq`
- `node` and `npm`
- `bash` (installed, doesn't have to be your default shell)
- `shasum` or `sha1sum`
- `base64`
- `find`

These are the most common make targets: `build`, `test`, `docker-build`, `run`, `kind-run`.
Run `make help` to get an overview over the relevant targets and their intentions.

### Code Structure

K8s consists of two main modules:

- The _operator_ module is the part that runs constantly within K8s and contains the various reconciliation loops.
- The _restic_ module is our interface to the `restic` binary and is invoked whenever a `Backup` or `Restore` (or similar) custom resource is instantiated.
  If it's job (like doing a backup or a restore) is done, the process ends.

```asciidoc
/
- api           Go Types for the Custom Resource Definitions (CRDs) [o]
- cmd           CLI definition and entrypoints
- common        Code that is not specific to either
- config        Various configuration files for the Operator SDK [o]
- controllers   The reconciliation loops of the operator module [o]
- docs          Out ASCIIdoc code as published on https://k8up.io
- e2e           The Bats-based End-To-End tests
- envtest       Infrastructure code for the integration tests
- operator      Code that is otherwise related to the _operator module_,
                but not part of the recommended Operator SDK structure.
- restic        Code that makes up the _restic module_.

[o]: this is part of the recommended Operator SDK structure
```

### Generate Kubernetes code

If you make changes to the CRD structs you'll need to run code generation.
This can be done with make:

```bash
make generate
```

### Install CRDs

CRDs can be either installed on the cluster by running `make install` or using `kubectl apply -k config/crd/apiextensions.k8s.io/v1`.

Currently there's an issue using [`make install`](https://github.com/kubernetes-sigs/kubebuilder/issues/1544) related to how the CRDs are specified.
Therefore settle to the second approach for now.

### Run the operator

You can run the operator in different ways:

1. as a container image (see [quickstart](https://sdk.operatorframework.io/docs/building-operators/golang/quickstart/))
2. using `make run-operator` (provide your own kubeconfig)
3. using `make kind-run` (uses KIND to install a cluster in docker and provides its own kubeconfig in `testbin/`)
4. using a configuration of your favorite IDE

Best is if you have [minio](https://min.io/download) installed somewhere to be able to setup the needed env values. It needs to be reachable from within your dev cluster.

### Run E2E tests

K8up supports both OpenShift 3.11 clusters and newer Kubernetes clusters 1.16+.
However, to support OpenShift 3.11 a legacy CRD definition with `apiextensions.k8s.io/v1beta1` is needed, while K8s 1.22+ only supports `apiextensions.k8s.io/v1`.
You need `node` and `npm` to run the tests, as it runs with [DETIK][detik].

To run e2e tests, execute:

```bash
make e2e-test
```

To test compatibility of k8up with OpenShift 3.11 (or any other specific K8s version), you can run end-to-end tests like this:

```bash
make e2e-test -e CRD_SPEC_VERSION=v1beta1 -e KIND_NODE_VERSION=v1.13.12 -e KIND_KUBECTL_ARGS=--validate=false -e BACKUP_ENABLE_LEADER_ELECTION=false
```

To test just a specific e2e test, run:

```bash
make e2e-test -e BATS_FILES=test-00-deployment.bats
```

To remove the local KIND cluster and other e2e resources, run:

```bash
make e2e-clean
```

To cleanup all created artifacts, there's always:

```bash
make clean
```

### Example configurations

There are a number of example configurations in [`config/samples`](config/samples).
Apply them using `kubectl apply -f config/samples/somesample.yaml`

[build]: https://github.com/vshn/k8up/actions?query=workflow%3ATest
[releases]: https://github.com/vshn/k8up/releases
[license]: https://github.com/vshn/k8up/blob/master/LICENSE
[dockerhub]: https://hub.docker.com/r/vshn/k8up
[detik]: https://github.com/bats-core/bats-detik
[codeclimate]: https://codeclimate.com/github/vshn/k8up
