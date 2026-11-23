{{ template "chart.header" . }}
{{ template "chart.deprecationWarning" . }}

{{ template "chart.badgesSection" . }}

{{ template "chart.description" . }}

{{ template "chart.homepageLine" . }}

## Installation

```bash
helm repo add k8up-io https://k8up-io.github.io/k8up
helm install {{ template "chart.name" . }} k8up-io/{{ template "chart.name" . }}
```

The chart is also published to an OCI registry:

```bash
helm install {{ template "chart.name" . }} oci://ghcr.io/k8up-io/k8up-chart/{{ template "chart.name" . }}
```

The OCI chart is signed with [Cosign](https://docs.sigstore.dev/) (keyless). To verify the signature:

```bash
cosign verify ghcr.io/k8up-io/k8up-chart/{{ template "chart.name" . }}:<version> \
  --certificate-identity-regexp '^https://github\.com/k8up-io/k8up/\.github/workflows/chart-release\.yml@refs/tags/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
