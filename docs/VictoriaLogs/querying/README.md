[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) can be queried with [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/)
via the following ways:

- [Command-line interface](#command-line)
- [HTTP API](#http-api)
- [Web UI](#web-ui) - a web-based UI for querying logs
- [Grafana plugin](#visualization-in-grafana)

## HTTP API

VictoriaLogs provides the following HTTP endpoints:

- [`/select/logsql/query`](#querying-logs) for querying logs.
- [`/select/logsql/tail`](#live-tailing) for live tailing of query results.
- [`/select/logsql/hits`](#querying-hits-stats) for querying log hits stats over the given time range.
- [`/select/logsql/stats_query`](#querying-log-stats) for querying log stats at the given time.
- [`/select/logsql/stats_query_range`](#querying-log-range-stats) for querying log stats over the given time range.
- [`/select/logsql/stream_ids`](#querying-stream_ids) for querying `_stream_id` values of [log streams](#https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
- [`/select/logsql/streams`](#querying-streams) for querying [log streams](#https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
- [`/select/logsql/stream_field_names`](#querying-stream-field-names) for querying [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field names.
- [`/select/logsql/stream_field_values`](#querying-stream-field-values) for querying [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field values.
- [`/select/logsql/field_names`](#querying-field-names) for querying [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names.
- [`/select/logsql/field_values`](#querying-field-values) for querying [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) values.

### Querying logs

Logs stored in VictoriaLogs can be queried at the `/select/logsql/query` HTTP endpoint.
The [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query must be passed via `query` argument.
For example, the following query returns all the log entries with the `error` word:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

The response by default contains all the [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for the selected logs.
Use [`fields` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe) for selecting only the needed fields.

The `query` argument can be passed either in the request url itself (aka HTTP GET request) or via request body
with the `x-www-form-urlencoded` encoding (aka HTTP POST request). The HTTP POST is useful for sending long queries
when they do not fit the maximum url length of the used clients and proxies.

See [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/) for details on what can be passed to the `query` arg.
The `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools.

By default the `/select/logsql/query` returns all the log entries matching the given `query`. The response size can be limited in the following ways:

- By closing the response stream at any time. VictoriaLogs stops query execution and frees all the resources occupied by the request as soon as it detects closed client connection.
  So it is safe running [`*` query](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter), which selects all the logs, even if trillions of logs are stored in VictoriaLogs.
- By specifying the maximum number of log entries, which can be returned in the response via `limit` query arg. For example, the following command returns
  up to 10 most recently added log entries with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
  in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):
  ```sh
  curl http://localhost:9428/select/logsql/query -d 'query=error' -d 'limit=10'
  ```
- By adding [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe) to the query. For example, the following command returns up to 10 **random** log entries
  with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):
  ```sh
  curl http://localhost:9428/select/logsql/query -d 'query=error | limit 10'
  ```
- By adding [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter). The time range for the query can be specified via optional
  `start` and `end` query args formatted according to [these docs](https://docs.victoriametrics.com/single-server-victoriametrics/#timestamp-formats).
- By adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) to the query, which select lower number of logs.

The `/select/logsql/query` endpoint returns [a stream of JSON lines](https://jsonlines.org/),
where each line contains JSON-encoded log entry in the form `{field1="value1",...,fieldN="valueN"}`.
Example response:

```
{"_msg":"error: disconnect from 19.54.37.22: Auth fail [preauth]","_stream":"{}","_time":"2023-01-01T13:32:13Z"}
{"_msg":"some other error","_stream":"{}","_time":"2023-01-01T13:32:15Z"}
```

Logs lines are sent to the response stream as soon as they are found in VictoriaLogs storage.
This means that the returned response may contain billions of lines for queries matching too many log entries.
The response can be interrupted at any time by closing the connection to VictoriaLogs server.
This allows post-processing the returned lines at the client side with the usual Unix commands such as `grep`, `jq`, `less`, `head`, etc.,
without worrying about resource usage at VictoriaLogs side. See [these docs](#command-line) for more details.

The returned lines aren't sorted by default, since sorting disables the ability to send matching log entries to response stream as soon as they are found.
Query results can be sorted in the following ways:

- By passing `limit=N` query arg to `/select/logsql/query`. The up to `N` most recent matching log entries are returned in the response.
- By adding [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) to the query.
- By using Unix `sort` command at client side according to [these docs](#command-line).

The maximum query execution time is limited by `-search.maxQueryDuration` command-line flag value. This limit can be overridden to smaller values
on a per-query basis by passing the needed timeout via `timeout` query arg. For example, the following command limits query execution time
to 4.2 seconds:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' -d 'timeout=4.2s'
```

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query searches
for log messages at `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/query -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=error'
```

The number of requests to `/select/logsql/query` can be [monitored](https://docs.victoriametrics.com/victorialogs/#monitoring)
with `vl_http_requests_total{path="/select/logsql/query"}` metric.

See also:

- [Live tailing](#live-tailing)
- [Querying hits stats](#querying-hits-stats)
- [Querying log stats](#querying-log-stats)
- [Querying log range stats](#querying-log-range-stats)
- [Querying streams](#querying-streams)
- [Querying stream field names](#querying-stream-field-names)
- [Querying stream field values](#querying-stream-field-values)
- [Querying field names](#querying-field-names)
- [Querying field values](#querying-field-values)


### Live tailing

VictoriaLogs provides `/select/logsql/tail?query=<query>` HTTP endpoint, which returns live tailing results for the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/),
e.g. it works in the way similar to `tail -f` unix command. For example, the following command returns live tailing logs with the `error` word:

```sh
curl -N http://localhost:9428/select/logsql/tail -d 'query=error'
```

The `-N` command-line flag is essential to pass to `curl` during live tailing, since otherwise curl may delay displaying matching logs
because of internal response bufferring.

The `<query>` must conform the following rules:

- It cannot contain the following [pipes](https://docs.victoriametrics.com/victorialogs/logsql/#pipes):
  - pipes, which calculate stats over the logs - [`stats`](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe),
    [`uniq`](https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe), [`top`](https://docs.victoriametrics.com/victorialogs/logsql/#top-pipe)
  - pipes, which change the order of logs - [`sort`](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
  - pipes, which limit or ignore some logs - [`limit`](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe),
    [`offset`](https://docs.victoriametrics.com/victorialogs/logsql/#offset-pipe).

- It must select [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) field.

- It is recommended to return [`_stream_id`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field for more accurate live tailing
  across multiple streams.

**Performance tip**: live tailing works the best if it matches newly ingested logs at relatively slow rate (e.g. up to 1K matching logs per second),
e.g. it is optimized for the case when real humans inspect the output of live tailing in the real time. If live tailing returns logs at too high rate,
then it is recommended adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) to the `<query>`, so it matches less logs.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query performs live tailing
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl -N http://localhost:9428/select/logsql/tail -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=error'
```

The number of currently executed live tailing requests to `/select/logsql/tail` can be [monitored](https://docs.victoriametrics.com/victorialogs/#monitoring)
with `vl_live_tailing_requests` metric.

See also:

- [Querying logs](#querying-logs)
- [Querying streams](#querying-streams)

### Querying hits stats

VictoriaLogs provides `/select/logsql/hits?query=<query>&start=<start>&end=<end>&step=<step>` HTTP endpoint, which returns the number
of matching log entries for the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]`
time range grouped by `<step>` buckets. The returned results are sorted by time.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

The `<step>` arg can contain values in [the format specified here](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-time-buckets).
If `<step>` is missing, then it equals to `1d` (one day).

For example, the following command returns per-hour number of [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) over logs for the last 3 hours:

```sh
curl http://localhost:9428/select/logsql/hits -d 'query=error' -d 'start=3h' -d 'step=1h'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "hits": [
    {
      "fields": {},
      "timestamps": [
        "2024-01-01T00:00:00Z",
        "2024-01-01T01:00:00Z",
        "2024-01-01T02:00:00Z"
      ],
      "values": [
        410339,
        450311,
        899506
      ],
      "total": 1760176
    }
  ]
}
```

Additionally, the `offset=<offset>` arg can be passed to `/select/logsql/hits` in order to group buckets according to the given timezone offset.
The `<offset>` can contain values in [the format specified here](https://docs.victoriametrics.com/victorialogs/logsql/#duration-values).
For example, the following command returns per-day number of logs with `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
over the last week in New York time zone (`-4h`):

```sh
curl http://localhost:9428/select/logsql/hits -d 'query=error' -d 'start=1w' -d 'step=1d' -d 'offset=-4h'
```

Additionally, any number of `field=<field_name>` args can be passed to `/select/logsql/hits` for grouping hits buckets by the mentioned `<field_name>` fields.
For example, the following query groups hits by `level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) additionally to the provided `step`:

```sh
curl http://localhost:9428/select/logsql/hits -d 'query=*' -d 'start=3h' -d 'step=1h' -d 'field=level'
```

The grouped fields are put inside `"fields"` object:

```json
{
  "hits": [
    {
      "fields": {
        "level": "error"
      },
      "timestamps": [
        "2024-01-01T00:00:00Z",
        "2024-01-01T01:00:00Z",
        "2024-01-01T02:00:00Z"
      ],
      "values": [
        25,
        20,
        15
      ],
      "total": 60
    },
    {
      "fields": {
        "level": "info"
      },
      "timestamps": [
        "2024-01-01T00:00:00Z",
        "2024-01-01T01:00:00Z",
        "2024-01-01T02:00:00Z"
      ],
      "values": [
        25625,
        35043,
        25230
      ],
      "total": 85898
    }
  ]
}
```

Optional `fields_limit=N` query arg can be passed to `/select/logsql/hits` for limiting the number of unique `"fields"` groups to return to `N`.
If more than `N` unique `"fields"` groups is found, then top `N` `"fields"` groups with the maximum number of `"total"` hits are returned.
The remaining hits are returned in `"fields": {}` group.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns hits stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/hits -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=error'
```

See also:

- [Querying logs](#querying-logs)
- [Querying log stats](#querying-log-stats)
- [Querying log range stats](#querying-log-range-stats)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying log stats

VictoriaLogs provides `/select/logsql/stats_query?query=<query>&time=<t>` HTTP endpoint, which returns log stats
for the given [`query`](https://docs.victoriametrics.com/victorialogs/logsql/) at the given timestamp `t`
in the format compatible with [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries).

The `<query>` must contain [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe). The calculated stats is converted into metrics
with labels from `by(...)` clause of the `| stats by(...)` pipe.

The `<t>` arg can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<t>` is missing, then it equals to the current time.

For example, the following command returns the number of logs per each `level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across logs over `2024-01-01` day by UTC:

```sh
curl http://localhost:9428/select/logsql/stats_query -d 'query=_time:1d | stats by (level) count(*)' -d 'time=2024-01-02Z'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "__name__": "count(*)",
          "level": "info"
        },
        "value": [
          1704153600,
          "20395342"
        ]
      },
      {
        "metric": {
          "__name__": "count(*)",
          "level": "warn"
        },
        "value": [
          1704153600,
          "1239222"
        ]
      },
      {
        "metric": {
          "__name__": "count(*)",
          "level": "error"
        },
        "value": [
          1704153600,
          "832"
        ]
      },
    ]
  }
}
```

The `/select/logsql/stats_query` API is useful for generating Prometheus-compatible alerts and calculating recording rules results.

See also:

- [Querying log range stats](#querying-log-range-stats)
- [Querying logs](#querying-logs)
- [Querying hits stats](#querying-hits-stats)
- [HTTP API](#http-api)

### Querying log range stats

VictoriaLogs provides `/select/logsql/stats_query_range?query=<query>&start=<start>&end=<end>&step=<step>` HTTP endpoint, which returns log stats
for the given [`query`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[start ... end]` time range with the given `step` interval.
The stats is returned in the format compatible with [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

The `<query>` must contain [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe). The calculated stats is converted into metrics
with labels from `by(...)` clause of the `| stats by(...)` pipe.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

The `<step>` arg can contain values in [the format specified here](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-time-buckets).
If `<step>` is missing, then it equals to `1d` (one day).

For example, the following command returns the number of logs per each `level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across logs over `2024-01-01` day by UTC with 6-hour granularity:

```sh
curl http://localhost:9428/select/logsql/stats_query_range -d 'query=* | stats by (level) count(*)' -d 'start=2024-01-01Z' -d 'end=2024-01-02Z' -d 'step=6h'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "status": "success",
  "data": {
    "resultType": "matrix",
    "result": [
      {
        "metric": {
          "__name__": "count(*)",
          "level": "info"
        },
        "values": [
          [
            1704067200,
            "103125"
          ],
          [
            1704088800,
            "102500"
          ],
          [
            1704110400,
            "103125"
          ],
          [
            1704132000,
            "102500"
          ]
        ]
      },
      {
        "metric": {
          "__name__": "count(*)",
          "level": "error"
        },
        "values": [
          [
            1704067200,
            "31"
          ],
          [
            1704088800,
            "25"
          ],
          [
            1704110400,
            "31"
          ],
          [
            1704132000,
            "125"
          ]
        ]
      }
    ]
  }
}
```

The `/select/logsql/stats_query_range` API is useful for generating Prometheus-compatible graphs in Grafana.

See also:

- [Querying log stats](#querying-log-stats)
- [Querying logs](#querying-logs)
- [Querying hits stats](#querying-hits-stats)
- [HTTP API](#http-api)

### Querying stream_ids

VictoriaLogs provides `/select/logsql/stream_ids?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns `_stream_id` values
for the [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) from results
of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every `_stream_id`.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns `_stream_id` values across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/stream_ids -d 'query=error' -d 'start=5m'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "0000000000000000106955b1744a71b78bd3a88c755751e8",
      "hits": 442953
    },
    {
      "value": "0000000000000000b80988e6012df3520a8e20cd5353c52b",
      "hits": 59349
    },
    {
      "value": "0000000000000000f8d02151e40a6cbbb1edb2050ea910ba",
      "hits": 59277
    }
  ]
}
```

The `/select/logsql/stream_ids` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned `_stream_id` values to `N`.
The endpoint returns arbitrary subset of `_stream_id` values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of `_stream_id` values.
When the `limit` is reached, `hits` are zeroed, since they cannot be calculated reliably.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns `_stream_id` stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/stream_ids -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying streams](#querying-streams)
- [Querying logs](#querying-logs)
- [Querying hits stats](#querying-hits-stats)
- [HTTP API](#http-api)

### Querying streams

VictoriaLogs provides `/select/logsql/streams?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
from results of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every `_stream`.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns streams across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/streams -d 'query=error' -d 'start=5m'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "{host=\"host-123\",app=\"foo\"}",
      "hits": 34980
    },
    {
      "value": "{host=\"host-124\",app=\"bar\"}",
      "hits": 32892
    },
    {
      "value": "{host=\"host-125\",app=\"baz\"}",
      "hits": 32877
    }
  ]
}
```

The `/select/logsql/streams` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned streams to `N`.
The endpoint returns arbitrary subset of streams if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of streams.
When the `limit` is reached, `hits` are zeroed, since they cannot be calculated reliably.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns stream stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/streams -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying stream_ids](#querying-stream_ids)
- [Querying logs](#querying-logs)
- [Querying hits stats](#querying-hits-stats)
- [HTTP API](#http-api)

### Querying stream field names

VictoriaLogs provides `/select/logsql/stream_field_names?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns
[log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field names from results
of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every field name.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns stream field names across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/stream_field_names -d 'query=error' -d 'start=5m'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "app",
      "hits": 1033300623
    },
    {
      "value": "container",
      "hits": 1033300623
    },
    {
      "value": "datacenter",
      "hits": 1033300623
    }
  ]
}
```

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns stream field names stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/stream_field_names -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying stream field names](#querying-stream-field-names)
- [Querying field values](#querying-field-values)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying stream field values

VictoriaLogs provides `/select/logsql/stream_field_values?query=<query>&start=<start>&<end>&field=<fieldName>` HTTP endpoint,
which returns [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field values for the field with the given `<fieldName>` name
from results of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every field value.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns values for the stream field `host` across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/stream_field_values -d 'query=error' -d 'start=5m' -d 'field=host'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "host-1",
      "hits": 69426656
    },
    {
      "value": "host-2",
      "hits": 66507749
    }
  ]
}
```

The `/select/logsql/stream_field_values` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned values to `N`.
The endpoint returns arbitrary subset of values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of field values.
When the `limit` is reached, `hits` are zeroed, since they cannot be calculated reliably.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns stream field values stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/stream_field_values -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying stream field values](#querying-stream-field-values)
- [Querying field names](#querying-field-names)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying field names

VictoriaLogs provides `/select/logsql/field_names?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns field names
from results of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every field name.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns field names across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/field_names -d 'query=error' -d 'start=5m'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "_msg",
      "hits": 1033300623
    },
    {
      "value": "_stream",
      "hits": 1033300623
    },
    {
      "value": "_time",
      "hits": 1033300623
    }
  ]
}
```

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns field names stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/field_names -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying stream field names](#querying-stream-field-names)
- [Querying field values](#querying-field-values)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying field values

VictoriaLogs provides `/select/logsql/field_values?query=<query>&field=<fieldName>&start=<start>&end=<end>` HTTP endpoint, which returns
unique values for the given `<fieldName>` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
from results of the given [`<query>`](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every field value.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns unique values for `host` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/field_values -d 'query=error' -d 'field=host' -d 'start=5m'
```

Below is an example JSON output returned from this endpoint:

```json
{
  "values": [
    {
      "value": "host-1",
      "hits": 69426656
    },
    {
      "value": "host-2",
      "hits": 66507749
    },
    {
      "value": "host-3",
      "hits": 65454351
    }
  ]
}
```

The `/select/logsql/field_values` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned values to `N`.
The endpoint returns arbitrary subset of values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of field values.
When the `limit` is reached, `hits` are zeroed, since they cannot be calculated reliably.

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) is queried.
If you need querying other tenant, then specify it via `AccountID` and `ProjectID` http request headers. For example, the following query returns field values stats
for `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/field_values -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=_time:5m'
```

See also:

- [Querying stream field values](#querying-stream-field-values)
- [Querying field names](#querying-field-names)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)


## Web UI

VictoriaLogs provides Web UI for logs [querying](https://docs.victoriametrics.com/victorialogs/logsql/) and exploration
at `http://localhost:9428/select/vmui`.

There are three modes of displaying query results:

- `Group` - results are displayed as a table with rows grouped by [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
- `Table` - displays query results as a table.
- `JSON` - displays raw JSON response from [`/select/logsql/query` HTTP API](#querying-logs).

See also [command line interface](#command-line).

## Visualization in Grafana 

[VictoriaLogs Grafana Datasource](https://docs.victoriametrics.com/victorialogs/victorialogs-datasource/) allows you to query and visualize VictoriaLogs data in Grafana

## Command-line

VictoriaLogs provides `vlogsqcli` interactive command-line tool for querying logs. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/vlogscli/).

VictoriaLogs [querying API](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs) integrates well with `curl`
and other Unix command-line tools because of the following features:

- Matching log entries are sent to the response stream as soon as they are found.
  This allows forwarding the response stream to arbitrary [Unix pipes](https://en.wikipedia.org/wiki/Pipeline_(Unix))
  without waiting until the response finishes.
- Query execution speed is automatically adjusted to the speed of the client, which reads the response stream.
  For example, if the response stream is piped to `less` command, then the query is suspended
  until the `less` command reads the next block from the response stream.
- Query is automatically canceled when the client closes the response stream.
  For example, if the query response is piped to `head` command, then VictoriaLogs stops executing the query
  when the `head` command closes the response stream.

These features allow executing queries at command-line interface, which potentially select billions of rows,
without the risk of high resource usage (CPU, RAM, disk IO) at VictoriaLogs.

For example, the following query can return very big number of matching log entries (e.g. billions) if VictoriaLogs contains
many log messages with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

If the command above returns "never-ending" response, then just press `ctrl+C` at any time in order to cancel the query.
VictoriaLogs notices that the response stream is closed, so it cancels the query and stops consuming CPU, RAM and disk IO for this query.

Then use `head` command for investigating the returned log messages and narrowing down the query:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' | head -10
```

The `head -10` command reads only the first 10 log messages from the response and then closes the response stream.
This automatically cancels the query at VictoriaLogs side, so it stops consuming CPU, RAM and disk IO resources.

Alternatively, you can limit the number of returned logs at VictoriaLogs side via [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error | limit 10'
```

Sometimes it may be more convenient to use `less` command instead of `head` during the investigation of the returned response:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' | less
```

The `less` command reads the response stream on demand, when the user scrolls down the output.
VictoriaLogs suspends query execution when `less` stops reading the response stream.
It doesn't consume CPU and disk IO resources during this time. It resumes query execution
after the `less` continues reading the response stream.

Suppose that the initial investigation of the returned query results helped determining that the needed log messages contain
`cannot open file` [phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter).
Then the query can be narrowed down to `error AND "cannot open file"`
(see [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter) about `AND` operator).
Then run the updated command in order to continue the investigation:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error AND "cannot open file"' | head
```

Note that the `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools. It is highly recommended to use [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/vlogscli/) -
it automatically performs all the needed encoding.

The `pipe the query to "head" or "less" -> investigate the results -> refine the query` iteration
can be repeated multiple times until the needed log messages are found.

The returned VictoriaLogs query response can be post-processed with any combination of Unix commands,
which are usually used for log analysis - `grep`, `jq`, `awk`, `sort`, `uniq`, `wc`, etc.

For example, the following command uses `wc -l` Unix command for counting the number of log messages
with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
received from [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with `app="nginx"` field
during the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/query -d 'query={app="nginx"} AND _time:5m AND error' | wc -l
```

See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) about `_stream` filter,
[these docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) about `_time` filter
and [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter) about `AND` operator.

Alternatively, you can count the number of matching logs at VictoriaLogs side with [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe):

```sh
curl http://localhost:9428/select/logsql/query -d 'query={app="nginx"} AND _time:5m AND error | stats count() logs_with_error'
```

The following example shows how to sort query results by the [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) with traditional Unix tools:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' | jq -r '._time + " " + ._msg' | sort | less
```

This command uses `jq` for extracting [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field)
and [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) fields from the returned results,
and piping them to `sort` command.

Note that the `sort` command needs to read all the response stream before returning the sorted results. So the command above
can take non-trivial amounts of time if the `query` returns too many results. The solution is to narrow down the `query`
before sorting the results. See [these tips](https://docs.victoriametrics.com/victorialogs/logsql/#performance-tips)
on how to narrow down query results.

Alternatively, sorting of matching logs can be performed at VictoriaLogs side via [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error | sort by (_time)' | less
```

The following example calculates stats on the number of log messages received during the last 5 minutes
grouped by `log.level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with traditional Unix tools:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=_time:5m log.level:*' | jq -r '."log.level"' | sort | uniq -c 
```

The query selects all the log messages with non-empty `log.level` field via ["any value" filter](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter),
then pipes them to `jq` command, which extracts the `log.level` field value from the returned JSON stream, then the extracted `log.level` values
are sorted with `sort` command and, finally, they are passed to `uniq -c` command for calculating the needed stats.

Alternatively, all the stats calculations above can be performed at VictoriaLogs side via [`stats by(...)`](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-fields):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=_time:5m log.level:* | stats by (log.level) count() matching_logs'
```

See also:

- [Key concepts](https://docs.victoriametrics.com/victorialogs/keyconcepts/).
- [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/).
