---
title: OpenTelemetry
weight: 8
menu:
  docs:
    parent: "integrations-vm"
    identifier: "integrations-opentelemetry-vm"
    weight: 8
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive data from
via [OpenTelemetry protocol for metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/ffddc289462dfe0c2041e3ca42a7b1df805706de/specification/metrics/data-model.md)
at `/opentelemetry/v1/metrics` path.

See guide [How to use OpenTelemetry metrics with VictoriaMetrics](https://docs.victoriametrics.com/guides/getting-started-with-opentelemetry/).

_For logs ingestion see [OpenTelemetry integration with VictoriaLogs](https://docs.victoriametrics.com/victorialogs/data-ingestion/opentelemetry/)._

VictoriaMetrics expects `protobuf`-encoded requests at `/opentelemetry/v1/metrics`.
Set HTTP request header `Content-Encoding: gzip` when sending gzip-compressed data to `/opentelemetry/v1/metrics`.

> VictoriaMetrics supports only [cumulative temporality](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#temporality)
for received measurements. The number of dropped unsupported samples is exposed via `vm_protoparser_rows_dropped_total{type="opentelemetry"}` metric.

VictoriaMetrics stores the ingested OpenTelemetry [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) as is, without any transformations.
Pass `-opentelemetry.usePrometheusNaming` command-line flag to VictoriaMetrics for automatic conversion of metric names and labels into Prometheus-compatible format.
OpenTelemetry [exponential histogram](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram) is automatically converted
to [VictoriaMetrics histogram format](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).

UsE the following exporter configuration in the OpenTelemetry collector to send metrics into VictoriaMetrics:
```yaml
exporters:
  otlphttp/victoriametrics:
    compression: gzip
    encoding: proto
    endpoint: http://<collector/victoriametrics>.<namespace>.svc.cluster.local:<port>/opentelemetry
```

For cluster version use vminsert address:
```yaml
endpoint: http://<collector/vminsert>.<namespace>.svc.cluster.local:<port>/insert/<tenant>/opentelemetry
```

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).
 
Remember to add the exporter to the desired service pipeline in order to activate the exporter.
```yaml
service:
  pipelines:
    metrics:
      exporters:
        - otlphttp/victoriametrics
      receivers:
        - otlp
```

OpenTelemetry collector can convert metrics into Prometheus format before sending if you use 
[`prometheusremotewrite` exporter](https://opentelemetry.io/docs/collector/configuration/#exporters):
```yaml
service:
  pipelines:
    metrics:
      exporters:
        - prometheusremotewrite/victoriametrics
```