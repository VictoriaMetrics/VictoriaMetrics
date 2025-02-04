---
weight: 23
title: MetricsQL
menu:
  docs:
    parent: 'victoriametrics'
    weight: 23
aliases:
- /ExtendedPromQL.html
- /MetricsQL.html
---
[VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) implements MetricsQL -
query language inspired by [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/).
MetricsQL is backwards-compatible with PromQL, so Grafana dashboards backed by Prometheus datasource should work
the same after switching from Prometheus to VictoriaMetrics.
However, there are some [intentional differences](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) between these two languages.

[Standalone MetricsQL package](https://godoc.org/github.com/VictoriaMetrics/metricsql) can be used for parsing MetricsQL in external apps.

If you are unfamiliar with PromQL, then it is suggested reading [this tutorial for beginners](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085)
and introduction into [basic querying via MetricsQL](https://docs.victoriametrics.com/keyconcepts/#metricsql).

The following functionality is implemented differently in MetricsQL compared to PromQL. This improves user experience:

* MetricsQL takes into account the last [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples) before the lookbehind window
  in square brackets for [increase](#increase) and [rate](#rate) functions. This allows returning the exact results users expect for `increase(metric[$__interval])` queries
  instead of incomplete results Prometheus returns for such queries. Prometheus misses the increase between the last sample before the lookbehind window
  and the first sample inside the lookbehind window.
* MetricsQL doesn't extrapolate [rate](#rate) and [increase](#increase) function results, so it always returns the expected results. For example, it returns
  integer results from `increase()` over slow-changing integer counter. Prometheus in this case returns unexpected fractional results,
  which may significantly differ from the expected results. This addresses [this issue from Prometheus](https://github.com/prometheus/prometheus/issues/3746).
  See technical details about VictoriaMetrics and Prometheus calculations for [rate](#rate)
  and [increase](#increase) [in this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1215#issuecomment-850305711).
* MetricsQL returns the expected non-empty responses for [rate](#rate) function when Grafana or [vmui](https://docs.victoriametrics.com/#vmui)
  passes `step` values smaller than the interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
  to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).
  This addresses [this issue from Grafana](https://github.com/grafana/grafana/issues/11451).
  See also [this blog post](https://www.percona.com/blog/2020/02/28/better-prometheus-rate-function-with-victoriametrics/).
* MetricsQL treats `scalar` type the same as `instant vector` without labels, since subtle differences between these types usually confuse users.
  See [the corresponding Prometheus docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#expression-language-data-types) for details.
* MetricsQL removes all the `NaN` values from the output, so some queries like `(-1)^0.5` return empty results in VictoriaMetrics,
  while returning a series of `NaN` values in Prometheus. Note that Grafana doesn't draw any lines or dots for `NaN` values,
  so the end result looks the same for both VictoriaMetrics and Prometheus.
* MetricsQL keeps metric names after applying functions, which don't change the meaning of the original time series.
  For example, [min_over_time(foo)](#min_over_time) or [round(foo)](#round) leaves `foo` metric name in the result.
  See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/674) for details.

Read more about the differences between PromQL and MetricsQL in [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e).

Other PromQL functionality should work the same in MetricsQL.
[File an issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you notice discrepancies between PromQL and MetricsQL results other than mentioned above.

## MetricsQL features

MetricsQL implements [PromQL](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085)
and provides additional functionality mentioned below, which is aimed towards solving practical cases.
Feel free [filing a feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you think MetricsQL misses certain useful functionality.

This functionality can be evaluated at [VictoriaMetrics playground](https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/prometheus/graph/)
or at your own [VictoriaMetrics instance](https://docs.victoriametrics.com/#how-to-start-victoriametrics).

The list of MetricsQL features on top of PromQL:

* Graphite-compatible filters can be passed via `{__graphite__="foo.*.bar"}` syntax.
  See [these docs](https://docs.victoriametrics.com/#selecting-graphite-metrics).
  VictoriaMetrics can be used as Graphite datasource in Grafana. See [these docs](https://docs.victoriametrics.com/#graphite-api-usage) for details.
  See also [label_graphite_group](#label_graphite_group) function, which can be used for extracting the given groups from Graphite metric name.
* Lookbehind window in square brackets for [rollup functions](#rollup-functions) may be omitted. VictoriaMetrics automatically selects the lookbehind window
  depending on the `step` query arg passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query)
  and the real interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) (aka `scrape_interval`).
  For instance, the following query is valid in VictoriaMetrics: `rate(node_network_receive_bytes_total)`.
  It is roughly equivalent to `rate(node_network_receive_bytes_total[$__interval])` when used in Grafana.
  The difference is documented in [rate() docs](#rate).
* Numeric values can contain `_` delimiters for better readability. For example, `1_234_567_890` can be used in queries instead of `1234567890`.
* [Series selectors](https://docs.victoriametrics.com/keyconcepts/#filtering) accept multiple `or` filters. For example, `{env="prod",job="a" or env="dev",job="b"}`
  selects series with `{env="prod",job="a"}` or `{env="dev",job="b"}` labels.
  See [these docs](https://docs.victoriametrics.com/keyconcepts/#filtering-by-multiple-or-filters) for details.
* Support for matching against multiple numeric constants via `q == (C1, ..., CN)` and `q != (C1, ..., CN)` syntax. For example, `status_code == (300, 301, 304)`
  returns `status_code` metrics with one of `300`, `301` or `304` values.
* Support for `group_left(*)` and `group_right(*)` for copying all the labels from time series on the `one` side
  of [many-to-one operations](https://prometheus.io/docs/prometheus/latest/querying/operators/#many-to-one-and-one-to-many-vector-matches).
  The copied label names may clash with the existing label names, so MetricsQL provides an ability to add prefix to the copied metric names
  via `group_left(*) prefix "..."` syntax.
  For example, the following query copies all the `namespace`-related labels from `kube_namespace_labels` to `kube_pod_info` series,
  while adding `ns_` prefix to the copied labels: `kube_pod_info * on(namespace) group_left(*) prefix "ns_" kube_namespace_labels`.
  Labels from the `on()` list aren't copied.
* [Aggregate functions](#aggregate-functions) accept arbitrary number of args.
  For example, `avg(q1, q2, q3)` would return the average values for every point across time series returned by `q1`, `q2` and `q3`.
* [@ modifier](https://prometheus.io/docs/prometheus/latest/querying/basics/#modifier) can be put anywhere in the query.
  For example, `sum(foo) @ end()` calculates `sum(foo)` at the `end` timestamp of the selected time range `[start ... end]`.
* Arbitrary subexpression can be used as [@ modifier](https://prometheus.io/docs/prometheus/latest/querying/basics/#modifier).
  For example, `foo @ (end() - 1h)` calculates `foo` at the `end - 1 hour` timestamp on the selected time range `[start ... end]`.
* [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier), lookbehind window in square brackets
  and `step` value for [subquery](#subqueries) may refer to the current step aka `$__interval` value from Grafana with `[Ni]` syntax.
  For instance, `rate(metric[10i] offset 5i)` would return per-second rate over a range covering 10 previous steps with the offset of 5 steps.
* [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier) may be put anywhere in the query. For instance, `sum(foo) offset 24h`.
* Lookbehind window in square brackets and [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier) may be fractional.
  For instance, `rate(node_network_receive_bytes_total[1.5m] offset 0.5d)`.
* The duration suffix is optional. The duration is in seconds if the suffix is missing.
  For example, `rate(m[300] offset 1800)` is equivalent to `rate(m[5m]) offset 30m`.
* The duration can be placed anywhere in the query. For example, `sum_over_time(m[1h]) / 1h` is equivalent to `sum_over_time(m[1h]) / 3600`.
* Numeric values can have `K`, `Ki`, `M`, `Mi`, `G`, `Gi`, `T` and `Ti` suffixes. For example, `8K` is equivalent to `8000`, while `1.2Mi` is equivalent to `1.2*1024*1024`.
* Trailing commas on all the lists are allowed - label filters, function args and with expressions.
  For instance, the following queries are valid: `m{foo="bar",}`, `f(a, b,)`, `WITH (x=y,) x`.
  This simplifies maintenance of multi-line queries.
* Metric names and label names may contain any unicode letter. For example `ტემპერატურა{πόλη="Київ"}` is a valid MetricsQL expression.
* Metric names and labels names may contain escaped chars. For example, `foo\-bar{baz\=aa="b"}` is valid expression.
  It returns time series with name `foo-bar` containing label `baz=aa` with value `b`.
  Additionally, the following escape sequences are supported:
  - `\xXX`, where `XX` is hexadecimal representation of the escaped ascii char.
  - `\uXXXX`, where `XXXX` is a hexadecimal representation of the escaped unicode char.
* Aggregate functions support optional `limit N` suffix in order to limit the number of output series.
  For example, `sum(x) by (y) limit 3` limits the number of output time series after the aggregation to 3.
  All the other time series are dropped.
* [histogram_quantile](#histogram_quantile) accepts optional third arg - `boundsLabel`.
  In this case it returns `lower` and `upper` bounds for the estimated percentile.
  See [this issue for details](https://github.com/prometheus/prometheus/issues/5706).
* `default` binary operator. `q1 default q2` fills gaps in `q1` with the corresponding values from `q2`. See also [drop_empty_series](#drop_empty_series).
* `if` binary operator. `q1 if q2` removes values from `q1` for missing values from `q2`.
* `ifnot` binary operator. `q1 ifnot q2` removes values from `q1` for existing values from `q2`.
* `WITH` templates. This feature simplifies writing and managing complex queries.
  Go to [WITH templates playground](https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/expand-with-exprs) and try it.
* String literals may be concatenated. This is useful with `WITH` templates:
  `WITH (commonPrefix="long_metric_prefix_") {__name__=commonPrefix+"suffix1"} / {__name__=commonPrefix+"suffix2"}`.
* `keep_metric_names` modifier can be applied to all the [rollup functions](#rollup-functions), [transform functions](#transform-functions)
  and [binary operators](https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operators).
  This modifier prevents from dropping metric names in function results. See [these docs](#keep_metric_names).

## keep_metric_names

By default, metric names are dropped after applying functions or [binary operators](https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operators),
since they may change the meaning of the original time series.
This may result in `duplicate time series` error when the function is applied to multiple time series with different names.
This error can be fixed by applying `keep_metric_names` modifier to the function or binary operator.

For example:
- `rate({__name__=~"foo|bar"}) keep_metric_names` leaves `foo` and `bar` metric names in the returned time series.
- `({__name__=~"foo|bar"} / 10) keep_metric_names` leaves `foo` and `bar` metric names in the returned time series.

## MetricsQL functions

If you are unfamiliar with PromQL, then please read [this tutorial](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) at first.

MetricsQL provides the following functions:

* [Rollup functions](#rollup-functions)
* [Transform functions](#transform-functions)
* [Label manipulation functions](#label-manipulation-functions)
* [Aggregate functions](#aggregate-functions)

### Rollup functions

**Rollup functions** (aka range functions or window functions) calculate rollups over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window for the [selected time series](https://docs.victoriametrics.com/keyconcepts/#filtering).
For example, `avg_over_time(temperature[24h])` calculates the average temperature over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) for the last 24 hours.

Additional details:

* If rollup functions are used for building graphs in Grafana, then the rollup is calculated independently per each point on the graph.
  For example, every point for `avg_over_time(temperature[24h])` graph shows the average temperature for the last 24 hours ending at this point.
  The interval between points is set as `step` query arg passed by Grafana to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).
* If the given [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) returns multiple time series,
  then rollups are calculated individually per each returned series.
* If lookbehind window in square brackets is missing, then it is automatically set to the following value:
  - To `step` value passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) or [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query)
    for all the [rollup functions](#rollup-functions) except of [default_rollup](#default_rollup) and [rate](#rate). This value is known as `$__interval` in Grafana or `1i` in MetricsQL.
    For example, `avg_over_time(temperature)` is automatically transformed to `avg_over_time(temperature[1i])`.
  - To the `max(step, scrape_interval)`, where `scrape_interval` is the interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
    for [default_rollup](#default_rollup) and [rate](#rate) functions. This allows avoiding unexpected gaps on the graph when `step` is smaller than `scrape_interval`.
* Every [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) in MetricsQL must be wrapped into a rollup function.
  Otherwise, it is automatically wrapped into [default_rollup](#default_rollup). For example, `foo{bar="baz"}`
  is automatically converted to `default_rollup(foo{bar="baz"})` before performing the calculations.
* If something other than [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) is passed to rollup function,
  then the inner arg is automatically converted to a [subquery](#subqueries).
* All the rollup functions accept optional `keep_metric_names` modifier. If it is set, then the function keeps metric names in results.
  See [these docs](#keep_metric_names).

See also [implicit query conversions](#implicit-query-conversions).

The list of supported rollup functions:

#### absent_over_time

`absent_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns 1
if the given lookbehind window `d` doesn't contain [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples). Otherwise, it returns an empty result.

This function is supported by PromQL.

See also [present_over_time](#present_over_time).

#### aggr_over_time

`aggr_over_time(("rollup_func1", "rollup_func2", ...), series_selector[d])` is a [rollup function](#rollup-functions),
which calculates all the listed `rollup_func*` for [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d`.
The calculations are performed individually per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

`rollup_func*` can contain any rollup function. For instance, `aggr_over_time(("min_over_time", "max_over_time", "rate"), m[d])`
would calculate [min_over_time](#min_over_time), [max_over_time](#max_over_time) and [rate](#rate) for `m[d]`.

#### ascent_over_time

`ascent_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates
ascent of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples) values on the given lookbehind window `d`. The calculations are performed individually
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is useful for tracking height gains in GPS tracking. Metric names are stripped from the resulting rollups.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [descent_over_time](#descent_over_time).

#### avg_over_time

`avg_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the average value
over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [median_over_time](#median_over_time), [min_over_time](#min_over_time) and [max_over_time](#max_over_time).

#### changes

`changes(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number of times
the [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) changed on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Unlike `changes()` in Prometheus it takes into account the change from the last sample before the given lookbehind window `d`.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [changes_prometheus](#changes_prometheus).

#### changes_prometheus

`changes_prometheus(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number of times
the [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) changed on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

It doesn't take into account the change from the last sample before the given lookbehind window `d` in the same way as Prometheus does.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [changes](#changes).

#### count_eq_over_time

`count_eq_over_time(series_selector[d], eq)` is a [rollup function](#rollup-functions), which calculates the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which are equal to `eq`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_over_time](#count_over_time), [share_eq_over_time](#share_eq_over_time) and [count_values_over_time](#count_values_over_time).

#### count_gt_over_time

`count_gt_over_time(series_selector[d], gt)` is a [rollup function](#rollup-functions), which calculates the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which are bigger than `gt`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_over_time](#count_over_time) and [share_gt_over_time](#share_gt_over_time).

#### count_le_over_time

`count_le_over_time(series_selector[d], le)` is a [rollup function](#rollup-functions), which calculates the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which don't exceed `le`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_over_time](#count_over_time) and [share_le_over_time](#share_le_over_time).

#### count_ne_over_time

`count_ne_over_time(series_selector[d], ne)` is a [rollup function](#rollup-functions), which calculates the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which aren't equal to `ne`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_over_time](#count_over_time) and [count_eq_over_time](#count_eq_over_time).

#### count_over_time

`count_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [count_le_over_time](#count_le_over_time), [count_gt_over_time](#count_gt_over_time), [count_eq_over_time](#count_eq_over_time) and [count_ne_over_time](#count_ne_over_time).

#### count_values_over_time

`count_values_over_time("label", series_selector[d])` is a [rollup function](#rollup-functions), which counts the number of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
with the same value over the given lookbehind window and stores the counts in a time series with an additional `label`, which contains each initial value.
The results are calculated independently per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_eq_over_time](#count_eq_over_time), [count_values](#count_values) and [distinct_over_time](#distinct_over_time) and [label_match](#label_match).

#### decreases_over_time

`decreases_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
value decreases over the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [increases_over_time](#increases_over_time).

#### default_rollup

`default_rollup(series_selector[d])` is a [rollup function](#rollup-functions), which returns the last [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
Compared to [last_over_time](#last_over_time) it accounts for [staleness markers](https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers) to detect stale series.

If the lookbehind window is skipped in square brackets, then it is automatically calculated as `max(step, scrape_interval)`, where `step` is the query arg value
passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) or [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query),
while `scrape_interval` is the interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) for the selected time series.
This allows avoiding unexpected gaps on the graph when `step` is smaller than the `scrape_interval`.

#### delta

`delta(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the difference between
the last sample before the given lookbehind window `d` and the last sample at the given lookbehind window `d`
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

The behaviour of `delta()` function in MetricsQL is slightly different to the behaviour of `delta()` function in Prometheus.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [increase](#increase) and [delta_prometheus](#delta_prometheus).

#### delta_prometheus

`delta_prometheus(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the difference between
the first and the last samples at the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

The behaviour of `delta_prometheus()` is close to the behaviour of `delta()` function in Prometheus.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [delta](#delta).

#### deriv

`deriv(series_selector[d])` is a [rollup function](#rollup-functions), which calculates per-second derivative over the given lookbehind window `d`
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
The derivative is calculated using linear regression.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [deriv_fast](#deriv_fast) and [ideriv](#ideriv).

#### deriv_fast

`deriv_fast(series_selector[d])` is a [rollup function](#rollup-functions), which calculates per-second derivative
using the first and the last [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [deriv](#deriv) and [ideriv](#ideriv).

#### descent_over_time

`descent_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates descent of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values on the given lookbehind window `d`. The calculations are performed individually per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is useful for tracking height loss in GPS tracking.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [ascent_over_time](#ascent_over_time).

#### distinct_over_time

`distinct_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the number of unique [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [count_values_over_time](#count_values_over_time).

#### duration_over_time

`duration_over_time(series_selector[d], max_interval)` is a [rollup function](#rollup-functions), which returns the duration in seconds
when time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering) were present
over the given lookbehind window `d`. It is expected that intervals between adjacent samples per each series don't exceed the `max_interval`.
Otherwise, such intervals are considered as gaps and aren't counted.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [lifetime](#lifetime) and [lag](#lag).

#### first_over_time

`first_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the first [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

See also [last_over_time](#last_over_time) and [tfirst_over_time](#tfirst_over_time).

#### geomean_over_time

`geomean_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates [geometric mean](https://en.wikipedia.org/wiki/Geometric_mean)
over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### histogram_over_time

`histogram_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates
[VictoriaMetrics histogram](https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram) over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`. It is calculated individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
The resulting histograms are useful to pass to [histogram_quantile](#histogram_quantile) for calculating quantiles
over multiple [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).
For example, the following query calculates median temperature by country over the last 24 hours:

`histogram_quantile(0.5, sum(histogram_over_time(temperature[24h])) by (vmrange,country))`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### hoeffding_bound_lower

`hoeffding_bound_lower(phi, series_selector[d])` is a [rollup function](#rollup-functions), which calculates
lower [Hoeffding bound](https://en.wikipedia.org/wiki/Hoeffding%27s_inequality) for the given `phi` in the range `[0...1]`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [hoeffding_bound_upper](#hoeffding_bound_upper).

#### hoeffding_bound_upper

`hoeffding_bound_upper(phi, series_selector[d])` is a [rollup function](#rollup-functions), which calculates
upper [Hoeffding bound](https://en.wikipedia.org/wiki/Hoeffding%27s_inequality) for the given `phi` in the range `[0...1]`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [hoeffding_bound_lower](#hoeffding_bound_lower).

#### holt_winters

`holt_winters(series_selector[d], sf, tf)` is a [rollup function](#rollup-functions), which calculates Holt-Winters value
(aka [double exponential smoothing](https://en.wikipedia.org/wiki/Exponential_smoothing#Double_exponential_smoothing)) for [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
over the given lookbehind window `d` using the given smoothing factor `sf` and the given trend factor `tf`.
Both `sf` and `tf` must be in the range `[0...1]`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [range_linear_regression](#range_linear_regression).

#### idelta

`idelta(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the difference between the last two [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [delta](#delta).

#### ideriv

`ideriv(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the per-second derivative based
on the last two [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
over the given lookbehind window `d`. The derivative is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [deriv](#deriv).

#### increase

`increase(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the increase over the given lookbehind window `d`
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Unlike Prometheus, it takes into account the last sample before the given lookbehind window `d` when calculating the result.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

This function is supported by PromQL.

See also [increase_pure](#increase_pure), [increase_prometheus](#increase_prometheus) and [delta](#delta).

#### increase_prometheus

`increase_prometheus(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the increase
over the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
It doesn't take into account the last sample before the given lookbehind window `d` when calculating the result in the same way as Prometheus does.
See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

See also [increase_pure](#increase_pure) and [increase](#increase).

#### increase_pure

`increase_pure(series_selector[d])` is a [rollup function](#rollup-functions), which works the same as [increase](#increase) except
of the following corner case - it assumes that [counters](https://docs.victoriametrics.com/keyconcepts/#counter) always start from 0,
while [increase](#increase) ignores the first value in a series if it is too big.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

See also [increase](#increase) and [increase_prometheus](#increase_prometheus).

#### increases_over_time

`increases_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
value increases over the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [decreases_over_time](#decreases_over_time).

#### integrate

`integrate(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the integral over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### irate

`irate(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the "instant" per-second increase rate over
the last two [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

This function is supported by PromQL.

See also [rate](#rate) and [rollup_rate](#rollup_rate).

#### lag

`lag(series_selector[d])` is a [rollup function](#rollup-functions), which returns the duration in seconds between the last sample
on the given lookbehind window `d` and the timestamp of the current point. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [lifetime](#lifetime) and [duration_over_time](#duration_over_time).

#### last_over_time

`last_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the last [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is supported by PromQL.

See also [first_over_time](#first_over_time) and [tlast_over_time](#tlast_over_time).

#### lifetime

`lifetime(series_selector[d])` is a [rollup function](#rollup-functions), which returns the duration in seconds between the last and the first sample
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [duration_over_time](#duration_over_time) and [lag](#lag).

#### mad_over_time

`mad_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates [median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation)
over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [mad](#mad), [range_mad](#range_mad) and [outlier_iqr_over_time](#outlier_iqr_over_time).

#### max_over_time

`max_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the maximum value over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [tmax_over_time](#tmax_over_time) and [min_over_time](#min_over_time).

#### median_over_time

`median_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates median value over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [avg_over_time](#avg_over_time).

#### min_over_time

`min_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the minimum value over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [tmin_over_time](#tmin_over_time) and [max_over_time](#max_over_time).

#### mode_over_time

`mode_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates [mode](https://en.wikipedia.org/wiki/Mode_(statistics))
for [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d`. It is calculated individually per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering). It is expected that [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values are discrete.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### outlier_iqr_over_time

`outlier_iqr_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the last sample on the given lookbehind window `d`
if its value is either smaller than the `q25-1.5*iqr` or bigger than `q75+1.5*iqr` where:
- `iqr` is an [Interquartile range](https://en.wikipedia.org/wiki/Interquartile_range) over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the lookbehind window `d`
- `q25` and `q75` are 25th and 75th [percentiles](https://en.wikipedia.org/wiki/Percentile) over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the lookbehind window `d`.

The `outlier_iqr_over_time()` is useful for detecting anomalies in gauge values based on the previous history of values.
For example, `outlier_iqr_over_time(memory_usage_bytes[1h])` triggers when `memory_usage_bytes` suddenly goes outside the usual value range for the last hour.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [outliers_iqr](#outliers_iqr).

#### predict_linear

`predict_linear(series_selector[d], t)` is a [rollup function](#rollup-functions), which calculates the value `t` seconds in the future using
linear interpolation over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d`.
The predicted value is calculated individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is supported by PromQL.

See also [range_linear_regression](#range_linear_regression).

#### present_over_time

`present_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns 1 if there is at least a single [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`. Otherwise, an empty result is returned.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### quantile_over_time

`quantile_over_time(phi, series_selector[d])` is a [rollup function](#rollup-functions), which calculates `phi`-quantile over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
The `phi` value must be in the range `[0...1]`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [quantiles_over_time](#quantiles_over_time).

#### quantiles_over_time

`quantiles_over_time("phiLabel", phi1, ..., phiN, series_selector[d])` is a [rollup function](#rollup-functions), which calculates `phi*`-quantiles
over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
The function returns individual series per each `phi*` with `{phiLabel="phi*"}` label. `phi*` values must be in the range `[0...1]`.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [quantile_over_time](#quantile_over_time).

#### range_over_time

`range_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates value range over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).
E.g. it calculates `max_over_time(series_selector[d]) - min_over_time(series_selector[d])`.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### rate

`rate(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the average per-second increase rate
over the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

If the lookbehind window is skipped in square brackets, then it is automatically calculated as `max(step, scrape_interval)`, where `step` is the query arg value
passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) or [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query),
while `scrape_interval` is the interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) for the selected time series.
This allows avoiding unexpected gaps on the graph when `step` is smaller than the `scrape_interval`.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [irate](#irate) and [rollup_rate](#rollup_rate).

#### rate_over_sum

`rate_over_sum(series_selector[d])` is a [rollup function](#rollup-functions), which calculates per-second rate over the sum of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`. The calculations are performed individually per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

#### resets

`resets(series_selector[d])` is a [rollup function](#rollup-functions), which returns the number
of [counter](https://docs.victoriametrics.com/keyconcepts/#counter) resets over the given lookbehind window `d`
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

This function is supported by PromQL.

#### rollup

`rollup(series_selector[d])` is a [rollup function](#rollup-functions), which calculates `min`, `max` and `avg` values for [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
These values are calculated individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [rollup_rate](#rollup_rate).

#### rollup_candlestick

`rollup_candlestick(series_selector[d])` is a [rollup function](#rollup-functions), which calculates `open`, `high`, `low` and `close` values (aka OHLC)
over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` and returns them in time series
with `rollup="open"`, `rollup="high"`, `rollup="low"` and `rollup="close"` additional labels.
The calculations are performed individually per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering). This function is useful for financial applications.

Optional 2nd argument `"open"`, `"high"` or `"low"` or `"close"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### rollup_delta

`rollup_delta(series_selector[d])` is a [rollup function](#rollup-functions), which calculates differences between adjacent [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated differences
and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
The calculations are performed individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [rollup_increase](#rollup_increase).

#### rollup_deriv

`rollup_deriv(series_selector[d])` is a [rollup function](#rollup-functions), which calculates per-second derivatives
for adjacent [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` and returns `min`, `max` and `avg` values
for the calculated per-second derivatives and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
The calculations are performed individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [rollup](#rollup) and [rollup_rate](#rollup_rate).

#### rollup_increase

`rollup_increase(series_selector[d])` is a [rollup function](#rollup-functions), which calculates increases for adjacent [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated increases
and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
The calculations are performed individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names. See also [rollup_delta](#rollup_delta).

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

See also [rollup](#rollup) and [rollup_rate](#rollup_rate).

#### rollup_rate

`rollup_rate(series_selector[d])` is a [rollup function](#rollup-functions), which calculates per-second change rates
for adjacent [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated per-second change rates
and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
The calculations are performed individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

See [this article](https://valyala.medium.com/why-irate-from-prometheus-doesnt-capture-spikes-45f9896d7832) in order to understand better
when to use `rollup_rate()`.

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [counters](https://docs.victoriametrics.com/keyconcepts/#counter).

See also [rollup](#rollup) and [rollup_increase](#rollup_increase).

#### rollup_scrape_interval

`rollup_scrape_interval(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the interval in seconds between
adjacent [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated interval
and returns them in time series with `rollup="min"`, `rollup="max"` and `rollup="avg"` additional labels.
The calculations are performed individually per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Optional 2nd argument `"min"`, `"max"` or `"avg"` can be passed to keep only one calculation result and without adding a label.
See also [label_match](#label_match).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names. See also [scrape_interval](#scrape_interval).

#### scrape_interval

`scrape_interval(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the average interval in seconds
between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [rollup_scrape_interval](#rollup_scrape_interval).

#### share_gt_over_time

`share_gt_over_time(series_selector[d], gt)` is a [rollup function](#rollup-functions), which returns share (in the range `[0...1]`)
of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which are bigger than `gt`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is useful for calculating SLI and SLO. Example: `share_gt_over_time(up[24h], 0)` - returns service availability for the last 24 hours.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [share_le_over_time](#share_le_over_time) and [count_gt_over_time](#count_gt_over_time).

#### share_le_over_time

`share_le_over_time(series_selector[d], le)` is a [rollup function](#rollup-functions), which returns share (in the range `[0...1]`)
of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which are smaller or equal to `le`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

This function is useful for calculating SLI and SLO. Example: `share_le_over_time(memory_usage_bytes[24h], 100*1024*1024)` returns
the share of time series values for the last 24 hours when memory usage was below or equal to 100MB.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [share_gt_over_time](#share_gt_over_time) and [count_le_over_time](#count_le_over_time).

#### share_eq_over_time

`share_eq_over_time(series_selector[d], eq)` is a [rollup function](#rollup-functions), which returns share (in the range `[0...1]`)
of [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d`, which are equal to `eq`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [count_eq_over_time](#count_eq_over_time).

#### stale_samples_over_time

`stale_samples_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the number
of [staleness markers](https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers) on the given lookbehind window `d`
per each time series matching the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

#### stddev_over_time

`stddev_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates standard deviation over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [stdvar_over_time](#stdvar_over_time).

#### stdvar_over_time

`stdvar_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates standard variance over [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

See also [stddev_over_time](#stddev_over_time).

#### sum_eq_over_time

`sum_eq_over_time(series_selector[d], eq)` is a [rollup function](#rollup-functions), which calculates the sum of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values equal to `eq` on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [sum_over_time](#sum_over_time) and [count_eq_over_time](#count_eq_over_time).

#### sum_gt_over_time

`sum_gt_over_time(series_selector[d], gt)` is a [rollup function](#rollup-functions), which calculates the sum of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values bigger than `gt` on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [sum_over_time](#sum_over_time) and [count_gt_over_time](#count_gt_over_time).

#### sum_le_over_time

`sum_le_over_time(series_selector[d], le)` is a [rollup function](#rollup-functions), which calculates the sum of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values smaller or equal to `le` on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [sum_over_time](#sum_over_time) and [count_le_over_time](#count_le_over_time).

#### sum_over_time

`sum_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the sum of [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples) values
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

This function is supported by PromQL.

#### sum2_over_time

`sum2_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which calculates the sum of squares for [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
values on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

#### timestamp

`timestamp(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision
for the last [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [time](#time) and [now](#now).

#### timestamp_with_name

`timestamp_with_name(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision
for the last [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are preserved in the resulting rollups.

See also [timestamp](#timestamp) and [keep_metric_names](#keep_metric_names) modifier.

#### tfirst_over_time

`tfirst_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision
for the first [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
on the given lookbehind window `d` per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [first_over_time](#first_over_time).

#### tlast_change_over_time

`tlast_change_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision for the last change
per each time series returned from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering) on the given lookbehind window `d`.

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [last_over_time](#last_over_time).

#### tlast_over_time

`tlast_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which is an alias for [timestamp](#timestamp).

See also [tlast_change_over_time](#tlast_change_over_time).

#### tmax_over_time

`tmax_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision
for the [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
with the maximum value on the given lookbehind window `d`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [max_over_time](#max_over_time).

#### tmin_over_time

`tmin_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns the timestamp in seconds with millisecond precision
for the [raw sample](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
with the minimum value on the given lookbehind window `d`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

See also [min_over_time](#min_over_time).

#### zscore_over_time

`zscore_over_time(series_selector[d])` is a [rollup function](#rollup-functions), which returns [z-score](https://en.wikipedia.org/wiki/Standard_score)
for [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) on the given lookbehind window `d`. It is calculated independently per each time series returned
from the given [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering).

Metric names are stripped from the resulting rollups. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is usually applied to [gauges](https://docs.victoriametrics.com/keyconcepts/#gauge).

See also [zscore](#zscore), [range_trim_zscore](#range_trim_zscore) and [outlier_iqr_over_time](#outlier_iqr_over_time).


### Transform functions

**Transform functions** calculate transformations over [rollup results](#rollup-functions).
For example, `abs(delta(temperature[24h]))` calculates the absolute value for every point of every time series
returned from the rollup `delta(temperature[24h])`.

Additional details:

* If transform function is applied directly to a [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering),
  then the [default_rollup()](#default_rollup) function is automatically applied before calculating the transformations.
  For example, `abs(temperature)` is implicitly transformed to `abs(default_rollup(temperature))`.
* All the transform functions accept optional `keep_metric_names` modifier. If it is set,
  then the function doesn't drop metric names from the resulting time series. See [these docs](#keep_metric_names).

See also [implicit query conversions](#implicit-query-conversions).

The list of supported transform functions:

#### abs

`abs(q)` is a [transform function](#transform-functions), which calculates the absolute value for every point of every time series returned by `q`.

This function is supported by PromQL.

#### absent

`absent(q)` is a [transform function](#transform-functions), which returns 1 if `q` has no points. Otherwise, returns an empty result.

This function is supported by PromQL.

See also [absent_over_time](#absent_over_time).

#### acos

`acos(q)` is a [transform function](#transform-functions), which returns [inverse cosine](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [asin](#asin) and [cos](#cos).

#### acosh

`acosh(q)` is a [transform function](#transform-functions), which returns
[inverse hyperbolic cosine](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_cosine) for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [sinh](#cosh).

#### asin

`asin(q)` is a [transform function](#transform-functions), which returns [inverse sine](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [acos](#acos) and [sin](#sin).

#### asinh

`asinh(q)` is a [transform function](#transform-functions), which returns
[inverse hyperbolic sine](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_sine) for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [sinh](#sinh).

#### atan

`atan(q)` is a [transform function](#transform-functions), which returns [inverse tangent](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [tan](#tan).

#### atanh

`atanh(q)` is a [transform function](#transform-functions), which returns
[inverse hyperbolic tangent](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_tangent) for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [tanh](#tanh).

#### bitmap_and

`bitmap_and(q, mask)` is a [transform function](#transform-functions), which calculates bitwise `v & mask` for every `v` point of every time series returned from `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

#### bitmap_or

`bitmap_or(q, mask)` is a [transform function](#transform-functions), which calculates bitwise `v | mask` for every `v` point of every time series returned from `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

#### bitmap_xor

`bitmap_xor(q, mask)` is a [transform function](#transform-functions), which calculates bitwise `v ^ mask` for every `v` point of every time series returned from `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

#### buckets_limit

`buckets_limit(limit, buckets)` is a [transform function](#transform-functions), which limits the number
of [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) to the given `limit`.

See also [prometheus_buckets](#prometheus_buckets) and [histogram_quantile](#histogram_quantile).

#### ceil

`ceil(q)` is a [transform function](#transform-functions), which rounds every point for every time series returned by `q` to the upper nearest integer.

This function is supported by PromQL.

See also [floor](#floor) and [round](#round).

#### clamp

`clamp(q, min, max)` is a [transform function](#transform-functions), which clamps every point for every time series returned by `q` with the given `min` and `max` values.

This function is supported by PromQL.

See also [clamp_min](#clamp_min) and [clamp_max](#clamp_max).

#### clamp_max

`clamp_max(q, max)` is a [transform function](#transform-functions), which clamps every point for every time series returned by `q` with the given `max` value.

This function is supported by PromQL.

See also [clamp](#clamp) and [clamp_min](#clamp_min).

#### clamp_min

`clamp_min(q, min)` is a [transform function](#transform-functions), which clamps every point for every time series returned by `q` with the given `min` value.

This function is supported by PromQL.

See also [clamp](#clamp) and [clamp_max](#clamp_max).

#### cos

`cos(q)` is a [transform function](#transform-functions), which returns `cos(v)` for every `v` point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [sin](#sin).

#### cosh

`cosh(q)` is a [transform function](#transform-functions), which returns [hyperbolic cosine](https://en.wikipedia.org/wiki/Hyperbolic_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [acosh](#acosh).

#### day_of_month

`day_of_month(q)` is a [transform function](#transform-functions), which returns the day of month for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[1...31]`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [day_of_week](#day_of_week) and [day_of_year](#day_of_year).

#### day_of_week

`day_of_week(q)` is a [transform function](#transform-functions), which returns the day of week for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[0...6]`, where `0` means Sunday and `6` means Saturday.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [day_of_month](#day_of_month) and [day_of_year](#day_of_year).

#### day_of_year

`day_of_year(q)` is a [transform function](#transform-functions), which returns the day of year for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[1...365]` for non-leap years, and `[1 to 366]` in leap years.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [day_of_week](#day_of_week) and [day_of_month](#day_of_month).

#### days_in_month

`days_in_month(q)` is a [transform function](#transform-functions), which returns the number of days in the month identified
by every point of every time series returned by `q`. It is expected that `q` returns unix timestamps.
The returned values are in the range `[28...31]`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### deg

`deg(q)` is a [transform function](#transform-functions), which converts [Radians to degrees](https://en.wikipedia.org/wiki/Radian#Conversions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [rad](#rad).

#### drop_empty_series

`drop_empty_series(q)` is a [transform function](#transform-functions), which drops empty series from `q`.

This function can be used when `default` operator should be applied only to non-empty series. For example,
`drop_empty_series(temperature < 30) default 42` returns series, which have at least a single sample smaller than 30 on the selected time range,
while filling gaps in the returned series with 42.

On the other hand `(temperature < 30) default 40` returns all the `temperature` series, even if they have no samples smaller than 30,
by replacing all the values bigger or equal to 30 with 40.

#### end

`end()` is a [transform function](#transform-functions), which returns the unix timestamp in seconds for the last point.
It is known as `end` query arg passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).

See also [start](#start), [time](#time) and [now](#now).

#### exp

`exp(q)` is a [transform function](#transform-functions), which calculates the `e^v` for every point `v` of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [ln](#ln).

#### floor

`floor(q)` is a [transform function](#transform-functions), which rounds every point for every time series returned by `q` to the lower nearest integer.

This function is supported by PromQL.

See also [ceil](#ceil) and [round](#round).

#### histogram_avg

`histogram_avg(buckets)` is a [transform function](#transform-functions), which calculates the average value for the given `buckets`.
It can be used for calculating the average over the given time range across multiple time series.
For example, `histogram_avg(sum(histogram_over_time(response_time_duration_seconds[5m])) by (vmrange,job))` would return the average response time
per each `job` over the last 5 minutes.

#### histogram_quantile

`histogram_quantile(phi, buckets)` is a [transform function](#transform-functions), which calculates `phi`-[percentile](https://en.wikipedia.org/wiki/Percentile)
over the given [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).
`phi` must be in the range `[0...1]`. For example, `histogram_quantile(0.5, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`
would return median request duration for all the requests during the last 5 minutes.

The function accepts optional third arg - `boundsLabel`. In this case it returns `lower` and `upper` bounds for the estimated percentile with the given `boundsLabel` label.
See [this issue for details](https://github.com/prometheus/prometheus/issues/5706).

When the [percentile](https://en.wikipedia.org/wiki/Percentile) is calculated over multiple histograms,
then all the input histograms **must** have buckets with identical boundaries, e.g. they must have the same set of `le` or `vmrange` labels.
Otherwise, the returned result may be invalid. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3231) for details.

This function is supported by PromQL (except of the `boundLabel` arg).

See also [histogram_quantiles](#histogram_quantiles), [histogram_share](#histogram_share) and [quantile](#quantile).

#### histogram_quantiles

`histogram_quantiles("phiLabel", phi1, ..., phiN, buckets)` is a [transform function](#transform-functions), which calculates the given `phi*`-quantiles
over the given [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).
Argument `phi*` must be in the range `[0...1]`. For example, `histogram_quantiles('le', 0.3, 0.5, sum(rate(http_request_duration_seconds_bucket[5m]) by (le))`.
Each calculated quantile is returned in a separate time series with the corresponding `{phiLabel="phi*"}` label.

See also [histogram_quantile](#histogram_quantile).

#### histogram_share

`histogram_share(le, buckets)` is a [transform function](#transform-functions), which calculates the share (in the range `[0...1]`)
for `buckets` that fall below `le`. This function is useful for calculating SLI and SLO. This is inverse to [histogram_quantile](#histogram_quantile).

The function accepts optional third arg - `boundsLabel`. In this case it returns `lower` and `upper` bounds for the estimated share with the given `boundsLabel` label.

#### histogram_stddev

`histogram_stddev(buckets)` is a [transform function](#transform-functions), which calculates standard deviation for the given `buckets`.

#### histogram_stdvar

`histogram_stdvar(buckets)` is a [transform function](#transform-functions), which calculates standard variance for the given `buckets`.
It can be used for calculating standard deviation over the given time range across multiple time series.
For example, `histogram_stdvar(sum(histogram_over_time(temperature[24])) by (vmrange,country))` would return standard deviation
for the temperature per each country over the last 24 hours.

#### hour

`hour(q)` is a [transform function](#transform-functions), which returns the hour for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[0...23]`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### interpolate

`interpolate(q)` is a [transform function](#transform-functions), which fills gaps with linearly interpolated values calculated
from the last and the next non-empty points per each time series returned by `q`.

See also [keep_last_value](#keep_last_value) and [keep_next_value](#keep_next_value).

#### keep_last_value

`keep_last_value(q)` is a [transform function](#transform-functions), which fills gaps with the value of the last non-empty point
in every time series returned by `q`.

See also [keep_next_value](#keep_next_value) and [interpolate](#interpolate).

#### keep_next_value

`keep_next_value(q)` is a [transform function](#transform-functions), which fills gaps with the value of the next non-empty point
in every time series returned by `q`.

See also [keep_last_value](#keep_last_value) and [interpolate](#interpolate).

#### limit_offset

`limit_offset(limit, offset, q)` is a [transform function](#transform-functions), which skips `offset` time series from series returned by `q`
and then returns up to `limit` of the remaining time series per each group.

This allows implementing simple paging for `q` time series. See also [limitk](#limitk).

#### ln

`ln(q)` is a [transform function](#transform-functions), which calculates `ln(v)` for every point `v` of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [exp](#exp) and [log2](#log2).

#### log2

`log2(q)` is a [transform function](#transform-functions), which calculates `log2(v)` for every point `v` of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [log10](#log10) and [ln](#ln).

#### log10

`log10(q)` is a [transform function](#transform-functions), which calculates `log10(v)` for every point `v` of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [log2](#log2) and [ln](#ln).

#### minute

`minute(q)` is a [transform function](#transform-functions), which returns the minute for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[0...59]`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### month

`month(q)` is a [transform function](#transform-functions), which returns the month for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps. The returned values are in the range `[1...12]`, where `1` means January and `12` means December.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### now

`now()` is a [transform function](#transform-functions), which returns the current timestamp as a floating-point value in seconds.

See also [time](#time).

#### pi

`pi()` is a [transform function](#transform-functions), which returns [Pi number](https://en.wikipedia.org/wiki/Pi).

This function is supported by PromQL.

#### rad

`rad(q)` is a [transform function](#transform-functions), which converts [degrees to Radians](https://en.wikipedia.org/wiki/Radian#Conversions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

See also [deg](#deg).

#### prometheus_buckets

`prometheus_buckets(buckets)` is a [transform function](#transform-functions), which converts
[VictoriaMetrics histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) with `vmrange` labels
to Prometheus histogram buckets with `le` labels. This may be useful for building heatmaps in Grafana.

See also [histogram_quantile](#histogram_quantile) and [buckets_limit](#buckets_limit).

#### rand

`rand(seed)` is a [transform function](#transform-functions), which returns pseudo-random numbers on the range `[0...1]` with even distribution.
Optional `seed` can be used as a seed for pseudo-random number generator.

See also [rand_normal](#rand_normal) and [rand_exponential](#rand_exponential).

#### rand_exponential

`rand_exponential(seed)` is a [transform function](#transform-functions), which returns pseudo-random numbers
with [exponential distribution](https://en.wikipedia.org/wiki/Exponential_distribution). Optional `seed` can be used as a seed for pseudo-random number generator.

See also [rand](#rand) and [rand_normal](#rand_normal).

#### rand_normal

`rand_normal(seed)` is a [transform function](#transform-functions), which returns pseudo-random numbers
with [normal distribution](https://en.wikipedia.org/wiki/Normal_distribution). Optional `seed` can be used as a seed for pseudo-random number generator.

See also [rand](#rand) and [rand_exponential](#rand_exponential).

#### range_avg

`range_avg(q)` is a [transform function](#transform-functions), which calculates the avg value across points per each time series returned by `q`.

#### range_first

`range_first(q)` is a [transform function](#transform-functions), which returns the value for the first point per each time series returned by `q`.

#### range_last

`range_last(q)` is a [transform function](#transform-functions), which returns the value for the last point per each time series returned by `q`.

#### range_linear_regression

`range_linear_regression(q)` is a [transform function](#transform-functions), which calculates [simple linear regression](https://en.wikipedia.org/wiki/Simple_linear_regression)
over the selected time range per each time series returned by `q`. This function is useful for capacity planning and predictions.

#### range_mad

`range_mad(q)` is a [transform function](#transform-functions), which calculates the [median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation)
across points per each time series returned by `q`.

See also [mad](#mad) and [mad_over_time](#mad_over_time).

#### range_max

`range_max(q)` is a [transform function](#transform-functions), which calculates the max value across points per each time series returned by `q`.

#### range_median

`range_median(q)` is a [transform function](#transform-functions), which calculates the median value across points per each time series returned by `q`.

#### range_min

`range_min(q)` is a [transform function](#transform-functions), which calculates the min value across points per each time series returned by `q`.

#### range_normalize

`range_normalize(q1, ...)` is a [transform function](#transform-functions), which normalizes values for time series returned by `q1, ...` into `[0 ... 1]` range.
This function is useful for correlating time series with distinct value ranges.

See also [share](#share).

#### range_quantile

`range_quantile(phi, q)` is a [transform function](#transform-functions), which returns `phi`-quantile across points per each time series returned by `q`.
`phi` must be in the range `[0...1]`.

#### range_stddev

`range_stddev(q)` is a [transform function](#transform-functions), which calculates [standard deviation](https://en.wikipedia.org/wiki/Standard_deviation)
per each time series returned by `q` on the selected time range.

#### range_stdvar

`range_stdvar(q)` is a [transform function](#transform-functions), which calculates [standard variance](https://en.wikipedia.org/wiki/Variance)
per each time series returned by `q` on the selected time range.

#### range_sum

`range_sum(q)` is a [transform function](#transform-functions), which calculates the sum of points per each time series returned by `q`.

#### range_trim_outliers

`range_trim_outliers(k, q)` is a [transform function](#transform-functions), which drops points located farther than `k*range_mad(q)`
from the `range_median(q)`. E.g. it is equivalent to the following query: `q ifnot (abs(q - range_median(q)) > k*range_mad(q))`.

See also [range_trim_spikes](#range_trim_spikes) and [range_trim_zscore](#range_trim_zscore).

#### range_trim_spikes

`range_trim_spikes(phi, q)` is a [transform function](#transform-functions), which drops `phi` percent of biggest spikes from time series returned by `q`.
The `phi` must be in the range `[0..1]`, where `0` means `0%` and `1` means `100%`.

See also [range_trim_outliers](#range_trim_outliers) and [range_trim_zscore](#range_trim_zscore).

#### range_trim_zscore

`range_trim_zscore(z, q)` is a [transform function](#transform-functions), which drops points located farther than `z*range_stddev(q)`
from the `range_avg(q)`. E.g. it is equivalent to the following query: `q ifnot (abs(q - range_avg(q)) > z*range_avg(q))`.

See also [range_trim_outliers](#range_trim_outliers) and [range_trim_spikes](#range_trim_spikes).

#### range_zscore

`range_zscore(q)` is a [transform function](#transform-functions), which calculates [z-score](https://en.wikipedia.org/wiki/Standard_score)
for points returned by `q`, e.g. it is equivalent to the following query: `(q - range_avg(q)) / range_stddev(q)`.

#### remove_resets

`remove_resets(q)` is a [transform function](#transform-functions), which removes counter resets from time series returned by `q`.

#### round

`round(q, nearest)` is a [transform function](#transform-functions), which rounds every point of every time series returned by `q` to the `nearest` multiple.
If `nearest` is missing then the rounding is performed to the nearest integer.

This function is supported by PromQL.

See also [floor](#floor) and [ceil](#ceil).

#### ru

`ru(free, max)` is a [transform function](#transform-functions), which calculates resource utilization in the range `[0%...100%]` for the given `free` and `max` resources.
For instance, `ru(node_memory_MemFree_bytes, node_memory_MemTotal_bytes)` returns memory utilization over [node_exporter](https://github.com/prometheus/node_exporter) metrics.

#### running_avg

`running_avg(q)` is a [transform function](#transform-functions), which calculates the running avg per each time series returned by `q`.

#### running_max

`running_max(q)` is a [transform function](#transform-functions), which calculates the running max per each time series returned by `q`.

#### running_min

`running_min(q)` is a [transform function](#transform-functions), which calculates the running min per each time series returned by `q`.

#### running_sum

`running_sum(q)` is a [transform function](#transform-functions), which calculates the running sum per each time series returned by `q`.

#### scalar

`scalar(q)` is a [transform function](#transform-functions), which returns `q` if `q` contains only a single time series. Otherwise, it returns nothing.

This function is supported by PromQL.

#### sgn

`sgn(q)` is a [transform function](#transform-functions), which returns `1` if `v>0`, `-1` if `v<0` and `0` if `v==0` for every point `v`
of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### sin

`sin(q)` is a [transform function](#transform-functions), which returns `sin(v)` for every `v` point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by MetricsQL.

See also [cos](#cos).

#### sinh

`sinh(q)` is a [transform function](#transform-functions), which returns [hyperbolic sine](https://en.wikipedia.org/wiki/Hyperbolic_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by MetricsQL.

See also [cosh](#cosh).

#### tan

`tan(q)` is a [transform function](#transform-functions), which returns `tan(v)` for every `v` point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by MetricsQL.

See also [atan](#atan).

#### tanh

`tanh(q)` is a [transform function](#transform-functions), which returns [hyperbolic tangent](https://en.wikipedia.org/wiki/Hyperbolic_functions)
for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by MetricsQL.

See also [atanh](#atanh).

#### smooth_exponential

`smooth_exponential(q, sf)` is a [transform function](#transform-functions), which smooths points per each time series returned
by `q` using [exponential moving average](https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average) with the given smooth factor `sf`.

#### sort

`sort(q)` is a [transform function](#transform-functions), which sorts series in ascending order by the last point in every time series returned by `q`.

This function is supported by PromQL.

See also [sort_desc](#sort_desc) and [sort_by_label](#sort_by_label).

#### sort_desc

`sort_desc(q)` is a [transform function](#transform-functions), which sorts series in descending order by the last point in every time series returned by `q`.

This function is supported by PromQL.

See also [sort](#sort) and [sort_by_label](#sort_by_label_desc).

#### sqrt

`sqrt(q)` is a [transform function](#transform-functions), which calculates square root for every point of every time series returned by `q`.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

#### start

`start()` is a [transform function](#transform-functions), which returns unix timestamp in seconds for the first point.

It is known as `start` query arg passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).

See also [end](#end), [time](#time) and [now](#now).

#### step

`step()` is a [transform function](#transform-functions), which returns the step in seconds (aka interval) between the returned points.
It is known as `step` query arg passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).

See also [start](#start) and [end](#end).

#### time

`time()` is a [transform function](#transform-functions), which returns unix timestamp for every returned point.

This function is supported by PromQL.

See also [timestamp](#timestamp), [now](#now), [start](#start) and [end](#end).

#### timezone_offset

`timezone_offset(tz)` is a [transform function](#transform-functions), which returns offset in seconds for the given timezone `tz` relative to UTC.
This can be useful when combining with datetime-related functions. For example, `day_of_week(time()+timezone_offset("America/Los_Angeles"))`
would return weekdays for `America/Los_Angeles` time zone.

Special `Local` time zone can be used for returning an offset for the time zone set on the host where VictoriaMetrics runs.

See [the list of supported timezones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

#### ttf

`ttf(free)` is a [transform function](#transform-functions), which estimates the time in seconds needed to exhaust `free` resources.
For instance, `ttf(node_filesystem_avail_byte)` returns the time to storage space exhaustion. This function may be useful for capacity planning.

#### union

`union(q1, ..., qN)` is a [transform function](#transform-functions), which returns a union of time series returned from `q1`, ..., `qN`.
The `union` function name can be skipped - the following queries are equivalent: `union(q1, q2)` and `(q1, q2)`.

It is expected that each `q*` query returns time series with unique sets of labels.
Otherwise, only the first time series out of series with identical set of labels is returned.
Use [alias](#alias) and [label_set](#label_set) functions for giving unique labelsets per each `q*` query:

#### vector

`vector(q)` is a [transform function](#transform-functions), which returns `q`, e.g. it does nothing in MetricsQL.

This function is supported by PromQL.

#### year

`year(q)` is a [transform function](#transform-functions), which returns the year for every point of every time series returned by `q`.
It is expected that `q` returns unix timestamps.

Metric names are stripped from the resulting series. Add [keep_metric_names](#keep_metric_names) modifier in order to keep metric names.

This function is supported by PromQL.

### Label manipulation functions

**Label manipulation functions** perform manipulations with labels on the selected [rollup results](#rollup-functions).

Additional details:

* If label manipulation function is applied directly to a [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering),
  then the [default_rollup()](#default_rollup) function is automatically applied before performing the label transformation.
  For example, `alias(temperature, "foo")` is implicitly transformed to `alias(default_rollup(temperature), "foo")`.

See also [implicit query conversions](#implicit-query-conversions).

The list of supported label manipulation functions:

#### alias

`alias(q, "name")` is [label manipulation function](#label-manipulation-functions), which sets the given `name` to all the time series returned by `q`.
For example, `alias(up, "foobar")` would rename `up` series to `foobar` series.


#### drop_common_labels

`drop_common_labels(q1, ...., qN)` is [label manipulation function](#label-manipulation-functions), which drops common `label="value"` pairs
among time series returned from `q1, ..., qN`.

#### label_copy

`label_copy(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")` is [label manipulation function](#label-manipulation-functions),
which copies label values from `src_label*` to `dst_label*` for all the time series returned by `q`.
If `src_label` is empty, then the corresponding `dst_label` is left untouched.

#### label_del

`label_del(q, "label1", ..., "labelN")` is [label manipulation function](#label-manipulation-functions), which deletes the given `label*` labels
from all the time series returned by `q`.

#### label_graphite_group

`label_graphite_group(q, groupNum1, ... groupNumN)` is [label manipulation function](#label-manipulation-functions), which replaces metric names
returned from `q` with the given Graphite group values concatenated via `.` char.

For example, `label_graphite_group({__graphite__="foo*.bar.*"}, 0, 2)` would substitute `foo<any_value>.bar.<other_value>` metric names with `foo<any_value>.<other_value>`.

This function is useful for aggregating Graphite metrics with [aggregate functions](#aggregate-functions). For example, the following query would return per-app memory usage:

```
sum by (__name__) (
    label_graphite_group({__graphite__="app*.host*.memory_usage"}, 0)
)
```

#### label_join

`label_join(q, "dst_label", "separator", "src_label1", ..., "src_labelN")` is [label manipulation function](#label-manipulation-functions),
which joins `src_label*` values with the given `separator` and stores the result in `dst_label`.
This is performed individually per each time series returned by `q`.
For example, `label_join(up{instance="xxx",job="yyy"}, "foo", "-", "instance", "job")` would store `xxx-yyy` label value into `foo` label.

This function is supported by PromQL.

#### label_keep

`label_keep(q, "label1", ..., "labelN")` is [label manipulation function](#label-manipulation-functions), which deletes all the labels
except of the listed `label*` labels in all the time series returned by `q`.

#### label_lowercase

`label_lowercase(q, "label1", ..., "labelN")` is [label manipulation function](#label-manipulation-functions), which lowercases values
for the given `label*` labels in all the time series returned by `q`.

#### label_map

`label_map(q, "label", "src_value1", "dst_value1", ..., "src_valueN", "dst_valueN")` is [label manipulation function](#label-manipulation-functions),
which maps `label` values from `src_*` to `dst*` for all the time series returned by `q`.

#### label_match

`label_match(q, "label", "regexp")` is [label manipulation function](#label-manipulation-functions),
which drops time series from `q` with `label` not matching the given `regexp`.
This function can be useful after [rollup](#rollup)-like functions, which may return multiple time series for every input series.

See also [label_mismatch](#label_mismatch) and [labels_equal](#labels_equal).

#### label_mismatch

`label_mismatch(q, "label", "regexp")` is [label manipulation function](#label-manipulation-functions),
which drops time series from `q` with `label` matching the given `regexp`.
This function can be useful after [rollup](#rollup)-like functions, which may return multiple time series for every input series.

See also [label_match](#label_match) and [labels_equal](#labels_equal).

#### label_move

`label_move(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")` is [label manipulation function](#label-manipulation-functions),
which moves label values from `src_label*` to `dst_label*` for all the time series returned by `q`.
If `src_label` is empty, then the corresponding `dst_label` is left untouched.

#### label_replace

`label_replace(q, "dst_label", "replacement", "src_label", "regex")` is [label manipulation function](#label-manipulation-functions),
which applies the given `regex` to `src_label` and stores the `replacement` in `dst_label` if the given `regex` matches `src_label`.
The `replacement` may contain references to regex captures such as `$1`, `$2`, etc.
These references are substituted by the corresponding regex captures.
For example, `label_replace(up{job="node-exporter"}, "foo", "bar-$1", "job", "node-(.+)")` would store `bar-exporter` label value into `foo` label.

This function is supported by PromQL.

#### label_set

`label_set(q, "label1", "value1", ..., "labelN", "valueN")` is [label manipulation function](#label-manipulation-functions),
which sets `{label1="value1", ..., labelN="valueN"}` labels to all the time series returned by `q`.

#### label_transform

`label_transform(q, "label", "regexp", "replacement")` is [label manipulation function](#label-manipulation-functions),
which substitutes all the `regexp` occurrences by the given `replacement` in the given `label`.

#### label_uppercase

`label_uppercase(q, "label1", ..., "labelN")` is [label manipulation function](#label-manipulation-functions),
which uppercases values for the given `label*` labels in all the time series returned by `q`.

See also [label_lowercase](#label_lowercase).

#### label_value

`label_value(q, "label")` is [label manipulation function](#label-manipulation-functions), which returns numeric values
for the given `label` for every time series returned by `q`.

For example, if `label_value(foo, "bar")` is applied to `foo{bar="1.234"}`, then it will return a time series
`foo{bar="1.234"}` with `1.234` value. Function will return no data for non-numeric label values.

#### labels_equal

`labels_equal(q, "label1", "label2", ...)` is [label manipulation function](#label-manipulation-functions), which returns `q` series with identical values for the listed labels
"label1", "label2", etc.

See also [label_match](#label_match) and [label_mismatch](#label_mismatch).

#### sort_by_label

`sort_by_label(q, "label1", ... "labelN")` is [label manipulation function](#label-manipulation-functions), which sorts series in ascending order by the given set of labels.
For example, `sort_by_label(foo, "bar")` would sort `foo` series by values of the label `bar` in these series.

See also [sort_by_label_desc](#sort_by_label_desc) and [sort_by_label_numeric](#sort_by_label_numeric).

#### sort_by_label_desc

`sort_by_label_desc(q, "label1", ... "labelN")` is [label manipulation function](#label-manipulation-functions), which sorts series in descending order by the given set of labels.
For example, `sort_by_label(foo, "bar")` would sort `foo` series by values of the label `bar` in these series.

See also [sort_by_label](#sort_by_label) and [sort_by_label_numeric_desc](#sort_by_label_numeric_desc).

#### sort_by_label_numeric

`sort_by_label_numeric(q, "label1", ... "labelN")` is [label manipulation function](#label-manipulation-functions), which sorts series in ascending order by the given set of labels
using [numeric sort](https://www.gnu.org/software/coreutils/manual/html_node/Version-sort-is-not-the-same-as-numeric-sort.html).
For example, if `foo` series have `bar` label with values `1`, `101`, `15` and `2`, then `sort_by_label_numeric(foo, "bar")` would return series
in the following order of `bar` label values: `1`, `2`, `15` and `101`.

See also [sort_by_label_numeric_desc](#sort_by_label_numeric_desc) and [sort_by_label](#sort_by_label).

#### sort_by_label_numeric_desc

`sort_by_label_numeric_desc(q, "label1", ... "labelN")` is [label manipulation function](#label-manipulation-functions), which sorts series in descending order
by the given set of labels using [numeric sort](https://www.gnu.org/software/coreutils/manual/html_node/Version-sort-is-not-the-same-as-numeric-sort.html).
For example, if `foo` series have `bar` label with values `1`, `101`, `15` and `2`, then `sort_by_label_numeric(foo, "bar")`
would return series in the following order of `bar` label values: `101`, `15`, `2` and `1`.

See also [sort_by_label_numeric](#sort_by_label_numeric) and [sort_by_label_desc](#sort_by_label_desc).


### Aggregate functions

**Aggregate functions** calculate aggregates over groups of [rollup results](#rollup-functions).

Additional details:

* By default, a single group is used for aggregation. Multiple independent groups can be set up by specifying grouping labels
  in `by` and `without` modifiers. For example, `count(up) by (job)` would group [rollup results](#rollup-functions) by `job` label value
  and calculate the [count](#count) aggregate function independently per each group, while `count(up) without (instance)`
  would group [rollup results](#rollup-functions) by all the labels except `instance` before calculating [count](#count) aggregate function independently per each group.
  Multiple labels can be put in `by` and `without` modifiers.
* If the aggregate function is applied directly to a [series_selector](https://docs.victoriametrics.com/keyconcepts/#filtering),
  then the [default_rollup()](#default_rollup) function is automatically applied before calculating the aggregate.
  For example, `count(up)` is implicitly transformed to `count(default_rollup(up))`.
* Aggregate functions accept arbitrary number of args. For example, `avg(q1, q2, q3)` would return the average values for every point
  across time series returned by `q1`, `q2` and `q3`.
* Aggregate functions support optional `limit N` suffix, which can be used for limiting the number of output groups.
  For example, `sum(x) by (y) limit 3` limits the number of groups for the aggregation to 3. All the other groups are ignored.

See also [implicit query conversions](#implicit-query-conversions).

The list of supported aggregate functions:

#### any

`any(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns a single series per `group_labels` out of time series returned by `q`.

See also [group](#group).

#### avg

`avg(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the average value per `group_labels` for time series returned by `q`.
The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### bottomk

`bottomk(k, q)` is [aggregate function](#aggregate-functions), which returns up to `k` points with the smallest values across all the time series returned by `q`.
The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

See also [topk](#topk), [bottomk_min](#bottomk_min) and [#bottomk_last](#bottomk_last).

#### bottomk_avg

`bottomk_avg(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the smallest averages.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `bottomk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series
with the smallest averages plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [topk_avg](#topk_avg).

#### bottomk_last

`bottomk_last(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the smallest last values.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series
with the smallest maximums plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [topk_last](#topk_last).

#### bottomk_max

`bottomk_max(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the smallest maximums.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series
with the smallest maximums plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [topk_max](#topk_max).

#### bottomk_median

`bottomk_median(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the smallest medians.
If an optional`other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `bottomk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series
with the smallest medians plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [topk_median](#topk_median).

#### bottomk_min

`bottomk_min(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the smallest minimums.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `bottomk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series
with the smallest minimums plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [topk_min](#topk_min).

#### count

`count(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the number of non-empty points per `group_labels`
for time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### count_values

`count_values("label", q)` is [aggregate function](#aggregate-functions), which counts the number of points with the same value
and stores the counts in a time series with an additional `label`, which contains each initial value.
The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

See also [count_values_over_time](#count_values_over_time) and [label_match](#label_match).

#### distinct

`distinct(q)` is [aggregate function](#aggregate-functions), which calculates the number of unique values per each group of points with the same timestamp.

See also [distinct_over_time](#distinct_over_time).

#### geomean

`geomean(q)` is [aggregate function](#aggregate-functions), which calculates geometric mean per each group of points with the same timestamp.

#### group

`group(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns `1` per each `group_labels` for time series returned by `q`.

This function is supported by PromQL. See also [any](#any).

#### histogram

`histogram(q)` is [aggregate function](#aggregate-functions), which calculates
[VictoriaMetrics histogram](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350)
per each group of points with the same timestamp. Useful for visualizing big number of time series via a heatmap.
See [this article](https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) for more details.

See also [histogram_over_time](#histogram_over_time) and [histogram_quantile](#histogram_quantile).

#### limitk

`limitk(k, q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns up to `k` time series per each `group_labels`
out of time series returned by `q`. The returned set of time series remain the same across calls.

See also [limit_offset](#limit_offset).

#### mad

`mad(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation)
per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

See also [range_mad](#range_mad), [mad_over_time](#mad_over_time), [outliers_mad](#outliers_mad) and [stddev](#stddev).

#### max

`max(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the maximum value per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### median

`median(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the median value per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### min

`min(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the minimum value per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### mode

`mode(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns [mode](https://en.wikipedia.org/wiki/Mode_(statistics))
per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### outliers_iqr

`outliers_iqr(q)` is [aggregate function](#aggregate-functions), which returns time series from `q` with at least a single point
outside e.g. [Interquartile range outlier bounds](https://en.wikipedia.org/wiki/Interquartile_range) `[q25-1.5*iqr .. q75+1.5*iqr]`
comparing to other time series at the given point, where:
- `iqr` is an [Interquartile range](https://en.wikipedia.org/wiki/Interquartile_range) calculated independently per each point on the graph across `q` series.
- `q25` and `q75` are 25th and 75th [percentiles](https://en.wikipedia.org/wiki/Percentile) calculated independently per each point on the graph across `q` series.

The `outliers_iqr()` is useful for detecting anomalous series in the group of series. For example, `outliers_iqr(temperature) by (country)` returns
per-country series with anomalous outlier values comparing to the rest of per-country series.

See also [outliers_mad](#outliers_mad), [outliersk](#outliersk) and [outlier_iqr_over_time](#outlier_iqr_over_time).

#### outliers_mad

`outliers_mad(tolerance, q)` is [aggregate function](#aggregate-functions), which returns time series from `q` with at least
a single point outside [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) (aka MAD) multiplied by `tolerance`.
E.g. it returns time series with at least a single point below `median(q) - mad(q)` or a single point above `median(q) + mad(q)`.

See also [outliers_iqr](#outliers_iqr), [outliersk](#outliersk) and [mad](#mad).

#### outliersk

`outliersk(k, q)` is [aggregate function](#aggregate-functions), which returns up to `k` time series with the biggest standard deviation (aka outliers)
out of time series returned by `q`.

See also [outliers_iqr](#outliers_iqr) and [outliers_mad](#outliers_mad).

#### quantile

`quantile(phi, q) by (group_labels)` is [aggregate function](#aggregate-functions), which calculates `phi`-quantile per each `group_labels`
for all the time series returned by `q`. `phi` must be in the range `[0...1]`.
The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

See also [quantiles](#quantiles) and [histogram_quantile](#histogram_quantile).

#### quantiles

`quantiles("phiLabel", phi1, ..., phiN, q)` is [aggregate function](#aggregate-functions), which calculates `phi*`-quantiles for all the time series
returned by `q` and return them in time series with `{phiLabel="phi*"}` label. `phi*` must be in the range `[0...1]`.
The aggregate is calculated individually per each group of points with the same timestamp.

See also [quantile](#quantile).

#### share

`share(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns shares in the range `[0..1]`
for every non-negative points returned by `q` per each timestamp, so the sum of shares per each `group_labels` equals 1.

This function is useful for normalizing [histogram bucket](https://docs.victoriametrics.com/keyconcepts/#histogram) shares
into `[0..1]` range:

```metricsql
share(
  sum(
    rate(http_request_duration_seconds_bucket[5m])
  ) by (le, vmrange)
)
```

See also [range_normalize](#range_normalize).

#### stddev

`stddev(q) by (group_labels)` is [aggregate function](#aggregate-functions), which calculates standard deviation per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### stdvar

`stdvar(q) by (group_labels)` is [aggregate function](#aggregate-functions), which calculates standard variance per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### sum

`sum(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns the sum per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

#### sum2

`sum2(q) by (group_labels)` is [aggregate function](#aggregate-functions), which calculates the sum of squares per each `group_labels`
for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### topk

`topk(k, q)` is [aggregate function](#aggregate-functions), which returns up to `k` points with the biggest values across all the time series returned by `q`.
The aggregate is calculated individually per each group of points with the same timestamp.

This function is supported by PromQL.

See also [bottomk](#bottomk), [topk_max](#topk_max) and [topk_last](#topk_last).

#### topk_avg

`topk_avg(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the biggest averages.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `topk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest averages
plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [bottomk_avg](#bottomk_avg).

#### topk_last

`topk_last(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the biggest last values.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest maximums
plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [bottomk_last](#bottomk_last).

#### topk_max

`topk_max(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the biggest maximums.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest maximums
plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [bottomk_max](#bottomk_max).

#### topk_median

`topk_median(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the biggest medians.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `topk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest medians
plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [bottomk_median](#bottomk_median).

#### topk_min

`topk_min(k, q, "other_label=other_value")` is [aggregate function](#aggregate-functions), which returns up to `k` time series from `q` with the biggest minimums.
If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label.
For example, `topk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest minimums
plus a time series with `{job="other"}` label with the sum of the remaining series if any.

See also [bottomk_min](#bottomk_min).

#### zscore

`zscore(q) by (group_labels)` is [aggregate function](#aggregate-functions), which returns [z-score](https://en.wikipedia.org/wiki/Standard_score) values
per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.
This function is useful for detecting anomalies in the group of related time series.

See also [zscore_over_time](#zscore_over_time), [range_trim_zscore](#range_trim_zscore) and [outliers_iqr](#outliers_iqr).

## Subqueries

MetricsQL supports and extends PromQL subqueries. See [this article](https://valyala.medium.com/prometheus-subqueries-in-victoriametrics-9b1492b720b3) for details.
Any [rollup function](#rollup-functions) for something other than [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) form a subquery.
Nested rollup functions can be implicit thanks to the [implicit query conversions](#implicit-query-conversions).
For example, `delta(sum(m))` is implicitly converted to `delta(sum(default_rollup(m))[1i:1i])`, so it becomes a subquery,
since it contains [default_rollup](#default_rollup) nested into [delta](#delta).
This behavior can be disabled or logged via `-search.disableImplicitConversion` and `-search.logImplicitConversion` command-line flags
starting from [`v1.101.0` release](https://docs.victoriametrics.com/changelog/).

VictoriaMetrics performs subqueries in the following way:

* It calculates the inner rollup function using the `step` value from the outer rollup function.
  For example, for expression `max_over_time(rate(http_requests_total[5m])[1h:30s])` the inner function `rate(http_requests_total[5m])`
  is calculated with `step=30s`. The resulting data points are aligned by the `step`.
* It calculates the outer rollup function over the results of the inner rollup function using the `step` value
  passed by Grafana to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query).

## Implicit query conversions

VictoriaMetrics performs the following implicit conversions for incoming queries before starting the calculations:

* If lookbehind window in square brackets is missing inside [rollup function](#rollup-functions), then it is automatically set to the following value:
  - To `step` value passed to [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) or [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query)
    for all the [rollup functions](#rollup-functions) except of [default_rollup](#default_rollup) and [rate](#rate). This value is known as `$__interval` in Grafana or `1i` in MetricsQL.
    For example, `avg_over_time(temperature)` is automatically transformed to `avg_over_time(temperature[1i])`.
  - To the `max(step, scrape_interval)`, where `scrape_interval` is the interval between [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
    for [default_rollup](#default_rollup) and [rate](#rate) functions. This allows avoiding unexpected gaps on the graph when `step` is smaller than `scrape_interval`.
* All the [series selectors](https://docs.victoriametrics.com/keyconcepts/#filtering),
  which aren't wrapped into [rollup functions](#rollup-functions), are automatically wrapped into [default_rollup](#default_rollup) function.
  Examples:
  * `foo` is transformed to `default_rollup(foo)`
  * `foo + bar` is transformed to `default_rollup(foo) + default_rollup(bar)`
  * `count(up)` is transformed to `count(default_rollup(up))`, because [count](#count) isn't a [rollup function](#rollup-functions) -
    it is [aggregate function](#aggregate-functions)
  * `abs(temperature)` is transformed to `abs(default_rollup(temperature))`, because [abs](#abs) isn't a [rollup function](#rollup-functions) -
    it is [transform function](#transform-functions)
* If `step` in square brackets is missing inside [subquery](#subqueries), then `1i` step is automatically added there.
  For example, `avg_over_time(rate(http_requests_total[5m])[1h])` is automatically converted to `avg_over_time(rate(http_requests_total[5m])[1h:1i])`.
* If something other than [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering)
  is passed to [rollup function](#rollup-functions), then a [subquery](#subqueries) with `1i` lookbehind window and `1i` step is automatically formed.
  For example, `rate(sum(up))` is automatically converted to `rate((sum(default_rollup(up)))[1i:1i])`.
  This behavior can be disabled or logged via `-search.disableImplicitConversion` and `-search.logImplicitConversion` command-line flags
  starting from [`v1.101.0` release](https://docs.victoriametrics.com/changelog/).
