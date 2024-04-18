---
sort: 11
weight: 11
title: VMScrapeConfig
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 11
aliases:
  - /operator/resources/vmscrapeconfig.html
---

# VMScrapeConfig

The `VMScrapeConfig` CRD allows to define a scrape config using [any of the service discovery options supported in victoriametrics](https://docs.victoriametrics.com/sd_configs/).

`VMScrapeConfig` object generates part of [VMAgent](./vmagent.md) configuration with Prometheus-compatible scrape targets.

## Specification

You can see the full actual specification of the `VMScrapeConfig` resource in
the **[API docs -> VMScrapeConfig](../api.md#vmscrapeconfig)**.

Also, you can check out the [examples](#examples) section.

## Migration from Prometheus

The `VMScrapeConfig` CRD from VictoriaMetrics Operator is a drop-in replacement 
for the Prometheus `ScrapeConfig` from prometheus-operator.

More details about migration from prometheus-operator you can read in [this doc](../migration.md).

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMScrapeConfig
metadata:
  name: mongodb
spec:
  consulSDConfigs:
  - server: https://consul-dns:8500
    services:
    - mongodb
  relabelConfigs:
  - action: replace
    sourceLabels:
    - __meta_consul_service
    targetLabel: job
```
