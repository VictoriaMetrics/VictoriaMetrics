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

# Fluentbit setup

Specify [http output](https://docs.fluentbit.io/manual/pipeline/outputs/http) section in the `fluentbit.conf`
for sending the collected logs to [VictoriaLogs](../README.md):

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

See [these docs](./#http-parameters) for details on the query args specified in the `uri`.

It is recommended verifying whether the initial setup generates the needed [log fields](../keyConcepts.md#data-model)
and uses the correct [stream fields](../keyConcepts.md#stream-fields).
This can be done by specifying `debug` [parameter](./#http-parameters) in the `uri`
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

If some [log fields](../keyConcepts.md#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](./#http-parameters).
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

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](../keyConcepts.md#multitenancy).
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

- [Data ingestion troubleshooting](./#troubleshooting).
- [How to query VictoriaLogs](../querying/README.md).
- [Fluentbit HTTP output config docs](https://docs.fluentbit.io/manual/pipeline/outputs/http).
- [Docker-compose demo for Fluentbit integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/fluentbit-docker).
