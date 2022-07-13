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
