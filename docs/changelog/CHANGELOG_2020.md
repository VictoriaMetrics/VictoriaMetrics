---
weight: 6
title: Year 2020
menu:
  docs:
    identifier: vm-changelog-2020
    parent: vm-changelog
    weight: 6
aliases:
- /CHANGELOG_2020.html
- /changelog_2020
---
## [v1.51.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.51.0)

Released at 2020-12-27

* FEATURE: add `/api/v1/status/top_queries` handler, which returns the most frequently executed queries and queries that took the most time for execution. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/907>
* FEATURE: vmagent: add support for `proxy_url` config option in Prometheus scrape configs. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/503>
* FEATURE: remove parts with stale data as soon as they go outside the configured `-retentionPeriod`. Previously such parts may remain active for long periods of time. This should help reducing disk usage for `-retentionPeriod` smaller than one month.
* FEATURE: vmalert: allow setting multiple values for `-notifier.tlsInsecureSkipVerify` command-line flag per each `-notifier.url`.

* BUGFIX: vmalert: properly escape multiline queries when passing them to Grafana. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890>
* BUGFIX: vmagent: set missing `__meta_kubernetes_service_*` labels in `kubernetes_sd_config` for `endpoints` and `endpointslices` roles. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/982>
* BUGFIX: do not adjust `offset` value provided in MetricsQL query. Previously it could be modified in order to improve response cache hit ratio. This is unneeded, since cache hit ratio should remain good because the query time range should be already aligned to multiple of `step` values. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/976>

## [v1.50.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.2)

Released at 2020-12-19

* FEATURE: do not publish duplicate Docker images with `-cluster` tag suffix for [vmagent](https://docs.victoriametrics.com/vmagent/), [vmalert](https://docs.victoriametrics.com/vmalert/), [vmauth](https://docs.victoriametrics.com/vmauth/), [vmbackup](https://docs.victoriametrics.com/vmbackup/) and [vmrestore](https://docs.victoriametrics.com/vmrestore/), since they are identical to images without `-cluster` tag suffix.

* BUGFIX: vmalert: properly populate template variables. This has been broken in v1.50.0. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/974>
* BUGFIX: properly parse negative combined duration in MetricsQL such as `-1h3m4s`. It must be parsed as `-(1h + 3m + 4s)`. Previously it was parsed as `-1h + 3m + 4s`.
* BUGFIX: properly parse lines in [Prometheus exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md) and in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md) with whitespace after the timestamp. For example, `foo 123 456 ## some comment here`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/970>

## [v1.50.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.1)

Released at 2020-12-15

* FEATURE: vmagent: export `vmagent_remotewrite_blocks_sent_total` and `vmagent_remotewrite_blocks_sent_total` metrics for each `-remoteWrite.url`.

* BUGFIX: vmagent: properly delete unregistered scrape targets from `/targets` and `/api/v1/targets` pages. They weren't deleted due to the bug in `v1.50.0`.

## [v1.50.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.0)

Released at 2020-12-15

* FEATURE: automatically reset response cache when samples with timestamps older than `now - search.cacheTimestampOffset` are ingested to VictoriaMetrics. This makes unnecessary disabling response cache during data backfilling or resetting it after backfilling is complete as described [in these docs](https://docs.victoriametrics.com/#backfilling). This feature applies only to single-node VictoriaMetrics. It doesn't apply to cluster version of VictoriaMetrics because `vminsert` nodes don't know about `vmselect` nodes where the response cache must be reset.
* FEATURE: vmalert: add `query`, `first` and `value` functions to alert templates. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/539>
* FEATURE: vmagent: return user-friendly HTML page when requesting `/targets` page from web browser. The page is returned in the old plaintext format when requesting via curl or similar tool.
* FEATURE: allow multiple whitespace chars between measurements, fields and timestamp when parsing InfluxDB line protocol.
  Though [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.8/write_protocols/line_protocol_tutorial/) denies multiple whitespace chars between these entities,
  some apps improperly put multiple whitespace chars. This workaround allows accepting data from such apps.
* FEATURE: export `vm_promscrape_active_scrapers{type="<sd_type>"}` metric for tracking the number of active scrapers per each service discovery type.
* FEATURE: export `vm_promscrape_scrapers_started_total{type="<sd_type>"}` and `vm_promscrape_scrapers_stopped_total{type="<sd_type>"}` metrics for tracking churn rate for scrapers
  per each service discovery type.
* FEATURE: vmagent: allow setting per-`-remoteWrite.url` command-line flags for `-remoteWrite.sendTimeout` and `-remoteWrite.tlsInsecureSkipVerify`.

* BUGFIX: properly handle `*` and `[...]` inside curly braces in query passed to Graphite Metrics API. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/952>
* BUGFIX: vmagent: fix memory leak when big number of targets is discovered via service discovery.
* BUGFIX: vmagent: properly pass `datacenter` filter to Consul API server. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/574#issuecomment-740454170>
* BUGFIX: properly handle CPU limits set on the host system or host container. The bugfix may result in lower memory usage on systems with CPU limits. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/946>
* BUGFIX: prevent from duplicate `name` tag returned from `/tags/autoComplete/tags` handler. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/942>
* BUGFIX: do not enable strict parsing for `-promscrape.config` if `-promscrape.config.dryRun` comand-line flag is set. Strict parsing can be enabled with `-promscrape.config.strictParse` command-line flag. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/944>
* BUGFIX: vminsert: properly update `vm_rpc_rerouted_rows_processed_total` metric. Previously it wasn't updated. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/955>
* BUGFIX: vmagent: properly recover when opening incorrectly stored persistent queue. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/964>
* BUGFIX: vmagent: properly handle scrape errors when stream parsing is enabled with `-promscrape.streamParse` command-line flag or with `stream_parse: true` per-target config option. Previously such errors weren't reported at `/targets` page. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/967>
* BUGFIX: assume the previous value is 0 when calculating `increase()` for the first point on the graph if its value doesn't exceed 100 and the delta between two first points equals to 0. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962>

## [v1.49.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.49.0)

Released at 2020-12-05

* FEATURE: optimize Consul service discovery speed when discovering big number of services. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/574>
* FEATURE: add `label_uppercase(q, label1, ... labelN)` and `label_lowercase(q, label1, ... labelN)` function to [MetricsQL](https://docs.victoriametrics.com/metricsql/)
  for uppercasing and lowercasing values for the given labels. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/936>
* FEATURE: add `count_eq_over_time(m[d], N)` and `count_ne_over_time(m[d], N)` for counting the number of samples for `m` over `d` that (equal / not equal) to `N`.
* FEATURE: do not print usage info for all the command-line flags when incorrect command-line flag is passed. Previously it could be hard reading the error message
  about incorrect command-line flag because of too big usage info for all the flags.
* FEATURE: upgrade Go builder from v1.15.5 to v1.15.6 . This fixes [issues found in Go since v1.15.5](https://github.com/golang/go/issues?q=milestone%3AGo1.15.6+label%3ACherryPickApproved).

* BUGFIX: properly parse timestamps in OpenMetrics format - they are exposed as floating-point number in seconds instead of integer milliseconds
  unlike in Prometheus exposition format. See [the docs](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md#timestamps).
* BUGFIX: return `nan` for `a >bool b` query when `a` equals to `nan` like Prometheus does. Previously `0` was returned in this case. This applies to any comparison operation
  with `bool` modifier. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/operators/#comparison-binary-operators) for details.
* BUGFIX: properly parse hex numbers in MetricsQL. Previously hex numbers with non-decimal digits such as `0x3b` couldn't be parsed.
* BUGFIX: handle `time() cmp_op metric` like Prometheus does - i.e. return `metric` value if `cmp_op` comparison is true. Previously `time()` value was returned.
* BUGFIX: return `nan` for `minute(m)` query when `m` equals to `nan` like Prometheus does. This applies to all the time-related functions such as `day_of_month`, `day_of_week`,
  `days_in_month`, `hour`, `month` and `year`.

## [v1.48.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.48.0)

Released at 2020-11-26

* FEATURE: added [Snap package for single-node VictoriaMetrics](https://snapcraft.io/victoriametrics). This simplifies installation under Ubuntu to a single command:

  ```sh
  snap install victoriametrics
  ```

* FEATURE: vmselect: add `-replicationFactor` command-line flag for reducing query duration when replication is enabled and a part of vmstorage nodes
  are temporarily slow and/or temporarily unavailable. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711>
* FEATURE: vminsert: export `vm_rpc_vmstorage_is_reachable` metric, which can be used for monitoring reachability of vmstorage nodes from vminsert nodes.
* FEATURE: vmagent: add [Netflix Eureka](https://github.com/Netflix/eureka) service discovery (aka [eureka_sd_config](https://docs.victoriametrics.com/sd_configs/#eureka_sd_configs)). See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/851>
* FEATURE: add `filters` option to `dockerswarm_sd_config` like Prometheus did in v2.23.0 - see <https://github.com/prometheus/prometheus/pull/8074>
* FEATURE: expose `__meta_ec2_ipv6_addresses` label for `ec2_sd_config` like Prometheus will do in the next release.
* FEATURE: add `-loggerWarnsPerSecondLimit` command-line flag for rate limiting of WARN messages in logs. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/905>
* FEATURE: apply `loggerErrorsPerSecondLimit` and `-loggerWarnsPerSecondLimit` rate limit per caller. I.e. log messages are suppressed if the same caller logs the same message
  at the rate exceeding the given limit. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/905#issuecomment-729395855>
* FEATURE: add remoteAddr to slow query log in order to simplify identifying the client that sends slow queries to VictoriaMetrics.
  Slow query logging is controlled with `-search.logSlowQueryDuration` command-line flag.
* FEATURE: add `/tags/delSeries` handler from Graphite Tags API. See <https://docs.victoriametrics.com/#graphite-tags-api-usage>
* FEATURE: log metric name plus all its labels when the metric timestamp is out of the configured retention. This should simplify detecting the source of metrics with unexpected timestamps.
* FEATURE: add `-dryRun` command-line flag to single-node VictoriaMetrics in order to check config file pointed by `-promscrape.config`.

* BUGFIX: properly parse Prometheus metrics with [exemplars](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#exemplars-1) such as `foo 123 ## {bar="baz"} 1`.
* BUGFIX: properly parse "infinity" values in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#abnf).
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/924>

## [v1.47.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.47.0)

Released at 2020-11-16

* FEATURE: vmselect: return the original error from `vmstorage` node in query response if `-search.denyPartialResponse` is set.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/891>
* FEATURE: vmselect: add `"isPartial":{true|false}` field in JSON output for `/api/v1/*` functions
  from [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/). `"isPartial":true` is set if the response contains partial data
  because of a part of `vmstorage` nodes were unavailable during query processing.
* FEATURE: improve performance for `/api/v1/series`, `/api/v1/labels` and `/api/v1/label/<labelName>/values` on time ranges exceeding one day.
* FEATURE: vmagent: reduce memory usage when service discovery detects big number of scrape targets and the set of discovered targets changes over time.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825>
* FEATURE: vmagent: add `-promscrape.dropOriginalLabels` command-line option, which can be used for reducing memory usage when scraping big number of targets.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-724308361> for details.
* FEATURE: vmalert: explicitly set extra labels to alert entities. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/870>
* FEATURE: add `-search.treatDotsAsIsInRegexps` command-line flag, which can be used for automatic escaping of dots in regexp label filters used in queries.
  For example, if `-search.treatDotsAsIsInRegexps` is set, then the query `foo{bar=~"aaa.bb.cc|dd.eee"}` is automatically converted to `foo{bar=~"aaa\\.bb\\.cc|dd\\.eee"}`.
  This may be useful for querying Graphite data.
* FEATURE: consistently return text-based HTTP responses such as `plain/text` and `application/json` with `charset=utf-8`.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/897>
* FEATURE: update Go builder from v1.15.4 to v1.15.5. This should fix [these issues in Go](https://github.com/golang/go/issues?q=milestone%3AGo1.15.5+label%3ACherryPickApproved).
* FEATURE: added `/internal/force_flush` http handler for flushing recently ingested data from in-memory buffers to persistent storage.
  See [troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting) for more details.
* FEATURE: added [Graphite Tags API](https://graphite.readthedocs.io/en/stable/tags.html) support.
  See [these docs](https://docs.victoriametrics.com/#graphite-tags-api-usage) for details.

* BUGFIX: do not return data points in the end of the selected time range for time series ending in the middle of the selected time range.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/887> and <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/845>
* BUGFIX: remove spikes at the end of time series gaps for `increase()` or `delta()` functions. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/894>
* BUGFIX: vminsert: properly return HTTP 503 status code when all the vmstorage nodes are unavailable. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/896>

## [v1.46.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.46.0)

Released at 2020-11-07

* FEATURE: optimize requests to `/api/v1/labels` and `/api/v1/label/<name>/values` when `start` and `end` args are set.
* FEATURE: reduce memory usage when query touches big number of time series.
* FEATURE: vmagent: reduce memory usage when `kubernetes_sd_config` discovers big number of scrape targets (e.g. hundreds of thousands) and the majority of these targets (99%)
  are dropped during relabeling. Previously labels for all the dropped targets were displayed at `/api/v1/targets` page. Now only up to `-promscrape.maxDroppedTargets` such
  targets are displayed. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/878> for details.
* FEATURE: vmagent: reduce memory usage when scraping big number of targets with big number of temporary labels starting with `__`.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825>
* FEATURE: vmagent: add `/ready` HTTP endpoint, which returns 200 OK status code when all the service discovery has been initialized.
  This may be useful during rolling upgrades. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/875>

* BUGFIX: vmagent: eliminate data race when `-promscrape.streamParse` command-line is set. Previously this mode could result in scraped metrics with garbage labels.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-723198247> for details.
* BUGFIX: properly calculate `topk_*` and `bottomk_*` functions from [MetricsQL](https://docs.victoriametrics.com/metricsql/) for time series with gaps.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/883>

## [v1.45.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.45.0)

Released at 2020-11-02

* FEATURE: allow setting `-retentionPeriod` smaller than one month. I.e. `-retentionPeriod=3d`, `-retentionPeriod=2w`, etc. is supported now.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/173>
* FEATURE: optimize more cases according to <https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization> . Now the following cases are optimized too:
  * `rollup_func(foo{filters}[d]) op bar` -> `rollup_func(foo{filters}[d]) op bar{filters}`
  * `transform_func(foo{filters}) op bar` -> `transform_func(foo{filters}) op bar{filters}`
  * `num_or_scalar op foo{filters} op bar` -> `num_or_scalar op foo{filters} op bar{filters}`
* FEATURE: improve time series search for queries with multiple label filters. I.e. `foo{label1="value", label2=~"regexp"}`.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/781>
* FEATURE: vmagent: add `stream parse` mode. This mode allows reducing memory usage when individual scrape targets expose tens of millions of metrics.
  For example, during scraping Prometheus in [federation](https://prometheus.io/docs/prometheus/latest/federation/) mode.
  See `-promscrape.streamParse` command-line option and `stream_parse: true` config option for `scrape_config` section in `-promscrape.config`.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825> and [troubleshooting docs for vmagent](https://docs.victoriametrics.com/vmagent/#troubleshooting).
* FEATURE: vmalert: add `-dryRun` command-line option for validating the provided config files without the need to start `vmalert` service.
* FEATURE: accept optional third argument of string type at `topk_*` and `bottomk_*` functions. This is label name for additional time series to return with the sum of time series outside top/bottom K. See [MetricsQL docs](https://docs.victoriametrics.com/metricsql/) for more details.
* FEATURE: vmagent: expose `/api/v1/targets` page according to [the corresponding Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/#targets).
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/643>

* BUGFIX: vmagent: properly handle OpenStack endpoint ending with `v3.0` such as `https://ostack.example.com:5000/v3.0`
  in the same way as Prometheus does. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/728#issuecomment-709914803>
* BUGFIX: drop trailing data points for time series with a single raw sample. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/748>
* BUGFIX: do not drop trailing data points for instant queries to `/api/v1/query`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/845>
* BUGFIX: vmbackup: fix panic when `-origin` isn't specified. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/856>
* BUGFIX: vmalert: skip automatically added labels on alerts restore. Label `alertgroup` was introduced in [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/611)
  and automatically added to generated time series. By mistake, this new label wasn't correctly purged on restore event and affected alert's ID uniqueness.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/870>
* BUGFIX: vmagent: fix panic at scrape error body formating. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/864>
* BUGFIX: vmagent: add leading missing slash to metrics path like Prometheus does. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/835>
* BUGFIX: vmagent: drop packet if remote storage returns 4xx status code. This make the behaviour consistent with Prometheus.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/873>
* BUGFIX: vmagent: properly handle 301 redirects. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/869>

## [v1.44.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.44.0)

Released at 2020-10-13

* FEATURE: automatically add missing label filters to binary operands as described at <https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization> .
  This should improve performance for queries with missing label filters in binary operands. For example, the following query should work faster now, because it shouldn't
  fetch and discard time series for `node_filesystem_files_free` metric without matching labels for the left side of the expression:

  ```
     node_filesystem_files{ host="$host", mountpoint="/" } - node_filesystem_files_free
  ```

* FEATURE: vmagent: add Docker Swarm service discovery (aka [dockerswarm_sd_config](https://docs.victoriametrics.com/sd_configs/#dockerswarm_sd_configs)).
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/656>
* FEATURE: add ability to export data in CSV format. See [these docs](https://docs.victoriametrics.com/#how-to-export-csv-data) for details.
* FEATURE: vmagent: add `-promscrape.suppressDuplicateScrapeTargetErrors` command-line flag for suppressing `duplicate scrape target` errors.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651> and <https://docs.victoriametrics.com/vmagent/#troubleshooting> .
* FEATURE: vmagent: show original labels before relabeling is applied on `duplicate scrape target` errors. This should simplify debugging for incorrect relabeling.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651>
* FEATURE: vmagent: `/targets` page now accepts optional `show_original_labels=1` query arg for displaying original labels for each target before relabeling is applied.
  This should simplify debugging for target relabeling configs. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651>
* FEATURE: add `-finalMergeDelay` command-line flag for configuring the delay before final merge for per-month partitions.
  The final merge is started after no new data is ingested into per-month partition during `-finalMergeDelay`.
* FEATURE: add `vm_rows_added_to_storage_total` metric, which shows the total number of rows added to storage since app start.
  The `sum(rate(vm_rows_added_to_storage_total))` can be smaller than `sum(rate(vm_rows_inserted_total))` if certain metrics are dropped
  due to [relabeling](https://docs.victoriametrics.com/#relabeling). The `sum(rate(vm_rows_added_to_storage_total))` can be bigger
  than `sum(rate(vm_rows_inserted_total))` if [replication](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety) is enabled.
* FEATURE: keep metric name after applying [MetricsQL](https://docs.victoriametrics.com/metricsql/) functions, which don't change time series meaning.
  The list of such functions:
  * `keep_last_value`
  * `keep_next_value`
  * `interpolate`
  * `running_min`
  * `running_max`
  * `running_avg`
  * `range_min`
  * `range_max`
  * `range_avg`
  * `range_first`
  * `range_last`
  * `range_quantile`
  * `smooth_exponential`
  * `ceil`
  * `floor`
  * `round`
  * `clamp_min`
  * `clamp_max`
  * `max_over_time`
  * `min_over_time`
  * `avg_over_time`
  * `quantile_over_time`
  * `mode_over_time`
  * `geomean_over_time`
  * `holt_winters`
  * `predict_linear`
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/674>

* BUGFIX: properly handle stale time series after K8S deployment. Previously such time series could be double-counted.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/748>
* BUGFIX: return a single time series at max from `absent()` function like Prometheus does.
* BUGFIX: vmalert: accept days, weeks and years in `for:` part of config like Prometheus does. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/817>
* BUGFIX: fix `mode_over_time(m[d])` calculations. Previously the function could return incorrect results.

## [v1.43.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.43.0)

Released at 2020-10-06

* FEATURE: reduce CPU usage for repeated queries over sliding time window when no new time series are added to the database.
  Typical use cases: repeated evaluation of alerting rules in [vmalert](https://docs.victoriametrics.com/vmalert/) or dashboard auto-refresh in Grafana.
* FEATURE: vmagent: add OpenStack service discovery aka [openstack_sd_config](https://docs.victoriametrics.com/sd_configs/#openstack_sd_configs).
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/728> .
* FEATURE: vmalert: make `-maxIdleConnections` configurable for datasource HTTP client. This option can be used for minimizing connection churn.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/795> .
* FEATURE: add `-influx.maxLineSize` command-line flag for configuring the maximum size for a single InfluxDB line during parsing.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/807>

* BUGFIX: properly handle `inf` values during [background merge of LSM parts](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
  Previously `Inf` values could result in `NaN` values for adjacent samples in time series. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/805> .
* BUGFIX: fill gaps on graphs for `range_*` and `running_*` functions. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/806> .
* BUGFIX: make a copy of label with new name during relabeling with `action: labelmap` in the same way as Prometheus does.
  Previously the original label name has been replaced. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/812> .
* BUGFIX: support parsing floating-point timestamp like Graphite Carbon does. Such timestmaps are truncated to seconds.

## [v1.42.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0)

Released at 2020-09-30

* FEATURE: use all the available CPU cores when accepting data via a single TCP connection
  for [all the supported protocols](https://docs.victoriametrics.com/#how-to-import-time-series-data).
  Previously data ingested via a single TCP connection could use only a single CPU core. This could limit data ingestion performance.
  The main benefit of this feature is that data can be imported at max speed via a single connection - there is no need to open multiple concurrent
  connections to VictoriaMetrics or [vmagent](https://docs.victoriametrics.com/vmagent/) in order to achieve the maximum data ingestion speed.
* FEATURE: cluster: improve performance for data ingestion path from `vminsert` to `vmstorage` nodes. The maximum data ingestion performance
  for a single connection between `vminsert` and `vmstorage` node scales with the number of available CPU cores on `vmstorage` side.
  This should help with <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791> .
* FEATURE: add ability to export / import data in native format via `/api/v1/export/native` and `/api/v1/import/native`.
  This is the most optimized approach for data migration between VictoriaMetrics instances. Both single-node and cluster instances are supported.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/787#issuecomment-700632551> .
* FEATURE: add `reduce_mem_usage` query option to `/api/v1/export` in order to reduce memory usage during data export / import.
  See [these docs](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format) for details.
* FEATURE: improve performance for `/api/v1/series` handler when it returns big number of time series.
* FEATURE: add `vm_merge_need_free_disk_space` metric, which can be used for estimating the number of deferred background data merges due to the lack of free disk space.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/686> .
* FEATURE: add OpenBSD support. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/785> .

* BUGFIX: properly apply `-search.maxStalenessInterval` command-line flag value. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/784> .
* BUGFIX: fix displaying data in Grafana tables. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/720> .
* BUGFIX: do not adjust the number of detected CPU cores found at `/sys/devices/system/cpu/online`.
  The adjustment was increasing the resulting GOMAXPROCS by 1, which looked confusing to users.
  See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-698595309> .
* BUGFIX: vmagent: do not show `-remoteWrite.url` in initial logs if `-remoteWrite.showURL` isn't set. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/773> .
* BUGFIX: properly handle case when [/metrics/find](https://docs.victoriametrics.com/#graphite-metrics-api-usage) finds both a leaf and a node for the given `query=prefix.*`.
  In this case only the node must be returned with stripped dot in the end of id as carbonapi does.

## Previous releases

See [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
