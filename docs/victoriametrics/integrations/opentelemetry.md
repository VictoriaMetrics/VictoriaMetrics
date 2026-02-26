---
title: OpenTelemetry
weight: 13
menu:
  docs:
    parent: "integrations-vm"
    identifier: "integrations-opentelemetry-vm"
    weight: 13
---

VictoriaMetrics supports data ingestion via [OpenTelemetry protocol (OTLP) for metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/97c826b70e2f89cfdf655d5150791f3f0c2bae19/specification/metrics/data-model.md) at `/opentelemetry/v1/metrics` path.
It expects `protobuf`-encoded requests at `/opentelemetry/v1/metrics`. For gzip-compressed workload set HTTP request header `Content-Encoding: gzip`.

See how to configure [OpenTelemetry Collector](https://docs.victoriametrics.com/victoriametrics/data-ingestion/opentelemetry-collector/) to push metrics to VictoriaMetrics.

## Metric naming

By default, VictoriaMetrics stores the ingested OpenTelemetry [metric samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) as is **without any transformations**.
The following label transformations can be enabled:
* `-usePromCompatibleNaming` - replaces characters unsupported by Prometheus with `_` in metric names and labels **for all ingestion protocols**.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time{service_name="foo"}`.
* `-opentelemetry.usePrometheusNaming` - converts metric names and labels according to [OTLP Metric points to Prometheus specification](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.33.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus) for metrics ingested via OTLP.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time_seconds_total{service_name="foo"}`.
* `-opentelemetry.convertMetricNamesToPrometheus` - converts **only metric names** according to [OTLP Metric points to Prometheus specification](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.33.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus) for metrics ingested via OTLP.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time_seconds_total{service.name="foo"}`. See more about this use case [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9830).

> These flags can be applied on vmagent, vminsert or VictoriaMetrics single-node.

## Exponential histograms

OpenTelemetry [exponential histogram](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram) is automatically converted
to [VictoriaMetrics histogram format](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).

## References

- See [How to use OpenTelemetry metrics with VictoriaMetrics](https://docs.victoriametrics.com/guides/getting-started-with-opentelemetry/).
- See more about [OpenTelemetry in VictoriaMetrics](https://docs.victoriametrics.com/opentelemetry/).
