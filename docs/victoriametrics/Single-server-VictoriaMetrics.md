---
weight: 1
menu:
  docs:
    identifier: vm-single-version
    parent: victoriametrics
    weight: 1
title: Single-node version
tags:
  - metrics
aliases:
  - /Single-server-VictoriaMetrics/
  - /Single-server-VictoriaMetrics.html
  - /single-server-victoriametrics/index.html
  - /single-server-victoriametrics/
---
{{% content "README.md" %}}

> **Prometheus Metric Metadata Support**
>
> When `vmagent` is run with `-promscrape.emitMetricMetadata`, VictoriaMetrics will ingest and serve Prometheus metric metadata, making the `/api/v1/metadata` endpoint fully compatible with Prometheus. See [vmagent documentation](./vmagent.md#emitting-prometheus-metric-metadata) for details.
