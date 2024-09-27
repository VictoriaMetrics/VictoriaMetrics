---
weight: 2
title: Fluentd setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 2
aliases:
  - /VictoriaLogs/data-ingestion/Fluentd.html
  - /victorialogs/data-ingestion/fluentd.html
  - /victorialogs/data-ingestion/Fluentd.html
---
VictoriaLogs supports given below Fluentd outputs:
- [Loki](#loki)
- [HTTP JSON](#http)

## Loki

Specify [loki output](https://docs.fluentd.io/manual/pipeline/outputs/loki) section in the `fluentd.conf`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```conf
<match **>
  @type loki
  url "http://localhost:9428/insert"
  <buffer>
    flush_interval 10s
    flush_at_shutdown true
  </buffer>
  custom_headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
  buffer_chunk_limit 1m
</match>
```

## HTTP

Specify [http output](https://docs.fluentd.io/manual/pipeline/outputs/http) section in the `fluentd.conf`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```fluentd
<match **>
  @type http
  endpoint "http://localhost:9428/insert/jsonline"
  headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
</match>
```

Substitute the host (`localhost`) and port (`9428`) with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on the query args specified in the `endpoint`.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) in the `endpoint`
and inspecting VictoriaLogs logs then:

```fluentd
<match **>
  @type http
  endpoint "http://localhost:9428/insert/jsonline&debug=1"
  headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
</match>
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```fluentd
<match **>
  @type http
  endpoint "http://localhost:9428/insert/jsonline&ignore_fields=log.offset,event.original"
  headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
</match>
```

If the Fluentd sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compress gzip` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```fluentd
<match **>
  @type http
  endpoint "http://localhost:9428/insert/jsonline&ignore_fields=log.offset,event.original"
  headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
  compress gzip
</match>
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/keyconcepts/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `header` options.
For example, the following `fluentd.conf` config instructs Fluentd to store the data to `(AccountID=12, ProjectID=34)` tenant:

```fluentd
<match **>
  @type http
  endpoint "http://localhost:9428/insert/jsonline"
  headers {"VL-Msg-Field": "log", "VL-Time-Field": "time", "VL-Stream-Fields": "path"}
  header AccountID 12
  header ProjectID 23
</match>
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Fluentd HTTP output config docs](https://docs.fluentd.org/output/http)
- [Docker-compose demo for Fluentd integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/fluentd).
