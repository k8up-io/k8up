# Operator Support Code

This Go module contains almost all of the code that supports the operator part K8up.

The CLI entrypoint is in [`cmd/operator`](../cmd/operator).

The rest of the operator's code follows the layout that [the _Operator SDK_](https://sdk.operatorframework.io/docs/building-operators/golang/) expects:

- [`/api`](../api/v1) contains the custom resource definitions (CRDs)
- [`/config`](../config) contains configuration and sample files
- [`/controllers`](../controllers) contains the controllers which act on the events of the CRDs (and other events) through reconciliation loops
