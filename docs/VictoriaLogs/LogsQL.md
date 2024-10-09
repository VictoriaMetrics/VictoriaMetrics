---
weight: 5
title: LogsQL
menu:
  docs:
    parent: "victorialogs"
    weight: 5
aliases:
- /VictoriaLogs/LogsQL.html
---
LogsQL is a simple yet powerful query language for [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/).
See [examples](https://docs.victoriametrics.com/victorialogs/logsql-examples/) and [tutorial](#logsql-tutorial)
in order to feel the language.

LogsQL provides the following features:

- Full-text search across [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
  See [word filter](#word-filter), [phrase filter](#phrase-filter) and [prefix filter](#prefix-filter).
- Ability to combine filters into arbitrary complex [logical filters](#logical-filter).
- Ability to extract structured fields from unstructured logs at query time. See [these docs](#transformations).
- Ability to calculate various stats over the selected log entries. See [these docs](#stats-pipe).

## LogsQL tutorial

If you aren't familiar with VictoriaLogs, then start with [key concepts docs](https://docs.victoriametrics.com/victorialogs/keyconcepts/).

Then follow these docs:
- [How to run VictoriaLogs](https://docs.victoriametrics.com/victorialogs/quickstart/).
- [how to ingest data into VictoriaLogs](https://docs.victoriametrics.com/victorialogs/data-ingestion/).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

The simplest LogsQL query is just a [word](#word), which must be found in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
For example, the following query finds all the logs with `error` word:

```logsql
error
```

It is recommended to use [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/vlogscli/) for querying VictoriaLogs.

If the queried [word](#word) clashes with LogsQL keywords, then just wrap it into quotes.
For example, the following query finds all the log messages with `and` [word](#word):

```logsql
"and"
```

It is OK to wrap any word into quotes. For example:

```logsql
"error"
```

Moreover, it is possible to wrap phrases containing multiple words in quotes. For example, the following query
finds log messages with the `error: cannot find file` phrase:

```logsql
"error: cannot find file"
```

Queries above match logs with any [timestamp](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field),
e.g. they may return logs from the previous year alongside recently ingested logs.

Usually logs from the previous year aren't so interesting comparing to the recently ingested logs.
So it is recommended adding [time filter](#time-filter) to the query.
For example, the following query returns logs with the `error` [word](#word),
which were ingested into VictoriaLogs during the last 5 minutes:

```logsql
error AND _time:5m
```

This query consists of two [filters](#filters) joined with `AND` [operator](#logical-filter):

- The filter on the `error` [word](#word).
- The filter on the [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).

The `AND` operator means that the [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must match both filters in order to be selected.

Typical LogsQL query consists of multiple [filters](#filters) joined with `AND` operator. It may be tiresome typing and then reading all these `AND` words.
So LogsQL allows omitting `AND` words. For example, the following query is equivalent to the query above:

```logsql
_time:5m error
```

The query returns logs in arbitrary order because sorting of big amounts of logs may require non-trivial amounts of CPU and RAM.
The number of logs with `error` word over the last 5 minutes isn't usually too big (e.g. less than a few millions), so it is OK to sort them with [`sort` pipe](#sort-pipe).
The following query sorts the selected logs by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) field:

```logsql
_time:5m error | sort by (_time)
```

It is unlikely you are going to investigate more than a few hundreds of logs returned by the query above. So you can limit the number of returned logs
with [`limit` pipe](#limit-pipe). The following query returns the last 10 logs with the `error` word over the last 5 minutes:

```logsql
_time:5m error | sort by (_time) desc | limit 10
```

By default VictoriaLogs returns all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
If you need only the given set of fields, then add [`fields` pipe](#fields-pipe) to the end of the query. For example, the following query returns only
[`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field), [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
and [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) fields:

```logsql
error _time:5m | fields _time, _stream, _msg
```

Suppose the query above selects too many rows because some buggy app pushes invalid error logs to VictoriaLogs. Suppose the app adds `buggy_app` [word](#word) to every log line.
Then the following query removes all the logs from the buggy app, allowing us paying attention to the real errors:

```logsql
_time:5m error NOT buggy_app
```

This query uses `NOT` [operator](#logical-filter) for removing log lines from the buggy app. The `NOT` operator is used frequently, so it can be substituted with `-` or `!` char
(the `!` must be used instead of `-` in front of [`=`](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter)
and [`~`](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter) filters like `!=` and `!~`).
The following query is equivalent to the previous one:

```logsql
_time:5m error -buggy_app
```

Suppose another buggy app starts pushing invalid error logs to VictoriaLogs - it adds `foobar` [word](#word) to every emitted log line.
No problems - just add `-foobar` to the query in order to remove these buggy logs:

```logsql
_time:5m error -buggy_app -foobar
```

This query can be rewritten to more clear query with the `OR` [operator](#logical-filter) inside parentheses:

```logsql
_time:5m error -(buggy_app OR foobar)
```

The parentheses are **required** here, since otherwise the query won't return the expected results.
The query `error -buggy_app OR foobar` is interpreted as `(error AND NOT buggy_app) OR foobar` according to [priorities for AND, OR and NOT operator](#logical-filters).
This query returns logs with `foobar` [word](#word), even if do not contain `error` word or contain `buggy_app` word.
So it is recommended wrapping the needed query parts into explicit parentheses if you are unsure in priority rules.
As an additional bonus, explicit parentheses make queries easier to read and maintain.

Queries above assume that the `error` [word](#word) is stored in the [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
If this word is stored in other [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) such as `log.level`, then add `log.level:` prefix
in front of the `error` word:

```logsql
_time:5m log.level:error -(buggy_app OR foobar)
```

The field name can be wrapped into quotes if it contains special chars or keywords, which may clash with LogsQL syntax.
Any [word](#word) also can be wrapped into quotes. So the following query is equivalent to the previous one:

```logsql
"_time":"5m" "log.level":"error" -("buggy_app" OR "foobar")
```

What if the application identifier - such as `buggy_app` and `foobar` - is stored in the `app` field? Correct - just add `app:` prefix in front of `buggy_app` and `foobar`:

```logsql
_time:5m log.level:error -(app:buggy_app OR app:foobar)
```

The query can be simplified by moving the `app:` prefix outside the parentheses:

```logsql
_time:5m log.level:error -app:(buggy_app OR foobar)
```

The `app` field uniquely identifies the application instance if a single instance runs per each unique `app`.
In this case it is recommended associating the `app` field with [log stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
during [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/). This usually improves both compression rate
and query performance when querying the needed streams via [`_stream` filter](#stream-filter).
If the `app` field is associated with the log stream, then the query above can be rewritten to more performant one:

```logsql
_time:5m log.level:error {app!~"buggy_app|foobar"}
```

This query skips scanning for [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) from `buggy_app` and `foobar` apps.
It inpsects only `log.level` and [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) labels.
This significantly reduces disk read IO and CPU time needed for performing the query.

LogsQL also provides [functions for statistics calculation](#stats-pipe) over the selected logs. For example, the following query returns the number of logs
with the `error` word for the last 5 minutes:

```logsql
_time:5m error | stats count() logs_with_error
```

Finally, it is recommended reading [performance tips](#performance-tips).

Now you are familiar with LogsQL basics. See [LogsQL examples](https://docs.victoriametrics.com/victorialogs/logsql-examples/) and [query syntax](#query-syntax)
if you want to continue learning LogsQL.

### Key concepts

#### Word

LogsQL splits all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into words
delimited by non-word chars such as whitespace, parens, punctuation chars, etc. For example, the `foo: (bar,"тест")!` string
is split into `foo`, `bar` and `тест` words. Words can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8) chars.
These words are taken into account by full-text search filters such as
[word filter](#word-filter), [phrase filter](#phrase-filter) and [prefix filter](#prefix-filter).

#### Query syntax

LogsQL query must contain at least a single [filter](#filters) for selecting the matching logs.
For example, the following query selects all the logs for the last 5 minutes by using [`_time` filter](#time-filter):

```logsql
_time:5m
```

Tip: try [`*` filter](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter), which selects all the logs stored in VictoriaLogs.
Do not worry - this doesn't crash VictoriaLogs, even if the query selects trillions of logs. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)
if you are curious why.

Additionally to filters, LogQL query may contain arbitrary mix of optional actions for processing the selected logs. These actions are delimited by `|` and are known as [`pipes`](#pipes).
For example, the following query uses [`stats` pipe](#stats-pipe) for returning the number of [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with the `error` [word](#word) for the last 5 minutes:

```logsql
_time:5m error | stats count() errors
```

See [the list of supported pipes in LogsQL](#pipes).

## Filters

LogsQL supports various filters for searching for log messages (see below).
They can be combined into arbitrary complex queries via [logical filters](#logical-filter).

Filters are applied to [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) by default.
If the filter must be applied to other [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then its' name followed by the colon must be put in front of the filter. For example, if `error` [word filter](#word-filter) must be applied
to the `log.level` field, then use `log.level:error` query.

Field names and filter args can be put into quotes if they contain special chars, which may clash with LogsQL syntax. LogsQL supports quoting via double quotes `"`,
single quotes `'` and backticks:

```logsql
"some 'field':123":i('some("value")') AND `other"value'`
```

If doubt, it is recommended quoting field names and filter args.

The list of LogsQL filters:

- [Time filter](#time-filter) - matches logs with [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) in the given time range
- [Day range filter](#day-range-filter) - matches logs with [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) in the given per-day time range
- [Week range filter](#week-range-filter) - matches logs with [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) in the given per-week day range
- [Stream filter](#stream-filter) - matches logs, which belong to the given [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
- [Word filter](#word-filter) - matches logs with the given [word](#word)
- [Phrase filter](#phrase-filter) - matches logs with the given phrase
- [Prefix filter](#prefix-filter) - matches logs with the given word prefix or phrase prefix
- [Substring filter](#substring-filter) - matches logs with the given substring
- [Range comparison filter](#range-comparison-filter) - matches logs with field values in the provided range
- [Empty value filter](#empty-value-filter) - matches logs without the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
- [Any value filter](#any-value-filter) - matches logs with the given non-empty [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
- [Exact filter](#exact-filter) - matches logs with the exact value
- [Exact prefix filter](#exact-prefix-filter) - matches logs starting with the given prefix
- [Multi-exact filter](#multi-exact-filter) - matches logs with one of the specified exact values
- [Case-insensitive filter](#case-insensitive-filter) - matches logs with the given case-insensitive word, phrase or prefix
- [Sequence filter](#sequence-filter) - matches logs with the given sequence of words or phrases
- [Regexp filter](#regexp-filter) - matches logs for the given regexp
- [Range filter](#range-filter) - matches logs with numeric [field values](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in the given range
- [IPv4 range filter](#ipv4-range-filter) - matches logs with ip address [field values](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in the given range
- [String range filter](#string-range-filter) - matches logs with [field values](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in the given string range
- [Length range filter](#length-range-filter) - matches logs with [field values](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) of the given length range
- [Logical filter](#logical-filter) - allows combining other filters


### Time filter

VictoriaLogs scans all the logs per each query if it doesn't contain the filter on [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).
It uses various optimizations in order to accelerate full scan queries without the `_time` filter,
but such queries can be slow if the storage contains large number of logs over long time range. The easiest way to optimize queries
is to narrow down the search with the filter on [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).

For example, the following query returns [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
ingested into VictoriaLogs during the last hour, which contain the `error` [word](#word):

```logsql
_time:1h AND error
```

The following formats are supported for `_time` filter:

- `_time:duration` matches logs with timestamps on the time range `(now-duration, now]`, where `duration` can have [these values](#duration-values). Examples:
  - `_time:5m` - returns logs for the last 5 minutes
  - `_time:2.5d15m42.345s` - returns logs for the last 2.5 days, 15 minutes and 42.345 seconds
  - `_time:1y` - returns logs for the last year
- `_time:YYYY-MM-DDZ` - matches all the logs for the particular day by UTC. For example, `_time:2023-04-25Z` matches logs on April 25, 2023 by UTC.
- `_time:YYYY-MMZ` - matches all the logs for the particular month by UTC. For example, `_time:2023-02Z` matches logs on February, 2023 by UTC.
- `_time:YYYYZ` - matches all the logs for the particular year by UTC. For example, `_time:2023Z` matches logs on 2023 by UTC.
- `_time:YYYY-MM-DDTHHZ` - matches all the logs for the particular hour by UTC. For example, `_time:2023-04-25T22Z` matches logs on April 25, 2023 at 22 hour by UTC.
- `_time:YYYY-MM-DDTHH:MMZ` - matches all the logs for the particular minute by UTC. For example, `_time:2023-04-25T22:45Z` matches logs on April 25, 2023 at 22:45 by UTC.
- `_time:YYYY-MM-DDTHH:MM:SSZ` - matches all the logs for the particular second by UTC. For example, `_time:2023-04-25T22:45:59Z` matches logs on April 25, 2023 at 22:45:59 by UTC.
- `_time:[min_time, max_time]` - matches logs on the time range `[min_time, max_time]`, including both `min_time` and `max_time`.
    The `min_time` and `max_time` can contain any format specified [here](https://docs.victoriametrics.com/#timestamp-formats).
    For example, `_time:[2023-04-01Z, 2023-04-30Z]` matches logs for the whole April, 2023 by UTC, e.g. it is equivalent to `_time:2023-04Z`.
- `_time:[min_time, max_time)` - matches logs on the time range `[min_time, max_time)`, not including `max_time`.
    The `min_time` and `max_time` can contain any format specified [here](https://docs.victoriametrics.com/#timestamp-formats).
    For example, `_time:[2023-02-01Z, 2023-03-01Z)` matches logs for the whole February, 2023 by UTC, e.g. it is equivalent to `_time:2023-02Z`.

It is possible to specify time zone offset for all the absolute time formats by appending `+hh:mm` or `-hh:mm` suffix.
For example, `_time:2023-04-25+05:30` matches all the logs on April 25, 2023 by India time zone,
while `_time:2023-02-07:00` matches all the logs on February, 2023 by California time zone.

If the timezone offset information is missing, then the local time zone of the host where VictoriaLogs runs is used.
For example, `_time:2023-10-20` matches all the logs for `2023-10-20` day according to the local time zone of the host where VictoriaLogs runs.

It is possible to specify generic offset for the selected time range by appending `offset` after the `_time` filter. Examples:

- `_time:5m offset 1h` matches logs on the time range `(now-1h5m, now-1h]`.
- `_time:2023-07Z offset 5h30m` matches logs on July, 2023 by UTC with offset 5h30m.
- `_time:[2023-02-01Z, 2023-03-01Z) offset 1w` matches logs the week before the time range `[2023-02-01Z, 2023-03-01Z)` by UTC.

Performance tips:

- It is recommended specifying the smallest possible time range during the search, since it reduces the amounts of log entries, which need to be scanned during the query.
  For example, `_time:1h` is usually faster than `_time:5h`.

- While LogsQL supports arbitrary number of `_time:...` filters at any level of [logical filters](#logical-filter),
  it is recommended specifying a single `_time` filter at the top level of the query.

- See [other performance tips](#performance-tips).

See also:

- [Day range filter](#day-range-filter)
- [Week range filter](#week-range-filter)
- [Stream filter](#stream-filter)
- [Word filter](#word-filter)

### Day range filter

`_time:day_range[start, end]` filter allows returning logs on the particular `start ... end` time per every day, where `start` and `end` have the format `hh:mm`.
For example, the following query matches logs between `08:00` and `18:00` UTC every day:

```logsql
_time:day_range[08:00, 18:00)
```

This query includes `08:00`, while `18:00` is excluded, e.g. the last matching time is `17:59:59.999999999`.
Replace `[` with `(` in order to exclude the starting time. Replace `)` with `]` in order to include the ending time.
For example, the following query matches logs between `08:00` and `18:00`, excluding `08:00:00.000000000` and including `18:00`:

```logsql
_time:day_range(08:00, 18:00]
```

If the time range must be applied to other than UTC time zone, then add `offset <duration>`, where `<duration>` can have [any supported duration value](#duration-values).
For example, the following query selects logs between `08:00` and `18:00` at `+0200` time zone:

```logsql
_time:day_range[08:00, 18:00) offset 2h
```

Performance tip: it is recommended specifying regular [time filter](#time-filter) additionally to `day_range` filter. For example, the following query selects logs
between `08:00` and `20:00` every day for the last week:

```logsql
_time:1w _time:day_range[08:00, 18:00)
```

See also:

- [Week range filter](#week-range-filter)
- [Time filter](#time-filter)

### Week range filter

`_time:week_range[start, end]` filter allows returning logs on the particular `start ... end` days per every day, where `start` and `end` can have the following values:

- `Sun` or `Sunday`
- `Mon` or `Monday`
- `Tue` or `Tuesday`
- `Wed` or `Wednesday`
- `Thu` or `Thusday`
- `Fri` or `Friday`
- `Sat` or `Saturday`

For example, the following query matches logs between Monday and Friday UTC every day:

```logsql
_time:week_range[Mon, Fri]
```

This query includes Monday and Friday.
Replace `[` with `(` in order to exclude the starting day. Replace `]` with `)` in order to exclude the ending day.
For example, the following query matches logs between Sunday and Saturday, excluding Sunday and Saturday (e.g. it is equivalent to the previous query):

```logsql
_time:week_range(Sun, Sat)
```

If the day range must be applied to other than UTC time zone, then add `offset <duration>`, where `<duration>` can have [any supported duration value](#duration-values).
For example, the following query selects logs between Monday and Friday at `+0200` time zone:

```logsql
_time:week_range[Mon, Fri] offset 2h
```

The `week_range` filter can be combined with [`day_range` filter](#day-range-filter) using [logical filters](#logical-filter). For example, the following query
selects logs between `08:00` and `18:00` every day of the week excluding Sunday and Saturday:

```logsql
_time:week_range[Mon, Fri] _time:day_range[08:00, 18:00)
```

Performance tip: it is recommended specifying regular [time filter](#time-filter) additionally to `week_range` filter. For example, the following query selects logs
between Monday and Friday per every week for the last 4 weeks:

```logsql
_time:4w _time:week_range[Mon, Fri]
```

See also:

- [Day range filter](#day-range-filter)
- [Time filter](#time-filter)

### Stream filter

VictoriaLogs provides an optimized way to select logs, which belong to particular [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done via `{...}` filter, which may contain arbitrary
[Prometheus-compatible label selector](https://docs.victoriametrics.com/keyconcepts/#filtering)
over fields associated with [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
For example, the following query selects [log entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with `app` field equal to `nginx`:

```logsql
{app="nginx"}
```

This query is equivalent to the following [`exact` filter](#exact-filter) query, but the upper query usually works much faster:

```logsql
app:="nginx"
```

It is allowed to add `_stream:` prefix in front of `{...}` filter. The following filter is equivalent to `{app="nginx"}`:

```logsql
_stream:{app="nginx"}
```

Performance tips:

- It is recommended using the most specific `{...}` filter matching the smallest number of log streams,
  which needs to be scanned by the rest of filters in the query.

- While LogsQL supports arbitrary number of `{...}` filters at any level of [logical filters](#logical-filter),
  it is recommended specifying a single `{...}` filter at the top level of the query.

- See [other performance tips](#performance-tips).

See also:

- [`_stream_id` filter](#_stream_id-filter)
- [Time filter](#time-filter)
- [Exact filter](#exact-filter)

### _stream_id filter

Every [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) in VictoriaMetrics is uniquely identified by `_stream_id` field.
The `_stream_id:...` filter allows quickly selecting all the logs belonging to the particular stream.

For example, the following query selects all the logs, which belong to the [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
with `_stream_id` equal to `0000007b000001c850d9950ea6196b1a4812081265faa1c7`:

```logsql
_stream_id:0000007b000001c850d9950ea6196b1a4812081265faa1c7
```

If the log stream contains too many logs, then it is good idea limiting the number of returned logs with [time filter](#time-filter). For example, the following
query selects logs for the given stream for the last hour:

```logsql
_time:1h _stream_id:0000007b000001c850d9950ea6196b1a4812081265faa1c7
```

The `_stream_id` filter supports specifying multiple `_stream_id` values via `_stream_id:in(...)` syntax. For example:

```logsql
_stream_id:in(0000007b000001c850d9950ea6196b1a4812081265faa1c7, 1230007b456701c850d9950ea6196b1a4812081265fff2a9)
```

It is also possible specifying subquery inside `in(...)`, which selects the needed `_stream_id` values. For example, the following query returns
logs for [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) containing `error` [word](#word)
in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) during the last 5 minutes:

```logsql
_stream_id:in(_time:5m error | fields _stream_id)
```

See also:

- [stream filter](#stream-filter)


### Word filter

The simplest LogsQL query consists of a single [word](#word) to search in log messages. For example, the following query matches
[log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) with `error` [word](#word) inside them:

```logsql
error
```

This query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `error`
- `an error happened`
- `error: cannot open file`

This query doesn't match the following log messages:

- `ERROR`, since the filter is case-sensitive by default. Use `i(error)` for this case. See [these docs](#case-insensitive-filter) for details.
- `multiple errors occurred`, since the `errors` word doesn't match `error` word. Use `error*` for this case. See [these docs](#prefix-filter) for details.

By default the given [word](#word) is searched in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the word and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries containing the `error` [word](#word) in the `log.level` field:

```logsql
log.level:error
```

Both the field name and the word in the query can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
სფერო:τιμή
```

Both the field name and the word in the query can be put inside quotes if they contain special chars, which may clash with the query syntax.
For example, the following query searches for the ip `1.2.3.45` in the field `ip:remote`:

```logsql
"ip:remote":"1.2.3.45"
```

See also:

- [Phrase filter](#phrase-filter)
- [Exact filter](#exact-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Phrase filter

Is you need to search for log messages with the specific phrase inside them, then just wrap the phrase in quotes.
The phrase can contain any chars, including whitespace, punctuation, parens, etc. They are taken into account during the search.
For example, the following query matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with `ssh: login fail` phrase inside them:

```logsql
"ssh: login fail"
```

This query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `ERROR: ssh: login fail for user "foobar"`
- `ssh: login fail!`

This query doesn't match the following log messages:

- `ssh login fail`, since the message misses `:` char just after the `ssh`.
  Use `seq("ssh", "login", "fail")` query if log messages with the sequence of these words must be found. See [these docs](#sequence-filter) for details.
- `login fail: ssh error`, since the message doesn't contain the full phrase requested in the query. If you need matching a message
  with all the [words](#word) listed in the query, then use `ssh AND login AND fail` query. See [these docs](#logical-filter) for details.
- `ssh: login failed`, since the message ends with `failed` [word](#word) instead of `fail` word. Use `"ssh: login fail"*` query for this case.
  See [these docs](#prefix-filter) for details.
- `SSH: login fail`, since the `SSH` word is in capital letters. Use `i("ssh: login fail")` for case-insensitive search.
  See [these docs](#case-insensitive-filter) for details.

If the phrase contains double quotes, then either put `\` in front of double quotes or put the phrase inside single quotes. For example, the following filter searches
logs with `"foo":"bar"` phrase:

```logsql
'"foo":"bar"'
```

By default the given phrase is searched in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the phrase and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries containing the `cannot open file` phrase in the `event.original` field:

```logsql
event.original:"cannot open file"
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
შეტყობინება:"Το αρχείο δεν μπορεί να ανοίξει"
```

The field name can be put inside quotes if it contains special chars, which may clash with the query syntax.
For example, the following query searches for the `cannot open file` phrase in the field `some:message`:

```logsql
"some:message":"cannot open file"
```

See also:

- [Exact filter](#exact-filter)
- [Word filter](#word-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Prefix filter

If you need to search for log messages with [words](#word) / phrases containing some prefix, then just add `*` char to the end of the [word](#word) / phrase in the query.
For example, the following query returns [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field), which contain [words](#word) with `err` prefix:

```logsql
err*
```

This query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `err: foobar`
- `cannot open file: error occurred`

This query doesn't match the following log messages:

- `Error: foobar`, since the `Error` [word](#word) starts with capital letter. Use `i(err*)` for this case. See [these docs](#case-insensitive-filter) for details.
- `fooerror`, since the `fooerror` [word](#word) doesn't start with `err`. Use `~"err"` for this case. See [these docs](#substring-filter) for details.

Prefix filter can be applied to [phrases](#phrase-filter). For example, the following query matches
[log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) containing phrases with `unexpected fail` prefix:

```logsql
"unexpected fail"*
```

This query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `unexpected fail: IO error`
- `error:unexpected failure`

This query doesn't match the following log messages:

- `unexpectedly failed`, since the `unexpectedly` doesn't match `unexpected` [word](#word). Use `unexpected* AND fail*` for this case.
  See [these docs](#logical-filter) for details.
- `failed to open file: unexpected EOF`, since `failed` [word](#word) occurs before the `unexpected` word. Use `unexpected AND fail*` for this case.
  See [these docs](#logical-filter) for details.

If the prefix contains double quotes, then either put `\` in front of double quotes or put the prefix inside single quotes. For example, the following filter searches
logs with `"foo":"bar` prefix:

```logsql
'"foo":"bar'*
```

By default the prefix filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the prefix filter
in order to apply it to the given field. For example, the following query matches `log.level` field containing any word with the `err` prefix:

```logsql
log.level:err*
```

If the field name contains special chars, which may clash with the query syntax, then it may be put into quotes in the query.
For example, the following query matches `log:level` field containing any word with the `err` prefix.

```logsql
"log:level":err*
```

Performance tips:

- Prefer using [word filters](#word-filter) and [phrase filters](#phrase-filter) combined via [logical filter](#logical-filter)
  instead of prefix filter.
- Prefer moving [word filters](#word-filter) and [phrase filters](#phrase-filter) in front of prefix filter when using [logical filter](#logical-filter).
- See [other performance tips](#performance-tips).

See also:

- [Exact prefix filter](#exact-prefix-filter)
- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Exact-filter](#exact-filter)
- [Logical filter](#logical-filter)


### Substring filter

If it is needed to find logs with some substring, then `~"substring"` filter can be used. For example, the following query matches log entries,
which contain `ampl` text in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```logsql
~"ampl"
```

It matches the following messages:

- `Example message`
- `This is a sample`

It doesn't match `EXAMPLE message`, since `AMPL` substring here is in uppercase. Use `~"(?i)ampl"` filter instead. Note that case-insensitive filter
may be much slower than case-sensitive one.

Performance tip: prefer using [word filter](#word-filter) and [phrase filter](#phrase-filter), since substring filter may be quite slow.

See also:

- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Regexp filter](#regexp-filter)


### Range comparison filter

LogsQL supports `field:>X`, `field:>=X`, `field:<X` and `field:<=X` filters, where `field` is the name of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and `X` is [numeric value](#numeric-values), IPv4 address or a string. For example, the following query returns logs containing numeric values for the `response_size` field bigger than `10*1024`:

```logsql
response_size:>10KiB
```

The following query returns logs with `user` field containing string values smaller than `John`:

```logsql
username:<"John"
```

See also:

- [String range filter](#string-range-filter)
- [Range filter](#range-filter)

### Empty value filter

Sometimes it is needed to find log entries without the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
This can be performed with `log_field:""` syntax. For example, the following query matches log entries without `host.hostname` field:

```logsql
host.hostname:""
```

See also:

- [Any value filter](#any-value-filter)
- [Word filter](#word-filter)
- [Logical filter](#logical-filter)


### Any value filter

Sometimes it is needed to find log entries containing any non-empty value for the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
This can be performed with `log_field:*` syntax. For example, the following query matches log entries with non-empty `host.hostname` field:

```logsql
host.hostname:*
```

See also:

- [Empty value filter](#empty-value-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Exact filter

The [word filter](#word-filter) and [phrase filter](#phrase-filter) return [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
which contain the given word or phrase inside them. The message may contain additional text other than the requested word or phrase. If you need searching for log messages
or [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) with the exact value, then use the `exact` filter.
For example, the following query returns log messages with the exact value `fatal error: cannot find /foo/bar`:

```logsql
="fatal error: cannot find /foo/bar"
```

The query doesn't match the following log messages:

- `fatal error: cannot find /foo/bar/baz` or `some-text fatal error: cannot find /foo/bar`, since they contain an additional text
  other than the specified in the `exact` filter. Use `"fatal error: cannot find /foo/bar"` query in this case. See [these docs](#phrase-filter) for details.

- `FATAL ERROR: cannot find /foo/bar`, since the `exact` filter is case-sensitive. Use `i("fatal error: cannot find /foo/bar")` in this case.
  See [these docs](#case-insensitive-filter) for details.

By default the `exact` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the `exact` filter and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries with the exact `error` value at `log.level` field:

```logsql
log.level:="error"
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
log.დონე:="შეცდომა"
```

The field name can be put inside quotes if it contains special chars, which may clash with the query syntax.
For example, the following query matches the `error` value in the field `log:level`:

```logsql
"log:level":="error"
```

See also:

- [Exact prefix filter](#exact-prefix-filter)
- [Multi-exact filter](#multi-exact-filter)
- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Exact prefix filter

Sometimes it is needed to find log messages starting with some prefix. This can be done with the `="prefix"*` filter.
For example, the following query matches log messages, which start from `Processing request` prefix:

```logsql
="Processing request"*
```

This filter matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `Processing request foobar`
- `Processing requests from ...`

It doesn't match the following log messages:

- `processing request foobar`, since the log message starts with lowercase `p`. Use `="processing request"* OR ="Processing request"*`
  query in this case. See [these docs](#logical-filter) for details.
- `start: Processing request`, since the log message doesn't start with `Processing request`. Use `"Processing request"` query in this case.
  See [these docs](#phrase-filter) for details.

By default the `exact` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the `exact` filter and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries with `log.level` field, which starts with `err` prefix:

```logsql
log.level:="err"*
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
log.დონე:="შეცდომა"*
```

The field name can be put inside quotes if it contains special chars, which may clash with the query syntax.
For example, the following query matches `log:level` values starting with `err` prefix:

```logsql
"log:level":="err"*
```

See also:

- [Exact filter](#exact-filter)
- [Prefix filter](#prefix-filter)
- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Logical filter](#logical-filter)


### Multi-exact filter

Sometimes it is needed to locate log messages with a field containing one of the given values. This can be done with multiple [exact filters](#exact-filter)
combined into a single [logical filter](#logical-filter). For example, the following query matches log messages with `log.level` field
containing either `error` or `fatal` exact values:

```logsql
log.level:(="error" OR ="fatal")
```

While this solution works OK, LogsQL provides simpler and faster solution for this case - the `in()` filter.

```logsql
log.level:in("error", "fatal")
```

It works very fast for long lists passed to `in()`.

It is possible to pass arbitrary [query](#query-syntax) inside `in(...)` filter in order to match against the results of this query.
The query inside `in(...)` must end with [`fields`](#fields-pipe) pipe containing a single field name, so VictoriaLogs could
fetch results from this field. For example, the following query selects all the logs for the last 5 minutes for users,
who visited pages with `admin` [word](#word) in the `path` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
during the last day:

```logsql
_time:5m AND user_id:in(_time:1d AND path:admin | fields user_id)
```

See also:

- [Exact filter](#exact-filter)
- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Case-insensitive filter

Case-insensitive filter can be applied to any word, phrase or prefix by wrapping the corresponding [word filter](#word-filter),
[phrase filter](#phrase-filter) or [prefix filter](#prefix-filter) into `i()`. For example, the following query returns
log messages with `error` word in any case:

```logsql
i(error)
```

The query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

- `unknown error happened`
- `ERROR: cannot read file`
- `Error: unknown arg`
- `An ErRoR occurred`

The query doesn't match the following log messages:

- `FooError`, since the `FooError` [word](#word) has superfluous prefix `Foo`. Use `~"(?i)error"` for this case. See [these docs](#regexp-filter) for details.
- `too many Errors`, since the `Errors` [word](#word) has superfluous suffix `s`. Use `i(error*)` for this case.

By default the `i()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the filter
in order to apply it to the given field. For example, the following query matches `log.level` field containing `error` [word](#word) in any case:

```logsql
log.level:i(error)
```

If the field name contains special chars, which may clash with the query syntax, then it may be put into quotes in the query.
For example, the following query matches `log:level` field containing `error` [word](#word) in any case.

```logsql
"log:level":i("error")
```

Performance tips:

- Prefer using case-sensitive filter over case-insensitive filter.
- Prefer moving [word filter](#word-filter), [phrase filter](#phrase-filter) and [prefix filter](#prefix-filter) in front of case-sensitive filter
  when using [logical filter](#logical-filter).
- See [other performance tips](#performance-tips).


See also:

- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Exact-filter](#exact-filter)
- [Logical filter](#logical-filter)


### Sequence filter

Sometimes it is needed to find [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with [words](#word) or phrases in a particular order. For example, if log messages with `error` word followed by `open file` phrase
must be found, then the following LogsQL query can be used:

```logsql
seq("error", "open file")
```

This query matches `some error: cannot open file /foo/bar` message, since the `open file` phrase goes after the `error` [word](#word).
The query doesn't match the `cannot open file: error` message, since the `open file` phrase is located in front of the `error` [word](#word).
If you need matching log messages with both `error` word and `open file` phrase, then use `error AND "open file"` query. See [these docs](#logical-filter)
for details.

By default the `seq()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the filter
in order to apply it to the given field. For example, the following query matches `event.original` field containing `(error, "open file")` sequence:

```logsql
event.original:seq(error, "open file")
```

If the field name contains special chars, which may clash with the query syntax, then it may be put into quotes in the query.
For example, the following query matches `event:original` field containing `(error, "open file")` sequence:

```logsql
"event:original":seq(error, "open file")
```

See also:

- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Exact-filter](#exact-filter)
- [Logical filter](#logical-filter)


### Regexp filter

LogsQL supports regular expression filter with [re2 syntax](https://github.com/google/re2/wiki/Syntax) via `~"regex"` syntax.
For example, the following query returns all the log messages containing `err` or `warn` susbstrings:

```logsql
~"err|warn"
```

The query matches the following [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field), which contain either `err` or `warn` substrings:

- `error: cannot read data`
- `2 warnings have been raised`
- `data transferring finished`

The query doesn't match the following log messages:

- `ERROR: cannot open file`, since the `ERROR` word is in uppercase letters. Use `~"(?i)(err|warn)"` query for case-insensitive regexp search.
  See [these docs](https://github.com/google/re2/wiki/Syntax) for details. See also [case-insensitive filter docs](#case-insensitive-filter).
- `it is warmer than usual`, since it doesn't contain neither `err` nor `warn` substrings.

If the regexp contains double quotes, then either put `\` in front of double quotes or put the regexp inside single quotes. For example, the following regexp searches
logs matching `"foo":"(bar|baz)"` regexp:

```logsql
'"foo":"(bar|baz)"'
```

By default the regexp filter is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the filter
in order to apply it to the given field. For example, the following query matches `event.original` field containing either `err` or `warn` substrings:

```logsql
event.original:~"err|warn"
```

If the field name contains special chars, which may clash with the query syntax, then it may be put into quotes in the query.
For example, the following query matches `event:original` field containing either `err` or `warn` substrings:

```logsql
"event:original":~"err|warn"
```

Performance tips:

- Prefer combining simple [word filter](#word-filter) with [logical filter](#logical-filter) instead of using regexp filter.
  For example, the `~"error|warning"` query can be substituted with `error OR warning` query, which usually works much faster.
  Note that the `~"error|warning"` matches `errors` as well as `warnings` [words](#word), while `error OR warning` matches
  only the specified [words](#word). See also [multi-exact filter](#multi-exact-filter).
- Prefer moving the regexp filter to the end of the [logical filter](#logical-filter), so lighter filters are executed first.
- Prefer using `="some prefix"*` instead of `~"^some prefix"`, since the [`exact` filter](#exact-prefix-filter) works much faster than the regexp filter.
- See [other performance tips](#performance-tips).

See also:

- [Case-insensitive filter](#case-insensitive-filter)
- [Logical filter](#logical-filter)


### Range filter

If you need to filter log message by some field containing only numeric values, then the `range()` filter can be used.
For example, if the `request.duration` field contains the request duration in seconds, then the following LogsQL query can be used
for searching for log entries with request durations exceeding 4.2 seconds:

```logsql
request.duration:range(4.2, Inf)
```

This query can be shortened to by using [range comparison filter](#range-comparison-filter):

```logsql
request.duration:>4.2
```

The lower and the upper bounds of the `range(lower, upper)` are excluded by default. If they must be included, then substitute the corresponding
parentheses with square brackets. For example:

- `range[1, 10)` includes `1` in the matching range
- `range(1, 10]` includes `10` in the matching range
- `range[1, 10]` includes `1` and `10` in the matching range

The range boundaries can contain any [supported numeric values](#numeric-values).

Note that the `range()` filter doesn't match [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with non-numeric values alongside numeric values. For example, `range(1, 10)` doesn't match `the request took 4.2 seconds`
[log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field), since the `4.2` number is surrounded by other text.
Extract the numeric value from the message with `parse(_msg, "the request took <request_duration> seconds")` [transformation](#transformations)
and then apply the `range()` [filter pipe](#filter-pipe) to the extracted `request_duration` field.

Performance tips:

- It is better to query pure numeric [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  instead of extracting numeric field from text field via [transformations](#transformations) at query time.
- See [other performance tips](#performance-tips).

See also:

- [Range comparison filter](#range-comparison-filter)
- [IPv4 range filter](#ipv4-range-filter)
- [String range filter](#string-range-filter)
- [Length range filter](#length-range-filter)
- [Logical filter](#logical-filter)


### IPv4 range filter

If you need to filter log message by some field containing only [IPv4](https://en.wikipedia.org/wiki/Internet_Protocol_version_4) addresses such as `1.2.3.4`,
then the `ipv4_range()` filter can be used. For example, the following query matches log entries with `user.ip` address in the range `[127.0.0.0 - 127.255.255.255]`:

```logsql
user.ip:ipv4_range(127.0.0.0, 127.255.255.255)
```

The `ipv4_range()` accepts also IPv4 subnetworks in [CIDR notation](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing#CIDR_notation).
For example, the following query is equivalent to the query above:

```logsql
user.ip:ipv4_range("127.0.0.0/8")
```

If you need matching a single IPv4 address, then just put it inside `ipv4_range()`. For example, the following query matches `1.2.3.4` IP
at `user.ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```logsql
user.ip:ipv4_range("1.2.3.4")
```

Note that the `ipv4_range()` doesn't match a string with IPv4 address if this string contains other text. For example, `ipv4_range("127.0.0.0/24")`
doesn't match `request from 127.0.0.1: done` [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
since the `127.0.0.1` ip is surrounded by other text. Extract the IP from the message with `parse(_msg, "request from <ip>: done")` [transformation](#transformations)
and then apply the `ipv4_range()` [filter pipe](#filter-pipe) to the extracted `ip` field.

Hints:

- If you need searching for [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) containing the given `X.Y.Z.Q` IPv4 address,
  then `"X.Y.Z.Q"` query can be used. See [these docs](#phrase-filter) for details.
- If you need searching for [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) containing
  at least a single IPv4 address out of the given list, then `"ip1" OR "ip2" ... OR "ipN"` query can be used. See [these docs](#logical-filter) for details.
- If you need finding log entries with `ip` field in multiple ranges, then use `ip:(ipv4_range(range1) OR ipv4_range(range2) ... OR ipv4_range(rangeN))` query.
  See [these docs](#logical-filter) for details.

Performance tips:

- It is better querying pure IPv4 [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  instead of extracting IPv4 from text field via [transformations](#transformations) at query time.
- See [other performance tips](#performance-tips).

See also:

- [Range filter](#range-filter)
- [String range filter](#string-range-filter)
- [Length range filter](#length-range-filter)
- [Logical filter](#logical-filter)


### String range filter

If you need to filter log message by some field with string values in some range, then `string_range()` filter can be used.
For example, the following LogsQL query matches log entries with `user.name` field starting from `A` and `B` chars:

```logsql
user.name:string_range(A, C)
```

The `string_range()` includes the lower bound, while excluding the upper bound. This simplifies querying distinct sets of logs.
For example, the `user.name:string_range(C, E)` would match `user.name` fields, which start from `C` and `D` chars.

See also:

- [Range comparison filter](#range-comparison-filter)
- [Range filter](#range-filter)
- [IPv4 range filter](#ipv4-range-filter)
- [Length range filter](#length-range-filter)
- [Logical filter](#logical-filter)


### Length range filter

If you need to filter log message by its length, then `len_range()` filter can be used.
For example, the following LogsQL query matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with lengths in the range `[5, 10]` chars:

```logsql
len_range(5, 10)
```

This query matches the following log messages, since their length is in the requested range:

- `foobar`
- `foo bar`

This query doesn't match the following log messages:

- `foo`, since it is too short
- `foo bar baz abc`, sinc it is too long

It is possible to use `inf` as the upper bound. For example, the following query matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with the length bigger or equal to 5 chars:

```logsql
len_range(5, inf)
```

The range boundaries can be expressed in the following forms:

- Hexadecimal form. For example, `len_range(0xff, 0xABCD)`.
- Binary form. Form example, `len_range(0b100110, 0b11111101)`
- Integer form with `_` delimiters for better readability. For example, `len_range(1_000, 2_345_678)`.

By default the `len_range()` is applied to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
Put the [field name](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) in front of the `len_range()` in order to apply
the filter to the needed field. For example, the following query matches log entries with the `foo` field length in the range `[10, 20]` chars:

```logsql
foo:len_range(10, 20)
```

See also:

- [Range filter](#range-filter)
- [Logical filter](#logical-filter)


### Logical filter

Simpler LogsQL [filters](#filters) can be combined into more complex filters with the following logical operations:

- `q1 AND q2` - matches common log entries returned by both `q1` and `q2`. Arbitrary number of [filters](#filters) can be combined with `AND` operation.
  For example, `error AND file AND app` matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
  which simultaneously contain `error`, `file` and `app` [words](#word).
  The `AND` operation is frequently used in LogsQL queries, so it is allowed to skip the `AND` word.
  For example, `error file app` is equivalent to `error AND file AND app`.

- `q1 OR q2` - merges log entries returned by both `q1` and `q2`. Arbitrary number of [filters](#filters) can be combined with `OR` operation.
  For example, `error OR warning OR info` matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
  which contain at least one of `error`, `warning` or `info` [words](#word).

- `NOT q` - returns all the log entries except of those which match `q`. For example, `NOT info` returns all the
  [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
  which do not contain `info` [word](#word). The `NOT` operation is frequently used in LogsQL queries, so it is allowed substituting `NOT` with `-` and `!` in queries.
  For example, `-info` and `!info` are equivalent to `NOT info`.
  The `!` must be used instead of `-` in front of [`=`](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter)
  and [`~`](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter) filters like `!=` and `!~`.


The `NOT` operation has the highest priority, `AND` has the middle priority and `OR` has the lowest priority.
The priority order can be changed with parentheses. For example, `NOT info OR debug` is interpreted as `(NOT info) OR debug`,
so it matches [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
which do not contain `info` [word](#word), while it also matches messages with `debug` word (which may contain the `info` word).
This is not what most users expect. In this case the query can be rewritten to `NOT (info OR debug)`,
which correctly returns log messages without `info` and `debug` [words](#word).

LogsQL supports arbitrary complex logical queries with arbitrary mix of `AND`, `OR` and `NOT` operations and parentheses.

By default logical filters apply to the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
unless the inner filters explicitly specify the needed [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) via `field_name:filter` syntax.
For example, `(error OR warn) AND host.hostname:host123` is interpreted as `(_msg:error OR _msg:warn) AND host.hostname:host123`.

It is possible to specify a single [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for multiple filters
with the following syntax:

```logsql
field_name:(q1 OR q2 OR ... qN)
```

For example, `log.level:error OR log.level:warning OR log.level:info` can be substituted with the shorter query: `log.level:(error OR warning OR info)`.

Performance tips:

- VictoriaLogs executes logical operations from the left to the right, so it is recommended moving the most specific
  and the fastest filters (such as [word filter](#word-filter) and [phrase filter](#phrase-filter)) to the left,
  while moving less specific and the slowest filters (such as [regexp filter](#regexp-filter) and [case-insensitive filter](#case-insensitive-filter))
  to the right. For example, if you need to find [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
  with the `error` word, which match some `/foo/(bar|baz)` regexp,
  it is better from performance PoV to use the query `error ~"/foo/(bar|baz)"` instead of `~"/foo/(bar|baz)" error`.

  The most specific filter means that it matches the lowest number of log entries comparing to other filters.

- See [other performance tips](#performance-tips).

## Pipes

Additionally to [filters](#filters), LogsQL query may contain arbitrary mix of '|'-delimited actions known as `pipes`.
For example, the following query uses [`stats`](#stats-pipe), [`sort`](#sort-pipe) and [`limit`](#limit-pipe) pipes
for returning top 10 [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
with the biggest number of logs during the last 5 minutes:

```logsql
_time:5m | stats by (_stream) count() per_stream_logs | sort by (per_stream_logs desc) | limit 10
```

LogsQL supports the following pipes:

- [`blocks_count`](#blocks_count-pipe) counts the number of blocks with logs processed by the query.
- [`copy`](#copy-pipe) copies [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`delete`](#delete-pipe) deletes [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`drop_empty_fields`](#drop_empty_fields-pipe) drops [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with empty values.
- [`extract`](#extract-pipe) extracts the specified text into the given log fields.
- [`extract_regexp`](#extract_regexp-pipe) extracts the specified text into the given log fields via [RE2 regular expressions](https://github.com/google/re2/wiki/Syntax).
- [`field_names`](#field_names-pipe) returns all the names of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`field_values`](#field_values-pipe) returns all the values for the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`fields`](#fields-pipe) selects the given set of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`filter`](#filter-pipe) applies additional [filters](#filters) to results.
- [`format`](#format-pipe) formats output field from input [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`len`](#len-pipe) calculates byte length of the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) value.
- [`limit`](#limit-pipe) limits the number selected logs.
- [`math`](#math-pipe) performs mathematical calculations over [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`offset`](#offset-pipe) skips the given number of selected logs.
- [`pack_json`](#pack_json-pipe) packs [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into JSON object.
- [`pack_logfmt`](#pack_logfmt-pipe) packs [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into [logfmt](https://brandur.org/logfmt) message.
- [`rename`](#rename-pipe) renames [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`replace`](#replace-pipe) replaces substrings in the specified [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`replace_regexp`](#replace_regexp-pipe) updates [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with regular expressions.
- [`sort`](#sort-pipe) sorts logs by the given [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`stats`](#stats-pipe) calculates various stats over the selected logs.
- [`stream_context`](#stream_context-pipe) allows selecting surrounding logs in front and after the matching logs
  per each [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
- [`top`](#top-pipe) returns top `N` field sets with the maximum number of matching logs.
- [`uniq`](#uniq-pipe) returns unique log entires.
- [`unpack_json`](#unpack_json-pipe) unpacks JSON messages from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`unpack_logfmt`](#unpack_logfmt-pipe) unpacks [logfmt](https://brandur.org/logfmt) messages from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`unpack_syslog`](#unpack_syslog-pipe) unpacks [syslog](https://en.wikipedia.org/wiki/Syslog) messages from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`unroll`](#unroll-pipe) unrolls JSON arrays from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

### blocks_count pipe

`<q> | blocks_count` [pipe](#pipes) counts the number of blocks with logs processed by `<q>`. This pipe is needed mostly for debugging.

### copy pipe

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be copied, then `| copy src1 as dst1, ..., srcN as dstN` [pipe](#pipes) can be used.
For example, the following query copies `host` field to `server` for logs over the last 5 minutes, so the output contains both `host` and `server` fields:

```logsql
_time:5m | copy host as server
```

Multiple fields can be copied with a single `| copy ...` pipe. For example, the following query copies
[`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) to `timestamp`, while [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
is copied to `message`:

```logsql
_time:5m | copy _time as timestmap, _msg as message
```

The `as` keyword is optional.

`cp` keyword can be used instead of `copy` for convenience. For example, `_time:5m | cp foo bar` is equivalent to `_time:5m | copy foo as bar`.

See also:

- [`rename` pipe](#rename-pipe)
- [`fields` pipe](#fields-pipe)
- [`delete` pipe](#delete-pipe)

### delete pipe

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be deleted, then `| delete field1, ..., fieldN` [pipe](#pipes) can be used.
For example, the following query deletes `host` and `app` fields from the logs over the last 5 minutes:

```logsql
_time:5m | delete host, app
```

`drop`, `del` and `rm` keywords can be used instead of `delete` for convenience. For example, `_time:5m | drop host` is equivalent to `_time:5m | delete host`.

See also:

- [`rename` pipe](#rename-pipe)
- [`fields` pipe](#fields-pipe)

### drop_empty_fields pipe

`| drop_empty_fields` pipe drops [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with empty values. It also skips log entries with zero non-empty fields.

For example, the following query drops possible empty `email` field generated by [`extract` pipe](#extract-pipe) if the `foo` field doesn't contain email:

```logsql
_time:5m | extract 'email: <email>,' from foo | drop_empty_fields
```

See also:

- [`filter` pipe](#filter-pipe)
- [`extract` pipe](#extract-pipe)


### extract pipe

`| extract "pattern" from field_name` [pipe](#pipes) allows extracting arbitrary text into output fields according to the [`pattern`](#format-for-extract-pipe-pattern) from the given
[`field_name`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). Existing log fields remain unchanged after the `| extract ...` pipe.

`| extract ...` can be useful for extracting additional fields needed for further data processing with other pipes such as [`stats` pipe](#stats-pipe) or [`sort` pipe](#sort-pipe).

For example, the following query selects logs with the `error` [word](#word) for the last day,
extracts ip address from [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) into `ip` field and then calculates top 10 ip addresses
with the biggest number of logs:

```logsql
_time:1d error | extract "ip=<ip> " from _msg | stats by (ip) count() logs | sort by (logs) desc limit 10
```

It is expected that `_msg` field contains `ip=...` substring ending with space. For example, `error ip=1.2.3.4 from user_id=42`.
If there is no such substring in the current `_msg` field, then the `ip` output field will be empty.

If the `| extract ...` pipe is applied to [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field), then the `from _msg` part can be omitted.
For example, the following query is equivalent to the previous one:

```logsql
_time:1d error | extract "ip=<ip> " | stats by (ip) count() logs | sort by (logs) desc limit 10
```

If the `pattern` contains double quotes, then either put `\` in front of double quotes or put the `pattern` inside single quotes.
For example, the following query extracts `ip` from the corresponding JSON field:

```logsql
_time:5m | extract '"ip":"<ip>"'
```

Add `keep_original_fields` to the end of `extract ...` when the original non-empty values of the fields mentioned in the pattern must be preserved
instead of overwriting it with the extracted values. For example, the following query extracts `<ip>` only if the original value for `ip` field is missing or is empty:

```logsql
_time:5m | extract 'ip=<ip> ' keep_original_fields
```

By default `extract` writes empty matching fields to the output, which may overwrite existing values. Add `skip_empty_results` to the end of `extract ...`
in order to prevent from overwriting the existing values for the corresponding fields with empty values.
For example, the following query preserves the original `ip` field value if `foo` field doesn't contain the matching ip:

```logsql
_time:5m | extract 'ip=<ip> ' from foo skip_empty_results
```

Performance tip: it is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `extract`.
See [general performance tips](#performance-tips) for details.

See also:

- [Format for extract pipe pattern](#format-for-extract-pipe-pattern)
- [Conditional extract](#conditional-extract)
- [`extract_regexp` pipe](#extract_regexp-pipe)
- [`unpack_json` pipe](#unpack_json-pipe)
- [`unpack_logfmt` pipe](#unpack_logfmt-pipe)
- [`math` pipe](#math-pipe)

#### Format for extract pipe pattern

The `pattern` part from [`extract ` pipe](#extract-pipe) has the following format:

```
text1<field1>text2<field2>...textN<fieldN>textN+1
```

Where `text1`, ... `textN+1` is arbitrary non-empty text, which matches as is to the input text.

The `field1`, ... `fieldN` are placeholders, which match a substring of any length (including zero length) in the input text until the next `textX`.
Placeholders can be anonymous and named. Anonymous placeholders are written as `<_>`. They are used for convenience when some input text
must be skipped until the next `textX`. Named placeholders are written as `<some_name>`, where `some_name` is the name of the log field to store
the corresponding matching substring to.

Matching starts from the first occurrence of the `text1` in the input text. If the `pattern` starts with `<field1>` and doesn't contain `text1`,
then the matching starts from the beginning of the input text. Matching is performed sequentially according to the `pattern`. If some `textX` isn't found
in the remaining input text, then the remaining named placeholders receive empty string values and the matching finishes prematurely.
The empty string values can be dropped with [`drop_empty_fields` pipe](#drop_empty_fields-pipe).

Matching finishes successfully when `textN+1` is found in the input text.
If the `pattern` ends with `<fieldN>` and doesn't contain `textN+1`, then the `<fieldN>` matches the remaining input text.

For example, if [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) contains the following text:

```
1.2.3.4 GET /foo/bar?baz 404 "Mozilla  foo bar baz" some tail here
```

Then the following `pattern` can be used for extracting `ip`, `path` and `user_agent` fields from it:

```
<ip> <_> <path> <_> "<user_agent>"
```

Note that the user-agent part of the log message is in double quotes. This means that it may contain special chars, including escaped double quote, e.g. `\"`.
This may break proper matching of the string in double quotes.

VictoriaLogs automatically detects quoted strings and automatically unquotes them if the first matching char in the placeholder is double quote or backtick.
So it is better to use the following `pattern` for proper matching of quoted `user_agent` string:

```
<ip> <_> <path> <_> <user_agent>
```

This is useful for extracting JSON strings. For example, the following `pattern` properly extracts the `message` JSON string into `msg` field, even if it contains special chars:

```
"message":<msg>
```

The automatic string unquoting can be disabled if needed by adding `plain:` prefix in front of the field name. For example, if some JSON array of string values must be captured
into `json_array` field, then the following `pattern` can be used:

```
some json string array: [<plain:json_array>]
```

If some special chars such as `<` must be matched by the `pattern`, then they can be [html-escaped](https://en.wikipedia.org/wiki/List_of_XML_and_HTML_character_entity_references).
For example, the following `pattern` properly matches `a < b` text by extracting `a` into `left` field and `b` into `right` field:

```
<left> &lt; <right>
```

#### Conditional extract

If some log entries must be skipped from [`extract` pipe](#extract-pipe), then add `if (<filters>)` filter after the `extract` word.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query extracts `ip` field
from [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) only
if the input [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) doesn't contain `ip` field or this field is empty:

```logsql
_time:5m | extract if (ip:"") "ip=<ip> "
```

An alternative approach is to add `keep_original_fields` to the end of `extract`, in order to keep the original non-empty values for the extracted fields.
For example, the following query is equivalent to the previous one:

```logsql
_time:5m | extract "ip=<ip> " keep_original_fields
```

### extract_regexp pipe

`| extract_regexp "pattern" from field_name` [pipe](#pipes) extracts substrings from the [`field_name` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
according to the provided `pattern`, and stores them into field names according to the named fields inside the `pattern`.
The `pattern` must contain [RE2 regular expression](https://github.com/google/re2/wiki/Syntax) with named fields (aka capturing groups) in the form `(?P<capture_field_name>...)`.
Matching substrings are stored to the given `capture_field_name` [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
For example, the following query extracts ipv4 addresses from [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
and puts them into `ip` field for logs over the last 5 minutes:

```logsql
_time:5m | extract_regexp "(?P<ip>([0-9]+[.]){3}[0-9]+)" from _msg
```

The `from _msg` part can be omitted if the data extraction is performed from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
So the following query is equivalent to the previous one:

```logsql
_time:5m | extract_regexp "(?P<ip>([0-9]+[.]){3}[0-9]+)"
```

Add `keep_original_fields` to the end of `extract_regexp ...` when the original non-empty values of the fields mentioned in the pattern must be preserved
instead of overwriting it with the extracted values. For example, the following query extracts `<ip>` only if the original value for `ip` field is missing or is empty:

```logsql
_time:5m | extract_regexp 'ip=(?P<ip>([0-9]+[.]){3}[0-9]+)' keep_original_fields
```

By default `extract_regexp` writes empty matching fields to the output, which may overwrite existing values. Add `skip_empty_results` to the end of `extract_regexp ...`
in order to prevent from overwriting the existing values for the corresponding fields with empty values.
For example, the following query preserves the original `ip` field value if `foo` field doesn't contain the matching ip:

```logsql
_time:5m | extract_regexp 'ip=(?P<ip>([0-9]+[.]){3}[0-9]+)' from foo skip_empty_results
```

Performance tip: it is recommended using [`extract` pipe](#extract-pipe) instead of `extract_regexp` for achieving higher query performance.

See also:

- [Conditional `extract_regexp`](#conditional-extract_regexp)
- [`extract` pipe](#extract-pipe)
- [`replace_regexp` pipe](#replace_regexp-pipe)
- [`unpack_json` pipe](#unpack_json-pipe)

#### Conditional extract_regexp

If some log entries must be skipped from [`extract_regexp` pipe](#extract-pipe), then add `if (<filters>)` filter after the `extract` word.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query extracts `ip`
from [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) only
if the input [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) doesn't contain `ip` field or this field is empty:

```logsql
_time:5m | extract_regexp if (ip:"") "ip=(?P<ip>([0-9]+[.]){3}[0-9]+)"
```

An alternative approach is to add `keep_original_fields` to the end of `extract_regexp`, in order to keep the original non-empty values for the extracted fields.
For example, the following query is equivalent to the previous one:

```logsql
_time:5m | extract_regexp "ip=(?P<ip>([0-9]+[.]){3}[0-9]+)" keep_original_fields
```

### field_names pipe

`| field_names` [pipe](#pipes) returns all the names of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with an estimated number of logs per each field name.
For example, the following query returns all the field names with the number of matching logs over the last 5 minutes:

```logsql
_time:5m | field_names
```

Field names are returned in arbitrary order. Use [`sort` pipe](#sort-pipe) in order to sort them if needed.

See also:

- [`field_values` pipe](#field_values-pipe)
- [`uniq` pipe](#uniq-pipe)

### field_values pipe

`| field_values field_name` [pipe](#pipe) returns all the values for the given [`field_name` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with the number of logs per each value.
For example, the following query returns all the values with the number of matching logs for the field `level` over logs for the last 5 minutes:

```logsql
_time:5m | field_values level
```

It is possible limiting the number of returned values by adding `limit N` to the end of the `field_values ...`. For example, the following query returns
up to 10 values for the field `user_id` over logs for the last 5 minutes:

```logsql
_time:5m | field_values user_id limit 10
```

If the limit is reached, then the set of returned values is random. Also the number of matching logs per each returned value is zeroed for performance reasons.

See also:

- [`field_names` pipe](#field_names-pipe)
- [`top` pipe](#top-pipe)
- [`uniq` pipe](#uniq-pipe)

### fields pipe

By default all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) are returned in the response.
It is possible to select the given set of log fields with `| fields field1, ..., fieldN` [pipe](#pipes). For example, the following query selects only `host`
and [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) fields from logs for the last 5 minutes:

```logsql
_time:5m | fields host, _msg
```

`keep` can be used instead of `fields` for convenience. For example, the following query is equivalent to the previous one:

```logsql
_time:5m | keep host, _msg
```

See also:

- [`copy` pipe](#copy-pipe)
- [`rename` pipe](#rename-pipe)
- [`delete` pipe](#delete-pipe)

### filter pipe

The `| filter ...` [pipe](#pipes) allows filtering the selected logs entries with arbitrary [filters](#filters).

For example, the following query returns `host` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) values
if the number of log messages with the `error` [word](#word) for them over the last hour exceeds `1_000`:

```logsql
_time:1h error | stats by (host) count() logs_count | filter logs_count:> 1_000
```

It is allowed to use `where` prefix instead of `filter` prefix for convenience. For example, the following query is equivalent to the previous one:

```logsql
_time:1h error | stats by (host) count() logs_count | where logs_count:> 1_000
```

It is allowed to omit `filter` prefix if the used filters do not clash with [pipe names](#pipes).
So the following query is equivalent to the previous one:

```logsql
_time:1h error | stats by (host) count() logs_count | logs_count:> 1_000
```

See also:

- [`stats` pipe](#stats-pipe)
- [`sort` pipe](#sort-pipe)

### format pipe

`| format "pattern" as result_field` [pipe](#pipe) combines [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
according to the `pattern` and stores it to the `result_field`.

For example, the following query stores `request from <ip>:<port>` text into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
by substituting `<ip>` and `<port>` with the corresponding [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) values:

```logsql
_time:5m | format "request from <ip>:<port>" as _msg
```

If the result of the `format` pattern is stored into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field),
then `as _msg` part can be omitted. The following query is equivalent to the previous one:

```logsql
_time:5m | format "request from <ip>:<port>"
```

If some field values must be put into double quotes before formatting, then add `q:` in front of the corresponding field name.
For example, the following command generates properly encoded JSON object from `_msg` and `stacktrace` [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and stores it into `my_json` output field:

```logsql
_time:5m | format '{"_msg":<q:_msg>,"stacktrace":<q:stacktrace>}' as my_json
```

Numeric fields can be transformed into the following string representation at `format` pipe:

- [RFC3339 time](https://www.rfc-editor.org/rfc/rfc3339) - by adding `time:` in front of the corresponding field name
  containing [Unix timestamp](https://en.wikipedia.org/wiki/Unix_time) in nanoseconds.
  For example, `format "time=<time:timestamp_nsecs>"`. The timestamp can be converted into nanoseconds with the [`math` pipe](#math-pipe).

- Human-readable duration - by adding `duration:` in front of the corresponding numeric field name containing duration in nanoseconds.
  For example, `format "duration=<duration:duration_nsecs>"`. The duration can be converted into nanoseconds with the [`math` pipe](#math-pipe).

- IPv4 - by adding `ipv4:` in front of the corresponding field name containing `uint32` representation of the IPv4 address.
  For example, `format "ip=<ipv4:ip_num>"`.

Add `keep_original_fields` to the end of `format ... as result_field` when the original non-empty value of the `result_field` must be preserved
instead of overwriting it with the `format` results. For example, the following query adds formatted result to `foo` field only if it was missing or empty:

```logsql
_time:5m | format 'some_text' as foo keep_original_fields
```

Add `skip_empty_results` to the end of `format ...` if empty results shouldn't be written to the output. For example, the following query adds formatted result to `foo` field
when at least `field1` or `field2` aren't empty, while preserving the original `foo` value:

```logsql
_time:5m | format "<field1><field2>" as foo skip_empty_results
```

Performance tip: it is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `format`.
See [general performance tips](#performance-tips) for details.

See also:

- [Conditional format](#conditional-format)
- [`replace` pipe](#replace-pipe)
- [`replace_regexp` pipe](#replace_regexp-pipe)
- [`extract` pipe](#extract-pipe)


#### Conditional format

If the [`format` pipe](#format-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` just after the `format` word.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query stores the formatted result to `message` field
only if `ip` and `host` [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) aren't empty:

```logsql
_time:5m | format if (ip:* and host:*) "request from <ip>:<host>" as message
```

### len pipe

The `| len(field) as result` pipe stores byte length of the given `field` value into the `result` field.
For example, the following query shows top 5 log entries with the maximum byte length of `_msg` field across
logs for the last 5 minutes:

```logsql
_time:5m | len(_msg) as msg_len | sort by (msg_len desc) | limit 1
```

See also:

- [`sum_len` stats function](#sum-len-stats)
- [`sort` pipe](#sort-pipe)
- [`limit` pipe](#limit-pipe)

### limit pipe

If only a subset of selected logs must be processed, then `| limit N` [pipe](#pipes) can be used, where `N` can contain any [supported integer numeric value](#numeric-values).
For example, the following query returns up to 100 logs over the last 5 minutes:

```logsql
_time:5m | limit 100
```

`head` keyword can be used instead of `limit` for convenience. For example, `_time:5m | head 100` is equivalent to `_time:5m | limit 100`.

The `N` in `head N` can be omitted - in this case up to 10 matching logs are returned:

```logsql
error | head
```

By default rows are selected in arbitrary order because of performance reasons, so the query above can return different sets of logs every time it is executed.
[`sort` pipe](#sort-pipe) can be used for making sure the logs are in the same order before applying `limit ...` to them.

See also:

- [`sort` pipe](#sort-pipe)
- [`offset` pipe](#offset-pipe)

### math pipe

`| math ...` [pipe](#pipes) performs mathematical calculations over [numeric values](#numeric-values) stored in [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
It has the following format:

```
| math
  expr1 as resultName1,
  ...
  exprN as resultNameN
```

Where `exprX` is one of the supported math expressions mentioned below, while `resultNameX` is the name of the field to store the calculated result to.
The `as` keyword is optional. The result name can be omitted. In this case the result is stored to a field with the name equal to string representation
of the corresponding math expression.

`exprX` may reference `resultNameY` calculated before the given `exprX`.

For example, the following query divides `duration_msecs` field value by 1000, then rounds it to integer and stores the result in the `duration_secs` field:

```logsql
_time:5m | math round(duration_msecs / 1000) as duration_secs
```

The following mathematical operations are supported by `math` pipe:

- `arg1 + arg2` - returns the sum of `arg1` and `arg2`
- `arg1 - arg2` - returns the difference between `arg1` and `arg2`
- `arg1 * arg2` - multiplies `arg1` by `arg2`
- `arg1 / arg2` - divides `arg1` by `arg2`
- `arg1 % arg2` - returns the remainder of the division of `arg1` by `arg2`
- `arg1 ^ arg2` - returns the power of `arg1` by `arg2`
- `arg1 & arg2` - returns bitwise `and` for `arg1` and `arg2`. It is expected that `arg1` and `arg2` are in the range `[0 .. 2^53-1]`
- `arg1 or arg2` - returns bitwise `or` for `arg1` and `arg2`. It is expected that `arg1` and `arg2` are in the range `[0 .. 2^53-1]`
- `arg1 xor arg2` - returns bitwise `xor` for `arg1` and `arg2`. It is expected that `arg1` and `arg2` are in the range `[0 .. 2^53-1]`
- `arg1 default arg2` - returns `arg2` if `arg1` is non-[numeric](#numeric-values) or equals to `NaN`
- `abs(arg)` - returns an absolute value for the given `arg`
- `ceil(arg)` - returns the least integer value greater than or equal to `arg`
- `exp(arg)` - powers [`e`](https://en.wikipedia.org/wiki/E_(mathematical_constant)) by `arg`
- `floor(arg)` - returns the greatest integer values less than or equal to `arg`
- `ln(arg)` - returns [natural logarithm](https://en.wikipedia.org/wiki/Natural_logarithm) for the given `arg`
- `max(arg1, ..., argN)` - returns the maximum value among the given `arg1`, ..., `argN`
- `min(arg1, ..., argN)` - returns the minimum value among the given `arg1`, ..., `argN`
- `round(arg)` - returns rounded to integer value for the given `arg`. The `round()` accepts optional `nearest` arg, which allows rounding the number to the given `nearest` multiple.
  For example, `round(temperature, 0.1)` rounds `temperature` field to one decimal digit after the point.

Every `argX` argument in every mathematical operation can contain one of the following values:

- The name of [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). For example, `errors_total / requests_total`.
  The log field is parsed into numeric value if it contains [supported numeric value](#numeric-values). The log field is parsed into [Unix timestamp](https://en.wikipedia.org/wiki/Unix_time)
  in nanoseconds if it contains [rfc3339 time](https://www.rfc-editor.org/rfc/rfc3339). The log field is parsed into `uint32` number if it contains IPv4 address.
  The log field is parsed into `NaN` in other cases.
- Any [supported numeric value](#numeric-values), [rfc3339 time](https://www.rfc-editor.org/rfc/rfc3339) or IPv4 address. For example, `1MiB`, `"2024-05-15T10:20:30.934324Z"` or `"12.34.56.78"`.
- Another mathematical expression, which can be put inside `(...)`. For example, `(a + b) * c`.

The parsed time, duration and IPv4 address can be converted back to string representation after math transformations with the help of [`format` pipe](#format-pipe). For example,
the following query rounds the `request_duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) to seconds before converting it back to string representation:

```logsql
_time:5m | math round(request_duration, 1e9) as request_duration_nsecs | format '<duration:request_duration_nsecs>' as request_duration
```

The `eval` keyword can be used instead of `math` for convenience. For example, the following query calculates `duration_msecs` field
by multiplying `duration_secs` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) to `1000`:

```logsql
_time:5m | eval (duration_secs * 1000) as duration_msecs
```

See also:

- [`stats` pipe](#stats-pipe)
- [`extract` pipe](#extract-pipe)
- [`format` pipe](#format-pipe)


### offset pipe

If some selected logs must be skipped after [`sort`](#sort-pipe), then `| offset N` [pipe](#pipes) can be used, where `N` can contain any [supported integer numeric value](#numeric-values).
For example, the following query skips the first 100 logs over the last 5 minutes after sorting them by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
_time:5m | sort by (_time) | offset 100
```

`skip` keyword can be used instead of `offset` keyword for convenience. For example, `_time:5m | skip 10` is equivalent to `_time:5m | offset 10`.

Note that skipping rows without sorting has little sense, since they can be returned in arbitrary order because of performance reasons.
Rows can be sorted with [`sort` pipe](#sort-pipe).

See also:

- [`limit` pipe](#limit-pipe)
- [`sort` pipe](#sort-pipe)

### pack_json pipe

`| pack_json as field_name` [pipe](#pipe) packs all [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into JSON object
and stores it as a string in the given `field_name`.

For example, the following query packs all the fields into JSON object and stores it into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
for logs over the last 5 minutes:

```logsql
_time:5m | pack_json as _msg
```

The `as _msg` part can be omitted if packed JSON object is stored into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | pack_json
```

If only a subset of labels must be packed into JSON, then it must be listed inside `fields (...)` after `pack_json`. For example, the following query builds JSON with `foo` and `bar` fields
only and stores the result in `baz` field:

```logsql
_time:5m | pack_json fields (foo, bar) as baz
```

The `pack_json` doesn't modify or delete other labels. If you do not need them, then add [`| fields ...`](#fields-pipe) after the `pack_json` pipe. For example, the following query
leaves only the `foo` label with the original log fields packed into JSON:

```logsql
_time:5m | pack_json as foo | fields foo
```

See also:

- [`pack_logfmt` pipe](#pack_logfmt-pipe)
- [`unpack_json` pipe](#unpack_json-pipe)


### pack_logfmt pipe

`| pack_logfmt as field_name` [pipe](#pipe) packs all [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into [logfmt](https://brandur.org/logfmt) message
and stores it as a string in the given `field_name`.

For example, the following query packs all the fields into [logfmt](https://brandur.org/logfmt) message and stores it
into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) for logs over the last 5 minutes:

```logsql
_time:5m | pack_logfmt as _msg
```

The `as _msg` part can be omitted if packed message is stored into [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | pack_logfmt
```

If only a subset of labels must be packed into [logfmt](https://brandur.org/logfmt), then it must be listed inside `fields (...)` after `pack_logfmt`.
For example, the following query builds [logfmt](https://brandur.org/logfmt) message with `foo` and `bar` fields only and stores the result in `baz` field:

```logsql
_time:5m | pack_logfmt fields (foo, bar) as baz
```

The `pack_logfmt` doesn't modify or delete other labels. If you do not need them, then add [`| fields ...`](#fields-pipe) after the `pack_logfmt` pipe. For example, the following query
leaves only the `foo` label with the original log fields packed into [logfmt](https://brandur.org/logfmt):

```logsql
_time:5m | pack_logfmt as foo | fields foo
```

See also:

- [`pack_json` pipe](#pack_json-pipe)
- [`unpack_logfmt` pipe](#unpack_logfmt-pipe)

### rename pipe

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be renamed, then `| rename src1 as dst1, ..., srcN as dstN` [pipe](#pipes) can be used.
For example, the following query renames `host` field to `server` for logs over the last 5 minutes, so the output contains `server` field instead of `host` field:

```logsql
_time:5m | rename host as server
```

Multiple fields can be renamed with a single `| rename ...` pipe. For example, the following query renames `host` to `instance` and `app` to `job`:

```logsql
_time:5m | rename host as instance, app as job
```

The `as` keyword is optional.

`mv` keyword can be used instead of `rename` keyword for convenience. For example, `_time:5m | mv foo bar` is equivalent to `_time:5m | rename foo as bar`.

See also:

- [`copy` pipe](#copy-pipe)
- [`fields` pipe](#fields-pipe)
- [`delete` pipe](#delete-pipe)

### replace pipe

`| replace ("old", "new") at field` [pipe](#pipes) replaces all the occurrences of the `old` substring with the `new` substring
in the given [`field`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query replaces all the `secret-password` substrings with `***` in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
for logs over the last 5 minutes:

```logsql
_time:5m | replace ("secret-password", "***") at _msg
```

The `at _msg` part can be omitted if the replacement occurs in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | replace ("secret-password", "***")
```

The number of replacements can be limited with `limit N` at the end of `replace`. For example, the following query replaces only the first `foo` substring with `bar`
at the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) `baz`:

```logsql
_time:5m | replace ('foo', 'bar') at baz limit 1
```

Performance tip: it is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `replace`.
See [general performance tips](#performance-tips) for details.

See also:

- [Conditional replace](#conditional-replace)
- [`replace_regexp` pipe](#replace_regexp-pipe)
- [`format` pipe](#format-pipe)
- [`extract` pipe](#extract-pipe)

#### Conditional replace

If the [`replace` pipe](#replace-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `replace`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query replaces `secret` with `***` in the `password` field
only if `user_type` field equals to `admin`:

```logsql
_time:5m | replace if (user_type:=admin) ("secret", "***") at password
```

### replace_regexp pipe

`| replace_regexp ("regexp", "replacement") at field` [pipe](#pipes) replaces all the substrings matching the given `regexp` with the given `replacement`
in the given [`field`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

The `regexp` must contain regular expression with [RE2 syntax](https://github.com/google/re2/wiki/Syntax).
The `replacement` may contain `$N` or `${N}` placeholders, which are substituted with the `N-th` capturing group in the `regexp`.

For example, the following query replaces all the substrings starting with `host-` and ending with `-foo` with the contents between `host-` and `-foo` in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) for logs over the last 5 minutes:

```logsql
_time:5m | replace_regexp ("host-(.+?)-foo", "$1") at _msg
```

The `at _msg` part can be omitted if the replacement occurs in the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | replace_regexp ("host-(.+?)-foo", "$1")
```

The number of replacements can be limited with `limit N` at the end of `replace`. For example, the following query replaces only the first `password: ...` substring
ending with whitespace with empty substring at the [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) `baz`:

```logsql
_time:5m | replace_regexp ('password: [^ ]+', '') at baz limit 1
```

Performance tips:

- It is recommended using [`replace` pipe](#replace-pipe) instead of `replace_regexp` if possible, since it works faster.
- It is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `replace`.
  See [general performance tips](#performance-tips) for details.

See also:

- [Conditional replace_regexp](#conditional-replace_regexp)
- [`replace` pipe](#replace-pipe)
- [`format` pipe](#format-pipe)
- [`extract` pipe](#extract-pipe)

#### Conditional replace_regexp

If the [`replace_regexp` pipe](#replace-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `replace_regexp`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query replaces `password: ...` substrings ending with whitespace
with `***` in the `foo` field only if `user_type` field equals to `admin`:

```logsql
_time:5m | replace_regexp if (user_type:=admin) ("password: [^ ]+", "") at foo
```

### sort pipe

By default logs are selected in arbitrary order because of performance reasons. If logs must be sorted, then `| sort by (field1, ..., fieldN)` [pipe](#pipes) can be used.
The returned logs are sorted by the given [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
using [natural sorting](https://en.wikipedia.org/wiki/Natural_sort_order).

For example, the following query returns logs for the last 5 minutes sorted by [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
and then by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field):

```logsql
_time:5m | sort by (_stream, _time)
```

Add `desc` after the given log field in order to sort in reverse order of this field. For example, the following query sorts log fields in reverse order of `request_duration_seconds` field:

```logsql
_time:5m | sort by (request_duration_seconds desc)
```

The reverse order can be applied globally via `desc` keyword after `by(...)` clause:

```logsql
_time:5m | sort by (foo, bar) desc
```

The `by` keyword can be skipped in `sort ...` pipe. For example, the following query is equivalent to the previous one:

```logsql
_time:5m | sort (foo, bar) desc
```

The `order` alias can be used instead of `sort`, so the following query is equivalent to the previous one:

```logsql
_time:5m | order by (foo, bar) desc
```

Sorting of big number of logs can consume a lot of CPU time and memory. Sometimes it is enough to return the first `N` entries with the biggest
or the smallest values. This can be done by adding `limit N` to the end of `sort ...` pipe.
Such a query consumes lower amounts of memory when sorting big number of logs, since it keeps in memory only `N` log entries.
For example, the following query returns top 10 log entries with the biggest values
for the `request_duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) during the last hour:

```logsql
_time:1h | sort by (request_duration desc) limit 10
```

If the first `N` sorted results must be skipped, then `offset N` can be added to `sort` pipe. For example,
the following query skips the first 10 logs with the biggest `request_duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
and then returns the next 20 sorted logs for the last 5 minutes:

```logsql
_time:1h | sort by (request_duration desc) offset 10 limit 20
```

It is possible returning a rank (sort order number) for every sorted log by adding `rank as <fieldName>` to the end of `| sort ...` pipe.
For example, the following query stores rank for sorted by [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) logs
into `position` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```logsql
_time:5m | sort by (_time) rank as position
```

Note that sorting of big number of logs can be slow and can consume a lot of additional memory.
It is recommended limiting the number of logs before sorting with the following approaches:

- Adding `limit N` to the end of `sort ...` pipe.
- Reducing the selected time range with [time filter](#time-filter).
- Using more specific [filters](#filters), so they select less logs.
- Limiting the number of selected [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) via [`fields` pipe](#fields-pipe).

See also:

- [`stats` pipe](#stats-pipe)
- [`limit` pipe](#limit-pipe)
- [`offset` pipe](#offset-pipe)

### stats pipe

`| stats ...` pipe allows calculating various stats over the selected logs. For example, the following LogsQL query
uses [`count` stats function](#count-stats) for calculating the number of logs for the last 5 minutes:

```logsql
_time:5m | stats count() as logs_total
```

`| stats ...` pipe has the following basic format:

```logsql
... | stats
  stats_func1(...) as result_name1,
  ...
  stats_funcN(...) as result_nameN
```

Where `stats_func*` is any of the supported [stats function](#stats-pipe-functions), while `result_name*` is the name of the log field
to store the result of the corresponding stats function. The `as` keyword is optional.

For example, the following query calculates the following stats for logs over the last 5 minutes:

- the number of logs with the help of [`count` stats function](#count-stats);
- the number of unique [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with the help of [`count_uniq` stats function](#count_uniq-stats):

```logsql
_time:5m | stats count() logs_total, count_uniq(_stream) streams_total
```

It is allowed omitting `stats` prefix for convenience. So the following query is equivalent to the previous one:

```logsql
_time:5m | count() logs_total, count_uniq(_stream) streams_total
```

It is allowed omitting the result name. In this case the result name equals to the string representation of the used [stats function](#stats-pipe-functions).
For example, the following query returns the same stats as the previous one, but gives uses `count()` and `count_uniq(_stream)` names for the returned fields:

```logsql
_time:5m | count(), count_uniq(_stream)
```

See also:

- [stats by fields](#stats-by-fields)
- [stats by time buckets](#stats-by-time-buckets)
- [stats by time buckets with timezone offset](#stats-by-time-buckets-with-timezone-offset)
- [stats by field buckets](#stats-by-field-buckets)
- [stats by IPv4 buckets](#stats-by-ipv4-buckets)
- [stats with additional filters](#stats-with-additional-filters)
- [stats pipe functions](#stats-pipe-functions)
- [`math` pipe](#math-pipe)
- [`sort` pipe](#sort-pipe)
- [`uniq` pipe](#uniq-pipe)
- [`top` pipe](#top-pipe)


#### Stats by fields

The following LogsQL syntax can be used for calculating independent stats per group of log fields:

```logsql
... | stats by (field1, ..., fieldM)
  stats_func1(...) as result_name1,
  ...
  stats_funcN(...) as result_nameN
```

This calculates `stats_func*` per each `(field1, ..., fieldM)` group of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query calculates the number of logs and unique ip addresses over the last 5 minutes,
grouped by `(host, path)` fields:

```logsql
_time:5m | stats by (host, path) count() logs_total, count_uniq(ip) ips_total
```

The `by` keyword can be skipped in `stats ...` pipe. For example, the following query is equivalent to the previous one:

```logsql
_time:5m | stats (host, path) count() logs_total, count_uniq(ip) ips_total
```

See also:

- [`row_min`](#row_min-stats)
- [`row_max`](#row_max-stats)
- [`row_any`](#row_any-stats)

#### Stats by time buckets

The following syntax can be used for calculating stats grouped by time buckets:

```logsql
... | stats by (_time:step)
  stats_func1(...) as result_name1,
  ...
  stats_funcN(...) as result_nameN
```

This calculates `stats_func*` per each `step` of [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) field.
The `step` can have any [duration value](#duration-values). For example, the following LogsQL query returns per-minute number of logs and unique ip addresses
over the last 5 minutes:

```
_time:5m | stats by (_time:1m) count() logs_total, count_uniq(ip) ips_total
```

Additionally, the following `step` values are supported:

- `nanosecond` - equals to `1ns` [duration](#duration-values).
- `microsecond` - equals to `1µs` [duration](#duration-values).
- `millisecond` - equals to `1ms` [duration](#duration-values).
- `second` - equals to `1s` [duration](#duration-values).
- `minute` - equals to `1m` [duration](#duration-values).
- `hour` - equals to `1h` [duration](#duration-values).
- `day` - equals to `1d` [duration](#duration-values).
- `week` - equals to `1w` [duration](#duration-values).
- `month` - equals to one month. It properly takes into account the number of days per each month.
- `year` - equals to one year. It properly takes into account the number of days per each year.

#### Stats by time buckets with timezone offset

VictoriaLogs stores [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) values as [Unix time](https://en.wikipedia.org/wiki/Unix_time)
in nanoseconds. This time corresponds to [UTC](https://en.wikipedia.org/wiki/Coordinated_Universal_Time) time zone. Sometimes it is needed calculating stats
grouped by days or weeks at non-UTC timezone. This is possible with the following syntax:

```logsql
... | stats by (_time:step offset timezone_offset) ...
```

For example, the following query calculates per-day number of logs over the last week, in `UTC+02:00` [time zone](https://en.wikipedia.org/wiki/Time_zone):

```logsql
_time:1w | stats by (_time:1d offset 2h) count() logs_total
```

#### Stats by field buckets

Every log field inside `| stats by (...)` can be bucketed in the same way at `_time` field in [this example](#stats-by-time-buckets).
Any [numeric value](#numeric-values) can be used as `step` value for the bucket. For example, the following query calculates
the number of requests for the last hour, bucketed by 10KB of `request_size_bytes` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```logsql
_time:1h | stats by (request_size_bytes:10KB) count() requests
```

#### Stats by IPv4 buckets

Stats can be bucketed by [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) containing [IPv4 addresses](https://en.wikipedia.org/wiki/IP_address)
via the `ip_field_name:/network_mask` syntax inside `by(...)` clause. For example, the following query returns the number of log entries per `/24` subnetwork
extracted from the `ip` [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) during the last 5 minutes:

```logsql
_time:5m | stats by (ip:/24) count() requests_per_subnet
```

#### Stats with additional filters

Sometimes it is needed to calculate stats on different subsets of matching logs. This can be done by inserting `if (<any_filters>)` condition
between [stats function](#stats-pipe-functions) and `result_name`, where `any_filter` can contain arbitrary [filters](#filters).
For example, the following query calculates individually the number of [logs messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
with `GET`, `POST` and `PUT` [words](#word), additionally to the total number of logs over the last 5 minutes:

```logsql
_time:5m | stats
  count() if (GET) gets,
  count() if (POST) posts,
  count() if (PUT) puts,
  count() total
```

### stream_context pipe

`| stream_context ...` [pipe](#pipes) allows selecting surrounding logs for the matching logs in [logs stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
in the way similar to `grep -A` / `grep -B`. The returned log chunks are delimited with `---` [log message](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
for easier investigation.

For example, the following query returns up to 10 additional logs after every log message with the `panic` [word](#word) across all the logs for the last 5 minutes:

```logsql
_time:5m panic | stream_context after 10
```

The following query returns up to 5 additional logs in front of every log message with the `stacktrace` [word](#word) across all the logs for the last 5 minutes:

```logsql
_time:5m stacktrace | stream_context before 5
```

The following query returns up to 2 logs in front of the log message with the `error` [word](#word) and up to 5 logs after this log message
across all the logs for the last 5 minutes:

```logsql
_time:5m error | stream_context before 2 after 5
```

The `| stream_context` [pipe](#pipes) must go first just after the [filters](#filters).

### top pipe

`| top N by (field1, ..., fieldN)` [pipe](#pipes) returns top `N` sets for `(field1, ..., fieldN)` [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with the maximum number of matching log entries.

For example, the following query returns top 7 [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
with the maximum number of log entries over the last 5 minutes:

```logsql
_time:5m | top 7 by (_stream)
```

The `N` is optional. If it is skipped, then top 10 entries are returned. For example, the following query returns top 10 values
for `ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) seen in logs for the last 5 minutes:

```logsql
_time:5m | top by (ip)
```

The `by (...)` part in the `top` [pipe](#pipes) is optional. If it is skipped, then all the log fields are taken into account
when determining top field sets. This is useful when the field sets are already limited by other pipes such as [`fields` pipe](#fields-pipe).
For example, the following query is equivalent to the previous one:

```logsql
_time:5m | fields ip | top
```

See also:

- [`uniq` pipe](#uniq-pipe)
- [`stats` pipe](#stats-pipe)

### uniq pipe

`| uniq ...` [pipe](#pipes) returns unique results over the selected logs. For example, the following LogsQL query
returns unique values for `ip` [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | uniq by (ip)
```

It is possible to specify multiple fields inside `by(...)` clause. In this case all the unique sets for the given fields
are returned. For example, the following query returns all the unique `(host, path)` pairs for the logs over the last 5 minutes:

```logsql
_time:5m | uniq by (host, path)
```

The unique entries are returned in arbitrary order. Use [`sort` pipe](#sort-pipe) in order to sort them if needed.

Add `with hits` after `uniq by (...)` in order to return the number of matching logs per each field value:

```logsql
_time:5m | uniq by (host) with hits
```

Unique entries are stored in memory during query execution. Big number of unique selected entries may require a lot of memory.
Sometimes it is enough to return up to `N` unique entries. This can be done by adding `limit N` after `by (...)` clause.
This allows limiting memory usage. For example, the following query returns up to 100 unique `(host, path)` pairs for the logs over the last 5 minutes:

```logsql
_time:5m | uniq by (host, path) limit 100
```

If the `limit` is reached, then arbitrary subset of unique values can be returned. The `hits` calculation doesn't work when the `limit` is reached.

The `by` keyword can be skipped in `uniq ...` pipe. For example, the following query is equivalent to the previous one:

```logsql
_time:5m | uniq (host, path) limit 100
```

See also:

- [`uniq_values` stats function](#uniq_values-stats)
- [`top` pipe](#top-pipe)
- [`stats` pipe](#stats-pipe)

### unpack_json pipe

`| unpack_json from field_name` [pipe](#pipes) unpacks `{"k1":"v1", ..., "kN":"vN"}` JSON from the given input [`field_name`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
into `k1`, ... `kN` output field names with the corresponding `v1`, ..., `vN` values. It overrides existing fields with names from the `k1`, ..., `kN` list. Other fields remain untouched.

Nested JSON is unpacked according to the rules defined [here](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query unpacks JSON fields from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) across logs for the last 5 minutes:

```logsql
_time:5m | unpack_json from _msg
```

The `from _msg` part can be omitted when JSON fields are unpacked from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | unpack_json
```

If only some fields must be extracted from JSON, then they can be enumerated inside `fields (...)`. For example, the following query unpacks only `foo` and `bar`
fields from JSON value stored in `my_json` [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model):

```logsql
_time:5m | unpack_json from my_json fields (foo, bar)
```

If it is needed to preserve the original non-empty field values, then add `keep_original_fields` to the end of `unpack_json ...`. For example,
the following query preserves the original non-empty values for `ip` and `host` fields instead of overwriting them with the unpacked values:

```logsql
_time:5m | unpack_json from foo fields (ip, host) keep_original_fields
```

Add `skip_empty_results` to the end of `unpack_json ...` if the original field values must be preserved when the corresponding unpacked values are empty.
For example, the following query preserves the original `ip` and `host` field values for empty unpacked values:

```logsql
_time:5m | unpack_json fields (ip, host) skip_empty_results
```

Performance tip: if you need extracting a single field from long JSON, it is faster to use [`extract` pipe](#extract-pipe). For example, the following query extracts `"ip"` field from JSON
stored in [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) at the maximum speed:

```
_time:5m | extract '"ip":<ip>'
```

If you want to make sure that the unpacked JSON fields do not clash with the existing fields, then specify common prefix for all the fields extracted from JSON,
by adding `result_prefix "prefix_name"` to `unpack_json`. For example, the following query adds `foo_` prefix for all the unpacked fields
form `foo`:

```logsql
_time:5m | unpack_json from foo result_prefix "foo_"
```

Performance tips:

- It is better from performance and resource usage PoV ingesting parsed JSON logs into VictoriaLogs
  according to the [supported data model](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  instead of ingesting unparsed JSON lines into VictoriaLogs and then parsing them at query time with [`unpack_json` pipe](#unpack_json-pipe).

- It is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `unpack_json`.
  See [general performance tips](#performance-tips) for details.

See also:

- [Conditional `unpack_json`](#conditional-unpack_json)
- [`unpack_logfmt` pipe](#unpack_logfmt-pipe)
- [`unpack_syslog` pipe](#unpack_syslog-pipe)
- [`extract` pipe](#extract-pipe)
- [`unroll` pipe](#unroll-pipe)
- [`pack_json` pipe](#pack_json-pipe)
- [`pack_logfmt` pipe](#pack_logfmt-pipe)

#### Conditional unpack_json

If the [`unpack_json` pipe](#unpack_json-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `unpack_json`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query unpacks JSON fields from `foo` field only if `ip` field in the current log entry isn't set or empty:

```logsql
_time:5m | unpack_json if (ip:"") from foo
```

### unpack_logfmt pipe

`| unpack_logfmt from field_name` [pipe](#pipes) unpacks `k1=v1 ... kN=vN` [logfmt](https://brandur.org/logfmt) fields
from the given [`field_name`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into `k1`, ... `kN` field names
with the corresponding `v1`, ..., `vN` values. It overrides existing fields with names from the `k1`, ..., `kN` list. Other fields remain untouched.

For example, the following query unpacks [logfmt](https://brandur.org/logfmt) fields from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
across logs for the last 5 minutes:

```logsql
_time:5m | unpack_logfmt from _msg
```

The `from _msg` part can be omitted when [logfmt](https://brandur.org/logfmt) fields are unpacked from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | unpack_logfmt
```

If only some fields must be unpacked from logfmt, then they can be enumerated inside `fields (...)`. For example, the following query extracts only `foo` and `bar` fields
from logfmt stored in the `my_logfmt` field:

```logsql
_time:5m | unpack_logfmt from my_logfmt fields (foo, bar)
```

If it is needed to preserve the original non-empty field values, then add `keep_original_fields` to the end of `unpack_logfmt ...`. For example,
the following query preserves the original non-empty values for `ip` and `host` fields instead of overwriting them with the unpacked values:

```logsql
_time:5m | unpack_logfmt from foo fields (ip, host) keep_original_fields
```

Add `skip_empty_results` to the end of `unpack_logfmt ...` if the original field values must be preserved when the corresponding unpacked values are empty.
For example, the following query preserves the original `ip` and `host` field values for empty unpacked values:

```logsql
_time:5m | unpack_logfmt fields (ip, host) skip_empty_results
```

Performance tip: if you need extracting a single field from long [logfmt](https://brandur.org/logfmt) line, it is faster to use [`extract` pipe](#extract-pipe).
For example, the following query extracts `"ip"` field from [logfmt](https://brandur.org/logfmt) line stored
in [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field):

```
_time:5m | extract ' ip=<ip>'
```

If you want to make sure that the unpacked [logfmt](https://brandur.org/logfmt) fields do not clash with the existing fields, then specify common prefix for all the fields extracted from logfmt,
by adding `result_prefix "prefix_name"` to `unpack_logfmt`. For example, the following query adds `foo_` prefix for all the unpacked fields
from `foo` field:

```logsql
_time:5m | unpack_logfmt from foo result_prefix "foo_"
```

Performance tips:

- It is better from performance and resource usage PoV ingesting parsed [logfmt](https://brandur.org/logfmt) logs into VictoriaLogs
  according to the [supported data model](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  instead of ingesting unparsed logfmt lines into VictoriaLogs and then parsing them at query time with [`unpack_logfmt` pipe](#unpack_logfmt-pipe).

- It is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `unpack_logfmt`.
  See [general performance tips](#performance-tips) for details.

See also:

- [Conditional unpack_logfmt](#conditional-unpack_logfmt)
- [`unpack_json` pipe](#unpack_json-pipe)
- [`unpack_syslog` pipe](#unpack_syslog-pipe)
- [`extract` pipe](#extract-pipe)

#### Conditional unpack_logfmt

If the [`unpack_logfmt` pipe](#unpack_logfmt-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `unpack_logfmt`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query unpacks logfmt fields from `foo` field
only if `ip` field in the current log entry isn't set or empty:

```logsql
_time:5m | unpack_logfmt if (ip:"") from foo
```

### unpack_syslog pipe

`| unpack_syslog from field_name` [pipe](#pipes) unpacks [syslog](https://en.wikipedia.org/wiki/Syslog) message
from the given [`field_name`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). It understands the following Syslog formats:

- [RFC3164](https://datatracker.ietf.org/doc/html/rfc3164) aka `<PRI>MMM DD hh:mm:ss HOSTNAME APP-NAME[PROCID]: MESSAGE`
- [RFC5424](https://datatracker.ietf.org/doc/html/rfc5424) aka `<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MESSAGE`

The following fields are unpacked:

- `priority` - it is obtained from `PRI`.
- `facility` - it is calculated as `PRI / 8`.
- `severity` - it is calculated as `PRI % 8`.
- `format` - either `rfc3164` or `rfc5424` depending on which Syslog format is unpacked.
- `timestamp` - timestamp in [ISO8601 format](https://en.wikipedia.org/wiki/ISO_8601). The `MMM DD hh:mm:ss` timestamp in [RFC3164](https://datatracker.ietf.org/doc/html/rfc3164)
  is automatically converted into [ISO8601 format](https://en.wikipedia.org/wiki/ISO_8601) by assuming that the timestamp belongs to the last 12 months.
- `hostname`
- `app_name`
- `proc_id`
- `msg_id`
- `message`

The `<PRI>` part is optional. If it is missing, then `priority`, `facility` and `severity` fields aren't set.

The `[STRUCTURED-DATA]` is parsed into fields with the `SD-ID.param1`, `SD-ID.param2`, ..., `SD-ID.paramN` names and the corresponding values
according to [the specification](https://datatracker.ietf.org/doc/html/rfc5424#section-6.3).

For example, the following query unpacks [syslog](https://en.wikipedia.org/wiki/Syslog) message from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
across logs for the last 5 minutes:

```logsql
_time:5m | unpack_syslog from _msg
```

The `from _msg` part can be omitted when [syslog](https://en.wikipedia.org/wiki/Syslog) message is unpacked
from the [`_msg` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field).
The following query is equivalent to the previous one:

```logsql
_time:5m | unpack_syslog
```

By default timestamps in [RFC3164 format](https://datatracker.ietf.org/doc/html/rfc3164) are converted to local timezone. It is possible to change the timezone
offset via `offset` option. For example, the following query adds 5 hours and 30 minutes to unpacked `rfc3164` timestamps:

```logsql
_time:5m | unpack_syslog offset 5h30m
```

If it is needed to preserve the original non-empty field values, then add `keep_original_fields` to the end of `unpack_syslog ...`:

```logsql
_time:5m | unpack_syslog keep_original_fields
```

If you want to make sure that the unpacked [syslog](https://en.wikipedia.org/wiki/Syslog) fields do not clash with the existing fields,
then specify common prefix for all the fields extracted from syslog, by adding `result_prefix "prefix_name"` to `unpack_syslog`.
For example, the following query adds `foo_` prefix for all the unpacked fields from `foo` field:

```logsql
_time:5m | unpack_syslog from foo result_prefix "foo_"
```

Performance tips:

- It is better from performance and resource usage PoV ingesting parsed [syslog](https://en.wikipedia.org/wiki/Syslog) messages into VictoriaLogs
  according to the [supported data model](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  instead of ingesting unparsed syslog lines into VictoriaLogs and then parsing them at query time with [`unpack_syslog` pipe](#unpack_syslog-pipe).

- It is recommended using more specific [log filters](#filters) in order to reduce the number of log entries, which are passed to `unpack_syslog`.
  See [general performance tips](#performance-tips) for details.

See also:

- [Conditional unpack_syslog](#conditional-unpack_syslog)
- [`unpack_json` pipe](#unpack_json-pipe)
- [`unpack_logfmt` pipe](#unpack_logfmt-pipe)
- [`extract` pipe](#extract-pipe)

#### Conditional unpack_syslog

If the [`unpack_syslog` pipe](#unpack_syslog-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `unpack_syslog`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query unpacks syslog message fields from `foo` field
only if `hostname` field in the current log entry isn't set or empty:

```logsql
_time:5m | unpack_syslog if (hostname:"") from foo
```

### unroll pipe

`| unroll by (field1, ..., fieldN)` [pipe](#pipes) can be used for unrolling JSON arrays from `field1`, `fieldN`
[log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into separate rows.

For example, the following query unrolls `timestamp` and `value` [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) from logs for the last 5 minutes:

```logsql
_time:5m | unroll (timestamp, value)
```

See also:

- [`unpack_json` pipe](#unpack_json-pipe)
- [`extract` pipe](#extract-pipe)
- [`uniq_values` stats function](#uniq_values-stats)
- [`values` stats function](#values-stats)

#### Conditional unroll

If the [`unroll` pipe](#unpack_logfmt-pipe) mustn't be applied to every [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
then add `if (<filters>)` after `unroll`.
The `<filters>` can contain arbitrary [filters](#filters). For example, the following query unrolls `value` field only if `value_type` field equals to `json_array`:

```logsql
_time:5m | unroll if (value_type:="json_array") (value)
```

## stats pipe functions

LogsQL supports the following functions for [`stats` pipe](#stats-pipe):

- [`avg`](#avg-stats) returns the average value over the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`count`](#count-stats) returns the number of log entries.
- [`count_empty`](#count_empty-stats) returns the number logs with empty [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`count_uniq`](#count_uniq-stats) returns the number of unique non-empty values for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`max`](#max-stats) returns the maximum value over the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`median`](#median-stats) returns the [median](https://en.wikipedia.org/wiki/Median) value over the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`min`](#min-stats) returns the minumum value over the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`quantile`](#quantile-stats) returns the given quantile for the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`row_any`](#row_any-stats) returns a sample [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) per each selected [stats group](#stats-by-fields).
- [`row_max`](#row_max-stats) returns the [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with the minimum value at the given field.
- [`row_min`](#row_min-stats) returns the [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with the maximum value at the given field.
- [`sum`](#sum-stats) returns the sum for the given numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`sum_len`](#sum_len-stats) returns the sum of lengths for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`uniq_values`](#uniq_values-stats) returns unique non-empty values for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- [`values`](#values-stats) returns all the values for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

### avg stats

`avg(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the average value across
all the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
Non-numeric values are ignored.

For example, the following query returns the average value for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats avg(duration) avg_duration
```

See also:

- [`median`](#median-stats)
- [`quantile`](#quantile-stats)
- [`min`](#min-stats)
- [`max`](#max-stats)
- [`sum`](#sum-stats)
- [`count`](#count-stats)

### count stats

`count()` [stats pipe function](#stats-pipe-functions) calculates the number of selected logs.

For example, the following query returns the number of logs over the last 5 minutes:

```logsql
_time:5m | stats count() logs
```

It is possible calculating the number of logs with non-empty values for some [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with the `count(fieldName)` syntax. For example, the following query returns the number of logs with non-empty `username` field over the last 5 minutes:

```logsql
_time:5m | stats count(username) logs_with_username
```

If multiple fields are enumerated inside `count()`, then it counts the number of logs with at least a single non-empty field mentioned inside `count()`.
For example, the following query returns the number of logs with non-empty `username` or `password` [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over the last 5 minutes:

```logsql
_time:5m | stats count(username, password) logs_with_username_or_password
```

See also:

- [`count_uniq`](#count_uniq-stats)
- [`count_empty`](#count_empty-stats)
- [`sum`](#sum-stats)
- [`avg`](#avg-stats)

### count_empty stats

`count_empty(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the number of logs with empty `(field1, ..., fieldN)` tuples.

For example, the following query calculates the number of logs with empty `username` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
during the last 5 minutes:

```logsql
_time:5m | stats count_empty(username) logs_with_missing_username
```

See also:

- [`count`](#count-stats)
- [`count_uniq`](#count_uniq-stats)

### count_uniq stats

`count_uniq(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the number of unique non-empty `(field1, ..., fieldN)` tuples.

For example, the following query returns the number of unique non-empty values for `ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over the last 5 minutes:

```logsql
_time:5m | stats count_uniq(ip) ips
```

The following query returns the number of unique `(host, path)` pairs for the corresponding [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over the last 5 minutes:

```logsql
_time:5m | stats count_uniq(host, path) unique_host_path_pairs
```

Every unique value is stored in memory during query execution. Big number of unique values may require a lot of memory.
Sometimes it is needed to know whether the number of unique values reaches some limit. In this case add `limit N` just after `count_uniq(...)`
for limiting the number of counted unique values up to `N`, while limiting the maximum memory usage. For example, the following query counts
up to `1_000_000` unique values for the `ip` field:

```logsql
_time:5m | stats count_uniq(ip) limit 1_000_000 as ips_1_000_000
```

See also:

- [`uniq_values`](#uniq_values-stats)
- [`count`](#count-stats)

### max stats

`max(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) returns the maximum value across
all the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query returns the maximum value for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats max(duration) max_duration
```

[`row_max`](#row_max-stats) function can be used for obtaining other fields with the maximum duration.

See also:

- [`row_max`](#row_max-stats)
- [`min`](#min-stats)
- [`quantile`](#quantile-stats)
- [`avg`](#avg-stats)

### median stats

`median(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the [median](https://en.wikipedia.org/wiki/Median) value across
the give numeric [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query return median for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats median(duration) median_duration
```

See also:

- [`quantile`](#quantile-stats)
- [`avg`](#avg-stats)

### min stats

`min(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) returns the minimum value across
all the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query returns the minimum value for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats min(duration) min_duration
```

[`row_min`](#row_min-stats) function can be used for obtaining other fields with the minimum duration.

See also:

- [`row_min`](#row_min-stats)
- [`max`](#max-stats)
- [`quantile`](#quantile-stats)
- [`avg`](#avg-stats)

### quantile stats

`quantile(phi, field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates `phi` [percentile](https://en.wikipedia.org/wiki/Percentile) over numeric values
for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). The `phi` must be in the range `0 ... 1`, where `0` means `0th` percentile,
while `1` means `100th` percentile.

For example, the following query calculates `50th`, `90th` and `99th` percentiles for the `request_duration_seconds` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats
  quantile(0.5, request_duration_seconds) p50,
  quantile(0.9, request_duration_seconds) p90,
  quantile(0.99, request_duration_seconds) p99
```

See also:

- [`min`](#min-stats)
- [`max`](#max-stats)
- [`median`](#median-stats)
- [`avg`](#avg-stats)

### row_any stats

`row_any()` [stats pipe function](#stats-pipe-functions) returns arbitrary [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
(aka sample) per each selected [stats group](#stats-by-fields). Log entry is returned as JSON-encoded dictionary with all the fields from the original log.

For example, the following query returns a sample log entry per each [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
across logs for the last 5 minutes:

```logsql
_time:5m | stats by (_stream) row_any() as sample_row
```

Fields from the returned values can be decoded with [`unpack_json`](#unpack_json-pipe) or [`extract`](#extract-pipe) pipes.

If only the specific fields are needed, then they can be enumerated inside `row_any(...)`.
For example, the following query returns only `_time` and `path` fields from a sample log entry for logs over the last 5 minutes:

```logsql
_time:5m | stats row_any(_time, path) as time_and_path_sample
```

See also:

- [`row_max`](#row_max-stats)
- [`row_min`](#row_min-stats)

### row_max stats

`row_max(field)` [stats pipe function](#stats-pipe-functions) returns [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with the maximum value for the given `field`. Log entry is returned as JSON-encoded dictionary with all the fields from the original log.

For example, the following query returns log entry with the maximum value for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across logs for the last 5 minutes:

```logsql
_time:5m | stats row_max(duration) as log_with_max_duration
```

Fields from the returned values can be decoded with [`unpack_json`](#unpack_json-pipe) or [`extract`](#extract-pipe) pipes.

If only the specific fields are needed from the returned log entry, then they can be enumerated inside `row_max(...)`.
For example, the following query returns only `_time`, `path` and `duration` fields from the log entry with the maximum `duration` over the last 5 minutes:

```logsql
_time:5m | stats row_max(duration, _time, path, duration) as time_and_path_with_max_duration
```

See also:

- [`max`](#max-stats)
- [`row_min`](#row_min-stats)
- [`row_any`](#row_any-stats)

### row_min stats

`row_min(field)` [stats pipe function](#stats-pipe-functions) returns [log entry](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
with the minimum value for the given `field`. Log entry is returned as JSON-encoded dictionary with all the fields from the original log.

For example, the following query returns log entry with the minimum value for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across logs for the last 5 minutes:

```logsql
_time:5m | stats row_min(duration) as log_with_min_duration
```

Fields from the returned values can be decoded with [`unpack_json`](#unpack_json-pipe) or [`extract`](#extract-pipe) pipes.

If only the specific fields are needed from the returned log entry, then they can be enumerated inside `row_max(...)`.
For example, the following query returns only `_time`, `path` and `duration` fields from the log entry with the minimum `duration` over the last 5 minutes:

```logsql
_time:5m | stats row_min(duration, _time, path, duration) as time_and_path_with_min_duration
```

See also:

- [`min`](#min-stats)
- [`row_max`](#row_max-stats)
- [`row_any`](#row_any-stats)

### sum stats

`sum(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the sum of numeric values across
all the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query returns the sum of numeric values for the `duration` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats sum(duration) sum_duration
```

See also:

- [`count`](#count-stats)
- [`avg`](#avg-stats)
- [`max`](#max-stats)
- [`min`](#min-stats)

### sum_len stats

`sum_len(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) calculates the sum of byte lengths of all the values
for the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

For example, the following query returns the sum of byte lengths of [`_msg` fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field)
across all the logs for the last 5 minutes:

```logsql
_time:5m | stats sum_len(_msg) messages_len
```

See also:

- [`count`](#count-stats)
- [`len` pipe](#len-pipe)

### uniq_values stats

`uniq_values(field1, ..., fieldN)` [stats pipe function](#stats-pipe-functions) returns the unique non-empty values across
the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
The returned values are encoded in sorted JSON array.

For example, the following query returns unique non-empty values for the `ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats uniq_values(ip) unique_ips
```

The returned unique ip addresses can be unrolled into distinct log entries with [`unroll` pipe](#unroll-pipe).

Every unique value is stored in memory during query execution. Big number of unique values may require a lot of memory. Sometimes it is enough to return
only a subset of unique values. In this case add `limit N` after `uniq_values(...)` in order to limit the number of returned unique values to `N`,
while limiting the maximum memory usage.
For example, the following query returns up to `100` unique values for the `ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over the logs for the last 5 minutes:

```logsql
_time:5m | stats uniq_values(ip) limit 100 as unique_ips_100
```

Arbitrary subset of unique `ip` values is returned every time if the `limit` is reached.

See also:

- [`uniq` pipe](#uniq-pipe)
- [`values`](#values-stats)
- [`count_uniq`](#count_uniq-stats)
- [`count`](#count-stats)

### values stats

`values(field1, ..., fieldN)` [stats pipe fuction](#stats-pipe-functions) returns all the values (including empty values)
for the mentioned [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
The returned values are encoded in JSON array.

For example, the following query returns all the values for the `ip` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
over logs for the last 5 minutes:

```logsql
_time:5m | stats values(ip) ips
```

The returned ip addresses can be unrolled into distinct log entries with [`unroll` pipe](#unroll-pipe).

See also:

- [`uniq_values`](#uniq_values-stats)
- [`count`](#count-stats)
- [`count_empty`](#count_empty-stats)

## Stream context

See [`stream_context` pipe](#stream_context-pipe).

## Transformations

LogsQL supports the following transformations on the log entries selected with [filters](#filters):

- Extracting arbitrary text from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) according to the provided pattern.
  See [these docs](#extract-pipe) for details.
- Unpacking JSON fields from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](#unpack_json-pipe).
- Unpacking [logfmt](https://brandur.org/logfmt) fields from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](#unpack_logfmt-pipe).
- Unpacking [Syslog](https://en.wikipedia.org/wiki/Syslog) messages from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](#unpack_syslog-pipe).
- Creating a new field from existing [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) according to the provided format. See [`format` pipe](#format-pipe).
- Replacing substrings in the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
  See [`replace` pipe](#replace-pipe) and [`replace_regexp` pipe](#replace_regexp-pipe) docs.
- Creating a new field according to math calculations over existing [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [`math` pipe](#math-pipe).

It is also possible to perform various transformations on the [selected log entries](#filters) at client side
with `jq`, `awk`, `cut`, etc. Unix commands according to [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line).

## Post-filters

Post-filtering of query results can be performed at any step by using [`filter` pipe](#filter-pipe).

It is also possible to perform post-filtering of the [selected log entries](#filters) at client side with `grep` and similar Unix commands
according to [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line).

## Stats

Stats over the selected logs can be calculated via [`stats` pipe](#stats-pipe).

It is also possible to perform stats calculations on the [selected log entries](#filters) at client side with `sort`, `uniq`, etc. Unix commands
according to [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line).

## Sorting

By default VictoriaLogs doesn't sort the returned results because of performance reasons. Use [`sort` pipe](#sort-pipe) for sorting the results.

## Limiters

LogsQL provides the following [pipes](#pipes) for limiting the number of returned log entries:

- [`fields`](#fields-pipe) and [`delete`](#delete-pipe) pipes allow limiting the set of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) to return.
- [`limit` pipe](#limit-pipe) allows limiting the number of log entries to return.

## Querying specific fields

Specific log fields can be queried via [`fields` pipe](#fields-pipe).

## Comments

LogsQL query may contain comments at any place. The comment starts with `#` and continues until the end of the current line.
Example query with comments:

```logsql
error                       # find logs with `error` word
  | stats by (_stream) logs # then count the number of logs per `_stream` label
  | sort by (logs) desc     # then sort by the found logs in descending order
  | limit 5                 # and show top 5 streams with the biggest number of logs
```

## Numeric values

LogsQL accepts numeric values in the following formats:

- regular integers like `12345` or `-12345`
- regular floating point numbers like `0.123` or `-12.34`
- [short numeric format](#short-numeric-values)
- [duration format](#duration-values)

### Short numeric values

LogsQL accepts integer and floating point values with the following suffixes:

- `K` and `KB` - the value is multiplied by `10^3`
- `M` and `MB` - the value is multiplied by `10^6`
- `G` and `GB` - the value is multiplied by `10^9`
- `T` and `TB` - the value is multiplied by `10^12`
- `Ki` and `KiB` - the value is multiplied by `2^10`
- `Mi` and `MiB` - the value is multiplied by `2^20`
- `Gi` and `GiB` - the value is multiplied by `2^30`
- `Ti` and `TiB` - the value is multiplied by `2^40`

All the numbers may contain `_` delimiters, which may improve readability of the query. For example, `1_234_567` is equivalent to `1234567`,
while `1.234_567` is equivalent to `1.234567`.

## Duration values

LogsQL accepts duration values with the following suffixes at places where the duration is allowed:

- `ns` - nanoseconds. For example, `123ns`.
- `µs` - microseconds. For example, `1.23µs`.
- `ms` - milliseconds. For example, `1.23456ms`
- `s` - seconds. For example, `1.234s`
- `m` - minutes. For example, `1.5m`
- `h` - hours. For example, `1.5h`
- `d` - days. For example, `1.5d`
- `w` - weeks. For example, `1w`
- `y` - years as 365 days. For example, `1.5y`

Multiple durations can be combined. For example, `1h33m55s`.

Internally duration values are converted into nanoseconds.

## Performance tips

- It is highly recommended specifying [time filter](#time-filter) in order to narrow down the search to specific time range.
- It is highly recommended specifying [stream filter](#stream-filter) in order to narrow down the search
  to specific [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
- Move faster filters such as [word filter](#word-filter) and [phrase filter](#phrase-filter) to the beginning of the query.
  This rule doesn't apply to [time filter](#time-filter) and [stream filter](#stream-filter), which can be put at any place of the query.
- Move more specific filters, which match lower number of log entries, to the beginning of the query.
  This rule doesn't apply to [time filter](#time-filter) and [stream filter](#stream-filter), which can be put at any place of the query.
- If the selected logs are passed to [pipes](#pipes) for further transformations and statistics' calculations, then it is recommended
  reducing the number of selected logs by using more specific [filters](#filters), which return lower number of logs to process by [pipes](#pipes).
