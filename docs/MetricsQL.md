---
sort: 13
---

# MetricsQL

[VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) implements MetricsQL - query language inspired by [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/).
MetricsQL is backwards-compatible with PromQL, so Grafana dashboards backed by Prometheus datasource should work the same after switching from Prometheus to VictoriaMetrics.
However, there are some [intentional differences](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) between these two languages.

[Standalone MetricsQL package](https://godoc.org/github.com/VictoriaMetrics/metricsql) can be used for parsing MetricsQL in external apps.

If you are unfamiliar with PromQL, then it is suggested reading [this tutorial for beginners](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085).

The following functionality is implemented differently in MetricsQL compared to PromQL. This improves user experience:
* MetricsQL takes into account the previous point before the window in square brackets for range functions such as [rate](#rate) and [increase](#increase). This allows returning the exact results users expect for `increase(metric[$__interval])` queries instead of incomplete results Prometheus returns for such queries.
* MetricsQL doesn't extrapolate range function results. This addresses [this issue from Prometheus](https://github.com/prometheus/prometheus/issues/3746). See technical details about VictoriaMetrics and Prometheus calculations for [rate](#rate) and [increase](#increase) [in this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1215#issuecomment-850305711).
* MetricsQL returns the expected non-empty responses for [rate](#rate) with `step` values smaller than scrape interval. This addresses [this issue from Grafana](https://github.com/grafana/grafana/issues/11451). See also [this blog post](https://www.percona.com/blog/2020/02/28/better-prometheus-rate-function-with-victoriametrics/).
* MetricsQL treats `scalar` type the same as `instant vector` without labels, since subtle differences between these types usually confuse users. See [the corresponding Prometheus docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#expression-language-data-types) for details.
* MetricsQL removes all the `NaN` values from the output, so some queries like `(-1)^0.5` return empty results in VictoriaMetrics, while returning a series of `NaN` values in Prometheus. Note that Grafana doesn't draw any lines or dots for `NaN` values, so the end result looks the same for both VictoriaMetrics and Prometheus.
* MetricsQL keeps metric names after applying functions, which don't change the meaning of the original time series. For example, [min_over_time(foo)](#min_over_time) or [round(foo)](#round) leaves `foo` metric name in the result. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/674) for details.

Read more about the diffferences between PromQL and MetricsQL in [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e).

Other PromQL functionality should work the same in MetricsQL. [File an issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you notice discrepancies between PromQL and MetricsQL results other than mentioned above.

## MetricsQL features

MetricsQL implements [PromQL](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) and provides additional functionality mentioned below, which is aimed towards solving practical cases. Feel free [filing a feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you think MetricsQL misses certain useful functionality.

This functionality can be evaluated at [an editable Grafana dashboard](https://play-grafana.victoriametrics.com/d/4ome8yJmz/node-exporter-on-victoriametrics-demo) or at your own [VictoriaMetrics instance](https://docs.victoriametrics.com/#how-to-start-victoriametrics).

- Graphite-compatible filters can be passed via `{__graphite__="foo.*.bar"}` syntax. See [these docs](https://docs.victoriametrics.com/#selecting-graphite-metrics). VictoriaMetrics also can be used as Graphite datasource in Grafana. See [these docs](https://docs.victoriametrics.com/#graphite-api-usage) for details. See also [label_graphite_group](#label_graphite_group) function, which can be used for extracting the given groups from Graphite metric name.
- Lookbehind window in square brackets may be omitted. VictoriaMetrics automatically selects the lookbehind window depending on the current step used for building the graph (e.g. `step` query arg passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries)). For instance, the following query is valid in VictoriaMetrics: `rate(node_network_receive_bytes_total)`. It is equivalent to `rate(node_network_receive_bytes_total[$__interval])` when used in Grafana.
- [Aggregate functions](#aggregate-functions) accept arbitrary number of args. For example, `avg(q1, q2, q3)` would return the average values for every point across time series returned by `q1`, `q2` and `q3`.
- [@ modifier](https://prometheus.io/docs/prometheus/latest/querying/basics/#modifier) can be put anywhere in the query. For example, `sum(foo) @ end()` calculates `sum(foo)` at the `end` timestamp of the selected time range `[start ... end]`.
- Arbitrary subexpression can be used as [@ modifier](https://prometheus.io/docs/prometheus/latest/querying/basics/#modifier). For example, `foo @ (end() - 1h)` calculates `foo` at the `end - 1 hour` timestamp on the selected time range `[start ... end]`.
- [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier), lookbehind window in square brackets and `step` value for [subquery](#subqueries) may refer to the current step aka `$__interval` value from Grafana with `[Ni]` syntax. For instance, `rate(metric[10i] offset 5i)` would return per-second rate over a range covering 10 previous steps with the offset of 5 steps.
- [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier) may be put anywere in the query. For instance, `sum(foo) offset 24h`.
- Lookbehind window in square brackets and [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier) may be fractional. For instance, `rate(node_network_receive_bytes_total[1.5m] offset 0.5d)`.
- The duration suffix is optional. The duration is in seconds if the suffix is missing. For example, `rate(m[300] offset 1800)` is equivalent to `rate(m[5m]) offset 30m`.
- The duration can be placed anywhere in the query. For example, `sum_over_time(m[1h]) / 1h` is equivalent to `sum_over_time(m[1h]) / 3600`.
- Trailing commas on all the lists are allowed - label filters, function args and with expressions. For instance, the following queries are valid: `m{foo="bar",}`, `f(a, b,)`, `WITH (x=y,) x`. This simplifies maintenance of multi-line queries.
- Metric names and metric labels may contain escaped chars. For instance, `foo\-bar{baz\=aa="b"}` is valid expression. It returns time series with name `foo-bar` containing label `baz=aa` with value `b`. Additionally, `\xXX` escape sequence is supported, where `XX` is hexadecimal representation of escaped char.
- Aggregate functions support optional `limit N` suffix in order to limit the number of output series. For example, `sum(x) by (y) limit 3` limits the number of output time series after the aggregation to 3. All the other time series are dropped.
- [histogram_quantile](#histogram_quantile) accepts optional third arg - `boundsLabel`. In this case it returns `lower` and `upper` bounds for the estimated percentile. See [this issue for details](https://github.com/prometheus/prometheus/issues/5706).
- `default` binary operator. `q1 default q2` fills gaps in `q1` with the corresponding values from `q2`.
- `if` binary operator. `q1 if q2` removes values from `q1` for missing values from `q2`.
- `ifnot` binary operator. `q1 ifnot q2` removes values from `q1` for existing values from `q2`.
- String literals may be concatenated. This is useful with `WITH` templates: `WITH (commonPrefix="long_metric_prefix_") {__name__=commonPrefix+"suffix1"} / {__name__=commonPrefix+"suffix2"}`.
- `WITH` templates. This feature simplifies writing and managing complex queries. Go to [WITH templates playground](https://play.victoriametrics.com/promql/expand-with-exprs) and try it.
- `keep_metric_names` modifier can be applied to all the [rollup functions](#rollup-functions) and [transform functions](#transform-functions). This modifier prevents from dropping metric names in function results. For example, `rate({__name__=~"foo|bar"}[5m]) keep_metric_names` leaves `foo` and `bar` metric names in the resulting time series.


## MetricsQL functions

If you are unfamiliar with PromQL, then please read [this tutorial](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) at first.

MetricsQL provides the following functions:

* [Rollup functions](#rollup-functions)
* [Transform functions](#transform-functions)
* [Label manipulation functions](#label-manipulation-functions)
* [Aggregate functions](#aggregate-functions)


### Rollup functions

**Rollup functions** (aka range functions or window functions) calculate rollups over **raw samples** on the given lookbehind window for the [selected time series](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). For example, `avg_over_time(temperature[24h])` calculates the average temperature over raw samples for the last 24 hours. Additional details:
  * If rollup functions are used for building graphs in Grafana, then the rollup is calculated independently per each point on the graph. For example, every point for `avg_over_time(temperature[24h])` graph shows the average temperature for the last 24 hours ending at this point. The interval between points is set as `step` query arg passed by Grafana to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).
  * If the given [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) returns multiple time series, then rollups are calculated individually per each returned series.
  * If lookbehind window in square brackets is missing, then MetricsQL automatically sets the lookbehind window to the interval between points on the graph (aka `step` query arg at [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries), `$__interval` value from Grafana or `1i` duration in MetricsQL). For example, `rate(http_requests_total)` is equivalent to `rate(http_requests_total[$__interval])` in Grafana. It is also equivalent to `rate(http_requests_total[1i])`.
  * Every [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) in MetricsQL must be wrapped into a rollup function. Otherwise it is automatically wrapped into [default_rollup](#default_rollup). For example, `foo{bar="baz"}` is automatically converted to `default_rollup(foo{bar="baz"}[1i])` before performing the calculations.
  * If something other than [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) is passed to rollup function, then the inner arg is automatically converted to a [subquery](#subqueries).
  * All the rollup functions accept optional `keep_metric_names` modifier. If it is set, then the function keeps metric names in results. For example, `rate({__name__=~"foo|bar}[5m]) keep_metric_names` leaves `foo` and `bar` metric names in results.

See also [implicit query conversions](#implicit-query-conversions).


#### absent_over_time

`absent_over_time(series_selector[d])` returns 1 if the given lookbehind window `d` doesn't contain raw samples. Otherwise it returns an empty result. This function is supported by PromQL. See also [present_over_time](#present_over_time).

#### aggr_over_time

`aggr_over_time(("rollup_func1", "rollup_func2", ...), series_selector[d])` calculates all the listed `rollup_func*` for raw samples on the given lookbehind window `d`. The calculations are perfomed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). `rollup_func*` can contain any rollup function. For instance, `aggr_over_time(("min_over_time", "max_over_time", "rate"), m[d])` would calculate [min_over_time](#min_over_time), [max_over_time](#max_over_time) and [rate](#rate) for `m[d]`.

#### ascent_over_time

`ascent_over_time(series_selector[d])` calculates ascent of raw sample values on the given lookbehind window `d`. The calculations are performed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Useful for tracking height gains in GPS tracking. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [descent_over_time](#descent_over_time).

#### avg_over_time

`avg_over_time(series_selector[d])` calculates the average value over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is supported by PromQL. See also [median_over_time](#median_over_time).

#### changes

`changes(series_selector[d])` calculates the number of times the raw samples changed on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Unlike `changes()` in Prometheus it takes into account the change from the last sample before the given lookbehind window `d`. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [changes_prometheus](#changes_prometheus).

#### changes_prometheus

`changes_prometheus(series_selector[d])` calculates the number of times the raw samples changed on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It doesn't take into account the change from the last sample before the given lookbehind window `d` in the same way as Prometheus does. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [changes](#changes).

#### count_eq_over_time

`count_eq_over_time(series_selector[d], eq)` calculates the number of raw samples on the given lookbehind window `d`, which are equal to `eq`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [count_over_time](#count_over_time).

#### count_gt_over_time

`count_gt_over_time(series_selector[d], gt)` calculates the number of raw samples on the given lookbehind window `d`, which are bigger than `gt`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [count_over_time](#count_over_time).

#### count_le_over_time

`count_le_over_time(series_selector[d], le)` calculates the number of raw samples on the given lookbehind window `d`, which don't exceed `le`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [count_over_time](#count_over_time).

#### count_ne_over_time

`count_ne_over_time(series_selector[d], ne)` calculates the number of raw samples on the given lookbehind window `d`, which aren't equal to `ne`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [count_over_time](#count_over_time).

#### count_over_time

`count_over_time(series_selector[d])` calculates the number of raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [count_le_over_time](#count_le_over_time), [count_gt_over_time](#count_gt_over_time), [count_eq_over_time](#count_eq_over_time) and [count_ne_over_time](#count_ne_over_time).

#### decreases_over_time

`decreases_over_time(series_selector[d])` calculates the number of raw sample value decreases over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [increases_over_time](#increases_over_time).

#### default_rollup

`default_rollup(series_selector[d])` returns the last raw sample value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors).

#### delta

`delta(series_selector[d])` calculates the difference between the last sample before the given lookbehind window `d` and the last sample at the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The behaviour of `delta()` function in MetricsQL is slighly different to the behaviour of `delta()` function in Prometheus. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [increase](#increase) and [delta_prometheus](#delta_prometheus).

#### delta_prometheus

`delta_prometheus(series_selector[d])` calculates the difference between the first and the last samples at the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The behaviour of `delta_prometheus()` is close to the behaviour of `delta()` function in Prometheus. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [delta](#delta).

#### deriv

`deriv(series_selector[d])` calculates per-second derivative over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The derivative is calculated using linear regression. Metric names are stripped from the resulting rollups. This function is supported by PromQL. Add `keep_metric_names` modifier in order to keep metric names. See also [deriv_fast](#deriv_fast) and [ideriv](#ideriv).

#### deriv_fast

`deriv_fast(series_selector[d])` calculates per-second derivative using the first and the last raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [deriv](#deriv) and [ideriv](#ideriv).

#### descent_over_time

`descent_over_time(series_selector[d])` calculates descent of raw sample values on the given lookbehind window `d`. The calculations are performed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Useful for tracking height loss in GPS tracking. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [ascent_over_time](#ascent_over_time).

#### distinct_over_time

`distinct_over_time(series_selector[d])` returns the number of distinct raw sample values on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### duration_over_time

`duration_over_time(series_selector[d], max_interval)` returns the duration in seconds when time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) were present over the given lookbehind window `d`. It is expected that intervals between adjacent samples per each series don't exceed the `max_interval`. Otherwise such intervals are considered as gaps and aren't counted. See also [lifetime](#lifetime) and [lag](#lag).

#### first_over_time

`first_over_time(series_selector[d])` returns the first raw sample value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). See also [last_over_time](#last_over_time) and [tfirst_over_time](#tfirst_over_time).

#### geomean_over_time

`geomean_over_time(series_selector[d])` calculates [geometric mean](https://en.wikipedia.org/wiki/Geometric_mean) over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### histogram_over_time

`histogram_over_time(series_selector[d])` calculates [VictoriaMetrics histogram](https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram) over raw samples on the given lookbehind window `d`. It is calculated individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The resulting histograms are useful to pass to [histogram_quantile](#histogram_quantile) for calculating quantiles over multiple gauges. For example, the following query calculates median temperature by country over the last 24 hours: `histogram_quantile(0.5, sum(histogram_over_time(temperature[24h])) by (vmrange,country))`.

#### hoeffding_bound_lower

`hoeffding_bound_lower(phi, series_selector[d])` calculates lower [Hoeffding bound](https://en.wikipedia.org/wiki/Hoeffding%27s_inequality) for the given `phi` in the range `[0...1]`. See also [hoeffding_bound_upper](#hoeffding_bound_upper).

#### hoeffding_bound_upper

`hoeffding_bound_upper(phi, series_selector[d])` calculates upper [Hoeffding bound](https://en.wikipedia.org/wiki/Hoeffding%27s_inequality) for the given `phi` in the range `[0...1]`. See also [hoeffding_bound_lower](#hoeffding_bound_lower).

#### holt_winters

`holt_winters(series_selector[d], sf, tf)` calculates Holt-Winters value (aka [double exponential smoothing](https://en.wikipedia.org/wiki/Exponential_smoothing#Double_exponential_smoothing)) for raw samples over the given lookbehind window `d` using the given smoothing factor `sf` and the given trend factor `tf`. Both `sf` and `tf` must be in the range `[0...1]`. It is expected that the [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) returns time series of [gauge type](https://prometheus.io/docs/concepts/metric_types/#gauge). This function is supported by PromQL.

#### idelta

`idelta(series_selector[d])` calculates the difference between the last two raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors).  Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### ideriv

`ideriv(series_selector[d])` calculates the per-second derivative based on the last two raw samples over the given lookbehind window `d`. The derivative is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [deriv](#deriv).

#### increase

`increase(series_selector[d])` calculates the increase over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that the `series_selector` returns time series of [counter type](https://prometheus.io/docs/concepts/metric_types/#counter). Unlike Prometheus it takes into account the last sample before the given lookbehind window `d` when calculating the result. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [increase_pure](#increase_pure), [increase_prometheus](#increase_prometheus) and [delta](#delta).

#### increase_prometheus

`increase_prometheus(series_selector[d])` calculates the increase over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that the `series_selector` returns time series of [counter type](https://prometheus.io/docs/concepts/metric_types/#counter). It doesn't take into account the last sample before the given lookbehind window `d` when calculating the result in the same way as Prometheus does. See [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [increase_pure](#increase_pure) and [increase](#increase).

#### increase_pure

`increase_pure(series_selector[d])` works the same as [increase](#increase) except of the following corner case - it assumes that [counters](https://prometheus.io/docs/concepts/metric_types/#counter) always start from 0, while [increase](#increase) ignores the first value in a series if it is too big.

#### increases_over_time

`increases_over_time(series_selector[d])` calculates the number of raw sample value increases over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [decreases_over_time](#decreases_over_time).

#### integrate

`integrate(series_selector[d])` calculates the integral over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### irate

`irate(series_selector[d])` calculates the "instant" per-second increase rate over the last two raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that the `series_selector` returns time series of [counter type](https://prometheus.io/docs/concepts/metric_types/#counter). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [rate](#rate).

#### lag

`lag(series_selector[d])` returns the duration in seconds between the last sample on the given lookbehind window `d` and the timestamp of the current point. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [lifetime](#lifetime) and [duration_over_time](#duration_over_time).

#### last_over_time

`last_over_time(series_selector[d])` returns the last raw sample value on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is supported by PromQL. See also [first_over_time](#first_over_time) and [tlast_over_time](#tlast_over_time).

#### lifetime

`lifetime(series_selector[d])` returns the duration in seconds between the last and the first sample on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [duration_over_time](#duration_over_time) and [lag](#lag).

#### max_over_time

`max_over_time(series_selector[d])` calculates the maximum value over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is supported by PromQL. See also [tmax_over_time](#tmax_over_time).

#### median_over_time

`median_over_time(series_selector[d])` calculates median value over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). See also [avg_over_time](#avg_over_time).

#### min_over_time

`min_over_time(series_selector[d])` calculates the minimum value over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is supported by PromQL. See also [tmin_over_time](#tmin_over_time).

#### mode_over_time

`mode_over_time(series_selector[d])` calculates [mode](https://en.wikipedia.org/wiki/Mode_(statistics)) for raw samples on the given lookbehind window `d`. It is calculated individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that raw sample values are discrete.

#### predict_linear

`predict_linear(series_selector[d], t)` calculates the value `t` seconds in the future using linear interpolation over raw samples on the given lookbehind window `d`. The predicted value is calculated individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is supported by PromQL.

#### present_over_time

`present_over_time(series_selector[d])` returns 1 if there is at least a single raw sample on the given lookbehind window `d`. Otherwise an empty result is returned. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### quantile_over_time

`quantile_over_time(phi, series_selector[d])` calculates `phi`-quantile over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The `phi` value must be in the range `[0...1]`. This function is supported by PromQL. See also [quantiles_over_time](#quantiles_over_time).

#### quantiles_over_time

`quantiles_over_time("phiLabel", phi1, ..., phiN, series_selector[d])` calculates `phi*`-quantiles over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). The function returns individual series per each `phi*` with `{phiLabel="phi*"}` label. `phi*` values must be in the range `[0...1]`. See also [quantile_over_time](#quantile_over_time).

#### range_over_time

`range_over_time(series_selector[d])` calculates value range over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). E.g. it calculates `max_over_time(series_selector[d]) - min_over_time(series_selector[d])`. Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### rate

`rate(series_selector[d])` calculates the average per-second increase rate over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that the `series_selector` returns time series of [counter type](https://prometheus.io/docs/concepts/metric_types/#counter). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### rate_over_sum

`rate_over_sum(series_selector[d])` calculates per-second rate over the sum of raw samples on the given lookbehind window `d`. The calculations are performed indiviually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### resets

`resets(series_selector[d])` returns the number of [counter](https://prometheus.io/docs/concepts/metric_types/#counter) resets over the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). It is expected that the `series_selector` returns time series of [counter type](https://prometheus.io/docs/concepts/metric_types/#counter). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### rollup

`rollup(series_selector[d])` calculates `min`, `max` and `avg` values for raw samples on the given lookbehind window `d`. These values are calculated individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors).

#### rollup_candlestick

`rollup_candlestick(series_selector[d])` calculates `open`, `high`, `low` and `close` values (aka OHLC) over raw samples on the given lookbehind window `d`. The calculations are perfomed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). This function is useful for financial applications.

#### rollup_delta

`rollup_delta(series_selector[d])` calculates differences between adjancent raw samples on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated differences. The calculations are performed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [rollup_increase](#rollup_increase).

#### rollup_deriv

`rollup_deriv(series_selector[d])` calculates per-second derivatives for adjancent raw samples on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated per-second derivatives. The calculations are performed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### rollup_increase

`rollup_increase(series_selector[d])` calculates increases for adjancent raw samples on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated increases. The calculations are performed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [rollup_delta](#rollup_delta).

#### rollup_rate

`rollup_rate(series_selector[d])` calculates per-second change rates for adjancent raw samples on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated per-second change rates. The calculations are perfomed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### rollup_scrape_interval

`rollup_scrape_interval(series_selector[d])` calculates the interval in seconds between adjancent raw samples on the given lookbehind window `d` and returns `min`, `max` and `avg` values for the calculated interval. The calculations are perfomed individually per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [scrape_interval](#scrape_interval).

#### scrape_interval

`scrape_interval(series_selector[d])` calculates the average interval in seconds between raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [rollup_scrape_interval](#rollup_scrape_interval).

#### share_gt_over_time

`share_gt_over_time(series_selector[d], gt)` returns share (in the range `[0...1]`) of raw samples on the given lookbehind window `d`, which are bigger than `gt`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. Useful for calculating SLI and SLO. Example: `share_gt_over_time(up[24h], 0)` - returns service availability for the last 24 hours. See also [share_le_over_time](#share_le_over_time).

#### share_le_over_time

`share_le_over_time(series_selector[d], le)` returns share (in the range `[0...1]`) of raw samples on the given lookbehind window `d`, which are smaller or equal to `le`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. Useful for calculating SLI and SLO. Example: `share_le_over_time(memory_usage_bytes[24h], 100*1024*1024)` returns the share of time series values for the last 24 hours when memory usage was below or equal to 100MB. See also [share_gt_over_time](#share_gt_over_time).

#### stale_samples_over_time

`stale_samples_over_time(series_selector[d])` calculates the number of [staleness markers](https://docs.victoriametrics.com/vmagent.html#prometheus-staleness-markers) on the given lookbehind window `d` per each time series matching the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### stddev_over_time

`stddev_over_time(series_selector[d])` calculates standard deviation over raw samples on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [stdvar_over_time](#stdvar_over_time).

#### stdvar_over_time

`stdvar_over_time(series_selector[d])` calculates stadnard variance over raw samples on the given lookbheind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [stddev_over_time](#stddev_over_time).

#### sum_over_time

`sum_over_time(series_selector[d])` calculates the sum of raw sample values on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### sum2_over_time

`sum2_over_time(series_selector[d])` calculates the sum of squares for raw sample values on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.

#### timestamp

`timestamp(series_selector[d])` returns the timestamp in seconds for the last raw sample on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [timestamp_with_name](#timestamp_with_name).

#### timestamp_with_name

`timestamp_with_name(series_selector[d])` returns the timestamp in seconds for the last raw sample on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are preserved in the resulting rollups. See also [timestamp](#timestamp).

#### tfirst_over_time

`tfirst_over_time(series_selector[d])` returns the timestamp in seconds for the first raw sample on the given lookbehind window `d` per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [first_over_time](#first_over_time).

#### tlast_over_time

`tlast_over_time(series_selector[d])` is an alias for [timestamp](#timestamp).

#### tmax_over_time

`tmax_over_time(series_selector[d])` returns the timestamp in seconds for the raw sample with the maximum value on the given lookbehind window `d`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [max_over_time](#max_over_time).

#### tmin_over_time

`tmin_over_time(series_selector[d])` returns the timestamp in seconds for the raw sample with the minimum value on the given lookbehind window `d`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names. See also [min_over_time](#min_over_time).

#### zscore_over_time

`zscore_over_time(series_selector[d])` calculates returns [z-score](https://en.wikipedia.org/wiki/Standard_score) for raw samples on the given lookbehind window `d`. It is calculated independently per each time series returned from the given [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). Metric names are stripped from the resulting rollups. Add `keep_metric_names` modifier in order to keep metric names.


### Transform functions

**Transform functions** calculate transformations over rollup results. For example, `abs(delta(temperature[24h]))` calculates the absolute value for every point of every time series returned from the rollup `delta(temperature[24h])`. Additional details:
  * If transform function is applied directly to a [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors), then the [default_rollup()](#default_rollup) function is automatically applied before calculating the transformations. For example, `abs(temperature)` is implicitly transformed to `abs(default_rollup(temperature[1i]))`.
  * All the transform functions accept optional `keep_metric_names` modifier. If it is set, then the function doesn't drop metric names from the resulting time series. For example, `ln({__name__=~"foo|bar"}) keep_metric_names` leaves `foo` and `bar` metric names in results.

See also [implicit query conversions](#implicit-query-conversions).


#### abs

`abs(q)` calculates the absolute value for every point of every time series returned by `q`. This function is supported by PromQL.

#### absent

`absent(q)` returns 1 if `q` has no points. Otherwise returns an empty result. This function is supported by PromQL.

#### acos

`acos(q)` returns [inverse cosine](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [asin](#asin) and [cos](#cos).

#### acosh

`acosh(q)` returns [inverse hyperbolic cosine](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_cosine) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [sinh](#cosh).

#### asin

`asin(q)` returns [inverse sine](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [acos](#acos) and [sin](#sin).

#### asinh

`asinh(q)` returns [inverse hyperbolic sine](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_sine) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [sinh](#sinh).

#### atan

`atan(q)` returns [inverse tangent](https://en.wikipedia.org/wiki/Inverse_trigonometric_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [tan](#tan).

#### atanh

`atanh(q)` returns [inverse hyperbolic tangent](https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_tangent) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [tanh](#tanh).

#### bitmap_and

`bitmap_and(q, mask)` - calculates bitwise `v & mask` for every `v` point of every time series returned from `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names.

#### bitmap_or

`bitmap_or(q, mask)` calculates bitwise `v | mask` for every `v` point of every time series returned from `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names.

#### bitmap_xor

`bitmap_xor(q, mask)` calculates bitwise `v ^ mask` for every `v` point of every time series returned from `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names.

#### buckets_limit

`buckets_limit(limit, buckets)` limits the number of [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) to the given `limit`. See also [prometheus_buckets](#prometheus_buckets) and [histogram_quantile](#histogram_quantile).

#### ceil

`ceil(q)` rounds every point for every time series returned by `q` to the upper nearest integer. This function is supported by PromQL. See also [floor](#floor) and [round](#round).

#### clamp

`clamp(q, min, max)` clamps every point for every time series returned by `q` with the given `min` and `max` values. This function is supported by PromQL. See also [clamp_min](#clamp_min) and [clamp_max](#clamp_max).

#### clamp_max

`clamp_max(q, max)` clamps every point for every time series returned by `q` with the given `max` value. This function is supported by PromQL. See also [clamp](#clamp) and [clamp_min](#clamp_min).

#### clamp_min

`clamp_min(q, min)` clamps every pount for every time series returned by `q` with the given `min` value. This function is supported by PromQL. See also [clamp](#clamp) and [clamp_max](#clamp_max).

#### cos

`cos(q)` returns `cos(v)` for every `v` point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [sin](#sin).

#### cosh

`cosh(q)` returns [hyperbolic cosine](https://en.wikipedia.org/wiki/Hyperbolic_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. This function is supported by PromQL. See also [acosh](#acosh).

#### day_of_month

`day_of_month(q)` returns the day of month for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[1...31]`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### day_of_week

`day_of_week(q)` returns the day of week for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[0...6]`, where `0` means Sunday and `6` means Saturday. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### days_in_month

`days_in_month(q)` returns the number of days in the month identified by every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[28...31]`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### deg

`deg(q)` converts [Radians to degrees](https://en.wikipedia.org/wiki/Radian#Conversions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [rad](#rad).

#### end

`end()` returns the unix timestamp in seconds for the last point. See also [start](#start). It is known as `end` query arg passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

#### exp

`exp(q)` calculates the `e^v` for every point `v` of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. See also [ln](#ln). This function is supported by PromQL.

#### floor

`floor(q)` rounds every point for every time series returned by `q` to the lower nearest integer. See also [ceil](#ceil) and [round](#round). This function is supported by PromQL.

#### histogram_avg

`histogram_avg(buckets)` calculates the average value for the given `buckets`. It can be used for calculating the average over the given time range across multiple time series. For exmple, `histogram_avg(sum(histogram_over_time(response_time_duration_seconds[5m])) by (vmrange,job))` would return the average response time per each `job` over the last 5 minutes.

#### histogram_quantile

`histogram_quantile(phi, buckets)` calculates `phi`-quantile over the given [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350). `phi` must be in the range `[0...1]`. For example, `histogram_quantile(0.5, sum(rate(http_request_duration_seconds_bucket[5m]) by (le))` would return median request duration for all the requests during the last 5 minutes. It accepts optional third arg - `boundsLabel`. In this case it returns `lower` and `upper` bounds for the estimated percentile. See [this issue for details](https://github.com/prometheus/prometheus/issues/5706). This function is supported by PromQL (except of the `boundLabel` arg). See also [histogram_quantiles](#histogram_quantiles) and [histogram_share](#histogram_share).

#### histogram_quantiles

`histogram_quantiles("phiLabel", phi1, ..., phiN, buckets)` calculates the given `phi*`-quantiles over the given [histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350). `phi*` must be in the range `[0...1]`. Each calculated quantile is returned in a separate time series with the corresponding `{phiLabel="phi*"}` label. See also [histogram_quantile](#histogram_quantile).

#### histogram_share

`histogram_share(le, buckets)` calculates the share (in the range `[0...1]`) for `buckets` that fall below `le`. Useful for calculating SLI and SLO. This is inverse to [histogram_quantile](#histogram_quantile).

#### histogram_stddev

`histogram_stddev(buckets)` calculates standard deviation for the given `buckets`.

#### histogram_stdvar

`histogram_stdvar(buckets)` calculates standard variance for the given `buckets`. It can be used for calculating standard deviation over the given time range across multiple time series. For example, `histogram_stdvar(sum(histogram_over_time(temperature[24])) by (vmrange,country))` would return standard deviation for the temperature per each country over the last 24 hours.

#### hour

`hour(q)` returns the hour for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[0...23]`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### interpolate

`interpolate(q)` fills gaps with linearly interpolated values calculated from the last and the next non-empty points per each time series returned by `q`. See also [keep_last_value](#keep_last_value) and [keep_next_value](#keep_next_value).

#### keep_last_value

`keep_last_value(q)` fills gaps with the value of the last non-empty point in every time series returned by `q`. See also [keep_next_value](#keep_next_value) and [interpolate](#interpolate).

#### keep_next_value

`keep_next_value(q)` fills gaps with the value of the next non-empty point in every time series returned by `q`. See also [keep_last_value](#keep_last_value) and [interpolate](#interpolate).

#### limit_offset

`limit_offset(limit, offset, q)` skips `offset` time series from series returned by `q` and then returns up to `limit` of the remaining time series per each group. This allows implementing simple paging for `q` time series. See also [limitk](#limitk).

#### ln

`ln(q)` calculates `ln(v)` for every point `v` of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [exp](#exp) and [log2](#log2).

#### log2

`log2(q)` calculates `log2(v)` for every point `v` of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [log10](#log10) and [ln](#ln).

#### log10

`log10(q)` calculates `log10(v)` for every point `v` of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [log2](#log2) and [ln](#ln).

#### minute

`minute(q)` returns the minute for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[0...59]`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### month

`month(q)` returns the month for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. The returned values are in the range `[1...12]`, where `1` means January and `12` means December. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### now

`now()` returns the current timestamp as a floating-point value in seconds. See also [time](#time).

#### pi

`pi()` returns [Pi number](https://en.wikipedia.org/wiki/Pi). This function is supported by PromQL.

#### rad

`rad(q)` converts [degrees to Radians](https://en.wikipedia.org/wiki/Radian#Conversions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL. See also [deg](#deg).


#### prometheus_buckets

`prometheus_buckets(buckets)` converts [VictoriaMetrics histogram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) with `vmrange` labels to Prometheus histogram buckets with `le` labels. This may be useful for building heatmaps in Grafana. See also [histogram_quantile](#histogram_quantile) and [buckets_limit](#buckets_limit).

#### rand

`rand(seed)` returns pseudo-random numbers on the range `[0...1]` with even distribution. Optional `seed` can be used as a seed for pseudo-random number generator. See also [rand_normal](#rand_normal) and [rand_exponential](#rand_exponential).

#### rand_exponential

`rand_exponential(seed)` returns pseudo-random numbers with [exponential distribution](https://en.wikipedia.org/wiki/Exponential_distribution). Optional `seed` can be used as a seed for pseudo-random number generator. See also [rand](#rand) and [rand_normal](#rand_normal).

#### rand_normal

`rand_normal(seed)` returns pesudo-random numbers with [normal distribution](https://en.wikipedia.org/wiki/Normal_distribution). Optional `seed` can be used as a seed for pseudo-random number generator. See also [rand](#rand) and [rand_exponential](#rand_exponential).

#### range_avg

`range_avg(q)` calculates the avg value across points per each time series returned by `q`.

#### range_first

`range_first(q)` returns the value for the first point per each time series returned by `q`.

#### range_last

`range_last(q)` returns the value for the last point per each time series returned by `q`.

#### range_max

`range_max(q)` calculates the max value across points per each time series returned by `q`.

#### range_median

`range_median(q)` calculates the median value across points per each time series returned by `q`.

#### range_min

`range_min(q)` calculates the min value across points per each time series returned by `q`.

#### range_quantile

`range_quantile(phi, q)` returns `phi`-quantile across points per each time series returned by `q`. `phi` must be in the range `[0...1]`.

#### range_sum

`range_sum(q)` calculates the sum of points per each time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names.

#### remove_resets

`remove_resets(q)` removes counter resets from time series returned by `q`.

#### round

`round(q, nearest)` round every point of every time series returned by `q` to the `nearest` multiple. If `nearest` is missing then the rounding is performed to the nearest integer. This function is supported by PromQL. See also [floor](#floor) and [ceil](#ceil).

#### ru

`ru(free, max)` calculates resource utilization in the range `[0%...100%]` for the given `free` and `max` resources. For instance, `ru(node_memory_MemFree_bytes, node_memory_MemTotal_bytes)` returns memory utilization over [node_exporter](https://github.com/prometheus/node_exporter) metrics.

#### running_avg

`running_avg(q)` calculates the running avg per each time series returned by `q`.

#### running_max

`running_max(q)` calculates the running max per each time series returned by `q`.

#### running_min

`running_min(q)` calculates the running min per each time series returned by `q`.

#### running_sum

`running_sum(q)` calculates the running sum per each time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names.

#### scalar

`scalar(q)` returns `q` if `q` contains only a single time series. Otherwise it returns nothing. This function is supported by PromQL.

#### sgn

`sgn(q)` returns `1` if `v>0`, `-1` if `v<0` and `0` if `v==0` for every point `v` of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### sin

`sin(q)` returns `sin(v)` for every `v` point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by MetricsQL. See also [cos](#cos).

#### sinh

`sinh(q)` returns [hyperbolic sine](https://en.wikipedia.org/wiki/Hyperbolic_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by MetricsQL. See also [cosh](#cosh).

#### tan

`tan(q)` returns `tan(v)` for every `v` point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by MetricsQL. See also [atan](#atan).

#### tanh

`tanh(q)` returns [hyperbolic tangent](https://en.wikipedia.org/wiki/Hyperbolic_functions) for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by MetricsQL. See also [atanh](#atanh).

#### smooth_exponential

`smooth_exponential(q, sf)` smooths points per each time series returned by `q` using [exponential moving average](https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average) with the given smooth factor `sf`.

#### sort

`sort(q)` sorts series in ascending order by the last point in every time series returned by `q`. This function is supported by PromQL. See also [sort_desc](#sort_desc).

#### sort_by_label

`sort_by_label(q, label1, ... labelN)` sorts series in ascending order by the given set of labels. For example, `sort_by_label(foo, "bar")` would sort `foo` series by values of the label `bar` in these series. See also [sort_by_label_desc](#sort_by_label_desc).

#### sort_by_label_desc

`sort_by_label_desc(q, label1, ... labelN)` sorts series in descending order by the given set of labels. For example, `sort_by_label(foo, "bar")` would sort `foo` series by values of the label `bar` in these series. See also [sort_by_label](#sort_by_label).

#### sort_desc

`sort_desc(q)` sorts series in descending order by the last point in every time series returned by `q`. This function is supported by PromQL. See also [sort](#sort).

#### sqrt

`sqrt(q)` calculates square root for every point of every time series returned by `q`. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.

#### start

`start()` returns unix timestamp in seconds for the first point. See also [end](#end). It is known as `start` query arg passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

#### step

`step()` returns the step in seconds (aka interval) between the returned points. It is known as `step` query arg passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

#### time

`time()` returns unix timestamp for every returned point. This function is supported by PromQL.

#### timezone_offset

`timezone_offset(tz)` returns offset in seconds for the given timezone `tz` relative to UTC. This can be useful when combining with datetime-related functions. For example, `day_of_week(time()+timezone_offset("America/Los_Angeles"))` would return weekdays for `America/Los_Angeles` time zone. Special `Local` time zone can be used for returning an offset for the time zone set on the host where VictoriaMetrics runs. See [the list of supported timezones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

#### ttf

`ttf(free)` estimates the time in seconds needed to exhaust `free` resources. For instance, `ttf(node_filesystem_avail_byte)` returns the time to storage space exhaustion. This function may be useful for capacity planning.

#### union

`union(q1, ..., qN)` returns a union of time series returned from `q1`, ..., `qN`. The `union` function name can be skipped - the following queries are quivalent: `union(q1, q2)` and `(q1, q2)`. It is expected that each `q*` query returns time series with unique sets of labels. Otherwise only the first time series out of series with identical set of labels is returned. Use [alias](#alias) and [label_set](#label_set) functions for giving unique labelsets per each `q*` query:

#### vector

`vector(q)` returns `q`, e.g. it does nothing in MetricsQL. This function is supported by PromQL.

#### year

`year(q)` returns the year for every point of every time series returned by `q`. It is expected that `q` returns unix timestamps. Metric names are stripped from the resulting series. Add `keep_metric_names` modifier in order to keep metric names. This function is supported by PromQL.


### Label manipulation functions

**Label manipulation functions** perform manipulations with lables on the selected rollup results. Additional details:
  * If label manipulation function is applied directly to a [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors), then the [default_rollup()](#default_rollup) function is automatically applied before performing the label transformation. For example, `alias(temperature, "foo")` is implicitly transformed to `alias(default_rollup(temperature[1i]), "foo")`.

See also [implicit query conversions](#implicit-query-conversions).


#### alias

`alias(q, "name")` sets the given `name` to all the time series returned by `q`. For example, `alias(up, "foobar")` would rename `up` series to `foobar` series.

#### label_copy

`label_copy(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")` copies label values from `src_label*` to `dst_label*` for all the time series returned by `q`. If `src_label` is empty, then the corresponding `dst_label` is left untouched.

#### label_del

`label_del(q, "label1", ..., "labelN")` deletes the given `label*` labels from all the time series returned by `q`.

#### label_graphite_group

`label_graphite_group(q, groupNum1, ... groupNumN)` replaces metric names returned from `q` with the given Graphite group values concatenated via `.` char. For example, `label_graphite_group({__graphite__="foo*.bar.*"}, 0, 2)` would substitute `foo<any_value>.bar.<other_value>` metric names with `foo<any_value>.<other_value>`. This function is useful for aggregating Graphite metrics with [aggregate functions](#aggregate-functions). For example, the following query would return per-app memory usage:

```
sum by (__name__) (
    label_graphite_group({__graphite__="app*.host*.memory_usage"}, 0)
)
```

#### label_join

`label_join(q, "dst_label", "separator", "src_label1", ..., "src_labelN")` joins `src_label*` values with the given `separator` and stores the result in `dst_label`. This is performed individually per each time series returned by `q`. For example, `label_join(up{instance="xxx",job="yyy"}, "foo", "-", "instance", "job")` would store `xxx-yyy` label value into `foo` label. This function is supported by PromQL.

#### label_keep

`label_keep(q, "label1", ..., "labelN")` deletes all the labels except of the listed `label*` labels in all the time series returned by `q`.

#### label_lowercase

`label_lowercase(q, "label1", ..., "labelN")` lowercases values for the given `label*` labels in all the time series returned by `q`.

#### label_map

`label_map(q, "label", "src_value1", "dst_value1", ..., "src_valueN", "dst_valueN")` maps `label` values from `src_*` to `dst*` for all the time seires returned by `q`.

#### label_match

`label_match(q, "label", "regexp")` drops time series from `q` with `label` not matching the given `regexp`. This function can be useful after [rollup](#rollup)-like functions, which may return multiple time series for every input series. See also [label_mismatch](#label_mismatch).

#### label_mismatch

`label_mismatch(q, "label", "regexp")` drops time series from `q` with `label` matching the given `regexp`. This function can be useful after [rollup](#rollup)-like functions, which may return multiple time series for every input series. See also [label_match](#label_match).

#### label_move

`label_move(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")` moves label values from `src_label*` to `dst_label*` for all the time series returned by `q`. If `src_label` is empty, then the corresponding `dst_label` is left untouched.

#### label_replace

`label_replace(q, "dst_label", "replacement", "src_label", "regex")` applies the given `regex` to `src_label` and stores the `replacement` in `dst_label` if the given `regex` matches `src_label`. The `replacement` may contain references to regex captures such as `$1`, `$2`, etc. These references are substituted by the corresponding regex captures. For example, `label_replace(up{job="node-exporter"}, "foo", "bar-$1", "job", "node-(.+)")` would store `bar-node-exporter` label value into `foo` label. This function is supported by PromQL.

#### label_set

`label_set(q, "label1", "value1", ..., "labelN", "valueN")` sets `{label1="value1", ..., labelN="valueN"}` labels to all the time series returned by `q`.

#### label_transform

`label_transform(q, "label", "regexp", "replacement")` substitutes all the `regexp` occurences by the given `replacement` in the given `label`.

#### label_uppercase

`label_uppercase(q, "label1", ..., "labelN")` uppercases values for the given `label*` labels in all the time series returned by `q`. 

#### label_value

`label_value(q, "label")` returns number values for the given `label` for every time series returned by `q`. For example, if `label_value(foo, "bar")` is applied to `foo{bar="1.234"}`, then it will return a time series `foo{bar="1.234"}` with `1.234` value.


### Aggregate functions

**Aggregate functions** calculate aggregates over groups of rollup results. Additional details:
  * By default a single group is used for aggregation. Multiple independent groups can be set up by specifying grouping labels in `by` and `without` modifiers. For example, `count(up) by (job)` would group rollup results by `job` label value and calculate the [count](#count) aggregate function independently per each group, while `count(up) without (instance)` would group rollup results by all the labels except `instance` before calculating [count](#count) aggregate function independently per each group. Multiple labels can be put in `by` and `without` modifiers.
  * If the aggregate function is applied directly to a [series_selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors), then the [default_rollup()](#default_rollup) function is automatically applied before cacluating the aggregate. For example, `count(up)` is implicitly transformed to `count(default_rollup(up[1i]))`.
  * Aggregate functions accept arbitrary number of args. For example, `avg(q1, q2, q3)` would return the average values for every point across time series returned by `q1`, `q2` and `q3`.
  * Aggregate functions support optional `limit N` suffix, which can be used for limiting the number of output groups. For example, `sum(x) by (y) limit 3` limits the number of groups for the aggregation to 3. All the other groups are ignored.

See also [implicit query conversions](#implicit-query-conversions).


#### any

`any(q) by (group_labels)` returns a single series per `group_labels` out of time series returned by `q`. See also [group](#group).

#### avg

`avg(q) by (group_labels)` returns the average value per `group_labels` for time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### bottomk

`bottomk(k, q)` returns up to `k` points with the smallest values across all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL. See also [topk](#topk).

#### bottomk_avg

`bottomk_avg(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the smallest averages. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `bottomk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the smallest averages plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [topk_avg](#topk_avg).

#### bottomk_last

`bottomk_last(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the smallest last values. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the smallest maximums plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [topk_last](#topk_last).

#### bottomk_max

`bottomk_max(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the smallest maximums. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the smallest maximums plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [topk_max](#topk_max).

#### bottomk_median

`bottomk_median(k, q, "other_label=other_value")` returns up to `k` time series from `q with the smallest medians. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `bottomk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the smallest medians plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [topk_median](#topk_median).

#### bottomk_min

`bottomk_min(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the smallest minimums. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `bottomk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the smallest minimums plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [topk_min](#topk_min).

#### count

`count(q) by (group_labels)` returns the number of non-empty points per `group_labels` for time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### count_values

`count_values("label", q)` counts the number of points with the same value and stores the counts in a time series with an additional `label`, wich contains each initial value. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### distinct

`distinct(q)` calculates the number of unique values per each group of points with the same timestamp.

#### geomean

`geomean(q)` calculates geometric mean per each group of points with the same timestamp.

#### group

`group(q) by (group_labels)` returns `1` per each `group_labels` for time series returned by `q`. This function is supported by PromQL. See also [any](#any).

#### histogram

`histogram(q)` calculates [VictoriaMetrics histogram](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) per each group of points with the same timestamp. Useful for visualizing big number of time series via a heatmap. See [this article](https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) for more details.

#### limitk

`limitk(k, q) by (group_labels)` returns up to `k` time series per each `group_labels` out of time series returned by `q`. The returned set of time series remain the same across calls. See also [limit_offset](#limit_offset).

#### mad

`mad(q) by (group_labels)` returns the [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. See also [outliers_mad](#outliers_mad) and [stddev](#stddev).

#### max

`max(q) by (group_labels)` returns the maximum value per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### median

`median(q) by (group_labels)` returns the median value per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### min

`min(q) by (group_labels)` returns the minimum value per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### mode

`mode(q) by (group_labels)` returns [mode](https://en.wikipedia.org/wiki/Mode_(statistics)) per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### outliers_mad

`outliers_mad(tolerance, q)` returns time series from `q` with at least a single point outside [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) (aka MAD) multiplied by `tolerance`. E.g. it returns time series with at least a single point below `median(q) - mad(q)` or a single point above `median(q) + mad(q)`. See also [outliersk](#outliersk) and [mad](#mad).

#### outliersk

`outliersk(k, q)` returns up to `k` time series with the biggest standard deviation (aka outliers) out of time series returned by `q`. See also [outliers_mad](#outliers_mad).

#### quantile

`quantile(phi, q) by (group_labels)` calculates `phi`-quantile per each `group_labels` for all the time series returned by `q`. `phi` must be in the range `[0...1]`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL. See also [quantiles](#quantiles).

#### quantiles

`quantiles("phiLabel", phi1, ..., phiN, q)` calculates `phi*`-quantiles for all the time series returned by `q` and return them in time series with `{phiLabel="phi*"}` label. `phi*` must be in the range `[0...1]`. The aggregate is calculated individually per each group of points with the same timestamp. See also [quantile](#quantile).

#### stddev

`stddev(q) by (group_labels)` calculates standard deviation per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### stdvar

`stdvar(q) by (group_labels)` calculates standard variance per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### sum

`sum(q) by (group_labels)` returns the sum per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL.

#### sum2

`sum2(q) by (group_labels)` calculates the sum of squares per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp.

#### topk

`topk(k, q)` returns up to `k` points with the biggest values across all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. This function is supported by PromQL. See also [bottomk](#bottomk).

#### topk_avg

`topk_avg(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the biggest averages. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `topk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest averages plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [bottomk_avg](#bottomk_avg).

#### topk_last

`topk_last(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the biggest last values. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest amaximums plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [bottomk_last](#bottomk_last).

#### topk_max

`topk_max(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the biggest maximums. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest amaximums plus a time series with `{job="other"}` label with the sum of the remaining series if any. See also [bottomk_max](#bottomk_max).

#### topk_median

`topk_median(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the biggest medians. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `topk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest medians plus a time series with `{job="other"}` label with the sum of the remaining series if any.  See also [bottomk_median](#bottomk_median).

#### topk_min

`topk_min(k, q, "other_label=other_value")` returns up to `k` time series from `q` with the biggest minimums. If an optional `other_label=other_value` arg is set, then the sum of the remaining time series is returned with the given label. For example, `topk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")` would return up to 3 time series with the biggest minimums plus a time series with `{job="other"}` label with the sum of the remaining series if any.  See also [bottomk_min](#bottomk_min).

#### zscore

`zscore(q) by (group_labels)` returns [z-score](https://en.wikipedia.org/wiki/Standard_score) values per each `group_labels` for all the time series returned by `q`. The aggregate is calculated individually per each group of points with the same timestamp. Useful for detecting anomalies in the group of related time series.


## Subqueries

MetricsQL supports and extends PromQL subqueries. See [this article](https://valyala.medium.com/prometheus-subqueries-in-victoriametrics-9b1492b720b3) for details. Any [rollup function](#rollup-functions) for something other than [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) form a subquery. Nested rollup functions can be implicit thanks to the [implicit query conversions](#implicit-query-conversions). For example, `delta(sum(m))` is implicitly converted to `delta(sum(default_rollup(m[1i]))[1i:1i])`, so it becomes a subquery, since it contains [default_rollup](#default_rollup) nested into [delta](#delta).

VictoriaMetrics performs subqueries in the following way:

* It calculates the inner rollup function using the `step` value from the outer rollup function. For example, for expression `max_over_time(rate(http_requests_total[5m])[1h:30s])` the inner function `rate(http_requests_total[5m])` is calculated with `step=30s`. The resulting data points are aligned by the `step`.
* It calculates the outer rollup function over the results of the inner rollup function using the `step` value passed by Grafana to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).


## Implicit query conversions

VictoriaMetrics performs the following implicit conversions for incoming queries before starting the calculations:

* If lookbehind window in square brackets is missing inside [rollup function](#rollup-functions), then `[1i]` is automatically added there. The `[1i]` means one `step` value, which is passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries). It is also known as `$__interval` in Grafana. For example, `rate(http_requests_count)` is automatically transformed to `rate(http_requests_count[1i])`.
* All the [series selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors), which aren't wrapped into [rollup functions](#rollup-functions), are automatically wrapped into [default_rollup](#default_rollup) function. Examples:
  * `foo` is transformed to `default_rollup(foo[1i])`
  * `foo + bar` is transformed to `default_rollup(foo[1i]) + default_rollup(bar[1i])`
  * `count(up)` is transformed to `count(default_rollup(up[1i]))`, because [count](#count) isn't a [rollup function](#rollup-functions) - it is [aggregate function](#aggregate-functions)
  * `abs(temperature)` is transformed to `abs(default_rollup(temperature[1i]))`, because [abs](#abs) isn't a [rollup function](#rollup-functions) - it is [transform function](#transform-functions)
* If `step` in square brackets is missing inside [subquery](#subqueries), then `1i` step is automatically added there. For example, `avg_over_time(rate(http_requests_total[5m])[1h])` is automatically converted to `avg_over_time(rate(http_requests_total[5m])[1h:1i])`.
* If something other than [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) is passed to [rollup function](#rollup-functions), then a [subquery](#subqueries) with `1i` lookbehind window and `1i` step is automatically formed. For example, `rate(sum(up))` is automatically converted to `rate((sum(default_rollup(up[1i])))[1i:1i])`.
