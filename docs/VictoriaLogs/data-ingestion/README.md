[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) can accept logs from the following log collectors:

- Syslog, Rsyslog and Syslog-ng - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/).
- Filebeat - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/filebeat/).
- Fluentbit - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/fluentbit/).
- Fluentd - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/fluentd/).
- Logstash - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/logstash/).
- Vector - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/vector/).
- Promtail (aka Grafana Loki, Grafana Agent or Grafana Alloy) - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/promtail/).
- Telegraf - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/telegraf/).
- OpenTelemetry Collector - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/opentelemetry/).
- Journald - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/).
- DataDog - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/datadog-agent/).

The ingested logs can be queried according to [these docs](https://docs.victoriametrics.com/victorialogs/querying/).

See also:

- [Log collectors and data ingestion formats](#log-collectors-and-data-ingestion-formats).
- [Data ingestion troubleshooting](#troubleshooting).

## HTTP APIs

VictoriaLogs supports the following data ingestion HTTP APIs:

- Elasticsearch bulk API. See [these docs](#elasticsearch-bulk-api).
- JSON stream API aka [ndjson](https://jsonlines.org/). See [these docs](#json-stream-api).
- Loki JSON API. See [these docs](#loki-json-api).
- OpenTelemetry API. See [these docs](#opentelemetry-api).
- Journald export format.

VictoriaLogs accepts optional [HTTP parameters](#http-parameters) at data ingestion HTTP APIs.

### Elasticsearch bulk API

VictoriaLogs accepts logs in [Elasticsearch bulk API](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html)
/ [OpenSearch Bulk API](http://opensearch.org/docs/1.2/opensearch/rest-api/document-apis/bulk/) format
at `http://localhost:9428/insert/elasticsearch/_bulk` endpoint.

The following command pushes a single log line to VictoriaLogs:

```sh
echo '{"create":{}}
{"_msg":"cannot open file","_time":"0","host.name":"host123"}
' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://localhost:9428/insert/elasticsearch/_bulk
```

It is possible to push thousands of log lines in a single request to this API.

If the [timestamp field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) is set to `"0"`,
then the current timestamp at VictoriaLogs side is used per each ingested log line.
Otherwise the timestamp field must be in one of the following formats:

- [ISO8601](https://en.wikipedia.org/wiki/ISO_8601) or [RFC3339](https://www.rfc-editor.org/rfc/rfc3339).
  For example, `2023-06-20T15:32:10Z` or `2023-06-20 15:32:10.123456789+02:00`.
  If timezone information is missing (for example, `2023-06-20 15:32:10`),
  then the time is parsed in the local timezone of the host where VictoriaLogs runs.

- Unix timestamp in seconds or in milliseconds. For example, `1686026893` (seconds) or `1686026893735` (milliseconds).

See [these docs](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for details on fields,
which must be present in the ingested log messages.

The API accepts various http parameters, which can change the data ingestion behavior - [these docs](#http-parameters) for details.

The following command verifies that the data has been successfully ingested to VictoriaLogs by [querying](https://docs.victoriametrics.com/victorialogs/querying/) it:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=host.name:host123'
```

The command should return the following response:

```sh
{"_msg":"cannot open file","_stream":"{}","_time":"2023-06-21T04:24:24Z","host.name":"host123"}
```

The response by default contains all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victorialogs/logsql/#querying-specific-fields).

The duration of requests to `/insert/elasticsearch/_bulk` can be monitored with `vl_http_request_duration_seconds{path="/insert/elasticsearch/_bulk"}` metric.

See also:

- [How to debug data ingestion](#troubleshooting).
- [HTTP parameters, which can be passed to the API](#http-parameters).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

### JSON stream API

VictoriaLogs accepts JSON line stream aka [ndjson](https://jsonlines.org/) at `http://localhost:9428/insert/jsonline` endpoint.

The following command pushes multiple log lines to VictoriaLogs:

 ```sh
echo '{ "log": { "level": "info", "message": "hello world" }, "date": "0", "stream": "stream1" }
{ "log": { "level": "error", "message": "oh no!" }, "date": "0", "stream": "stream1" }
{ "log": { "level": "info", "message": "hello world" }, "date": "0", "stream": "stream2" }
' | curl -X POST -H 'Content-Type: application/stream+json' --data-binary @- \
  'http://localhost:9428/insert/jsonline?_stream_fields=stream&_time_field=date&_msg_field=log.message'
```

It is possible to push unlimited number of log lines in a single request to this API.

If the [timestamp field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) is set to `"0"`,
then the current timestamp at VictoriaLogs side is used per each ingested log line.
Otherwise the timestamp field must be in one of the following formats:

- [ISO8601](https://en.wikipedia.org/wiki/ISO_8601) or [RFC3339](https://www.rfc-editor.org/rfc/rfc3339).
  For example, `2023-06-20T15:32:10Z` or `2023-06-20 15:32:10.123456789+02:00`.
  If timezone information is missing (for example, `2023-06-20 15:32:10`),
  then the time is parsed in the local timezone of the host where VictoriaLogs runs.

- Unix timestamp in seconds or in milliseconds. For example, `1686026893` (seconds) or `1686026893735` (milliseconds).

See [these docs](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for details on fields,
which must be present in the ingested log messages.

The API accepts various http parameters, which can change the data ingestion behavior - [these docs](#http-parameters) for details.

The following command verifies that the data has been successfully ingested into VictoriaLogs by [querying](https://docs.victoriametrics.com/victorialogs/querying/) it:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=log.level:*'
```

The command should return the following response:

```sh
{"_msg":"hello world","_stream":"{stream=\"stream2\"}","_time":"2023-06-20T13:35:11.56789Z","log.level":"info"}
{"_msg":"hello world","_stream":"{stream=\"stream1\"}","_time":"2023-06-20T15:31:23Z","log.level":"info"}
{"_msg":"oh no!","_stream":"{stream=\"stream1\"}","_time":"2023-06-20T15:32:10.567Z","log.level":"error"}
```

The response by default contains all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victorialogs/logsql/#querying-specific-fields).

The duration of requests to `/insert/jsonline` can be monitored with `vl_http_request_duration_seconds{path="/insert/jsonline"}` metric.

See also:

- [How to debug data ingestion](#troubleshooting).
- [HTTP parameters, which can be passed to the API](#http-parameters).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

### Loki JSON API

VictoriaLogs accepts logs in [Loki JSON API](https://grafana.com/docs/loki/latest/reference/loki-http-api/#ingest-logs) format at `http://localhost:9428/insert/loki/api/v1/push` endpoint.

The following command pushes a single log line to Loki JSON API at VictoriaLogs:

```sh
curl -H "Content-Type: application/json" -XPOST "http://localhost:9428/insert/loki/api/v1/push" --data-raw \
  '{"streams": [{ "stream": { "instance": "host123", "job": "app42" }, "values": [ [ "0", "foo fizzbuzz bar" ] ] }]}'
```

It is possible to push thousands of log streams and log lines in a single request to this API.

The following command verifies that the data has been successfully ingested into VictoriaLogs by [querying](https://docs.victoriametrics.com/victorialogs/querying/) it:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=fizzbuzz'
```

The command should return the following response:

```sh
{"_msg":"foo fizzbuzz bar","_stream":"{instance=\"host123\",job=\"app42\"}","_time":"2023-07-20T23:01:19.288676497Z"}
```

The response by default contains all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victorialogs/logsql/#querying-specific-fields).

The `/insert/loki/api/v1/push` accepts various http parameters, which can change the data ingestion behavior - [these docs](#http-parameters) for details.
There is no need in specifying `_msg_field` and `_time_field` query args, since VictoriaLogs automatically extracts log message and timestamp from the ingested Loki data.

The `_stream_fields` arg is optional. If it isn't set, then all the labels inside the `"stream":{...}` are treated
as [log stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields). Use `_stream_fields` query arg for overriding the list of stream fields.
For example, the following query instructs using only the `instance` label from the `"stream":{...}` as a stream field, while `ip` and `trace_id` fields will be stored
as usual [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```sh
curl -H "Content-Type: application/json" -XPOST "http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance" --data-raw \
  '{"streams": [{ "stream": { "instance": "host123", "ip": "foo", "trace_id": "bar" }, "values": [ [ "0", "foo fizzbuzz bar" ] ] }]}'
```

The duration of requests to `/insert/loki/api/v1/push` can be monitored with `vl_http_request_duration_seconds{path="/insert/loki/api/v1/push"}` metric.

See also:

- [How to debug data ingestion](#troubleshooting).
- [HTTP parameters, which can be passed to the API](#http-parameters).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

### HTTP parameters

VictoriaLogs accepts the following configuration parameters via [HTTP headers](https://en.wikipedia.org/wiki/List_of_HTTP_header_fields)
or via [HTTP query string args](https://en.wikipedia.org/wiki/Query_string) at [data ingestion HTTP APIs](#http-apis).
HTTP query string parameters have priority over HTTP Headers.

#### HTTP Query string parameters

All the [HTTP-based data ingestion protocols](#http-apis) support the following [HTTP query string](https://en.wikipedia.org/wiki/Query_string) args:

- `_msg_field` - the name of the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  containing [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
  This is usually the `message` field for Filebeat and Logstash.

  The `_msg_field` arg may contain comma-separated list of field names. In this case the first non-empty field from the list
  is treated as [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).

  If the `_msg_field` arg isn't set, then VictoriaLogs reads the log message from the `_msg` field. If the `_msg` field is empty,
  then it is set to `-defaultMsgValue` command-line flag value.

- `_time_field` - the name of the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  containing [log timestamp](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).
  This is usually the `@timestamp` field for Filebeat and Logstash.

  If the `_time_field` arg isn't set, then VictoriaLogs reads the timestamp from the `_time` field. If this field doesn't exist, then the current timestamp is used.

- `_stream_fields` - comma-separated list of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names,
  which uniquely identify every [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).

  If the `_stream_fields` arg isn't set, then all the ingested logs are written to default log stream - `{}`.

- `ignore_fields` - an optional comma-separated list of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names,
  which must be ignored during data ingestion.

- `extra_fields` - an optional comma-separated list of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
  which must be added to all the ingested logs. The format of every `extra_fields` entry is `field_name=field_value`.
  If the log entry contains fields from the `extra_fields`, then they are overwritten by the values specified in `extra_fields`.

- `debug` - if this arg is set to `1`, then the ingested logs aren't stored in VictoriaLogs. Instead,
  the ingested data is logged by VictoriaLogs, so it can be investigated later.

See also [HTTP headers](#http-headers).

#### HTTP headers

All the [HTTP-based data ingestion protocols](#http-apis) support the following [HTTP Headers](https://en.wikipedia.org/wiki/List_of_HTTP_header_fields)
additionally to [HTTP query args](#http-query-string-parameters):

- `AccountID` - accountID of the tenant to ingest data to. See [multitenancy docs](https://docs.victoriametrics.com/victorialogs/#multitenancy) for details.

- `ProjectID`- projectID of the tenant to ingest data to. See [multitenancy docs](https://docs.victoriametrics.com/victorialogs/#multitenancy) for details.

- `VL-Msg-Field` - the name of the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  containing [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
  This is usually the `message` field for Filebeat and Logstash.

  The `VL-Msg-Field` header may contain comma-separated list of field names. In this case the first non-empty field from the list
  is treated as [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).

  If the `VL-Msg-Field` header isn't set, then VictoriaLogs reads log message from the `_msg` field. If the `_msg` field is empty,
  then it is set to `-defaultMsgValue` command-line flag value.

- `VL-Time-Field` - the name of the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  containing [log timestamp](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).
  This is usually the `@timestamp` field for Filebeat and Logstash.

  If the `VL-Time-Field` header isn't set, then VictoriaLogs reads the timestamp from the `_time` field. If this field doesn't exist, then the current timestamp is used.

- `VL-Stream-Fields` - comma-separated list of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names,
  which uniquely identify every [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).

  If the `VL-Stream-Fields` header isn't set, then all the ingested logs are written to default log stream - `{}`.

- `VL-Ignore-Fields` - an optional comma-separated list of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names,
  which must be ignored during data ingestion.

- `VL-Extra-Fields` - an optional comma-separated list of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
  which must be added to all the ingested logs. The format of every `extra_fields` entry is `field_name=field_value`.
  If the log entry contains fields from the `extra_fields`, then they are overwritten by the values specified in `extra_fields`.

- `VL-Debug` - if this parameter is set to `1`, then the ingested logs aren't stored in VictoriaLogs. Instead,
  the ingested data is logged by VictoriaLogs, so it can be investigated later.

See also [HTTP Query string parameters](#http-query-string-parameters).

## Troubleshooting

The following command can be used for verifying whether the data is successfully ingested into VictoriaLogs:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=*' | head
```

This command selects all the data ingested into VictoriaLogs via [HTTP query API](https://docs.victoriametrics.com/victorialogs/querying/#http-api)
using [any value filter](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter),
while `head` cancels query execution after reading the first 10 log lines. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)
for more details on how `head` integrates with VictoriaLogs.

The response by default contains all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victorialogs/logsql/#querying-specific-fields).

VictoriaLogs provides the following command-line flags, which can help debugging data ingestion issues:

- `-logNewStreams` - if this flag is passed to VictoriaLogs, then it logs all the newly
  registered [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
  This may help debugging [high cardinality issues](https://docs.victoriametrics.com/victorialogs/keyconcepts/#high-cardinality).
- `-logIngestedRows` - if this flag is passed to VictoriaLogs, then it logs all the ingested
  [log entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
  See also `debug` [parameter](#http-parameters).

VictoriaLogs exposes various [metrics](https://docs.victoriametrics.com/victorialogs/#monitoring), which may help debugging data ingestion issues:

- `vl_rows_ingested_total` - the number of ingested [log entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  since the last VictoriaLogs restart. If this number increases over time, then logs are successfully ingested into VictoriaLogs.
  The ingested logs can be inspected in the following ways:
  - By passing `debug=1` parameter to every request to [data ingestion APIs](#http-apis). The ingested rows aren't stored in VictoriaLogs
    in this case. Instead, they are logged, so they can be investigated later.
    The `vl_rows_dropped_total` [metric](https://docs.victoriametrics.com/victorialogs/#monitoring) is incremented for each logged row.
  - By passing `-logIngestedRows` command-line flag to VictoriaLogs. In this case it logs all the ingested data, so it can be investigated later.
- `vl_streams_created_total` - the number of created [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
  since the last VictoriaLogs restart. If this metric grows rapidly during extended periods of time, then this may lead
  to [high cardinality issues](https://docs.victoriametrics.com/victorialogs/keyconcepts/#high-cardinality).
  The newly created log streams can be inspected in logs by passing `-logNewStreams` command-line flag to VictoriaLogs.

## Log collectors and data ingestion formats

Here is the list of log collectors and their ingestion formats supported by VictoriaLogs:

| How to setup the collector | Format: Elasticsearch | Format: JSON Stream | Format: Loki | Format: syslog | Format: OpenTelemetry | Format: Journald | Format: DataDog |
|----------------------------|-----------------------|---------------------|--------------|----------------|-----------------------|------------------|-----------------|
| [Rsyslog](https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/) | [Yes](https://www.rsyslog.com/doc/configuration/modules/omelasticsearch.html) | No | No | [Yes](https://www.rsyslog.com/doc/configuration/modules/omfwd.html) | No | No | No |
| [Syslog-ng](https://docs.victoriametrics.com/victorialogs/data-ingestion/filebeat/) | Yes, [v1](https://support.oneidentity.com/technical-documents/syslog-ng-open-source-edition/3.16/administration-guide/28#TOPIC-956489), [v2](https://support.oneidentity.com/technical-documents/doc/syslog-ng-open-source-edition/3.16/administration-guide/29#TOPIC-956494) | No | No | [Yes](https://support.oneidentity.com/technical-documents/doc/syslog-ng-open-source-edition/3.16/administration-guide/44#TOPIC-956553) | No | No | No |
| [Filebeat](https://docs.victoriametrics.com/victorialogs/data-ingestion/filebeat/) | [Yes](https://www.elastic.co/guide/en/beats/filebeat/current/elasticsearch-output.html) | No | No | No | No | No | No |
| [Fluentbit](https://docs.victoriametrics.com/victorialogs/data-ingestion/fluentbit/) | No | [Yes](https://docs.fluentbit.io/manual/pipeline/outputs/http) | [Yes](https://docs.fluentbit.io/manual/pipeline/outputs/loki) | [Yes](https://docs.fluentbit.io/manual/pipeline/outputs/syslog) | [Yes](https://docs.fluentbit.io/manual/pipeline/outputs/opentelemetry) | No | [Yes](https://docs.fluentbit.io/manual/pipeline/outputs/datadog) |
| [Logstash](https://docs.victoriametrics.com/victorialogs/data-ingestion/logstash/)   | [Yes](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html) | No | No | [Yes](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-syslog.html) | [Yes](https://github.com/paulgrav/logstash-output-opentelemetry) | No | [Yes](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-datadog.html) |
| [Vector](https://docs.victoriametrics.com/victorialogs/data-ingestion/vector/) | [Yes](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/) | [Yes](https://vector.dev/docs/reference/configuration/sinks/http/) | [Yes](https://vector.dev/docs/reference/configuration/sinks/loki/) | No | No | No | [Yes](https://vector.dev/docs/reference/configuration/sinks/datadog_logs/) |
| [Promtail](https://docs.victoriametrics.com/victorialogs/data-ingestion/promtail/)   | No | No | [Yes](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#clients) | No | No | No | No |
| [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) | [Yes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/elasticsearchexporter) | No | [Yes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/lokiexporter) | [Yes](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/syslogexporter) | [Yes](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter) | No | [Yes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/datadogexporter) |
| [Telegraf](https://docs.victoriametrics.com/victorialogs/data-ingestion/telegraf/) | [Yes](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/elasticsearch) | [Yes](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/http) | [Yes](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/loki) | [Yes](https://github.com/influxdata/telegraf/blob/master/plugins/outputs/syslog) | Yes | No | No |
| [Fluentd](https://docs.victoriametrics.com/victorialogs/data-ingestion/fluentd/) | [Yes](https://github.com/uken/fluent-plugin-elasticsearch) | [Yes](https://docs.fluentd.org/output/http) | [Yes](https://grafana.com/docs/loki/latest/send-data/fluentd/) | [Yes](https://github.com/fluent-plugins-nursery/fluent-plugin-remote_syslog) | No | No | No |
| [Journald](https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/) | No | No | No | No | No | Yes | No |
| [DataDog Agent](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/datadog-agent) | No | No | No | No | No | No | Yes |

