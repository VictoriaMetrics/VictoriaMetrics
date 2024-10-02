---
weight: 1
title: Filebeat setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 1
aliases:
  - /VictoriaLogs/data-ingestion/Filebeat.html
  - /victorialogs/data-ingestion/Filebeat.html
  - /victorialogs/data-ingestion/filebeat.html
---
Specify [`output.elasticsearch`](https://www.elastic.co/guide/en/beats/filebeat/current/elasticsearch-output.html) section in the `filebeat.yml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.hostname,log.file.path"
```

Substitute the `localhost:9428` address inside `hosts` section with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on the `parameters` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters)
and inspecting VictoriaLogs logs then:

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.hostname,log.file.path"
    debug: "1"
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
    ignore_fields: "log.offset,event.original"
```

When Filebeat ingests logs into VictoriaLogs at a high rate, then it may be needed to tune `worker` and `bulk_max_size` options.
For example, the following config is optimized for higher than usual ingestion rate:

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
  worker: 8
  bulk_max_size: 1000
```

If the Filebeat sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compression_level` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
  compression_level: 1
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `headers` at `output.elasticsearch` section.
For example, the following `filebeat.yml` config instructs Filebeat to store the data to `(AccountID=12, ProjectID=34)` tenant:

```yaml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  headers:
    AccountID: 12
    ProjectID: 34
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
```

Filebeat checks a version of ElasticSearch on startup and refuses to start sending logs if the version is not compatible.
In order to bypass this check please add `allow_older_versions: true` into `output.elasticsearch` section:

```yaml
output.elasticsearch:
  hosts: [ "http://localhost:9428/insert/elasticsearch/" ]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
  allow_older_versions: true
```

Alternatively, is also possible to change version which VictoriaLogs reports to Filebeat by using `-elasticsearch.version`
command-line flag.

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Filebeat `output.elasticsearch` docs](https://www.elastic.co/guide/en/beats/filebeat/current/elasticsearch-output.html).
- [Docker-compose demo for Filebeat integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/filebeat).
