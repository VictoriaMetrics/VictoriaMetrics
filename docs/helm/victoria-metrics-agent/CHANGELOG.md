## Next release

- TODO

## 0.14.3

**Release date:** 2024-10-21

![AppVersion: v1.105.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.105.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.105.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.105.0)

## 0.14.2

**Release date:** 2024-10-11

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Human-readable error about Helm version requirement

## 0.14.1

**Release date:** 2024-10-04

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- upgraded common chart dependency

## 0.14.0

**Release date:** 2024-10-02

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.104.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.104.0)

## 0.13.0

**Release date:** 2024-09-27

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: `remoteWriteUrls` and `multiTenantUrls` parameters were replaced by `remoteWrite`. Please follow [upgrade guide](./README.md#upgrade-to-0130)

- Fail if no remoteWriteUrls set
- Added `remoteWrite` array param, which can contain all `remoteWrite.*` flag values. Please check chart docs for details.

## 0.12.2

**Release date:** 2024-09-12

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables
- Removed deprecated API from RBAC. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1438)

## 0.12.1

**Release date:** 2024-09-03

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed PVC in StatefulSet

## 0.12.0

**Release date:** 2024-08-29

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.103.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.103.0)
- Added ability to configure container port
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)
- Removed necessity to set `.Values.persistentVolume.existingClaim` when volume is expected to be created by chart. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/189)

## 0.11.0

**Release date:** 2024-08-21

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: main container name was changed to `vmagent`, which will recreate a pod.

**Update note**: requires Helm 3.14+

- Removed `PodSecurityPolicy`
- Set minimal kubernetes version to `1.25`
- Removed support for `policy/v1beta1/PodDisruptionBudget`
- Added params to configure probes `.Values.probe.readiness`, `.Values.probe.liveness` and `.Values.probe.startup`
- Added `.Values.global.imagePullSecrets` and `.Values.global.image.registry`
- Added `.Values.emptyDir` to customize default cache directory
- Use static container names in a pod
- Removed `networking.k8s.io/v1beta1/Ingress` and `extensions/v1beta1/Ingress` support
- Added `.Values.service.ipFamilies` and `.Values.service.ipFamilyPolicy` for service IP family management

## 0.10.14

**Release date:** 2024-08-14

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- add basicAuth support for ServiceMonitor
- fix license volume mount when using statefulset mode

## 0.10.13

**Release date:** 2024-08-01

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.1)

## 0.10.12

**Release date:** 2024-08-01

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix possible template error when `.Values.rbac.namespaced=true`. Thanks to @guptaachin for [the pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1179).

## 0.10.11

**Release date:** 2024-07-23

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.0)

## 0.10.10

**Release date:** 2024-07-08

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- add missing API version and kind for volumeClaimTemplates, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1092).

## 0.10.9

**Release date:** 2024-06-14

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: The VictoriaMetrics components image tag template has been updated. This change introduces `.Values.<component>.image.variant` to specify tag suffixes like `-scratch`, `-cluster`, `-enterprise`. Additionally, you can now omit `.Values.<component>.image.tag` to automatically use the version specified in `.Chart.AppVersion`.

- support specifying image tag suffix like "-enterprise" for VictoriaMetrics components using `.Values.<component>.image.variant`.

## 0.10.8

**Release date:** 2024-05-16

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix lost customized securityContext when introduced new default behavior for securityContext in [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995).

## 0.10.7

**Release date:** 2024-05-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- support disabling default securityContext to keep compatible with platform like openshift, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995) by @Baboulinet-33 for details.

## 0.10.6

**Release date:** 2024-04-26

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- properly truncate value of `app.kubernetes.io/managed-by` and `app.kubernetes.io/instance` labels in case release name exceeds 63 characters.
- bump version of VM components to [v1.101.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.101.0)

## 0.10.5

**Release date:** 2024-04-16

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.100.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.100.1)

## 0.10.4

**Release date:** 2024-03-28

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to use slice variables in extraArgs (#944)
- support adding `metricRelabelings` for server serviceMonitor (#946)

## 0.10.3

**Release date:** 2024-03-11

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Actually fix templating of HPA for vmagent. Previously, it was rendered with incorrect `kind` and thus was unusable. See [this pull request](https://github.com/VictoriaMetrics/helm-charts/pull/922).

## 0.10.2

**Release date:** 2024-03-05

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.99.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.99.0)

## 0.10.1

**Release date:** 2024-03-05

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix templating of HPA for vmagent. Previously, it was rendered with incorrect `kind` and thus was unusable. See [this pull request](https://github.com/VictoriaMetrics/helm-charts/pull/905).

## 0.10.0

**Release date:** 2024-02-28

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add ability to use HPA for vmagent. See [this](https://github.com/VictoriaMetrics/helm-charts/pull/863) pull request.

## 0.9.17

**Release date:** 2024-02-01

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.97.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.97.1)

## 0.9.16

**Release date:** 2023-12-20

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add init containers for deployment and statefulset. See [#806](https://github.com/VictoriaMetrics/helm-charts/pull/806) by @MemberIT.

## 0.9.15

**Release date:** 2023-12-13

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix configuration of volume mount for license key referenced by using secret.

## 0.9.14

**Release date:** 2023-12-12

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.96.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.96.0)

## 0.9.13

**Release date:** 2023-11-16

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.1)

## 0.9.12

**Release date:** 2023-11-15

![AppVersion: v1.95.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.0)

## 0.9.11

**Release date:** 2023-10-04

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.94.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.94.0)
- Add support of providing enterprise license key for VictoriaMetrics enterprise. See [these docs](https://docs.victoriametrics.com/enterprise) for details.

## 0.9.10

**Release date:** 2023-09-21

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5)

## 0.9.9

**Release date:** 2023-09-11

![AppVersion: v1.93.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.4)

## 0.9.8

**Release date:** 2023-09-04

![AppVersion: v1.93.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of vmagent to `v1.93.3`

## 0.9.6

**Release date:** 2023-08-30

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix `honor_timestamps` field for cadvisor scrape job (#626)

## 0.9.5

**Release date:** 2023-08-24

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Disable `honorTimestamps` for cadvisor scrape job by default (#617)

## 0.9.4

**Release date:** 2023-08-23

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.93.0 to v1.93.1

## 0.9.3

**Release date:** 2023-08-12

![AppVersion: v1.93.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.1 to v1.93.0

## 0.9.2

**Release date:** 2023-07-28

![AppVersion: v1.92.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.0 to v1.92.1 (#599)

## 0.9.1

**Release date:** 2023-07-27

![AppVersion: v1.92.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.91.3 to v1.92.0
- make package merge gen-docs

## 0.9.0

**Release date:** 2023-07-13

![AppVersion: v1.91.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.91.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of agent, alert, auth, cluster, single
- feat(vmagent): Add support for topologySpreadConstraints field (#583)
- Feat: Add custom objects to vma helm charts (#558)
- fix(vmagent): Use endpoints instead of endpointslices on kubernetes-apiservers (#574)
