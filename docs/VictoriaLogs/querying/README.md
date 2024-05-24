---
sort: 4
title: Querying
weight: 4
menu:
  docs:
    identifier: victorialogs-querying
    parent: "victorialogs"
    weight: 4
aliases:
  - /VictoriaLogs/querying/
  - /VictoriaLogs/querying/index.html
---

# Querying

[VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/) can be queried with [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/)
via the following ways:

- [Web UI](#web-ui) - a web-based UI for querying logs
- [HTTP API](#http-api)
- [Command-line interface](#command-line)

## HTTP API

VictoriaLogs provides the following HTTP endpoints:

- [`/select/logsql/query`](#querying-logs) for querying logs
- [`/select/logsql/hits`](#querying-hits-stats) for querying log hits stats over the given time range
- [`/select/logsql/streams`](#querying-streams) for querying [log streams](#https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
- [`/select/logsql/stream_label_names`](#querying-stream-label-names) for querying [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) label names
- [`/select/logsql/stream_label_values`](#querying-stream-label-values) for querying [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) label values
- [`/select/logsql/field_names`](#querying-field-names) for querying [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names.
- [`/select/logsql/field_values`](#querying-field-values) for querying [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) values.

### Querying logs

Logs stored in VictoriaLogs can be queried at the `/select/logsql/query` HTTP endpoint.
The [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query must be passed via `query` argument.
For example, the following query returns all the log entries with the `error` word:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

The response by default contains all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
See [how to query specific fields](https://docs.victoriametrics.com/victorialogs/logsql/#querying-specific-fields).

The `query` argument can be passed either in the request url itself (aka HTTP GET request) or via request body
with the `x-www-form-urlencoded` encoding (aka HTTP POST request). The HTTP POST is useful for sending long queries
when they do not fit the maximum url length of the used clients and proxies.

See [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/) for details on what can be passed to the `query` arg.
The `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools.

By default the `/select/logsql/query` returns all the log entries matching the given `query`. The response size can be limited in the following ways:

- By closing the response stream at any time. In this case VictoriaLogs stops query execution and frees all the resources occupied by the request.
- By specifying the maximum number of log entries, which can be returned in the response via `limit` query arg. For example, the following request returns
  up to 10 matching log entries:
  ```sh
  curl http://localhost:9428/select/logsql/query -d 'query=error' -d 'limit=10'
  ```
- By adding [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe) to the query. For example:
  ```sh
  curl http://localhost:9428/select/logsql/query -d 'query=error | limit 10'
  ```
- By adding [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter). The time range for the query can be specified via optional
  `start` and `end` query ars formatted according to [these docs](https://docs.victoriametrics.com/single-server-victoriametrics/#timestamp-formats).
- By adding other [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) to the query.

The `/select/logsql/query` endpoint returns [a stream of JSON lines](https://jsonlines.org/),
where each line contains JSON-encoded log entry in the form `{field1="value1",...,fieldN="valueN"}`.
Example response:

```
{"_msg":"error: disconnect from 19.54.37.22: Auth fail [preauth]","_stream":"{}","_time":"2023-01-01T13:32:13Z"}
{"_msg":"some other error","_stream":"{}","_time":"2023-01-01T13:32:15Z"}
```

The matching lines are sent to the response stream as soon as they are found in VictoriaLogs storage.
This means that the returned response may contain billions of lines for queries matching too many log entries.
The response can be interrupted at any time by closing the connection to VictoriaLogs server.
This allows post-processing the returned lines at the client side with the usual Unix commands such as `grep`, `jq`, `less`, `head`, etc.
See [these docs](#command-line) for more details.

The returned lines aren't sorted, since sorting disables the ability to send matching log entries to response stream as soon as they are found.
Query results can be sorted either at VictoriaLogs side according [to these docs](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
or at client side with the usual `sort` command according to [these docs](#command-line).

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/#multitenancy) is queried.
If you need querying other tenant, then specify the needed tenant via http request headers. For example, the following query searches
for log messages at `(AccountID=12, ProjectID=34)` tenant:

```sh
curl http://localhost:9428/select/logsql/query -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=error'
```

The number of requests to `/select/logsql/query` can be [monitored](https://docs.victoriametrics.com/VictoriaLogs/#monitoring)
with `vl_http_requests_total{path="/select/logsql/query"}` metric.

- [Querying hits stats](#querying-hits-stats)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying hits stats

VictoriaMetrics provides `/select/logsql/hits?query=<query>&start=<start>&end=<end>&step=<step>` HTTP endpoint, which returns the number
of matching log entries for the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]`
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
      ]
    }
  ]
}
```

Additionally, the `offset=<offset>` arg can be passed to `/select/logsql/hits` in order to group buckets according to the given timezone offset.
The `<offset>` can contain values in [the format specified here](https://docs.victoriametrics.com/victorialogs/logsql/#duration-values).
For example, the following command returns per-day number of logs with `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
over the last week in New York time zone (`-4h`):

```logsql
curl http://localhost:9428/select/logsql/hits -d 'query=error' -d 'start=1w' -d 'step=1d' -d 'offset=-4h'
```

Additionally, any number of `field=<field_name>` args can be passed to `/select/logsql/hits` for grouping hits buckets by the mentioned `<field_name>` fields.
For example, the following query groups hits by `level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) additionally to the provided `step`:

```logsql
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
      ]
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
      ]
    }
  ]
}
```

See also:

- [Querying logs](#querying-logs)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying streams

VictoriaLogs provides `/select/logsql/streams?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
from results of the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every `stream`.

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
The endpoint returns arbitrary subset of values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of streams.

See also:

- [Querying logs](#querying-logs)
- [Querying hits stats](#querying-hits-stats)
- [HTTP API](#http-api)

### Querying stream label names

VictoriaLogs provides `/select/logsql/stream_label_names?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns
[log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) label names from results
of the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every label name.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns stream label names across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/stream_label_names -d 'query=error' -d 'start=5m'
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

See also:

- [Querying stream label names](#querying-stream-label-names)
- [Querying field values](#querying-field-values)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying stream label values

VictoriaLogs provides `/select/logsql/stream_label_values?query=<query>&start=<start>&<end>&label=<labelName>` HTTP endpoint,
which returns [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) label values for the label with the given `<labelName>` name
from results of the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
The response also contains the number of log results per every label value.

The `<start>` and `<end>` args can contain values in [any supported format](https://docs.victoriametrics.com/#timestamp-formats).
If `<start>` is missing, then it equals to the minimum timestamp across logs stored in VictoriaLogs.
If `<end>` is missing, then it equals to the maximum timestamp across logs stored in VictoriaLogs.

For example, the following command returns values for the stream label `host` across logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
for the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/stream_label_values -d 'query=error' -d 'start=5m' -d 'label=host'
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

The `/select/logsql/stream_label_names` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned values to `N`.
The endpoint returns arbitrary subset of values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of field values.

See also:

- [Querying stream label values](#querying-stream-label-values)
- [Querying field names](#querying-field-names)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying field names

VictoriaLogs provides `/select/logsql/field_names?query=<query>&start=<start>&end=<end>` HTTP endpoint, which returns field names
from results of the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
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

See also:

- [Querying stream label names](#querying-stream-label-names)
- [Querying field values](#querying-field-values)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)

### Querying field values

VictoriaLogs provides `/select/logsql/field_values?query=<query>&field=<fieldName>&start=<start>&end=<end>` HTTP endpoint, which returns
unique values for the given `<fieldName>` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
from results of the given `<query>` [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) on the given `[<start> ... <end>]` time range.
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

The `/select/logsql/field_names` endpoint supports optional `limit=N` query arg, which allows limiting the number of returned values to `N`.
The endpoint returns arbitrary subset of values if their number exceeds `N`, so `limit=N` cannot be used for pagination over big number of field values.
When the `limit` is reached, `hits` are zeroed, since they cannot be calculated reliably.

See also:

- [Querying stream label values](#querying-stream-label-values)
- [Querying field names](#querying-field-names)
- [Querying streams](#querying-streams)
- [HTTP API](#http-api)


## Web UI

VictoriaLogs provides a simple Web UI for logs [querying](https://docs.victoriametrics.com/victorialogs/logsql/) and exploration
at `http://localhost:9428/select/vmui`. The UI allows exploring query results:

<img src="vmui.webp" />

There are three modes of displaying query results:

- `Group` - results are displayed as a table with rows grouped by stream and fields for filtering.
- `Table` - displays query results as a table.
- `JSON` - displays raw JSON response from [HTTP API](#http-api).

This is the first version that has minimal functionality. It comes with the following limitations:

- The number of query results is always limited to 1000 lines. Iteratively add
  more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) to the query
  in order to get full response with less than 1000 lines.
- Queries are always executed against [tenant](https://docs.victoriametrics.com/VictoriaLogs/#multitenancy) `0`.

These limitations will be removed in future versions.

To get around the current limitations, you can use an alternative - the [command line interface](#command-line).

## Command-line

VictoriaLogs integrates well with `curl` and other command-line tools during querying because of the following features:

- VictoriaLogs sends the matching log entries to the response stream as soon as they are found.
  This allows forwarding the response stream to arbitrary [Unix pipes](https://en.wikipedia.org/wiki/Pipeline_(Unix)).
- VictoriaLogs automatically adjusts query execution speed to the speed of the client, which reads the response stream.
  For example, if the response stream is piped to `less` command, then the query is suspended
  until the `less` command reads the next block from the response stream.
- VictoriaLogs automatically cancels query execution when the client closes the response stream.
  For example, if the query response is piped to `head` command, then VictoriaLogs stops executing the query
  when the `head` command closes the response stream.

These features allow executing queries at command-line interface, which potentially select billions of rows,
without the risk of high resource usage (CPU, RAM, disk IO) at VictoriaLogs server.

For example, the following query can return very big number of matching log entries (e.g. billions) if VictoriaLogs contains
many log messages with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

If the command returns "never-ending" response, then just press `ctrl+C` at any time in order to cancel the query.
VictoriaLogs notices that the response stream is closed, so it cancels the query and instantly stops consuming CPU, RAM and disk IO for this query.

Then just use `head` command for investigating the returned log messages and narrowing down the query:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' | head -10
```

The `head -10` command reads only the first 10 log messages from the response and then closes the response stream.
This automatically cancels the query at VictoriaLogs side, so it stops consuming CPU, RAM and disk IO resources.

Sometimes it may be more convenient to use `less` command instead of `head` during the investigation of the returned response:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' | less
```

The `less` command reads the response stream on demand, when the user scrolls down the output.
VictoriaLogs suspends query execution when `less` stops reading the response stream.
It doesn't consume CPU and disk IO resources during this time. It resumes query execution
when the `less` continues reading the response stream.

Suppose that the initial investigation of the returned query results helped determining that the needed log messages contain
`cannot open file` [phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter).
Then the query can be narrowed down to `error AND "cannot open file"`
(see [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter) about `AND` operator).
Then run the updated command in order to continue the investigation:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error AND "cannot open file"' | head
```

Note that the `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools.

The `pipe the query to "head" or "less" -> investigate the results -> refine the query` iteration
can be repeated multiple times until the needed log messages are found.

The returned VictoriaLogs query response can be post-processed with any combination of Unix commands,
which are usually used for log analysis - `grep`, `jq`, `awk`, `sort`, `uniq`, `wc`, etc.

For example, the following command uses `wc -l` Unix command for counting the number of log messages
with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
received from [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with `app="nginx"` field
during the last 5 minutes:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=_stream:{app="nginx"} AND _time:5m AND error' | wc -l
```

See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) about `_stream` filter,
[these docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) about `_time` filter
and [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter) about `AND` operator.

The following example shows how to sort query results by the [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

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

The following example calculates stats on the number of log messages received during the last 5 minutes
grouped by `log.level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=_time:5m log.level:*' | jq -r '."log.level"' | sort | uniq -c 
```

The query selects all the log messages with non-empty `log.level` field via ["any value" filter](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter),
then pipes them to `jq` command, which extracts the `log.level` field value from the returned JSON stream, then the extracted `log.level` values
are sorted with `sort` command and, finally, they are passed to `uniq -c` command for calculating the needed stats.

See also:

- [Key concepts](https://docs.victoriametrics.com/victorialogs/keyconcepts/).
- [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/).
