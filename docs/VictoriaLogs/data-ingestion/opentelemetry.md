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

 Specify `EndpointURL`  for http-exporter builder.

Consider the following example for `golang` `SDK`:

```go
 // Create the OTLP log exporter that sends logs to configured destination
 logExporter, err := otlploghttp.New(ctx,
  otlploghttp.WithEndpointURL("http://victorialogs:9428/insert/opentelemetry/v1/logs"),
 )
```

 Optionally, [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) could be defined via headers:

```go
 // Create the OTLP log exporter that sends logs to configured destination
 logExporter, err := otlploghttp.New(ctx,
  otlploghttp.WithEndpointURL("http://victorialogs:9428/insert/opentelemetry/v1/logs"),
   otlploghttp.WithHeaders(map[string]string{"VL-Stream-Fields": "telemetry.sdk.language,severity"}),
 )

```

 Given config defines 2 stream fields - `severity` and `telemetry.sdk.language`.

See also [HTTP headers](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers)

## Collector configuration

VictoriaLogs supports given below OpenTelemetry collector exporters:

* [Elasticsearch](#elasticsearch)
* [Loki](#loki)
* [OpenTelemetry](#opentelemetry)

### Elasticsearch

```yaml
exporters:
  elasticsearch:
    endpoints:
      - http://victorialogs:9428/insert/elasticsearch
    headers:
      VL-Msg-Field: "Body" # Optional.
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

Please note that every ingested log entry **must** contain at least a `_msg` field with the actual log message. By default, 
the Elasticsearch exporter may place the log message in the `Body` field. In this case, you can specify the field mapping via:
```yaml
    headers:
      VL-Msg-Field: "Body"
```

VictoriaLogs also support specify `AccountID`, `ProjectID`, log timestamp and other fields via [HTTP headers](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers).

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
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```yaml
exporters:
  otlphttp:
    logs_endpoint: http://localhost:9428/insert/opentelemetry/v1/logs
```

 Optionally, [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) could be defined via headers:

```yaml
exporters:
  otlphttp:
    logs_endpoint: http://localhost:9428/insert/opentelemetry/v1/logs
    headers:
      VL-Stream-Fields: telemetry.sdk.language,severity
```

See also [HTTP headers](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers)

Substitute `localhost:9428` address inside `exporters.otlphttp.logs_endpoint` with the real address of VictoriaLogs.

The ingested log entries can be queried according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

See also:

* [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
* [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
* [Docker-compose demo for OpenTelemetry collector integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/opentelemetry-collector).
