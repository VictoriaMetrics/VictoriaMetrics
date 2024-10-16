## Next release

- TODO

## 0.35.4

**Release date:** 2024-10-11

![AppVersion: v0.48.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.48.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Human-readable error about Helm version requirement

## 0.35.3

**Release date:** 2024-10-10

![AppVersion: v0.48.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.48.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- upgraded common chart dependency
- made webhook pod port configurable. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1565)
- added configurable cleanup hook resources. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1571)
- added ability to configure `terminationGracePeriodSeconds` and `lifecycle`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1563) for details

## 0.35.2

**Release date:** 2024-09-29

![AppVersion: v0.48.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.48.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.48.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.48.3) version

## 0.35.1

**Release date:** 2024-09-26

![AppVersion: v0.48.1](https://img.shields.io/static/v1?label=AppVersion&message=v0.48.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.48.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.48.1) version

## 0.35.0

**Release date:** 2024-09-26

![AppVersion: v0.48.0](https://img.shields.io/static/v1?label=AppVersion&message=v0.48.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Made webhook port configurable. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1506)
- Changed crd cleanup hook delete policy to prevent `resource already exists` error.
- updates operator to [v0.48.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.48.0) version

## 0.34.8

**Release date:** 2024-09-10

![AppVersion: v0.47.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to override deployment namespace using `namespaceOverride` and `global.namespaceOverride` variables
- Fixed template for cert-manager certificates
- Fixed operator Role creation when only watching own namespace using `watchNamespaces`
- Changed webhook service port from 443 to 9443

## 0.34.7

**Release date:** 2024-09-03

![AppVersion: v0.47.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not create ClusterRole if `watchNamespaces` contains only namespace, where operator is deployed

## 0.34.6

**Release date:** 2024-08-29

![AppVersion: v0.47.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.47.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.47.3) version
- Made `cleanupCRD` deprecated in a favour of `crd.cleanup.enabled`
- Made `cleanupImage` deprecated in a favour of `crd.cleanup.image`
- Made `watchNamespace` string deprecated in a favour of `watchNamespaces` slice
- Decreased rendering time by 2 seconds

## 0.34.5

**Release date:** 2024-08-26

![AppVersion: v0.47.2](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixes typo at clean webhook. vmlogs->vlogs.

## 0.34.4

**Release date:** 2024-08-26

![AppVersion: v0.47.2](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixes RBAC by rollback <https://github.com/VictoriaMetrics/helm-charts/commit/7d75b93525bb0a99a8011b700d0a51b6b762321c>

## 0.34.3

**Release date:** 2024-08-26

![AppVersion: v0.47.2](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- removes not implemented scrape CRDs from validation webhook

## 0.34.2

**Release date:** 2024-08-26

![AppVersion: v0.47.2](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- set `admissionWebhooks.keepTLSSecret` to `true` by default
- fixed indent, for Issuer crd, when `cert-manager.enabled: true`
- updates operator to [v0.47.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.47.2) version

## 0.34.1

**Release date:** 2024-08-23

![AppVersion: v0.47.1](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: main container name was changed to `operator`, which will recreate a pod.

- Updated operator to v0.47.1 release
- Added global imagePullSecrets and image.registry
- Use static container names in a pod
- Updated operator service scrape config
- Added `.Values.vmstorage.service.ipFamilies` and `.Values.vmstorage.service.ipFamilyPolicy` for service IP family management
- Enabled webhook by default
- Generate webhook certificate when Cert Manager is not enabled
- Added ability to configure container port
- Fixed image pull secrets. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1285)

## 0.34.0

**Release date:** 2024-08-15

![AppVersion: v0.47.0](https://img.shields.io/static/v1?label=AppVersion&message=v0.47.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Set minimal kubernetes version to 1.25
- Removed support for policy/v1beta1/PodDisruptionBudget
- Added configurable probes at `.Values.probe`
- updates operator to [v0.47.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.47.0) release
- adds RBAC permissions to VLogs object

## 0.33.6

**Release date:** 2024-08-07

![AppVersion: v0.46.4](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- add missing permission to allow patching `horizontalpodautoscalers` when operator watches single namespace.

## 0.33.5

**Release date:** 2024-08-01

![AppVersion: v0.46.4](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix cleanup job image tag when `.Capabilities.KubeVersion.Minor` returns version with plus sign. See [this pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1169) by @dimaslv.

## 0.33.4

**Release date:** 2024-07-10

![AppVersion: v0.46.4](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.46.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.46.4) release

## 0.33.3

**Release date:** 2024-07-05

![AppVersion: v0.46.3](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.46.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.46.3) release

## 0.33.2

**Release date:** 2024-07-04

![AppVersion: v0.46.2](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- breaking change: operator uses different entrypoint, remove `command` entrypoint
- breaking change: operator uses new flag for leader election `leader-elect`
- removes podsecurity policy. It's longer supported by kubernetes
- updates operator to [v0.46.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.46.2) release

## 0.33.1

**Release date:** 2024-07-03

![AppVersion: v0.46.0](https://img.shields.io/static/v1?label=AppVersion&message=v0.46.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- breaking change: operator uses different entrypoint, remove `command` entrypoint
- breaking change: operator uses new flag for leader election `leader-elect`
- removes podsecurity policy. It's longer supported by kubernetes
- updates operator to [v0.46.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.46.0) release

## 0.32.3

**Release date:** 2024-07-02

![AppVersion: v0.45.0](https://img.shields.io/static/v1?label=AppVersion&message=v0.45.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- use bitnami/kubectl image for cleanup instead of deprecated gcr.io/google_containers/hyperkube

## 0.32.2

**Release date:** 2024-06-14

![AppVersion: v0.45.0](https://img.shields.io/static/v1?label=AppVersion&message=v0.45.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix default image tag when using `Chart.AppVersion`, previously the version is missing "v".

## 0.32.1

**Release date:** 2024-06-14

![AppVersion: 0.45.0](https://img.shields.io/static/v1?label=AppVersion&message=0.45.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: The VictoriaMetrics components image tag template has been updated. This change introduces `.Values.<component>.image.variant` to specify tag suffixes like `-scratch`, `-cluster`, `-enterprise`. Additionally, you can now omit `.Values.<component>.image.tag` to automatically use the version specified in `.Chart.AppVersion`.

- support specifying image tag suffix like "-enterprise" for VictoriaMetrics components using `.Values.<component>.image.variant`.

## 0.32.0

**Release date:** 2024-06-10

![AppVersion: 0.45.0](https://img.shields.io/static/v1?label=AppVersion&message=0.45.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.45.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.45.0)

## 0.31.2

**Release date:** 2024-05-14

![AppVersion: 0.44.0](https://img.shields.io/static/v1?label=AppVersion&message=0.44.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix missing serviceaccounts patch permission in ClusterRole, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1012) for details.

## 0.31.1

**Release date:** 2024-05-10

![AppVersion: 0.44.0](https://img.shields.io/static/v1?label=AppVersion&message=0.44.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix serviceAccount template when `.Values.serviceAccount.create=false`, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1002) by @tylerturk for details.
- support creating aggregated clusterRoles for VM CRDs with admin and read permissions, see this [pull request](https://github.com/VictoriaMetrics/helm-charts/pull/996) by @reegnz for details.

## 0.31.0

**Release date:** 2024-05-09

![AppVersion: 0.44.0](https://img.shields.io/static/v1?label=AppVersion&message=0.44.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.44.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.44.0)

## 0.30.3

**Release date:** 2024-04-26

![AppVersion: 0.43.5](https://img.shields.io/static/v1?label=AppVersion&message=0.43.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.43.5](https://github.com/VictoriaMetrics/operator/releases/tag/v0.43.5)

## 0.30.2

**Release date:** 2024-04-23

![AppVersion: 0.43.3](https://img.shields.io/static/v1?label=AppVersion&message=0.43.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to v0.43.1 version
- fixes typo at single-namespace role for `vmscrapeconfig`. See this [issue](https://github.com/VictoriaMetrics/helm-charts/issues/987) for details.

## 0.30.1

**Release date:** 2024-04-18

![AppVersion: 0.43.1](https://img.shields.io/static/v1?label=AppVersion&message=0.43.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- TODO

- updates operator to v0.43.1 version

## 0.30.0

**Release date:** 2024-04-18

![AppVersion: 0.43.0](https://img.shields.io/static/v1?label=AppVersion&message=0.43.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to v0.43.0-0 version
- adds `events` create permission
- properly truncate value of `app.kubernetes.io/managed-by` and `app.kubernetes.io/instance` labels in case release name exceeds 63 characters.

## 0.29.6

**Release date:** 2024-04-16

![AppVersion: 0.42.4](https://img.shields.io/static/v1?label=AppVersion&message=0.42.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- clean up vmauth as well when uninstall chart with `cleanupCRD: true`, since it also has `finalizers`.
- sync new crd VMScrapeConfig from operator, see detail in <https://docs.victoriametrics.com/operator/api/#vmscrapeconfig>.

## 0.29.5

**Release date:** 2024-04-02

![AppVersion: 0.42.4](https://img.shields.io/static/v1?label=AppVersion&message=0.42.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.42.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.42.4)

## 0.29.4

**Release date:** 2024-03-28

![AppVersion: 0.42.3](https://img.shields.io/static/v1?label=AppVersion&message=0.42.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to use slice variables in extraArgs (#944)

## 0.29.3

**Release date:** 2024-03-12

![AppVersion: 0.42.3](https://img.shields.io/static/v1?label=AppVersion&message=0.42.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- TODO

## 0.29.2

**Release date:** 2024-03-06

![AppVersion: 0.42.2](https://img.shields.io/static/v1?label=AppVersion&message=0.42.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.42.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.42.2)

## 0.29.0

**Release date:** 2024-03-06

![AppVersion: 0.42.1](https://img.shields.io/static/v1?label=AppVersion&message=0.42.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.42.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.42.1)

## 0.29.0

**Release date:** 2024-03-04

![AppVersion: 0.42.0](https://img.shields.io/static/v1?label=AppVersion&message=0.42.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.42.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.42.0)

## 0.28.1

**Release date:** 2024-02-21

![AppVersion: 0.41.2](https://img.shields.io/static/v1?label=AppVersion&message=0.41.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.41.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.41.2)

## 0.28.0

**Release date:** 2024-02-09

![AppVersion: 0.41.1](https://img.shields.io/static/v1?label=AppVersion&message=0.41.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update victoriametrics CRD resources yaml.

## 0.27.11

**Release date:** 2024-02-01

![AppVersion: 0.41.1](https://img.shields.io/static/v1?label=AppVersion&message=0.41.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.41.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.41.1)

## 0.27.10

**Release date:** 2024-01-24

![AppVersion: 0.40.0](https://img.shields.io/static/v1?label=AppVersion&message=0.40.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump operator version to [0.40.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.40.0)

## 0.27.9

**Release date:** 2023-12-12

![AppVersion: 0.39.4](https://img.shields.io/static/v1?label=AppVersion&message=0.39.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.39.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.4)

## 0.27.8

**Release date:** 2023-12-08

![AppVersion: 0.39.3](https://img.shields.io/static/v1?label=AppVersion&message=0.39.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Sync CRD resources with operator [v0.39.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.3).

## 0.27.7

**Release date:** 2023-12-08

![AppVersion: 0.39.3](https://img.shields.io/static/v1?label=AppVersion&message=0.39.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Skip deleting victoriametrics CRD resources when uninstall release.

## 0.27.6

**Release date:** 2023-11-16

![AppVersion: 0.39.3](https://img.shields.io/static/v1?label=AppVersion&message=0.39.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.39.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.3)

## 0.27.5

**Release date:** 2023-11-15

![AppVersion: 0.39.2](https://img.shields.io/static/v1?label=AppVersion&message=0.39.2&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.39.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.2)
- Add `extraObjects` to allow deploying additional resources with the chart release. (#751)

## 0.27.4

**Release date:** 2023-11-01

![AppVersion: 0.39.1](https://img.shields.io/static/v1?label=AppVersion&message=0.39.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.39.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.1)

## 0.27.3

**Release date:** 2023-10-08

![AppVersion: 0.39.0](https://img.shields.io/static/v1?label=AppVersion&message=0.39.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added endpointslices permissions to operator roles (#708)

## 0.27.2

**Release date:** 2023-10-04

![AppVersion: 0.39.0](https://img.shields.io/static/v1?label=AppVersion&message=0.39.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.39.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.39.0)

## 0.27.1

**Release date:** 2023-09-28

![AppVersion: 0.38.0](https://img.shields.io/static/v1?label=AppVersion&message=0.38.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix `relabelConfigs` for operator's VMServiceScrape (#624)

## 0.27.0

**Release date:** 2023-09-11

![AppVersion: 0.38.0](https://img.shields.io/static/v1?label=AppVersion&message=0.38.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of operator to [v0.38.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.38.0)

## 0.26.2

**Release date:** 2023-09-07

![AppVersion: 0.37.1](https://img.shields.io/static/v1?label=AppVersion&message=0.37.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Updated CRDs for operator

## 0.26.1

**Release date:** 2023-09-04

![AppVersion: 0.37.1](https://img.shields.io/static/v1?label=AppVersion&message=0.37.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of Victoria Metrics operator to `v0.37.1`

## 0.26.0

**Release date:** 2023-08-30

![AppVersion: 0.37.0](https://img.shields.io/static/v1?label=AppVersion&message=0.37.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump operator version to [v0.37.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.37.0)
- `psp_auto_creation_enabled` for operator is disabled by default

## 0.25.0

**Release date:** 2023-08-24

![AppVersion: 0.36.0](https://img.shields.io/static/v1?label=AppVersion&message=0.36.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added `topologySpreadConstraints` for the operator + a small refactoring (#611)
- Fix vm operator appVersion (#589)
- Fixes operator doc description
- Add `cleanupCRD` option to clean up vm cr resources when uninstalling (#593)
- Bump operator version to [v0.36.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.36.0)

## 0.24.1

**Release date:** 2023-07-13

![AppVersion: 0.35.](https://img.shields.io/static/v1?label=AppVersion&message=0.35.&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- operator release v0.35.1

## 0.24.0

**Release date:** 2023-07-03

![AppVersion: 0.35.0](https://img.shields.io/static/v1?label=AppVersion&message=0.35.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator for v0.35.0
- updates for v1.91.1 release

## 0.23.1

**Release date:** 2023-05-29

![AppVersion: 0.34.1](https://img.shields.io/static/v1?label=AppVersion&message=0.34.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator for v0.34.1 version

## 0.23.0

**Release date:** 2023-05-25

![AppVersion: 0.34.0](https://img.shields.io/static/v1?label=AppVersion&message=0.34.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump operator version
- feat(operator): add PodDisruptionBudget (#546)
