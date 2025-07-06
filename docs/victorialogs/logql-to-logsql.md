---
weight: 130
title: How to convert Loki queries to VictoriaLogs queries
menu:
  docs:
    parent: "victorialogs"
    weight: 120
tags:
  - logs
  - guide
---

Loki provides [LogQL](https://grafana.com/docs/loki/latest/query/) query language, while VictoriaLogs provides [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/)
query language. Both languages are optimized for querying logs. The docs below show how to convert typical LogQL queries to LogsQL queries.

## Data model

Both Loki and VictoriaLogs support log streams - these are timestamp-ordered streams of logs, where every stream may have its own set of labels. These labels can be used
in [log stream selectors](#log-stream-selector) for quickly narrowing down the amounts of logs for further processing by the query.

The main difference is that VictoriaLogs is optimized for structured logs with big number of labels (aka [wide events](https://jeremymorrell.dev/blog/a-practitioners-guide-to-wide-events/)).
Hundreds of labels per every log entry is OK for VictoriaLogs.

VictoriaLogs is also optimized for log labels with big number of unique values such as `trace_id`, `user_id`, `duration` and `ip` (aka high-cardinality labels).
It is highly recommended storing all the labels as is without the need to pack them into a JSON and storing it into the log line (message).
Storing labels separately results in much faster filtering on such labels (1000x faster and more).
This also results in storage space savings because of better compression for per-label values.

It is recommended reading [VictoriaLogs key concepts](https://docs.victoriametrics.com/victorialogs/keyconcepts/) in order to understand VictoriaLogs data model.

## Log stream selector

The basic practical [LogQL query](https://grafana.com/docs/loki/latest/query/) consists of a [log stream selector](https://grafana.com/docs/loki/latest/query/log_queries/#log-stream-selector),
which returns logs for the matching log streams. For example:

```logql
{app="nginx",host="host-42"}
```

VictoriaLogs supports the same `log streams` concept as Loki does. See [Loki docs about log streams](https://grafana.com/docs/loki/latest/get-started/overview/)
and [VictoriaLogs docs about log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields). That's why log stream selector
in VictoriaLogs looks identical to the log stream selector in Loki:

```logsql
{app="nginx",host="host-42"}
```

Log stream selector is required in Loki query, while it is optional in VictoriaLogs query.

Log stream filters in VictoriaLogs provide additional functionality compared to the log stream selectors in Loki.
Read [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) for more details.

See also [this article](https://itnext.io/why-victorialogs-is-a-better-alternative-to-grafana-loki-7e941567c4d5) for more details.

## Line filter

Loki allows filtering log lines (log messages) with the following filters:

* Substring filter - `{...} |= "some_text"`. It selects logs with lines containing `some_text` substring.
  VictoriaLogs provides similar filters - [word filter](https://docs.victoriametrics.com/victorialogs/logsql/#word-filter)
  and [phrase filter](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter),
  so the similar LogsQL query is `{...} "some_text"`, e.g. it is enough replacing `|=` with a whitespace in order
  to convert LogQL query to LogsQL query.
  A sequence of substring filters - `{...} |= "foo" |= "bar"` - is converted into the following VictoriaLogs query - `{...} "foo" "bar"`.

  There is a subtle difference between substring filter in Loki and word / phrase filter in VictoriaLogs:
  substring filter matches substrings inside words, while word / phrase filters match full words only.
  For example, `{...} |= "error"` in Loki matches `foo error bar`, `foo errors bar` and `foo someerrors bar`,
  while `{...} "error"` in VictoriaLogs matches only `foo error bar`, while it doesn't match other cases which have
  no `error` word. Such cases are very rare in practice. They can be covered with the following VictoriaLogs filters if needed:

  * [Prefix filter](https://docs.victoriametrics.com/victorialogs/logsql/#prefix-filter), which matches word / phrase prefix.
  * [Regexp filter](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter), which matches the given regexp at any position of the log line.

* Negative substring filter - `{...} != "some_text"`. It selects logs with lines without the `some_text` substring.
  This query can be written as `{...} -"some_text"` in VictoriaLogs, e.g. just prepend the `"some_text"` with `-`.
  See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter) for details.

* Regexp filter - `{...} |~ "regexp"`. It selects logs with lines matching the given `regexp`.
  This query can be written as `{...} ~"regexp"` in VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter)
  for details.

* Negative regexp filter - `{...} !~ "regexp"`. It selects logs with lines not matching the given `regexp`.
  This query can be written as `{...} NOT ~"regexp"` in VictoriaLogs according to [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

## Label filter

Loki allows applying filters to log labels with `{...} | label op value` syntax:

* `{...} | label = value` or `{...} | label == value`. This is equivalent to `{...} label:=value` in VictoriaLogs.
  See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter).

* `label != value`. This is equivalent to `{...} -label:=value` in VictoriaLogs. E.g. just add `-` in front of `label:=value`
  according to [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

* `{...} | label > value`, `{...} label >= value`, `{...} label < value` and `{...} label <= value`.
  This is equivalent to `{...} label:>value`, `{...} label:>=value`, `{...} label:<value` and `{...} label:<=value`
  in VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#range-comparison-filter).

* `{...} | label ~= value`. This is equivalent to `{...} label:~value` in VictoriaLogs.
  See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter).

* `{...} | label !~ value`. This is equivalent to `{...} -label:~value` in VictoriaLogs. E.g. just add `-` in front of `label:~value`
  according to [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

Note that VictoriaLogs expects `:` after log labels (field names) in query filters.

Multiple label filters can be combined with `and`, `or` and `(...)` in both Loki and VictoriaLogs.
Additionally, VictoriaLogs supports `not` in front of any filter or combination of filters.
See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter).

## IP filter

Loki provides the ability to filter logs by IP and IP ranges according to [these docs](https://grafana.com/docs/loki/latest/query/ip/).
These filters can be substituted with [`ipv4_range` filter](https://docs.victoriametrics.com/victorialogs/logsql/#ipv4-range-filter) at VictoriaLogs.

## JSON parser

Loki has poor support for high-cardinality labels with big number of unique values, such as `trace_id`, `user_id`, `duration`, etc.
That's why Loki recommends encoding such labels into JSON and storing this JSON as a log line (message). Later, these labels can be parsed
at query time with the `{...} | unpack` or `{...} | json` syntax according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#json),
for further filtering or stats' calculation.

VictoriaLogs supports high-cardinality labels and recommends storing them separately instead of storing them together as a packed JSON in the log message.
This provides the following advantages over Loki:

* Better on-disk compression rate for the ingested logs, so they occupy less storage space.
* Much better query performance over such labels, since VictoriaLogs needs to read only the data for the labels referred in the query,
  while it completely skips the data for the rest of the labels. The performance improvement may reach 1000 times and more on real production logs.

Given these recommendations, a typical Loki query, which includes parsing JSON lines, can be simplified and significantly sped up at VictoriaLogs.
For example, the following Loki query selects logs with the given `trace_id` at log lines:

```logql
{...} | unpack | trace_id == "abcdef"
```

This query is equivalent to the following VictoriaLogs query if the `trace_id` field is stored separately according to the recommendations above:

```logsql
{...} trace_id:=abcdef
```

VictoriaLogs supports parsing JSON inside any label (log field) with the [`unpack_json` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe).
For example, if, despite the recommendations to store every log label separately, you decided to pack labels into a JSON and store it in the log line (message)
in VictoriaLogs, then the following query can be used instead of the query above:

```logsql
{...} | unpack_json fields (trace_id) | trace_id:=abcdef
```

Note that this query will be much slower than the recommended query above (though it should be still faster than the corresponding Loki query :) ).

See [this article](https://itnext.io/why-victorialogs-is-a-better-alternative-to-grafana-loki-7e941567c4d5) for more details.

## Logfmt parser

Loki supports parsing logfmt-formatted log lines with the `{...} | logfmt` syntax according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#pattern).
Such a query can be replaced with `{...} | unpack_logmt` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe).

It is recommended parsing logfmt-formatted structured logs before ingesting them into VictoriaLogs, so log labels are stored separately. VictoriaLogs is optimized for storing logs
with big number of labels (fields), and every such field may contain arbitrary big number of unique values (e.g. VictoriaLogs works great with high-cardinality labels).
See [JSON parser](#json-parser) docs for more details.

## Pattern parser

Loki supports parsing log lines according to the provided pattern with the `{...} | pattern "..."` syntax according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#pattern).
Such a query can be replaced with `{...} | extract "..."` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe).

## Regular expression parser

Loki supports parsing log lines according to the provided regexp with the `{...} | regexp "..."` syntax.
Such a query can be replaced with `{...} | extract_regexp "..."` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#extract_regexp-pipe).

## Line formatting

Loki provides the ability to format log lines with the `{...} | line_format "..."` syntax according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#line-format-expression).
Such a query can be replaced with `{...} | format "..."` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe).

Note that VictoriaLogs uses `<label>` format syntax identical to [pattern parser](#pattern-parser) syntax instead of `{{.label}}` format syntax from Loki.

## Label formatting

Loki provides the ability to format log labels with the `{...} | label_format label_name="..."` syntax according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#labels-format-expression).
Such a query can be replaced with `{...} | format  "..." as label_name` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe).

Note that VictoriaLogs uses `<label>` format syntax identical to [pattern parser](#pattern-parser) syntax instead of `{{.label}}` format syntax from Loki.

## Dropping labels

Loki provides the ability to drop log labels with the `{...} | drop label1, ..., labelN` syntax
according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#drop-labels-expression).
The similar syntax is also supported by VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#delete-pipe).

Loki supports conditional dropping of labels with the `{...} | drop label="value"` syntax.
This can be replaced with [conditional format](https://docs.victoriametrics.com/victorialogs/logsql/#conditional-format) at VictoriaLogs:

```logsql
{...} | format if (label:="value") "" as label
```

## Keeping labels

Loki provides the ability to keep log labels with the `{...} | keep label1, ..., labelN` syntax
according to [these docs](https://grafana.com/docs/loki/latest/query/log_queries/#keep-labels-expression).
The similar syntax is also supported by VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe).

Loki supports conditional keeping of labels with the `{...} | leep label="value"` syntax.
This can be replaced with [conditional format](https://docs.victoriametrics.com/victorialogs/logsql/#conditional-format) at VictoriaLogs:

```logsql
{...} | format if (label:="value") "<label>" as label
```

## Metric queries

Loki allows calculating various stats / metrics from the selected logs with the [metric queries](https://grafana.com/docs/loki/latest/query/metric_queries/).
VictoriaLogs covers all this functionality with [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe), and extends it
with additional [statistical functions](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions) and features:

* Ability to calculate multiple stats over any labels in a single query. For example, the following query calculates the average request duration (from the `duration` label (field)),
  the maximum response size (from the `response_size` label (field)) and the number of request logs in a single query:

  ```logsql
  * | stats
        avg(duration) as avg_duration,
        max(response_size) as max_response_size,
        count() as requests
  ```

* Ability to calculate conditional stats. For example, the following query calculates the number of successful requests (`status=200`) and the total number of requests:

  ```logsql
  * | stats
        count() if (status:=200) as success_requests,
        count() as total_requests
  ```

Let's look at Loki queries, which calculate typical metrics in practice:

### Logs rate

The `rate({...}[d])` query at Loki is substituted with `_time:d {...} | stats by (_stream) rate()` in VictoriaLogs.
See [`_time` filter docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) and [`rate` stats function docs](https://docs.victoriametrics.com/victorialogs/logsql/#rate-stats).

If the logs rate query is used for building hits rate graph in Grafana, then the `_time:d` filter isn't needed in the VictoriaLogs query,
e.g. the `{...} | stats by (_stream) rate()` is enough. It obtains the needed step between dots on the graph from the `step` query arg, which is automatically calculated by Grafana
depending on the selected time range, and sent to VictoriaLogs in the query to [`/select/logsql/stats_query_range`](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats).

By default VictoriaLogs calculates the summary rate over all the matching logs with the `rate()` function. If the rate must be calculated individually per log stream or per some other labels,
then these labels must be enumerated in the `by(...)` clause like shown above. This also means that the frequently used `sum(rate({...}[d]))` query at Loki
can be substituted with the simple `{...} | rate()` query at VictoriaLogs.

### Count the number of logs over time

The `count_over_time({...}[d])` query at Loki is substituted with `_time:d {...} | stats by (_stream) count()` in VictoriaLogs.
See [`_time` filter docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) and [`count` stats function docs](https://docs.victoriametrics.com/victorialogs/logsql/#count-stats).

If the query is used for building hits graph in Grafana, then the `_time:d` filter isn't needed in the VictoriaLogs query,
e.g. the `{...} | stats by (_stream) count()` is enough. It obtains the needed step between dots on the graph from the `step` query arg, which is automatically calculated by Grafana
depending on the selected time range, and sent to VictoriaLogs in the query to [`/select/logsql/stats_query_range`](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats).

By default VictoriaLogs counts all the matching logs with the `count()` function. If logs must be calculated individually per log stream or per some other labels,
then these labels must be enumerated in the `by(...)` clause like shown above. This also means that the frequently used `sum(count_over_time({...}[d]))` query at Loki
can be substituted with the simple `{...} | count()` query at VictoriaLogs.

### Unwrapped range aggregations

Loki allows calculating metrics from label values by using the `func_name({...} | unwrap label_name)` syntax. There is no need in unwrapping any labels in VictoriaLogs -
just pass the needed label names into the needed [`stats` pipe function](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions).

VictoriaLogs aggregates all the selected logs by default, while Loki groups stats by log stream. Use `... | stats by (_stream) ...`
for obtaining results grouped by log stream in VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) for details.

### Topk and bottomk

Loki allows selecting top K metrics with the biggest values via `topk(K, (func_name({...} | unwrap label_name))` syntax.
This query can be translated to `... | first K (label_name desc)` at VictoriaLogs. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#first-pipe).

The `bottomk(K, func_name({...} | unwrap label_name))` query at Loki can be translated to `... | first K (label_name)` at VictoriaLogs.

### Approximate calculations

Loki provides [`approx_topk(K, ...)`](https://grafana.com/docs/loki/latest/query/metric_queries/#probabilistic-aggregation) for probabilistic
selecting up to K metrics with the biggest values. VictoriaLogs provides [`sample` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sample-pipe),
which can be used for probabilistic calculations.

### Arithmetic operators

Loki allows performing math calculations over the calculated metrics with the `a op b` syntax, where `op` can be `+`, `-`, `/` and `*`.
These calculations can be replaced with [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe) at VictoriaLogs.
