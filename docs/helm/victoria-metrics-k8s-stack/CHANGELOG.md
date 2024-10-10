## Next release

- TODO

## 0.27.2

**Release date:** 2024-10-10

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed dashboards variable queries

## 0.27.1

**Release date:** 2024-10-10

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Generate VM components tag version from chart app name by default
- Added k8s apiserver, kube-proxy, controller-manager and kubelet dashboards
- Moved `dashboards.<dashboard>` to `defaultDashboards.dashboards.<dashboard>.enabled`
- Moved `defaultDashboardsEnabled` to `defaultDashboards.enabled`
- Moved `grafanaOperatorDashboardsFormat` to `defaultDashboards.grafanaOperator`
- Added condition for `grafana-overview`, `alertmanager-overview` and `vmbackupmanager` dashboards. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1564)
- Removed `experimentalDashboardsEnabled` param
- Upgraded default Alertmanager tag 0.25.0 -> 0.27.0
- Upgraded operator chart 0.35.2 -> 0.35.3

## 0.27.0

**Release date:** 2024-10-02

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.104.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.104.0)

## 0.26.0

**Release date:** 2024-09-29

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.48.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.48.3)

## 0.25.17

**Release date:** 2024-09-20

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added VMAuth to k8s stack. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/829)
- Fixed ETCD dashboard
- Use path prefix from args as a default path prefix for ingress. Related [issue](https://github.com/VictoriaMetrics/helm-charts/issues/1260)
- Allow using vmalert without notifiers configuration. Note that it is required to use `.vmalert.spec.extraArgs["notifiers.blackhole"]: true` in order to start vmalert with a blackhole configuration.

## 0.25.16

**Release date:** 2024-09-10

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not truncate servicemonitor, datasources, rules, dashboard, alertmanager & vmalert templates names
- Use service label for node-exporter instead of podLabel. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1458)
- Added common chart to a k8s-stack. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1456)
- Fixed value of custom alertmanager configSecret. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1461)

## 0.25.15

**Release date:** 2024-09-05

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Drop empty endpoints param from scrape configuration
- Fixed proto when TLS is enabled. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1449)

## 0.25.14

**Release date:** 2024-09-04

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed alertmanager templates

## 0.25.13

**Release date:** 2024-09-04

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Use operator's own service monitor

## 0.25.12

**Release date:** 2024-09-03

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed dashboards rendering. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1414)
- Fixed service monitor label name.

## 0.25.11

**Release date:** 2024-09-03

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Merged ingress templates
- Removed custom VMServiceScrape for operator
- Added ability to override default Prometheus-compatible datatasources with all available parameters. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/860).
- Do not use `grafana.dashboards` and `grafana.dashboardProviders`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1312).
- Migrated Node Exporter dashboard into chart
- Deprecated `grafana.sidecar.jsonData`, `grafana.provisionDefaultDatasource` in a favour of `grafana.sidecar.datasources.default` slice of datasources.
- Fail if no notifiers are set, do not set `notifiers` to null if empty

## 0.25.10

**Release date:** 2024-08-31

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed ingress extraPaths and externalVM urls rendering

## 0.25.9

**Release date:** 2024-08-31

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed vmalert ingress name typo
- Added ability to override default Prometheus-compatible datatasources with all available parameters. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/860).
- Do not use `grafana.dashboards` and `grafana.dashboardProviders`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1312).

## 0.25.8

**Release date:** 2024-08-30

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed external notifiers rendering, when alertmanager is disabled. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1378)

## 0.25.7

**Release date:** 2024-08-30

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed extra rules template context

## 0.25.6

**Release date:** 2024-08-29

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: Update `kubeProxy.spec` to `kubeProxy.vmScrape.spec`

**Update note**: Update `kubeScheduler.spec` to `kubeScheduler.vmScrape.spec`

**Update note**: Update `kubeEtcd.spec` to `kubeEtcd.vmScrape.spec`

**Update note**: Update `coreDns.spec` to `coreDns.vmScrape.spec`

**Update note**: Update `kubeDns.spec` to `kubeDns.vmScrape.spec`

**Update note**: Update `kubeProxy.spec` to `kubeProxy.vmScrape.spec`

**Update note**: Update `kubeControllerManager.spec` to `kubeControllerManager.vmScrape.spec`

**Update note**: Update `kubeApiServer.spec` to `kubeApiServer.vmScrape.spec`

**Update note**: Update `kubelet.spec` to `kubelet.vmScrape.spec`

**Update note**: Update `kube-state-metrics.spec` to `kube-state-metrics.vmScrape.spec`

**Update note**: Update `prometheus-node-exporter.spec` to `prometheus-node-exporter.vmScrape.spec`

**Update note**: Update `grafana.spec` to `grafana.vmScrape.spec`

- bump version of VM components to [v1.103.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.103.0)
- Added `dashboards.<dashboardName>` bool flag to enable dashboard even if component it is for is not installed.
- Allow extra `vmalert.notifiers` without dropping default notifier if `alertmanager.enabled: true`
- Do not drop default notifier, when vmalert.additionalNotifierConfigs is set
- Replaced static url proto with a template, which selects proto depending on a present tls configuration
- Moved kubernetes components monitoring config from `spec` config to `vmScrape.spec`
- Merged servicemonitor templates

## 0.25.5

**Release date:** 2024-08-26

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- TODO

## 0.25.4

**Release date:** 2024-08-26

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to [v0.47.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.47.2)
- kube-state-metrics - 5.16.4 -> 5.25.1
- prometheus-node-exporter - 4.27.0 -> 4.29.0
- grafana - 8.3.8 -> 8.4.7
- added configurable `.Values.global.clusterLabel` to all alerting and recording rules `by` and `on` expressions

## 0.25.3

**Release date:** 2024-08-23

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updated operator to v0.47.1 release
- Build `app.kubernetes.io/instance` label consistently. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1282)

## 0.25.2

**Release date:** 2024-08-21

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fixed vmalert ingress name. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1271)
- fixed alertmanager ingress host template rendering. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1270)

## 0.25.1

**Release date:** 2024-08-21

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added `.Values.global.license` configuration
- Fixed extraLabels rendering. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1248)
- Fixed vmalert url to alertmanager by including its path prefix
- Removed `networking.k8s.io/v1beta1/Ingress` and `extensions/v1beta1/Ingress` support
- Fixed kubedns servicemonitor template. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1255)

## 0.25.0

**Release date:** 2024-08-16

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

**Update note**: it requires to update CRD dependency manually before upgrade

**Update note**: requires Helm 3.14+

- Moved dashboards templating logic out of sync script to Helm template
- Allow to disable default grafana datasource
- Synchronize Etcd dashboards and rules with mixin provided by Etcd
- Add alerting rules for VictoriaMetrics operator.
- Updated alerting rules for VictoriaMetrics components.
- Fixed exact rule annotations propagation to other rules.
- Set minimal kubernetes version to 1.25
- updates operator to v0.47.0 version

## 0.24.5

**Release date:** 2024-08-01

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.1)

## 0.24.4

**Release date:** 2024-08-01

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update dependencies: grafana -> 8.3.6.
- Added `.Values.defaultRules.alerting` and `.Values.defaultRules.recording` to setup common properties for all alerting an recording rules

## 0.24.3

**Release date:** 2024-07-23

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.0)

## 0.24.2

**Release date:** 2024-07-15

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix vmalertmanager configuration when using `.VMAlertmanagerSpec.ConfigRawYaml`. See [this pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1136).

## 0.24.1

**Release date:** 2024-07-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to v0.46.4

## 0.24.0

**Release date:** 2024-07-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to override alerting rules labels and annotations:
- globally - `.Values.defaultRules.rule.spec.labels` (before it was `.Values.defaultRules.additionalRuleLabels`) and `.Values.defaultRules.rule.spec.annotations`
- for all rules in a group  - `.Values.defaultRules.groups.<groupName>.rules.spec.labels` and `.Valeus.defaultRules.groups.<groupName>.rules.spec.annotations`
- for each rule individually - `.Values.defaultRules.rules.<ruleName>.spec.labels` and `.Values.defaultRules.rules.<ruleName>.spec.annotations`
- changed `.Values.defaultRules.rules.<groupName>` to `.Values.defaultRules.groups.<groupName>.create`
- changed `.Values.defaultRules.appNamespacesTarget` to `.Values.defaultRules.groups.<groupName>.targetNamespace`
- changed `.Values.defaultRules.params` to `.Values.defaultRules.group.spec.params` with ability to override it at `.Values.defaultRules.groups.<groupName>.spec.params`

## 0.23.6

**Release date:** 2024-07-08

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- added ability to override alerting rules labels and annotations:
- globally - `.Values.defaultRules.rule.spec.labels` (before it was `.Values.defaultRules.additionalRuleLabels`) and `.Values.defaultRules.rule.spec.annotations`
- for all rules in a group  - `.Values.defaultRules.groups.<groupName>.rules.spec.labels` and `.Valeus.defaultRules.groups.<groupName>.rules.spec.annotations`
- for each rule individually - `.Values.defaultRules.rules.<ruleName>.spec.labels` and `.Values.defaultRules.rules.<ruleName>.spec.annotations`
- changed `.Values.defaultRules.rules.<groupName>` to `.Values.defaultRules.groups.<groupName>.create`
- changed `.Values.defaultRules.appNamespacesTarget` to `.Values.defaultRules.groups.<groupName>.targetNamespace`
- changed `.Values.defaultRules.params` to `.Values.defaultRules.group.spec.params` with ability to override it at `.Values.defaultRules.groups.<groupName>.spec.params`

## 0.23.5

**Release date:** 2024-07-04

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Support configuring vmalert `-notifier.config` with `.Values.vmalert.additionalNotifierConfigs`.

## 0.23.4

**Release date:** 2024-07-02

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add `extraObjects` to allow deploying additional resources with the chart release.

## 0.23.3

**Release date:** 2024-06-26

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Enable [conversion of Prometheus CRDs](https://docs.victoriametrics.com/operator/migration/#objects-conversion) by default. See [this](https://github.com/VictoriaMetrics/helm-charts/pull/1069) pull request for details.
- use bitnami/kubectl image for cleanup instead of deprecated gcr.io/google_containers/hyperkube

## 0.23.2

**Release date:** 2024-06-14

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not add `cluster` external label at VMAgent by default. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/774) for the details.

## 0.23.1

**Release date:** 2024-06-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- updates operator to v0.45.0 release
- sync latest vm alerts and dashboards.

## 0.23.0

**Release date:** 2024-05-30

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- sync latest etcd v3.5.x rules from [upstream](https://github.com/etcd-io/etcd/blob/release-3.5/contrib/mixin/mixin.libsonnet).
- add Prometheus operator CRDs as an optional dependency. See [this PR](https://github.com/VictoriaMetrics/helm-charts/pull/1022) and [related issue](https://github.com/VictoriaMetrics/helm-charts/issues/341) for the details.

## 0.22.1

**Release date:** 2024-05-14

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- fix missing serviceaccounts patch permission in VM operator, see [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1012) for details.

## 0.22.0

**Release date:** 2024-05-10

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.44.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.44.0)

## 0.21.3

**Release date:** 2024-04-26

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.101.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.101.0)

## 0.21.2

**Release date:** 2024-04-23

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.43.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.43.3)

## 0.21.1

**Release date:** 2024-04-18

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

## 0.21.0

**Release date:** 2024-04-18

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- TODO

- bump version of VM operator to [0.43.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.43.0)
- updates CRDs definitions.

## 0.20.1

**Release date:** 2024-04-16

![AppVersion: v1.100.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.100.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- upgraded dashboards and alerting rules, added values file for local (Minikube) setup
- bump version of VM components to [v1.100.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.100.1)

## 0.20.0

**Release date:** 2024-04-02

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM operator to [0.42.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.42.3)

## 0.19.4

**Release date:** 2024-03-05

![AppVersion: v1.99.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.99.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.99.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.99.0)

## 0.19.3

**Release date:** 2024-03-05

![AppVersion: v1.98.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.98.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Commented default configuration for alertmanager. It simplifies configuration and makes it more explicit. See this [issue](https://github.com/VictoriaMetrics/helm-charts/issues/473) for details.
- Allow enabling/disabling default k8s rules when installing. See [#904](https://github.com/VictoriaMetrics/helm-charts/pull/904) by @passie.

## 0.19.2

**Release date:** 2024-02-26

![AppVersion: v1.98.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.98.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix templating of VMAgent `remoteWrite` in case both `VMSingle` and `VMCluster` are disabled. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/865) for details.

## 0.19.1

**Release date:** 2024-02-21

![AppVersion: v1.98.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.98.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update dependencies: victoria-metrics-operator -> 0.28.1, grafana -> 7.3.1.
- Update victoriametrics CRD resources yaml.

## 0.19.0

**Release date:** 2024-02-09

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not store original labels in `vmagent`'s memory by default. This reduces memory usage of `vmagent` but makes `vmagent`'s debugging UI less informative. See [this docs](https://docs.victoriametrics.com/vmagent/#relabel-debug) for details on relabeling debug.
- Update dependencies: kube-state-metrics -> 5.16.0, prometheus-node-exporter -> 4.27.0, grafana -> 7.3.0.
- Update victoriametrics CRD resources yaml.
- Update builtin dashboards and rules.

## 0.18.12

**Release date:** 2024-02-01

![AppVersion: v1.97.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.97.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.97.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.97.1)
- Fix helm lint when ingress resources enabled - split templates of resources per kind. See [#820](https://github.com/VictoriaMetrics/helm-charts/pull/820) by @MemberIT.

## 0.18.11

**Release date:** 2023-12-15

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix missing `.Values.defaultRules.rules.vmcluster` value. See [#801](https://github.com/VictoriaMetrics/helm-charts/pull/801) by @MemberIT.

## 0.18.10

**Release date:** 2023-12-12

![AppVersion: v1.96.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.96.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.96.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.96.0)
- Add optional allowCrossNamespaceImport to GrafanaDashboard(s) (#788)

## 0.18.9

**Release date:** 2023-12-08

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Properly use variable from values file for Grafana datasource type. (#769)
- Update dashboards from upstream sources. (#780)

## 0.18.8

**Release date:** 2023-11-16

![AppVersion: v1.95.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.1)

## 0.18.7

**Release date:** 2023-11-15

![AppVersion: v1.95.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.95.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.95.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.0)
- Support adding extra group parameters for default vmrules. (#752)

## 0.18.6

**Release date:** 2023-11-01

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix kube scheduler default scraping port from 10251 to 10259, Kubernetes changed it since 1.23.0. See [this pr](https://github.com/VictoriaMetrics/helm-charts/pull/736) for details.
- Bump version of operator chart to [0.27.4](https://github.com/VictoriaMetrics/helm-charts/releases/tag/victoria-metrics-operator-0.27.4)

## 0.18.5

**Release date:** 2023-10-08

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update operator chart to [v0.27.3](https://github.com/VictoriaMetrics/helm-charts/releases/tag/victoria-metrics-operator-0.27.3) for fixing [#708](https://github.com/VictoriaMetrics/helm-charts/issues/708)

## 0.18.4

**Release date:** 2023-10-04

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update dependencies: [victoria-metrics-operator -> 0.27.2](https://github.com/VictoriaMetrics/helm-charts/releases/tag/victoria-metrics-operator-0.27.2), prometheus-node-exporter -> 4.23.2, grafana -> 6.59.5.

## 0.18.3

**Release date:** 2023-10-04

![AppVersion: v1.94.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.94.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.94.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.94.0)

## 0.18.2

**Release date:** 2023-09-28

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fix behavior of `vmalert.remoteWriteVMAgent` - remoteWrite.url for VMAlert is correctly generated considering endpoint, name, port and http.pathPrefix of VMAgent

## 0.18.1

**Release date:** 2023-09-21

![AppVersion: v1.93.5](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.5&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5)

## 0.18.0

**Release date:** 2023-09-12

![AppVersion: v1.93.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of `grafana` helm-chart to `6.59.*`
- Bump version of `prometheus-node-exporter` helm-chart to `4.23.*`
- Bump version of `kube-state-metrics` helm-chart to `0.59.*`
- Update alerting rules
- Update grafana dashboards
- Add `make` commands `sync-rules` and `sync-dashboards`
- Add support of VictoriaMetrics datasource

## 0.17.8

**Release date:** 2023-09-11

![AppVersion: v1.93.4](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.4&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of VM components to [v1.93.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.4)
- Bump version of operator chart to [0.27.0](https://github.com/VictoriaMetrics/helm-charts/releases/tag/victoria-metrics-operator-0.27.0)

## 0.17.7

**Release date:** 2023-09-07

![AppVersion: v1.93.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Bump version of operator helm-chart to `0.26.2`

## 0.17.6

**Release date:** 2023-09-04

![AppVersion: v1.93.3](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.3&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Move `cleanupCRD` option to victoria-metrics-operator chart (#593)
- Disable `honorTimestamps` for cadvisor scrape job by default (#617)
- For vmalert all replicas of alertmanager are added to notifiers (only if alertmanager is enabled) (#619)
- Add `grafanaOperatorDashboardsFormat` option (#615)
- Fix query expression for memory calculation in `k8s-views-global` dashboard (#636)
- Bump version of Victoria Metrics components to `v1.93.3`
- Bump version of operator helm-chart to `0.26.0`

## 0.17.5

**Release date:** 2023-08-23

![AppVersion: v1.93.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.93.0 to v1.93.1

## 0.17.4

**Release date:** 2023-08-12

![AppVersion: v1.93.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.93.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.1 to v1.93.0
- delete an obsolete parameter remaining by mistake (see <https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-k8s-stack#upgrade-to-0130>) (#602)

## 0.17.3

**Release date:** 2023-07-28

![AppVersion: v1.92.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.92.0 to v1.92.1 (#599)

## 0.17.2

**Release date:** 2023-07-27

![AppVersion: v1.92.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.92.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Update VictoriaMetrics components from v1.91.3 to v1.92.0
