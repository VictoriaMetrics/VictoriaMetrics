---
weight: 20
title: Vector setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 20
aliases:
  - /VictoriaLogs/data-ingestion/Vector.html
  - /victorialogs/data-ingestion/Vector.html
  - /victorialogs/data-ingestion/vector.html
---
VictoriaLogs supports given below Vector sinks:
- [Elasticsearch](#elasticsearch)
- [Loki](#loki)
- [HTTP JSON](#http)

## Elasticsearch

Specify [Elasticsearch sink type](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/) in the `vector.yaml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    query:
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
```

## Loki

Specify [Loki sink type](https://vector.dev/docs/reference/configuration/sinks/loki/) in the `vector.yaml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```yaml
sinks:
  vlogs:
    type: "loki"
    endpoint: "http://localhost:9428/insert/loki/"
    inputs:
      - your_input
    compression: gzip
    path: /api/v1/push?_msg_field=message.message&_time_field=timestamp&_stream_fields=source
    encoding:
      codec: json
    labels:
      source: vector
```

Substitute the `localhost:9428` address inside `endpoints` section with the real TCP address of VictoriaLogs.

Replace `your_input` with the name of the `inputs` section, which collects logs. See [these docs](https://vector.dev/docs/reference/configuration/sources/) for details.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on parameters specified
in the `sinks.vlogs.query` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters)
in the `sinks.vlogs.query` section and inspecting VictoriaLogs logs then:

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    query:
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
      debug: "1"
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    query:
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
      _ignore_fields: log.offset,event.original
```

When Vector ingests logs into VictoriaLogs at a high rate, then it may be needed to tune `batch.max_events` option.
For example, the following config is optimized for higher than usual ingestion rate:

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    query: 
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
    batch:
      max_events: 1000
```

If the Vector sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compression = "gzip"` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    compression: gzip
    query:
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/keyconcepts/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `sinks.vlogs.request.headers` section.
For example, the following `vector.yaml` config instructs Vector to store the data to `(AccountID=12, ProjectID=34)` tenant:

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: elasticsearch
    endpoints:
      - http://localhost:9428/insert/elasticsearch/
    mode: bulk
    api_version: v8
    healthcheck:
      enabled: false
    query:
      _msg_field: message
      _time_field: timestamp
      _stream_fields: host,container_name
    request:
      headers:
        AccountID: "12"
        ProjectID: "34"
```

## HTTP

Vector can be configured with [HTTP](https://vector.dev/docs/reference/configuration/sinks/http/) sink type 
for sending data to [JSON stream API](https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api):

```yaml
sinks:
  vlogs:
    inputs:
      - your_input
    type: http
    uri: http://localhost:9428/insert/jsonline?_stream_fields=host,container_name&_msg_field=message&_time_field=timestamp
    encoding:
      codec: json
    framing:
      method: newline_delimited
    healthcheck:
      enabled: false
    request:
      headers:
        AccountID: "12"
        ProjectID: "34"
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Elasticsearch output docs for Vector](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/).
- [Docker-compose demo for Vector integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/vector).
