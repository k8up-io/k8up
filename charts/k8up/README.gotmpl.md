```bash
kubectl apply -f https://github.com/k8up-io/k8up/releases/download/{{ template "chart.appVersion" . }}/k8up-crd.yaml
```

<!---
The README.md file is automatically generated with helm-docs!

Edit the README.gotmpl.md template instead.
-->

## Handling CRDs

* Always upgrade the CRDs before upgrading the Helm release.
* Watch out for breaking changes in the {{ title .Name }} release notes.

{{ template "chart.sourcesSection" . }}

{{ template "chart.requirementsSection" . }}
<!---
The values below are generated with helm-docs!

Document your changes in values.yaml and let `make docs:helm` generate this section.
-->
{{ template "chart.valuesSection" . }}

## Upgrading from Charts 0.x to 1.x

* In `image.repository` the registry domain was moved into its own parameter `image.registry`.
* K8up 1.x features leader election, this enables rolling updates and multiple replicas.
  `k8up.enableLeaderElection` defaults to `true`. Disable this for older Kubernetes versions (<= 1.15)
* `replicaCount` is now configurable, defaults to `1`.
* Note: Deployment strategy type has changed from `Recreate` to `RollingUpdate`.
* CRDs need to be installed separately, they are no longer included in this chart.

## Upgrading from Charts 1.x to 2.x

* Note: `image.repository` changed from `vshn/k8up` to `k8up-io/k8up`.
* Note: `image.registry` changed from `quay.io` to `ghcr.io`.
* Note: `image.tag` changed from `v1.x` to `v2.x`. Please see the [full changelog](https://github.com/k8up-io/k8up/releases/tag/v2.0.0).
* `metrics.prometheusRule.legacyRules` has been removed (no support for OpenShift 3.11 anymore).
* Note: `k8up.backupImage.repository` changed from `quay.io/vshn/wrestic` to `ghcr.io/k8up-io/k8up` (`wrestic` is not needed anymore in K8up v2).
