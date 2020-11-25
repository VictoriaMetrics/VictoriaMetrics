# CHANGELOG

# tip

* FEATURE: added [Snap package for single-node VictoriaMetrics](https://snapcraft.io/victoriametrics). This simplifies installation under Ubuntu to a single command:
  ```bash
  snap install victoriametrics
  ```
* FEATURE: vmselect: add `-replicationFactor` command-line flag for reducing query duration when replication is enabled and a part of vmstorage nodes
  are temporarily slow and/or temporarily unavailable. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711
* FEATURE: vminsert: export `vm_rpc_vmstorage_is_reachable` metric, which can be used for monitoring reachability of vmstorage nodes from vminsert nodes.
* FEATURE: vmagent: add [Netflix Eureka](https://github.com/Netflix/eureka) service discovery (aka [eureka_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#eureka_sd_config)). See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/851
* FEATURE: add `filters` option to `dockerswarm_sd_config` like Prometheus did in v2.23.0 - see https://github.com/prometheus/prometheus/pull/8074
* FEATURE: expose `__meta_ec2_ipv6_addresses` label for `ec2_sd_config` like Prometheus will do in the next release.
* FEATURE: add `-loggerWarnsPerSecondLimit` command-line flag for rate limiting of WARN messages in logs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/905
* FEATURE: apply `loggerErrorsPerSecondLimit` and `-loggerWarnsPerSecondLimit` rate limit per caller. I.e. log messages are suppressed if the same caller logs the same message
  at the rate exceeding the given limit. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/905#issuecomment-729395855
* FEATURE: add remoteAddr to slow query log in order to simplify identifying the client that sends slow queries to VictoriaMetrics.
  Slow query logging is controlled with `-search.logSlowQueryDuration` command-line flag.
* FEATURE: add `/tags/delSeries` handler from Graphite Tags API. See https://victoriametrics.github.io/#graphite-tags-api-usage
* FEATURE: log metric name plus all its labels when the metric timestamp is out of the configured retention. This should simplify detecting the source of metrics with unexpected timestamps.
* FEATURE: add `-dryRun` command-line flag to single-node VictoriaMetrics in order to check config file pointed by `-promscrape.config`.

* BUGFIX: properly parse Prometheus metrics with [exemplars](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#exemplars-1) such as `foo 123 # {bar="baz"} 1`.
* BUGFIX: properly parse "infinity" values in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#abnf).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/924


# [v1.47.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.47.0)

* FEATURE: vmselect: return the original error from `vmstorage` node in query response if `-search.denyPartialResponse` is set.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/891
* FEATURE: vmselect: add `"isPartial":{true|false}` field in JSON output for `/api/v1/*` functions
  from [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/). `"isPartial":true` is set if the response contains partial data
  because of a part of `vmstorage` nodes were unavailable during query processing.
* FEATURE: improve performance for `/api/v1/series`, `/api/v1/labels` and `/api/v1/label/<labelName>/values` on time ranges exceeding one day.
* FEATURE: vmagent: reduce memory usage when service discovery detects big number of scrape targets and the set of discovered targets changes over time.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825
* FEATURE: vmagent: add `-promscrape.dropOriginalLabels` command-line option, which can be used for reducing memory usage when scraping big number of targets.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-724308361 for details.
* FEATURE: vmalert: explicitly set extra labels to alert entities. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/870
* FEATURE: add `-search.treatDotsAsIsInRegexps` command-line flag, which can be used for automatic escaping of dots in regexp label filters used in queries.
  For example, if `-search.treatDotsAsIsInRegexps` is set, then the query `foo{bar=~"aaa.bb.cc|dd.eee"}` is automatically converted to `foo{bar=~"aaa\\.bb\\.cc|dd\\.eee"}`.
  This may be useful for querying Graphite data.
* FEATURE: consistently return text-based HTTP responses such as `plain/text` and `application/json` with `charset=utf-8`.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/897
* FEATURE: update Go builder from v1.15.4 to v1.15.5. This should fix [these issues in Go](https://github.com/golang/go/issues?q=milestone%3AGo1.15.5+label%3ACherryPickApproved).
* FEATURE: added `/internal/force_flush` http handler for flushing recently ingested data from in-memory buffers to persistent storage.
  See [troubleshooting docs](https://victoriametrics.github.io/#troubleshooting) for more details.
* FEATURE: added [Graphite Tags API](https://graphite.readthedocs.io/en/stable/tags.html) support.
  See [these docs](https://victoriametrics.github.io/#graphite-tags-api-usage) for details.

* BUGFIX: do not return data points in the end of the selected time range for time series ending in the middle of the selected time range.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/887 and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/845
* BUGFIX: remove spikes at the end of time series gaps for `increase()` or `delta()` functions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/894
* BUGFIX: vminsert: properly return HTTP 503 status code when all the vmstorage nodes are unavailable. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/896


# [v1.46.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.46.0)

* FEATURE: optimize requests to `/api/v1/labels` and `/api/v1/label/<name>/values` when `start` and `end` args are set.
* FEATURE: reduce memory usage when query touches big number of time series.
* FEATURE: vmagent: reduce memory usage when `kubernetes_sd_config` discovers big number of scrape targets (e.g. hundreds of thouthands) and the majority of these targets (99%)
  are dropped during relabeling. Previously labels for all the dropped targets were displayed at `/api/v1/targets` page. Now only up to `-promscrape.maxDroppedTargets` such
  targets are displayed. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/878 for details.
* FEATURE: vmagent: reduce memory usage when scraping big number of targets with big number of temporary labels starting with `__`.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825
* FEATURE: vmagent: add `/ready` HTTP endpoint, which returns 200 OK status code when all the service discovery has been initialized.
  This may be useful during rolling upgrades. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/875

* BUGFIX: vmagent: eliminate data race when `-promscrape.streamParse` command-line is set. Previously this mode could result in scraped metrics with garbage labels.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-723198247 for details.
* BUGFIX: properly calculate `topk_*` and `bottomk_*` functions from [MetricsQL](https://victoriametrics.github.io/MetricsQL.html) for time series with gaps.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/883


# [v1.45.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.45.0)

* FEATURE: allow setting `-retentionPeriod` smaller than one month. I.e. `-retentionPeriod=3d`, `-retentionPeriod=2w`, etc. is supported now.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/173
* FEATURE: optimize more cases according to https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization . Now the following cases are optimized too:
  * `rollup_func(foo{filters}[d]) op bar` -> `rollup_func(foo{filters}[d]) op bar{filters}`
  * `transform_func(foo{filters}) op bar` -> `transform_func(foo{filters}) op bar{filters}`
  * `num_or_scalar op foo{filters} op bar` -> `num_or_scalar op foo{filters} op bar{filters}`
* FEATURE: improve time series search for queries with multiple label filters. I.e. `foo{label1="value", label2=~"regexp"}`.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/781
* FEATURE: vmagent: add `stream parse` mode. This mode allows reducing memory usage when individual scrape targets expose tens of millions of metrics.
  For example, during scraping Prometheus in [federation](https://prometheus.io/docs/prometheus/latest/federation/) mode.
  See `-promscrape.streamParse` command-line option and `stream_parse: true` config option for `scrape_config` section in `-promscrape.config`.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 and [troubleshooting docs for vmagent](https://victoriametrics.github.io/vmagent.html#troubleshooting).
* FEATURE: vmalert: add `-dryRun` command-line option for validating the provided config files without the need to start `vmalert` service.
* FEATURE: accept optional third argument of string type at `topk_*` and `bottomk_*` functions. This is label name for additional time series to return with the sum of time series outside top/bottom K. See [MetricsQL docs](https://victoriametrics.github.io/MetricsQL.html) for more details.
* FEATURE: vmagent: expose `/api/v1/targets` page according to [the corresponding Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/#targets).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/643

* BUGFIX: vmagent: properly handle OpenStack endpoint ending with `v3.0` such as `https://ostack.example.com:5000/v3.0`
  in the same way as Prometheus does. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/728#issuecomment-709914803
* BUGFIX: drop trailing data points for time series with a single raw sample. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/748
* BUGFIX: do not drop trailing data points for instant queries to `/api/v1/query`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/845
* BUGFIX: vmbackup: fix panic when `-origin` isn't specified. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/856
* BUGFIX: vmalert: skip automatically added labels on alerts restore. Label `alertgroup` was introduced in [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/611)
  and automatically added to generated time series. By mistake, this new label wasn't correctly purged on restore event and affected alert's ID uniqueness.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/870
* BUGFIX: vmagent: fix panic at scrape error body formating. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/864
* BUGFIX: vmagent: add leading missing slash to metrics path like Prometheus does. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/835
* BUGFIX: vmagent: drop packet if remote storage returns 4xx status code. This make the behaviour consistent with Prometheus.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/873
* BUGFIX: vmagent: properly handle 301 redirects. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/869


# [v1.44.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.44.0)

* FEATURE: automatically add missing label filters to binary operands as described at https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization .
  This should improve performance for queries with missing label filters in binary operands. For example, the following query should work faster now, because it shouldn't
  fetch and discard time series for `node_filesystem_files_free` metric without matching labels for the left side of the expression:
  ```
     node_filesystem_files{ host="$host", mountpoint="/" } - node_filesystem_files_free
  ```
* FEATURE: vmagent: add Docker Swarm service discovery (aka [dockerswarm_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config)).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/656
* FEATURE: add ability to export data in CSV format. See [these docs](https://victoriametrics.github.io/#how-to-export-csv-data) for details.
* FEATURE: vmagent: add `-promscrape.suppressDuplicateScrapeTargetErrors` command-line flag for suppressing `duplicate scrape target` errors.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651 and https://victoriametrics.github.io/vmagent.html#troubleshooting .
* FEATURE: vmagent: show original labels before relabeling is applied on `duplicate scrape target` errors. This should simplify debugging for incorrect relabeling.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651
* FEATURE: vmagent: `/targets` page now accepts optional `show_original_labels=1` query arg for displaying original labels for each target before relabeling is applied.
  This should simplify debugging for target relabeling configs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651
* FEATURE: add `-finalMergeDelay` command-line flag for configuring the delay before final merge for per-month partitions.
  The final merge is started after no new data is ingested into per-month partition during `-finalMergeDelay`.
* FEATURE: add `vm_rows_added_to_storage_total` metric, which shows the total number of rows added to storage since app start.
  The `sum(rate(vm_rows_added_to_storage_total))` can be smaller than `sum(rate(vm_rows_inserted_total))` if certain metrics are dropped
  due to [relabeling](https://victoriametrics.github.io/#relabeling). The `sum(rate(vm_rows_added_to_storage_total))` can be bigger
  than `sum(rate(vm_rows_inserted_total))` if [replication](https://victoriametrics.github.io/Cluster-VictoriaMetrics.html#replication-and-data-safety) is enabled.
* FEATURE: keep metric name after applying [MetricsQL](https://victoriametrics.github.io/MetricsQL.html) functions, which don't change time series meaning.
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
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/674

* BUGFIX: properly handle stale time series after K8S deployment. Previously such time series could be double-counted.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/748
* BUGFIX: return a single time series at max from `absent()` function like Prometheus does.
* BUGFIX: vmalert: accept days, weeks and years in `for: ` part of config like Prometheus does. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/817
* BUGFIX: fix `mode_over_time(m[d])` calculations. Previously the function could return incorrect results.


# [v1.43.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.43.0)

* FEATURE: reduce CPU usage for repeated queries over sliding time window when no new time series are added to the database.
  Typical use cases: repeated evaluation of alerting rules in [vmalert](https://victoriametrics.github.io/vmalert.html) or dashboard auto-refresh in Grafana.
* FEATURE: vmagent: add OpenStack service discovery aka [openstack_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/728 .
* FEATURE: vmalert: make `-maxIdleConnections` configurable for datasource HTTP client. This option can be used for minimizing connection churn.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/795 .
* FEATURE: add `-influx.maxLineSize` command-line flag for configuring the maximum size for a single Influx line during parsing.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/807

* BUGFIX: properly handle `inf` values during [background merge of LSM parts](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
  Previously `Inf` values could result in `NaN` values for adjancent samples in time series. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/805 .
* BUGFIX: fill gaps on graphs for `range_*` and `running_*` functions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/806 .
* BUGFIX: make a copy of label with new name during relabeling with `action: labelmap` in the same way as Prometheus does.
  Previously the original label name has been replaced. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/812 .
* BUGFIX: support parsing floating-point timestamp like Graphite Carbon does. Such timestmaps are truncated to seconds.


# [v1.42.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0)

* FEATURE: use all the available CPU cores when accepting data via a single TCP connection
  for [all the supported protocols](https://victoriametrics.github.io/#how-to-import-time-series-data).
  Previously data ingested via a single TCP connection could use only a single CPU core. This could limit data ingestion performance.
  The main benefit of this feature is that data can be imported at max speed via a single connection - there is no need to open multiple concurrent
  connections to VictoriaMetrics or [vmagent](https://victoriametrics.github.io/vmagent.html) in order to achieve the maximum data ingestion speed.
* FEATURE: cluster: improve performance for data ingestion path from `vminsert` to `vmstorage` nodes. The maximum data ingestion performance
  for a single connection between `vminsert` and `vmstorage` node scales with the number of available CPU cores on `vmstorage` side.
  This should help with https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791 .
* FEATURE: add ability to export / import data in native format via `/api/v1/export/native` and `/api/v1/import/native`.
  This is the most optimized approach for data migration between VictoriaMetrics instances. Both single-node and cluster instances are supported.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/787#issuecomment-700632551 .
* FEATURE: add `reduce_mem_usage` query option to `/api/v1/export` in order to reduce memory usage during data export / import.
  See [these docs](https://victoriametrics.github.io/#how-to-export-data-in-json-line-format) for details.
* FEATURE: improve performance for `/api/v1/series` handler when it returns big number of time series.
* FEATURE: add `vm_merge_need_free_disk_space` metric, which can be used for estimating the number of deferred background data merges due to the lack of free disk space.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/686 .
* FEATURE: add OpenBSD support. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/785 .

* BUGFIX: properly apply `-search.maxStalenessInterval` command-line flag value. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/784 .
* BUGFIX: fix displaying data in Grafana tables. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/720 .
* BUGFIX: do not adjust the number of detected CPU cores found at `/sys/devices/system/cpu/online`.
  The adjustement was increasing the resulting GOMAXPROC by 1, which looked confusing to users.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-698595309 .
* BUGFIX: vmagent: do not show `-remoteWrite.url` in initial logs if `-remoteWrite.showURL` isn't set. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/773 .
* BUGFIX: properly handle case when [/metrics/find](https://victoriametrics.github.io/#graphite-metrics-api-usage) finds both a leaf and a node for the given `query=prefix.*`.
  In this case only the node must be returned with stripped dot in the end of id as carbonapi does.


# Previous releases

See [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
