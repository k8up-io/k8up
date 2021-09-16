{{- define "gvList" -}}
{{- $groupVersions := . -}}

// Generated documentation. Please do not edit.
:anchor_prefix: k8s-api

[id="api-reference"]
= API Reference

This is a https://github.com/elastic/crd-ref-docs[generated] API documentation.

TIP: A more sophisticated documentation is available under https://doc.crds.dev/github.com/k8up-io/k8up.

.Packages
{{- range $groupVersions }}
- {{ asciidocRenderGVLink . }}
{{- end }}

{{ range $groupVersions }}
{{ template "gvDetails" . }}
{{ end }}

{{- end -}}
