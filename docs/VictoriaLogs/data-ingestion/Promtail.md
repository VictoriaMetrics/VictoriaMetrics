# Promtail setup

Specify [`clients`](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#clients) section in the configuration file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```yaml
clients:
  - url: http://vlogs:9428/insert/loki/api/v1/push?_stream_fields=filename,job,stream,host,app,pid
```

Substitute `vlogs:9428` address inside `clients` with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters) for details on the used URL query parameter section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters)
and inspecting VictoriaLogs logs then:

```yaml
clients:
  - url: http://vlogs:9428/insert/loki/api/v1/push?_stream_fields=filename,job,stream,host,app,pid&debug=1
```

If some [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```yaml
clients:
  - url: http://vlogs:9428/insert/loki/api/v1/push?_stream_fields=filename,job,stream,host,app,pid&debug=1
```

By default the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/#multitenancy).
If you need storing logs in other tenant, then It is possible to either use `tenant_id` provided by Loki configuration, or to use `headers` and provide
`AccountID` and `ProjectID` headers. For example, the following config instructs VictoriaLogs to store logs in the `(AccountID=12, ProjectID=12)` tenant:

```yaml
clients:
  - url: http://vlogs:9428/insert/loki/api/v1/push?_stream_fields=filename,job,stream,host,app,pid&debug=1
    tenant_id: "12:12"
```

The ingested log entries can be queried according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

See also [data ingestion troubleshooting](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#troubleshooting) docs.
