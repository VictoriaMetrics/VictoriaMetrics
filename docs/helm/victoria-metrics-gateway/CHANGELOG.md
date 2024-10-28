## Next release

- TODO

## 0.5.3

**Release date:** 2024-10-21

![AppVersion: v1.105.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.105.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.105.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.105.0)

## 0.5.2

**Release date:** 2024-10-11

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Human-readable error about Helm version requirement

## 0.5.1

**Release date:** 2024-10-04

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- upgraded common chart dependency

## 0.5.0

**Release date:** 2024-10-02

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.104.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.104.0)

## 0.4.0

**Release date:** 2024-09-12

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Deprecated `rateLimiter.enable` in a favour of `rateLimiter.enabled`
- Deprecated `auth.enable` in a favour of `auth.enabled`
- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables

## 0.3.0

**Release date:** 2024-08-29

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.103.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.103.0)
- Added ability to configure container port
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)
- Removed `eula` support

## 0.2.0

**Release date:** 2024-08-21

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: main container name was changed to `vmgateway`, which will reacreate a pod.

**Update note**: requires Helm 3.14+

- Added `basicAuth` support for `ServiceMonitor`
- Set minimal kubernetes version to `1.25`
- Removed support for `policy/v1beta1/PodDisruptionBudget`
- Added params to configure probes `.Values.probe.readiness`, `.Values.probe.liveness` and `.Values.probe.startup`
- Added `.Values.global.imagePullSecrets` and `.Values.global.image.registry`
- Use static container names in a pod
- Added `.Values.service.ipFamilies` and `.Values.service.ipFamilyPolicy` for service IP family management

## 0.1.64

**Release date:** 2024-08-01

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.1)

## 0.1.63

**Release date:** 2024-07-23

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.0)

## 0.1.62

**Release date:** 2024-06-14

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: The VictoriaMetrics components image tag template has been updated. This change introduces `.Values.<component>.image.variant` to specify tag suffixes like `-scratch`, `-cluster`, `-enterprise`. Additionally, you can now omit `.Values.<component>.image.tag` to automatically use the version specified in `.Chart.AppVersion`.

- support specifying image tag suffix like "-enterprise" for VictoriaMetrics components using `.Values.<component>.image.variant`.

## 0.1.61

**Release date:** 2024-05-16

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix lost customized securityContext when introduced new default behavior for securityContext in [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995).

## 0.1.60

**Release date:** 2024-05-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- support disabling default securityContext to keep compatible with platform like openshift, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995) by @Baboulinet-33 for details.

## 0.1.59

**Release date:** 2024-04-26

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.101.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.101.0)

## 0.1.58

**Release date:** 2024-04-16

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.100.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.100.1)

## 0.1.57

**Release date:** 2024-03-28

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to use slice variables in extraArgs (#944)
- support adding `metricRelabelings` for server serviceMonitor (#946)

## 0.1.56

**Release date:** 2024-03-05

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.99.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.99.0)

## 0.1.55

**Release date:** 2024-02-01

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.97.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.97.1)

## 0.1.54

**Release date:** 2023-12-13

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix configuration of volume mount for license key referenced by using secret.
- Fix `vmbackupmanager` flags configuration when providing license key via secret.

## 0.1.53

**Release date:** 2023-12-12

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.96.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.96.0)

## 0.1.52

**Release date:** 2023-11-16

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.1)

## 0.1.51

**Release date:** 2023-11-15

![AppVersion: v1.95.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.0)

## 0.1.50

**Release date:** 2023-10-25

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix deployment `volumeMounts` when providing enterprise license key for VictoriaMetrics enterprise. See [this pr](https://github.com/VictoriaMetrics/helm-charts/pull/734) for details.

## 0.1.49

**Release date:** 2023-10-04

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.94.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.94.0)
- Add support of providing enterprise license key for VictoriaMetrics enterprise. See [these docs](https://docs.victoriametrics.com/enterprise) for details.

## 0.1.48

**Release date:** 2023-09-21

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5)

## 0.1.47

**Release date:** 2023-09-11

![AppVersion: v1.93.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.4)

## 0.1.46

**Release date:** 2023-09-04

![AppVersion: v1.93.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of vmgateway to `v1.93.3`

## 0.1.43

**Release date:** 2023-08-23

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.93.0 to v1.93.1

## 0.1.42

**Release date:** 2023-08-12

![AppVersion: v1.93.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.1 to v1.93.0

## 0.1.41

**Release date:** 2023-07-28

![AppVersion: v1.92.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.0 to v1.92.1 (#599)

## 0.1.40

**Release date:** 2023-07-27

![AppVersion: v1.92.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.91.3 to v1.92.0
