## Next release

- TODO

## 1.5.1

**Release date:** 2024-10-04

![AppVersion: v1.16.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.16.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- upgraded common chart dependency

## 1.5.0

**Release date:** 2024-10-03

![AppVersion: v1.16.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.16.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Upgraded vmanomaly to [1.16.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1161)
- Added the ability to enable persistence for models and data via `.Values.persistentVolume.dumpModels` and `.Values.persistentVolume.dumpData` variables respectively.
- Fix default `podSecurityContext` configuration to ensure fs group matches container user.
- Fix passing empty `tenant_id` in case tenant is not defined in values.

## 1.4.6

**Release date:** 2024-09-16

![AppVersion: v1.15.9](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.9&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added the ability to add annotations to the configMap using `values.configMapAnnotations`
- Fixed license file flag name

## 1.4.5

**Release date:** 2024-09-12

![AppVersion: v1.15.9](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.9&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables
- Removed vmanomaly not existing `loggerFormat` extra arg. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1476)

## 1.4.4

**Release date:** 2024-09-03

![AppVersion: v1.15.9](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.9&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed PVC in StatefulSet

## 1.4.3

**Release date:** 2024-08-27

![AppVersion: v1.15.9](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.9&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Removed `eula` support
- Disable PodMonitor, when pull port is not defined
- Upgraded application version to 1.15.9
- Fixed default podDisruptionBudget configuration

## 1.4.2

**Release date:** 2024-08-26

![AppVersion: v1.15.6](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.6&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Set minimal kubernetes version to `1.25`
- Added `.Values.global.imagePullSecrets` and `.Values.global.image.registry`
- Fixed volume template. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1280)
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)
- Renamed `.Values.persistentVolume.storageClass` to `.Values.persistentVolume.storageClassName`
- Removed necessity to set `.Values.persistentVolume.existingClaim` when it should be created by chart. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/189)
- Added PDB, PodMonitor, extra volumes and extra volumeMounts

## 1.4.1

**Release date:** 2024-08-15

![AppVersion: v1.15.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update vmanomaly to [v1.15.4](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1154).

## 1.4.0

**Release date:** 2024-08-14

![AppVersion: v1.15.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.15.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update vmanomaly to [v1.15.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1153).
- Update configuration example format to match the latest version of vmanomaly.

## 1.3.4

**Release date:** 2024-07-19

![AppVersion: v1.13.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.13.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- leave empty `schedulers` and `models` section to fix aliases error

## 1.3.3

**Release date:** 2024-07-17

![AppVersion: v1.13.2](https://img.shields.io/static/v1?label=AppVersion&message=v1.13.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix default value for `.Values.config.schedulers.class`.

## 1.3.2

**Release date:** 2024-07-17

![AppVersion: v1.13.2](https://img.shields.io/static/v1?label=AppVersion&message=v1.13.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- changes made for vmanomaly [v1.13.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1132)

## 1.3.1

**Release date:** 2024-07-08

![AppVersion: v1.13.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.13.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- add missing API version and kind for volumeClaimTemplates, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1092).

## 1.3.0

**Release date:** 2024-06-11

![AppVersion: v1.13.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.13.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add ability to configure persistent volume for vmanomaly models storage.
- Fix `.Values.podSecurityContext` not being applied to the pod.
- Update vmanomaly to [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130).

## 1.2.4

**Release date:** 2024-05-16

![AppVersion: v1.12.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.12.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix lost customized securityContext when introduced new default behavior for securityContext in [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995).

## 1.2.3

**Release date:** 2024-05-10

![AppVersion: v1.12.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.12.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- support disabling default securityContext to keep compatible with platform like openshift, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995) by @Baboulinet-33 for details.

## 1.2.2

**Release date:** 2024-04-02

![AppVersion: v1.12.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.12.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- apply [v1.12](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1120) as a default (no config changes).

## 1.2.1

**Release date:** 2024-03-20

![AppVersion: v1.11.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.11.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add support of passing preset configuration.

## 1.2.0

**Release date:** 2024-02-26

![AppVersion: v1.11.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.11.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- apply [v1.11](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110) change in [schedulers section](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/): add configuration for using multiple schedulers at once via `schedulers`. Old `scheduler` field is deprecated and will be automatically converted to `schedulers` definition starting from [v1.11](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110).
- docs fixes

## 1.1.1

**Release date:** 2024-02-20

![AppVersion: v1.10.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.10.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix passing path to license file location when using `license.secret` mount.

## 1.1.0

**Release date:** 2024-02-19

![AppVersion: v1.10.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.10.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- apply [v1.10](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1100) change in [models section](https://docs.victoriametrics.com/anomaly-detection/components/models/): add configuration for using multiple models at once via `models`. Old `model` field is deprecated and will be automatically converted to `models` definition starting from [v1.10](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1100).
- docs fixes

## 1.0.0

**Release date:** 2024-02-05

![AppVersion: v1.9.2](https://img.shields.io/static/v1?label=AppVersion&message=v1.9.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Breaking change: passing [full vmanomaly config](https://docs.victoriametrics.com/anomaly-detection/components/) via `config` parameter.
- vmanomaly image moving to DockerHub

## 0.5.0

**Release date:** 2023-10-31

![AppVersion: v1.6.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.6.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add options to use `bearer_token` for reader and writer authentication.
- Add `verify_tls` option to bypass TLS verification for reader and writer.
- Add `extra_filters` option to supply additional filters to enforce for reader queries.

## 0.4.1

**Release date:** 2023-10-10

![AppVersion: v1.5.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.5.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add an options to override default `metric_format` for remote write configuration of vmanomaly.

## 0.4.0

**Release date:** 2023-08-21

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* add ability to provide license key

## 0.3.5

**Release date:** 2023-06-22

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* bump version of vmanomaly
* charts/victoria-metrics-anomaly: fix monitoring config indentation (#567)

## 0.3.4

**Release date:** 2023-06-22

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* bump vmanomaly remove tricky make command
* charts/victoria-metrics-anomaly: make monitoring config more configurable (#562)

## 0.3.3

**Release date:** 2023-06-07

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* bump anomaly chart, make package make merge

## 0.3.2

**Release date:** 2023-06-06

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* Anomaly: change defaults (#518)
* charts/operator: update version to 0.30.4 adds extraArgs and serviceMonitor options for operator
* vmanomaly re-release

## 0.3.1

**Release date:** 2023-01-26

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* vmanomaly: fix monitoring part of config (#457)

## 0.3.0

**Release date:** 2023-01-24

![AppVersion: v1.1.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.1.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

* relase vmanomaly v1.1.0 (#454)
* vmanomaly: fix config for pull-based monitoring (#446)
