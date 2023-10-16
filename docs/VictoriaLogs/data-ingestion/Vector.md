---
weight: 5
title: Vector setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 5
aliases:
  - /VictoriaLogs/data-ingestion/Vector.html
---
# Vector setup

Specify [Elasticsearch sink type](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/) in the `vector.toml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"
```

Substitute the `localhost:9428` address inside `endpoints` section with the real TCP address of VictoriaLogs.

Replace `your_input` with the name of the `inputs` section, which collects logs. See [these docs](https://vector.dev/docs/reference/configuration/sources/) for details.

See [these docs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters) for details on parameters specified
in the `[sinks.vlogs.query]` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters)
in the `[sinks.vlogs.query]` section and inspecting VictoriaLogs logs then:

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"
    debug = "1"
```

If some [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"
    ignore_fields = "log.offset,event.original"
```

When Vector ingests logs into VictoriaLogs at a high rate, then it may be needed to tune `batch.max_events` option.
For example, the following config is optimized for higher than usual ingestion rate:

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"

  [sinks.vlogs.batch]
    max_events = 1000
```

If the Vector sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compression = "gzip"` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false
  compression = "gzip"

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `[sinks.vlogq.request.headers]` section.
For example, the following `vector.toml` config instructs Vector to store the data to `(AccountID=12, ProjectID=34)` tenant:

```toml
[sinks.vlogs]
  inputs = [ "your_input" ]
  type = "elasticsearch"
  endpoints = [ "http://localhost:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "message"
    _time_field = "timestamp"
    _stream_fields = "host,container_name"

  [sinks.vlogs.request.headers]
    AccountID = "12"
    ProjectID = "34"
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/).
- [Elasticsearch output docs for Vector](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/).
- [Docker-compose demo for Filebeat integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/vector-docker).
