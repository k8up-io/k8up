# k8up

![Version: 3.1.0](https://img.shields.io/badge/Version-3.1.0-informational?style=flat-square)

Kubernetes and OpenShift Backup Operator based on restic

**Homepage:** <https://k8up.io/>

## Installation

```bash
helm repo add k8up-io https://k8up-io.github.io/k8up
helm install k8up k8up-io/k8up
```
```bash
kubectl apply -f https://github.com/k8up-io/k8up/releases/download/k8up-3.1.0/k8up-crd.yaml
```

<!---
The README.md file is automatically generated with helm-docs!

Edit the README.gotmpl.md template instead.
-->

## Handling CRDs

* Always upgrade the CRDs before upgrading the Helm release.
* Watch out for breaking changes in the K8up release notes.

## Source Code

* <https://github.com/k8up-io/k8up>

<!---
The values below are generated with helm-docs!

Document your changes in values.yaml and let `make docs:helm` generate this section.
-->
## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| image.pullPolicy | string | `"Always"` | Operator image pull policy |
| image.registry | string | `"ghcr.io"` | Operator image registry |
| image.repository | string | `"k8up-io/k8up"` | Operator image repository |
| image.tag | string | `"v2"` | Operator image tag (version) |
| imagePullSecrets | list | `[]` |  |
| k8up.backupImage.repository | string | `"ghcr.io/k8up-io/k8up"` | The backup runner image repository |
| k8up.backupImage.tag | string | `"v2"` | The backup runner image tag |
| k8up.enableLeaderElection | bool | `true` | Specifies whether leader election should be enabled. |
| k8up.envVars | list | `[]` | envVars allows the specification of additional environment variables. See [values.yaml](values.yaml) how to specify See documentation which variables are supported. |
| k8up.globalResources | object | empty values | Specify the resource requests and limits that the Pods should have when they are scheduled by K8up. You are still able to override those via K8up resources, but this gives cluster administrators custom defaults. |
| k8up.globalResources.limits.cpu | string | `""` | Global CPU resource limit applied to jobs. See [supported units][resource-units]. |
| k8up.globalResources.limits.memory | string | `""` | Global Memory resource limit applied to jobs. See [supported units][resource-units]. |
| k8up.globalResources.requests.cpu | string | `""` | Global CPU resource requests applied to jobs. See [supported units][resource-units]. |
| k8up.globalResources.requests.memory | string | `""` | Global Memory resource requests applied to jobs. See [supported units][resource-units]. |
| k8up.operatorNamespace | string | `""` | `DEPRECATED` This field is no longer used for recent K8up versions (v2.6+). It is still in the chart to support older image tags. |
| k8up.timezone | string | `""` | Specifies the timezone K8up is using for scheduling. Empty value defaults to the timezone in which Kubernetes is deployed. Accepts `tz database` compatible entries, e.g. `Europe/Zurich` |
| metrics.prometheusRule.additionalLabels | object | `{}` | Add labels to the PrometheusRule object |
| metrics.prometheusRule.additionalRules | list | `[]` | Provide additional alert rules in addition to the defaults |
| metrics.prometheusRule.createDefaultRules | bool | `true` | Whether the default rules should be installed |
| metrics.prometheusRule.enabled | bool | `false` | Whether to enable PrometheusRule manifest for [Prometheus Operator][prometheus-operator] |
| metrics.prometheusRule.jobFailedRulesFor | list | `["archive","backup","check","prune","restore"]` | Create default rules for the given job types. Valid values are "archive", "backup", "check", "prune", and "restore". |
| metrics.prometheusRule.namespace | string | `""` | If the object should be installed in a different namespace than operator |
| metrics.service.annotations | object | `{}` | Annotations to add to the service |
| metrics.service.nodePort | int | `0` | Service node port of the metrics endpoint, requires `metrics.service.type=NodePort` |
| metrics.service.port | int | `8080` |  |
| metrics.service.type | string | `"ClusterIP"` |  |
| metrics.serviceMonitor.additionalLabels | object | `{}` | Add labels to the ServiceMonitor object |
| metrics.serviceMonitor.enabled | bool | `false` | Whether to enable ServiceMonitor manifests for [Prometheus Operator][prometheus-operator] |
| metrics.serviceMonitor.namespace | string | `""` | If the object should be installed in a different namespace than operator |
| metrics.serviceMonitor.scrapeInterval | string | `"60s"` | Scrape interval to collect metrics |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` | Annotations to add to the Pod spec. |
| podSecurityContext | object | `{}` | Security context to add to the Pod spec. |
| rbac.create | bool | `true` | Create cluster roles and rolebinding. May need elevated permissions to create cluster roles and -bindings. |
| replicaCount | int | `1` | How many operator pods should run. Note: Operator features leader election for K8s 1.16 and later, so that only 1 pod is reconciling/scheduling jobs. Follower pods reduce interruption time as they're on hot standby when leader is unresponsive. |
| resources.limits.memory | string | `"256Mi"` | Memory limit of K8up operator. See [supported units][resource-units]. |
| resources.requests.cpu | string | `"20m"` | CPU request of K8up operator. See [supported units][resource-units]. |
| resources.requests.memory | string | `"128Mi"` | Memory request of K8up operator. See [supported units][resource-units]. |
| securityContext | object | `{}` | Container security context |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` |  |

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

## Upgrading from Charts 2.x to 3.x

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

## Source Code

* <https://github.com/k8up-io/k8up>

<!---
Common/Useful Link references from values.yaml
-->
[resource-units]: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes
[prometheus-operator]: https://github.com/coreos/prometheus-operator
