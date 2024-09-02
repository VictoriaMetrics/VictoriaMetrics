---
weight: 5
title: Year 2021
menu:
  docs:
    identifier: vm-changelog-2021
    parent: vm-changelog
    weight: 5
aliases:
- /CHANGELOG_2021.html
- /changelog_2021
---
## [v1.71.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.71.0)

Released at 2021-12-20

**Update notes:** deduplication logic was slightly changed on the release, which may cause extra
[background merges](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for already existing data parts for installations with `-dedup.minScrapeInterval` flag value greater than 0. This process is intentionally limited by one CPU core, but still can result
into increase of CPU usage until merges are finished.

We recommend updating in "off-peak" time when load on the VictoriaMetrics is on its minimum.

* FEATURE: [VictoriaMetrics enterprise](https://docs.victoriametrics.com/enterprise/): add multi-level downsampling support. See [these docs](https://docs.victoriametrics.com/#downsampling) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/36).
* FEATURE: [vmui](https://docs.victoriametrics.com/#vmui): add ability to analyze the correlation between two queries on a single graph. Just click `+Query` button, enter the second query in the newly appeared input field and press `Ctrl+Enter`. Results for both queries should be displayed simultaneously on the same graph. Every query has its own vertical scale, which is displayed on the left and the right side of the graph. Lines for the second query are dashed. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1916).
* FEATURE: [vmui](https://docs.victoriametrics.com/#vmui): add ability to override the interval between returned datapoints. By default it is automatically calculated depending on the selected time range and horizontal resolution of the graph. Now it is possible to override it with custom values. This may be useful during data exploration and debugging.
* FEATURE: accept optional `extra_filters[]=series_selector` query args at Prometheus query APIs additionally to `extra_label` query args. This allows enforcing additional filters for all the Prometheus query APIs by using [vmgateway](https://docs.victoriametrics.com/vmgateway/) or [vmauth](https://docs.victoriametrics.com/vmauth/). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1863).
* FEATURE: [vmauth](https://docs.victoriametrics.com/vmauth/): allow specifying `http` and `https` urls in `-auth.config` command-line flag. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1898). Thanks for @TFM93 .
* FEATURE: [vmagent](https://docs.victoriametrics.com/vmagent/): allow specifying `http` and `https` urls in the following command-line flags: `-promscrape.config`, `-remoteWrite.relabelConfig` and `-remoteWrite.urlRelabelConfig`.
* FEATURE: vminsert: allow specifying `http` and `https` urls in `-relabelConfig` command-line flag.
* FEATURE: vminsert: add `-maxLabelValueLen` command-line flag for the ability to configure the maximum length of label value. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1908).
* FEATURE: preserve the order of time series passed to [limit_offset](https://docs.victoriametrics.com/metricsql/#limit_offset) function. This allows implementing series paging via `limit_offset(limit, offset, sort_by_label(...))`. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1920) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/951) issues.
* FEATURE: automatically convert `(value1|...|valueN)` into `{value1,...,valueN}` inside `__graphite__` pseudo-label. This allows using [Grafana multi-value template variables](https://grafana.com/docs/grafana/latest/variables/formatting-multi-value-variables/) inside `__graphite__` pseudo-label. For example, `{__graphite__=~"foo.($bar)"}` is expanded to `{__graphite__=~"foo.{x,y}"}` if both `x` and `y` are selected for `$bar` template variable. See [these docs](https://docs.victoriametrics.com/#selecting-graphite-metrics) for details.
* FEATURE: add [timestamp_with_name](https://docs.victoriametrics.com/metricsql/#timestamp_with_name) function. It works the same as [timestamp](https://docs.victoriametrics.com/metricsql/#timestamp), but leaves the original time series names, so it can be used in queries, which match multiple time series names: `timestamp_with_name({foo="bar"}[1h])`. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/949#issuecomment-995222388) for more context.
* FEATURE: add [changes_prometheus](https://docs.victoriametrics.com/metricsql/#changes_prometheus), [increase_prometheus](https://docs.victoriametrics.com/metricsql/#increase_prometheus) and [delta_prometheus](https://docs.victoriametrics.com/metricsql/#delta_prometheus) functions, which don't take into account the previous sample before the given lookbehind window specified in square brackets. These functions may be used when the Prometheus behaviour for `changes()`, `increase()` and `delta()` functions is needed to be preserved. VictoriaMetrics uses slightly different behaviour for `changes()`, `increase()` and `delta()` functions by default - see [this article](https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e) for details. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1962).

* BUGFIX: fix `unaligned 64-bit atomic operation` panic on 32-bit architectures, which has been introduced in v1.70.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1944).
* BUGFIX: [vmalert](https://docs.victoriametrics.com/vmalert/): restore the ability to use `$labels.alertname` in labels templating. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1921).
* BUGFIX: [vmui](https://docs.victoriametrics.com/#vmui): add missing `query` caption to the input field for the query. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1900).
* BUGFIX: [vmui](https://docs.victoriametrics.com/#vmui): fix navigation over query history with `Ctrl+up/down` and fix zoom relatively to the cursor position. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1936).
* BUGFIX: deduplicate samples more thoroughly if [deduplication](https://docs.victoriametrics.com/#deduplication) is enabled. Previously some duplicate samples may be left on disk for time series with high churn rate. This may result in bigger storage space requirements.
* BUGFIX: [vmagent](https://docs.victoriametrics.com/vmagent/): follow up to 5 redirects when `follow_redirects: true` is set for a particular scrape config. Previously only a single redirect was performed in this case. It is expected these redirects are performed to the original hostname. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1945).
* BUGFIX: de-duplicate data exported via [/api/v1/export/csv](https://docs.victoriametrics.com/#how-to-export-csv-data) by default if [deduplication](https://docs.victoriametrics.com/#deduplication) is enabled. The de-duplication can be disabled by passing `reduce_mem_usage=1` query arg to `/api/v1/export/csv`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1837).
* BUGFIX: [vmalert](https://docs.victoriametrics.com/vmalert/): properly store [historical data](https://docs.victoriametrics.com/vmalert/#rules-backfilling) to old Prometheus versions. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943).

## [v1.70.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.70.0)

Released at 2021-12-02

* FEATURE: [vmalert](https://docs.victoriametrics.com/vmalert/): add ability to pass arbitrary query args to `-datasource.url` on a per-group basis via `params` option. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1892).
* FEATURE: add `now()` function to MetricsQL. This function returns the current timestamp in seconds. See [these docs](https://docs.victoriametrics.com/metricsql/#now).
* FEATURE: vmauth: allow using optional `name` field in configs. This field is then used as `username` label value for `vmauth_user_requests_total` metric. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1805).
* FEATURE: vmagent: export `vm_persistentqueue_read_duration_seconds_total` and `vm_persistentqueue_write_duration_seconds_total` metrics, which can be used for detecting persistent queue saturation with `rate(vm_persistentqueue_write_duration_seconds_total) > 0.9` alerting rule.
* FEATURE: export `vm_filestream_read_duration_seconds_total` and `vm_filestream_write_duration_seconds_total` metrics, which can be used for detecting persistent disk saturation with `rate(vm_filestream_read_duration_seconds_total) > 0.9` alerting rule.
* FEATURE: export `vm_cache_size_max_bytes` metrics, which show capacity for various caches. These metrics can be used for determining caches with reach its capacity with `vm_cache_size_bytes / vm_cache_size_max_bytes > 0.9` query.
* FEATURE: [vmbackup](https://docs.victoriametrics.com/vmbackup/), [vmrestore](https://docs.victoriametrics.com/vmrestore/): add `-s3ForcePathStyle` command-line flag, which can be used for making backups to [Aliyun OSS](https://www.aliyun.com/product/oss). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1802).
* FEATURE: [vmctl](https://docs.victoriametrics.com/vmctl/): improve data migration from OpenTSDB. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1809). Thanks to @johnseekins .
* FEATURE: suppress `connection reset by peer` errors when remote client resets TCP connection to VictoriaMetrics / vmagent while ingesting the data via InfluxDB line protocol, Graphite protocol or OpenTSDB protocol. This error is expected, so there is no need in logging it.
* FEATURE: [vmui](https://docs.victoriametrics.com/#vmui): store the display type in URL, so it isn't lost when copy-pasting the URL. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1804).
* FEATURE: vmalert: make `-notifier.url` command-line flag optional. This flag can be omitted if `vmalert` is used solely for recording rules and doesn't evaluate alerting rules. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1870).
* FEATURE: [vmbackup](https://docs.victoriametrics.com/vmbackup/), [vmrestore](https://docs.victoriametrics.com/vmrestore/): export internal metrics at `http://vmbackup:8420/metrics` and `http://vmrestore:8421/metrics` for better visibility of the backup/restore process.
* FEATURE: allow trailing whitespace after the timestamp when [parsing Graphite plaintext lines](https://docs.victoriametrics.com/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1865).
* FEATURE: expose `/-/healthy` and `/-/ready` endpoints as Prometheus does. This is needed for improving integration with third-party solutions, which rely on these endpoints. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1833).

* BUGFIX: vmagent: prevent from scraping duplicate targets if `-promscrape.dropOriginalLabels` command-line flag is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1830). Thanks to @guidao for the fix.
* BUGFIX: vmstorage [enterprise](https://docs.victoriametrics.com/enterprise/): added missing `vm_tenant_used_tenant_bytes` metric, which shows the approximate per-tenant disk usage. See [these docs](https://docs.victoriametrics.com/pertenantstatistic/) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1605).
* BUGFIX: vmauth: properly take into account the value passed to `-maxIdleConnsPerBackend` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1300).
* BUGFIX: vmagent: fix [reading data from Kafka](https://docs.victoriametrics.com/vmagent/#reading-metrics-from-kafka).
* BUGFIX: vmalert: fix [replay mode](https://docs.victoriametrics.com/vmalert/#rules-backfilling) in enterprise version.
* BUGFIX: consistently return zero from [deriv()](https://docs.victoriametrics.com/metricsql/#deriv) function applied to a constant time series. Previously it could return small non-zero values in this case.
* BUGFIX: [vmrestore](https://docs.victoriametrics.com/vmrestore/): properly resume downloading for partially downloaded big files. Previously such files were re-downloaded from the beginning after the interrupt. Now only the remaining parts of the file are downloaded. This allows saving network bandwidth. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/487).
* BUGFIX: [vmui](https://docs.victoriametrics.com/#vmui): do not store the last query across vmui page reloads. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1694).
* BUGFIX: [vmui](https://docs.victoriametrics.com/#vmui): fix `Cannot read properties of undefined` error at table view. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1797).

## [v1.69.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.69.0)

Released at 2021-11-08

* FEATURE: vmalert: allow groups with empty rules list like Prometheus does. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1742).
* FEATURE: vmalert: allow groups with default `tenant` in `-clusterMode`. Default `tenant` values can be specified via `-defaultTenant.prometheus` and `-defaultTenant.graphite`. See [these docs](https://docs.victoriametrics.com/vmalert/#multitenancy).
* FEATURE: vmagent: add `collapse` and `expand` buttons per each group of targets with the same `job_name` at `http://vmagent:8429/targets` page.
* FEATURE: automatically detect timestamp precision (ns, us, ms or s) for the data ingested into VictoriaMetrics via [InfluxDB line protocol](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf).
* FEATURE: vmagent: add ability to protect `/config` page with auth key via `-configAuthKey` command-line flag. This page may contain sensitive config information, so it may be good to restrict access to this page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1764).
* FEATURE: vmagent: hide passwords and auth tokens at `/config` page like Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1764).
* FEATURE: vmagent: add `-promscrape.maxResponseHeadersSize` command-line flag for tuning the maximum HTTP response headers size for Prometheus scrape targets.
* FEATURE: vmagent: send data to multiple configured remote storage systems in parallel (e.g. when multiple `-remoteWrite.url` flag values are specified). This should improve data ingestion speed.
* FEATURE: vmagent: add `-remoteWrite.maxRowsPerBlock` command-line flag for tuning the number of samples to send to remote storage per each block. Bigger values may improve data ingestion performance at the cost of higher memory usage.
* FEATURE: vmagent: distribute Kafka messages among all the partitions when [writing data to Kafka](https://docs.victoriametrics.com/vmagent/#writing-metrics-to-kafka).
* FEATURE: add [label_graphite_group](https://docs.victoriametrics.com/metricsql/#label_graphite_group) function for extracting the given groups from Graphite metric names.
* FEATURE: add [duration_over_time](https://docs.victoriametrics.com/metricsql/#duration_over_time) function for calculating the actual lifetime of the time series with possible gaps. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1780).
* FEATURE: add [limit_offset](https://docs.victoriametrics.com/metricsql/#limit_offset) function, which can be used for implementing simple paging over big number of time series. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1778).

* BUGFIX: vmagent: reduce the increased memory usage when scraping targets with big number of metrics which periodically change. The memory usage has been increased in v1.68.0 after vmagent started generating staleness markers in [stream parse mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1745).
* BUGFIX: vmagent: properly display `proxy_url` config option at `http://vmagent:8429/config` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1755).
* BUGFIX: fix tests for Apple M1. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1653).

## [v1.68.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.68.0)

Released at 2021-10-22

* FEATURE: vmagent: expose `-promscrape.config` contents at `/config` page as Prometheus does. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1695).
* FEATURE: vmagent: add `show original labels` button per each scrape target displayed at `http://vmagent:8429/targets` page. This should improve debuggability for service discovery and relabeling issues similar to [this one](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1664). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1698).
* FEATURE: vmagent: shard targets among cluster nodes after the relabeling is applied. This should guarantee that targets with the same set of labels go to the same `vmagent` node in the cluster. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1687).
* FEATURE: vmagent: automatically switch to [stream parsing mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode) if the response from the given target exceeds the command-line flag value `-promscrape.minResponseSizeForStreamParse`. This should reduce memory usage when `vmagent` scrapes targets with non-uniform response sizes (this is the case in Kubernetes monitoring).
* FEATURE: vmagent: send Prometheus-like staleness marks in [stream parsing mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode). Previously staleness marks weren't sent in stream parsing mode. See [these docs](https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers) for details.
* FEATURE: vmagent: properly calculate `scrape_series_added` metric for targets in [stream parsing mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode). Previously it was set to 0 in stream parsing mode. See [more details about this metric](https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series).
* FEATURE: vmagent: expose `promscrape_series_limit_max_series` and `promscrape_series_limit_current_series` metrics at `http://vmagent:8429/metrics` for scrape targets with the [enabled series limiter](https://docs.victoriametrics.com/vmagent/#cardinality-limiter).
* FEATURE: vmagent: return error if `sample_limit` or `series_limit` options are set when [stream parsing mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode) is enabled, since these limits cannot be applied in stream parsing mode.
* FEATURE: vmalert: add `-remoteRead.disablePathAppend` command-line flag, which allows specifying the full `-remoteRead.url`. If `-remoteRead.disablePathAppend` is set, then `vmalert` doesn't add `/api/v1/query` suffix to `-remoteRead.url`.
* FEATURE: add trigonometric functions, which are going to be added in [Prometheus 2.31](https://github.com/prometheus/prometheus/pull/9239): [acosh](https://docs.victoriametrics.com/metricsql/#acosh), [asinh](https://docs.victoriametrics.com/metricsql/#asinh), [atan](https://docs.victoriametrics.com/metricsql/#atan), [atanh](https://docs.victoriametrics.com/metricsql/#atanh), [cosh](https://docs.victoriametrics.com/metricsql/#cosh), [deg](https://docs.victoriametrics.com/metricsql/#deg), [rad](https://docs.victoriametrics.com/metricsql/#rad), [sinh](https://docs.victoriametrics.com/metricsql/#sinh), [tan](https://docs.victoriametrics.com/metricsql/#tan), [tanh](https://docs.victoriametrics.com/metricsql/#tanh). Also add `atan2` binary operator. See [this pull request](https://github.com/prometheus/prometheus/pull/9248).
* FEATURE: consistently return the same set of time series from [limitk](https://docs.victoriametrics.com/metricsql/#limitk) function. This improves the usability of periodically refreshed graphs.
* FEATURE: [vmui](https://docs.victoriametrics.com/#vmui): varios UX improvements. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1711) and [these docs](https://docs.victoriametrics.com/#vmui).
* FEATURE: [vmauth](https://docs.victoriametrics.com/vmauth/): add ability to specify HTTP headers, which will be sent in requests to backends. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1736).
* FEATURE: add `/flags` page to all the VictoriaMetrics components. This page contains command-line flags passed to the component.
* FEATURE: allow using tab separators additionally to whitespace separators when [ingesting data in Graphite plaintext protocol](https://docs.victoriametrics.com/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd). Such separators are [supported by Carbon-c-relay](https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528).

* BUGFIX: vmstorage: fix `unaligned 64-bit atomic operation` panic on 32-bit architectures (arm and 386). The panic has been introduced in v1.67.0.
* BUGFIX: vmalert, vmauth: prevent from frequent closing of TCP connections established to backends under high load. This should reduce the number of TCP sockets in `TIME_WAIT` state at `vmalert` and `vmauth` under high load. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1704).
* BUGFIX: vmalert: correctly calculate alert ID including extra labels. Previously, ID for alert entity was generated without alertname or groupname. This led to collision, when multiple alerting rules within the same group producing same labelsets. E.g. expr: `sum(metric1) by (job) > 0` and expr: `sum(metric2) by (job) > 0` could result into same labelset `job: "job"`. The bugfix adds all extra labels right after receiving response from the datasource. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1734).
* BUGFIX: vmalert: fix links in [Web UI](https://docs.victoriametrics.com/vmalert/#web). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1717).
* BUGFIX: vmagent: set `honor_timestamps: true` by default in [scrape configs](https://docs.victoriametrics.com/sd_configs/#scrape_configs) if this options isn't set explicitly. This aligns the behaviour with Prometheus.
* BUGFIX: vmagent: group scrape targets by the original job names at `http://vmagent:8429/targets` page like Prometheus does. Previously they were grouped by the job name after relabeling, which may result in unexpected empty target groups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1707).
* BUGFIX: [vmctl](https://docs.victoriametrics.com/vmctl/): fix importing boolean fields from InfluxDB line protocol. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1709).

## [v1.67.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.67.0)

Released at 2021-10-08

* FEATURE: add ability to accept metrics from [DataDog agent](https://docs.datadoghq.com/agent/) and [DogStatsD](https://docs.datadoghq.com/developers/dogstatsd/). See [these docs](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-datadog-agent). This option simplifies the migration path from DataDog to VictoriaMetrics. See also [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/206).
* FEATURE: vmagent [enterprise](https://docs.victoriametrics.com/enterprise/): add support for data reading and writing from/to [Apache Kafka](https://kafka.apache.org/). See [these docs](https://docs.victoriametrics.com/vmagent/#kafka-integration).
* FEATURE: vmui: switch to [μPlot](https://github.com/leeoniya/uPlot) and add ability to naturally scroll and zoom graphs. See [these docs](https://docs.victoriametrics.com/#vmui). Thanks to @Loori-R.
* FEATURE: vmstorage: stop accepting new data if `-storageDataPath` directory contains less than `-storage.minFreeDiskSpaceBytes` of free space. This should prevent from `out of disk space` crashes. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/269).
* FEATURE: calculate quantiles in the same way as Prometheus does in such functions as [quantile_over_time](https://docs.victoriametrics.com/metricsql/#quantile_over_time) and [quantile](https://docs.victoriametrics.com/metricsql/#quantile). Previously results from VictoriaMetrics could be slightly different than results from Prometheus. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1625) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1612) issues.
* FEATURE: add `rollup_scrape_interval(m[d])` function to [MetricsQL](https://docs.victoriametrics.com/metricsql/), which returns `min`, `max` and `avg` values for the interval between samples for `m` on the given lookbehind window `d`.
* FEATURE: add `topk_last(k, q)` and `bottomk_last(k, q)` functions to [MetricsQL](https://docs.victoriametrics.com/metricsql/), which return up to `k` time series from `q` with the maximum / minimum last value on the graph.

* BUGFIX: align behavior of the queries `a or on (labels) b`, `a and on (labels) b` and `a unless on (labels) b` where `b` has multiple time series with the given `labels` to Prometheus behavior. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1643).
* BUGFIX: vmagent: fix `openstack_sd_config` service discovery when both `domain_name` and `project_id` config options are set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1655).
* BUGFIX: return proper values (zeroes) from [stddev_over_time](https://docs.victoriametrics.com/metricsql/#stddev_over_time) and [stdvar_over_time](https://docs.victoriametrics.com/metricsql/#stdvar_over_time) functions when the lookbehind window in square brackets contains only a single sample. Previously the sample value was incorrectly returned in this case.
* BUGFIX: vminsert: fix uneven distribution of time series among storage nodes in [multi-level cluster setup](https://docs.victoriametrics.com/cluster-victoriametrics/#multi-level-cluster-setup). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1672).

## [v1.66.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.66.2)

Released at 2021-09-23

* FEATURE: vmagent: add `vm_promscrape_max_scrape_size_exceeded_errors_total` metric for counting of the failed scrapes due to the exceeded response size (the response size limit can be configured via `-promscrape.maxScrapeSize` command-line flag). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1639).

* BUGFIX: vmalert: properly reload rule groups if only the `interval` config option is changed. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1641).
* BUGFIX: properly handle `{__name__=~"prefix(suffix1|suffix2)",other_label="..."}` queries. They may return unexpected empty responses since v1.66.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1644).

## [v1.66.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.66.1)

Released at 2021-09-22

* FEATURE: add `-cluster` and/or `-enterprise` suffixes to `short_version` label at `vm_app_version` metric exposed at `/metrics` page of every VictoriaMetrics component. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1635).

* BUGFIX: vmselect: fix accessing [Graphite APIs](https://docs.victoriametrics.com/#graphite-api-usage). The access has been broken in v1.66.0, because `/graphite/*` path prefix accidentally clashed with `/graph*` path prefix used for VictoriaMetrics UI (aka `vmui`).
* BUGFIX: fix parsing `regex: <bool_or_number>` in relabeling rules (for example, `regex: true` or `regex: 123`). The bug has been introduced in v1.66.0.

## [v1.66.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.66.0)

Released at 2021-09-20

* FEATURE: vmalert: add web UI with the list of alerting groups, alerts and alert statuses. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1602).
* FEATURE: vmalert: add `-rule.maxResolveDuration` command-line flag, which could be used for limiting the auto-resolve duration for the alerting rule. By default it is limited to 3x evaluation interval. This could be too high for big evaluation intervals. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1586).
* FEATURE: vmalert: add support for Bearer token authorization for `-datasource.url`, `-remoteRead.url` and `-remoteWrite.url`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1608).
* FEATURE: vmagent: send stale markers for disappeared metrics like Prometheus does. Previously stale markers were sent only when the scrape target disappears or when it becomes temporarily unavailable. See [these docs](https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers) for details.
* FEATURE: vmagent: add ability to set `series_limit` option for a particular scrape target via `__series_limit__` label. This allows setting the limit on the number of time series on a per-target basis. See [these docs](https://docs.victoriametrics.com/vmagent/#cardinality-limiter) for details.
* FEATURE: vmagent: add ability to set `stream_parse` option for a particular scrape target via `__stream_parse__` label. This allows managing the stream parsing mode on a per-target basis. See [these docs](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode) for details.
* FEATURE: vmagent: add ability to set `scrape_interval` and `scrape_timeout` options for a particular target via `__scrape_interval__` and `__scrape_timeout__` labels in the same way as Prometheus 2.30 does. See [this pull request](https://github.com/prometheus/prometheus/pull/8911).
* FEATURE: vmagent: generate `scrape_timeout_seconds` metric per each scrape target, so the target saturation could be calculated with `scrape_duration_seconds / scrape_timeout_seconds`. See the corresponding [pull request from Prometheus 2.30](https://github.com/prometheus/prometheus/pull/9247).
* FEATURE: vmagent: reduce CPU usage when calculating the number of newly added series per scrape (this number is sent to remote storage in `scrape_series_added` metric).
* FEATURE: vmagent: reduce CPU usage when applying `series_limit` to scrape targets with constant set of metrics. See more information about `series_limit` [here](https://docs.victoriametrics.com/vmagent/#cardinality-limiter).
* FEATURE: vminsert: disable rerouting by default when a few of `vmstorage` nodes start accepting data at lower speed than the rest of `vmstorage` nodes. This should improve VictoriaMetrics cluster stability during rolling restarts and during spikes in [time series churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate). The rerouting can be enabled by passing `-disableRerouting=false` command-line flag to `vminsert`.
* FEATURE: vmauth: do not put invalid auth tokens into log by default due to security reasons. The logging can be returned back by passing `-logInvalidAuthTokens` command-line flag to `vmauth`. Requests with invalid auth tokens are counted at `vmagent_http_request_errors_total{reason="invalid_auth_token"}` metric exposed by `vmauth` at `/metrics` page.
* FEATURE: add new relabeling actions: `keep_metrics` and `drop_metrics`. This simplifies metrics filtering by metric names. See [these docs](https://docs.victoriametrics.com/vmagent/#relabeling) for more details.
* FEATURE: allow splitting long `regex` in relabeling filters into an array of shorter regexps, which can be put into multiple lines for better readability and maintainability. See [these docs](https://docs.victoriametrics.com/vmagent/#relabeling) for more details.
* FEATURE: optimize performance for queries with regexp filters on metric name like `{__name__=~"metric1|...|metricN"}`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1610) from @faceair.
* FEATURE: vmui: use Prometheus-compatible query args, so `vmui` could be accessed from graph editor in Grafana. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1619). Thanks to @Loori-R.
* FEATURE: vmselect: automatically add missing port to `-storageNode` hostnames. For example, `-storageNode=vmstorage1,vmstorage2` is automatically translated to `-storageNode=vmstorage1:8401,vmstorage2:8401`. This simplifies [manual setup of VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-setup).
* FEATURE: vminsert: automatically add missing port to `-storageNode` hostnames. For example, `-storageNode=vmstorage1,vmstorage2` is automatically translated to `-storageNode=vmstorage1:8400,vmstorage2:8400`. This simplifies [manual setup of VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-setup).
* FEATURE: add [mad(q)](https://docs.victoriametrics.com/metricsql/#mad) function to [MetricsQL](https://docs.victoriametrics.com/metricsql/). It calculates [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) for groups of points with identical timestamps across multiple time series.
* FEATURE: add [outliers_mad(tolerance, q)](https://docs.victoriametrics.com/metricsql/#outliers_mad) function to [MetricsQL](https://docs.victoriametrics.com/metricsql/). It returns time series with peaks outside the [Median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) multiplied by `tolerance`.
* FEATURE: add `histogram_quantiles("phiLabel", phi1, ..., phiN, buckets)` function to [MetricsQL](https://docs.victoriametrics.com/metricsql/). It calculates the given `phi*`-quantiles over the given `buckets` and returns time series per each quantile with the corresponding `{phiLabel="phi*"}` label.
* FEATURE: add `quantiles_over_time("phiLabel", phi1, ..., phiN, series_selector[d])` function to [MetricsQL](https://docs.victoriametrics.com/metricsql/). It calculates the given `phi*`-quantiles over raw samples selected by `series_selector` on the given lookbehind window `d`. It returns time series per each quantile with the corresponding `{phiLabel="phi*"}` label.
* FEATURE: [enterprise](https://docs.victoriametrics.com/enterprise/): do not ask for `-eula` flag if `-version` flag is passed to enterprise app. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1621).

* BUGFIX: properly handle queries with multiple filters matching empty labels such as `metric{label1=~"foo|",label2="bar|"}`. This filter must match the following series: `metric`, `metric{label1="foo"}`, `metric{label2="bar"}` and `metric{label1="foo",label2="bar"}`. Previously it was matching only `metric{label1="foo",label2="bar"}`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601).
* BUGFIX: vmselect: reset connection timeouts after each request to `vmstorage`. This should prevent from `cannot read data in 0.000 seconds: unexpected EOF` warning in logs. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1562). Thanks to @mxlxm .
* BUGFIX: keep metric name for time series returned from [rollup_candlestick](https://docs.victoriametrics.com/metricsql/#rollup_candlestick) function, since the returned series don't change the meaning of the original series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1600).
* BUGFIX: use Prometheus-compatible label value formatting for [count_values](https://docs.victoriametrics.com/metricsql/#count_values) function. Previously big values could be improperly formatted, which could break query results, which rely on label value such as `... on(label) count_values("label", ...)`.
* BUGFIX: vmagent: properly use `https` scheme for wildcard TLS certificates for `role: ingress` targets in Kubernetes service discovery. See [this issue](https://github.com/prometheus/prometheus/issues/8902).
* BUGFIX: vmagent: support host networking mode for `docker_sd_config`. See [this issue](https://github.com/prometheus/prometheus/issues/9116).
* BUGFIX: fix non-repeatable results from `quantile_over_time()` function when the number of input samples exceeds 1000. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1612).
* BUGFIX: vmagent: fix EC2 zone discovery when `filters` are specified in [ec2_sc_config](https://docs.victoriametrics.com/sd_configs/#ec2_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1626).

## [v1.65.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.65.0)

Released at 2021-09-01

* FEATURE: vmagent: add ability to read scrape configs from multiple files specified in `scrape_config_files` section. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1559).
* FEATURE: vmagent: reduce memory usage and CPU usage when [Prometheus staleness tracking](https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers) is enabled for metrics exported from the deleted or disappeared scrape targets.
* FEATURE: vmagent: add the ability to limit the number of unique time series scraped per each target. This can be done either globally via `-promscrape.seriesLimitPerTarget` command-line option or on per-target basis via `series_limit` option at `scrape_config` section. See [the updated docs on cardinality limiter](https://docs.victoriametrics.com/vmagent/#cardinality-limiter) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1561).
* FEATURE: vmagent: discover `role: ingress` and `role: endpointslice` in [kubernetes_sd_config](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) via v1 API instead of v1beta1 API if Kubernetes supports it. This fixes service discovery in Kubernetes v1.22 and newer versions. See [these docs](https://kubernetes.io/docs/reference/using-api/deprecation-guide/#ingress-v122).
* FEATURE: take into account failed queries in `vm_request_duration_seconds` summary at `/metrics`. Previously only successful queries were taken into account. This could result in skewed summary. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1537).
* FEATURE: vmalert: add an official dashboard for vmalert. See [these docs](https://docs.victoriametrics.com/vmalert/#monitoring).
* FEATURE: vmalert: add ability to set additional labels per group via `labels` config section. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1471).
* FEATURE: vmalert: add `-disableAlertgroupLabel` command-line flag for disabling the label with alert group name. This may be needed for proper deduplication in Alertmanager. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1532).
* FEATURE: update Go builder from v1.16.7 to v1.17.0. This improves data ingestion and query performance by up to 5% according to benchmarks. See [the release post for Go1.17](https://go.dev/blog/go1.17).
* FEATURE: vmagent: expose `promscrape_discovery_http_errors_total` metric, which can be used for monitoring the number of failed discovery attempts per each `http_sd` config.
* FEATURE: do not reset response cache when a sample with old timestamp is ingested into VictoriaMetrics if `-search.disableAutoCacheReset` command-line option is set. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1570).
* FEATURE: add `quantiles("phiLabel", phi1, ..., phiN, q)` aggregate function to [MetricsQL](https://docs.victoriametrics.com/metricsql/), which calculates the given `phi*` quantiles over time series returned by `q`. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1573).

* BUGFIX: rename `sign` function to `sgn` in order to be consistent with PromQL. See [this pull request from Prometheus](https://github.com/prometheus/prometheus/pull/8457).
* BUGFIX: vmagent: add `role: endpointslice` in [kubernetes_sd_config](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) in order to be consistent with Prometheus. Previously this role was supported with incorrect name: `role: endpointslices`. Now both `endpointslice` and `endpointslices` are supported. See [the corresponding code in Prometheus](https://github.com/prometheus/prometheus/blob/2ec6c7dbb82b72834021e01f1773eb90a67a371f/discovery/kubernetes/kubernetes.go#L99).
* BUGFIX: improve the detection of the needed free space for background merge operation. This should prevent from possible out of disk space crashes during big merges. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1560).
* BUGFIX: vmauth: remove trailing slash from the full url before requesting it from the backend. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1554).
* BUGFIX: [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager/): fix timeout error when snapshot takes longer than 10 seconds. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1571).
* BUGFIX: properly parse OpenTSDB `put` messages with multiple spaces between message elements. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1574). Thanks to @envzhu for the fix.

## [v1.64.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.64.1)

Released at 2021-08-19

* FEATURE: add `bitmap_and(q, mask)`, `bitmap_or(q, mask)` and `bitmak_xor(q, mask)` functions to [MetricsQL](https://docs.victoriametrics.com/metricsql/). These functions allow performing bitwise operations over data points in time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1541).
* FEATURE: vmalert: add `-remoteWrite.disablePathAppend` command-line flag, which can be used when custom `-remoteWrite.url` must be specified. For example, `./vmalert -disablePathAppend -remoteWrite.url='http://foo.bar/a/b/c?d=e'` would write data to `http://foo.bar/a/b/c?d=e` instead of `http://foo.bar/a/b/c?d=e/api/v1/write`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1536).
* FEATURE: vmagent: add `-promscrape.noStaleMarkers` command-line flag for disabling sending Prometheus stale markers for metrics from disappeared scrape targets. This option may be used for reducing memory usage when scraping big number of metrics with big number of labels and when stale markers aren't needed.
* FEATURE: vmselect: add `-search.noStaleMarkers` command-line flag for disabling stale markers handling in queries. This may save some CPU time when the queried data doesn't contain stale markers.

* BUGFIX: vmagent: stop scrapers for deleted targets before starting scrapers for added targets. This should prevent from possible time series overlap when old targets are substituted by new targets (for example, during new deployment in Kubernetes). The overlap could lead to incorrect query results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1509).
* BUGFIX: vmagent: send Prometheus stale markers for the previously scraped metrics on failed scrapes like Prometheus does. See [this article](https://www.robustperception.io/staleness-and-promql).
* BUGFIX: upgrade base Docker image from Alpine 3.14.0 to Alpine 3.14.1 . This fixes potential security issues - see [Alpine 3.14.1 release notes](https://www.alpinelinux.org/posts/Alpine-3.14.1-released.html).
* BUGFIX: disable overriding the lookbehind window `[d]` at `last_over_time(m[d])` if `d` is smaller than the interval between samples, since users don't expect implicit overriding of explicitly set `[d]` in `last_over_time(m[d])`.

## [v1.64.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.64.0)

Released at 2021-08-15

* FEATURE: add support for Prometheus staleness markers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1526).
* FEATURE: vmagent: automatically generate Prometheus staleness markers for the scraped metrics when scrape targets disappear in the same way as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1526).
* FEATURE: add `present_over_time(m[d])` function, which returns 1 if `m` has a least a single sample over the previous duration `d`. This function has been added also to [Prometheus 2.29](https://github.com/prometheus/prometheus/releases/tag/v2.29.0).
* FEATURE: vmagent: support multitenant writes according to [these docs](https://docs.victoriametrics.com/vmagent/#multitenancy). This allows using a single `vmagent` instance in front of VictoriaMetrics cluster for all the tenants. Thanks to @omarghader for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1505). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1491).
* FEATURE: vmagent: add `__meta_ec2_availability_zone_id` label to discovered Amazon EC2 targets. This label is available in Prometheus [starting from v2.29](https://github.com/prometheus/prometheus/releases/tag/v2.29.0).
* FEATURE: vmagent: add `__meta_gce_interface_ipv4_<name>` labels to discovered GCE targets. These labels are available in Prometheus [starting from v2.29](https://github.com/prometheus/prometheus/releases/tag/v2.29.0).
* FEATURE: add `-search.maxSamplesPerSeries` command-line flag for limiting the number of raw samples a single query can process per each time series. This option can protect from out of memory errors when a query processes tens of millions of raw samples per series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1067).
* FEATURE: add `-search.maxSamplesPerQuery` command-line flag for limiting the number of raw samples a single query can process across all the time series. This option can protect from heavy queries, which select too big number of raw samples. Thanks to @jiangxinlingdu for [the initial pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1478).
* FEATURE: improve performance for queries that process big number of time series and/or samples on systems with big number of CPU cores.
* FEATURE: vmalert: expose `vmalert_alerting_rules_last_evaluation_samples` and `vmalert_recording_rules_last_evaluation_samples` metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1494).
* FEATURE: vminsert: expose `vm_rpc_send_duration_seconds_total` counter, which can be used for determining high saturation of every `vminsert -> vmstorage` link with an alerting query `rate(vm_rpc_send_duration_seconds_total) > 0.9s`. This query triggers when the link is saturated by more than 90%. This usually means that more `vminsert` or `vmstorage` nodes must be added to the cluster in order to increase the total number of `vminsert -> vmstorage` links.
* FEATURE: vmagent: expose `vmagent_remotewrite_send_duration_seconds_total` counter, which can be used for determining high saturation of every connection to remote storage with an alerting query `rate(vmagent_remotewrite_send_duration_seconds_total) > 0.9s`. This query triggers when a connection is saturated by more than 90%. This usually means that `-remoteWrite.queues` command-line flag must be increased in order to increase the number of connections per each remote storage.
* FEATURE: vmui: automatically fill Server URL field. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1506).

* BUGFIX: fix corner cases for queries on time ranges exceeding 40 days. Previously some series can be missing in query results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1486).
* BUGFIX: vmselect: return dummy response at `/rules` page in the same way as for `/api/v1/rules` page. The `/rules` page is requested by Grafana 8. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1493) for details.
* BUGFIX: vmbackup: automatically set default `us-east-1` S3 region if it is missing. This should simplify using S3-compatible services such as MinIO for backups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1449).
* BUGFIX: vmselect: prevent from possible deadlock when multiple `target` query args are passed to [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage).
* BUGFIX: return series with `a op b` labels and `N` values for `(a op b) default N` if `(a op b)` returns series with all NaN values. Previously such series were removed.
* BUGFIX: vmui: fix layout when the query selects more than 27 time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1497).
* BUGFIX: vmagent: restore highlighting in red for DOWN targets at `/targets` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1461).

## [v1.63.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.63.0)

Released at 2021-07-15

* FEATURE: reduce memory usage by up to 30% on production workloads.
* FEATURE: vmselect: embed [vmui](https://github.com/VictoriaMetrics/vmui) into a single-node VictoriaMetrics and into `vmselect` component of cluster version. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1413). The web interface is available at the following paths:
  * `/vmui/` for a single-node VictoriaMetrics
  * `/select/<accountID>/vmui/` for `vmselect` at cluster version of VictoriaMetrics
* FEATURE: support durations anywhere in [MetricsQL queries](https://docs.victoriametrics.com/metricsql/). For example, `sum_over_time(m[1h]) / 1h` is a valid query, which is equivalent to `sum_over_time(m[1h]) / 3600`.
* FEATURE: support durations without suffixes in [MetricsQL queries](https://docs.victoriametrics.com/metricsql/). For example, `rate(m[3600])` is a valid query, which is equivalent to `rate(m[1h])`.
* FEATURE: export `vmselect_request_duration_seconds` and `vminsert_request_duration_seconds` [VictoriaMetrics histograms](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) at `/metrics` page. These histograms can be used for determining latency distribution and SLI/SLO for the served requests. For example, the following query would return the percent of queries that took less than 500ms during the last hour: `histogram_share(500ms, sum(rate(vmselect_request_duration_seconds_bucket[1h])) by (vmrange))`.
* FEATURE: vmagent: dynamically reload client TLS certificates from disk on every [mTLS connection](https://developers.cloudflare.com/cloudflare-one/identity/devices/mutual-tls-authentication). This should allow using `vmagent` with [Istio service mesh](https://istio.io/latest/about/service-mesh/). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1420).
* FEATURE: log http request path plus all the query args on errors during request processing. Previously only http request path was logged without query args, so it could be hard debugging such errors.
* FEATURE: add `is_set` label to `flag` metrics. This allows determining explicitly set command-line flags with the query `flag{is_set="true"}`.
* FEATURE: add ability to remove caches stored inside `<-storageDataPath>/cache` on startup if `reset_cache_on_startup` file is present there. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1447).

* BUGFIX: vmagent: remove `{ %space %}` typo in `/targets` output. The typo has been introduced in v1.62.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1408).
* BUGFIX: vmagent: fix CSS styles on `/targets` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1422).
* BUGFIX: vmalert: accept Prometheus-like durations in `interval` config option inside `group` section. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1444).
* BUGFIX: properly update `vm_merge_need_free_disk_space` metric at `/metrics` page when there is no enough free disk space for performing optimal merges. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1373).

## [v1.62.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.62.0)

Released at 2021-06-25

* FEATURE: vmagent: add service discovery for Docker (aka [docker_sd_config](https://docs.victoriametrics.com/sd_configs/#docker_sd_configs)). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1402).
* FEATURE: vmagent: add service discovery for DigitalOcean (aka [digitalocean_sd_config](https://docs.victoriametrics.com/sd_configs/#digitalocean_sd_configs)). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1367).
* FEATURE: vmagent: change the default value for `-remoteWrite.queues` from 4 to `2 * numCPUs`. This should reduce scrape duration for highly loaded vmagent, which scrapes tens of thousands of targets. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1385).
* FEATURE: vmagent: show the number of samples the target returns during the last scrape on `/targets` and `/api/v1/targets` pages. This should simplify debugging targets, which may return too big or too low number of samples. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1377).
* FEATURE: vmagent: show jobs with zero discovered targets on `/targets` page. This should help debugging improperly configured scrape configs.
* FEATURE: vmagent: support for http-based service discovery (aka [http_sd_config](https://docs.victoriametrics.com/sd_configs/#http_sd_configs)), which has been added since Prometheus 2.28. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1392).
* FEATURE: vmagent: support namespace in Consul service discovery in the same way as Prometheus 2.28 does. See [this issue](https://github.com/prometheus/prometheus/issues/8894) for details.
* FEATURE: vmagent: support generic auth configs in `consul_sd_configs` in the same way as Prometheus 2.28 does. See [this issue](https://github.com/prometheus/prometheus/issues/8924) for details.
* FEATURE: [vmctl](https://docs.victoriametrics.com/vmctl/): limit the number of samples per each imported JSON line. This should limit the memory usage at VictoriaMetrics side when importing time series with big number of samples.
* FEATURE: vmselect: log slow queries across all the `/api/v1/*` handlers (aka [Prometheus query API](https://prometheus.io/docs/prometheus/latest/querying/api)) if their execution duration exceeds `-search.logSlowQueryDuration`. This should simplify debugging slow requests to such handlers as `/api/v1/labels` or `/api/v1/series` additionally to `/api/v1/query` and `/api/v1/query_range`, which were logged in the previous releases.
* FEATURE: vminsert: sort the `-storageNode` list in order to guarantee the identical `series -> vmstorage` mapping across all the `vminsert` nodes. This should reduce resource usage (RAM, CPU and disk IO) at `vmstorage` nodes if `vmstorage` addresses are passed in random order to `vminsert` nodes.
* FEATURE: vmstorage: reduce memory usage on a system with many CPU cores under high ingestion rate.

* BUGFIX: prevent from adding new samples to deleted time series after the rotation of the inverted index (the rotation is performed once per `-retentionPeriod`). See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347#issuecomment-861232136) for details.
* BUGFIX: vmstorage: reduce high disk write IO usage on systems with big number of CPU cores. The issue has been introduced in the release [v1.59.0](#v1590). See [this commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/aa9b56a046b6ae8083fa659df35dd5e994bf9115) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1338#issuecomment-863046999) for details.
* BUGFIX: vmstorage: prevent from incorrect stats collection when multiple concurrent queries execute the same tag filter. This may help reducing CPU usage under certain workloads. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1338).
* BUGFIX: vmselect: return the last timestamp for the max / min value from `tmax_over_time(m[d])` and `tmin_over_time(m[d])` [MetricsQL functions](https://docs.victoriametrics.com/metricsql/) as most users expect. See also [this issue](https://github.com/prometheus/prometheus/issues/8966).
* BUGFIX: vmselect: return the expected value for `increase_pure()` [MetricsQL function](https://docs.victoriametrics.com/metricsql/) after a gap in a time series. Previously incorrect too big value could be returned after the gap from `increase_pure()`.

## [v1.61.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.61.1)

Released at 2021-06-11

* BUGFIX: vmalert: fix recording rules, which were broken in v1.61.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1369).
* BUGFIX: reset the on-disk cache for mapping from the full metric name to an internal metric id (e.g. `metric_name{labels} -> internal_metric_id`) after deleting metrics via [delete API](https://docs.victoriametrics.com/#how-to-delete-time-series). This should prevent from possible inconsistent state after unclean shutdown. This [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347).

## [v1.61.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.61.0)

Released at 2021-06-09

* FEATURE: vmalert: add support for backfilling (aka replay) of recording and alerting rules. See [these docs](https://docs.victoriametrics.com/vmalert/#rules-backfilling) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/836).
* FEATURE: vmalert: add a command-line flag `-rule.configCheckInterval` for automatic re-reading of `-rule` files without the need to send SIGHUP signal. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/512).
* FEATURE: vmagent: respect the `sample_limit` and `-promscrape.maxScrapeSize` values when scraping targets in [stream parsing mode](https://docs.victoriametrics.com/vmagent/#stream-parsing-mode). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1331).
* FEATURE: vmauth: add ability to specify mutliple `url_prefix` entries for balancing the load among multiple `vmselect` and/or `vminsert` nodes in a cluster. See [these docs](https://docs.victoriametrics.com/vmauth/#load-balancing).
* FEATURE: vminsert: add `-disableRerouting` command-line flag for forcibly disabling the rerouting. This should help resolving [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1054) issues.
* FEATURE: vminsert: reduce the probability of global re-routing storm if all the vmstorage nodes cannot keep up with the given ingestion rate for some time. This should improve cluster stability in such cases. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1054) issues.
* FEATURE: allow building VictoriaMetrics components for Solaris / SmartOS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1322).
* FEATURE: vmagent: add ability to debug relabeling rules. See [these docs](https://docs.victoriametrics.com/vmagent/#relabeling) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1343).

* BUGFIX: reduce CPU usage by up to 2x during querying a database with big number of active daily time series. The issue has been introduced in `v1.59.0`.
* BUGFIX: vmagent: properly apply auth and tls configs in `eureka_sd_configs`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1350).
* BUGFIX: vmauth: do not panic on aborted http requests. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353).
* BUGFIX: properly generate `target` property for `*Series(foo.*.bar)` responses returned from [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage). Previously the `target` contained the expanded list of series for `foo.*.bar`, e.g. `sumSeries(foo.a.bar,foo.b.bar,...foo.z.bar)`. Now VictoriaMetrics returns `sumSeries(foo.*.bar)` as a target in the same way as Graphite does.

## [v1.60.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.60.0)

Released at 2021-05-24

* FEATURE: add ability to limit the number of unique time series, which can be added to storage per hour and per day. This can help dealing with high cardinality and high churn rate issues. See [these docs](https://docs.victoriametrics.com/#cardinality-limiter).
* FEATURE: vmagent: add ability to limit the number of unique time series, which can be sent to remote storage systems per hour and per day. This can help dealing with high cardinality and high churn rate issues. See [these docs](https://docs.victoriametrics.com/vmagent/#cardinality-limiter).
* FEATURE: vmalert: add ability to run alerting and recording rules for multiple tenants. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/740) and [these docs](https://docs.victoriametrics.com/vmalert/#multitenancy).
* FEATURE: vminsert: add support for data ingestion via other `vminsert` nodes. This allows building multi-level data ingestion paths in VictoriaMetrics cluster by writing data from one level of `vminsert` nodes to another level of `vminsert` nodes. See [these docs](https://docs.victoriametrics.com/cluster-victoriametrics/#multi-level-cluster-setup) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/541#issuecomment-835487858) for details.
* FEATURE: vmagent: reload `bearer_token_file`, `credentials_file` and `password_file` contents every second. This allows dynamically changing the contents of these files during target scraping and service discovery without the need to restart `vmagent`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1297).
* FEATURE: vmalert: add a flag to control behaviour on startup for state restore errors. Such errors were returned and logged before as well. Now user can specify whether to just log these errors (`-remoteRead.ignoreRestoreErrors=true`) or to stop the process (`-remoteRead.ignoreRestoreErrors=false`). The latter is important when VM isn't ready yet to serve queries from vmalert and it needs to wait. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1252).
* FEATURE: vmalert: add ability to pass `round_digits` query arg to datasource via `-datasource.roundDigits` command-line flag. This can be used for limiting the number of decimal digits after the point in recording rule results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/525).
* FEATURE: return `X-Server-Hostname` header in http responses of all the VictoriaMetrics components. This should simplify tracing the origin server behind a load balancer or behind auth proxy during troubleshooting.
* FEATURE: vmselect: allow to use 2x more memory for query processing at `vmselect` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/). This should allow processing heavy queries without the need to increase RAM size at `vmselect` nodes.
* FEATURE: add ability to filter `/api/v1/status/tsdb` output with arbitrary [time series selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) passed via `match[]` query args. See [these docs](https://docs.victoriametrics.com/#tsdb-stats) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1168) for details.
* FEATURE: automatically detect memory and cpu limits for VictoriaMetrics components running under [cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) environments such as [HashiCorp Nomad](https://www.nomadproject.io/). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1269).
* FEATURE: vmauth: allow `-auth.config` reloading via `/-/reload` http endpoint. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1194).
* FEATURE: add `timezone_offset(tz)` function. It returns offset in seconds for the given timezone `tz` relative to UTC. This can be useful when combining with datetime-related functions. For example, `day_of_week(time()+timezone_offset("America/Los_Angeles"))` would return weekdays for `America/Los_Angeles` time zone. Special `Local` time zone can be used for returning an offset for the time zone set on the host where VictoriaMetrics runs. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1306) and [MetricsQL docs](https://docs.victoriametrics.com/metricsql/) for more details.
* FEATURE: vmagent: add support for OAuth2 authorization for scrape targets and service discovery in the same way as Prometheus does. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2).
* FEATURE: vmagent: add support for OAuth2 authorization when writing data to `-remoteWrite.url`. See `-remoteWrite.oauth2.*` config params in `/path/to/vmagent -help` output.
* FEATURE: vmalert: add ability to set `extra_filter_labels` at alerting and recording group configs. See [these docs](https://docs.victoriametrics.com/vmalert/#groups).
* FEATURE: vmstorage: reduce memory usage by up to 30% when ingesting big number of active time series.

* BUGFIX: vmagent: do not retry scraping targets, which don't support HTTP. This should reduce CPU load and network usage at `vmagent` and at scrape target. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1289).
* BUGFIX: vmagent: fix possible race when refreshing `role: endpoints` and `role: endpointslices` scrape targets in `kubernetes_sd_config`. Previously `pod` objects could be updated after the related `endpoints` object update. This could lead to missing scrape targets. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240).
* BUGFIX: vmagent: properly spread scrape targets among `vmagent` replicas if `-promscrape.cluster.replicationFactor` exceeds 1. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1292).
* BUGFIX: vmagent: limit `scrape_timeout` by `scrape_interval`. This guarantees that only a single sample is lost during the configured `scrape_interval` when scrape target responds slowly. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1281#issuecomment-840538907) for details.
* BUGFIX: properly remove stale parts outside the configured retention if `-retentionPeriod` is smaller than one month. Previously stale parts could remain active for up to a month after they go outside the retention.
* BUGFIX: stop the process on panic errors, since such errors may leave the process in inconsistent state. Previously panics could be recovered, which could result in unexpected hard-to-debug further behavior of running process.
* BUGFIX: vminsert, vmagent: make sure data ingestion connections are closed before completing graceful shutdown. Previously the connection may remain open, which could result in trailing samples loss.
* BUGFIX: vmauth, vmalert: properly re-use HTTP keep-alive connections to backends and datasources. Previously only 2 keep-alive connections per backend could be re-used. Other connections were closed after the first request. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1300) for details.
* BUGFIX: vmalert: fix false positive error `result contains metrics with the same labelset after applying rule labels`, which could be triggered when recording rules generate unique metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1293).
* BUGFIX: vmctl: properly import InfluxDB rows if they have a field and a tag with identical names. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1299).
* BUGFIX: properly reload configs if `SIGHUP` signal arrives during service initialization. Previously such `SIGHUP` signal could be ignored and configs weren't reloaded.
* BUGFIX: vmalert: properly import default rules from OpenShift. See [this issue](https://github.com/VictoriaMetrics/operator/issues/243).
* BUGFIX: reduce the probability of `the removal queue is full` panic when highly loaded VictoriaMetrics stores data on NFS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1313).

## [v1.59.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.59.0)

Released at 2021-05-01

* FEATURE: improved new time series registration speed on systems with many CPU cores. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1244). Thanks to @waldoweng for the idea and [draft implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1243).
* FEATURE: vmalert: use the same technique as Grafana for determining evaluation timestamps for recording rules. This should make consistent graphs for series generated by recording rules compared to graphs generated for queries from recording rules in Grafana. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1232).
* FEATURE: vmauth: add ability to set mandatory query args in `url_prefix`. For example, `url_prefix: http://vm:8428/?extra_label=team=dev` would add `extra_label=team=dev` query arg to all the incoming requests. See [the example](https://docs.victoriametrics.com/vmauth/#auth-config) for more details.
* FEATURE: vmctl: add OpenTSDB migration option. See more details [here](https://docs.victoriametrics.com/vmctl#migrating-data-from-opentsdb).
Thanks to @johnseekins!
* FEATURE: log metrics with dropped labels if the number of labels in the ingested metric exceeds `-maxLabelsPerTimeseries`. This should simplify debugging for this case.
* FEATURE: vmagent: list user-visible endpoints at `http://vmagent:8429/`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1251).

* BUGFIX: vmagent: properly update `role: endpoints` and `role: endpointslices` scrape targets if the underlying service objects are updated in `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240).
* BUGFIX: vmagent: apply `scrape_timeout` on receiving the first response byte from `stream_parse: true` scrape targets. Previously it was applied to receiving and *processing* the full response stream. This could result in false timeout errors when scrape target exposes millions of metrics as described [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1017#issuecomment-767235047).
* BUGFIX: vmagent: eliminate possible data race when obtaining value for the metric `vm_persistentqueue_bytes_pending`. The data race could result in incorrect value for this metric.
* BUGFIX: vmstorage: remove empty directories on startup. Such directories can be left after unclean shutdown on NFS storage. Previously such directories could lead to crashloop until manually removed. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1142).

## [v1.58.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.58.0)

Released at 2021-04-08

* FEATURE: vminsert and vmagent: add `-sortLabels` command-line flag for sorting metric labels before pushing them to `vmstorage`. This should reduce the size of `MetricName -> internal_series_id` cache (aka `vm_cache_size_bytes{type="storage/tsid"}`) when ingesting samples for the same time series with distinct order of labels. For example, `foo{k1="v1",k2="v2"}` and `foo{k2="v2",k1="v1"}` represent a single time series. Labels sorting is disabled by default, since the majority of established exporters preserve the order of labels for the exported metrics.
* FEATURE: allow specifying label value alongside label name for the `others sum` time series returned from `topk_*` and `bottomk_*` functions from [MetricsQL](https://docs.victoriametrics.com/metricsql/). For example, `topk_avg(3, max(process_resident_memory_bytes) by (instance), "instance=other_sum")` would return top 3 series from `max(process_resident_memory_bytes) by (instance)` plus a series containing the sum of other series. The `others sum` series will have `{instance="other_sum"}` label.
* FEATURE: do not delete `dst_label` when applying `label_copy(q, "src_label", "dst_label")` and `label_move(q, "src_label", "dst_label")` to series without `src_label` and with non-empty `dst_label`. See more details at [MetricsQL docs](https://docs.victoriametrics.com/metricsql/).
* FEATURE: update Go builder from `v1.16.2` to `v1.16.3`. This should fix [these issues](https://github.com/golang/go/issues?q=milestone%3AGo1.16.3+label%3ACherryPickApproved).
* FEATURE: vmagent: add support for `follow_redirects` option to `scrape_configs` section in the same way as [Prometheus 2.26 does](https://github.com/prometheus/prometheus/pull/8546).
* FEATURE: vmagent: add support for `authorization` section in `-promscrape.config` in the same way as [Prometheus 2.26 does](https://github.com/prometheus/prometheus/pull/8512).
* FEATURE: vmagent: add support for socks5 proxy in `proxy_url` config option. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1177).
* FEATURE: vmagent: add support for `socks5 over tls` proxy in `proxy_url` config option. It can be set up with the following config: `proxy_url: "tls+socks5://proxy-addr:port"`.
* FEATURE: vmagent: reduce memory usage when `-remoteWrite.queues` is set to a big value. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1167).
* FEATURE: vmagent: add AWS IAM roles for tasks support for EC2 service discovery according to [these docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html).
* FEATURE: vmagent: add support for `proxy_tls_config`, `proxy_authorization`, `proxy_basic_auth`, `proxy_bearer_token` and `proxy_bearer_token_file` options in `consul_sd_config`, `dockerswarm_sd_config` and `eureka_sd_config` sections.
* FEATURE: vmagent: pass `X-Prometheus-Scrape-Timeout-Seconds` header to scrape targets as Prometheus does. In this case scrape targets can limit the time needed for performing the scrape. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1179#issuecomment-813118733) for details.
* FEATURE: vmagent: drop corrupted persistent queue files at `-remoteWrite.tmpDataPath` instead of throwing a fatal error. Corrupted files can appear after unclean shutdown of `vmagent` such as OOM kill or hardware reset. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1030).
* FEATURE: vmauth: add support for authorization via [bearer token](https://swagger.io/docs/specification/authentication/bearer-authentication/). See [the docs](https://docs.victoriametrics.com/vmauth/#auth-config) for details.
* FEATURE: publish `arm64` and `amd64` binaries for cluster version of VictoriaMetrics at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).

* BUGFIX: properly handle `/api/v1/labels` and `/api/v1/label/<label_name>/values` queries on big `start ... end` time range. This should fix big resource usage when VictoriaMetrics is queried with [Promxy](https://github.com/jacksontj/promxy) v0.0.62 or newer versions.
* BUGFIX: do not break sort order for series returned from `topk*`, `bottomk*` and `outliersk` [MetricsQL](https://docs.victoriametrics.com/metricsql/) functions. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1189).
* BUGFIX: vmagent: properly work with simple HTTP proxies which don't support `CONNECT` method. For example, [PushProx](https://github.com/prometheus-community/PushProx). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1179).
* BUGFIX: vmagent: properly discover targets if multiple namespace selectors are put inside `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1170).
* BUGFIX: vmagent: properly discover `role: endpoints` and `role: endpointslices` targets in `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1182).
* BUGFIX: properly generate filename for `*.tar.gz` archive inside `_checksums.txt` file posted at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1171).

## [v1.57.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.57.1)

Released at 2021-03-30

* FEATURE: publish vmutils for `GOOS=arm` on [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).

* BUGFIX: prevent from possible incomplete query results after timed out query.
* BUGFIX: vmselect: remove `-search.storageTimeout` command-line flag, since it has the same meaning as `-search.maxQueryDuration`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711#issuecomment-808884995).
* BUGFIX: vminsert: return back `type` label to per-tenant metric `vm_tenant_inserted_rows_total`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/932).

## [v1.57.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.57.0)

Released at 2021-03-29

* FEATURE: optimize query performance by up to 10x on systems with many CPU cores. See [this tweet](https://twitter.com/MetricsVictoria/status/1375064484860067840).
* FEATURE: add the following metrics at `/metrics` page for every VictoriaMetrics app:
  * `process_resident_memory_anon_bytes` - RSS share for memory allocated by the process itself.  This share cannot be freed by the OS, so it must be taken into account by OOM killer.
  * `process_resident_memory_file_bytes` - RSS share for page cache memory (aka memory-mapped files). This share can be freed by the OS at any time, so it must be ignored by OOM killer.
  * `process_resident_memory_shared_bytes` - RSS share for memory shared with other processes (aka shared memory). This share can be freed by the OS at any time, so it must be ignored by OOM killer.
  * `process_resident_memory_peak_bytes` - peak RSS usage for the process.
  * `process_virtual_memory_peak_bytes` - peak virtual memory usage for the process.
* FEATURE: accept and enforce `extra_label=<label_name>=<label_value>` query arg at [Graphite APIs](https://docs.victoriametrics.com/#graphite-api-usage).
* FEATURE: use InfluxDB field as metric name if measurement is empty and `-influxSkipSingleField` command-line is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1139).
* FEATURE: vmagent: add `-promscrape.consul.waitTime` command-line flag for tuning the maximum wait time for Consul service discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1144).
* FEATURE: vmagent: add `vm_promscrape_discovery_kubernetes_stale_resource_versions_total` metric for monitoring the frequency of `too old resource version` errors during Kubernetes service discovery.
* FEATURE: single-node VictoriaMetrics: log metrics with timestamps older than `-search.cacheTimestampOffset` compared to the current time. See [these docs](https://docs.victoriametrics.com/#backfilling) for details.

* BUGFIX: prevent from infinite loop on `{__graphite__="..."}` filters when a metric name contains `*`, `{` or `[` chars.
* BUGFIX: prevent from infinite loop in `/metrics/find` and `/metrics/expand` [Graphite Metrics API handlers](https://docs.victoriametrics.com/#graphite-metrics-api-usage) when they match metric names or labels with `*`, `{` or `[` chars.
* BUGFIX: do not merge duplicate time series during requests to `/api/v1/query`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1141>
* BUGFIX: vmagent: properly handle `too old resource version` error messages from Kubernetes watch API. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1150>
* BUGFIX: vmagent: do not retry sending data blocks if remote storage returns `400 Bad Request` error. The number of dropped blocks due to such errors can be monitored with `vmagent_remotewrite_packets_dropped_total` metrics. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1149>
* BUGFIX: properly calculate `summarize` and `*Series` functions in [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage).

## [v1.56.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.56.0)

Released at 2021-03-17

* FEATURE: add the following functions to [MetricsQL](https://docs.victoriametrics.com/metricsql/):
  * `histogram_avg(buckets)` - returns the average value for the given buckets.
  * `histogram_stdvar(buckets)` - returns standard variance for the given buckets.
  * `histogram_stddev(buckets)` - returns standard deviation for the given buckets.
* FEATURE: export `vm_available_memory_bytes` and `vm_available_cpu_cores` metrics, which show the number of available RAM and available CPU cores for VictoriaMetrics apps.
* FEATURE: export `vm_index_search_duration_seconds` histogram, which can be used for troubleshooting time series search performance.
* FEATURE: vmagent: add ability to replicate scrape targets among `vmagent` instances in the cluster with `-promscrape.cluster.replicationFactor` command-line flag. See [these docs](https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets).
* FEATURE: vmagent: accept `scrape_offset` option at `scrape_config`. This option may be useful when scrapes must start at the specified offset of every scrape interval. See [these docs](https://docs.victoriametrics.com/vmagent/#troubleshooting) for details.
* FEATURE: vmagent: support `proxy_tls_config`, `proxy_basic_auth`, `proxy_bearer_token` and `proxy_bearer_token_file` options at `scrape_config` section for configuring proxies specified via `proxy_url`. See [these docs](https://docs.victoriametrics.com/vmagent/#scraping-targets-via-a-proxy).
* FEATURE: vmauth: allow using regexp paths in `url_map`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1112) for details.
* FEATURE: accept `round_digits` query arg at `/api/v1/query` and `/api/v1/query_range` handlers. This option can be set at Prometheus datasource in Grafana for limiting the number of digits after the decimal point in response values.
* FEATURE: add `-influx.databaseNames` command-line flag, which can be used for accepting data from some Telegraf plugins such as [fluentd plugin](https://github.com/fangli/fluent-plugin-influxdb). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1124).
* FEATURE: add `-logNewSeries` command-line flag, which can be used for debugging the source of time series churn rate.
* FEATURE: publish Windows builds for [vmagent](https://docs.victoriametrics.com/vmagent/), [vmalert](https://docs.victoriametrics.com/vmalert/), [vmauth](https://docs.victoriametrics.com/vmauth/) and [vmctl](https://docs.victoriametrics.com/vmctl/) at `vmutils-windows-*.zip` archives at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
* FEATURE: listen for IPv6 UDP if `-enableTCP6` command-line flag is passed to VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1131).

* BUGFIX: vmagent: prevent from high CPU usage bug during failing scrapes with small `scrape_timeout` (less than a few seconds).
* BUGFIX: vmagent: reduce memory usage when Kubernetes service discovery is used in big number of distinct scrape config jobs by sharing Kubernetes object cache. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1113>
* BUGFIX: vmagent: apply `sample_limit` only after `metric_relabel_configs` are applied as Prometheus does. Previously the `sample_limit` was applied before metrics relabeling.
* BUGFIX: vmagent: properly apply `tls_config`, `basic_auth` and `bearer_token` to proxy connections if `proxy_url` option is set. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1116>
* BUGFIX: vmagent: properly scrape targets via https proxy specified in `proxy_url` if `insecure_skip_verify` flag isn't set in `tls_config` section. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1116>
* BUGFIX: avoid `duplicate time series` error if `prometheus_buckets()` covers a time range with distinct set of buckets.
* BUGFIX: prevent exponent overflow when processing extremely small values close to zero such as `2.964393875E-314`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1114>
* BUGFIX: do not include datapoints with a timestamp `t-d` when returning results from `/api/v1/query?query=m[d]&time=t` as Prometheus does.
* BUGFIX: do not crash if a query contains `histogram_over_time()` function name with uppercase chars. For example, `Histogram_Over_Time(m[5m])`.

## [v1.55.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.55.1)

Released at 2021-03-03

* BUGFIX: vmagent: fix a panic in Kubernetes service discovery when a target is filtered out with relabeling. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1107>
* BUGFIX: vmagent: fix Kubernetes service discovery for `role: ingress`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1110>

## [v1.55.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.55.0)

Released at 2021-03-02

* FEATURE: add `sign(q)` and `clamp(q, min, max)` functions, which are planned to be added in [the upcoming Prometheus release](https://twitter.com/roidelapluie/status/1363428376162295811) . The `last_over_time(m[d])` function is already supported in [MetricsQL](https://docs.victoriametrics.com/metricsql/).
* FEATURE: vmagent: add `scrape_align_interval` config option, which can be used for aligning scrapes to the beginning of the configured interval. See [these docs](https://docs.victoriametrics.com/vmagent/#troubleshooting) for details.
* FEATURE: expose io-related metrics at `/metrics` page for every VictoriaMetrics component:
  * `process_io_read_bytes_total` - the number of bytes read via io syscalls such as read and pread
  * `process_io_written_bytes_total` - the number of bytes written via io syscalls such as write and pwrite
  * `process_io_read_syscalls_total` - the number of read syscalls such as read and pread
  * `process_io_write_syscalls_total` - the number of write syscalls such as write and pwrite
  * `process_io_storage_read_bytes_total` - the number of bytes read from storage layer
  * `process_io_storage_written_bytes_total` - the number of bytes written to storage layer
* FEATURE: vmagent: add ability to spread scrape targets among multiple `vmagent` instances. See [these docs](https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1084) for details.
* FEATURE: vmagent: use watch API for Kubernetes service discovery. This should reduce load on Kubernetes API server when it tracks big number of objects (for example, 10K pods). This should also reduce the time needed for k8s targets discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1057) for details.
* FEATURE: vmagent: export `vm_promscrape_target_relabel_duration_seconds` metric, which can be used for monitoring the time spend on relabeling for discovered targets.
* FEATURE: vmagent: optimize [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling) performance for common cases.
* FEATURE: add `increase_pure(m[d])` function to MetricsQL. It works the same as `increase(m[d])` except of various edge cases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962) for details.
* FEATURE: increase accuracy for `buckets_limit(limit, buckets)` results for small `limit` values. See [MetricsQL docs](https://docs.victoriametrics.com/metricsql/) for details.
* FEATURE: vmagent: initial support for Windows build with `CGO_ENABLED=0 GOOS=windows go build ./app/vmagent`. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1036).
* FEATURE: vmagent: support WebIdentityToken auth in EC2 service discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1080) for details.
* FEATURE: vmalert: properly process query params in `-datasource.url` and `-remoteRead.url` command-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1087) for details.

* BUGFIX: vmagent: properly apply `-remoteWrite.rateLimit` when `-remoteWrite.queues` is greater than 1. Previously there was a data race, which could prevent from proper rate limiting.
* BUGFIX: vmagent: properly perform graceful shutdown on `SIGINT` and `SIGTERM` signals. The graceful shutdown has been broken in `v1.54.0`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1065>
* BUGFIX: reduce the probability of `duplicate time series` errors when querying Kubernetes metrics.
* BUGFIX: properly calculate `histogram_quantile()` over time series with only a single non-zero bucket with `{le="+Inf"}`. Previously `NaN` was returned, now the value for the last bucket before `{le="+Inf"}` is returned like Prometheus does.
* BUGFIX: vmselect: do not cache partial query results on timeout when receiving data from `vmstorage` nodes. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1085>
* BUGFIX: properly handle `stale NFS file handle` error.
* BUGFIX: properly cache query results when `extra_label` query arg is used. Previously the cached results could clash for different `extra_label` values. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1095>
* BUGFIX: fix `http: superfluous response.WriteHeader call` issue. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1078>
* BUGFIX: fix arm64 builds due to the issue in `github.com/golang/snappy`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1074>
* BUGFIX: fix `index out of range [1024819115206086200] with length 27` panic, which could occur when `1e-9` value is passed to VictoriaMetrics histogram. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1096>
* BUGFIX: fix parsing for Graphite line with empty tags such as `foo; 123 456`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1100>
* BUGFIX: unescape only `\\`, `\n` and `\"` in label names when parsing Prometheus text exposition format as Prometheus does. Previously other escape sequences could be improperly unescaped.

## [v1.54.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.1)

Released at 2021-08-18

* BUGFIX: properly handle queries containing a filter on metric name plus any number of negative filters and zero non-negative filters. For example, `node_cpu_seconds_total{mode!="idle"}`. The bug was introduced in [v1.54.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.0).

## [v1.54.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.0)

Released at 2021-02-18

* FEATURE: optimize searching for matching metrics for `metric{<label_filters>}` queries if `<label_filters>` contains at least a single filter. For example, the query `up{job="foobar"}` should find the matching time series much faster than previously.
* FEATURE: reduce execution times for `q1 <binary_op> q2` queries by executing `q1` and `q2` in parallel.
* FEATURE: switch from Go1.15 to [Go1.16](https://golang.org/doc/go1.16) for building prod binaries.
* FEATURE: single-node VictoriaMetrics now accepts requests to handlers with `/prometheus` and `/graphite` prefixes such as `/prometheus/api/v1/query`. This improves compatibility with [handlers from VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format).
* FEATURE: expose `process_open_fds` and `process_max_fds` metrics. These metrics can be used for alerting when `process_open_fds` reaches `process_max_fds`. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/402> and <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1037>
* FEATURE: vmalert: add `-datasource.appendTypePrefix` command-line option for querying both Prometheus and Graphite datasource in cluster version of VictoriaMetrics. See [these docs](https://docs.victoriametrics.com/vmalert/#graphite) for details.
* FEATURE: vmauth: add ability to route requests from a single user to multiple destinations depending on the requested paths. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1064>
* FEATURE: remove dependency on external programs such as `cat`, `grep` and `cut` when detecting cpu and memory limits inside Docker or LXC container.
* FEATURE: vmagent: add `__meta_kubernetes_endpoints_label_*`, `__meta_kubernetes_endpoints_labelpresent_*`, `__meta_kubernetes_endpoints_annotation_*` and `__meta_kubernetes_endpoints_annotationpresent_*` labels for `role: endpoints` in Kubernetes service discovery. These labels where added in Prometheus 2.25.
* FEATURE: reduce the minimum supported retention period for inverted index (aka `indexdb`) from one month to one day. This should reduce disk space usage for `<-storageDataPath>/indexdb` folder if `-retentionPeriod` is set to values smaller than one month.
* FEATURE: vmselect: export per-tenant metrics `vm_vmselect_http_requests_total` and `vm_vmselect_http_requests_duration_ms_total` . Other per-tenant metrics are available as a part of [enterprise package](https://docs.victoriametrics.com/enterprise/). See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/932> for details.

* BUGFIX: properly convert regexp tag filters containing escaped dots to non-regexp tag filters. For example, `{foo=~"bar\.baz"}` should be converted to `{foo="bar.baz"}`. Previously it was incorrectly converted to `{foo="bar\.baz"}`, which could result in missing time series for this tag filter.
* BUGFIX: do not spam error logs when discovering Docker Swarm targets without dedicated IP. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1028> .
* BUGFIX: properly embed timezone data into VictoriaMetrics apps. This should fix `-loggerTimezone` usage inside Docker containers.
* BUGFIX: properly build Docker images for non-amd64 architectures (arm, arm64, ppc64le, 386) on [Docker hub](https://hub.docker.com/u/victoriametrics/). Previously these images were incorrectly based on amd64 base image, so they didn't work.
* BUGFIX: vmagent: return back unsent block to the queue during graceful shutdown. Previously this block could be dropped if remote storage is unavailable during vmagent shutdown. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1065> .

## [v1.53.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.53.1)

Released at 2021-02-03

* BUGFIX: vmselect: fix the bug preventing from proper searching by Graphite filter with wildcards such as `{__graphite__="foo.*.bar"}`.

## [v1.53.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.53.0)

Released at 2021-02-03

* FEATURE: added [vmctl tool](https://docs.victoriametrics.com/vmctl/) to VictoriaMetrics release process. Now it is packaged in `vmutils-*.tar.gz` archive on [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). Source code for `vmctl` tool has been moved from [github.com/VictoriaMetrics/vmctl](https://github.com/VictoriaMetrics/vmctl) to [github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmctl).
* FEATURE: added `-loggerTimezone` command-line flag for adjusting time zone for timestamps in log messages. By default UTC is used.
* FEATURE: added `-search.maxStepForPointsAdjustment` command-line flag, which can be used for disabling adjustment for points returned by `/api/v1/query_range` handler if such points have timestamps closer than `-search.latencyOffset` to the current time. Such points may contain incomplete data, so they are substituted by the previous values for `step` query args smaller than one minute by default.
* FEATURE: vmselect: added ability to use Graphite-compatible filters in MetricsQL via `{__graphite__="foo.*.bar"}` syntax. This expression is equivalent to `{__name__=~"foo[.][^.]*[.]bar"}`, but it works faster and it is easier to use when migrating from Graphite to VictoriaMetrics. This feature deprecates the usage of `-search.treatDotsAsIsInRegexps` command-line flag.
* FEATURE: vmselect: added ability to set additional label filters, which must be applied during queries. Such label filters can be set via optional `extra_label` query arg, which is accepted by [querying API](https://docs.victoriametrics.com/#prometheus-querying-api-usage) handlers. For example, the request to `/api/v1/query_range?extra_label=tenant_id=123&query=<query>` adds `{tenant_id="123"}` label filter to the given `<query>`. It is expected that the `extra_label` query arg is automatically set by auth proxy sitting
in front of VictoriaMetrics. [Contact us](mailto:sales@victoriametrics.com) if you need assistance with such a proxy. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1021> .
* FEATURE: vmalert: added `-datasource.queryStep` command-line flag for passing optional `step` query arg to `/api/v1/query` endpoint. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1025>
* FEATURE: vmalert: added ability to query Graphite datasource when evaluating alerting and recording rules. See [these docs](https://docs.victoriametrics.com/vmalert/#graphite) for details.
* FEATURE: vmagent: added `-remoteWrite.roundDigits` command-line option for rounding metric values to the given number of decimal digits after the point before sending the metric to the corresponding `-remoteWrite.url`. This option can be used for improving data compression on the remote storage, because values with lower number of decimal digits can be compressed better than values with bigger number of decimal digits.
* FEATURE: vmagent: added `-remoteWrite.rateLimit` command-line flag for limiting data transfer rate to `-remoteWrite.url`. This may be useful when big amounts of buffered data is sent after temporarily unavailability of the remote storage. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1035>
* FEATURE: vmagent: export the following additional metrics, which may be useful during troubleshooting:
  * `vm_promscrape_scrapes_failed_per_url_total`
  * `vm_promscrape_scrapes_skipped_by_sample_limit_per_url_total`
  * `vm_promscrape_discovery_requests_total`
  * `vm_promscrape_discovery_retries_total`
  * `vm_promscrape_scrape_retries_total`
  * `vm_promscrape_service_discovery_duration_seconds`
* FEATURE: vmselect: initial implementation for [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage).

* BUGFIX: vmagent: reduce HTTP reconnection rate for scrape targets. Previously vmagent could erroneously close HTTP keep-alive connections more frequently than needed.
* BUGFIX: vmagent: retry scrape and service discovery requests when the remote server closes HTTP keep-alive connection. Previously `disable_keepalive: true` option could be used under `scrape_configs` section when working with such servers.

## [v1.52.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.52.0)

Released at 2021-01-13

* FEATURE: provide a sample list of alerting rules for VictoriaMetrics components. It is available [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts).
* FEATURE: disable final merge for data for the previous month at the beginning of new month, since it may result in high disk IO and CPU usage. Final merge can be enabled by setting `-finalMergeDelay` command-line flag to positive duration.
* FEATURE: add `tfirst_over_time(m[d])` and `tlast_over_time(m[d])` functions to [MetricsQL](https://docs.victoriametrics.com/metricsql/) for returning timestamps for the first and the last data point in `m` over `d` duration.
* FEATURE: add ability to pass multiple labels to `sort_by_label()` and `sort_by_label_desc()` functions. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/992> .
* FEATURE: enforce at least TLS v1.2 when accepting HTTPS requests if `-tls`, `-tlsCertFile` and `-tlsKeyFile` command-line flags are set, because older TLS protocols such as v1.0 and v1.1 have been deprecated due to security vulnerabilities.
* FEATURE: support `extra_label` query arg for all HTTP-based [data ingestion protocols](https://docs.victoriametrics.com/#how-to-import-time-series-data). This query arg can be used for specifying extra labels which should be added for the ingested data.
* FEATURE: vmbackup: increase backup chunk size from 128MB to 1GB. This should reduce the number of Object storage API calls during backups by 8x. This may also reduce costs, since object storage API calls usually have non-zero costs. See <https://aws.amazon.com/s3/pricing/> and <https://cloud.google.com/storage/pricing#operations-pricing> .

* BUGFIX: properly parse escaped unicode chars in MetricsQL metric names, label names and function names. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/990>
* BUGFIX: override user-provided labels with labels set in `extra_label` query args during data ingestion over HTTP-based protocols.
* BUGFIX: vmagent: prevent from `dialing to the given TCP address time out` error when scraping big number of unavailable targets. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/987>
* BUGFIX: vmagent: properly show scrape duration on `/targets` page. Previously it was incorrectly shown as 0.000s.
* BUGFIX: vmagent: properly log errors when `-promscrape.streamParse` command-line flag is set. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1009>
* BUGFIX: vmagent: properly suppress errors when both `-promscrape.suppressScrapeErrors` and `-promscrape.streamParse` command-line flags are set. See <https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1009> .
* BUGFIX: vmalert: return non-empty result in template func `query` stub to pass validation. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/989> .
* BUGFIX: upgrade base image for Docker packages from Alpine 3.12.1 to Alpine 3.12.3 in order to fix potential security issues. See <https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1010>

## Previous releases

See changes for older releases [here](https://docs.victoriametrics.com/changelog_2020/).
