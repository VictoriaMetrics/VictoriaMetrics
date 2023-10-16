---
sort: 5
weight: 5
title: LogsQL
menu:
  docs:
    parent: "victorialogs"
    weight: 5
aliases:
- /VictoriaLogs/LogsQL.html
---

# LogsQL

LogsQL is a simple yet powerful query language for [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/).
It provides the following features:

- Full-text search across [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
  See [word filter](#word-filter), [phrase filter](#phrase-filter) and [prefix filter](#prefix-filter).
- Ability to combine filters into arbitrary complex [logical filters](#logical-filter).
- Ability to extract structured fields from unstructured logs at query time. See [these docs](#transformations).
- Ability to calculate various stats over the selected log entries. See [these docs](#stats).

## LogsQL tutorial

If you aren't familiar with VictoriaLogs, then start with [key concepts docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html).

Then follow these docs:
- [How to run VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html).
- [how to ingest data into VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

The simplest LogsQL query is just a [word](#word), which must be found in the [log message](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
For example, the following query finds all the logs with `error` word:

```logsql
error
```

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

Queries above match logs with any [timestamp](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field),
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
- The filter on the [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field).

The `AND` operator means that the [log entry](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) must match both filters in order to be selected.

Typical LogsQL query constists of multiple [filters](#filters) joined with `AND` operator. It may be tiresome typing and then reading all these `AND` words.
So LogsQL allows omitting `AND` words. For example, the following query is equivalent to the query above:

```logsql
error _time:5m
```

The query returns the following [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) by default:

- [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
- [`_stream` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields)
- [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field)

Logs may contain arbitrary number of other fields. If you need obtaining some of these fields in query results,
then just refer them in the query with `field_name:*` [filter](#any-value-filter). See [these docs](#querying-specific-fields) for more details.

For example, the following query returns `host.hostname` field additionally to `_msg`, `_stream` and `_time` fields:

```logsql
error _time:5m host.hostname:*
```

Suppose the query above selects too many rows because some buggy app pushes invalid error logs to VictoriaLogs. Suppose the app adds `buggy_app` [word](#word) to every log line.
Then the following query removes all the logs from the buggy app, allowing us paying attention to the real errors:

```logsql
_time:5m error NOT buggy_app
```

This query uses `NOT` [operator](#logical-filter) for removing log lines from the buggy app. The `NOT` operator is used frequently, so it can be substituted with `!` char.
So the following query is equivalent to the previous one:

```logsql
_time:5m error !buggy_app
```

Suppose another buggy app starts pushing invalid error logs to VictoriaLogs - it adds `foobar` [word](#word) to every emitted log line.
No problems - just add `!foobar` to the query in order to remove these buggy logs:

```logsql
_time:5m error !buggy_app !foobar
```

This query can be rewritten to more clear query with the `OR` [operator](#logical-filter) inside parentheses:

```logsql
_time:5m error !(buggy_app OR foobar)
```

Note that the parentheses are required here, since otherwise the query won't return the expected results.
The query `error !buggy_app OR foobar` is interpreted as `(error AND NOT buggy_app) OR foobar`. This query may return error logs
from the buggy app if they contain `foobar` [word](#word). This query also continues returning all the error logs from the second buggy app.
This is because of different priorities for `NOT`, `AND` and `OR` operators.
Read [these docs](#logical-filter) for more details. There is no need in remembering all these priority rules -
just wrap the needed query parts into explicit parentheses if you aren't sure in priority rules.
As an additional bonus, explicit parentheses make queries easier to read and maintain.

Queries above assume that the `error` [word](#word) is stored in the [log message](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
This word can be stored in other [field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) such as `log.level`.
How to select error logs in this case? Just add the `log.level:` prefix in front of the `error` word:

```logsq
_time:5m log.level:error !(buggy_app OR foobar)
```

The field name can be wrapped into quotes if it contains special chars or keywords, which may clash with LogsQL syntax.
Any [word](#word) also can be wrapped into quotes. So the following query is equivalent to the previous one:

```logsql
"_time":"5m" "log.level":"error" !("buggy_app" OR "foobar")
```

What if the application identifier - such as `buggy_app` and `foobar` - is stored in the `app` field? Correct - just add `app:` prefix in front of `buggy_app` and `foobar`:

```logsql
_time:5m log.level:error !(app:buggy_app OR app:foobar)
```

The query can be simplified by moving the `app:` prefix outside the parentheses:

```logsql
_time:5m log.level:error !app:(buggy_app OR foobar)
```

The `app` field uniquely identifies the application instance if a single instance runs per each unique `app`.
In this case it is recommended associating the `app` field with [log stream fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields)
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/). This usually improves both compression rate
and query performance when querying the needed streams via [`_stream` filter](#stream-filter).
If the `app` field is associated with the log stream, then the query above can be rewritten to more performant one:

```logsql
_time:5m log.level:error _stream:{app!~"buggy_app|foobar"}
```

This query completely skips scanning for logs from `buggy_app` and `foobar` apps, thus significantly reducing disk read IO and CPU time
needed for performing the query.

Finally, it is recommended reading [performance tips](#performance-tips).

Now you are familiar with LogsQL basics. Read [query syntax](#query-syntax) if you want to continue learning LogsQL.

### Key concepts

#### Word

LogsQL splits all the [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) into words
delimited by non-word chars such as whitespace, parens, punctuation chars, etc. For example, the `foo: (bar,"тест")!` string
is split into `foo`, `bar` and `тест` words. Words can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8) chars.
These words are taken into account by full-text search filters such as
[word filter](#word-filter), [phrase filter](#phrase-filter) and [prefix filter](#prefix-filter).

#### Query syntax

LogsQL query consists of the following parts delimited by `|`:

- [Filters](#filters), which select log entries for further processing. This part is required in LogsQL. Other parts are optional.
- Optional [stream context](#stream-context), which allows selecting surrounding log lines for the matching log lines.
- Optional [transformations](#transformations) for the selected log fields.
  For example, an additional fields can be extracted or constructed from existing fields.
- Optional [post-filters](#post-filters) for post-filtering of the selected results. For example, post-filtering can filter
  results based on the fields constructed by [transformations](#transformations).
- Optional [stats](#stats) transformations, which can calculate various stats across selected results.
- Optional [sorting](#sorting), which can sort the results by the sepcified fields.
- Optional [limiters](#limiters), which can apply various limits on the selected results.

## Filters

LogsQL supports various filters for searching for log messages (see below).
They can be combined into arbitrary complex queries via [logical filters](#logical-filter).

Filters are applied to [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) by default.
If the filter must be applied to other [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model),
then its' name followed by the colon must be put in front of the filter. For example, if `error` [word filter](#word-filter) must be applied
to the `log.level` field, then use `log.level:error` query.

Field names and filter args can be put into quotes if they contain special chars, which may clash with LogsQL syntax. LogsQL supports quoting via double quotes `"`,
single quotes `'` and backticks:

```logsql
"some 'field':123":i('some("value")') AND `other"value'`
```

If doubt, it is recommended quoting field names and filter args.


The list of LogsQL filters:

- [Time filter](#time-filter) - matches logs with [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field) in the given time range
- [Stream filter](#stream-filter) - matches logs, which belong to the given [streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields)
- [Word filter](#word-filter) - matches logs with the given [word](#word)
- [Phrase filter](#phrase-filter) - matches logs with the given phrase
- [Prefix filter](#prefix-filter) - matches logs with the given word prefix or phrase prefix
- [Empty value filter](#empty-value-filter) - matches logs without the given [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
- [Any value filter](#any-value-filter) - matches logs with the given non-empty [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
- [Exact filter](#exact-filter) - matches logs with the exact value
- [Exact prefix filter](#exact-prefix-filter) - matches logs starting with the given prefix
- [Multi-exact filter](#multi-exact-filter) - matches logs with one of the specified exact values
- [Case-insensitive filter](#case-insensitive-filter) - matches logs with the given case-insensitive word, phrase or prefix
- [Sequence filter](#sequence-filter) - matches logs with the given sequence of words or phrases
- [Regexp filter](#regexp-filter) - matches logs for the given regexp
- [Range filter](#range-filter) - matches logs with numeric [field values](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in the given range
- [IPv4 range filter](#ipv4-range-filter) - matches logs with ip address [field values](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in the given range
- [String range filter](#string-range-filter) - matches logs with [field values](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in the given string range
- [Length range filter](#length-range-filter) - matches logs with [field values](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) of the given length range
- [Logical filter](#logical-filter) - allows combining other filters


### Time filter

VictoriaLogs scans all the logs per each query if it doesn't contain the filter on [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field).
It uses various optimizations in order to speed up full scan queries without the `_time` filter,
but such queries can be slow if the storage contains large number of logs over long time range. The easiest way to optimize queries
is to narrow down the search with the filter on [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field).

For example, the following query returns [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
ingested into VictoriaLogs during the last hour, which contain the `error` [word](#word):

```logsql
_time:1h AND error
```

The following formats are supported for `_time` filter:

- `_time:duration` matches logs with timestamps on the time range `(now-duration, now]`. Examples:
  - `_time:5m` - returns logs for the last 5 minutes
  - `_time:2.5d15m42.345s` - returns logs for the last 2.5 days, 15 minutes and 42.345 seconds
  - `_time:1y` - returns logs for the last year
- `_time:YYYY-MM-DD` - matches all the logs for the particular day by UTC. For example, `_time:2023-04-25` matches logs on April 25, 2023 by UTC.
- `_time:YYYY-MM` - matches all the logs for the particular month by UTC. For example, `_time:2023-02` matches logs on February, 2023 by UTC.
- `_time:YYYY` - matches all the logs for the particular year by UTC. For example, `_time:2023` matches logs on 2023 by UTC.
- `_time:YYYY-MM-DDTHH` - matches all the logs for the particular hour by UTC. For example, `_time:2023-04-25T22` matches logs on April 25, 2023 at 22 hour by UTC.
- `_time:YYYY-MM-DDTHH:MM` - matches all the logs for the particular minute by UTC. For example, `_time:2023-04-25T22:45` matches logs on April 25, 2023 at 22:45 by UTC.
- `_time:YYYY-MM-DDTHH:MM:SS` - matches all the logs for the particular second by UTC. For example, `_time:2023-04-25T22:45:59` matches logs on April 25, 2023 at 22:45:59 by UTC.
- `_time:[min_time, max_time]` - matches logs on the time range `[min_time, max_time]`, including both `min_time` and `max_time`.
    The `min_time` and `max_time` can contain any format specified [here](https://docs.victoriametrics.com/#timestamp-formats).
    For example, `_time:[2023-04-01, 2023-04-30]` matches logs for the whole April, 2023 by UTC, e.g. it is equivalent to `_time:2023-04`.
- `_time:[min_time, max_time)` - matches logs on the time range `[min_time, max_time)`, not including `max_time`.
    The `min_time` and `max_time` can contain any format specified [here](https://docs.victoriametrics.com/#timestamp-formats).
    For example, `_time:[2023-02-01, 2023-03-01)` matches logs for the whole February, 2023 by UTC, e.g. it is equivalent to `_time:2023-02`.

It is possible to specify time zone offset for all the absolute time formats by appending `+hh:mm` or `-hh:mm` suffix.
For example, `_time:2023-04-25+05:30` matches all the logs on April 25, 2023 by India time zone,
while `_time:2023-02-07:00` matches all the logs on February, 2023 by California time zone.

It is possible to specify generic offset for the selected time range by appending `offset` after the `_time` filter. Examples:

- `_time:5m offset 1h` matches logs on the time range `(now-1h5m, now-1h]`.
- `_time:2023-07 offset 5h30m` matches logs on July, 2023 by UTC with offset 5h30m.
- `_time:[2023-02-01, 2023-03-01) offset 1w` matches logs the week before the time range `[2023-02-01, 2023-03-01)` by UTC.

Performance tips:

- It is recommended specifying the smallest possible time range during the search, since it reduces the amounts of log entries, which need to be scanned during the query.
  For example, `_time:1h` is usually faster than `_time:5h`.

- While LogsQL supports arbitrary number of `_time:...` filters at any level of [logical filters](#logical-filter),
  it is recommended specifying a single `_time` filter at the top level of the query.

- See [other performance tips](#performance-tips).

See also:

- [Stream filter](#stream-filter)
- [Word filter](#word-filter)

### Stream filter

VictoriaLogs provides an optimized way to select log entries, which belong to particular [log streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
This can be done via `_stream:{...}` filter. The `{...}` may contain arbitrary
[Prometheus-compatible label selector](https://docs.victoriametrics.com/keyConcepts.html#filtering)
over fields associated with [log streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
For example, the following query selects [log entries](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
with `app` field equal to `nginx`:

```logsql
_stream:{app="nginx"}
```

This query is equivalent to the following [exact()](#exact-filter) query, but the upper query usually works much faster:

```logsql
app:exact("nginx")
```

Performance tips:

- It is recommended using the most specific `_stream:{...}` filter matching the smallest number of log streams,
  which needs to be scanned by the rest of filters in the query.

- While LogsQL supports arbitrary number of `_stream:{...}` filters at any level of [logical filters](#logical-filter),
  it is recommended specifying a single `_stream:...` filter at the top level of the query.

- See [other performance tips](#performance-tips).

See also:

- [Time filter](#time-filter)
- [Exact filter](#exact-filter)

### Word filter

The simplest LogsQL query consists of a single [word](#word) to search in log messages. For example, the following query matches
[log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) with `error` [word](#word) inside them:

```logsql
error
```

This query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

- `error`
- `an error happened`
- `error: cannot open file`

This query doesn't match the following log messages:

- `ERROR`, since the filter is case-sensitive by default. Use `i(error)` for this case. See [these docs](#case-insensitive-filter) for details.
- `multiple errors occurred`, since the `errors` word doesn't match `error` word. Use `error*` for this case. See [these docs](#prefix-filter) for details.

By default the given [word](#word) is searched in the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the word and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries containing the `error` [word](#word) in the `log.level` field:

```logsql
log.level:error
```

Both the field name and the word in the query can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
поле:значение
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
For example, the following query matches [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
with `ssh: login fail` phrase inside them:

```logsql
"ssh: login fail"
```

This query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

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

By default the given phrase is searched in the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the phrase and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries containing the `cannot open file` phrase in the `event.original` field:

```logsql
event.original:"cannot open file"
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
сообщение:"невозможно открыть файл"
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
For example, the following query returns [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field), which contain [words](#word) with `err` prefix:

```logsql
err*
```

This query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

- `err: foobar`
- `cannot open file: error occurred`

This query doesn't match the following log messages:

- `Error: foobar`, since the `Error` [word](#word) starts with capital letter. Use `i(err*)` for this case. See [these docs](#case-insensitive-filter) for details.
- `fooerror`, since the `fooerror` [word](#word) doesn't start with `err`. Use `re("err")` for this case. See [these docs](#regexp-filter) for details.

Prefix filter can be applied to [phrases](#phrase-filter). For example, the following query matches
[log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) containing phrases with `unexpected fail` prefix:

```logsql
"unexpected fail"*
```

This query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

- `unexpected fail: IO error`
- `error:unexpected failure`

This query doesn't match the following log messages:

- `unexpectedly failed`, since the `unexpectedly` doesn't match `unexpected` [word](#word). Use `unexpected* AND fail*` for this case.
  See [these docs](#logical-filter) for details.
- `failed to open file: unexpected EOF`, since `failed` [word](#word) occurs before the `unexpected` word. Use `unexpected AND fail*` for this case.
  See [these docs](#logical-filter) for details.

By default the prefix filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the prefix filter
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


### Empty value filter

Sometimes it is needed to find log entries without the given [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
This can be performed with `log_field:""` syntax. For example, the following query matches log entries without `host.hostname` field:

```logsql
host.hostname:""
```

See also:

- [Any value filter](#any-value-filter)
- [Word filter](#word-filter)
- [Logical filter](#logical-filter)


### Any value filter

Sometimes it is needed to find log entries containing any non-empty value for the given [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
This can be performed with `log_field:*` syntax. For example, the following query matches log entries with non-empty `host.hostname` field:

```logsql
host.hostname:*
```

See also:

- [Empty value filter](#empty-value-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Exact filter

The [word filter](#word-filter) and [phrase filter](#phrase-filter) return [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
which contain the given word or phrase inside them. The message may contain additional text other than the requested word or phrase. If you need searching for log messages
or [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) with the exact value, then use the `exact(...)` filter.
For example, the following query returns log messages wih the exact value `fatal error: cannot find /foo/bar`:

```logsql
exact("fatal error: cannot find /foo/bar")
```

The query doesn't match the following log messages:

- `fatal error: cannot find /foo/bar/baz` or `some-text fatal error: cannot find /foo/bar`, since they contain an additional text
  other than the specified in the `exact()` filter. Use `"fatal error: cannot find /foo/bar"` query in this case. See [these docs](#phrase-filter) for details.

- `FATAL ERROR: cannot find /foo/bar`, since the `exact()` filter is case-sensitive. Use `i("fatal error: cannot find /foo/bar")` in this case.
  See [these docs](#case-insensitive-filter) for details.

By default the `exact()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the `exact()` filter and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries with the exact `error` value at `log.level` field:

```logsql
log.level:exact("error")
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
log.уровень:exact("ошибка")
```

The field name can be put inside quotes if it contains special chars, which may clash with the query syntax.
For example, the following query matches the `error` value in the field `log:level`:

```logsql
"log:level":exact("error")
```

See also:

- [Exact prefix filter](#exact-prefix-filter)
- [Multi-exact filter](#multi-exact-filter)
- [Word filter](#word-filter)
- [Phrase filter](#phrase-filter)
- [Prefix filter](#prefix-filter)
- [Logical filter](#logical-filter)


### Exact prefix filter

Sometimes it is needed to find log messages starting with some prefix. This can be done with the `exact("prefix"*)` filter.
For example, the following query matches log messages, which start from `Processing request` prefix:

```logsql
exact("Processing request"*)
```

This filter matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

- `Processing request foobar`
- `Processing requests from ...`

It doesn't match the following log messages:

- `processing request foobar`, since the log message starts with lowercase `p`. Use `exact("processing request"*) OR exact("Processing request"*)`
  query in this case. See [these docs](#logical-filter) for details.
- `start: Processing request`, since the log message doesn't start with `Processing request`. Use `"Processing request"` query in this case.
  See [these docs](#phrase-filter) for details.

By default the `exact()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the `exact()` filter and put a colon after it
if it must be searched in the given field. For example, the following query returns log entries with `log.level` field, which starts with `err` prefix:

```logsql
log.level:exact("err"*)
```

Both the field name and the phrase can contain arbitrary [utf-8](https://en.wikipedia.org/wiki/UTF-8)-encoded chars. For example:

```logsql
log.уровень:exact("ошиб"*)
```

The field name can be put inside quotes if it contains special chars, which may clash with the query syntax.
For example, the following query matches `log:level` values starting with `err` prefix:

```logsql
"log:level":exact("err"*)
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
log.level:(exact("error") OR exact("fatal"))
```

While this solution works OK, LogsQL provides simpler and faster solution for this case - the `in()` filter.

```logsql
log.level:in("error", "fatal")
```

It works very fast for long lists passed to `in()`.

The future VictoriaLogs versions will allow passing arbitrary [queries](#query-syntax) into `in()` filter.
For example, the following query selects all the logs for the last hour for users, who visited pages with `admin` [word](#word) in the `path`
during the last day:

```logsql
_time:1h AND user_id:in(_time:1d AND path:admin | fields user_id)
```

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

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

The query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field):

- `unknown error happened`
- `ERROR: cannot read file`
- `Error: unknown arg`
- `An ErRoR occured`

The query doesn't match the following log messages:

- `FooError`, since the `FooError` [word](#word) has superflouos prefix `Foo`. Use `re("(?i)error")` for this case. See [these docs](#regexp-filter) for details.
- `too many Errors`, since the `Errors` [word](#word) has superflouos suffix `s`. Use `i(error*)` for this case.

By default the `i()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the filter
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

Sometimes it is needed to find [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
with [words](#word) or phrases in a particular order. For example, if log messages with `error` word followed by `open file` phrase
must be found, then the following LogsQL query can be used:

```logsql
seq("error", "open file")
```

This query matches `some error: cannot open file /foo/bar` message, since the `open file` phrase goes after the `error` [word](#word).
The query doesn't match the `cannot open file: error` message, since the `open file` phrase is located in front of the `error` [word](#word).
If you need matching log messages with both `error` word and `open file` phrase, then use `error AND "open file"` query. See [these docs](#logical-filter)
for details.

By default the `seq()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the filter
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

LogsQL supports regular expression filter with [re2 syntax](https://github.com/google/re2/wiki/Syntax) via `re(...)` expression.
For example, the following query returns all the log messages containing `err` or `warn` susbstrings:

```logsql
re("err|warn")
```

The query matches the following [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field), which contain either `err` or `warn` substrings:

- `error: cannot read data`
- `2 warnings have been raised`
- `data trasferring finished`

The query doesn't match the following log messages:

- `ERROR: cannot open file`, since the `ERROR` word is in uppercase letters. Use `re("(?i)(err|warn)")` query for case-insensitive regexp search.
  See [these docs](https://github.com/google/re2/wiki/Syntax) for details. See also [case-insenstive filter docs](#case-insensitive-filter).
- `it is warmer than usual`, since it doesn't contain neither `err` nor `warn` substrings.

By default the `re()` filter is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Specify the needed [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the filter
in order to apply it to the given field. For example, the following query matches `event.original` field containing either `err` or `warn` substrings:

```logsql
event.original:re("err|warn")
```

If the field name contains special chars, which may clash with the query syntax, then it may be put into quotes in the query.
For example, the following query matches `event:original` field containing either `err` or `warn` substrings:

```logsql
"event:original":re("err|warn")
```

Performance tips:

- Prefer combining simple [word filter](#word-filter) with [logical filter](#logical-filter) instead of using regexp filter.
  For example, the `re("error|warning")` query can be substituted with `error OR warning` query, which usually works much faster.
  Note that the `re("error|warning")` matches `errors` as well as `warnings` [words](#word), while `error OR warning` matches
  only the specified [words](#word). See also [multi-exact filter](#multi-exact-filter).
- Prefer moving the regexp filter to the end of the [logical filter](#logical-filter), so lightweighter filters are executed first.
- Prefer using `exact("some prefix"*)` instead of `re("^some prefix")`, since the [exact()](#exact-prefix-filter) works much faster than the `re()` filter.
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

The lower and the upper bounds of the range are excluded by default. If they must be included, then substitute the corresponding
parentheses with square brackets. For example:

- `range[1, 10)` includes `1` in the matching range
- `range(1, 10]` includes `10` in the matching range
- `range[1, 10]` includes `1` and `10` in the matching range

Note that the `range()` filter doesn't match [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
with non-numeric values alongside numeric values. For example, `range(1, 10)` doesn't match `the request took 4.2 seconds`
[log message](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field), since the `4.2` number is surrounded by other text.
Extract the numeric value from the message with `parse(_msg, "the request took <request_duration> seconds")` [transformation](#transformations)
and then apply the `range()` [post-filter](#post-filters) to the extracted `request_duration` field.

Performance tips:

- It is better to query pure numeric [field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
  instead of extracting numeric field from text field via [transformations](#transformations) at query time.
- See [other performance tips](#performance-tips).

See also:

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
at `user.ip` [field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model):

```logsql
user.ip:ipv4_range("1.2.3.4")
```

Note that the `ipv4_range()` doesn't match a string with IPv4 address if this string contains other text. For example, `ipv4_range("127.0.0.0/24")`
doesn't match `request from 127.0.0.1: done` [log message](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
since the `127.0.0.1` ip is surrounded by other text. Extract the IP from the message with `parse(_msg, "request from <ip>: done")` [transformation](#transformations)
and then apply the `ipv4_range()` [post-filter](#post-filters) to the extracted `ip` field.

Hints:

- If you need searching for [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) containing the given `X.Y.Z.Q` IPv4 address,
  then `"X.Y.Z.Q"` query can be used. See [these docs](#phrase-filter) for details.
- If you need searching for [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) containing
  at least a single IPv4 address out of the given list, then `"ip1" OR "ip2" ... OR "ipN"` query can be used. See [these docs](#logical-filter) for details.
- If you need finding log entries with `ip` field in multiple ranges, then use `ip:(ipv4_range(range1) OR ipv4_range(range2) ... OR ipv4_range(rangeN))` query.
  See [these docs](#logical-filter) for details.

Performance tips:

- It is better querying pure IPv4 [field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
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

- [Range filter](#range-filter)
- [IPv4 range filter](#ipv4-range-filter)
- [Length range filter](#length-range-filter)
- [Logical filter](#logical-filter)


### Length range filter

If you need to filter log message by its length, then `len_range()` filter can be used.
For example, the following LogsQL query matches [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
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

By default the `len_range()` is applied to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field).
Put the [field name](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) in front of the `len_range()` in order to apply
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
  For example, `error AND file AND app` matches [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
  which simultaneously contain `error`, `file` and `app` [words](#word).
  The `AND` operation is frequently used in LogsQL queries, so it is allowed to skip the `AND` word.
  For example, `error file app` is equivalent to `error AND file AND app`.

- `q1 OR q2` - merges log entries returned by both `q1` and `q2`. Aribtrary number of [filters](#filters) can be combined with `OR` operation.
  For example, `error OR warning OR info` matches [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
  which contain at least one of `error`, `warning` or `info` [words](#word).

- `NOT q` - returns all the log entries except of those which match `q`. For example, `NOT info` returns all the
  [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
  which do not contain `info` [word](#word). The `NOT` operation is frequently used in LogsQL queries, so it is allowed substituting `NOT` with `!` in queries.
  For example, `!info` is equivalent to `NOT info`.

The `NOT` operation has the highest priority, `AND` has the middle priority and `OR` has the lowest priority.
The priority order can be changed with parentheses. For example, `NOT info OR debug` is interpreted as `(NOT info) OR debug`,
so it matches [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
which do not contain `info` [word](#word), while it also matches messages with `debug` word (which may contain the `info` word).
This is not what most users expect. In this case the query can be rewritten to `NOT (info OR debug)`,
which correctly returns log messages without `info` and `debug` [words](#word).

LogsQL supports arbitrary complex logical queries with arbitrary mix of `AND`, `OR` and `NOT` operations and parentheses.

By default logical filters apply to the [`_msg` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
unless the inner filters explicitly specify the needed [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) via `field_name:filter` syntax.
For example, `(error OR warn) AND host.hostname:host123` is interpreted as `(_msg:error OR _msg:warn) AND host.hostname:host123`.

It is possible to specify a single [log field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) for multiple filters
with the following syntax:

```logsql
field_name:(q1 OR q2 OR ... qN)
```

For example, `log.level:error OR log.level:warning OR log.level:info` can be substituted with the shorter query: `log.level:(error OR warning OR info)`.

Performance tips:

- VictoriaLogs executes logical operations from the left to the right, so it is recommended moving the most specific
  and the fastest filters (such as [word filter](#word-filter) and [phrase filter](#phrase-filter)) to the left,
  while moving less specific and the slowest filters (such as [regexp filter](#regexp-filter) and [case-insensitive filter](#case-insensitive-filter))
  to the right. For example, if you need to find [log messages](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field)
  with the `error` word, which match some `/foo/(bar|baz)` regexp,
  it is better from performance PoV to use the query `error re("/foo/(bar|baz)")` instead of `re("/foo/(bar|baz)") error`.

  The most specific filter means that it matches the lowest number of log entries comparing to other filters.

- See [other performance tips](#performance-tips).

## Stream context

LogsQL will support the ability to select the given number of surrounding log lines for the selected log lines
on a [per-stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) basis.

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Transformations

It is possible to perform various transformations on the [selected log entries](#filters) at client side
with `jq`, `awk`, `cut`, etc. Unix commands according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

LogsQL will support the following transformations for the [selected](#filters) log entries:

- Extracting the specified fields from text [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) according to the provided pattern.
- Extracting the specified fields from JSON strings stored inside [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
- Extracting the specified fields from [logfmt](https://brandur.org/logfmt) strings stored
  inside [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
- Creating a new field from existing [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
  according to the provided format.
- Creating a new field according to math calculations over existing [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
- Copying of the existing [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).
- Parsing duration strings into floating-point seconds for further [stats calculations](#stats).
- Creating a boolean field with the result of arbitrary [post-filters](#post-filters) applied to the current fields.
  Boolean fields may be useful for [conditional stats calculation](#stats).
- Creating an integer field with the length of the given field value. This can be useful for [stats calculations](#stats).

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Post-filters

It is possible to perform post-filtering on the [selected log entries](#filters) at client side with `grep` or similar Unix commands
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

LogsQL will support post-filtering on the original [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and fields created by various [transformations](#transformations). The following post-filters will be supported:

- Full-text [filtering](#filters).
- [Logical filtering](#logical-filter).

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Stats

It is possible to perform stats calculations on the [selected log entries](#filters) at client side with `sort`, `uniq`, etc. Unix commands
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

LogsQL will support calculating the following stats based on the [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and fields created by [transformations](#transformations):

- The number of selected logs.
- The number of non-empty values for the given field.
- The number of unique values for the given field.
- The min, max, avg, and sum for the given field.
- The median and [percentile](https://en.wikipedia.org/wiki/Percentile) for the given field.

It will be possible specifying an optional condition [filter](#post-filters) when calculating the stats.
For example, `sumIf(response_size, is_admin:true)` calculates the total response size for admins only.

It will be possible to group stats by the specified [fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and by the specified time buckets.

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Sorting

By default VictoriaLogs sorts the returned results by [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field)
if their total size doesn't exceed `-select.maxSortBufferSize` command-line value (by default it is set to one megabytes).
Otherwise sorting is skipped because of performance and efficiency concerns described [here](https://docs.victoriametrics.com/VictoriaLogs/querying/).

It is possible to sort the [selected log entries](#filters) at client side with `sort` Unix command
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

LogsQL will support results' sorting by the given set of [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Limiters

It is possible to limit the returned results with `head`, `tail`, `less`, etc. Unix commands
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

LogsQL will support the ability to limit the number of returned results alongside the ability to page the returned results.
Additionally, LogsQL will provide the ability to select fields, which must be returned in the response.

See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Querying specific fields

By default VictoriaLogs query response contains [`_msg`](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field),
[`_stream`](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) and
[`_time`](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field) fields.

If you want selecting other fields from the ingested [structured logs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model),
then they must be mentioned in query filters. For example, if you want selecting `log.level` field, and this field isn't mentioned in the query yet, then add
`log.level:*` [filter](#any-value-filter) filter to the end of the query.
The `field_name:*` filter doesn't return log entries with empty or missing `field_name`. If you want returning log entries
with and without the given field, then `(field_name:* OR field_name:"")` filter can be used.
See the following docs for details:

- [Any value filter](#any-value-filter)
- [Empty value filter](#empty-value-filter)
- [Logical filter](#logical-filter)

In the future LogsQL will support `| fields field1, field2, ... fieldN` syntax for selecting the listed fields.
It will also support the ability to select all the fields for the matching log entries with `| fields *` syntax.
See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

## Performance tips

- It is highly recommended specifying [time filter](#time-filter) in order to narrow down the search to specific time range.
- It is highly recommended specifying [stream filter](#stream-filter) in order to narrow down the search
  to specific [log streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
- Move faster filters such as [word filter](#word-filter) and [phrase filter](#phrase-filter) to the beginning of the query.
  This rule doesn't apply to [time filter](#time-filter) and [stream filter](#stream-filter), which can be put at any place of the query.
- Move more specific filters, which match lower number of log entries, to the beginning of the query.
  This rule doesn't apply to [time filter](#time-filter) and [stream filter](#stream-filter), which can be put at any place of the query.
