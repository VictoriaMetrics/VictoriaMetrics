---
title: OpenTelemetry Collector
weight: 7
menu:
  docs:
    identifier: "opentelemetry-collector"
    parent: "data-ingestion"
    weight: 7
tags:
  - metrics
---

[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) is a vendor-agnostic agent for receiving, processing, and exporting telemetry data.
VictoriaMetrics supports the [OTLP metrics protocol](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/) natively,
so the collector can push metrics directly using the `otlphttp` exporter.

Use the following exporter configuration:

```yaml
exporters:
  otlphttp/victoriametrics:
    compression: gzip
    encoding: proto
    metrics_endpoint: http://<vmsinle>:8428/opentelemetry/v1/metrics
```

> For the [cluster version](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format) specify the tenant ID:
> `http://<vminsert>:8480/insert/<accountID>/opentelemetry/v1/metrics`.
> See more about [multitenancy](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

Add the exporter to the desired service pipeline to activate it:

```yaml
service:
  pipelines:
    metrics:
      exporters:
        - otlphttp/victoriametrics
      receivers:
        - otlp
```

See [OpenTelemetry integration](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/) for details on metric naming and histogram conversion.
