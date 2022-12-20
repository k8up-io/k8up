```bash
kubectl apply -f https://github.com/k8up-io/k8up/releases/download/k8up-{{ template "chart.version" . }}/k8up-crd.yaml
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

## Upgrading from Charts v0 to v1

* In `image.repository` the registry domain was moved into its own parameter `image.registry`.
* K8up 1.x features leader election, this enables rolling updates and multiple replicas.
  `k8up.enableLeaderElection` defaults to `true`. Disable this for older Kubernetes versions (<= 1.15)
* `replicaCount` is now configurable, defaults to `1`.
* Note: Deployment strategy type has changed from `Recreate` to `RollingUpdate`.
* CRDs need to be installed separately, they are no longer included in this chart.

## Upgrading from Charts v1 to v2

* Note: `image.repository` changed from `vshn/k8up` to `k8up-io/k8up`.
* Note: `image.registry` changed from `quay.io` to `ghcr.io`.
* Note: `image.tag` changed from `v1.x` to `v2.x`. Please see the [full changelog](https://github.com/k8up-io/k8up/releases/tag/v2.0.0).
* `metrics.prometheusRule.legacyRules` has been removed (no support for OpenShift 3.11 anymore).
* Note: `k8up.backupImage.repository` changed from `quay.io/vshn/wrestic` to `ghcr.io/k8up-io/k8up` (`wrestic` is not needed anymore in K8up v2).

## Upgrading from Charts v2 to v3

Due to the migration of the chart from [APPUiO](https://github.com/appuio/charts/tree/master/appuio/k8up) to this repo, we decided to make a breaking change for the chart.
Only chart archives from version 3.x can be downloaded from the https://k8up-io.github.io/k8up index.
No 2.x chart releases will be migrated from the APPUiO Helm repo.

Some RBAC roles and role bindings have change the name.
In most cases this shouldn't be an issue and Helm should be able to cleanup the old resources without impact on the RBAC permissions.

* New parameter: `podAnnotations`, default `{}`.
* New parameter: `service.annotations`, default `{}`.
* Parameter changed: `image.tag` now defaults to `v2` instead of a pinned version.
* Parameter changed: `image.pullPolicy` now defaults to `Always` instead of `IfNotPresent`.
* Note: Renamed ClusterRole `${release-name}-manager-role` to `${release-name}-manager`.
* Note: Spec of ClusterRole `${release-name}-leader-election-role` moved to `${release-name}-manager`.
* Note: Renamed ClusterRoleBinding `${release-name}-manager-rolebinding` to `${release-name}`.
* Note: ClusterRoleBinding `${release-name}-leader-election-rolebinding` removed (not needed anymore).
* Note: Renamed ClusterRole `${release-name}-k8up-view` to `${release-name}-view`.
* Note: Renamed ClusterRole `${release-name}-k8up-edit` to `${release-name}-edit`.

## Upgrading from Charts v3 to v4

The image tag is now pinned again and not using a floating tag.

* Parameter changed: `image.tag` now defaults to a pinned version. Each new K8up version now requires also a new chart version.
* Parameter changed: `image.pullPolicy` now defaults to `IfNotPresent` instead of `Always`.
* Parameter changed: `k8up.backupImage.repository` is now unset, which defaults to the same image as defined in `image.{registry/repository}`.
* Parameter changed: `k8up.backupImage.tag` is now unset, which defaults to the same image tag as defined in `image.tag`.
