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

## Label sanitization

By default, VictoriaMetrics stores the ingested OpenTelemetry [metric points](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#metric-points) as is **without any transformations**.
The following label sanitization options can be enabled:
* `-usePromCompatibleNaming` - replaces characters unsupported by Prometheus with `_` in metric names and labels **for all ingestion protocols**.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time{service_name="foo"}`.
* `-opentelemetry.usePrometheusNaming` - converts metric names and labels according to [OTLP Metric points to Prometheus specification](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.33.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus) for metrics ingested via OTLP.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time_seconds_total{service_name="foo"}`.
* `-opentelemetry.convertMetricNamesToPrometheus` - converts **only metric names** according to [OTLP Metric points to Prometheus specification](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.33.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus) for metrics ingested via OTLP.
  For example, `process.cpu.time{service.name="foo"}` is converted to `process_cpu_time_seconds_total{service.name="foo"}`. See more about this use case [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9830).

> These flags can be applied on vmagent, vminsert or VictoriaMetrics single-node.

## Resource Attributes

By default, VictoriaMetrics promotes all [OpenTelemetry resource](https://opentelemetry.io/docs/specs/otel/resource/data-model/) attributes to labels and attaches them to all ingested OTLP metrics.

## Exponential histograms

OpenTelemetry [exponential histogram](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram) is automatically converted
to [VictoriaMetrics histogram format](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) during ingestion. Since VictoriaMetrics histogram doesn't support negative observations, all buckets in the negative range are dropped.
The number of dropped data points can be monitored via `vm_protoparser_rows_dropped_total{type="opentelemetry",reason="negative_exponential_histogram_buckets"}` metric.

## Delta Temporality

In OpenTelemetry, some metric types(including sums, histograms, and exponential histograms) support delta and cumulative aggregation temporality. VictoriaMetrics works best with cumulative temporality, and it's recommended to export metrics with cumulative temporality or convert delta to cumulative temporality using [OpenTelemetry Collector deltatocumulative processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/deltatocumulativeprocessor) before sending to VictoriaMetrics.
VictoriaMetrics stores delta temporality metric values as is {{% available_from "v1.132.0" %}}, they can be queried with [sum_over_time()](https://docs.victoriametrics.com/victoriametrics/metricsql/#sum_over_time) and [rate_over_sum()](https://docs.victoriametrics.com/victoriametrics/metricsql/#rate_over_sum).

> Do not apply [deduplication](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication) or [downsampling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#downsampling) to delta temporality metrics, since it might cause data loss.

## References

- See [How to use OpenTelemetry metrics with VictoriaMetrics](https://docs.victoriametrics.com/guides/getting-started-with-opentelemetry/).
- See more about [OpenTelemetry in VictoriaMetrics](https://docs.victoriametrics.com/opentelemetry/).
