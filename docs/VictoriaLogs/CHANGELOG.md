---
sort: 7
weight: 7
title: VictoriaLogs changelog
menu:
  docs:
    identifier: "victorialogs-changelog"
    parent: "victorialogs"
    weight: 7
    title: CHANGELOG
aliases:
- /VictoriaLogs/CHANGELOG.html
---

# VictoriaLogs changelog

The following `tip` changes can be tested by building VictoriaLogs from the latest commit of [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/) repository
according to [these docs](https://docs.victoriametrics.com/victorialogs/quickstart/#building-from-source-code)

## tip

## [v0.21.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.21.0-victorialogs)

Released at 2024-06-20

* FEATURE: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): add a bar chart displaying the number of log entries over a time range. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6404).
* FEATURE: expose `_stream_id` field, which uniquely identifies [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields). This field can be used for quick obtaining of all the logs belonging to a particular stream via [`_stream_id` filter](https://docs.victoriametrics.com/victorialogs/logsql/#_stream_id-filter).

## [v0.20.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.20.2-victorialogs)

Released at 2024-06-18

* BUGFIX: properly parse timestamps with nanosecond precision for logs ingested via [jsonline format](https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api). The bug has been introduced in [v0.20.0 release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.20.0-victorialogs).

## [v0.20.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.20.1-victorialogs)

Released at 2024-06-18

* FEATURE: allow configuring multiple receivers with distinct configs for syslog messages. See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#multiple-configs).

* BUGFIX: properly read syslog messages over TCP and TLS connections according to [RFC5425](https://datatracker.ietf.org/doc/html/rfc5425) when [data ingestion for syslog protocol](https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/) is enabled.

## [v0.20.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.20.0-victorialogs)

Released at 2024-06-17

* FEATURE: add ability to accept logs in [Syslog format](https://en.wikipedia.org/wiki/Syslog). See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/).
* FEATURE: add abitlity to specify timezone offset when parsing [rfc3164](https://datatracker.ietf.org/doc/html/rfc3164) syslog messages with [`unpack_syslog` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe).
* FEATURE: add [`top` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#top-pipe) for returning top N sets of the given fields with the maximum number of matching log entries.

## [v0.19.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.19.0-victorialogs)

Released at 2024-06-11

* FEATURE: do not allow starting the [filter](https://docs.victoriametrics.com/victorialogs/logsql/#filters) with [pipe names](https://docs.victoriametrics.com/victorialogs/logsql/#pipes) and [stats function names](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions). This prevents from unexpected results returned by incorrect queries, which miss mandatory [filter](https://docs.victoriametrics.com/victorialogs/logsql/#query-syntax).
* FEATURE: treat unexpected syslog message as [RFC3164](https://datatracker.ietf.org/doc/html/rfc3164) containing only the `message` field when using [`unpack_syslog` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe).
* FEATURE: allow using `where` prefix instead of `filter` prefix in [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe).
* FEATURE: disallow unescaped `!` char in [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) queries, since it permits writing incorrect query, which may look like correct one. For example, `foo!:bar` instead of `foo:!bar`.
* FEATURE: [web UI](https://docs.victoriametrics.com/VictoriaLogs/querying/#web-ui): add markdown support to the `Group` view. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/6292).

* BUGFIX: return back the improved performance for queries with `*` filters (aka `SELECT *`). This has been broken in [v0.16.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.16.0-victorialogs).

## [v0.18.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.18.0-victorialogs)

Released at 2024-06-06

* FEATURE: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): improve displaying of logs. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/6419) and the following issues: [6408](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6408), [6405](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6405), [6406](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6406) and [6407](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6407).
* FEATURE: add support for [day range filter](https://docs.victoriametrics.com/victorialogs/logsql/#day-range-filter) and [week range filter](https://docs.victoriametrics.com/victorialogs/logsql/#week-range-filter). These filters allow selecting logs on a particular time range per every day or on a particular day of the week.
* FEATURE: allow using `eval` instead of `math` keyword in [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe).

## [v0.17.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.17.0-victorialogs)

Released at 2024-06-05

* FEATURE: add [`pack_logfmt` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#pack_logfmt-pipe) for formatting [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into [logfmt](https://brandur.org/logfmt) messages.
* FEATURE: allow using IPv4 addresses in [range comparison filters](https://docs.victoriametrics.com/victorialogs/logsql/#range-comparison-filter). For example, `ip:>'12.34.56.78'` is valid filter now.
* FEATURE: add `ceil()` and `floor()` functions to [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe).
* FEATURE: add support for bitwise `and`, `or` and `xor` operations at [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe).
* FEATURE: add support for automatic conversion of [RFC3339 time](https://www.rfc-editor.org/rfc/rfc3339) and IPv4 addresses into numeric representation at [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe).
* FEATURE: add ability to format numeric fields into string representation of time, duration and IPv4 with [`format` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe).
* FEATURE: set `format` field to `rfc3164` or `rfc5424` depending on the [Syslog format](https://en.wikipedia.org/wiki/Syslog) parsed via [`unpack_syslog` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe).

* BUGFIX: always respect the limit set in [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe). Previously the limit could be exceeded in some cases.

## [v0.16.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.16.0-victorialogs)

Released at 2024-06-04

* FEATURE: add [`unpack_syslog` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe) for unpacking [syslog](https://en.wikipedia.org/wiki/Syslog) messages from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
* FEATURE: parse timestamps in [`_time` filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) with nanosecond precision.
* FEATURE: return the last `N` matching logs from [`/select/logsql/query` HTTP API](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs) with the maximum timestamps if `limit=N` query arg is passed to it. Previously a random subset of matching logs could be returned, which could complicate investigation of the returned logs.
* FEATURE: add [`drop_empty_fields` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#drop_empty_fields-pipe) for dropping [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with empty values.

## [v0.15.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.15.0-victorialogs)

Released at 2024-05-30

* FEATURE: add [`row_any`](https://docs.victoriametrics.com/victorialogs/logsql/#row_any-stats) function for [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe). This function returns a sample log entry per every calculated [group of results](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-fields).
* FEATURE: add `default` operator to [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe). It allows overriding `NaN` results with the given default value.
* FEATURE: add `exp()` and `ln()` functions to [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe).
* FEATURE: allow omitting result name in [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe) expresions. In this case the result name is automatically set to string representation of the corresponding math expression. For example, `_time:5m | math duration / 1000` is equivalent to `_time:5m | math (duration / 1000) as "duration / 1000"`.
* FEATURE: allow omitting result name in [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe). In this case the result name is automatically set to string representation of the corresponding [stats function expression](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions). For example, `_time:5m | count(*)` is valid [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/) now. It is equivalent to `_time:5m | stats count(*) as "count(*)"`.

* BUGFIX: properly calculate the number of matching rows in `* | field_values x | stats count() rows` and in `* | unroll (x) | stats count() rows` queries.

## [v0.14.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.14.0-victorialogs)

Released at 2024-05-29

* FEATURE: allow specifying fields, which must be packed into JSON in [`pack_json` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#pack_json-pipe) via `pack_json fields (field1, ..., fieldN)` syntax.

* BUGFIX: properly apply `if (...)` filters to calculated results in [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe) when [grouping by fields](https://docs.victoriametrics.com/victorialogs/logsql/#stats-by-fields) is enabled. For example, `_time:5m | stats by (host) count() logs, count() if (error) errors` now properly calculates per-`host` `errors`.

## [v0.13.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.13.0-victorialogs)

Released at 2024-05-28

* FEATURE: add [`extract_regexp` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#extract_regexp-pipe) for extracting arbitrary substrings from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with [RE2 regular expressions](https://github.com/google/re2/wiki/Syntax).
* FEATURE: add [`math` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe) for mathematical calculations over [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
* FEATURE: add [`field_values` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#field_values-pipe), which returns unique values for the given [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
* FEATURE: allow omitting `stats` prefix in [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe). For example, `_time:5m | count() rows` is a valid query now. It is equivalent to `_time:5m | stats count() as rows`.
* FEATURE: allow omitting `filter` prefix in [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe) if the filter doesn't clash with [pipe names](#https://docs.victoriametrics.com/victorialogs/logsql/#pipes). For example, `_time:5m | stats by (host) count() rows | rows:>1000` is a valid query now. It is equivalent to `_time:5m | stats by (host) count() rows | filter rows:>1000`.
* FEATURE: allow [`head` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe) without number. For example, `error | head`. In this case 10 first values are returned as `head` Unix command does by default.
* FEATURE: allow using [comparison filters](https://docs.victoriametrics.com/victorialogs/logsql/#range-comparison-filter) with strings. For example, `some_text_field:>="foo"` matches [log entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with `some_text_field` field values bigger or equal to `foo`.

## [v0.12.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.12.1-victorialogs)

Released at 2024-05-26

* FEATURE: add support for comments in multi-line LogsQL queries. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#comments).

* BUGFIX: properly apply [`in(...)` filter](https://docs.victoriametrics.com/victorialogs/logsql/#multi-exact-filter) inside `if (...)` conditions at various [pipes](https://docs.victoriametrics.com/victorialogs/logsql/#pipes). This bug has been introduced in [v0.12.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.12.0-victorialogs).

## [v0.12.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.12.0-victorialogs)

Released at 2024-05-26

* FEATURE: add [`pack_json` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#pack_json-pipe), which packs all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into a JSON object and stores it into the given field.
* FEATURE: add [`unroll` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unroll-pipe), which can be used for unrolling JSON arrays stored in [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
* FEATURE: add [`replace_regexp` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#replace_regexp-pipe), which allows updating [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) with regular expressions.
* FEATURE: improve performance for [`format`](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe) and [`extract`](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe) pipes.
* FEATURE: improve performance for [`/select/logsql/field_names` HTTP API](https://docs.victoriametrics.com/victorialogs/querying/#querying-field-names).

* BUGFIX: prevent from panic in [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) when VictoriaLogs runs on a system with one CPU core.
* BUGFIX: do not return referenced fields if they weren't present in the original logs. For example, `_time:5m | format if (non_existing_field:"") "abc"` could return empty `non_exiting_field`, while it shouldn't be returned because it is missing in the original logs.
* BUGFIX: properly initialize values for [`in(...)` filter](https://docs.victoriametrics.com/victorialogs/logsql/#multi-exact-filter) inside [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe) if the `in(...)` contains other [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters). For example, `_time:5m | filter ip:in(user_type:admin | fields ip)` now works correctly.

## [v0.11.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.11.0-victorialogs)

Released at 2024-05-25

* FEATURE: add [`replace` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#replace-pipe), which allows replacing substrings in [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
* FEATURE: support [comparing](https://docs.victoriametrics.com/victorialogs/logsql/#range-filter) log field values with [special numeric values](https://docs.victoriametrics.com/victorialogs/logsql/#numeric-values). For example, `duration:>1.5s` and `response_size:<15KiB` are valid filters now.
* FEATURE: properly sort [durations](https://docs.victoriametrics.com/victorialogs/logsql/#duration-values) and [short numeric values](https://docs.victoriametrics.com/victorialogs/logsql/#short-numeric-values) in [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe). For example, `10s` goes in front of `1h`, while `10KB` goes in front of `1GB`.
* FEATURE: add an ability to preserve the original non-empty field values when executing [`extract`](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe), [`unpack_json`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe), [`unpack_logfmt`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe) and [`format`](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe) pipes.
* FEATURE: add an ability to preserve the original field values if the corresponding unpacked values are empty when executing [`extract`](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe), [`unpack_json`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe), [`unpack_logfmt`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe) and [`format`](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe) pipes.

## [v0.10.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.10.0-victorialogs)

Released at 2024-05-24

* FEATURE: return the number of matching log entries per returned value in [HTTP API](https://docs.victoriametrics.com/victorialogs/querying/#http-api) results. This simplifies detecting [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) / [stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) values with the biggest number of logs for the given [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/).
* FEATURE: improve performance for [regexp filter](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter) in the following cases:
  - If the regexp contains just a phrase without special regular expression chars. For example, `~"foo"`.
  - If the regexp starts with `.*` or ends with `.*`. For example, `~".*foo.*"`.
  - If the regexp contains multiple strings delimited by `|`. For example, `~"foo|bar|baz"`.
  - If the regexp contains multiple [words](https://docs.victoriametrics.com/victorialogs/logsql/#word). For example, `~"foo bar baz"`.
* FEATURE: allow disabling automatic unquoting of the matched placeholders in [`extract` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe). See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#format-for-extract-pipe-pattern).

* BUGFIX: properly parse `!` in front of [exact filter](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter), [exact-prefix filter](https://docs.victoriametrics.com/victorialogs/logsql/#exact-prefix-filter) and [regexp filter](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter). For example, `!~"some regexp"` is properly parsed as `not ="some regexp"`. Previously it was incorrectly parsed as `'~="some regexp"'` [phrase filter](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter).
* BUGFIX: properly sort results by [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) when [`limit` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe) is applied. For example, `_time:5m | sort by (_time) desc | limit 10` properly works now.

## [v0.9.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.9.1-victorialogs)

Released at 2024-05-22

* BUGFIX: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): fix loading web UI, which has been broken in [v0.9.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.9.0-victorialogs).

## [v0.9.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.9.0-victorialogs)

Released at 2024-05-22

* FEATURE: allow using `~"some_regexp"` [regexp filter](https://docs.victoriametrics.com/victorialogs/logsql/#regexp-filter) instead of `re("some_regexp")`.
* FEATURE: allow using `="some phrase"` [exact filter](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter) instead of `exact("some phrase")`.
* FEATURE: allow using `="some prefix"*` [exact prefix filter](https://docs.victoriametrics.com/victorialogs/logsql/#exact-prefix-filter) instead of `exact("some prefix"*)`.
* FEATURE: add ability to generate output fields according to the provided format string. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe).
* FEATURE: add ability to extract fields with [`extract` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe) only if the given condition is met. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#conditional-extract).
* FEATURE: add ability to unpack JSON fields with [`unpack_json` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe) only if the given condition is met. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#conditional-unpack_json).
* FEATURE: add ability to unpack [logfmt](https://brandur.org/logfmt) fields with [`unpack_logfmt` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe) only if the given condition is met. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#conditional-unpack_logfmt).
* FEATURE: add [`row_min`](https://docs.victoriametrics.com/victorialogs/logsql/#row_min-stats) and [`row_max`](https://docs.victoriametrics.com/victorialogs/logsql/#row_max-stats) functions for [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe), which allow returning all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) for the log entry with the minimum / maximum value at the given field.
* FEATURE: add `/select/logsql/streams` HTTP endpoint for returning [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) from results of the given query. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-streams) for details.
* FEATURE: add `/select/logsql/stream_field_names` HTTP endpoint for returning [stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field names from results of the given query. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-stream-field-names) for details.
* FEATURE: add `/select/logsql/stream_field_values` HTTP endpoint for returning [stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field values for the given label from results of the given query. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-stream-field-values) for details.
* FEATURE: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): change time range limitation from `_time` in the expression to `start` and `end` query args.

* BUGFIX: fix `invalid memory address or nil pointer dereference` panic when using [`extract`](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe), [`unpack_json`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe) or [`unpack_logfmt`](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe) pipes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6306).
* BUGFIX: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): fix an issue where logs with long `_msg` values might not display. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6281).
* BUGFIX: properly handle time range boundaries with millisecond precision. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6293).

## [v0.8.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.8.0-victorialogs)

Released at 2024-05-20

* FEATURE: add ability to extract JSON fields from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe).
* FEATURE: add ability to extract [logfmt](https://brandur.org/logfmt) fields from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe).
* FEATURE: add ability to extract arbitrary text from [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) into the output fields. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe).
* FEATURE: add ability to put arbitrary [queries](https://docs.victoriametrics.com/victorialogs/logsql/#query-syntax) inside [`in()` filter](https://docs.victoriametrics.com/victorialogs/logsql/#multi-exact-filter).
* FEATURE: add support for post-filtering of query results with [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe).
* FEATURE: allow applying individual [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) per each [stats function](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions). See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#stats-with-additional-filters).
* FEATURE: allow passing string values to [`min`](https://docs.victoriametrics.com/victorialogs/logsql/#min-stats) and [`max`](https://docs.victoriametrics.com/victorialogs/logsql/#max-stats) functions. Previously only numeric values could be passed to them.
* FEATURE: speed up [`sort ... limit N` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) for typical cases.
* FEATURE: allow using more convenient syntax for [`range` filters](https://docs.victoriametrics.com/victorialogs/logsql/#range-filter) if upper or lower bound isn't needed. For example, it is possible to write `response_size:>=10KiB` instead of `response_size:range[10KiB, inf)`, or `temperature:<42` instead of `temperature:range(-inf, 42)`.
* FEATURE: add `/select/logsql/hits` HTTP endpoint for returning the number of matching logs per the given time bucket over the selected time range. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-hits-stats) for details.
* FEATURE: add `/select/logsql/field_names` HTTP endpoint for returning [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) names from results of the given query. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-field-names) for details.
* FEATURE: add `/select/logsql/field_values` HTTP endpoint for returning unique values for the given [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) obtained from results of the given query. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#querying-field-values) for details.

* BUGFIX: properly take into account `offset` at [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) when it already has `limit`. For example, `_time:5m | sort by (foo) offset 20 limit 10`.

## [v0.7.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.7.0-victorialogs)

Released at 2024-05-15

* FEATURE: add support for optional `start` and `end` query args to [HTTP querying API](https://docs.victoriametrics.com/victorialogs/querying/#http-api), which can be used for limiting the time range for [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/).
* FEATURE: add ability to return the first `N` results from [`sort` pipe](#https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe). This is useful when `N` biggest or `N` smallest values must be returned from large amounts of logs.
* FEATURE: add [`quantile`](https://docs.victoriametrics.com/victorialogs/logsql/#quantile-stats) and [`median`](https://docs.victoriametrics.com/victorialogs/logsql/#median-stats) [stats functions](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe).


## [v0.6.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.6.1-victorialogs)

Released at 2024-05-14

* FEATURE: use [natural sort order](https://en.wikipedia.org/wiki/Natural_sort_order) when sorting logs via [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe).

* BUGFIX: properly return matching logs in [streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with small number of entries. Previously they could be skipped. The issue has been introduced in [the release v0.6.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.6.0-victorialogs).
* BUGFIX: fix `runtime error: index out of range` panic when using [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe) like `_time:1h | sort by (_time)`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6258).


## [v0.6.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.6.0-victorialogs)

Released at 2024-05-12

* FEATURE: return all the log fields by default in query results. Previously only [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields), [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) and [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) fields were returned by default.
* FEATURE: add support for returning only the requested log [fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe).
* FEATURE: add support for calculating various stats over [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model). Grouping by arbitrary set of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) is supported. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe) for details.
* FEATURE: add support for sorting the returned results. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe).
* FEATURE: add support for returning unique results. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe).
* FEATURE: add support for limiting the number of returned results. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#limiters).
* FEATURE: add support for copying and renaming the selected log fields. See [these](https://docs.victoriametrics.com/victorialogs/logsql/#copy-pipe) and [these](https://docs.victoriametrics.com/victorialogs/logsql/#rename-pipe) docs.
* FEATURE: allow using `_` inside numbers. For example, `score:range[1_000, 5_000_000]` for [`range` filter](https://docs.victoriametrics.com/victorialogs/logsql/#range-filter).
* FEATURE: allow numbers in hexadecimal and binary form. For example, `response_size:range[0xff, 0b10001101101]` for [`range` filter](https://docs.victoriametrics.com/victorialogs/logsql/#range-filter).
* FEATURE: allow using duration and byte size suffixes in numeric values inside LogsQL queries. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#numeric-values).
* FEATURE: improve data ingestion performance by up to 50%.
* FEATURE: optimize performance for [LogsQL query](https://docs.victoriametrics.com/victorialogs/logsql/), which contains multiple filters for [words](https://docs.victoriametrics.com/victorialogs/logsql/#word-filter) or [phrases](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter) delimited with [`AND` operator](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter). For example, `foo AND bar` query must find [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) with `foo` and `bar` words at faster speed.

* BUGFIX: prevent from possible corruption of short [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) during data ingestion.
* BUGFIX: prevent from additional CPU usage for up to a few seconds after canceling the query.
* BUGFIX: prevent from returning log entries with emtpy `_stream` field in the form `"_stream":""` in [search query results](https://docs.victoriametrics.com/victorialogs/querying/). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6042).

## [v0.5.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.2-victorialogs)

Released at 2024-04-11

* BUGFIX: properly register new [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) under high data ingestion rate. The issue has been introduced in [v0.5.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.0-victorialogs).


## [v0.5.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.1-victorialogs)

Released at 2024-04-04

* BUGFIX: properly apply time range filter for queries containing [`OR` operators](https://docs.victoriametrics.com/victorialogs/logsql/#logical-filter). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5920).
* BUGFIX: do not log debug lines `DEBUG: start trimLines` and `DEBUG: end trimLines`. This bug has been introduced in [v0.5.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.0-victorialogs) in [this commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/0514091948cf8e00e42f44318c0e5e5b63b6388f).

## [v0.5.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.5.0-victorialogs)

Released at 2024-03-01

* FEATURE: support the ability to limit the number of returned log entries from [HTTP querying API](https://docs.victoriametrics.com/victorialogs/querying/#http-api) by passing `limit` query arg. Previously all the matching log entries were returned until closing the response stream. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5674). Thanks to @dmitryk-dk for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5778).

* BUGFIX: do not panic on incorrect regular expression in [stream filter](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter). Thanks to @XLONG96 for [the bugfix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5897).
* BUGFIX: properly determine when the assisted merge is needed. Previously the logs for determining whether the assisted merge is needed was broken. This could lead to too big number of parts under high data ingestion rate. Thanks to @lujiajing1126 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5447).
* BUGFIX: properly stop execution of aborted query when the query doesn't contain [`_stream` filter](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter). Previously such a query could continue consuming resources after being aborted by the client. Thanks to @z-anshun for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5400).

## [v0.4.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.2-victorialogs)

Released at 2023-11-15

* BUGFIX: properly locate logs for the [requested streams](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter). Previously logs for some streams may be missing in query results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4856). Thanks to @XLONG96 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5295)!
* BUGFIX: [web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui): properly sort found logs by time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5300).

## [v0.4.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.1-victorialogs)

Released at 2023-10-04

* BUGFIX: fix the free space verification process in VictoriaLogs that was erroneously shifting to read-only mode, despite there being sufficient free space available. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5112) issue.

## [v0.4.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.0-victorialogs)

Released at 2023-10-03

* FEATURE: add `-elasticsearch.version` command-line flag, which can be used for specifying Elasticsearch version returned by VictoriaLogs to Filebeat at [elasticsearch bulk API](https://docs.victoriametrics.com/victorialogs/data-ingestion/#elasticsearch-bulk-api). This helps resolving [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4777).
* FEATURE: expose the following metrics at [/metrics](https://docs.victoriametrics.com/victorialogs/#monitoring) page:
  * `vl_data_size_bytes{type="storage"}` - on-disk size for data excluding [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) indexes.
  * `vl_data_size_bytes{type="indexdb"}` - on-disk size for [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) indexes.
* FEATURE: add `-insert.maxFieldsPerLine` command-line flag, which can be used for limiting the number of fields per line in logs sent to VictoriaLogs via ingestion protocols. This helps to avoid issues like [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762).
* FEATURE: expose `vl_http_request_duration_seconds` histogram at the [/metrics](https://docs.victoriametrics.com/victorialogs/#monitoring) page. Thanks to @crossoverJie for [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4934).
* FEATURE: add support of `-storage.minFreeDiskSpaceBytes` command-line flag to allow switching to read-only mode when running out of disk space at `-storageDataPath`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4737).

* BUGFIX: fix possible panic when no data is written to VictoriaLogs for a long time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4895). Thanks to @crossoverJie for filing and fixing the issue.
* BUGFIX: add `/insert/loki/ready` endpoint, which is used by Promtail for healthchecks. This should remove `unsupported path requested: /insert/loki/ready` warning logs. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762#issuecomment-1690966722).
* BUGFIX: prevent from panic during background merge when the number of columns in the resulting block exceeds the maximum allowed number of columns per block. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762).

## [v0.3.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.3.0-victorialogs)

Released at 2023-07-20

* FEATURE: add support for data ingestion via Promtail (aka default log shipper for Grafana Loki). See [these](https://docs.victoriametrics.com/victorialogs/data-ingestion/promtail/) and [these](https://docs.victoriametrics.com/victorialogs/data-ingestion/#loki-json-api) docs.

## [v0.2.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.2.0-victorialogs)

Released at 2023-07-17

* FEATURE: support short form of `_time` filters over the last X minutes/hours/days/etc. For example, `_time:5m` is a short form for `_time:(now-5m, now]`, which matches logs with [timestamps](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) for the last 5 minutes. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) for details.
* FEATURE: add ability to specify offset for the selected time range. For example, `_time:5m offset 1h` is equivalent to `_time:(now-5m-1h, now-1h]`. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) for details.
* FEATURE: [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/): replace `exact_prefix("...")` with `exact("..."*)`. This makes it consistent with [i()](https://docs.victoriametrics.com/victorialogs/logsql/#case-insensitive-filter) filter, which can accept phrases and prefixes, e.g. `i("phrase")` and `i("phrase"*)`. See [these docs](https://docs.victoriametrics.com/victorialogs/logsql/#exact-prefix-filter).

## [v0.1.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.1.0-victorialogs)

Released at 2023-06-21

Initial release
