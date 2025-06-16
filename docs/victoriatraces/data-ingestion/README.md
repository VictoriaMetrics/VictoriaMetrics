> Currently, VictoriaTraces is in the development version and built on VictoriaLogs. Therefore, they will share some flags and APIs. These flags and APIs will be completely separated once VictoriaTraces reaches a beta/stable version.

[VictoriaTraces](https://docs.victoriametrics.com/victoriatraces/) can accept trace spans via [the OpenTelemetry protocol (OTLP)](https://opentelemetry.io/docs/specs/otlp/).

## HTTP APIs

VictoriaTraces supports the following data ingestion HTTP API:

- OpenTelemetry API. See [these docs](#opentelemetry-api).

VictoriaTraces accepts optional [HTTP parameters](#http-parameters) at data ingestion HTTP APIs.

### Opentelemetry API

VictoriaTraces accepts trace spans in [OpenTelemetry format](https://opentelemetry.io/docs/specs/otel/traces/data-model/) at the `/insert/opentelemetry/v1/traces` HTTP endpoint.
See more details [in these docs](https://docs.victoriametrics.com/victoriatraces/data-ingestion/opentelemetry/).

### HTTP parameters

VictoriaTraces accepts the following configuration parameters via [HTTP headers](https://en.wikipedia.org/wiki/List_of_HTTP_header_fields)
or via [HTTP query string args](https://en.wikipedia.org/wiki/Query_string) at [data ingestion HTTP APIs](#http-apis).
HTTP query string parameters have priority over HTTP Headers.

#### HTTP Query string parameters

All the [HTTP-based data ingestion protocols](#http-apis) support the following [HTTP query string](https://en.wikipedia.org/wiki/Query_string) args:

- `extra_fields` - an optional comma-separated list of [trace fields](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model),
  which must be added to all the ingested traces. The format of every `extra_fields` entry is `field_name=field_value`.
  If the trace entry contains fields from the `extra_fields`, then they are overwritten by the values specified in `extra_fields`.

- `debug` - if this arg is set to `1`, then the ingested traces aren't stored in VictoriaTraces. Instead,
  the ingested data is traceged by VictoriaTraces, so it can be investigated later.

See also [HTTP headers](#http-headers).

#### HTTP headers

All the [HTTP-based data ingestion protocols](#http-apis) support the following [HTTP Headers](https://en.wikipedia.org/wiki/List_of_HTTP_header_fields)
additionally to [HTTP query args](#http-query-string-parameters):

- `AccountID` - accountID of the tenant to ingest data to. See [multitenancy docs](https://docs.victoriametrics.com/victoriatraces/#multitenancy) for details.

- `ProjectID`- projectID of the tenant to ingest data to. See [multitenancy docs](https://docs.victoriametrics.com/victoriatraces/#multitenancy) for details.

- `VL-Extra-Fields` - an optional comma-separated list of [trace fields](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model),
  which must be added to all the ingested traces. The format of every `extra_fields` entry is `field_name=field_value`.
  If the trace entry contains fields from the `extra_fields`, then they are overwritten by the values specified in `extra_fields`.

- `VL-Debug` - if this parameter is set to `1`, then the ingested traces aren't stored in VictoriaTraces. Instead,
  the ingested data is traceged by VictoriaTraces, so it can be investigated later.

See also [HTTP Query string parameters](#http-query-string-parameters).

## Troubleshooting

The following command can be used for verifying whether the data is successfully ingested into VictoriaTraces:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=*' | head
```

This command selects all the data ingested into VictoriaTraces via [HTTP query API](https://docs.victoriametrics.com/victoriatraces/querying/#http-api)
using [any value filter](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter),
while `head` cancels query execution after reading the first 10 trace spans. See [these docs](https://docs.victoriametrics.com/victoriatraces/querying/#command-line)
for more details on how `head` integrates with VictoriaTraces.

The response by default contains all the [trace span fields](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victoriatraces/logsql/#querying-specific-fields).

VictoriaTraces provides the following command-line flags, which can help debugging data ingestion issues:

- `-logNewStreams` - if this flag is passed to VictoriaTraces, then it traces all the newly
  registered [streams](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#stream-fields).
  This may help debugging [high cardinality issues](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#high-cardinality).
- `-logIngestedRows` - if this flag is passed to VictoriaTraces, then it traces all the ingested
  [trace span entries](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model).
  See also `debug` [parameter](#http-parameters).

VictoriaTraces exposes various [metrics](https://docs.victoriametrics.com/victoriatraces/#monitoring), which may help debugging data ingestion issues:

- `vl_rows_ingested_total` - the number of ingested [trace span entries](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#data-model)
  since the last VictoriaTraces restart. If this number increases over time, then trace spans are successfully ingested into VictoriaTraces.
  The ingested trace spans can be inspected in the following ways:
    - By passing `debug=1` parameter to every request to [data ingestion APIs](#http-apis). The ingested spans aren't stored in VictoriaTraces
      in this case. Instead, they are logged, so they can be investigated later.
      The `vl_rows_dropped_total` [metric](https://docs.victoriametrics.com/victoriatraces/#monitoring) is incremented for each logged row.
    - By passing `-logIngestedRows` command-line flag to VictoriaTraces. In this case it traces all the ingested data, so it can be investigated later.
- `vl_streams_created_total` - the number of created [trace streams](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#stream-fields)
  since the last VictoriaTraces restart. If this metric grows rapidly during extended periods of time, then this may lead
  to [high cardinality issues](https://docs.victoriametrics.com/victoriatraces/keyconcepts/#high-cardinality).
  The newly created trace streams can be inspected in traces by passing `-logNewStreams` command-line flag to VictoriaTraces.

