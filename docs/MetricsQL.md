# MetricsQL

VictoriaMetrics implements MetricsQL - query language inspired by [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/).
It is backwards compatible with PromQL, so Grafana dashboards backed by Prometheus datasource should work the same after switching from Prometheus to VictoriaMetrics.
[Standalone MetricsQL package](https://godoc.org/github.com/VictoriaMetrics/metricsql) can be used for parsing MetricsQL in external apps.

If you are unfamiliar with PromQL, then it is suggested reading [this tutorial for beginners](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085).

The following functionality is implemented differently in MetricsQL comparing to PromQL in order to improve user experience:
* MetricsQL takes into account the previous point before the window in square brackets for range functions such as `rate` and `increase`.
  It also doesn't extrapolate range function results. This addresses [this issue from Prometheus](https://github.com/prometheus/prometheus/issues/3746).
* MetricsQL returns the expected non-empty responses for requests with `step` values smaller than scrape interval. This addresses [this issue from Grafana](https://github.com/grafana/grafana/issues/11451).
* MetricsQL treats `scalar` type the same as `instant vector` without labels, since subtle difference between these types usually confuses users.
  See [the corresponding Prometheus docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#expression-language-data-types) for details.
* MetricsQL removes all the `NaN` values from the output, so some queries like `(-1)^0.5` return empty results in VictoriaMetrics, while returning
  a series of `NaN` values in Prometheus. Note that Grafana doesn't draw any lines or dots for `NaN` values, so usually the end result looks the same for both
  VictoriaMetrics and Prometheus.
* MetricsQL keeps metric names after applying functions, which don't change the meaining of the original time series. For example, `min_over_time(foo)` or `round(foo)`
  leave `foo` metric name in the result. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/674) for details.

Other PromQL functionality should work the same in MetricsQL. [File an issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues)
if you notice discrepancies between PromQL and MetricsQL results other than mentioned above.

MetricsQL provides additional functionality mentioned below, which is aimed towards solving practical cases.
Feel free [filing a feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you think MetricsQL misses certain useful functionality.

*Note that the functionality mentioned below doesn't work in PromQL, so it is impossible switching back to Prometheus after you start using it.*

This functionality can be tried at [an editable Grafana dashboard](http://play-grafana.victoriametrics.com:3000/d/4ome8yJmz/node-exporter-on-victoriametrics-demo).

- [`WITH` templates](https://play.victoriametrics.com/promql/expand-with-exprs). This feature simplifies writing and managing complex queries. Go to [`WITH` templates playground](https://play.victoriametrics.com/promql/expand-with-exprs) and try it.
- Graphite-compatible filters can be passed via `{__graphite__="foo.*.bar"}` syntax. This is equivalent to `{__name__=~"foo[.][^.]*[.]bar"}`, but usually works faster and is easier to use when migrating from Graphite to VictoriaMetrics.
- Range duration in functions such as [rate](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate()) may be omitted. VictoriaMetrics automatically selects range duration depending on the current step used for building the graph. For instance, the following query is valid in VictoriaMetrics: `rate(node_network_receive_bytes_total)`.
- All the aggregate functions support optional `limit N` suffix in order to limit the number of output series. For example, `sum(x) by (y) limit 10` limits
  the number of output time series after the aggregation to 10. All the other time series are dropped.
- Metric names and metric labels may contain escaped chars. For instance, `foo\-bar{baz\=aa="b"}` is valid expression. It returns time series with name `foo-bar` containing label `baz=aa` with value `b`. Additionally, `\xXX` escape sequence is supported, where `XX` is hexadecimal representation of escaped char.
- `offset`, range duration and step value for range vector may refer to the current step aka `$__interval` value from Grafana.
  For instance, `rate(metric[10i] offset 5i)` would return per-second rate over a range covering 10 previous steps with the offset of 5 steps.
- `offset` may be put anywere in the query. For instance, `sum(foo) offset 24h`.
- `offset` may be negative. For example, `q offset -1h`.
- [Range duration](https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors) and [offset](https://prometheus.io/docs/prometheus/latest/querying/basics/#offset-modifier) may be fractional. For instance, `rate(node_network_receive_bytes_total[1.5m] offset 0.5d)`.
- `default` binary operator. `q1 default q2` fills gaps in `q1` with the corresponding values from `q2`.
- Most aggregate functions accept arbitrary number of args. For example, `avg(q1, q2, q3)` would return the average values for every point across `q1`, `q2` and `q3`.
- `histogram_quantile` accepts optional third arg - `boundsLabel`. In this case it returns `lower` and `upper` bounds for the estimated percentile. See [this issue for details](https://github.com/prometheus/prometheus/issues/5706).
- `if` binary operator. `q1 if q2` removes values from `q1` for missing values from `q2`.
- `ifnot` binary operator. `q1 ifnot q2` removes values from `q1` for existing values from `q2`.
- Trailing commas on all the lists are allowed - label filters, function args and with expressions. For instance, the following queries are valid: `m{foo="bar",}`, `f(a, b,)`, `WITH (x=y,) x`. This simplifies maintenance of multi-line queries.
- String literals may be concatenated. This is useful with `WITH` templates: `WITH (commonPrefix="long_metric_prefix_") {__name__=commonPrefix+"suffix1"} / {__name__=commonPrefix+"suffix2"}`.
- Comments starting with `#` and ending with newline. For instance, `up # this is a comment for 'up' metric`.
- Rollup functions - `rollup(m[d])`, `rollup_rate(m[d])`, `rollup_deriv(m[d])`, `rollup_increase(m[d])`, `rollup_delta(m[d])` - return `min`, `max` and `avg`
  values for all the `m` data points over `d` duration.
- `rollup_candlestick(m[d])` - returns `open`, `close`, `low` and `high` values (OHLC) for all the `m` data points over `d` duration. This function is useful for financial applications.
- `union(q1, ... qN)` function for building multiple graphs for `q1`, ... `qN` subqueries with a single query. The `union` function name may be skipped -
  the following queries are equivalent: `union(q1, q2)` and `(q1, q2)`.
- `ru(freeResources, maxResources)` function for returning resource utilization percentage in the range `0% - 100%`. For instance, `ru(node_memory_MemFree_bytes, node_memory_MemTotal_bytes)` returns memory utilization over [node_exporter](https://github.com/prometheus/node_exporter) metrics.
- `ttf(slowlyChangingFreeResources)` function for returning the time in seconds when the given `slowlyChangingFreeResources` expression reaches zero. For instance, `ttf(node_filesystem_avail_byte)` returns the time to storage space exhaustion. This function may be useful for capacity planning.
- Functions for label manipulation:
  - `alias(q, name)` for setting metric name across all the time series `q`.
  - `label_set(q, label1, value1, ... labelN, valueN)` for setting the given values for the given labels on `q`.
  - `label_map(q, label, srcValue1, dstValue1, ... srcValueN, dstValueN)` for mapping `label` values from `src*` to `dst*`.
  - `label_uppercase(q, label1, ... labelN)` for uppercasing values for the given labels.
  - `label_lowercase(q, label2, ... labelN)` for lowercasing value for the given labels.
  - `label_del(q, label1, ... labelN)` for deleting the given labels from `q`.
  - `label_keep(q, label1, ... labelN)` for deleting all the labels except the given labels from `q`.
  - `label_copy(q, src_label1, dst_label1, ... src_labelN, dst_labelN)` for copying label values from `src_*` to `dst_*`.
  - `label_move(q, src_label1, dst_label1, ... src_labelN, dst_labelN)` for moving label values from `src_*` to `dst_*`.
  - `label_transform(q, label, regexp, replacement)` for replacing all the `regexp` occurences with `replacement` in the `label` values from `q`.
  - `label_value(q, label)` - returns numeric values for the given `label` from `q`.
- `label_match(q, label, regexp)` and `label_mismatch(q, label, regexp)` for filtering time series with labels matching (or not matching) the given regexps.
- `sort_by_label(q, label1, ... labelN)` and `sort_by_label_desc(q, label1, ... labelN)` for sorting time series by the given set of labels.
- `step()` function for returning the step in seconds used in the query.
- `start()` and `end()` functions for returning the start and end timestamps of the `[start ... end]` range used in the query.
- `integrate(m[d])` for returning integral over the given duration `d` for the given metric `m`.
- `ideriv(m[d])` - for calculating `instant` derivative for the metric `m` over the duration `d`.
- `increase_pure(m[d])` - for calculating increase of `m` over `d` without edge-case handling compared to `increase(m[d])`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962) for details.
- `deriv_fast(m[d])` - for calculating `fast` derivative for `m` based on the first and the last points from duration `d`.
- `running_` functions - `running_sum`, `running_min`, `running_max`, `running_avg` - for calculating [running values](https://en.wikipedia.org/wiki/Running_total) on the selected time range.
- `range_` functions - `range_sum`, `range_min`, `range_max`, `range_avg`, `range_first`, `range_last`, `range_median`, `range_quantile` - for calculating global value over the selected time range. Note that global value is based on calculated datapoints for the inner query. The calculated datapoints can differ from raw datapoints stored in the database. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness) for details.
- `smooth_exponential(q, sf)` - smooths `q` using [exponential moving average](https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average) with the given smooth factor `sf`.
- `remove_resets(q)` - removes counter resets from `q`.
- `lag(m[d])` - returns lag between the current timestamp and the timestamp from the previous data point in `m` over `d`.
- `lifetime(m[d])` - returns lifetime of `q` over `d` in seconds. It is expected that `d` exceeds the lifetime of `m`.
- `scrape_interval(m[d])` - returns the average interval in seconds between data points of `m` over `d` aka `scrape interval`.
- Trigonometric functions - `sin(q)`, `cos(q)`, `asin(q)`, `acos(q)` and `pi()`.
- `range_over_time(m[d])` - returns value range for `m` over `d` time window, i.e. `max_over_time(m[d])-min_over_time(m[d])`.
- `median_over_time(m[d])` - calculates median values for `m` over `d` time window. Shorthand to `quantile_over_time(0.5, m[d])`.
- `median(q)` - median aggregate. Shorthand to `quantile(0.5, q)`.
- `limitk(k, q) by (group_labels)` - limits the number of time series returned from `q` to `k` per each `group_labels`. The returned set of `k` time series per each `group_labels` can change with each call.
- `any(q) by (x)` - returns any time series from `q` for each group in `x`.
- `keep_last_value(q)` - fills missing data (gaps) in `q` with the previous non-empty value.
- `keep_next_value(q)` - fills missing data (gaps) in `q` with the next non-empty value.
- `interpolate(q)` - fills missing data (gaps) in `q` with linearly interpolated values.
- `distinct_over_time(m[d])` - returns distinct number of values for `m` data points over `d` duration.
- `distinct(q)` - returns a time series with the number of unique values for each timestamp in `q`.
- `sum2_over_time(m[d])` - returns sum of squares for all the `m` values over `d` duration.
- `sum2(q)` - returns a time series with sum of square values for each timestamp in `q`.
- `geomean_over_time(m[d])` - returns [geomean](https://en.wikipedia.org/wiki/Geometric_mean) value for all the `m` value over `d` duration.
- `geomean(q)` - returns a time series with [geomean](https://en.wikipedia.org/wiki/Geometric_mean) value for each timestamp in `q`.
- `rand()`, `rand_normal()` and `rand_exponential()` functions - for generating pseudo-random series with even, normal and exponential distribution.
- `increases_over_time(m[d])` and `decreases_over_time(m[d])` - returns the number of `m` increases or decreases over the given duration `d`.
- `prometheus_buckets(q)` - converts [VictoriaMetrics histogram](https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram) buckets to Prometheus buckets with `le` labels.
- `buckets_limit(k, q)` - limits the number of buckets (Prometheus-style or [VictoriaMetrics-style](https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram))
  per each metric returned by by `q` to `k`. It also converts VictoriaMetrics-style buckets to Prometheus-style buckets, i.e. the end result are buckets with with `le` labels.
- `histogram(q)` - calculates aggregate histogram over `q` time series for each point on the graph. See [this article](https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) for more details.
- `histogram_over_time(m[d])` - calculates [VictoriaMetrics histogram](https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram) for `m` over `d`.
  For example, the following query calculates median temperature by country over the last 24 hours:
  `histogram_quantile(0.5, sum(histogram_over_time(temperature[24h])) by (vmbucket, country))`.
- `histogram_share(le, buckets)` - returns share (in the range 0..1) for `buckets` that fall below `le`. Useful for calculating SLI and SLO.
  For instance, the following query returns the share of requests which are performed under 1.5 seconds during the last 5 minutes: `histogram_share(1.5, sum(rate(request_duration_seconds_bucket[5m])) by (le))`.
- `topk_*` and `bottomk_*` aggregate functions, which return up to K time series. Note that the standard `topk` function may return more than K time series -
   see [this article](https://www.robustperception.io/graph-top-n-time-series-in-grafana) for details.
   - `topk_min(k, q)` - returns top K time series with the max minimums on the given time range
   - `topk_max(k, q)` - returns top K time series with the max maximums on the given time range
   - `topk_avg(k, q)` - returns top K time series with the max averages on the given time range
   - `topk_median(k, q)` - returns top K time series with the max medians on the given time range
   - `bottomk_min(k, q)` - returns bottom K time series with the min minimums on the given time range
   - `bottomk_max(k, q)` - returns bottom K time series with the min maximums on the given time range
   - `bottomk_avg(k, q)` - returns bottom K time series with the min averages on the given time range
   - `bottomk_median(k, q)` - returns bottom K time series with the min medians on the given time range.

  All the `topk_*` and `bottomk_*` functions accept optional third argument - label name for the sum of the remaining time series outside top K or bottom K time series. For example, `topk_max(3, process_resident_memory_bytes, "remaining_sum")` would return up to 3 time series with the maximum value for `process_resident_memory_bytes` plus fourth time series with the sum of the remaining time series if any. The fourth time series will contain `remaining_sum="remaining_sum"` additional label.
- `share_le_over_time(m[d], le)` - returns share (in the range 0..1) of values in `m` over `d`, which are smaller or equal to `le`. Useful for calculating SLI and SLO.
  Example: `share_le_over_time(memory_usage_bytes[24h], 100*1024*1024)` returns the share of time series values for the last 24 hours when memory usage was below or equal to 100MB.
- `share_gt_over_time(m[d], gt)` - returns share (in the range 0..1) of values in `m` over `d`, which are bigger than `gt`. Useful for calculating SLI and SLO.
  Example: `share_gt_over_time(up[24h], 0)` - returns service availability for the last 24 hours.
- `count_le_over_time(m[d], le)` - returns the number of raw samples for `m` over `d`, which don't exceed `le`.
- `count_gt_over_time(m[d], gt)` - returns the number of raw samples for `m` over `d`, which are bigger than `gt`.
- `count_eq_over_time(m[d], N)` - returns the number of raw samples for `m` over `d` with values equal to `N`.
- `count_ne_over_time(m[d], N)` - returns the number of raw samples for `m` over `d` with values not equal to `N`.
- `tmin_over_time(m[d])` - returns timestamp for the minimum value for `m` over `d` time range.
- `tmax_over_time(m[d])` - returns timestamp for the maximum value for `m` over `d` time range.
- `tfirst_over_time(m[d])` - returns timestamp for the first sample for `m` over `d` time range.
- `tlast_over_time(m[d])` - returns timestamp for the last sample for `m` over `d` time range.
- `aggr_over_time(("aggr_func1", "aggr_func2", ...), m[d])` - simultaneously calculates all the listed `aggr_func*` for `m` over `d` time range.
  `aggr_func*` can contain any functions that accept range vector. For instance, `aggr_over_time(("min_over_time", "max_over_time", "rate"), m[d])`
  would calculate `min_over_time`, `max_over_time` and `rate` for `m[d]`.
- `hoeffding_bound_upper(phi, m[d])` and `hoeffding_bound_lower(phi, m[d])` - return upper and lower [Hoeffding bounds](https://en.wikipedia.org/wiki/Hoeffding%27s_inequality)
  for the given `phi` in the range `[0..1]`.
- `last_over_time(m[d])` - returns the last value for `m` on the time range `d`.
- `first_over_time(m[d])` - returns the first value for `m` on the time range `d`.
- `outliersk(N, q) by (group)` - returns up to `N` outlier time series for `q` in every `group`. Outlier time series have the highest deviation from the `median(q)`.
  This aggregate function is useful to detect anomalies across groups of similar time series.
- `ascent_over_time(m[d])` - returns the sum of positive deltas between adjancent data points in `m` over `d`. Useful for tracking height gains in GPS track.
- `descent_over_time(m[d])` - returns the absolute sum of negative deltas between adjancent data points in `m` over `d`. Useful for tracking height loss in GPS track.
- `mode_over_time(m[d])` - returns [mode](https://en.wikipedia.org/wiki/Mode_(statistics)) for `m` values over `d`. It is expected that `m` values are discrete.
- `mode(q) by (x)` - returns [mode](https://en.wikipedia.org/wiki/Mode_(statistics)) for each point in `q` grouped by `x`. It is expected that `q` points are discrete.
- `rate_over_sum(m[d])` - returns rate over the sum of `m` values over `d` duration.
- `zscore_over_time(m[d])` - returns [z-score](https://en.wikipedia.org/wiki/Standard_score) for `m` values over `d` duration. Useful for detecting
  anomalies in time series comparing to historical samples.
- `zscore(q) by (group)` - returns independent [z-score](https://en.wikipedia.org/wiki/Standard_score) values for every point in every `group` of `q`.
  Useful for detecting anomalies in the group of related time series.
