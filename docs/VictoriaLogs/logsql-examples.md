---
sort: 100
weight: 100
title: LogsQL examples
menu:
  docs:
    parent: "victorialogs"
    weight: 100
---

# LogsQL examples

## How to select recently ingested logs?

[Run](https://docs.victoriametrics.com/victorialogs/querying/) the following query:

```logsql
_time:5m
```

It returns logs over the last 5 minutes by using [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter).
The logs are returned in arbitrary order because of performance reasons.
Add [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) to the query if you need sorting
the returned logs by some field (usually [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field)):

```logsql
_time:5m | sort by (_time)
```

If the number of returned logs is too big, it may be limited with the [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe).
For example, the following query returns 10 most recent logs, which were ingested during the last 5 minutes:

```logsql
_time:5m | sort by (_time desc) | limit 10
```

See also:

- [How to count the number of matching logs](#how-to-count-the-number-of-matching-logs)

## How to select logs with the given word in log message?

Just put the needed [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the query.
For example, the following query returns all the logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
in [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsql
error
```

If the number of returned logs is too big, then add [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the time range for the selected logs. For example, the following query returns logs with `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word)
over the last hour:

```logsql
error _time:1h
```

If the number of returned logs is still too big, then consider adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to the query. For example, the following query selects logs with `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word),
which do not contain `kubernetes` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word), over the last hour:

```logsql
error !kubernetes _time:1h
```

The logs are returned in arbitrary order because of performance reasons. Add [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
for sorting logs by the needed [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). For example, the following query
sorts the selected logs by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
error _time:1h | sort by (_time)
```

See also:

- [How to select logs with all the given words in log message?](#how-to-select-logs-with-all-the-given-words-in-log-message)
- [How to select logs with some of the given words in log message?](#how-to-select-logs-with-some-of-the-given-words-in-log-message)
- [How to skip logs with the given word in log message?](#how-to-skip-logs-with-the-given-word-in-log-message)
- [Filtering by phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter)
- [Filtering by prefix](https://docs.victoriametrics.com/victorialogs/logsql/#prefix-filter)
- [Filtering by regular expression](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter)
- [Filtering by substring](https://docs.victoriametrics.com/victorialogs/logsql/#substring-filter)


## How to skip logs with the given word in log message?

Use [`NOT` logical filter](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter). For example, the following query returns all the logs
without the `INFO` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsq
!INFO
```

If the number of returned logs is too big, then add [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the time range for the selected logs. For example, the following query returns matching logs over the last hour:

```logsql
!INFO _time:1h
```

If the number of returned logs is still too big, then consider adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to the query. For example, the following query selects logs without `INFO` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word),
which contain `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word), over the last hour:

```logsql
!INFO error _time:1h
```

The logs are returned in arbitrary order because of performance reasons. Add [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
for sorting logs by the needed [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). For example, the following query
sorts the selected logs by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
!INFO _time:1h | sort by (_time)
```

See also:

- [How to select logs with all the given words in log message?](#how-to-select-logs-with-all-the-given-words-in-log-message)
- [How to select logs with some of given words in log message?](#how-to-select-logs-with-some-of-the-given-words-in-log-message)
- [Filtering by phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter)
- [Filtering by prefix](https://docs.victoriametrics.com/victorialogs/logsql/#prefix-filter)
- [Filtering by regular expression](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter)
- [Filtering by substring](https://docs.victoriametrics.com/victorialogs/logsql/#substring-filter)


## How to select logs with all the given words in log message?

Just enumerate the needed [words](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the query, by deliming them with whitespace.
For example, the following query selects logs containing both `error` and `kubernetes` [words](https://docs.victoriametrics.com/victorialogs/logsql/#word)
in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsql
error kubernetes
```

This query uses [`AND` logical filter](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

If the number of returned logs is too big, then add [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the time range for the selected logs. For example, the following query returns matching logs over the last hour:

```logsql
error kubernetes _time:1h
```

If the number of returned logs is still too big, then consider adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to the query. For example, the following query selects logs with `error` and `kubernetes` [words](https://docs.victoriametrics.com/victorialogs/logsql/#word)
from [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) containing `container="my-app"` field, over the last hour:

```logsql
error kubernetes _stream:{container="my-app"} _time:1h
```

The logs are returned in arbitrary order because of performance reasons. Add [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
for sorting logs by the needed [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). For example, the following query
sorts the selected logs by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
error kubernetes _time:1h | sort by (_time)
```

See also:

- [How to select logs with some of given words in log message?](#how-to-select-logs-with-some-of-the-given-words-in-log-message)
- [How to skip logs with the given word in log message?](#how-to-skip-logs-with-the-given-word-in-log-message)
- [Filtering by phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter)
- [Filtering by prefix](https://docs.victoriametrics.com/victorialogs/logsql/#prefix-filter)
- [Filtering by regular expression](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter)
- [Filtering by substring](https://docs.victoriametrics.com/victorialogs/logsql/#substring-filter)


## How to select logs with some of the given words in log message?

Put the needed [words](https://docs.victoriametrics.com/victorialogs/logsql/#word) into `(...)`, by delimiting them with ` or `.
For example, the following query selects logs with `error`, `ERROR` or `Error` [words](https://docs.victoriametrics.com/victorialogs/logsql/#word)
in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsql
(error or ERROR or Error)
```

This query uses [`OR` logical filter](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

If the number of returned logs is too big, then add [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the time range for the selected logs. For example, the following query returns matching logs over the last hour:

```logsql
(error or ERROR or Error) _time:1h
```

If the number of returned logs is still too big, then consider adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to the query. For example, the following query selects logs without `error`, `ERROR` or `Error` [words](https://docs.victoriametrics.com/victorialogs/logsql/#word),
which do not contain `kubernetes` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word), over the last hour:

```logsql
(error or ERROR or Error) !kubernetes _time:1h
```

The logs are returned in arbitrary order because of performance reasons. Add [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
for sorting logs by the needed [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). For example, the following query
sorts the selected logs by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
(error or ERROR or Error) _time:1h | sort by (_time)
```

See also:

- [How to select logs with all the given words in log message?](#how-to-select-logs-with-all-the-given-words-in-log-message)
- [How to skip logs with the given word in log message?](#how-to-skip-logs-with-the-given-word-in-log-message)
- [Filtering by phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter)
- [Filtering by prefix](https://docs.victoriametrics.com/victorialogs/logsql/#prefix-filter)
- [Filtering by regular expression](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter)
- [Filtering by substring](https://docs.victoriametrics.com/victorialogs/logsql/#substring-filter)


## How to select logs from the given application instance?

Make sure the application is properly configured with [stream-level log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
Then just use [`_stream` filter](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) for selecting logs for the given application instance.
For example, if the application contains `job="app-42"` and `instance="host-123:5678"` [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields),
then the following query selects all the logs from this application:

```logsql
_stream:{job="app-42",instance="host-123:5678"}
```

If the number of returned logs is too big, it is recommended adding [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
to the query in order to reduce the number of matching logs. For example, the following query returns logs for the given application for the last day:

```logsql
_stream:{job="app-42",instance="host-123:5678"} _time:1d
```

If the number of returned logs is still too big, then consider adding more specific [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to the query. For example, the following query selects logs from the given [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields),
which contain `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
over the last day:

```logsql
_stream:{job="app-42",instance="host-123:5678"} error _time:1d
```

The logs are returned in arbitrary order because of performance reasons. Use [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe)
for sorting the returned logs by the needed fields. For example, the following query sorts the selected logs
by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
_stream:{job="app-42",instance="host-123:5678"} _time:1d | sort by (_time)
```

See also:

- [How to determine applications with the most logs?](#how-to-determine-applications-with-the-most-logs)
- [How to skip logs with the given word in log message?](#how-to-skip-logs-with-the-given-word-in-log-message)


## How to count the number of matching logs?

Use [`count()` stats function](https://docs.victoriametrics.com/victorialogs/logsql/#count-stats). For example, the following query returns
the number of results returned by `your_query_here`:

```logsql
your_query_here | count()
```

## How to determine applications with the most logs?

[Run](https://docs.victoriametrics.com/victorialogs/querying/) the following query:

```logsql
_time:5m | stats by (_stream) count() as logs | sort by (logs desc) | limit 10
```

This query returns top 10 application instances (aka [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields))
with the most logs over the last 5 minutes.

This query uses the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) features:

- [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) for selecting logs on the given time range (5 minutes in the query above).
- [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe) for calculating the number of logs.
  per each [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields). [`count` stats function](https://docs.victoriametrics.com/victorialogs/logsql/#count-stats)
  is used for calculating the needed stats.
- [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) for sorting the stats by `logs` field in descending order.
- [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe) for limiting the number of returned results to 10.

See also:

- [How to filter out data after stats calculation?](#how-to-filter-out-data-after-stats-calculation)
- [How to calculate the number of logs per the given interval?](#how-to-calculate-the-number-of-logs-per-the-given-interval)
- [How to select logs from the given application instance?](#how-to-select-logs-from-the-given-application-instance)


## How to parse JSON inside log message?

It is better from performance and resource usage PoV to avoid storing JSON inside [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
It is recommended storing individual JSON fields as log fields instead according to [VictoriaLogs data model](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

If you have to store JSON inside log message or inside any other [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then the stored JSON can be parsed during query time via [`unpack_json` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe).
For example, the following query unpacks JSON from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
across all the logs for the last 5 minutes:

```logsql
_time:5m | unpack_json
```

If you need to parse JSON array, then take a look at [`unroll` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unroll-pipe).


## How to extract some data from text log message?

Use [`extract`](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe) or [`extract_regexp`](https://docs.victoriametrics.com/victorialogs/logsql/#extract_regexp-pipe) pipe.
For example, the following query extracts `username` and `user_id` fields from text [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsql
_time:5m | extract "username=<username>, user_id=<user_id>,"
```

See also:

- [Replacing substrings in text fields](https://docs.victoriametrics.com/victorialogs/logsql/#replace-pipe)


## How to filter out data after stats calculation?

Use [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe). For example, the following query
returns only [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with more than 1000 logs
over the last 5 minutes:

```logsql
_time:5m | stats by (_stream) count() rows | filter rows:>1000
```

## How to calculate the number of logs per the given interval?

Use [`stats` by time bucket](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-time-buckets). For example, the following query
returns per-hour number of logs with the `error` [word](https://docs.victoriametrics.com/victorialogs/logsql/#word) for the last day:

```logsq
_time:1d error | stats by (_time:1h) count() rows | sort by (_time)
```

This query uses [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) in order to sort per-hour stats
by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).


## How to calculate the number of logs per every value of the given field?

Use [`stats` by field](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-fields). For example, the following query
calculates the number of logs per `level` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for logs over the last 5 minutes:

```logsql
_time:5m | stats by (level) count() rows
```

An alternative is to use [`field_values` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#field_values-pipe):

```logsql
_time:5m | field_values level
```

## How to get unique values for the given field?

Use [`uniq` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe). For example, the following query returns unique values for the `ip` field
over logs for the last 5 minutes:

```logsql
_time:5m | uniq by (ip)
```

## How to get unique sets of values for the given fields?

Use [`uniq` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe). For example, the following query returns unique sets for (`host`, `path`) fields
over logs for the last 5 minutes:

```logsql
_time:5m | uniq by (host, path)
```

## How to return last N logs for the given query?

Use [`sort` pipe with limit](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe). For example, the following query returns the last 10 logs with the `error`
[word](https://docs.victoriametrics.com/victorialogs/logsql/#word) in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
over the logs for the last 5 minutes:

```logsql
_time:5m error | sort by (_time desc) limit 10
```

It sorts the matching logs by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) in descending order and then selects
the first 10 logs with the highest values for the `_time` field.

If the query is sent to [`/select/logsql/query` HTTP API](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs), then `limit=N` query arg
can be passed to it in order to return up to `N` latest log entries. For example, the following command returns up to 10 latest log entries with the `error` word:

```sh
curl http://localhost:9428/select/logsql/query -d 'query=error' -d 'limit=10'
```
