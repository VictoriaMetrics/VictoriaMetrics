---
weight: 4
title: OpenTelemetry setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 4
aliases:
  - /VictoriaLogs/data-ingestion/OpenTelemetry.html
---
VictoriaLogs supports both client open-telemetry [SDK](https://opentelemetry.io/docs/languages/) and [collector](https://opentelemetry.io/docs/collector/).

## Client SDK

Specify `EndpointURL` for http-exporter builder to `/insert/opentelemetry/v1/logs`.

Consider the following example for Go SDK:

```go
logExporter, err := otlploghttp.New(ctx,
	otlploghttp.WithEndpointURL("http://victorialogs:9428/insert/opentelemetry/v1/logs"),
)
```

VictoriaLogs treats all the resource labels as [log stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
The list of log stream fields can be overriden via `VL-Stream-Fields` HTTP header if needed. For example, the following config uses only `host` and `app`
labels as log stream fields, while the remaining labels are stored as [regular log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```go
logExporter, err := otlploghttp.New(ctx,
	otlploghttp.WithEndpointURL("http://victorialogs:9428/insert/opentelemetry/v1/logs"),
	otlploghttp.WithHeaders(map[string]string{
		"VL-Stream-Fields": "host,app",
	}),
)
```

VictoriaLogs supports other HTTP headers - see the list [here](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers).

The ingested log entries can be queried according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

## Collector configuration

VictoriaLogs supports receiving logs from the following OpenTelemetry collectors:

* [Elasticsearch](#elasticsearch)
* [Loki](#loki)
* [OpenTelemetry](#opentelemetry)

### Elasticsearch

```yaml
exporters:
  elasticsearch:
    endpoints:
      - http://victorialogs:9428/insert/elasticsearch
receivers:
  filelog:
    include: [/tmp/logs/*.log]
    resource:
      region: us-east-1
service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [elasticsearch]
```

If Elasticsearch stores the log message in the field other than [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
then it can be moved to `_msg` field by using the `VL-Msg-Field` HTTP header. For example, if the log message is stored in the `Body` field,
then it can be moved to `_msg` field via the following config:

```yaml
exporters:
  elasticsearch:
    endpoints:
      - http://victorialogs:9428/insert/elasticsearch
    headers:
      VL-Msg-Field: Body
```

VictoriaLogs supports other HTTP headers - see the list [here](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers).

### Loki

```yaml
exporters:
  loki:
    endpoint: http://victorialogs:9428/insert/loki/api/v1/push
receivers:
  filelog:
    include: [/tmp/logs/*.log]
    resource:
      region: us-east-1
service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [loki]
```

### OpenTelemetry

Specify logs endpoint for [OTLP/HTTP exporter](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlphttpexporter/README.md) in configuration file
for sending the collected logs to VictoriaLogs:

```yaml
exporters:
  otlphttp:
    logs_endpoint: http://localhost:9428/insert/opentelemetry/v1/logs
```

VictoriaLogs supports various HTTP headers, which can be used during data ingestion - see the list [here](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers).
These headers can be pssed to OpenTelemetry exporter config via `headers` options. For example, the following config instructs ignoring `foo` and `bar` fields during data ingestion:

```yaml
exporters:
  otlphttp:
    logs_endpoint: http://localhost:9428/insert/opentelemetry/v1/logs
    headers:
      VL-Ignore-Fields: foo,bar
```

See also:

* [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
* [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
* [Docker-compose demo for OpenTelemetry collector integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/opentelemetry-collector).
