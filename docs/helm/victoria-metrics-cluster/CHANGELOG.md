## Next release

- TODO

## 0.14.2

**Release date:** 2024-10-11

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Human-readable error about Helm version requirement

## 0.14.1

**Release date:** 2024-10-04

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Support extra storageNodes. Fail if no storageNodes set
- Support enabling automatic discovery of vmstorage addresses using DNS SRV records in enterprise version
- Added HPA with scaledown disabled by default
- Allow excluding vmstorage nodes from vminsert. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1549)
- Upgraded common chart dependency

## 0.14.0

**Release date:** 2024-10-02

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.104.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.104.0)

## 0.13.7

**Release date:** 2024-09-12

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed security context for vmstorage statefulset

## 0.13.6

**Release date:** 2024-09-11

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed cluster native address port rendering on vmselect statefulset. Thanks to @Red-M

## 0.13.5

**Release date:** 2024-09-05

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed priority class name templating for vmselect deployment

## 0.13.4

**Release date:** 2024-09-05

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Removed deprecated autoscaling v2beta2 API support
- Added podLabels support

## 0.13.3

**Release date:** 2024-09-04

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed vmstorage resources templating

## 0.13.2

**Release date:** 2024-09-04

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables
- Fixed vmstorage restore cmd args template

## 0.13.1

**Release date:** 2024-09-03

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Deprecated `vmstorage.vmbackupmanager.enable` in a favour of `vmstorage.vmbackupmanager.enabled`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/940).
- Fixed PVC in StatefulSet

## 0.13.0

**Release date:** 2024-08-29

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.103.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.103.0)
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)
- Renamed `.Values.vmstorage.persistentVolume.storageClass` to `.Values.vmstorage.persistentVolume.storageClassName`
- Removed necessity to set `.Values.vmstorage.persistentVolume.existintClaim` when it should be created by chart. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/189)
- Removed `eula` support

## 0.12.1

**Release date:** 2024-08-22

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to configure container port
- Fixed volume template. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1280)

## 0.12.0

**Release date:** 2024-08-21

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: `vmselect` main container name was changed to `vmselect`, which will recreate a pod.

**Update note**: `vmstorage` main container name was changed to `vmstorage`, which will recreate a pod.

**Update note**: `vminsert` main container name was changed to `vminsert`, which will recreate a pod.

**Update note**: requires Helm 3.14+

- Added `basicAuth` support for `ServiceMonitor`
- Removed `PodSecurityPolicy`
- Set minimal kubernetes version to `1.25`
- Removed support for `policy/v1beta1/PodDisruptionBudget`
- Added ability to override liveness and readiness probes
- Updated `.Values.vmbackupmanager.readinessProbe` to `.Values.vmbackupmanager.probe.readiness`
- Updated `.Values.vmbackupmanager.livenessProbe` to `.Values.vmbackupmanager.probe.liveness`
- Updated `.Values.vmbackupmanager.startupProbe` to `.Values.vmbackupmanager.probe.startup`
- Added `.Values.global.imagePullSecrets` and `.Values.global.image.registry`
- Fix templating of Ingress port when using custom port name.
- Added `.Values.vmselect.emptyDir` and `.Values.vmstorage.emptyDir` to customize default cache directory
- Merged headless and non-headless services, removed statefulset service specific variables
- Added `.Values.vmselect.service.healthCheckNodePort` and `.Values.vmselect.service.externalTrafficPolicy`
- Added `.Values.vmstorage.service.healthCheckNodePort` and `.Values.vmstorage.service.externalTrafficPolicy`
- Added `.Values.vminsert.service.healthCheckNodePort` and `.Values.vminsert.service.externalTrafficPolicy`
- Use static container names in a pod
- Removed `networking.k8s.io/v1beta1/Ingress` and `extensions/v1beta1/Ingress` support
- Added `.Values.vmstorage.service.ipFamilies` and `.Values.vmstorage.service.ipFamilyPolicy` for service IP family management
- Added `.Values.vmselect.service.ipFamilies` and `.Values.vmselect.service.ipFamilyPolicy` for service IP family management
- Added `.Values.vminsert.service.ipFamilies` and `.Values.vminsert.service.ipFamilyPolicy` for service IP family management

## 0.11.23

**Release date:** 2024-08-01

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.1)

## 0.11.22

**Release date:** 2024-07-31

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix mount name for `vmstorage` when using `.Values.vmstorage.persistentVolume.name` to override PVC name. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1181) for the details. Thanks to @srinusanduri for [the pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1182).

## 0.11.21

**Release date:** 2024-07-23

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.0)

## 0.11.20

**Release date:** 2024-07-08

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- add missing API version and kind for volumeClaimTemplates, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1092).

## 0.11.19

**Release date:** 2024-06-14

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: The VictoriaMetrics components image tag template has been updated. This change introduces `.Values.<component>.image.variant` to specify tag suffixes like `-scratch`, `-cluster`, `-enterprise`. Additionally, you can now omit `.Values.<component>.image.tag` to automatically use the version specified in `.Chart.AppVersion`.

- fix workload's readinessProbe and livenessProbe when using custom port name. Thanks to @hanumanhuda for [the pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1061).
- support specifying image tag suffix like "-enterprise" for VictoriaMetrics components using `.Values.<component>.image.variant`.

## 0.11.18

**Release date:** 2024-05-16

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix lost customized securityContext when introduced new default behavior for securityContext in [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995).

## 0.11.17

**Release date:** 2024-05-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- support disabling default securityContext to keep compatible with platform like openshift, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/995) by @Baboulinet-33 for details.

## 0.11.16

**Release date:** 2024-04-26

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- properly truncate value of `app.kubernetes.io/managed-by` and `app.kubernetes.io/instance` labels in case release name exceeds 63 characters. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/931) and [this PR](https://github.com/VictoriaMetrics/helm-charts/pull/936).
- enable templating for `port name` so users can replace the default with custom values in use cases such as outlined in [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/972) and has been addressed in [this PR](https://github.com/VictoriaMetrics/helm-charts/pull/975).
- bump version of VM components to [v1.101.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.101.0)

## 0.11.15

**Release date:** 2024-04-16

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.100.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.100.1)

## 0.11.14

**Release date:** 2024-03-28

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to use slice variables in extraArgs (#944)
- support adding `metricRelabelings` for server serviceMonitor (#946)

## 0.11.13

**Release date:** 2024-03-05

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.99.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.99.0)

## 0.11.12

**Release date:** 2024-02-09

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Remove unsupported `scheme` field under `livenessProbe.tcpSocket`. See [#844](https://github.com/VictoriaMetrics/helm-charts/pull/844) by @MisguidedEmails.

## 0.11.11

**Release date:** 2024-02-01

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.97.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.97.1)
- Switch probes scheme to `HTTPS` if vminsert and vmselect enabled tls in extraArgs.

## 0.11.10

**Release date:** 2023-12-19

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix templating `podManagementPolicy` value in StatefulSet configuration of vmselect. See [#807](https://github.com/VictoriaMetrics/helm-charts/pull/807) by @MemberIT.

## 0.11.9

**Release date:** 2023-12-13

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix configuration of volume mount for license key referenced by using secret.

## 0.11.8

**Release date:** 2023-12-12

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.96.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.96.0)

## 0.11.7

**Release date:** 2023-12-08

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix `vminsert` configuration for volumes when using `extraVolumes`.

## 0.11.6

**Release date:** 2023-11-16

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.1)

## 0.11.5

**Release date:** 2023-11-15

![AppVersion: v1.95.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.0)

## 0.11.4

**Release date:** 2023-10-25

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix deployment `volumeMounts` when providing enterprise license key for VictoriaMetrics enterprise. See [this pr](https://github.com/VictoriaMetrics/helm-charts/pull/734) for details.

## 0.11.3

**Release date:** 2023-10-12

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add license enforcement for vmbackupmanager in order to avoid running it without enterprise license key. See [these docs](https://docs.victoriametrics.com/enterprise) for details.

## 0.11.2

**Release date:** 2023-10-04

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.94.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.94.0)
- Add support of providing enterprise license key for VictoriaMetrics enterprise. See [these docs](https://docs.victoriametrics.com/enterprise) for details.

## 0.11.1

**Release date:** 2023-10-04

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- From pod labels removed dynamic label `helm.sh/chart` to avoid restarting every time the chart is updated without changing the pods parameters. **Note that this time it will cause the pods to restart** (#695)

## 0.11.0

**Release date:** 2023-09-28

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add `extraObjects` which to allow deploying additional resources with the chart release (#689)

## 0.10.9

**Release date:** 2023-09-21

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Support `behavior` setting in horizontal pod autoscalers for vminsert and vmselect (#679)
- Bump version of VM components to [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5)

## 0.10.8

**Release date:** 2023-09-11

![AppVersion: v1.93.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.4)

## 0.10.7

**Release date:** 2023-09-04

![AppVersion: v1.93.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of Victoria Metrics Cluster to `v1.93.3`

## 0.10.5

**Release date:** 2023-08-23

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.93.0 to v1.93.1

## 0.10.4

**Release date:** 2023-08-12

![AppVersion: v1.93.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.1 to v1.93.0
- charts/victoria-metrics-cluster: remove incorrect comment (#607)
- vmstorage, vminsert: Add topoloogySpreadConstraints (#596)

## 0.10.3

**Release date:** 2023-07-28

![AppVersion: v1.92.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.0 to v1.92.1 (#599)

## 0.10.2

**Release date:** 2023-07-27

![AppVersion: v1.92.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.91.3 to v1.92.0
- fix misused securityContext and podSecurityContext (#592)

## 0.10.1

**Release date:** 2023-07-25

![AppVersion: v1.91.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.91.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix typo in suppressStorageFQDNsRender, address #580 (#581)

## 0.10.0

**Release date:** 2023-07-13

![AppVersion: v1.91.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.91.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of agent, alert, auth, cluster, single
- charts/victoria-metrics-cluster: fix indent for vmselect statefulset (#576)
