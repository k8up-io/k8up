[![Build](https://img.shields.io/github/actions/workflow/status/k8up-io/k8up/.github/workflows/test.yml?branch=master)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/k8up-io/k8up)
[![Version](https://img.shields.io/github/v/release/k8up-io/k8up)][releases]
[![GitHub downloads](https://img.shields.io/github/downloads/k8up-io/k8up/total)][releases]
[![License](https://img.shields.io/github/license/k8up-io/k8up)][license]
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/5388/badge)](https://bestpractices.coreinfrastructure.org/projects/5388)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/k8up-io/k8up/badge)](https://api.securityscorecards.dev/projects/github.com/k8up-io/k8up)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/k8up)](https://artifacthub.io/packages/helm/k8up/k8up)

![K8up logo](docs/modules/ROOT/assets/images/k8up-logo.svg "K8up")

# K8up Backup Operator

K8up is a Kubernetes backup operator based on [Restic](https://restic.readthedocs.io) that will handle PVC and application backups on a Kubernetes or OpenShift cluster.

Just create a `schedule` and a `credentials` object in the namespace you’d like to backup.
It’s that easy. K8up takes care of the rest. It also provides a Prometheus endpoint for monitoring.

K8up is production ready. It is used in production deployments since 2019.

## Documentation

The documentation is written in AsciiDoc and published with Antora to [k8up.io](https://k8up.io/).
It's source is available in the `docs/` directory.

Run `make docs-preview` to build the docs and preview changes.

## Contributing

K8up is written using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

You'll need:

- A running Kubernetes cluster (minishift, minikube, k3s, ... you name it)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- Go development environment
- Your favorite IDE (with a Go plugin)
- Docker
- `make`
- `sed` (or `gsed` for MacOS)

To run the end-to-end test (e.g. `make e2e-test`), you additionally need:

- `helm` (version 3)
- `jq`
- `yq`
- `node` and `npm`
- `bash` (installed, doesn't have to be your default shell)
- `base64`
- `find`

These are the most common make targets: `build`, `test`, `docker-build`, `run`, `kind-run`.
Run `make help` to get an overview over the relevant targets and their intentions.

You can find the project roadmap at [k8up.io](https://k8up.io/k8up/about/roadmap.html).

We use [Snyk](https://snyk.io/test/github/k8up-io/k8up) to test the code regularly for vulnerabilities and other security issues.

If you find any security issue, please follow our [Vulnerability Reporting](https://k8up.io/k8up/about/vulnerability_reporting.html) process.

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

CRDs can be either installed on the cluster by running `make install` or using `kubectl apply -f config/crd/apiextensions.k8s.io/v1`.

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

You need `node` and `npm` to run the tests, as it runs with [DETIK][detik].

To run e2e tests, execute:

```bash
make e2e-test
```

To test just a specific e2e test, run:

```bash
make e2e-test -e BATS_FILES=test-02-deployment.bats
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

# Community

Read more about our community [in the documentation](https://k8up.io/k8up/about/community.html).

## Chat with us

The K8up project is present in the [CNCF Slack Workspace](https://slack.cncf.io/) in the [#k8up channel](https://app.slack.com/client/T08PSQ7BQ/C06GP0D5FEF).

## Monthly community meeting

We host a monthly community meeting. For more information, head over to [the community documentation](https://k8up.io/k8up/about/community.html).

## Code of Conduct

Our code of conduct can be read at [k8up.io](https://k8up.io/k8up/about/code_of_conduct.html).

[build]: https://github.com/k8up-io/k8up/actions?query=workflow%3ATest
[releases]: https://github.com/k8up-io/k8up/releases
[license]: https://github.com/k8up-io/k8up/blob/master/LICENSE
[detik]: https://github.com/bats-core/bats-detik
