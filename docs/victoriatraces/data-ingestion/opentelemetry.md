---
weight: 4
title: OpenTelemetry setup
disableToc: true
menu:
  docs:
    parent: "victoriatraces-data-ingestion"
    weight: 4
tags:
  - traces
aliases:
  - /victoriatraces/data-ingestion/OpenTelemetry.html
---

> VictoriaTraces is currently under active development and not ready for production use. It is built on top of VictoriaLogs and therefore shares some flags and APIs. These will be fully separated once VictoriaTraces reaches a stable release. Until then, features may change or break without notice.

VictoriaTraces supports both client open-telemetry [SDK](https://opentelemetry.io/docs/languages/) and [collector](https://opentelemetry.io/docs/collector/).

## Client SDK

The OpenTelemetry provides detailed document and examples for various programming languages:
- [C++](https://opentelemetry.io/docs/languages/cpp/)
- [C#/.NET](https://opentelemetry.io/docs/languages/dotnet/)
- [Erlang/Elixir](https://opentelemetry.io/docs/languages/erlang/)
- [Go](https://opentelemetry.io/docs/languages/go/)
- [Java](https://opentelemetry.io/docs/languages/java/)
- [JavaScript](https://opentelemetry.io/docs/languages/js/)
- [PHP](https://opentelemetry.io/docs/languages/php/)
- [Python](https://opentelemetry.io/docs/languages/python/)
- [Ruby](https://opentelemetry.io/docs/languages/ruby/)
- [Rust](https://opentelemetry.io/docs/languages/rust/)
- [Swift](https://opentelemetry.io/docs/languages/swift/)

To send data to VictoriaTraces, specify the `EndpointURL` for http-exporter builder to `http://<victoria-traces>:9428/insert/opentelemetry/v1/traces`.

Consider the following example for Go SDK:

```go
traceExporter, err := otlptracehttp.New(ctx,
	otlptracehttp.WithEndpointURL("http://<victoria-traces>:9428/insert/opentelemetry/v1/traces"),
)
```

VictoriaTraces automatically use `service.name` in **resource attributes** and `name` in **span** as [stream fields](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#stream-fields).
While the remaining data (including [resource](https://opentelemetry.io/docs/specs/otel/overview/#resources), [instrumentation scope](https://opentelemetry.io/docs/specs/otel/common/instrumentation-scope/), and fields in [span](https://opentelemetry.io/docs/specs/otel/trace/api/#span), like `trace_id`, `span_id`, span `attributes` and more) are stored as [regular fields](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model):

VictoriaTraces supports other HTTP headers - see the list [here](https://docs.victoriametrics.com/victoriatraces/data-ingestion/#http-headers).

The ingested trace spans can be queried according to [these docs](https://docs.victoriametrics.com/victoriatraces/querying/).

## Collector configuration

VictoriaTraces supports receiving traces from the following OpenTelemetry collector:

* [OpenTelemetry](#opentelemetry)

### OpenTelemetry

To send the collected traces to VictoriaTraces, specify traces endpoint for [OTLP/HTTP exporter](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlphttpexporter/README.md) in configuration file:

```yaml
exporters:
  otlphttp:
    traces_endpoint: http://<victoria-traces>:9428/insert/opentelemetry/v1/traces
```

VictoriaTraces supports various HTTP headers, which can be used during data ingestion - see the list [here](https://docs.victoriametrics.com/victoriatraces/data-ingestion/#http-headers).
These headers can be passed to OpenTelemetry exporter config via `headers` options. For example, the following configs add (or overwrites) `foo: bar` field to each trace span during data ingestion:

```yaml
exporters:
  otlphttp:
    traces_endpoint: http://<victoria-traces>:9428/insert/opentelemetry/v1/traces
    headers:
      VL-Extra-Fields: foo=bar
```

See also:

* [Data ingestion troubleshooting](https://docs.victoriametrics.com/victoriatraces/data-ingestion/#troubleshooting).
* [How to query VictoriaTraces](https://docs.victoriametrics.com/victoriatraces/querying/).
* [Docker-compose demo for OpenTelemetry collector integration with VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victoriatraces/opentelemetry-collector).
