## Next release

- TODO

## 0.8.16

**Release date:** 07 Feb 2025

![Helm: v3](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0) ![AppVersion: v1.8.0](https://img.shields.io/badge/v1.8.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%23v180)

- add `.Values.server.vmServiceScrape` for [VMOperator](https://docs.victoriametrics.com/operator/) [VMServiceScrape](https://docs.victoriametrics.com/operator/api/#vmservicescrape) resource
- update victorialogs version to [v1.8.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.8.0-victorialogs)

## 0.8.15

**Release date:** 06 Feb 2025

![Helm: v3](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0) ![AppVersion: v1.6.1](https://img.shields.io/badge/v1.6.1-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%23v161)

- added ability to override default headless service .Values.server.service.clusterIP with empty value
- vector chart 0.37.x -> 0.40.x
- updated common dependency 0.0.37 -> 0.0.39

## 0.8.14

**Release date:** 20 Jan 2025

![Helm: v3](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0) ![AppVersion: v1.6.1](https://img.shields.io/badge/v1.6.1-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%23v161)

- update victorialogs version to v1.6.1
- add `.Values.statefulSet.updateStrategy`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1928) for details.

## 0.8.13

**Release date:** 14 Jan 2025

![Helm: v3](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0) ![AppVersion: v1.5.0](https://img.shields.io/badge/v1.5.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%23v150)

- victorialogs version: v1.4.0 -> v1.5.0

## 0.8.12

**Release date:** 06 Jan 2025

![Helm: v3](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0) ![AppVersion: v1.4.0](https://img.shields.io/badge/v1.4.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%23v140)

- updated common dependency 0.0.34 -> 0.0.37
- Exclude markdown files from package
- victorialogs version: v1.3.2 -> v1.4.0
- support templating in `.Values.extraObjects`

## 0.8.11

**Release date:** 2024-12-09

![AppVersion: v1.3.2](https://img.shields.io/badge/v1.3.2-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v132)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- victorialogs version: v1.1.0 -> v1.3.2

## 0.8.10

**Release date:** 2024-12-05

![AppVersion: v1.1.0](https://img.shields.io/badge/v1.1.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v110)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- added `server.retentionDiskUsage` to control [retention by disk space](https://docs.victoriametrics.com/victorialogs/#retention-by-disk-space-usage). See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1845)
- victorialogs version: v1.0.0 -> v1.1.0

## 0.8.9

**Release date:** 2024-12-03

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- do not set clusterIP value, when service type is not ClusterIP. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1838)

## 0.8.8

**Release date:** 2024-12-02

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- enable watch-config in vector by default
- exclude default service port, when serviceMonitor.targetPort is defined

## 0.8.7

**Release date:** 2024-11-28

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- updated common dependency 0.0.32 -> 0.0.33

## 0.8.6

**Release date:** 2024-11-28

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Use only pod FQDN for statefulsets in `vm.es.urls` template

## 0.8.5

**Release date:** 2024-11-27

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Added `vector.customConfigNamespace` to force Vector configmap creation in a given namespace even if vector.enabled: false
- Added `service.targetPort` and `serviceMonitor.targetPort` to add ability to point service to one of extraContainers port, like oauth2-proxy

## 0.8.4

**Release date:** 2024-11-26

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- updated common dependency 0.0.31 -> 0.0.32
- synced rules and dashboards
- added .Values.dashboards.namespace to override default namespace for dashboards

## 0.8.3

**Release date:** 2024-11-25

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- removed duplicated app label from dashboards. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1795).
- synced dashboards

## 0.8.2

**Release date:** 2024-11-22

![AppVersion: v1.0.0](https://img.shields.io/badge/v1.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v100)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Removed redundant `VECTOR_SELF_NODE_NAME` env variable from vector values. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1727).
- Added Vector dashboard. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1721).
- updated common dependency 0.0.23 -> 0.0.31
- fixed app.kubernetes.io/version tag override if custom tag is set. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1766).
- Upgraded VictoriaLogs 0.41.0 -> 1.0.0

## 0.8.1

**Release date:** 2024-11-08

![AppVersion: v0.41.0](https://img.shields.io/badge/v0.41.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0410)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- updated common dependency 0.0.21 -> 0.0.23
- added `log.message` to a list of default vector message fields

## 0.8.0

**Release date:** 2024-11-08

![AppVersion: v0.41.0](https://img.shields.io/badge/v0.41.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0410)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

**Update note 1**: In a response to multiple collector related configuration feature requests and inability to use fluent-bit YAML configuration parser to provide
ability easily override parts of default configuration we've considered using Vector as a default logs collector instead of fluent-bit.
Please consider using [vector chart docs](https://github.com/vectordotdev/helm-charts/tree/develop/charts/vector) values under `vector` key
to update your agent configuration if you are using it.

- Allow selecting multiple/all instances in a dashboard. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1699)
- Upgraded VictoriaLogs 0.40.0 -> 0.41.0
- Replaced default fluent-bit collector with vector
- Parsed json fields are now nested into `log` key. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1708)
- updated common dependency 0.0.19 -> 0.0.21

## 0.7.3

**Release date:** 2024-11-06

![AppVersion: v0.40.0](https://img.shields.io/badge/v0.40.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0400)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- set default DNS domain to `cluster.local.`
- made message, time, stream fields configurable. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1695)

## 0.7.2

**Release date:** 2024-11-06

![AppVersion: v0.40.0](https://img.shields.io/badge/v0.40.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0400)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- added ability to override PVC name for Deployment
- updated dashboards
- added JSON parsing
- upgraded victorialogs 0.37.0 -> 0.40.0
- made configurable service port in fluent-bit output config. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1693)

## 0.7.1

**Release date:** 2024-10-25

![AppVersion: v0.37.0](https://img.shields.io/badge/v0.37.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0370)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Support multiple paths for a single ingress domain

## 0.7.0

**Release date:** 2024-10-25

![AppVersion: v0.37.0](https://img.shields.io/badge/v0.37.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0370)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Added grafana dashboard. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1590)
- Custom fluent-bit template to push data to multiple VLogs instances when replica count is greater than 1

## 0.6.6

**Release date:** 2024-10-11

![AppVersion: v0.29.0](https://img.shields.io/badge/v0.29.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0290)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Human-readable error about Helm version requirement

## 0.6.5

**Release date:** 2024-10-04

![AppVersion: v0.29.0](https://img.shields.io/badge/v0.29.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0290)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- upgraded common chart dependency

## 0.6.4

**Release date:** 2024-09-23

![AppVersion: v0.29.0](https://img.shields.io/badge/v0.29.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0290)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- VictoriaLogs 0.29.0
- Fixed host template in default fluent-bit output configuration

## 0.6.3

**Release date:** 2024-09-16

![AppVersion: v0.28.0](https://img.shields.io/badge/v0.28.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0280)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Avoid variant if tag is set explicitly

## 0.6.2

**Release date:** 2024-09-12

![AppVersion: v0.28.0](https://img.shields.io/badge/v0.28.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0280)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables
- Made replicas configurable
- Allow override default for statefulset headless service

## 0.6.1

**Release date:** 2024-09-03

![AppVersion: v0.28.0](https://img.shields.io/badge/v0.28.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0280)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

- Added ability to configure container port
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)
- Renamed `.Values.server.persistentVolume.storageClass` to `.Values.server.persistentVolume.storageClassName`
- Removed necessity to set `.Values.server.persistentVolume.existingClaim` when volume is expected to be created by chart. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/189)
- Fixed PVC in StatefulSet

## 0.6.0

**Release date:** 2024-08-21

![AppVersion: v0.28.0](https://img.shields.io/badge/v0.28.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0280)
![Helm: v3.14](https://img.shields.io/badge/Helm-v3.14%2B-informational?color=informational&logo=helm&link=https%3A%2F%2Fgithub.com%2Fhelm%2Fhelm%2Freleases%2Ftag%2Fv3.14.0)

**Update note**: The VictoriaMetrics components image tag template has been updated. This change introduces `.Values.<component>.image.variant` to specify tag suffixes like `-scratch`, `-cluster`, `-enterprise`. Additionally, you can now omit `.Values.<component>.image.tag` to automatically use the version specified in `.Chart.AppVersion`.

**Update note**: main container name was changed to `vlogs`, which will recreate a pod.

**Update note**: requires Helm 3.14+

- Added `basicAuth` support for `ServiceMonitor`
- Set minimal kubernetes version to `1.25`
- Removed support for `policy/v1beta1/PodDisruptionBudget`
- Updated `.Values.server.readinessProbe` to `.Values.server.probe.readiness`
- Updated `.Values.server.livenessProbe` to `.Values.server.probe.liveness`
- Updated `.Values.server.startupProbe` to `.Values.server.probe.startup`
- Added `.Values.global.imagePullSecrets` and `.Values.global.image.registry`
- Added `.Values.server.emptyDir` to customize default data directory
- Merged headless and non-headless services, removed statefulset service specific variables
- Use static container names in a pod
- Removed `networking.k8s.io/v1beta1/Ingress` and `extensions/v1beta1/Ingress` support
- Added `.Values.server.service.ipFamilies` and `.Values.server.service.ipFamilyPolicy` for service IP family management

## 0.5.4

**Release date:** 2024-07-25

![AppVersion: v0.28.0](https://img.shields.io/badge/v0.28.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0280)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- update VictoriaLogs to [v0.28.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.28.0-victorialogs).

## 0.5.3

**Release date:** 2024-07-08

![AppVersion: v0.15.0](https://img.shields.io/badge/v0.15.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0150)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- add missing API version and kind for volumeClaimTemplates, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1092).

## 0.5.2

**Release date:** 2024-06-17

![AppVersion: v0.15.0](https://img.shields.io/badge/v0.15.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0150)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- fix invalid label selector usage in notes printed after chart installation

## 0.5.1

**Release date:** 2024-05-30

![AppVersion: v0.15.0](https://img.shields.io/badge/v0.15.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v0150)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- Update VictoriaLogs to [v0.15.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.15.0-victorialogs).

## 0.5.0

**Release date:** 2024-05-23

![AppVersion: v0.8.0](https://img.shields.io/badge/v0.8.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v080)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- Update fluent-bit chart to 0.46.7 and fluentbit to 3.0.4
- Update VictoriaLogs version to 0.9.1

## 0.4.0

**Release date:** 2024-05-20

![AppVersion: v0.8.0](https://img.shields.io/badge/v0.8.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v080)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- Upgrade VictoriaLogs to [v0.8.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.8.0-victorialogs)
- Move `.Values.server.name`, `.Values.server.fullnameOverride` to `.Values.global.victoriaLogs.server`. This allows to avoid issues with Fluent Bit output definition. See the [pull request]() for the details.
- Include `kubernetes_namespace_name` field in the [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) configuration of Fluent Bit output.

## 0.3.8

**Release date:** 2024-05-10

![AppVersion: v0.5.2](https://img.shields.io/badge/v0.5.2-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v052)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- properly truncate value of `app.kubernetes.io/managed-by` and `app.kubernetes.io/instance` labels in case release name exceeds 63 characters.
- support disabling default securityContext to keep compatible with platform like openshift, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995) by @Baboulinet-33 for details.

## 0.3.7

**Release date:** 2024-04-16

![AppVersion: v0.5.2](https://img.shields.io/badge/v0.5.2-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v052)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- bump version of victorialogs to [0.5.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.2-victorialogs)

## 0.3.6

**Release date:** 2024-03-28

![AppVersion: v0.5.0](https://img.shields.io/badge/v0.5.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v050)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- support adding `metricRelabelings` for server serviceMonitor (#946)

## 0.3.5

**Release date:** 2024-03-05

![AppVersion: v0.5.0](https://img.shields.io/badge/v0.5.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v050)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- bump version of vlogs single to [0.5.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.0-victorialogs)

## 0.3.4

**Release date:** 2023-11-15

![AppVersion: v0.4.2](https://img.shields.io/badge/v0.4.2-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v042)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- bump version of vlogs single to [0.4.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.2-victorialogs)

## 0.3.3

**Release date:** 2023-10-10

![AppVersion: v0.4.1](https://img.shields.io/badge/v0.4.1-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v041)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- Add `kubernetes_container_name` into default [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts#stream-fields) configuration `fluent-bit`.

## 0.3.2

**Release date:** 2023-10-04

![AppVersion: v0.4.1](https://img.shields.io/badge/v0.4.1-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v041)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

- bump version of vlogs single to [0.4.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.1-victorialogs)

## 0.3.1

**Release date:** 2023-09-13

![AppVersion: v0.3.0](https://img.shields.io/badge/v0.3.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v030)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* added: extraObjects: [] for dynamic supportive objects configuration

## 0.3.0

**Release date:** 2023-08-15

![AppVersion: v0.3.0](https://img.shields.io/badge/v0.3.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v030)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* vlogs-single: update to 0.3.0 (#598)
* Remove repeated volumeMounts section (#610)

## 0.1.3

**Release date:** 2023-07-27

![AppVersion: v0.3.0](https://img.shields.io/badge/v0.3.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v030)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* vlogs-single: fix podSecurityContext and securityContext usage (#597)
* charts/victoria-logs-single: fix STS render when using statefulset is disabled (#585)
* charts/victoria-logs-single: add imagePullSecrets (#586)

## 0.1.2

**Release date:** 2023-06-23

![AppVersion: v0.1.0](https://img.shields.io/badge/v0.1.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v010)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* bump version of logs single
* Fix wrong condition on fluent-bit dependency (#568)

### Default value changes

```diff
# No changes in this release
```

## 0.1.1

**Release date:** 2023-06-22

![AppVersion: v0.1.0](https://img.shields.io/badge/v0.1.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v010)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* charts/victoria-logs-single: template Host field (#566)

## 0.1.0

**Release date:** 2023-06-22

![AppVersion: v0.1.0](https://img.shields.io/badge/v0.1.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v010)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* fix the chart image and jsonline endpoint
* add victoria-logs to build process, make package
* charts/victoria-logs-single: add fluentbit setup (#563)

## 0.0.1

**Release date:** 2023-06-12

![AppVersion: v0.0.0](https://img.shields.io/badge/v0.0.0-success?logo=VictoriaMetrics&labelColor=gray&link=https%3A%2F%2Fdocs.victoriametrics.com%2Fvictorialogs%2Fchangelog%2F%23v000)
![Helm: v3](https://img.shields.io/badge/Helm-v3-informational?color=informational&logo=helm)

* charts/victoria-logs-single: add new chart (#560)
