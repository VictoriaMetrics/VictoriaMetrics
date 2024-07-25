---
weight: 4
title: Promtail setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 4
aliases:
  - /VictoriaLogs/data-ingestion/Promtail.html
  - /victorialogs/data-ingestion/Promtail.html
  - /victorialogs/data-ingestion/promtail.html
---
[Promtail](https://grafana.com/docs/loki/latest/clients/promtail/) is a default log shipper for Grafana Loki.
Promtail can be configured to send the collected logs to VictoriaLogs according to the following docs.

Specify [`clients`](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#clients) section in the configuration file
for sending the collected logs to [VictoriaLogs](../README.md):

```yaml
clients:
  - url: http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance,job,host,app
```

Substitute `localhost:9428` address inside `clients` with the real TCP address of VictoriaLogs.

By default VictoriaLogs stores all the ingested logs into a single [log stream](../keyConcepts.md#stream-fields).
Storing all the logs in a single log stream may be not so efficient, so it is recommended to specify `_stream_fields` query arg
with the list of labels, which uniquely identify log streams. There is no need in specifying all the labels Promtail generates there -
it is usually enough specifying `instance` and `job` labels. See [these docs](../keyConcepts.md#stream-fields)
for details.

See also [these docs](./#http-parameters) for details on other supported query args.
There is no need in specifying `_msg_field` and `_time_field` query args, since VictoriaLogs automatically extracts log message and timestamp from the ingested Loki data.

It is recommended verifying whether the initial setup generates the needed [log fields](../keyConcepts.md#data-model)
and uses the correct [stream fields](../keyConcepts.md#stream-fields).
This can be done by specifying `debug` [parameter](./#http-parameters)
and inspecting VictoriaLogs logs then:

```yaml
clients:
  - url: http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance,job,host,app&debug=1
```

If some [log fields](../keyConcepts.md#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](./#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `filename` and `stream` fields in the ingested logs:

```yaml
clients:
  - url: http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance,job,host,app&ignore_fields=filename,stream
```

By default the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](../#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `tenant_id` field
in the [Loki client configuration](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#clients)
The `tenant_id` must have `AccountID:ProjectID` format, where `AccountID` and `ProjectID` are arbitrary uint32 numbers.
For example, the following config instructs VictoriaLogs to store logs in the `(AccountID=12, ProjectID=34)` [tenant](../#multitenancy):

```yaml
clients:
  - url: http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance,job,host,app&debug=1
    tenant_id: "12:34"
```

The ingested log entries can be queried according to [these docs](../querying/README.md).

See also [data ingestion troubleshooting](./#troubleshooting) docs.
