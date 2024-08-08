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

# OpenTelemetry setup

VictoriaLogs supports given below OpenTelemetry collector exporters:
- [Elasticsearch](#elasticsearch)
- [Loki](#loki)
- [OpenTelemetry](#opentelemetry)

## Elasticsearch

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

## Loki

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

## OpenTelemetry

Specify logs endpoint for [OTLP/HTTP exporter](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlphttpexporter/README.md) in configuration file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```yaml
exporters:
  otlphttp:
    logs_endpoint: http://localhost:9428/insert/opentelemetry/api/v1/push
```

Substitute `localhost:9428` address inside `exporters.oltphttp.logs_endpoint` with the real TCP address of VictoriaLogs.

VictoriaLogs divides all the ingested logs into a log streams [log stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) relying on resource attributes. In example below resource attributes are set for [filelog OpenTelemetry receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver):

```yaml
receivers:
  filelog:
    include: [/var/log/myservice/*.json]
    resource:
      region: us-east-1
```

The ingested log entries can be queried according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Docker-compose demo for OpenTelemetry collector integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/opentelemetry-collector).
