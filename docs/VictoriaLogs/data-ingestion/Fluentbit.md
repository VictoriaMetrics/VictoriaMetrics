---
weight: 2
title: Fluentbit setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 2
aliases:
  - /VictoriaLogs/data-ingestion/Fluentbit.html
  - /victorialogs/data-ingestion/fluentbit.html
  - /victorialogs/data-ingestion/Fluentbit.html
---
VictoriaLogs supports given below Fluentbit outputs:
- [Loki](#loki)
- [HTTP JSON](#http)

## Loki

Specify [loki output](https://docs.fluentbit.io/manual/pipeline/outputs/loki) section in the `fluentbit.conf`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```conf
[OUTPUT]
    name       loki
    match      *
    host       victorialogs
    uri        /insert/loki/api/v1/push
    port       9428
    label_keys $path,$log,$time
    header     VL-Msg-Field log
    header     VL-Time-Field time
    header     VL-Stream-Fields path
```

## HTTP

Specify [http output](https://docs.fluentbit.io/manual/pipeline/outputs/http) section in the `fluentbit.conf`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```fluentbit
[Output]
     Name http
     Match *
     host localhost
     port 9428
     uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date
     format json_lines
     json_date_format iso8601
```

Substitute the host (`localhost`) and port (`9428`) with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on the query args specified in the `uri`.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) in the `uri`
and inspecting VictoriaLogs logs then:

```fluentbit
[Output]
     Name http
     Match *
     host localhost
     port 9428
     uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date&debug=1
     format json_lines
     json_date_format iso8601
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```fluentbit
[Output]
     Name http
     Match *
     host localhost
     port 9428
     uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date&ignore_fields=log.offset,event.original
     format json_lines
     json_date_format iso8601
```

If the Fluentbit sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compress gzip` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```fluentbit
[Output]
     Name http
     Match *
     host localhost
     port 9428
     uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date
     format json_lines
     json_date_format iso8601
     compress gzip
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/keyconcepts/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `header` options.
For example, the following `fluentbit.conf` config instructs Fluentbit to store the data to `(AccountID=12, ProjectID=34)` tenant:

```fluentbit
[Output]
     Name http
     Match *
     host localhost
     port 9428
     uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date
     format json_lines
     json_date_format iso8601
     header AccountID 12
     header ProjectID 23
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Fluentbit HTTP output config docs](https://docs.fluentbit.io/manual/pipeline/outputs/http).
- [Docker-compose demo for Fluentbit integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/fluentbit).
