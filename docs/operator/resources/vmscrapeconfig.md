---
weight: 11
title: VMScrapeConfig
menu:
  docs:
    identifier: operator-cr-vmscrapeconfig
    parent: operator-cr
    weight: 11
aliases:
  - /operator/resources/vmscrapeconfig/
  - /operator/resources/vmscrapeconfig/index.html
---
The `VMScrapeConfig` CRD allows to define a scrape config using [any of the service discovery options supported in victoriametrics](https://docs.victoriametrics.com/sd_configs).

`VMScrapeConfig` object generates part of [VMAgent](https://docs.victoriametrics.com/vmagent) configuration with Prometheus-compatible scrape targets.

## Specification

You can see the full actual specification of the `VMScrapeConfig` resource in
the **[API docs -> VMScrapeConfig](https://docs.victoriametrics.com/operator/api#vmscrapeconfig)**.

Also, you can check out the [examples](#examples) section.

## Migration from Prometheus

The `VMScrapeConfig` CRD from VictoriaMetrics Operator is a drop-in replacement 
for the Prometheus `ScrapeConfig` from prometheus-operator.

More details about migration from prometheus-operator you can read in [this doc](https://docs.victoriametrics.com/operator/migration).

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
