## Next release

- `vmauthIngestGlobal` was changed to `write.global.vmauth`
- `vmauthQueryGlobal` was changed to `read.global.vmauth`
- `availabilityZones[*].allowIngest` was changed to `availabilityZones[*].write.allow`
- `availabilityZones[*].allowRead` was changed to `availabilityZones[*].read.allow`
- `availabilityZones[*].nodeSelector` was moved to `availabilityZones[*].common.spec.nodeSelector`
- `availabilityZones[*].extraAffinity` was moved to `availabilityZones[*].common.spec.affinity`
- `availabilityZones[*].topologySpreadConstraints` was moved to `availabilityZones[*].common.spec.topologySpreadConstraints`
- `availabilityZones[*].vmauthIngest` was moved to `availabilityZones[*].write.vmauth`
- `availabilityZones[*].vmauthQueryPerZone` was moved to `availabilityZones[*].read.perZone.vmauth`
- `availabilityZones[*].vmauthCrossAZQuery` was moved to `availabilityZones[*].read.crossZone.vmauth`
- set default DNS domain to `cluster.local.`

## 0.4.2

**Release date:** 2024-11-05

![AppVersion: v1.106.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.106.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.106.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.106.0)

## 0.4.1

**Release date:** 2024-10-21

![AppVersion: v1.105.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.105.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Human-readable error about Helm version requirement
- bump version of VM components to [v1.105.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.105.0)

## 0.4.0

**Release date:** 2024-10-02

![AppVersion: v1.104.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.104.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.104.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.104.0)

## 0.3.1

**Release date:** 2024-09-19

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not truncate datasource name
- Support customizing vmauthQueryGlobal spec. Thanks to @olivierbouffet for [the pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1511).
- Support overriding the default name for extra vmagent and vmcluster per zone.

## 0.3.0

**Release date:** 2024-08-29

![AppVersion: v1.103.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.103.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.103.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.103.0)

## 0.2.2

**Release date:** 2024-08-01

![AppVersion: v1.102.1](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.1&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.1)

## 0.2.1

**Release date:** 2024-07-23

![AppVersion: v1.102.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.102.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- bump version of VM components to [v1.102.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.102.0)

## 0.2.0

**Release date:** 2024-07-15

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Breaking change: disable multitenancy mode by default, see how to enable it in <https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-distributed#how-to-use-multitenancy>. See [this pull request](https://github.com/VictoriaMetrics/helm-charts/pull/1137) for details.

## 0.1.1

**Release date:** 2024-06-27

![AppVersion: v1.101.0](https://img.shields.io/static/v1?label=AppVersion&message=v1.101.0&color=success&logo=)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- [vmauth-read-balancer-zone]: change server from vmselect pod enumeration to service DNS address, so it still work when vmselect scales.
