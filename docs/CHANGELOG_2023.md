---
sort: 104
weight: 104
title: CHANGELOG for the year 2023
menu:
  docs:
    parent: 'victoriametrics'
    weight: 104
aliases:
- /CHANGELOG_2023.html
---
## [v1.96.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.96.0)

Released at 2023-12-13

**vmalert's metrics `vmalert_alerting_rules_error` and `vmalert_recording_rules_error` were replaced with `vmalert_alerting_rules_errors_total` and `vmalert_recording_rules_errors_total`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5160) for details.**

* SECURITY: upgrade base docker image (Alpine) from 3.18.4 to 3.19.0. See [alpine 3.19.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.19.0-released.html).
* SECURITY: upgrade Go builder from Go1.21.4 to Go1.21.5. See [the list of issues addressed in Go1.21.5](https://github.com/golang/go/issues?q=milestone%3AGo1.21.5+label%3ACherryPickApproved).

* FEATURE: [vmauth](./vmauth.md): add ability to send requests to the first available backend and fall back to other `hot standby` backends when the first backend is unavailable. This allows building highly available setups as shown in [these docs](./vmauth.md#high-availability). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4792).
* FEATURE: `vmselect`: allow specifying multiple groups of `vmstorage` nodes with independent `-replicationFactor` per each group. See [these docs](./Cluster-VictoriaMetrics.md#vmstorage-groups-at-vmselect) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5197) for details.
* FEATURE: `vmselect`: allow opening [vmui](./README.md#vmui) and investigating [Top queries](./README.md#top-queries) and [Active queries](./README.md#active-queries) when the `vmselect` is overloaded with concurrent queries (e.g. when more than `-search.maxConcurrentRequests` concurrent queries are executed). Previously an attempt to open `Top queries` or `Active queries` at `vmui` could result in `couldn't start executing the request in ... seconds, since -search.maxConcurrentRequests=... concurrent requests are executed` error, which could complicate debugging of overloaded `vmselect` or single-node VictoriaMetrics.
* FEATURE: [vmagent](./vmagent.md): add `-enableMultitenantHandlers` command-line flag, which allows receiving data via [VictoriaMetrics cluster urls](./Cluster-VictoriaMetrics.md#url-format) at `vmagent` and converting [tenant ids](./Cluster-VictoriaMetrics.md#multitenancy) to (`vm_account_id`, `vm_project_id`) labels before sending the data to the configured `-remoteWrite.url`. See [these docs](./vmagent.md#multitenancy) for details.
* FEATURE: [vmagent](./vmagent.md): add `-remoteWrite.disableOnDiskQueue` command-line flag, which can be used for disabling data queueing to disk when the remote storage cannot keep up with the data ingestion rate. See [these docs](./vmagent.md#disabling-on-disk-persistence) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2110).
* FEATURE: [vmagent](./vmagent.md): add support for reading and writing samples via [Google PubSub](https://cloud.google.com/pubsub). See [these docs](./vmagent.md#google-pubsub-integration).
* FEATURE: [vmagent](./vmagent.md): show all the dropped targets together with the reason why they are dropped at `http://vmagent:8429/service-discovery` page. Previously targets, which were dropped because of [target sharding](./vmagent.md#scraping-big-number-of-targets) weren't displayed on this page. This could complicate service discovery debugging. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5389) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4018).
* FEATURE: reduce the default value for `-import.maxLineLen` command-line flag from 100MB to 10MB in order to prevent excessive memory usage during data import via [/api/v1/import](./README.md#how-to-import-data-in-json-line-format).
* FEATURE: [vmagent](./vmagent.md): add `keep_if_contains` and `drop_if_contains` relabeling actions. See [these docs](./vmagent.md#relabeling-enhancements) for details.
* FEATURE: [vmagent](./vmagent.md): export `vm_promscrape_scrape_pool_targets` [metric](./vmagent.md#monitoring) to track the number of targets each scrape job discovers. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5311).
* FEATURE: [vmalert](./vmalert.md): provide `/vmalert/api/v1/rule` and `/api/v1/rule` API endpoints to get the rule object in JSON format. See [these docs](./vmalert.md#web) for details.
* FEATURE: [vmalert](./vmalert.md): deprecate process gauge metrics `vmalert_alerting_rules_error` and `vmalert_recording_rules_error` in favour of `vmalert_alerting_rules_errors_total` and `vmalert_recording_rules_errors_total` counter metrics. [Counter](./keyConcepts.md#counter) metric type is more suitable for error counting as it preserves the state change between the scrapes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5160) for details.
* FEATURE: [MetricsQL](./MetricsQL.md): add [day_of_year()](./MetricsQL.md#day_of_year) function, which returns the day of the year for each of the given unix timestamps. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5345) for details. Thanks to @luckyxiaoqiang for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5368/).
* FEATURE: all VictoriaMetrics binaries: expose additional metrics at `/metrics` page, which may simplify debugging of VictoriaMetrics components (see [this feature request](https://github.com/VictoriaMetrics/metrics/issues/54)):
  * `go_sched_latencies_seconds` - the [histogram](./keyConcepts.md#histogram), which shows the time goroutines have spent in runnable state before actually running. Big values point to the lack of CPU time for the current workload.
  * `go_mutex_wait_seconds_total` - the [counter](./keyConcepts.md#counter), which shows the total time spent by goroutines waiting for locked mutex. Big values point to mutex contention issues.
  * `go_gc_cpu_seconds_total` - the [counter](./keyConcepts.md#counter), which shows the total CPU time spent by Go garbage collector.
  * `go_gc_mark_assist_cpu_seconds_total` - the [counter](./keyConcepts.md#counter), which shows the total CPU time spent by goroutines in GC mark assist state.
  * `go_gc_pauses_seconds` - the [histogram](./keyConcepts.md#histogram), which shows the duration of GC pauses.
  * `go_scavenge_cpu_seconds_total` - the [counter](./keyConcepts.md#counter), which shows the total CPU time spent by Go runtime for returning memory to the Operating System.
  * `go_memlimit_bytes` - the value of [GOMEMLIMIT](https://pkg.go.dev/runtime#hdr-Environment_Variables) environment variable.
* FEATURE: [vmui](./README.md#vmui): enhance autocomplete functionality with caching. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5348).
* FEATURE: add field `version` to the response for `/api/v1/status/buildinfo` API for using more efficient API in Grafana for receiving label values. Add additional info about setup Grafana datasource. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5370) and [these docs](./README.md#grafana-setup) for details.
* FEATURE: add `-search.maxResponseSeries` command-line flag for limiting the number of time series a single query to [`/api/v1/query`](./keyConcepts.md#instant-query) or [`/api/v1/query_range`](./keyConcepts.md#range-query) can return. This limit can protect Grafana from high memory usage when the query returns too many series. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5372).
* FEATURE: [Alerting rules for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts): ease aggregation for certain alerting rules to keep more useful labels for the context. Before, all extra labels except `job` and `instance` were ignored. See this [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5429) and this [follow-up commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/8fb68152e67712ed2c16dcfccf7cf4d0af140835). Thanks to @7840vz.
* FEATURE: [vmctl](./vmctl.md): allow reversing the migrating order from the newest to the oldest data for [vm-native](./vmctl.md#migrating-data-from-victoriametrics) and [remote-read](./vmctl.md#migrating-data-by-remote-read-protocol) modes via `--vm-native-filter-time-reverse` and `--remote-read-filter-time-reverse` command-line flags respectively. See: ./vmctl.md#using-time-based-chunking-of-migration and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5376).

* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate values for the first point on the graph for queries, which do not use [rollup functions](./MetricsQL.md#rollup-functions). For example, previously `count(up)` could return lower than expected values for the first point on the graph. This also could result in lower than expected values in the middle of the graph like in [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5388) when the response caching isn't disabled. The issue has been introduced in [v1.95.0](./CHANGELOG.md#v1950).
* BUGFIX: [vmagent](./vmagent.md): prevent from `FATAL: cannot flush metainfo` panic when [`-remoteWrite.multitenantURL`](./vmagent.md#multitenancy) command-line flag is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5357).
* BUGFIX: [vmagent](./vmagent.md): properly decode zstd-encoded data blocks received via [VictoriaMetrics remote_write protocol](./vmagent.md#victoriametrics-remote-write-protocol). See [this issue comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301#issuecomment-1815871992).
* BUGFIX: [vmagent](./vmagent.md): properly add new labels at `output_relabel_configs` during [stream aggregation](./stream-aggregation.md). Previously this could lead to corrupted labels in output samples. Thanks to @ChengChung for providing [detailed report for this bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5402).
* BUGFIX: [vmalert-tool](./vmalert-tool.md): allow using arbitrary `eval_time`  in [alert_rule_test](./vmalert-tool.md#alert_test_case) case. Previously, test cases with `eval_time`  not being a multiple of `evaluation_interval` would fail.
* BUGFIX: [vmalert](./vmalert.md): sanitize label names before sending the alert notification to Alertmanager. Before, vmalert would send notifications with labels containing characters not supported by Alertmanager validator, resulting into validation errors like `msg="Failed to validate alerts" err="invalid label set: invalid name "foo.bar"`.
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): fix `vmbackupmanager` not deleting previous object versions from S3 when applying retention policy with `-deleteAllObjectVersions` command-line flag.
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): fix panic when ingesting data via [NewRelic protocol](./README.md#how-to-send-data-from-newrelic-agent) into VictoriaMetrics cluster. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5416).
* BUGFIX: properly escape `<` character in responses returned via [`/federate`](./README.md#federation) endpoint. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431).
* BUGFIX: [vmctl](./vmctl.md): check for Error field in response from influx client during migration. Before, only network errors were checked. Thanks to @wozz for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5446).

## [v1.95.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.1)

Released at 2023-11-16

* FEATURE: dashboards: use `version` instead of `short_version` in version change annotation for single/cluster dashboards. The update should reflect version changes even if different flavours of the same release were applied (custom builds).

* BUGFIX: fix a bug, which could result in improper results and/or to `cannot merge series: duplicate series found` error during [range query](./keyConcepts.md#range-query) execution. The issue has been introduced in [v1.95.0](./CHANGELOG.md#v1950). See [this bugreport](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5332) for details.
* BUGFIX: improve deadline detection when using buffered connection for communication between cluster components. Before, due to nature of a buffered connection the deadline could have been exceeded while reading or writing buffered data to connection. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5327).

## [v1.95.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.95.0)

Released at 2023-11-15

**It is recommended upgrading to [v1.95.1](./CHANGELOG.md#v1951) because v1.95.0 contains a bug, which can lead to incorrect query results and to `cannot merge series: duplicate series found` error. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5332) for details.**

**vmalert's cmd-line flag `-datasource.lookback` will be deprecated soon. Please use `-rule.evalDelay` command-line flag instead and see more details on how to use it [here](./vmalert.md#data-delay). The flag `datasource.lookback` will have no effect in the next release and will be removed in the future releases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5155).**

**vmalert's cmd-line flag `-datasource.queryTimeAlignment` was deprecated and will have no effect anymore. It will be completely removed in next releases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5049) and more detailed changes related to vmalert below.**

* SECURITY: upgrade Go builder from Go1.21.1 to Go1.21.4. See [the list of issues addressed in Go1.21.2](https://github.com/golang/go/issues?q=milestone%3AGo1.21.2+label%3ACherryPickApproved), [the list of issues addressed in Go1.21.3](https://github.com/golang/go/issues?q=milestone%3AGo1.21.3+label%3ACherryPickApproved) and [the list of issues addressed in Go1.21.4](https://github.com/golang/go/issues?q=milestone%3AGo1.21.4+label%3ACherryPickApproved).

* FEATURE: `vmselect`: improve performance for repeated [instant queries](./keyConcepts.md#instant-query) if they contain one of the following [rollup functions](./MetricsQL.md#rollup-functions):
  - [`avg_over_time`](./MetricsQL.md#avg_over_time)
  - [`sum_over_time`](./MetricsQL.md#sum_over_time)
  - [`count_eq_over_time`](./MetricsQL.md#count_eq_over_time)
  - [`count_gt_over_time`](./MetricsQL.md#count_gt_over_time)
  - [`count_le_over_time`](./MetricsQL.md#count_le_over_time)
  - [`count_ne_over_time`](./MetricsQL.md#count_ne_over_time)
  - [`count_over_time`](./MetricsQL.md#count_over_time)
  - [`increase`](./MetricsQL.md#increase)
  - [`max_over_time`](./MetricsQL.md#max_over_time)
  - [`min_over_time`](./MetricsQL.md#min_over_time)
  - [`rate`](./MetricsQL.md#rate)
  
  The optimization is enabled when these functions contain lookbehind window in square brackets bigger or equal to `6h` (the threshold can be changed via `-search.minWindowForInstantRollupOptimization` command-line flag). The optimization improves performance for SLO/SLI-like queries such as `avg_over_time(up[30d])` or `sum(rate(http_request_errors_total[3d])) / sum(rate(http_requests_total[3d]))`, which can be generated by [sloth](https://github.com/slok/sloth) or similar projects.
* FEATURE: `vmselect`: improve query performance on systems with big number of CPU cores (`>=32`). Add `-search.maxWorkersPerQuery` command-line flag, which can be used for fine-tuning query performance on systems with big number of CPU cores. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5195).
* FEATURE: `vmselect`: expose `vm_memory_intensive_queries_total` counter metric which gets increased each time `-search.logQueryMemoryUsage` memory limit is exceeded by a query. This metric should help to identify expensive and heavy queries without inspecting the logs.
* FEATURE: [MetricsQL](./MetricsQL.md): add [drop_empty_series()](./MetricsQL.md#drop_empty_series) function, which can be used for filtering out empty series before performing additional calculations as shown in [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5071).
* FEATURE: [MetricsQL](./MetricsQL.md): add [labels_equal()](./MetricsQL.md#labels_equal) function, which can be used for searching series with identical values for the given labels. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5148).
* FEATURE: [MetricsQL](./MetricsQL.md): add [`outlier_iqr_over_time(m[d])`](./MetricsQL.md#outlier_iqr_over_time) and [`outliers_iqr(q)`](./MetricsQL.md#outliers_iqr) functions, which allow detecting anomalies in samples and series using [Interquartile range method](https://en.wikipedia.org/wiki/Interquartile_range).
* FEATURE: [vmalert](./vmalert.md): add `eval_alignment` attribute for [Groups](./vmalert.md#groups), it will align group query requests timestamp with interval like `datasource.queryTimeAlignment` did.
  This also means that `datasource.queryTimeAlignment` command-line flag becomes deprecated now and will have no effect if configured. If `datasource.queryTimeAlignment` was set to `false` before, then `eval_alignment` has to be set to `false` explicitly under group.
  See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5049).
* FEATURE: [vmalert](./vmalert.md): add `-rule.evalDelay` flag and `eval_delay` attribute for [Groups](./vmalert.md#groups). The new flag and param can be used to adjust the `time` parameter for rule evaluation requests to match [intentional query delay](./keyConcepts.md#query-latency) from the datasource. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5155).
* FEATURE: [vmalert](./vmalert.md): allow specifying full url in notifier static_configs target address, like `http://alertmanager:9093/test/api/v2/alerts`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5184).
* FEATURE: [vmalert](./vmalert.md): reduce the number of queries for restoring alerts state on start-up. The change should speed up the restore process and reduce pressure on `remoteRead.url`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5265).
* FEATURE: [vmalert](./vmalert.md): add label `file` pointing to the group's filename to metrics `vmalert_recording_.*` and `vmalert_alerts_.*`. The filename should help identifying alerting rules belonging to specific groups with identical names but different filenames. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5267).
* FEATURE: [vmalert](./vmalert.md): automatically retry remote-write requests on closed connections. The change should reduce the amount of logs produced in environments with short-living connections or environments without support of keep-alive on network balancers.
* FEATURE: [vmagent](./vmagent.md): support data ingestion from [NewRelic infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent). See [these docs](./Single-Server-VictoriaMetrics.md#how-to-send-data-from-newrelic-agent), [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3520) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4712).
* FEATURE: [vmagent](./vmagent.md): add `-remoteWrite.shardByURL.labels` command-line flag, which can be used for specifying a list of labels for sharding outgoing samples among the configured `-remoteWrite.url` destinations if `-remoteWrite.shardByURL` command-line flag is set. See [these docs](./vmagent.md#sharding-among-remote-storages) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4942) for details.
* FEATURE: [vmagent](./vmagent.md): do not exit on startup when [scrape_configs](./sd_configs.md#scrape_configs) refer to non-existing or invalid files with auth configs, since these files may appear / updated later. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4959) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5153).
* FEATURE: [vmagent](./vmagent.md): allow loading TLS certificates from HTTP and HTTPS urls by specifying these urls at `cert_file` and `key_file` options inside `tls_config` and `proxy_tls_config` sections at [http client settings](./sd_configs.md#http-api-client-options).
* FEATURE: [vmagent](./vmagent.md): reduce CPU load when big number of targets are scraped over HTTPS with the same custom TLS certificate configured via `tls_config->cert_file` and `tls_config->key_file` at [scrape_config](./sd_configs.md#scrape_configs).
* FEATURE: [vmbackup](./vmbackup.md): add `-filestream.disableFadvise` command-line flag, which can be used for disabling `fadvise` syscall during backup upload to the remote storage. By default `vmbackup` uses `fadvise` syscall in order to prevent from eviction of recently accessed data from the [OS page cache](https://en.wikipedia.org/wiki/Page_cache) when backing up large files. Sometimes the `fadvise` syscall may take significant amounts of CPU when the backup is performed with large value of `-concurrency` command-line flag on systems with big number of CPU cores. In this case it is better to manually disable `fadvise` syscall by passing `-filestream.disableFadvise` command-line flag to `vmbackup`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5120) for details.
* FEATURE: [vmbackup](./vmbackup.md): add `-deleteAllObjectVersions` command-line flag, which can be used for forcing removal of all object versions in remote object storage. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5121) issue and [these docs](./vmbackup.md#permanent-deletion-of-objects-in-s3-compatible-storages) for the details.
* FEATURE: [Alerting rules for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts): account for `vmauth` component for alerts `ServiceDown` and `TooManyRestarts`.
* FEATURE: [Alerting rules for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts): make `TooHighMemoryUsage` more tolerable to spikes or near-the-threshold states. The change should reduce number of false positives.
* FEATURE: [Alerting rules for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts): add `TooManyMissedIterations` alerting rule for vmalert to detect groups that miss their evaluations due to slow queries.
* FEATURE: [vmui](./README.md#vmui): add support for functions, labels, values in autocomplete. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3006).
* FEATURE: [vmui](./README.md#vmui): retain specified time interval when executing a query from `Top Queries`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5097).
* FEATURE: [vmui](./README.md#vmui): improve repeated VMUI page load times by enabling caching of static js and css at web browser side according to [these recommendations](https://developer.chrome.com/docs/lighthouse/performance/uses-long-cache-ttl/).
* FEATURE: [vmui](./README.md#vmui): sort legend under the graph in descending order of median values. This should simplify graph analysis, since usually the most important lines have bigger values.
* FEATURE: [vmui](./README.md#vmui): reduce vertical space usage, so more information is visible on the screen without scrolling.
* FEATURE: [vmui](./README.md#vmui): show query execution duration in the header of query input field. This should help optimizing query performance.
* FEATURE: support `Strict-Transport-Security`, `Content-Security-Policy` and `X-Frame-Options` HTTP response headers in the all VictoriaMetrics components. The values for headers can be specified via the following command-line flags: `-http.header.hsts`, `-http.header.csp` and `-http.header.frameOptions`.
* FEATURE: [vmalert-tool](./vmalert-tool.md): add `unittest` command to run unittest for alerting and recording rules. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4789) for details.
* FEATURE: dashboards/vmalert: add new panel `Missed evaluations` for indicating alerting groups that miss their evaluations.
* FEATURE: all: track requests with wrong auth key and wrong basic auth at `vm_http_request_errors_total` [metric](./README.md#monitoring) with `reason="wrong_auth_key"` and `reason="wrong_basic_auth"`. See [this issue](https://github.com/victoriaMetrics/victoriaMetrics/issues/4590). Thanks to @venkatbvc for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5166).
* FEATURE: [vmauth](./vmauth.md): add ability to drop the specified number of `/`-delimited prefix parts from the request path before proxying the request to the matching backend. See [these docs](./vmauth.md#dropping-request-path-prefix).
* FEATURE: [vmauth](./vmauth.md): add ability to skip TLS verification and to specify TLS Root CA when connecting to backends. See [these docs](./vmauth.md#backend-tls-setup) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5240).
* FEATURE: `vmstorage`: gradually close `vminsert` connections during 25 seconds at [graceful shutdown](./Cluster-VictoriaMetrics.md#updating--reconfiguring-cluster-nodes). This should reduce data ingestion slowdown during rolling restarts. The duration for gradual closing of `vminsert` connections can be configured via `-storage.vminsertConnsShutdownDuration` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4922) and [these docs](./Cluster-VictoriaMetrics.md#improving-re-routing-performance-during-restart) for details.
* FEATURE: `vmstorage`: add `-blockcache.missesBeforeCaching` command-line flag, which can be used for fine-tuning RAM usage for `indexdb/dataBlocks` cache when queries touching big number of time series are executed.
* FEATURE: add `-loggerMaxArgLen` command-line flag for fine-tuning the maximum lengths of logged args.

* BUGFIX: [vmalert](./vmalert.md): strip sensitive information such as auth headers or passwords from datasource, remote-read, remote-write or notifier URLs in log messages or UI. This behavior is by default and is controlled via `-datasource.showURL`, `-remoteRead.showURL`, `remoteWrite.showURL` or `-notifier.showURL` cmd-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5044).
* BUGFIX: [vmalert](./vmalert.md): fix vmalert web UI when running on 32-bit architectures machine.
* BUGFIX: [vmalert](./vmalert.md): do not send requests to configured remote systems when `-datasource.*`, `-remoteWrite.*`, `-remoteRead.*` or `-notifier.*` command-line flags refer files with invalid auth configs. Previously such requests were sent without properly set auth headers. Now the requests are sent only after the files are updated with valid auth configs. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5153).
* BUGFIX: [vmalert](./vmalert.md): properly maintain alerts state in [replay mode](./vmalert.md#rules-backfilling) if alert's `for` param was bigger than replay request range (usually a couple of hours). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5186) for details.
* BUGFIX: [vmalert](./vmalert.md): increment `vmalert_remotewrite_errors_total` metric if all retries to send remote-write request failed. Before, this metric was incremented only if remote-write client's buffer is overloaded.
* BUGFIX: [vmalert](./vmalert.md): increment `vmalert_remotewrite_dropped_rows_total` metric if remote-write client's buffer is overloaded. Before, these metrics were incremented only after unsuccessful HTTP calls.
* BUGFIX: `vmselect`: improve performance and memory usage during query processing on machines with big number of CPU cores. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5087).
* BUGFIX: dashboards: fix vminsert/vmstorage/vmselect metrics filtering when dashboard is used to display data from many sub-clusters with unique job names. Before, only one specific job could have been accounted for component-specific panels, instead of all available jobs for the component.
* BUGFIX: dashboards: respect `job` and `instance` filters for `alerts` annotation in cluster and single-node dashboards.
* BUGFIX: dashboards: update description for RSS and anonymous memory panels to be consistent for single-node, cluster and vmagent dashboards.
* BUGFIX: dashboards/vmalert: apply `desc` sorting in tooltips for vmalert dashboard in order to improve visibility of the outliers on graph.
* BUGFIX: dashboards/vmalert: properly apply time series filter for panel `No data errors`. Before, the panel didn't respect `job` or `instance` filters.
* BUGFIX: dashboards/vmalert: fix panel `Errors rate to Alertmanager` not showing any data due to wrong label filters.
* BUGFIX: dashboards/cluster: fix description about `max` threshold for `Concurrent selects` panel. Before, it was mistakenly implying that `max` is equal to the double of available CPUs.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): bump hard-coded limit for search query size at `vmstorage` from 1MB to 5MB. The change should be more suitable for real-world scenarios and protect vmstorage from excessive memory usage. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5154) for details
* BUGFIX: [vmbackup](./vmbackup.md): fix error when creating an incremental backup with the `-origin` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5144) for details.
* BUGFIX: [vmagent](./vmagent.md): properly apply [relabeling](./vmagent.md#relabeling) with `regex`, which start and end with `.+` or `.*` and which contain alternate sub-regexps. For example, `.+;|;.+` or `.*foo|bar|baz.*`. Previously such regexps were improperly parsed, which could result in unexpected relabeling results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5297).
* BUGFIX: [vmagent](./vmagent.md): properly discover Kubernetes targets via [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs). Previously some targets and some labels could be skipped during service discovery because of the bug introduced in [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4850). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5216) for more details.
* BUGFIX: [vmagent](./vmagent.md): fix vmagent ignoring configuration reload for streaming aggregation if it was started with empty streaming aggregation config. Thanks to @aluode99 for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5178).
* BUGFIX: [vmagent](./vmagent.md): do not scrape targets if the corresponding [scrape_configs](./sd_configs.md#scrape_configs) refer to files with invalid auth configs. Previously the targets were scraped without properly set auth headers in this case. Now targets are scraped only after the files are updated with valid auth configs. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5153).
* BUGFIX: [vmagent](./vmagent.md): properly parse `ca`, `cert` and `key` options at `tls_config` section inside [http client settings](./sd_configs.md#http-api-client-options). Previously string values couldn't be parsed for these options, since the parser was mistakenly expecting a list of `uint8` values instead.
* BUGFIX: [vmagent](./vmagent.md): properly drop samples if `-streamAggr.dropInput` command-line flag is set and `-remoteWrite.streamAggr.config` contains an empty file. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5207).
* BUGFIX: [vmagent](./vmagent.md): do not print redundant error logs when failed to scrape consul or nomad target. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5239).
* BUGFIX: [vmagent](./vmagent.md): generate proper link to the main page and to `favicon.ico` at http pages served by `vmagent` such as `/targets` or `/service-discovery` when `vmagent` sits behind an http proxy with custom http path prefixes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5306).
* BUGFIX: [vmagent](./vmagent.md): properly decode Snappy-encoded data blocks received via [VictoriaMetrics remote_write protocol](./vmagent.md#victoriametrics-remote-write-protocol). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md): prevent deleted series to be searchable via `/api/v1/series` API if they were re-ingested with staleness markers. This situation could happen if user deletes the series from the target and from VM, and then vmagent sends stale markers for absent series. Thanks to @ilyatrefilov for the [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5069) and [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5174).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md): log warning about switching to ReadOnly mode only on state change. Before, vmstorage would log this warning every 1s. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5159) for details.
* BUGFIX: [vmauth](./vmauth.md): show browser authorization window for unauthorized requests to unsupported paths if the `unauthorized_user` section is specified. This allows properly authorizing the user. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5236) for details.
* BUGFIX: [vmauth](./vmauth.md): properly proxy requests to HTTP/2.0 backends and properly pass `Host` header to backends.
* BUGFIX: [vmui](./README.md#vmui): fix the `Disable cache` toggle at `JSON` and `Table` views. Previously response caching was always enabled and couldn't be disabled at these views.
* BUGFIX: [vmui](./README.md#vmui): correctly display query errors on [Explore Prometheus Metrics](./README.md#metrics-explorer) page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5202) for details.
* BUGFIX: [vmui](./README.md#vmui): properly handle trailing slash in the server URL. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5203).
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): correctly print error in logs when copying backup fails. Previously, error was displayed in metrics but was missing in logs.
* BUGFIX: fix panic, which could occur when [query tracing](./README.md#query-tracing) is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5319).

## [v1.94.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.94.0)

Released at 2023-10-02

* FEATURE: [MetricsQL](./MetricsQL.md): add support for numbers with underscore delimiters such as `1_234_567_890` and `1.234_567_890`. These numbers are easier to read than `1234567890` and `1.234567890`.
* FEATURE: [vmbackup](./vmbackup.md): add support for server-side copy of existing backups. See [these docs](./vmbackup.md#server-side-copy-of-the-existing-backup) for details.
* FEATURE: [vmui](./README.md#vmui): add the option to see the latest 25 queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4718).
* FEATURE: [vmagent](./vmagent.md): add ability to set `member num` label for all the metrics scraped by a particular `vmagent` instance in [a cluster of vmagents](./vmagent.md#scraping-big-number-of-targets) via `-promscrape.cluster.memberLabel` command-line flag. See [these docs](./vmagent.md#scraping-big-number-of-targets) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4247).
* FEATURE: [vmagent](./vmagent.md): do not log `unexpected EOF` when reading incoming metrics, since this error is expected and is handled during metrics' parsing. This reduces the amounts of noisy logs. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4817).
* FEATURE: [vmagent](./vmagent.md): retry failed write request on the closed connection immediately, without waiting for backoff. This should improve data delivery speed and reduce amount of error logs emitted by vmagent when using idle connections. See related [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4139).
* FEATURE: [vmagent](./vmagent.md): reduces load on Kubernetes control plane during initial service discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4855) for details.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): reduce the maximum recovery time at `vmselect` and `vminsert` when some of `vmstorage` nodes become unavailable because of networking issues from 60 seconds to 3 seconds by default. The recovery time can be tuned at `vmselect` and `vminsert` nodes with `-vmstorageUserTimeout` command-line flag if needed. Thanks to @wjordan for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4423).
* FEATURE: [vmui](./README.md#vmui): add Prometheus data support to the "Explore cardinality" page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4320) for details.
* FEATURE: [vmui](./README.md#vmui): make the warning message more noticeable for text fields. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4848).
* FEATURE: [vmui](./README.md#vmui): add button for auto-formatting PromQL/MetricsQL queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4681). Thanks to @aramattamara for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4694).
* FEATURE: [vmui](./README.md#vmui): improve accessibility score to 100 according to [Google's Lighthouse](https://developer.chrome.com/docs/lighthouse/accessibility/) tests.
* FEATURE: [vmui](./README.md#vmui): organize `min`, `max`, `median` values on the chart legend and tooltips for better visibility.
* FEATURE: [vmui](./README.md#vmui): add explanation about [cardinality explorer](./README.md#cardinality-explorer) statistic inaccuracy in VictoriaMetrics cluster. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3070).
* FEATURE: [vmui](./README.md#vmui): add storage of query history in `localStorage`. See [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5022).
* FEATURE: dashboards: provide copies of Grafana dashboards alternated with VictoriaMetrics datasource at [dashboards/vm](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/dashboards/vm).
* FEATURE: [vmauth](./vmauth.md): added ability to set, override and clear request and response headers on a per-user and per-path basis. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4825) and [these docs](./vmauth.md#auth-config) for details.
* FEATURE: [vmauth](./vmauth.md): add ability to retry requests to the [remaining backends](./vmauth.md#load-balancing) if they return response status codes specified in the `retry_status_codes` list. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4893).
* FEATURE: [vmauth](./vmauth.md): expose metrics `vmauth_config_last_reload_*` for tracking the state of config reloads, similarly to vmagent/vmalert components.
* FEATURE: [vmauth](./vmauth.md): do not print logs like `SIGHUP received...` once per configured `-configCheckInterval` cmd-line flag. This log will be printed only if config reload was invoked manually. 
* FEATURE: [vmalert](./vmalert.md): add `eval_offset` attribute for [Groups](./vmalert.md#groups). If specified, Group will be evaluated at the exact time offset on the range of [0...evaluationInterval]. The setting might be useful for cron-like rules which must be evaluated at specific moments of time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3409) for details.
* FEATURE: [vmalert](./vmalert.md): validate [MetricsQL](./MetricsQL.md) function names in alerting and recording rules when `vmalert` runs with `-dryRun` command-line flag. Previously it was allowed to use unknown (aka invalid) MetricsQL function names there. For example, `foo()` was counted as a valid query. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4933).
* FEATURE: limit the length of string params in log messages to 500 chars. Longer string params are replaced with the `first_250_chars..last_250_chars`. This prevents from too long log lines, which can be emitted by VictoriaMetrics components.
* FEATURE: [docker compose environment](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker): add `vmauth` component to cluster's docker-compose example for balancing load among multiple `vmselect` components.
* FEATURE: [MetricsQL](./MetricsQL.md): make sure that `q2` series are returned after `q1` series in the results of `q1 or q2` query, in the same way as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4763).
* FEATURE: [MetricsQL](./MetricsQL.md): return empty result from [`bitmap_and(a, b)`](./MetricsQL.md#bitmap_and), [`bitmap_or(a, b)`](./MetricsQL.md#bitmap_or) and [`bitmap_xor(a, b)`](./MetricsQL.md#bitmap_xor) if `a` or `b` have no value at the particular timestamp. Previously `0` was returned in this case. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4996).
* FEATURE: stop exposing `vm_merge_need_free_disk_space` metric, since it has been appeared that it confuses users while doesn't bring any useful information. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/686#issuecomment-1733844128).

* BUGFIX: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): fix display of ingested rows rate for `Samples ingested/s` and `Samples rate` panels for vmagent's dashboard. Previously, not all ingested protocols were accounted in these panels. An extra panel `Rows rate` was added to `Ingestion` section to display the split for rows ingested rate by protocol.
* BUGFIX: [vmui](./README.md#vmui): fix the bug causing render looping when switching to heatmap.
* BUGFIX: [VictoriaMetrics enterprise](./enterprise.md) validate `-dedup.minScrapeInterval` value and `-downsampling.period` intervals are multiples of each other. See [these docs](./README.md#downsampling).
* BUGFIX: [vmbackup](./vmbackup.md): properly copy `appliedRetention.txt` files inside `<-storageDataPath>/{data}` folders during [incremental backups](./vmbackup.md#incremental-backups). Previously the new `appliedRetention.txt` could be skipped during incremental backups, which could lead to increased load on storage after restoring from backup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5005).
* BUGFIX: [vmagent](./vmagent.md): suppress `context canceled` error messages in logs when `vmagent` is reloading service discovery config. This error could appear starting from [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5). See [this PR](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5048). 
* BUGFIX: [vmagent](./vmagent.md): remove concurrency limit during parsing of scraped metrics, which was mistakenly applied to it. With this change cmd-line flag `-maxConcurrentInserts` won't have effect on scraping anymore.
* BUGFIX: [MetricsQL](./MetricsQL.md): allow passing [median_over_time](./MetricsQL.md#median_over_time) to [aggr_over_time](./MetricsQL.md#aggr_over_time). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5034).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): fix ingestion via [multitenant url](./Cluster-VictoriaMetrics.md#multitenancy-via-labels) for opentsdbhttp. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5061). The bug has been introduced in [v1.93.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.2).
* BUGFIX: [vmagent](./vmagent.md): fix support of legacy DataDog agent, which adds trailing slashes to urls. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5078). Thanks to @maxb for spotting the issue.

## [v1.93.9](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.9)

Released at 2023-12-10

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* SECURITY: upgrade base docker image (Alpine) from 3.18.4 to 3.19.0. See [alpine 3.19.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.19.0-released.html).
* SECURITY: upgrade Go builder from Go1.21.4 to Go1.21.5. See [the list of issues addressed in Go1.21.5](https://github.com/golang/go/issues?q=milestone%3AGo1.21.5+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): prevent from `FATAL: cannot flush metainfo` panic when [`-remoteWrite.multitenantURL`](./vmagent.md#multitenancy) command-line flag is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5357).
* BUGFIX: [vmagent](./vmagent.md): properly decode zstd-encoded data blocks received via [VictoriaMetrics remote_write protocol](./vmagent.md#victoriametrics-remote-write-protocol). See [this issue comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301#issuecomment-1815871992).
* BUGFIX: [vmagent](./vmagent.md): properly add new labels at `output_relabel_configs` during [stream aggregation](./stream-aggregation.md). Previously this could lead to corrupted labels in output samples. Thanks to @ChengChung for providing [detailed report for this bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5402).
* BUGFIX: [vmalert](./vmalert.md): sanitize label names before sending the alert notification to Alertmanager. Before, vmalert would send notifications with labels containing characters not supported by Alertmanager validator, resulting into validation errors like `msg="Failed to validate alerts" err="invalid label set: invalid name "foo.bar"`.
* BUGFIX: properly escape `<` character in responses returned via [`/federate`](./README.md#federation) endpoint. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431).

## [v1.93.8](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.8)

Released at 2023-11-15

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* SECURITY: upgrade Go builder from Go1.21.3 to Go1.21.4. See [the list of issues addressed in Go1.21.4](https://github.com/golang/go/issues?q=milestone%3AGo1.21.4+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): properly apply [relabeling](./vmagent.md#relabeling) with `regex`, which start and end with `.+` or `.*` and which contain alternate sub-regexps. For example, `.+;|;.+` or `.*foo|bar|baz.*`. Previously such regexps were improperly parsed, which could result in unexpected relabeling results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5297).
* BUGFIX: [vmagent](./vmagent.md): properly decode Snappy-encoded data blocks received via [VictoriaMetrics remote_write protocol](./vmagent.md#victoriametrics-remote-write-protocol). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301).
* BUGFIX: fix panic, which could occur when [query tracing](./README.md#query-tracing) is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5319).

## [v1.93.7](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.7)

Released at 2023-11-02

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* BUGFIX: [vmagent](./vmagent.md): properly discover Kubernetes targets via [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs). Previously some targets and some labels could be skipped during service discovery because of the bug introduced in [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4850). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5216) for more details.
* BUGFIX: [vmagent](./vmagent.md): properly parse `ca`, `cert` and `key` options at `tls_config` section inside [http client settings](./sd_configs.md#http-api-client-options). Previously string values couldn't be parsed for these options, since the parser was mistakenly expecting a list of `uint8` values instead.
* BUGFIX: [vmagent](./vmagent.md): properly drop samples if `-streamAggr.dropInput` command-line flag is set and `-remoteWrite.streamAggr.config` contains an empty file. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5207).
* BUGFIX: [vmagent](./vmagent.md): do not print redundant error logs when failed to scrape consul or nomad target. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5239).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md): prevent deleted series to be searchable via `/api/v1/series` API if they were re-ingested with staleness markers. This situation could happen if user deletes the series from the target and from VM, and then vmagent sends stale markers for absent series. Thanks to @ilyatrefilov for the [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5069) and [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5174).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md): log warning about switching to ReadOnly mode only on state change. Before, vmstorage would log this warning every 1s. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5159) for details.
* BUGFIX: [vmauth](./vmauth.md): show browser authorization window for unauthorized requests to unsupported paths if the `unauthorized_user` section is specified. This allows properly authorizing the user. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5236) for details.

## [v1.93.6](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.6)

Released at 2023-10-16

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* SECURITY: upgrade Go builder from Go1.21.1 to Go1.21.3. See [the list of issues addressed in Go1.21.2](https://github.com/golang/go/issues?q=milestone%3AGo1.21.2+label%3ACherryPickApproved) and [the list of issues addressed in Go1.21.3](https://github.com/golang/go/issues?q=milestone%3AGo1.21.3+label%3ACherryPickApproved).

* BUGFIX: [vmalert](./vmalert.md): strip sensitive information such as auth headers or passwords from datasource, remote-read, remote-write or notifier URLs in log messages or UI. This behavior is by default and is controlled via `-datasource.showURL`, `-remoteRead.showURL`, `remoteWrite.showURL` or `-notifier.showURL` cmd-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5044).
* BUGFIX: `vmselect`: improve performance and memory usage during query processing on machines with big number of CPU cores. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5087) for details.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): bump hard-coded limit for search query size at `vmstorage` from 1MB to 5MB. The change should be more suitable for real-world scenarios and protect vmstorage from excessive memory usage. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5154) for details
* BUGFIX: [vmagent](./vmagent.md): fix vmagent ignoring configuration reload for streaming aggregation if it was started with empty streaming aggregation config. Thanks to @aluode99 for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5178).
* BUGFIX: [vmbackup](./vmbackup.md): properly copy `appliedRetention.txt` files inside `<-storageDataPath>/{data}` folders during [incremental backups](./vmbackup.md#incremental-backups). Previously the new `appliedRetention.txt` could be skipped during incremental backups, which could lead to increased load on storage after restoring from backup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5005).
* BUGFIX: [vmagent](./vmagent.md): suppress `context canceled` error messages in logs when `vmagent` is reloading service discovery config. This error could appear starting from [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5). See [this PR](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5048).
* BUGFIX: [MetricsQL](./MetricsQL.md): allow passing [median_over_time](./MetricsQL.md#median_over_time) to [aggr_over_time](./MetricsQL.md#aggr_over_time). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5034).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): fix ingestion via [multitenant url](./Cluster-VictoriaMetrics.md#multitenancy-via-labels) for opentsdbhttp. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5061). The bug has been introduced in [v1.93.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.2).
* BUGFIX: [vmagent](./vmagent.md): fix support of legacy DataDog agent, which adds trailing slashes to urls. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5078). Thanks to @maxb for spotting the issue.

## [v1.93.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.5)

Released at 2023-09-19

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* BUGFIX: [storage](./Single-Server-VictoriaMetrics.md): prevent from livelock when [forced merge](./README.md#forced-merge) is called under high data ingestion. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4987).
* BUGFIX: [Graphite Render API](./README.md#graphite-render-api-usage): correctly return `null` instead of `Inf` in JSON query responses. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3783).
* BUGFIX: [vmbackup](./vmbackup.md): properly copy `parts.json` files inside `<-storageDataPath>/{data,indexdb}` folders during [incremental backups](./vmbackup.md#incremental-backups). Previously the new `parts.json` could be skipped during incremental backups, which could lead to inability to restore from the backup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5005). This issue has been introduced in [v1.90.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.90.0).
* BUGFIX: [vmagent](./vmagent.md): properly close connections to Kubernetes API server after the change in `selectors` or `namespaces` sections of [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs). Previously `vmagent` could continue polling Kubernetes API server with the old `selectors` or `namespaces` configs additionally to polling new configs. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4850).
* BUGFIX: [vmauth](./vmauth.md): prevent configuration reloading if there were no changes in config. This improves memory usage when `-configCheckInterval` cmd-line flag is configured and config has extensive list of regexp expressions requiring additional memory on parsing. 

## [v1.93.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.4)

Released at 2023-09-10

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* SECURITY: upgrade Go builder from Go1.21.0 to Go1.21.1. See [the list of issues addressed in Go1.20.6](https://github.com/golang/go/issues?q=milestone%3AGo1.21.1+label%3ACherryPickApproved).

* BUGFIX: [vminsert enterprise](./enterprise.md): properly parse `/insert/multitenant/*` urls, which have been broken since [v1.93.2](#v1932). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4947).
* BUGFIX: properly build production armv5 binaries for `GOARCH=arm`. This has been broken after the upgrading of Go builder to Go1.21.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4965).
* BUGFIX: [vmselect](./Cluster-VictoriaMetrics.md): return `503 Service Unavailable` status code when [partial responses](./Cluster-VictoriaMetrics.md#cluster-availability) are denied and some of `vmstorage` nodes are temporarily unavailable. Previously `422 Unprocessable Entity` status code was mistakenly returned in this case, which could prevent from automatic recovery by re-sending the request to healthy cluster replica in another availability zone.
* BUGFIX: [vmalert](./vmalert.md): fix the bug when Group's `params` fields with multiple values were overriding each other instead of adding up. The bug was introduced in [this commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/eccecdf177115297fa1dc4d42d38e23de9a9f2cb) starting from [v1.91.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.91.1). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4908).
* BUGFIX: [vmagent](./vmagent.md): fix possible corruption of labels in the collected samples if `-remoteWrite.label` is set together with multiple `-remoteWrite.url` options. The bug has been introduced in [v1.93.1](#v1931). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4972).

## [v1.93.3](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.3)

Released at 2023-09-02

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): properly close broken vmstorage connection during [read-only state](./Cluster-VictoriaMetrics.md#readonly-mode) checks at `vmstorage`. Previously it wasn't properly closed, which prevents restoring `vmstorage` node from read-only mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md):  prevent from breaking `vmselect` -> `vmstorage` RPC communication when `vmstorage` returns an empty label name at `/api/v1/labels` request. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4932).

## [v1.93.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.2)

Released at 2023-09-01

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* BUGFIX: [build](./README.md): fix Docker builds for old Docker releases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4907).
* BUGFIX: [vmagent](./vmagent.md): consistently set `User-Agent` header to `vm_promscrape` during scraping with enabled or disabled [stream parsing mode](./vmagent.md#stream-parsing-mode). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4884).
* BUGFIX: [vmagent](./vmagent.md): consistently set timeout for scraping with enabled or disabled [stream parsing mode](./vmagent.md#stream-parsing-mode). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4847).
* BUGFIX: [vmalert](./vmalert.md): correctly re-use HTTP request object on `EOF` retries when querying the configured datasource. Previously, there was a small chance that query retry wouldn't succeed.
* BUGFIX: [vmselect](./Cluster-VictoriaMetrics.md): correctly handle requests with `/select/multitenant` prefix. Such requests must be rejected since vmselect does not support multitenancy endpoint. Previously, such requests were causing panic. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4910).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): properly check for [read-only state](./Cluster-VictoriaMetrics.md#readonly-mode) at `vmstorage`. Previously it wasn't properly checked, which could lead to increased resource usage and data ingestion slowdown when some of `vmstorage` nodes are in read-only mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870).

## [v1.93.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.1)

Released at 2023-08-23

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

* BUGFIX: prevent from possible data loss during `indexdb` rotation. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4873) for details.
* BUGFIX: do not allow starting VictoriaMetrics components with improperly set boolean command-line flags in the form `-boolFlagName value`, since this leads to silent incomplete flags' parsing. This form should be replaced with `-boolFlagName=value`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4845).
* BUGFIX: [vmagent](./vmagent.md): properly set labels from `-remoteWrite.label` command-line flag just before sending samples to the configured `-remoteWrite.url` according to [these docs](./vmagent.md#adding-labels-to-metrics). Previously these labels were incorrectly set before [the relabeling](./vmagent.md#relabeling) configured via `-remoteWrite.urlRelabelConfigs` and [the stream aggregation](./stream-aggregation.md) configured via `-remoteWrite.streamAggr.config`, so these labels could be lost or incorrectly transformed before sending the samples to remote storage. The fix allows using `-remoteWrite.label` for identifying `vmagent` instances in [cluster mode](./vmagent.md#scraping-big-number-of-targets). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4247) and [these docs](./stream-aggregation.md#cluster-mode) for more details.
* BUGFIX: remove `DEBUG` logging when parsing `if` filters inside [relabeling rules](./vmagent.md#relabeling-enhancements) and when parsing `match` filters inside [stream aggregation rules](./stream-aggregation.md).
* BUGFIX: properly replace `:` chars in label names with `_` when `-usePromCompatibleNaming` command-line flag is passed to `vmagent`, `vminsert` or single-node VictoriaMetrics. This addresses [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3113#issuecomment-1275077071).
* BUGFIX: [vmbackup](./vmbackup.md): correctly check if specified `-dst` belongs to specified `-storageDataPath`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4837).
* BUGFIX: [vmctl](./vmctl.md): don't interrupt the migration process if no metrics were found for a specific tenant. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4796).

## [v1.93.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.93.0)

Released at 2023-08-12

**It is recommended upgrading to [VictoriaMetrics v1.93.1](./CHANGELOG.md#v1931) because v1.93.0 contains a bug, which can lead to data loss because of incorrect `indexdb` rotation. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4873) for details.**

**v1.93.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.93.x line will be supported for at least 12 months since [v1.93.0](./CHANGELOG.md#v1930) release**

**Update note**: starting from this release, [vmagent](./vmagent.md) ignores timestamps provided by scrape targets by default - it associates scraped metrics with local timestamps instead. Set `honor_timestamps: true` in [scrape configs](./sd_configs.md#scrape_configs) if timestamps provided by scrape targets must be used instead. This change helps removing gaps for metrics collected from [cadvisor](https://github.com/google/cadvisor) such as `container_memory_usage_bytes`. This also improves data compression and query performance over metrics collected from `cadvisor`. See more details [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697).

* SECURITY: upgrade Go builder from Go1.20.6 to Go1.21.0 in order to fix [this issue](https://github.com/golang/go/issues/61460).
* SECURITY: upgrade base docker image (Alpine) from 3.18.2 to 3.18.3. See [alpine 3.18.3 release notes](https://alpinelinux.org/posts/Alpine-3.15.10-3.16.7-3.17.5-3.18.3-released.html).

* FEATURE: [MetricsQL](./MetricsQL.md): add `share_eq_over_time(m[d], eq)` function for calculating the share (in the range `[0...1]`) of raw samples on the given lookbehind window `d`, which are equal to `eq`. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4441). Thanks to @Damon07 for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4725).
* FEATURE: [vmauth](./vmauth.md): allow configuring deadline for a backend to be excluded from the rotation on errors via `-failTimeout` cmd-line flag. This feature could be useful when it is expected for backends to be not available for significant periods of time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4415) for details. Thanks to @SunKyu for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4416).  
* FEATURE: [vmalert](./vmalert.md): remove deprecated in [v1.61.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.61.0) `-rule.configCheckInterval` command-line flag. Use `-configCheckInterval` command-line flag instead.
* FEATURE: [vmalert](./vmalert.md): remove support of deprecated web links of `/api/v1/<groupID>/<alertID>/status` form in favour of `/api/v1/alerts?group_id=<>&alert_id=<>` links. Links of `/api/v1/<groupID>/<alertID>/status` form were deprecated in v1.79.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2825) for details.
* FEATURE: [vmctl](./vmctl.md): allow disabling binary export API protocol via `-vm-native-disable-binary-protocol` cmd-line flag when [migrating data from VictoriaMetrics](./vmctl.md#migrating-data-from-victoriametrics). Disabling binary protocol can be useful for deduplication of the exported data before ingestion. For this, deduplication need [to be configured](./README.md#deduplication) at `-vm-native-src-addr` side and `-vm-native-disable-binary-protocol` should be set on vmctl side.
* FEATURE: [vmctl](./vmctl.md): add support of `week` step for [time-based chunking migration](./vmctl.md#using-time-based-chunking-of-migration). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4738).
* FEATURE: [vmctl](./vmctl.md): allow specifying custom full url at `--remote-read-src-addr` command-line flag if `--remote-read-disable-path-append` command-line flag is set. This allows importing data from urls, which do not end with `/api/v1/read`. For example, from Promscale. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4655).
* FEATURE: [vmui](./README.md#vmui): add warning in query field of vmui for partial data responses. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4721).
* FEATURE: [vmui](./README.md#vmui): allow displaying the full error message on click for trimmed error messages in vmui. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4719).
* FEATURE: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): add `Concurrent inserts` panel to vmagent's dashboard. The new panel supposed to show whether the number of concurrent inserts processed by vmagent isn't reaching the limit.
* FEATURE: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): add panels for absolute Mem and CPU usage by vmalert. See related issue [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4627).
* FEATURE: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): correctly calculate `Bytes per point` value for single-server and cluster VM dashboards. Before, the calculation mistakenly accounted for the number of entries in indexdb in denominator, which could have shown lower values than expected.
* FEATURE: [Alerting rules for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts): `ConcurrentFlushesHitTheLimit` alerting rule was moved from [single-server](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts.yml) and [cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts-cluster.yml) alerts to the [list of "health" alerts](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts-health.yml) as it could be related to many VictoriaMetrics components. 

* BUGFIX: [storage](./Single-Server-VictoriaMetrics.md): properly set next retention time for indexDB. Previously it may enter into endless retention loop. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4873) for details.
* BUGFIX: [vmagent](./vmagent.md): return human readable error if opentelemetry has json encoding. Follow-up after [PR](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2570).
* BUGFIX: [vmagent](./vmagent.md): properly validate scheme for `proxy_url` field at the scrape config. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4811) for details.
* BUGFIX: [vmagent](./vmagent.md): properly apply `if` filters during [relabeling](./vmagent.md#relabeling-enhancements). Previously the `if` filter could improperly work. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4806) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4816).
* BUGFIX: [vmagent](./vmagent.md): use local scrape timestamps for the scraped metrics unless `honor_timestamps: true` option is explicitly set at [scrape_config](./sd_configs.md#scrape_configs). This fixes gaps for metrics collected from [cadvisor](https://github.com/google/cadvisor) or similar exporters, which export metrics with invalid timestamps. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697#issuecomment-1654614799) for details. The issue has been introduced in [v1.68.0](./CHANGELOG_2021.md#v1680).
* BUGFIX: [vmagent](./vmagent.md): fixes runtime panic at OpenTelemetry parser. Opentelemetry format allows histograms without `sum` fields. Such histogram converted as counter with `_count` suffix. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4814).
* BUGFIX: [vmagent](./vmagent.md): keep unmatched series at [stream aggregation](./stream-aggregation.md) when `-remoteWrite.streamAggr.dropInput` is set to `false` to match intended behaviour introduced at [v1.92.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.92.0). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4804).
* BUGFIX: [vmalert](./vmalert.md): properly set `vmalert_config_last_reload_successful` value on configuration updates or rollbacks. The bug was introduced in [v1.92.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.92.0) in [this PR](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4543).
* BUGFIX: [vmalert](./vmalert.md): fix `vmalert_remotewrite_send_duration_seconds_total` value, before it didn't count in the real time spending on remote write requests. See [this pr](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4801) for details.
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): fix panic when creating a backup to a local filesystem on Windows. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4704).
* BUGFIX: [vmui](./README.md#vmui): properly handle client address with `X-Forwarded-For` part at the [Active queries](./README.md#active-queries) page. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4676#issuecomment-1663203424).
* BUGFIX: [MetricsQL](./MetricsQL.md): prevent from panic when the lookbehind window in square brackets of [rollup function](./MetricsQL.md#rollup-functions) is parsed into negative value. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4795).

## [v1.92.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.92.1)

Released at 2023-07-28

* BUGFIX: [vmalert](./vmalert.md): revert unit test feature for alerting and recording rules introduced in [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4596). See the following [change](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4734).

## [v1.92.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.92.0)

Released at 2023-07-27

**Update note: this release contains backwards-incompatible change to indexdb,
so rolling back to the previous versions of VictoriaMetrics may result in partial data loss of entries in indexdb.**

**Update note**: starting from this release, [stream aggregation](./stream-aggregation.md) writes
the following samples to the configured remote storage by default:

- aggregated samples;
- the original input samples, which match zero `match` options from the provided [config](./stream-aggregation.md#stream-aggregation-config).

Previously only aggregated samples were written to the storage by default.
The previous behavior can be restored in the following ways:

- by passing `-streamAggr.dropInput` command-line flag to single-node VictoriaMetrics;
- by passing `-remoteWrite.streamAggr.dropInput` command-line flag per each configured `-remoteWrite.streamAggr.config` at `vmagent`.

---

* SECURITY: upgrade base docker image (alpine) from 3.18.0 to 3.18.2. See [alpine 3.18.2 release notes](https://alpinelinux.org/posts/Alpine-3.15.9-3.16.6-3.17.4-3.18.2-released.html).
* SECURITY: upgrade Go builder from Go1.20.5 to Go1.20.6. See [the list of issues addressed in Go1.20.6](https://github.com/golang/go/issues?q=milestone%3AGo1.20.6+label%3ACherryPickApproved).

* FEATURE: reduce memory usage by up to 5x for setups with [high churn rate](./FAQ.md#what-is-high-churn-rate) and long [retention](./README.md#retention). See [the description for this change](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/7094fa38bc207c7bd7330ea8a834310a310ce5e3) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4563) for details.
* FEATURE: reduce spikes in CPU and disk IO usage during `indexdb` rotation (aka inverted index), which is performed once per [`-retentionPeriod`](./README.md#retention). The new algorithm gradually pre-populates newly created `indexdb` during the last hour before the rotation. The number of pre-populated series in the newly created `indexdb` can be [monitored](./README.md#monitoring) via `vm_timeseries_precreated_total` metric. This should resolve [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401).
* FEATURE: [MetricsQL](./MetricsQL.md): allow selecting time series matching at least one of multiple `or` filters. For example, `{env="prod",job="a" or env="dev",job="b"}` selects series with either `{env="prod",job="a"}` or `{env="dev",job="b"}` labels. This functionality allows passing the selected series to [rollup functions](./MetricsQL.md#rollup-functions) without the need to use [subqueries](./MetricsQL.md#subqueries). See [these docs](./keyConcepts.md#filtering-by-multiple-or-filters).
* FEATURE: [MetricsQL](./MetricsQL.md): add ability to preserve metric names for binary operation results via `keep_metric_names` modifier. For example, `({__name__=~"foo|bar"} / 10) keep_metric_names` leaves `foo` and `bar` metric names in division results. See [these docs](./MetricsQL.md#keep_metric_names). This helps to address issues like [this one](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3710).
* FEATURE: [MetricsQL](./MetricsQL.md): add ability to copy all the labels from `one` side of [many-to-one operations](https://prometheus.io/docs/prometheus/latest/querying/operators/#many-to-one-and-one-to-many-vector-matches) by specifying `*` inside `group_left()` or `group_right()`. Also allow adding a prefix for copied label names via `group_left(*) prefix "..."` syntax. For example, the following query copies Kubernetes namespace labels to `kube_pod_info` series and adds `ns_` prefix for the copied label names: `kube_pod_info * on(namespace) group_left(*) prefix "ns_" kube_namespace_labels`. The labels from `on()` list aren't prefixed. This feature resolves [this](https://stackoverflow.com/questions/76661818/how-to-add-namespace-labels-to-pod-labels-in-prometheus) and [that](https://stackoverflow.com/questions/76653997/how-can-i-make-a-new-copy-of-kube-namespace-labels-metric-with-a-different-name) questions at StackOverflow.
* FEATURE: [MetricsQL](./MetricsQL.md): add ability to specify durations via [`WITH` templates](https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/prometheus/expand-with-exprs). Examples:
  - `WITH (w = 5m) m[w]` is automatically transformed to `m[5m]`
  - `WITH (f(window, step, off) = m[window:step] offset off) f(5m, 10s, 1h)` is automatically transformed to `m[5m:10s] offset 1h`
  Thanks to @lujiajing1126 for the initial idea and [implementation](https://github.com/VictoriaMetrics/metricsql/pull/13). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4025).
* FEATURE: [vmui](./README.md#vmui): added a new page with the list of currently running queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4598) and [these docs](./README.md#active-queries).
* FEATURE: [vmagent](./vmagent.md): add support for data ingestion via [OpenTelemetry protocol](https://opentelemetry.io/docs/reference/specification/metrics/). See [these docs](./Single-Server-VictoriaMetrics.md#sending-data-via-opentelemetry), [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2424) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2570).
* FEATURE: [vmagent](./vmagent.md): allow sharding outgoing time series among the configured remote storage systems. This can be useful for building horizontally scalable [stream aggregation](./stream-aggregation.md), when samples for the same time series must be aggregated by the same `vmagent` instance at the second level. See [these docs](./vmagent.md#sharding-among-remote-storages) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4637) for details.
* FEATURE: [vmagent](./vmagent.md): allow configuring staleness interval in [stream aggregation](./stream-aggregation.md) config. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4667) for details.
* FEATURE: [vmagent](./vmagent.md): allow specifying a list of [series selectors](./keyConcepts.md#filtering) inside `if` option of relabeling rules. The corresponding relabeling rule is executed when at least a single series selector matches. See [these docs](./vmagent.md#relabeling-enhancements).
* FEATURE: [stream aggregation](./stream-aggregation.md): allow specifying a list of [series selectors](./keyConcepts.md#filtering) inside `match` option of [stream aggregation configs](./stream-aggregation.md#stream-aggregation-config). The input sample is aggregated when at least a single series selector matches. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4635).
* FEATURE: [stream aggregation](./stream-aggregation.md): preserve input samples, which match zero `match` options from the [configured aggregations](./stream-aggregation.md#stream-aggregation-config). Previously all the input samples were dropped by default, so only the aggregated samples are written to the output storage. The previous behavior can be restored by passing `-streamAggr.dropInput` command-line flag to single-node VictoriaMetrics or by passing `-remoteWrite.streamAggr.dropInput` command-line flag to `vmagent`.
* FEATURE: [vmctl](./vmctl.md): add verbose output for docker installations or when TTY isn't available. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4081).
* FEATURE: [vmctl](./vmctl.md): interrupt backoff retries when import process is cancelled. The change makes vmctl more responsive in case of errors during the import. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4442).
* FEATURE: [vmctl](./vmctl.md): update backoff policy on retries to reduce probability of overloading for `source` or `destination` databases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4402).
* FEATURE: vmstorage: suppress "broken pipe" and "connection reset by peer" errors for search queries on vmstorage side. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4418/commits/a6a7795b9e1f210d614a2c5f9a3016b97ded4792) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4498/commits/830dac177f0f09032165c248943a5da0e10dfe90) commits.
* FEATURE: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): add panel for tracking rate of syscalls while writing or reading from disk via `process_io_(read|write)_syscalls_total` metrics.
* FEATURE: accept timestamps in milliseconds at `start`, `end` and `time` query args in [Prometheus querying API](./README.md#prometheus-querying-api-usage). See [these docs](./README.md#timestamp-formats) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4459).
* FEATURE: [vmalert](./vmalert.md): update retry policy for pushing data to `-remoteWrite.url`. By default, vmalert will make multiple retry attempts with exponential delay. The total time spent during retry attempts shouldn't exceed `-remoteWrite.retryMaxTime` (default is 30s). When retry time is exceeded vmalert drops the data dedicated for `-remoteWrite.url`. Before, vmalert dropped data after 5 retry attempts with 1s delay between attempts (not configurable). See `-remoteWrite.retryMinInterval` and `-remoteWrite.retryMaxTime` cmd-line flags.
* FEATURE: [vmalert](./vmalert.md): expose `vmalert_remotewrite_send_duration_seconds_total` counter, which can be used for determining high saturation of every connection to remote storage with an alerting query `sum(rate(vmalert_remotewrite_send_duration_seconds_total[5m])) by(job, instance) > 0.9 * max(vmalert_remotewrite_concurrency) by(job, instance)`. This query triggers when a connection is saturated by more than 90%. This usually means that `-remoteWrite.concurrency` command-line flag must be increased in order to increase the number of concurrent writings into remote endpoint. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4516).
* FEATURE: [vmalert](./vmalert.md): display the error message received during unsuccessful config reload in vmalert's UI. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4076) for details.
* FEATURE: [vmalert](./vmalert.md): allow disabling of `step` param attached to [instant queries](./keyConcepts.md#instant-query). This might be useful for using vmalert with datasources that to not support this param, unlike VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4573) for details.
* FEATURE: [vmalert](./vmalert.md): support option for "blackholing" alerting notifications if `-notifier.blackhole` cmd-line flag is set. Enable this flag if you want vmalert to evaluate alerting rules without sending any notifications to external receivers (eg. alertmanager). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4122) for details. Thanks to @venkatbvc for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4639).
* FEATURE: [vmalert](./vmalert.md): add unit test for alerting and recording rules, see more [details](./vmalert.md#unit-testing-for-rules) here. Thanks to @Haleygo for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4596).
* FEATURE: [vmalert](./vmalert.md): allow overriding default GET params for rules  with `graphite` datasource type, in the same way as it happens for `prometheus` type. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4685).
* FEATURE: [vmalert](./vmalert.md): support `keep_firing_for` field for alerting rules. See docs updated [here](./vmalert.md#alerting-rules) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4529). Thanks to @Haleygo for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4669).
* FEATURE: [vmauth](./vmauth.md): expose `vmauth_user_request_duration_seconds` and `vmauth_unauthorized_user_request_duration_seconds` summary metrics for measuring requests latency per user.
* FEATURE: [vmbackup](./vmbackup.md): show backup progress percentage in log during backup uploading. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4460).
* FEATURE: [vmrestore](./vmrestore.md): show restoring progress percentage in log during backup downloading. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4460).
* FEATURE: add ability to fine-tune Graphite API limits via the following command-line flags:
  `-search.maxGraphiteTagKeys` for limiting the number of tag keys returned from [Graphite API for tags](./README.md#graphite-tags-api-usage)
  `-search.maxGraphiteTagValues` for limiting the number of tag values returned from [Graphite API for tag values](./README.md#graphite-tags-api-usage)
  `-search.maxGraphiteSeries` for limiting the number of series (aka paths) returned from [Graphite API for series](./README.md#graphite-tags-api-usage)
  See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4339).

* BUGFIX: properly return series from [/api/v1/series](./Single-Server-VictoriaMetrics.md#prometheus-querying-api-usage) if it finds more than the `limit` series (`limit` is an optional query arg passed to this API). Previously the `limit exceeded error` error was returned in this case. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2841#issuecomment-1560055631).
* BUGFIX: [vmui](./README.md#vmui): fix application routing issues and problems with manual URL changes. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4408) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4604).
* BUGFIX: add validation for invalid [partial RFC3339 timestamp formats](./Single-Server-VictoriaMetrics.md#timestamp-formats) in query and export APIs.
* BUGFIX: [vmctl](./vmctl.md): interrupt explore procedure in influx mode if vmctl found no numeric fields.
* BUGFIX: [vmctl](./vmctl.md): fix panic in case `--remote-read-filter-time-start` flag is not set for remote-read mode. This flag is now required to use remote-read mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4553).
* BUGFIX: [vmctl](./vmctl.md): fix formatting issue, which could add superfluous `s` characters at the end of `samples/s` output during data migration. For example, it could write `samples/ssssss`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4555).
* BUGFIX: [vmalert](./vmalert.md): use RFC3339 time format in query args instead of unix timestamp for all issued queries to Prometheus-like datasources.
* BUGFIX: [vmalert](./vmalert.md): correctly calculate evaluation time for rules. Before, there was a low probability for discrepancy between actual time and rules evaluation time if evaluation interval was lower than the execution time for rules within the group.
* BUGFIX: [vmalert](./vmalert.md): reset evaluation timestamp after modifying group interval. Before, there could have latency on rule evaluation time.
* BUGFIX: vmselect: fix timestamp alignment for Prometheus querying API if time argument is less than 10m from the beginning of Unix epoch.
* BUGFIX: [vmagent](./vmagent.md): close HTTP connections to [service discovery](./sd_configs.md) servers when they are no longer needed. This should prevent from possible connection exhaustion in some cases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4724).
* BUGFIX: [vmagent](./vmagent.md): do not show [relabel debug](./vmagent.md#relabel-debug) links at the `/targets` page when `vmagent` runs with `-promscrape.dropOriginalLabels` command-line flag, since it has no the original labels needed for relabel debug. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4597).
* BUGFIX: vminsert: fixed decoding of label values with slash when accepting data via [pushgateway protocol](./README.md#how-to-import-data-in-prometheus-exposition-format). This fixes Prometheus golang client compatibility. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4692).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly parse binary operations with reserved words on the right side such as `foo + (on{bar="baz"})`. Previously such queries could lead to panic. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4422).
* BUGFIX: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): display cache usage for all components on panel `Cache usage % by type` for cluster dashboard. Before, only vmstorage caches were shown.

## [v1.91.3](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.91.3)

Released at 2023-06-30

* SECURITY: upgrade Go builder from Go1.20.4 to Go1.20.5. See [the list of issues addressed in Go1.20.5](https://github.com/golang/go/issues?q=milestone%3AGo1.20.5+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): fix possible panic at shutdown when [stream aggregation](./stream-aggregation.md) is enabled. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4407) for details.
* BUGFIX: [vmagent](./vmagent.md): fixed service name detection for [consulagent service discovery](./sd_configs.md#consulagent_sd_configs) in case of a difference in service name and service id. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4390) for details.
* BUGFIX: [vmalert](./vmalert.md): retry all errors except 4XX status codes while pushing via remote-write to the remote storage. Previously, errors like broken connection could prevent vmalert from retrying the request.
* BUGFIX: [vmalert](./vmalert.md): properly interrupt retry attempts on vmalert shutdown. Before, vmalert could have waited for all retries to finish for shutdown.
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): fix an issue with `vmbackupmanager` not being able to restore data from a backup stored in GCS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4420) for details.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly return error from [/api/v1/query](./keyConcepts.md#instant-query) and [/api/v1/query_range](./keyConcepts.md#range-query) at `vmselect` when the `-search.maxSamplesPerQuery` or `-search.maxSamplesPerSeries` [limit](./Cluster-VictoriaMetrics.md#resource-usage-limits) is exceeded. Previously incomplete response could be returned without the error if `vmselect` runs with `-replicationFactor` greater than 1. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4472).
* BUGFIX: [storage](./Single-Server-VictoriaMetrics.md): prevent from possible crashloop after the migration from versions below `v1.90.0` to newer versions. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4336) for details.
* BUGFIX: [vmui](./README.md#vmui): fix a memory leak issue associated with chart updates. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4455).
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): fix removing storage data dir before restoring from backup.
* BUGFIX: vmselect: wait for all vmstorage nodes to respond when the `-replicationFactor` flag is set bigger than > 1. Before, vmselect could have [skip waiting for the slowest replicas](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711) to respond. This could have resulted in issues illustrated [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1207). Now, this optimization is disabled by default and could be re-enabled by passing `-search.skipSlowReplicas` cmd-line flag to vmselect. See more details [here](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4538). 


## [v1.91.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.91.2)

Released at 2023-06-02

* BUGFIX: [vmalert](./vmalert.md): fix nil map assignment panic in runtime introduced in this [change](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4341).

## [v1.91.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.91.1)

Released at 2023-06-01

* FEATURE:[vmagent](./vmagent.md): Adds `follow_redirects` at service discovery level of scrape configuration. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4282). Thanks to @Haleygo for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4286).
* FEATURE: vmselect: Decreases startup time for vmselect with a big number of vmstorage nodes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4364). Thanks to @Haleygo for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4366).

* BUGFIX: [vmalert](./vmalert.md): Properly form path to static assets in WEB UI if `http.pathPrefix` set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4349).
* BUGFIX: [vmalert](./vmalert.md): Properly set datasource query params. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4340). Thanks to @gsakun for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4341).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` for groups with present name but absent `rules`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).
* BUGFIX: [vmauth](./vmauth.md): Properly handle LOCAL command for proxy protocol. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3335#issuecomment-1569864108).
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): Fixes crash on startup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4378).
* BUGFIX: [vmui](./README.md#vmui): fix bug with custom URL in global settings not respecting tenantID change. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4322).

## [v1.91.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.91.0)

Released at 2023-05-18

* SECURITY: upgrade Go builder from Go1.20.3 to Go1.20.4. See [the list of issues addressed in Go1.20.4](https://github.com/golang/go/issues?q=milestone%3AGo1.20.4+label%3ACherryPickApproved).
* SECURITY: serve `/robots.txt` content to disallow indexing of the exposed instances by search engines. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4128) for details.

* FEATURE: update [docker compose environment](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#docker-compose-environment-for-victoriametrics) to V2 in respect to V1 deprecation notice from June 2023. See [Migrate to Compose V2](https://docs.docker.com/compose/migrate/).
* FEATURE: deprecate `-bigMergeConcurrency` command-line flag, since improper configuration for this flag frequently led to uncontrolled growth of unmerged parts, which, in turn, could lead to queries slowdown and increased CPU usage. The concurrency for [background merges](./README.md#storage) can be controlled via `-smallMergeConcurrency` command-line flag, though it isn't recommended to change this flag in general case.
* FEATURE: do not execute the incoming request if it has been canceled by the client before the execution start. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4223).
* FEATURE: support time formats with timezones. For example, `2024-01-02+02:00` means `January 2, 2024` at `+02:00` time zone. See [these docs](./README.md#timestamp-formats).
* FEATURE: expose `process_*` metrics at `/metrics` page of all the VictoriaMetrics components under Windows OS. See [this pull request](https://github.com/VictoriaMetrics/metrics/pull/47).
* FEATURE: reduce the amounts of unimportant `INFO` logging during VictoriaMetrics startup / shutdown. This should improve visibility for potentially important logs.
* FEATURE: upgrade base docker image (alpine) from 3.17.3 to 3.18.0. See [alpine 3.18.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.18.0-released.html).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): do not pollute logs with `cannot read hello: cannot read message with size 11: EOF` messages at `vmstorage` during TCP health checks performed by [Consul](https://developer.hashicorp.com/consul/docs/services/usage/checks) or [other services](https://docs.nginx.com/nginx/admin-guide/load-balancer/tcp-health-check/). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1762).
* FEATURE: [vmagent](./vmagent.md): support the ability to filter [consul_sd_configs](./sd_configs.md#consul_sd_configs) targets in more optimal way via new `filter` option. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4183).
* FEATURE: [vmagent](./vmagent.md): add support for [consulagent_sd_configs](./sd_configs.md#consulagent_sd_configs). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3953).
* FEATURE: [vmagent](./vmagent.md): emit a warning if too small value is passed to `-remoteWrite.maxDiskUsagePerURL` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4195).
* FEATURE: [vmalert](./vmalert.md): add support of recursive globs for `-rule` and `-rule.templates` command-line flags by using `**` in the glob pattern. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4041).
* FEATURE: [vmalert](./vmalert.md): add ability to specify custom per-group HTTP headers sent to the configured notifiers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3260). Thanks to @Haleygo for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4088).
* FEATURE: [vmalert](./vmalert.md): detect alerting rules which don't match any series. See [these docs](./vmalert.md#never-firing-alerts) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4039).
* FEATURE: [vmalert](./vmalert.md): support loading rules via HTTP URL. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3352). Thanks to @Haleygo for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4212).
* FEATURE: [vmalert](./vmalert.md): add buttons for filtering groups/rules with errors or with no-match warning in web UI for page `/groups`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4039).
* FEATURE: [vmalert](./vmalert.md): do not retry remote-write requests for responses with 4XX status codes. This aligns with [Prometheus remote write specification](https://prometheus.io/docs/concepts/remote_write_spec/). Thanks to @MichaHoffmann for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4134).  
* FEATURE: [vmauth](./vmauth.md): add ability to filter incoming requests by IP. See [these docs](./vmauth.md#ip-filters) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3491).
* FEATURE: [vmauth](./vmauth.md): add ability to proxy requests to the specified backends for unauthorized users. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4083).
* FEATURE: [vmauth](./vmauth.md): add ability to specify default route for unmatched requests. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4084).
* FEATURE: [vmauth](./vmauth.md): retry `POST` requests on the remaining backends if the currently selected backend isn't reachable. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4242).
* FEATURE: [vmui](./README.md#vmui): add ability to compare the data for the previous day with the data for the current day at [Cardinality Explorer](./README.md#cardinality-explorer). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3967).
* FEATURE: [vmui](./README.md#vmui): display histograms as heatmaps in [Metrics explorer](./README.md#metrics-explorer). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4111).
* FEATURE: [vmui](./README.md#vmui): add `WITH template` playground. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3811).
* FEATURE: [vmui](./README.md#vmui): add ability to debug relabeling. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3807).
* FEATURE: [vmui](./README.md#vmui): add an ability to copy and execute queries listed at [top queries](./README.md#top-queries) page. Also make more human readable the query duration column. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4292) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4299).
* FEATURE: [vmui](./README.md#vmui): increase default font size for better readability.
* FEATURE: [vmui](./README.md#vmui): [cardinality explorer](./README.md#cardinality-explorer): return back a table with labels containing the highest number of unique label values. See [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4213).
* FEATURE: [vmui](./README.md#vmui): add notification icon for queries that do not match any time series. A warning icon appears next to the query field when the executed query does not match any time series. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4211).
* FEATURE: [vmbackup](./vmbackup.md): add `-s3StorageClass` command-line flag for setting the storage class for AWS S3 backups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4164). Thanks to @justcompile for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4166).
* FEATURE: [vmbackup](./vmbackup.md): store backup creation and completion time in `backup_complete.ignore` file of backup contents. This allows determining the exact timestamp when the backup was created and completed.
* FEATURE: [vmbackupmanager](./vmbackupmanager.md): add `created_at` field to the output of `/api/v1/backups` API and `vmbackupmanager backup list` command. See this [doc](./vmbackupmanager.md#api-methods) for data format details.
* FEATURE: [vmbackupmanager](./vmbackupmanager.md): add commands for locking/unlocking backups against deletion by retention policy. See this [doc](./vmbackupmanager.md#api-methods) for data format details.
* FEATURE: [vmctl](./vmctl.md): add support for [different time formats](./Single-Server-VictoriaMetrics.md#timestamp-formats) for `--vm-native-filter-time-start` and `--vm-native-filter-time-end` command-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4091).
* FEATURE: [vmctl](./vmctl.md): set default value for `--vm-native-step-interval` command-line flag to `month`. This enables [time-based chunking](./vmctl.md#using-time-based-chunking-of-migration) of data based on monthly step value when using native migration mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4309).

* BUGFIX: reduce the probability of sudden increase in the number of small parts on systems with small number of CPU cores.
* BUGFIX: reduce the possibility of increased CPU usage when data with timestamps older than one hour is ingested into VictoriaMetrics. This reduces spikes for the graph `sum(rate(vm_slow_per_day_index_inserts_total))`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4258).
* BUGFIX: fix possible infinite loop during `indexdb` rotation when `-retentionTimezoneOffset` command-line flag is set and the local timezone is not UTC. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4207). Thanks to @faceair for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4206).
* BUGFIX: do not panic at Windows during [snapshot deletion](./README.md#how-to-work-with-snapshots). Instead, delete the snapshot on the next restart. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70#issuecomment-1491529183) for details.
* BUGFIX: change the max allowed value for `-memory.allowedPercent` from 100 to 200. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4171).
* BUGFIX: properly limit the number of [OpenTSDB HTTP](./README.md#sending-opentsdb-data-via-http-apiput-requests) concurrent requests specified via `-maxConcurrentInserts` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4204). Thanks to @zouxiang1993 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4208).
* BUGFIX: do not ignore trailing empty field in CSV lines when [importing data in CSV format](./README.md#how-to-import-csv-data). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4048).
* BUGFIX: disallow `"` chars when parsing Prometheus label names, since they aren't allowed by [Prometheus text exposition format](https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-format-example). Previously this could result in silent incorrect parsing of incorrect Prometheus labels such as `foo{"bar"="baz"}` or `{foo:"bar",baz="aaa"}`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4284).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): prevent from possible panic when the number of vmstorage nodes increases when [automatic vmstorage discovery](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery) is enabled.
* BUGFIX: [MetricsQL](./MetricsQL.md): fix a panic when the duration in the query contains uppercase `M` suffix. Such a suffix isn't allowed to use in durations, since it clashes with `a million` suffix, e.g. it isn't clear whether `rate(metric[5M])` means rate over 5 minutes, 5 months or 5 million seconds. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3589) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4120) issues.
* BUGFIX: [vmagent](./vmagent.md): properly handle the `vm_promscrape_config_last_reload_successful` metric after config reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4260).
* BUGFIX: [vmagent](./vmagent.md): add `__meta_kubernetes_endpoints_name` label for all ports discovered from endpoint. Previously, ports not matched by `Service` did not have this label. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4154) for details. Thanks to @thunderbird86 for discovering and [fixing](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4253) the issue.
* BUGFIX: [vmalert](./vmalert.md): retry failed read request on the closed connection one more time. This improves rules execution reliability when connection between vmalert and datasource closes unexpectedly.
* BUGFIX: [vmalert](./vmalert.md): properly display an error when using `query` function for templating value of `-external.alert.source` flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4181).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` and `/api/v1/alerts` API handlers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).
* BUGFIX: [vmauth](./vmauth.md): do not return invalid auth credentials in http response by default, since it may be logged by client. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4188).
* BUGFIX: [vmui](./README.md#vmui): fix the display of the tenant selector. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4160).
* BUGFIX: [vmui](./README.md#vmui): fix UI freeze when the query returns non-histogram series alongside histogram series.
* BUGFIX: [vmui](./README.md#vmui): fix the text display on buttons in Safari 16.4.
* BUGFIX: [alerts-health](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts-health.yml): update threshold for `TooHighMemoryUsage` alert from 90% to 80%, since 90% is too high for production environments.
* BUGFIX: [vmbackup](./vmbackup.md): fix compatibility with Windows OS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70).
* BUGFIX: [vmctl](./vmctl.md): fix performance issue when migrating data from VictoriaMetrics according to [these docs](./vmctl.md#migrating-data-from-victoriametrics). Add the ability to speed up the data migration via `--vm-native-disable-retries` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4092).
* BUGFIX: [stream aggregation](./stream-aggregation.md): fix bug with duplicated labels during stream aggregation via single-node VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4277).

## [v1.90.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.90.0)

Released at 2023-04-06

**Update note: this release contains backwards-incompatible change in storage data format,
so the previous versions of VictoriaMetrics will exit with the `unexpected number of substrings in the part name` error when trying to run them on the data
created by v1.90.0 or newer versions. The solution is to upgrade to v1.90.0 or newer releases**

* SECURITY: upgrade base docker image (alpine) from 3.17.2 to 3.17.3. See [alpine 3.17.3 release notes](https://alpinelinux.org/posts/Alpine-3.17.3-released.html).
* SECURITY: upgrade Go builder from Go1.20.2 to Go1.20.3. See [the list of issues addressed in Go1.20.3](https://github.com/golang/go/issues?q=milestone%3AGo1.20.3+label%3ACherryPickApproved).

* FEATURE: open source [Graphite Render API](./README.md#graphite-render-api-usage). This API allows using VictoriaMetrics as a drop-in replacement for Graphite at both data ingestion and querying sides and reducing infrastructure costs by up to 10x comparing to Graphite. See [this case study](./CaseStudies.md#grammarly) as an example.
* FEATURE: release Windows binaries for [single-node VictoriaMetrics](./README.md), [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md), [vmbackup](./vmbackup.md) and [vmrestore](./vmrestore.md). See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3236), [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3821) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70) issues. This release of VictoriaMetrics for Windows cannot delete [snapshots](./README.md#how-to-work-with-snapshots) due to Windows constraints. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70#issuecomment-1491529183) for details. This issue should be resolved in future releases.
* FEATURE: log metrics with truncated labels if the length of label value in the ingested metric exceeds `-maxLabelValueLen`. This should simplify debugging for this case.
* FEATURE: [vmagent](./vmagent.md): show target URL when debugging [target relabeling](./vmagent.md#relabel-debug). This should simplify target relabel debugging a bit. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3882).
* FEATURE: [vmagent](./vmagent.md): add support for [VictoriaMetrics remote write protocol](./vmagent.md#victoriametrics-remote-write-protocol) when [sending / receiving data to / from Kafka](./vmagent.md#kafka-integration). This protocol allows saving egress network bandwidth costs when sending data from `vmagent` to `Kafka` located in another datacenter or availability zone. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1225).
* FEATURE: [vmagent](./vmagent.md): add `-kafka.consumer.topic.concurrency` command-line flag. It controls the number of Kafka consumer workers to use by `vmagent`. It should eliminate the need to start multiple `vmagent` instances to improve data transfer rate. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1957).
* FEATURE: [vmagent](./vmagent.md): add support for [Kafka producer and consumer](./vmagent.md#kafka-integration) on `arm64` machines. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2271).
* FEATURE: [vmagent](./vmagent.md): delete unused buffered data at `-remoteWrite.tmpDataPath` directory when there is no matching `-remoteWrite.url` to send this data to. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4014).
* FEATURE: [vmagent](./vmagent.md): add the ability for hot reloading of [stream aggregation](./stream-aggregation.md) configs. See [these docs](./stream-aggregation.md#configuration-update) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3969).
* FEATURE: check the contents of `-relabelConfig` and `-streamAggr.config` files additionally to `-promscrape.config` when single-node VictoriaMetrics runs with `-dryRun` command-line flag. This aligns the behaviour of single-node VictoriaMetrics with [vmagent](./vmagent.md) behaviour for `-dryRun` command-line flag.
* FEATURE: [vmui](./README.md#vmui): automatically draw a heatmap graph when the query selects a single [histogram](./keyConcepts.md#histogram). This simplifies analyzing histograms. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3384).
* FEATURE: [vmui](./README.md#vmui): add support for drag'n'drop and paste from clipboard in the "Trace analyzer" page. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3971).
* FEATURE: [vmui](./README.md#vmui): hide messages longer than 3 lines in the trace. You can view the full message by clicking on the `show more` button. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3971).
* FEATURE: [vmui](./README.md#vmui): add the ability to manually input date and time when selecting a time range. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3968).
* FEATURE: [vmui](./README.md#vmui): updated usability and the search process in cardinality explorer. Made this process straightforward for user. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3986).
* FEATURE: [vmui](./README.md#vmui): add the ability to collapse/expand the legend. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4045).
* FEATURE: [vmui](./README.md#vmui): add tips for working with the graph and legend. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4045).
* FEATURE: [vmui](./README.md#vmui): add `apply` and `cancel` buttons to settings popup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4013).
* FEATURE: [vmctl](./vmctl.md): automatically disable progress bar when TTY isn't available. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3823).
* FEATURE: [vmauth](./vmauth.md): add `-configCheckInterval` command-line flag, which can be used for automatic re-reading the `-auth.config` file. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3990).

* BUGFIX: prevent from slow [snapshot creating](./README.md#how-to-work-with-snapshots) under high data ingestion rate. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3551).
* BUGFIX: [vmauth](./vmauth.md):  suppress [proxy protocol](https://www.haproxy.org/download/2.3/doc/proxy-protocol.txt) parsing errors in case of `EOF`. Usually, the error is caused by health checks and is not a sign of an actual error.
* BUGFIX: [vmui](./README.md#vmui): fix displaying errors for each query. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3987).
* BUGFIX: [vmbackup](./vmbackup.md): fix snapshot not being deleted in case of error during backup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2055).
* BUGFIX: [stream aggregation](./stream-aggregation.md): suppress `series after dedup` error message in logs when `-remoteWrite.streamAggr.dedupInterval` command-line flag is set at [vmagent](./vmagent.md) or when `-streamAggr.dedupInterval` command-line flag is set at [single-node VictoriaMetrics](./README.md).
* BUGFIX: allow using dashes and dots in environment variables names referred in config files via `%{ENV-VAR.SYNTAX}`. See [these docs](./README.md#environment-variables) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3999).
* BUGFIX: return back query performance scalability on hosts with big number of CPU cores. The scalability has been reduced in [v1.86.0](./CHANGELOG.md#v1860). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3966).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly convert [VictoriaMetrics historgram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) to Prometheus histogram buckets when VictoriaMetrics histogram contain zero buckets. Previously these buckets were ignored, and this could lead to missing Prometheus histogram buckets after the conversion. Thanks to @zklapow for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4021).
* BUGFIX: [vmagent](./vmagent.md): fix CPU and memory usage spikes when files pointed by [file_sd_config](./sd_configs.md#file_sd_configs) cannot be re-read. See [this_issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3989).
* BUGFIX: prevent unexpected merges on start-up when `-storage.minFreeDiskSpaceBytes` is set. See [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4023).
* BUGFIX: properly support comma-separated filters inside [retention filters](./README.md#retention-filters). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3915).
* BUGFIX: verify response code when fetching configuration files via HTTP. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4034).
* BUGFIX: [vmalert](./vmalert.md): replace empty labels with `""` instead of `"<no value>"` during templating, as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4012).
* BUGFIX: [vmctl](./vmctl.md): properly pass multiple filters from `--vm-native-filter-match` command-line flag to the data source. Previously filters from `--vm-native-filter-match` were only used to discover the metric names, and the metric names like `__name__="metric_name"` has been taken into account, while the remaining filters were ignored. For example `--vm-native-src-addr={foo="bar",baz="abc"}` may found `metric_name{foo="bar",baz="abc"}` and filter was treated as `--vm-native-src-addr={__name__="metrics_name"}`, e.g. `foo="bar",baz="abc"` filter was ignored. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4062). 


## [v1.89.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.89.1)

Released at 2023-03-12

* BUGFIX: prevent from possible `cannot unmarshal timeseries from rollupResultCache` panic after the upgrade to [v1.89.0](./CHANGELOG.md#v1890).

## [v1.89.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.89.0)

Released at 2023-03-12

**Update note: this release can crash with `cannot unmarshal timeseries from rollupResultCache` panic after the upgrade from the previous releases.
This issue can be fixed by removing caches stored on disk according to [these docs](./README.md#cache-removal).
Another option is to upgrade to [v1.89.1](./CHANGELOG.md#v1891).**

* SECURITY: upgrade Go builder from Go1.20.1 to Go1.20.2. See [the list of issues addressed in Go1.20.2](https://github.com/golang/go/issues?q=milestone%3AGo1.20.2+label%3ACherryPickApproved).

* FEATURE: [vmctl](./vmctl.md): increase the default value for `--remote-read-http-timeout` command-line option from 30s (30 seconds) to 5m (5 minutes). This reduces the probability of timeout errors when migrating big number of time series. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3879).
* FEATURE: [vmctl](./vmctl.md): migrate series one-by-one in [vm-native mode](./vmctl.md#migrating-data-from-victoriametrics). This allows better tracking the migration progress and resuming the migration process from the last migrated time series. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3859) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3600).
* FEATURE: [vmctl](./vmctl.md): add `--vm-native-src-headers` and `--vm-native-dst-headers` command-line flags, which can be used for setting custom HTTP headers during [vm-native migration mode](./vmctl.md#migrating-data-from-victoriametrics). Thanks to @baconmania for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3906).
* FEATURE: [vmctl](./vmctl.md): add `--vm-native-src-bearer-token` and `--vm-native-dst-bearer-token` command-line flags, which can be used for setting Bearer token headers for the source and the destination storage during [vm-native migration mode](./vmctl.md#migrating-data-from-victoriametrics). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3835).
* FEATURE: [vmctl](./vmctl.md): add `--vm-native-disable-http-keep-alive` command-line flag to allow `vmctl` to use non-persistent HTTP connections in [vm-native migration mode](./vmctl.md#migrating-data-from-victoriametrics). Thanks to @baconmania for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3909).
* FEATURE: [vmalert](./vmalert.md): log number of configration files found for each specified `-rule` command-line flag.
* FEATURE: [vmalert enterprise](./vmalert.md): concurrently [read config files from S3, GCS or S3-compatible object storage](./vmalert.md#reading-rules-from-object-storage). This significantly improves config load speed for cases when there are thousands of files to read from the object storage.

* BUGFIX: vmstorage: fix a bug, which could lead to incomplete or empty results for heavy queries selecting tens of thousands of time series. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3946).
* BUGFIX: vmselect: reduce memory usage and CPU usage when performing heavy queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3692).
* BUGFIX: prevent from possible `invalid memory address or nil pointer dereference` panic during [background merge](./README.md#storage). The issue has been introduced at [v1.85.0](./CHANGELOG.md#v1850). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3897).
* BUGFIX: prevent from possible `SIGBUS` crash on ARM architectures (Raspberry Pi), which deny unaligned access to 8-byte words. Thanks to @oliverpool for narrowing down the issue and for [the initial attempt to fix it](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): always return `is_partial: true` in partial responses. Previously partial responses could be returned as non-partial in some cases.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly take into account `-rpc.disableCompression` command-line flag at `vmstorage`. It was ignored since [v1.78.0](./CHANGELOG.md#v1780). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3932).
* BUGFIX: [vmagent](./vmagent.md): fix panic when [writing data to Kafka](./vmagent.md#writing-metrics-to-kafka). The panic has been introduced in [v1.88.0](./CHANGELOG.md#v1880).
* BUGFIX: [vmui](./README.md#vmui): stop showing `Please enter a valid Query and execute it` error message on the first load of vmui.
* BUGFIX: [vmui](./README.md#vmui): properly process `Run in VMUI` button click in [VictoriaMetrics datasource plugin for Grafana](https://github.com/VictoriaMetrics/victoriametrics-datasource).
* BUGFIX: [vmui](./README.md#vmui): fix the display of the selected value for dropdowns on `Explore` page.
* BUGFIX: [vmui](./README.md#vmui): do not send `step` param for instant queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3896).
* BUGFIX: [vmauth](./vmauth.md): fix `cannot serve http` panic when plain HTTP request is sent to `vmauth` configured to accept requests over [proxy protocol](https://www.haproxy.org/download/2.3/doc/proxy-protocol.txt)-encoded request (e.g. when `vmauth` runs with `-httpListenAddr.useProxyProtocol` command-line flag). The issue has been introduced at [v1.87.0](./CHANGELOG.md#v1870) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3335).
* BUGFIX: [vmgateway](./vmgateway.md): properly parse RSA public key discovered via JWK endpoint.

## [v1.88.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.88.1)

Released at 2023-02-27

* FEATURE: add `-snapshotCreateTimeout` flag to allow configuring timeout for [snapshot process](./README.md#how-to-work-with-snapshots). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3551).
* FEATURE: expose `vm_http_requests_total` and `vm_http_request_errors_total` metrics for `snapshot/*` [paths](./README.md#how-to-work-with-snapshots) at [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md) `vmstorage` and [VictoriaMetrics Single](./Single-Server-VictoriaMetrics.md). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3551).
* FEATURE: [vmgateway](./vmgateway.md): add the ability to discover keys for JWT verification via [OpenID discovery endpoint](https://openid.net/specs/openid-connect-discovery-1_0.html). See [these docs](./vmgateway.md#using-openid-discovery-endpoint-for-jwt-signature-verification).
* FEATURE: add `-internStringDisableCache` command-line flag for disabling the cache for [interned strings](https://en.wikipedia.org/wiki/String_interning). This flag may be useful in [some cases](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3863) for reducing memory usage at the cost of higher CPU usage.
* FEATURE: add `-internStringCacheExpireDuration` command-line flag for controlling the lifetime of cached [interned strings](https://en.wikipedia.org/wiki/String_interning).

* BUGFIX: [MetricsQL](./MetricsQL.md): fix panic when executing the query `aggr_func(rollup*(some_value))`. The panic has been introduced in [v1.88.0](./CHANGELOG.md#v1880).
* BUGFIX: [vmagent](./vmagent.md): use the provided `-remoteWrite.*` auth options when determining whether the remote storage supports [VictoriaMetrics remote write protocol](./vmagent.md#victoriametrics-remote-write-protocol). Previously the auth options were ignored. This was preventing from automatic switch to VictoriaMetrics remote write protocol.
* BUGFIX: [vmagent](./vmagent.md): do not register `vm_promscrape_config_*` metrics if `-promscrape.config` flag is not used. Previously those metrics were registered and never updated, which was confusing and could trigger false-positive alerts.
* BUGFIX: [vmctl](./vmctl.md): skip measurements with no fields when migrating data from influxdb. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3837).
* BUGFIX: delete failed snapshot contents from disk on failed attempt to [create snapshot](./README.md#how-to-work-with-snapshots). Previously failed snapshot contents could remain on disk in incomplete state. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3858)

## [v1.88.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.88.0)

Released at 2023-02-24

* SECURITY: upgrade base docker image (alpine) from 3.17.1 to 3.17.2. See [alpine 3.17.2 release notes](https://alpinelinux.org/posts/Alpine-3.17.2-released.html).
* SECURITY: upgrade Go builder from Go1.20.0 to Go1.20.1. See [the list of issues addressed in Go1.20.1](https://github.com/golang/go/issues?q=milestone%3AGo1.20.1+label%3ACherryPickApproved).

* FEATURE: [vmagent](./vmagent.md): add support for [VictoriaMetrics remote write protocol](./vmagent.md#victoriametrics-remote-write-protocol). This protocol allows saving egress network bandwidth costs when sending data from `vmagent` to VictoriaMetrics located in another datacenter or availability zone. This also allows reducing disk IO under high load when `vmagent` starts queuing the collected data to disk when the remote storage is temporarily unavailable or cannot keep up with the data ingestion rate. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1225).
* FEATURE: [vmagent](./vmagent.md): add support for [Kuma](http://kuma.io/) Control Plane targets discovery aka [kuma_sd_configs](./sd_configs.md#kuma_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3389).
* FEATURE: [vmgateway](./vmgateway.md): add the ability to verify JWT signature via [JWKS endpoint](https://auth0.com/docs/secure/tokens/json-web-tokens/json-web-key-sets). See [these docs](./vmgateway.md#using-jwks-endpoint-for-jwt-signature-verification).
* FEATURE: [vmauth](./vmauth.md): add the ability to limit the number of concurrent requests on a per-user basis via `-maxConcurrentPerUserRequests` command-line flag and via `max_concurrent_requests` config option. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3346) and [these docs](./vmauth.md#concurrency-limiting).
* FEATURE: [vmauth](./vmauth.md): automatically retry failing `GET` requests on all [the configured backends](./vmauth.md#load-balancing). Previously the backend error has been immediately returned to the client without retrying the request on the remaining backends.
* FEATURE: [vmauth](./vmauth.md): choose the backend with the minimum number of concurrently executed requests [among the configured backends](./vmauth.md#load-balancing) in a round-robin manner for serving the incoming requests. This allows spreading the load among backends more evenly, while improving the response time.
* FEATURE: [vmalert enterprise](./vmalert.md): add ability to read alerting and recording rules from S3, GCS or S3-compatible object storage. See [these docs](./vmalert.md#reading-rules-from-object-storage).
* FEATURE: [vmctl](./vmctl.md): automatically retry requests to remote storage if up to 5 errors occur during the data migration process. This should help continuing the data migration process on temporary errors. Previously `vmctl` was stopping after the first error. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3600).
* FEATURE: [MetricsQL](./MetricsQL.md): support optional 2nd argument `min`, `max` or `avg` for [rollup](./MetricsQL.md#rollup), [rollup_delta](./MetricsQL.md#rollup_delta), [rollup_deriv](./MetricsQL.md#rollup_deriv), [rollup_increase](./MetricsQL.md#rollup_increase), [rollup_rate](./MetricsQL.md#rollup_rate) and [rollup_scrape_interval](./MetricsQL.md#rollup_scrape_interval) function. If the second argument is passed, then the function returns only the selected aggregation type. This change can be useful for situations where only one type of rollup calculation is needed. For example, `rollup_rate(requests_total[1i], "max")` would return only the max increase rates for `requests_total` metric per each interval between adjacent points on the graph. See [this article](https://valyala.medium.com/why-irate-from-prometheus-doesnt-capture-spikes-45f9896d7832) for details.
* FEATURE: [MetricsQL](./MetricsQL.md): support optional 2nd argument `open`, `low`, `high`, `close` for [rollup_candlestick](./MetricsQL.md#rollup_candlestick) function. If the second argument is passed, then the function returns only the selected aggregation type.
* FEATURE: [MetricsQL](./MetricsQL.md): add [share(q)](./MetricsQL.md#share) aggregate function.
* FEATURE: [MetricsQL](./MetricsQL.md): add `mad_over_time(m[d])` function for calculating the [median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) over raw samples on the lookbehind window `d`. See [this feature request](https://github.com/prometheus/prometheus/issues/5514).
* FEATURE: [MetricsQL](./MetricsQL.md): add `range_mad(q)` function for calculating the [median absolute deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation) over points per each time series returned by `q`.
* FEATURE: [MetricsQL](./MetricsQL.md): add `range_zscore(q)` function for calculating [z-score](https://en.wikipedia.org/wiki/Standard_score) over points per each time series returned from `q`.
* FEATURE: [MetricsQL](./MetricsQL.md): add `range_trim_outliers(k, q)` function for dropping outliers located farther than `k*range_mad(q)` from the `range_median(q)`. This should help removing outliers during query time at [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3759).
* FEATURE: [MetricsQL](./MetricsQL.md): add `range_trim_zscore(z, q)` function for dropping outliers located farther than `z*range_stddev(q)` from `range_avg(q)`. This should help removing outliers during query time at [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3759).
* FEATURE: [vmui](./README.md#vmui): show `median` instead of `avg` in graph tooltip and line legend, since `median` is more tolerant against spikes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3706).
* FEATURE: add `-search.maxSeriesPerAggrFunc` command-line flag, which can be used for limiting the number of time series [MetricsQL aggregate functions](./MetricsQL.md#aggregate-functions) can return in a single query. This flag can be useful for preventing OOMs when [count_values](./MetricsQL.md#count_values) function is improperly used.
* FEATURE: [vmui](./README.md#vmui): small UX improvements for mobile view. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3707) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3848).
* FEATURE: add `-search.logQueryMemoryUsage` command-line flag for logging queries, which need more memory than specified by this command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3553). Thanks to @michal-kralik for the idea and the initial implementation.
* FEATURE: allow setting zero value for `-search.latencyOffset` command-line flag. This may be needed in [some cases](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2061#issuecomment-1299109836). Previously the minimum supported value for `-search.latencyOffset` command-line flag was `1s`.

* BUGFIX: [vmagent](./vmagent.md): immediately cancel in-flight scrape requests during configuration reload when [stream parsing mode](./vmagent.md#stream-parsing-mode) is disabled. Previously `vmagent` could wait for long time until all the in-flight requests are completed before reloading the configuration. This could significantly slow down configuration reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3747).
* BUGFIX: [vmagent](./vmagent.md): do not wait for 2 seconds after the first unsuccessful attempt to scrape the target before performing the next attempt. This should improve scrape speed when the target closes [http keep-alive connection](https://en.wikipedia.org/wiki/HTTP_persistent_connection) between scrapes. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3293) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3747) issues.
* BUGFIX: [vmagent](./vmagent.md): fix [Azure service discovery](./sd_configs.md#azure_sd_configs) inside [Azure Container App](https://learn.microsoft.com/en-us/azure/container-apps/overview). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3830). Thanks to @MattiasAng for the fix!
* BUGFIX: do not put auxiliary directories scheduled for removal into snapshots. This should prevent from `cannot create hard links from ...must-remove...` errors when making snapshots / backups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3858).
* BUGFIX: prevent from possible data ingestion slowdown and query performance slowdown during [background merges of big parts](./README.md#storage) on systems with small number of CPU cores (1 or 2 CPU cores). The issue has been introduced in [v1.85.0](./CHANGELOG.md#v1850) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3337). See also [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3790).
* BUGFIX: properly parse timestamps in milliseconds when [ingesting data via OpenTSDB telnet put protocol](./README.md#sending-data-via-telnet-put-protocol). Previously timestamps in milliseconds were mistakenly multiplied by 1000. Thanks to @Droxenator for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3810).
* BUGFIX: [MetricsQL](./MetricsQL.md): do not add extrapolated points outside the real points when using [interpolate()](./MetricsQL.md#interpolate) function. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3816).

# [v1.87.12](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.12)

Released at 2023-12-10

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade base docker image (Alpine) from 3.18.4 to 3.19.0. See [alpine 3.19.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.19.0-released.html).
* SECURITY: upgrade Go builder from Go1.21.4 to Go1.21.5. See [the list of issues addressed in Go1.21.5](https://github.com/golang/go/issues?q=milestone%3AGo1.21.5+label%3ACherryPickApproved).

* BUGFIX: [vmalert](./vmalert.md): sanitize label names before sending the alert notification to Alertmanager. Before, vmalert would send notifications with labels containing characters not supported by Alertmanager validator, resulting into validation errors like `msg="Failed to validate alerts" err="invalid label set: invalid name "foo.bar"`.
* BUGFIX: properly escape `<` character in responses returned via [`/federate`](./README.md#federation) endpoint. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431).

## [v1.87.11](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.11)

Released at 2023-11-14

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.21.3 to Go1.21.4. [the list of issues addressed in Go1.21.4](https://github.com/golang/go/issues?q=milestone%3AGo1.21.4+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): properly apply [relabeling](./vmagent.md#relabeling) with `regex`, which start and end with `.+` or `.*` and which contain alternate sub-regexps. For example, `.+;|;.+` or `.*foo|bar|baz.*`. Previously such regexps were improperly parsed, which could result in unexpected relabeling results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5297).
* BUGFIX: fix panic, which could occur when [query tracing](./README.md#query-tracing) is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5319).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md): log warning about switching to ReadOnly mode only on state change. Before, vmstorage would log this warning every 1s. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5159) for details.

## [v1.87.10](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.10)

Released at 2023-10-16

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.21.1 to Go1.21.3. See [the list of issues addressed in Go1.21.2](https://github.com/golang/go/issues?q=milestone%3AGo1.21.2+label%3ACherryPickApproved) and [the list of issues addressed in Go1.21.3](https://github.com/golang/go/issues?q=milestone%3AGo1.21.3+label%3ACherryPickApproved).

* BUGFIX: [storage](./Single-Server-VictoriaMetrics.md): prevent from livelock when [forced merge](./README.md#forced-merge) is called under high data ingestion. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4987).
* BUGFIX: [Graphite Render API](./README.md#graphite-render-api-usage): correctly return `null` instead of `Inf` in JSON query responses. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3783).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): fix ingestion via [multitenant url](./Cluster-VictoriaMetrics.md#multitenancy-via-labels) for opentsdbhttp. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5061). The bug has been introduced in [v1.87.8](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.8).
* BUGFIX: [vmagent](./vmagent.md): fix support of legacy DataDog agent, which adds trailing slashes to urls. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5078). Thanks to @maxb for spotting the issue.

## [v1.87.9](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.9)

Released at 2023-09-10

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.21.0 to Go1.21.1. See [the list of issues addressed in Go1.20.6](https://github.com/golang/go/issues?q=milestone%3AGo1.21.1+label%3ACherryPickApproved).

* BUGFIX: [vminsert enterprise](./enterprise.md): properly parse `/insert/multitenant/*` urls, which have been broken since [v1.93.2](#v1932). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4947).
* BUGFIX: properly build production armv5 binaries for `GOARCH=arm`. This has been broken after the upgrading of Go builder to Go1.21.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4965).
* BUGFIX: [vmselect](./Cluster-VictoriaMetrics.md): return `503 Service Unavailable` status code when [partial responses](./Cluster-VictoriaMetrics.md#cluster-availability) are denied and some of `vmstorage` nodes are temporarily unavailable. Previously `422 Unprocessable Entity` status code was mistakenly returned in this case, which could prevent from automatic recovery by re-sending the request to healthy cluster replica in another availability zone.
* BUGFIX: [vmalert](./vmalert.md): fix the bug when Group's `params` fields with multiple values were overriding each other instead of adding up. The bug was introduced in [this commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/eccecdf177115297fa1dc4d42d38e23de9a9f2cb) starting from [v1.87.7](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.7). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4908).

## [v1.87.8](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.8)

Released at 2023-09-01

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* BUGFIX: [build](./README.md): fix Docker builds for old Docker releases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4907).
* BUGFIX: vmselect: correctly handle requests with `/select/multitenant` prefix. Such requests must be rejected since vmselect does not support multitenancy endpoint. Previously, such requests were causing panic. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4910).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): properly check for [read-only state](./Cluster-VictoriaMetrics.md#readonly-mode) at `vmstorage`. Previously it wasn't properly checked, which could lead to increased resource usage and data ingestion slowdown when some of `vmstorage` nodes are in read-only mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870).
* BUGFIX: [vminsert](./Cluster-VictoriaMetrics.md): properly close broken vmstorage connection during [read-only state](./Cluster-VictoriaMetrics.md#readonly-mode) checks at `vmstorage`. Previously it wasn't properly closed, which prevents restoring `vmstorage` node from read-only mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870).
* BUGFIX: [vmstorage](./Cluster-VictoriaMetrics.md):  prevent from breaking `vmselect` -> `vmstorage` RPC communication when `vmstorage` returns an empty label name at `/api/v1/labels` request. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4932).
* BUGFIX: do not allow starting VictoriaMetrics components with improperly set boolean command-line flags in the form `-boolFlagName value`, since this leads to silent incomplete flags' parsing. This form should be replaced with `-boolFlagName=value`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4845).
* BUGFIX: properly replace `:` chars in label names with `_` when `-usePromCompatibleNaming` command-line flag is passed to `vmagent`, `vminsert` or single-node VictoriaMetrics. This addresses [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3113#issuecomment-1275077071).
* BUGFIX: [vmbackup](./vmbackup.md): correctly check if specified `-dst` belongs to specified `-storageDataPath`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4837).

## [v1.87.7](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.7)

Released at 2023-08-12

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.20.4 to Go1.21.0.
* SECURITY: upgrade base docker image (Alpine) from 3.18.2 to 3.18.3. See [alpine 3.18.3 release notes](https://alpinelinux.org/posts/Alpine-3.15.10-3.16.7-3.17.5-3.18.3-released.html).

* BUGFIX: vmselect: fix timestamp alignment for Prometheus querying API if time argument is less than 10m from the beginning of Unix epoch.
* BUGFIX: vminsert: fixed decoding of label values with slash when accepting data via [pushgateway protocol](./README.md#how-to-import-data-in-prometheus-exposition-format). This fixes Prometheus golang client compatibility. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4692).
* BUGFIX: [vmagent](./vmagent.md): properly validate scheme for `proxy_url` field at the scrape config. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4811) for details.
* BUGFIX: [vmagent](./vmagent.md): close HTTP connections to [service discovery](./sd_configs.md) servers when they are no longer needed. This should prevent from possible connection exhaustion in some cases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4724).
* BUGFIX: [vmagent](./vmagent.md): properly apply `if` filters during [relabeling](./vmagent.md#relabeling-enhancements). Previously the `if` filter could improperly work. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4806) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4816).
* BUGFIX: [vmagent](./vmagent.md): fix possible panic at shutdown when [stream aggregation](./stream-aggregation.md) is enabled. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4407) for details.
* BUGFIX: [vmagent](./vmagent.md): use local scrape timestamps for the scraped metrics unless `honor_timestamps: true` option is explicitly set at [scrape_config](./sd_configs.md#scrape_configs). This fixes gaps for metrics collected from [cadvisor](https://github.com/google/cadvisor) or similar exporters, which export metrics with invalid timestamps. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697#issuecomment-1654614799) for details.
* BUGFIX: [vmauth](./vmauth.md): Properly handle LOCAL command for proxy protocol. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3335#issuecomment-1569864108).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly return error from [/api/v1/query](./keyConcepts.md#instant-query) and [/api/v1/query_range](./keyConcepts.md#range-query) at `vmselect` when the `-search.maxSamplesPerQuery` or `-search.maxSamplesPerSeries` [limit](./Cluster-VictoriaMetrics.md#resource-usage-limits) is exceeded. Previously incomplete response could be returned without the error if `vmselect` runs with `-replicationFactor` greater than 1. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4472).
* BUGFIX: [vmalert](./vmalert.md): correctly calculate evaluation time for rules. Before, there was a low probability for discrepancy between actual time and rules evaluation time if evaluation interval was lower than the execution time for rules within the group.
* BUGFIX: [vmalert](./vmalert.md): reset evaluation timestamp after modifying group interval. Before, there could have latency on rule evaluation time.
* BUGFIX: [vmalert](./vmalert.md): Properly set datasource query params. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4340). Thanks to @gsakun for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4341).
* BUGFIX: [vmalert](./vmalert.md): Properly form path to static assets in WEB UI if `http.pathPrefix` set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4349).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` for groups with present name but absent `rules`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).
* BUGFIX: [vmctl](./vmctl.md): interrupt explore procedure in influx mode if vmctl found no numeric fields.
* BUGFIX: [vmctl](./vmctl.md): fix panic in case `--remote-read-filter-time-start` flag is not set for remote-read mode. This flag is now required to use remote-read mode. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4553).

## [v1.87.6](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.6)

Released at 2023-05-18

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.20.3 to Go1.20.4. See [the list of issues addressed in Go1.20.4](https://github.com/golang/go/issues?q=milestone%3AGo1.20.4+label%3ACherryPickApproved).
* SECURITY: upgrade base docker image (alpine) from 3.17.3 to 3.18.0. See [alpine 3.18.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.18.0-released.html).
* SECURITY: serve `/robots.txt` content to disallow indexing of the exposed instances by search engines. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4128) for details.

* BUGFIX: reduce the probability of sudden increase in the number of small parts on systems with small number of CPU cores.
* BUGFIX: reduce the possibility of increased CPU usage when data with timestamps older than one hour is ingested into VictoriaMetrics. This reduces spikes for the graph `sum(rate(vm_slow_per_day_index_inserts_total))`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4258).
* BUGFIX: do not ignore trailing empty field in CSV lines when [importing data in CSV format](./README.md#how-to-import-csv-data). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4048).
* BUGFIX: disallow `"` chars when parsing Prometheus label names, since they aren't allowed by [Prometheus text exposition format](https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-format-example). Previously this could result in silent incorrect parsing of incorrect Prometheus labels such as `foo{"bar"="baz"}` or `{foo:"bar",baz="aaa"}`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4284).
* BUGFIX: [MetricsQL](./MetricsQL.md): fix a panic when the duration in the query contains uppercase `M` suffix. Such a suffix isn't allowed to use in durations, since it clashes with `a million` suffix, e.g. it isn't clear whether `rate(metric[5M])` means rate over 5 minutes, 5 months or 5 million seconds. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3589) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4120) issues.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): prevent from possible panic when the number of vmstorage nodes increases when [automatic vmstorage discovery](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery) is enabled.
* BUGFIX: properly limit the number of [OpenTSDB HTTP](./README.md#sending-opentsdb-data-via-http-apiput-requests) concurrent requests specified via `-maxConcurrentInserts` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4204). Thanks to @zouxiang1993 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4208).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` and `/api/v1/alerts` API handlers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).
* BUGFIX: [vmagent](./vmagent.md): add `__meta_kubernetes_endpoints_name` label for all ports discovered from endpoint. Previously, ports not matched by `Service` did not have this label. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4154) for details. Thanks to @thunderbird86 for discovering and [fixing](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4253) the issue.
* BUGFIX: fix possible infinite loop during `indexdb` rotation when `-retentionTimezoneOffset` command-line flag is set and the local timezone is not UTC. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4207). Thanks to @faceair for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4206).
* BUGFIX: [vmauth](./vmauth.md): do not return invalid auth credentials in http response by default, since it may be logged by client. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4188).
* BUGFIX: [alerts-health](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts-health.yml): update threshold for `TooHighMemoryUsage` alert from 90% to 80%, since 90% is too high for production environments.
* BUGFIX: [vmagent](./vmagent.md): properly handle the `vm_promscrape_config_last_reload_successful` metric after config reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4260).
* BUGFIX: [stream aggregation](./stream-aggregation.md): fix bug with duplicated labels during stream aggregation via single-node VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4277).
* BUGFIX: [stream aggregation](./stream-aggregation.md): suppress `series after dedup` error message in logs when `-remoteWrite.streamAggr.dedupInterval` command-line flag is set at [vmagent](./vmagent.md) or when `-streamAggr.dedupInterval` command-line flag is set at [single-node VictoriaMetrics](./README.md).

## [v1.87.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.5)

Released at 2023-04-06

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade base docker image (alpine) from 3.17.2 to 3.17.3. See [alpine 3.17.3 release notes](https://alpinelinux.org/posts/Alpine-3.17.3-released.html).
* SECURITY: upgrade Go builder from Go1.20.2 to Go1.20.3. See [the list of issues addressed in Go1.20.3](https://github.com/golang/go/issues?q=milestone%3AGo1.20.3+label%3ACherryPickApproved).

* BUGFIX: [MetricsQL](./MetricsQL.md): properly convert [VictoriaMetrics historgram buckets](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) to Prometheus histogram buckets when VictoriaMetrics histogram contain zero buckets. Previously these buckets were ignored, and this could lead to missing Prometheus histogram buckets after the conversion. Thanks to @zklapow for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4021).
* BUGFIX: [vmagent](./vmagent.md): fix CPU and memory usage spikes when files pointed by [file_sd_config](./sd_configs.md#file_sd_configs) cannot be re-read. See [this_issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3989).
* BUGFIX: prevent unexpected merges on start-up when `-storage.minFreeDiskSpaceBytes` is set. See [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4023).
* BUGFIX: properly support comma-separated filters inside [retention filters](./README.md#retention-filters). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3915).
* BUGFIX: verify response code when fetching configuration files via HTTP. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4034).

## [v1.87.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.4)

Released at 2023-03-25

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* BUGFIX: prevent from slow [snapshot creating](./README.md#how-to-work-with-snapshots) under high data ingestion rate. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3551).
* BUGFIX: [vmauth](./vmauth.md):  suppress [proxy protocol](https://www.haproxy.org/download/2.3/doc/proxy-protocol.txt) parsing errors in case of `EOF`. Usually, the error is caused by health checks and is not a sign of an actual error.
* BUGFIX: [vmbackup](./vmbackup.md): fix snapshot not being deleted in case of error during backup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2055).
* BUGFIX: allow using dashes and dots in environment variables names referred in config files via `%{ENV-VAR.SYNTAX}`. See [these docs](./README.md#environment-variables) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3999).
* BUGFIX: return back query performance scalability on hosts with big number of CPU cores. The scalability has been reduced in [v1.86.0](./CHANGELOG.md#v1860). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3966).

## [v1.87.3](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.3)

Released at 2023-03-12

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade Go builder from Go1.20.1 to Go1.20.2. See [the list of issues addressed in Go1.20.2](https://github.com/golang/go/issues?q=milestone%3AGo1.20.2+label%3ACherryPickApproved).

* BUGFIX: vmstorage: fix a bug, which could lead to incomplete or empty results for heavy queries selecting tens of thousands of time series. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3946).
* BUGFIX: vmselect: reduce memory usage and CPU usage when performing heavy queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3692).
* BUGFIX: prevent from possible `invalid memory address or nil pointer dereference` panic during [background merge](./README.md#storage). The issue has been introduced at [v1.85.0](./CHANGELOG.md#v1850). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3897).
* BUGFIX: prevent from possible `SIGBUS` crash on ARM architectures (Raspberry Pi), which deny unaligned access to 8-byte words. Thanks to @oliverpool for narrowing down the issue and for [the initial attempt to fix it](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): always return `is_partial: true` in partial responses. Previously partial responses could be returned as non-partial in some cases.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly take into account `-rpc.disableCompression` command-line flag at `vmstorage`. It was ignored since [v1.78.0](./CHANGELOG.md#v1780). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3932).
* BUGFIX: [vmagent](./vmagent.md): do not register `vm_promscrape_config_*` metrics if `-promscrape.config` flag is not used. Previously those metrics were registered and never updated, which was confusing and could trigger false-positive alerts.
* BUGFIX: [vmctl](./vmctl.md): skip measurements with no fields when migrating data from influxdb. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3837).
* BUGFIX: [vmauth](./vmauth.md): fix `cannot serve http` panic when plain HTTP request is sent to `vmauth` configured to accept requests over [proxy protocol](https://www.haproxy.org/download/2.3/doc/proxy-protocol.txt)-encoded request (e.g. when `vmauth` runs with `-httpListenAddr.useProxyProtocol` command-line flag). The issue has been introduced at [v1.87.0](./CHANGELOG.md#v1870) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3335).

## [v1.87.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.2)

Released at 2023-02-24

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* SECURITY: upgrade base docker image (alpine) from 3.17.1 to 3.17.2. See [alpine 3.17.2 release notes](https://alpinelinux.org/posts/Alpine-3.17.2-released.html).
* SECURITY: upgrade Go builder from Go1.20.0 to Go1.20.1. See [the list of issues addressed in Go1.20.1](https://github.com/golang/go/issues?q=milestone%3AGo1.20.1+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): immediately cancel in-flight scrape requests during configuration reload when [stream parsing mode](./vmagent.md#stream-parsing-mode) is disabled. Previously `vmagent` could wait for long time until all the in-flight requests are completed before reloading the configuration. This could significantly slow down configuration reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3747).
* BUGFIX: [vmagent](./vmagent.md): do not wait for 2 seconds after the first unsuccessful attempt to scrape the target before performing the next attempt. This should improve scrape speed when the target closes [http keep-alive connection](https://en.wikipedia.org/wiki/HTTP_persistent_connection) between scrapes. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3293) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3747) issues.
* BUGFIX: [vmagent](./vmagent.md): fix [Azure service discovery](./sd_configs.md#azure_sd_configs) inside [Azure Container App](https://learn.microsoft.com/en-us/azure/container-apps/overview). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3830). Thanks to @MattiasAng for the fix!
* BUGFIX: do not put auxiliary directories scheduled for removal into snapshots. This should prevent from `cannot create hard links from ...must-remove...` errors when making snapshots / backups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3858).
* BUGFIX: prevent from possible data ingestion slowdown and query performance slowdown during [background merges of big parts](./README.md#storage) on systems with small number of CPU cores (1 or 2 CPU cores). The issue has been introduced in [v1.85.0](./CHANGELOG.md#v1850) when implementing [this feature](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3337). See also [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3790).
* BUGFIX: properly parse timestamps in milliseconds when [ingesting data via OpenTSDB telnet put protocol](./README.md#sending-data-via-telnet-put-protocol). Previously timestamps in milliseconds were mistakenly multiplied by 1000. Thanks to @Droxenator for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3810).
* BUGFIX: [MetricsQL](./MetricsQL.md): do not add extrapolated points outside the real points when using [interpolate()](./MetricsQL.md#interpolate) function. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3816).

## [v1.87.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.1)

Released at 2023-02-09

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* FEATURE: [vmalert](./vmalert.md): alerts state restore procedure was changed to become asynchronous. It doesn't block groups start anymore which significantly improves vmalert's startup time.
  This also means that `-remoteRead.ignoreRestoreErrors` command-line flag becomes deprecated now and will have no effect if configured.
  While previously state restore attempt was made for all the loaded alerting rules, now it is called only for alerts which became active after the first evaluation. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2608).
* FEATURE: [vmui](./README.md#vmui): optimize VMUI for use from smartphones and tablets. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3707).
* FEATURE: [vmui](./README.md#vmui): add ability to search tenants in the drop-down list for the tenant selector. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3792).
* FEATURE: [vmui](./README.md#vmui): add avg/min/max/last values to line legends and tooltips for graphs. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3706).
* FEATURE: [vmui](./README.md#vmui): hide the default `per-job resource usage` dashboard if there is a custom dashboard exists at the directory specified via `-vmui.customDashboardsPath` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3740).

* BUGFIX: [vmagent](./vmagent.md): fix panic in [HashiCorp Nomad service discovery](./sd_configs.md#nomad_sd_configs). Thanks to @mr-karan for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3784).
* BUGFIX: [vmalert](./vmalert.md): fix display of rules number per-group for groups with identical names in UI.
* BUGFIX: [vmalert](./vmalert.md): prevent disabling state updates tracking per rule via setting values < 1. The minimum number of update states to track is now set to 1.
* BUGFIX: [vmalert](./vmalert.md): properly update `debug` and `update_entries_limit` rule's params on config's hot-reload.
* BUGFIX: properly initialize the `vm_concurrent_insert_current` metric before exposing it. Previously this metric could be left uninitialized in some cases, e.g. its value was zero. This could lead to false alerts for the query `avg_over_time(vm_concurrent_insert_current[1m]) >= vm_concurrent_insert_capacity`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3761).
* BUGFIX: [vmagent](./vmagent.md): immediately cancel in-flight scrape requests during configuration reload when using [stream parsing mode](./vmagent.md#stream-parsing-mode). Previously `vmagent` could wait for long time until all the in-flight requests are completed before reloading the configuration. This could significantly slow down configuration reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3747).
* BUGFIX: [vmgateway](./vmgateway.md): do not validate JWT signature if no public keys are provided. Previously this could result in the `error setting up jwt verification` error.

## [v1.87.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.87.0)

Released at 2023-02-01

**v1.87.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.87.x line will be supported for at least 12 months since [v1.87.0](./CHANGELOG.md#v1870) release**

* FEATURE: [stream aggregation](./stream-aggregation.md): add the ability to [de-duplicate](./README.md#deduplication) input samples before aggregation via `-streamAggr.dedupInterval` and `-remoteWrite.streamAggr.dedupInterval` command-line options.
* FEATURE: [vmui](./README.md#vmui): add dark mode - it can be selected via `settings` menu in the top right corner. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3704).
* FEATURE: [vmui](./README.md#vmui): improve visual appearance of the top menu. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3678).
* FEATURE: [vmui](./README.md#vmui): embed fonts into binary instead of loading them from external sources. This allows using `vmui` in full from isolated networks without access to Internet. Thanks to @ScottKevill for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3696).
* FEATURE: [vmui](./README.md#vmui): add ability to switch between tenants by selecting the needed tenant in the drop-down list at the top right corner of the UI. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3673).
* FEATURE: [vmagent](./vmagent.md): reduce memory usage when sending stale markers for targets, which expose big number of metrics. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3668) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3675) issues.
* FEATURE: [vmagent](./vmagent.md): add `__meta_kubernetes_pod_container_id` meta-label to the targets discovered via [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs). This label has been added in Prometheus starting from `v2.42.0`. See [this feature request](https://github.com/prometheus/prometheus/issues/11843).
* FEATURE: [vmagent](./vmagent.md): add `__meta_azure_machine_size` meta-label to the targets discovered via [azure_sd_configs](./sd_configs.md#azure_sd_configs). This label has been added in Prometheus starting from `v2.42.0`. See [this pull request](https://github.com/prometheus/prometheus/pull/11650).
* FEATURE: [vmauth](./vmauth.md): allow limiting the number of concurrent requests sent to `vmauth` via `-maxConcurrentRequests` command-line flag. This allows controlling memory usage of `vmauth` and the resource usage of backends behind `vmauth`. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3346). Thanks to @dmitryk-dk for [the initial implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3486).
* FEATURE: allow using VictoriaMetrics components behind proxies, which communicate with the backend via [proxy protocol](https://www.haproxy.org/download/2.3/doc/proxy-protocol.txt). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3335). For example, [vmauth](./vmauth.md) accepts proxy protocol connections when it starts with `-httpListenAddr.useProxyProtocol` command-line flag.
* FEATURE: add `-internStringMaxLen` command-line flag, which can be used for fine-tuning RAM vs CPU usage in certain workloads. For example, if the stored time series contain long labels, then it may be useful reducing the `-internStringMaxLen` in order to reduce memory usage at the cost of increased CPU usage. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3692).
* FEATURE: provide GOARCH=386 binaries for single-node VictoriaMetrics, vmagent, vmalert, vmauth, vmbackup and vmrestore components at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3661). Thanks to @denisgolius for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3725).

* BUGFIX: fix a bug, which could prevent background merges for the previous partitions until restart if the storage didn't have enough disk space for final deduplication and down-sampling.
* BUGFIX: fix a bug, which could lead to increased CPU usage and disk IO usage when adding data to previous months and when the [deduplication](./README.md#deduplication) or [downsampling](./README.md#downsampling) is enabled. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3737).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): propagate all the timeout-related errors from `vmstorage` to `vmselect`. Previously some timeout errors weren't returned from `vmselect` to `vmstorage`. Instead, `vmstorage` could log the error and close the connection to `vmselect`, so `vmselect` was logging cryptic errors such as `cannot execute funcName="..." on vmstorage "...": EOF`.
* BUGFIX: [vmui](./README.md#vmui): add support for time zone selection for older versions of browsers. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3680).
* BUGFIX: [vmagent](./vmagent.md): update API version for [ec2_sd_configs](./sd_configs.md#ec2_sd_configs) to fix [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3700) with missing `__meta_ec2_availability_zone_id` attribute.
* BUGFIX: [vmagent](./vmagent.md): properly return `200 OK` HTTP status code when importing data via [Pushgateway protocol](./README.md#how-to-import-data-in-prometheus-exposition-format). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3636).
* BUGFIX: [vmagent](./vmagent.md): do not add `exported_` prefix to scraped metric names, which clash with the [automatically generated metric names](./vmagent.md#automatically-generated-metrics) if `honor_labels: true` option is set in the [scrape_config](./sd_configs.md#scrape_configs). See the [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3557) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3406) issues.
* BUGFIX: [vmauth](./vmauth.md): allow re-entering authorization info in the web browser if the entered info was incorrect. Previously it was non-trivial to do via the web browser, since `vmauth` was returning `400 Bad Request` instead of `401 Unauthorized` http response code.
* BUGFIX: [vmauth](./vmauth.md): always log the client address and the requested URL on proxying errors. Previously some errors could miss this information.
* BUGFIX: [vmbackup](./vmbackup.md): fix snapshot not being deleted after backup completion. This issue could result in unnecessary snapshots being stored, it is required to delete unnecessary snapshots manually. See the [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3735).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): fix panic on top-level vmselect nodes of [multi-level setup](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup) when the `-replicationFactor` flag is set and request contains `trace` query parameter. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3734).

## [v1.86.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.86.2)

Released at 2023-01-18

* SECURITY: [vmbackup](./vmbackup.md): do not expose basic auth passwords from `-snapshot.createURL` and `-snapshot.deleteURL` command-line flags in logs. Thanks to @toanju for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3672).

* FEATURE: [vmui](./README.md#vmui): add ability to show custom dashboards at vmui by specifying a path to a directory with dashboard config files via `-vmui.customDashboardsPath` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3322) and [these docs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards).
* FEATURE: [vmui](./README.md#vmui): apply the `step` globally to all the displayed graphs. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3574).
* FEATURE: [vmui](./README.md#vmui): improve the appearance of graph lines by using more visually distinct colors. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3571).

* BUGFIX: do not slow down concurrently executed queries during assisted merges, since assisted merges already prioritize data ingestion over queries. The probability of assisted merges has been increased starting from [v1.85.0](./CHANGELOG.md#v1850) because of internal refactoring. This could result in slowed down queries when there is a plenty of free CPU resources. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3647) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3641) issues.
* BUGFIX: reduce the increased CPU usage at `vmselect` to v1.85.3 level when processing heavy queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3641).
* BUGFIX: [retention filters](./README.md#retention-filters): fix `FATAL: cannot locate metric name for metricID=...: EOF` panic, which could occur when retention filters are enabled.
* BUGFIX: [vmagent](./vmagent.md): properly cancel in-flight service discovery requests for [consul_sd_configs](./sd_configs.md#consul_sd_configs) and [nomad_sd_configs](./sd_configs.md#nomad_sd_configs) when the service list changes. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3468).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): apply `filters` only to objects of the specified `role`. Previously filters were applied to all the objects, which could cause errors when different types of objects were used with filters that were not compatible with them. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3579).
* BUGFIX: [vmagent](./vmagent.md): suppress all the scrape errors when `-promscrape.suppressScrapeErrors` is enabled. Previously some scrape errors were logged even if `-promscrape.suppressScrapeErrors` flag was set.
* BUGFIX: [vmagent](./vmagent.md): consistently put the scrape url with scrape target labels to all error logs for failed scrapes. Previously some failed scrapes were logged without this information.
* BUGFIX: [vmagent](./vmagent.md): do not send stale markers to remote storage for series exceeding the configured [series limit](./vmagent.md#cardinality-limiter). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3660).
* BUGFIX: [vmagent](./vmagent.md): properly apply [series limit](./vmagent.md#cardinality-limiter) when [staleness tracking](./vmagent.md#prometheus-staleness-markers) is disabled.
* BUGFIX: [vmagent](./vmagent.md): reduce memory usage spikes when big number of scrape targets disappear at once. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3668). Thanks to @lzfhust for [the initial fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3669).
* BUGFIX: [Pushgateway import](./README.md#how-to-import-data-in-prometheus-exposition-format): properly return `200 OK` HTTP response code. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3636).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly parse `M` and `Mi` suffixes as `1e6` multipliers in `1M` and `1Mi` numeric constants. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3664). The issue has been introduced in [v1.86.0](./CHANGELOG.md#v1860).
* BUGFIX: [vmui](./README.md#vmui): properly display range query results at `Table` view. For example, `up[5m]` query now shows all the raw samples for the last 5 minutes for the `up` metric at the `Table` view. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3516).

## [v1.86.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.86.1)

Released at 2023-01-10

* BUGFIX: return correct query results over time series with gaps. The issue has been introduced in [v1.86.0](./CHANGELOG.md#v1860).
* BUGFIX: properly take into account the timeout passed by `vmselect` to `vmstorage` during query execution. This issue could result in the following error logs at `vmstorage` under load: `cannot process vmselect request: cannot execute "search_v7": couldn't start executing the request in 0.000 seconds, since -search.maxConcurrentRequests=... concurrent requests are already executed`. The issue has been introduced in [v1.86.0](./CHANGELOG.md#v1860).


## [v1.86.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.86.0)

Released at 2023-01-10

**It is recommended upgrading to [VictoriaMetrics v1.86.1](./CHANGELOG.md#v1861) because v1.86.0 contains a bug, which could lead to incorrect query results over time series with gaps.**

**Update note 1:** This release changes the logic behind `-maxConcurrentInserts` command-line flag. Previously this flag was limiting the number of concurrent connections established from clients, which send data to VictoriaMetrics. Some of these connections could be temporarily idle. Such connections do not take significant CPU and memory resources, so there is no need in limiting their count. The new logic takes into account only those connections, which **actively** ingest new data to VictoriaMetrics and to [vmagent](./vmagent.md). This means that the default `-maxConcurrentInserts` value should handle cases, which could require increasing the value in the previous releases. So it is recommended trying to remove the explicitly set `-maxConcurrentInserts` command-line flag after upgrading to this release and verifying whether this reduces CPU and memory usage.

**Update note 2:** The `vm_concurrent_addrows_current` and `vm_concurrent_addrows_capacity` metrics [exported](./Cluster-VictoriaMetrics.md#monitoring) by `vmstorage` are replaced with `vm_concurrent_insert_current` and `vm_concurrent_insert_capacity` metrics in order to be consistent with the corresponding metrics exported by `vminsert`. Please update queries in dahsboards and alerting rules with new metric names if old metric names are used there.

* FEATURE: [vmagent](./vmagent.md): add support for aggregation of incoming [samples](./keyConcepts.md#raw-samples) by time and by labels. See [these docs](./stream-aggregation.md) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3460).
* FEATURE: [vmagent](./vmagent.md): reduce memory usage when scraping big number of targets without the need to enable [stream parsing mode](./vmagent.md#stream-parsing-mode).
* FEATURE: [vmagent](./vmagent.md): add support for Prometheus-compatible target discovery for [HashiCorp Nomad](https://www.nomadproject.io/) services via [nomad_sd_configs](./sd_configs.md#nomad_sd_configs). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3367). Thanks to @mr-karan for [the implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3549).
* FEATURE: [vmagent](./vmagent.md): automatically pre-fetch `metric_relabel_configs` and the target labels when clicking on the `debug metrics relabeling` link at the `http://vmagent:8429/targets` page at the particular target. See [these docs](./vmagent.md#relabel-debug).
* FEATURE: [vmui](./README.md#vmui): add ability to explore metrics exported by a particular `job` / `instance`. See [these docs](./README.md#metrics-explorer) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3386).
* FEATURE: allow passing partial `RFC3339` date/time to `time`, `start` and `end` query args at [querying APIs](./README.md#prometheus-querying-api-usage) and [export APIs](./README.md#how-to-export-time-series). For example, `2022` is equivalent to `2022-01-01T00:00:00Z`, while `2022-01-30T14` is equivalent to `2022-01-30T14:00:00Z`. See [these docs](./README.md#timestamp-formats).
* FEATURE: [MetricsQL](./MetricsQL.md): allow using unicode letters in identifiers. For example, `температура{город="Київ"}` is a valid MetricsQL expression now. Previously every non-ascii letters should be escaped with `\` char when used inside MetricsQL expression: `\т\е\м\п\е\р\а\т\у\р\а{\г\о\р\о\д="Киев"}`. Now both expressions are equivalent. Thanks to @hzwwww for the [pull request](https://github.com/VictoriaMetrics/metricsql/pull/7).
* FEATURE: [relabeling](./vmagent.md#relabeling): add support for `keepequal` and `dropequal` relabeling actions, which are supported by Prometheus starting from [v2.41.0](https://github.com/prometheus/prometheus/releases/tag/v2.41.0). These relabeling actions are almost identical to `keep_if_equal` and `drop_if_equal` relabeling actions supported by VictoriaMetrics since `v1.38.0` - see [these docs](./vmagent.md#relabeling-enhancements) - so it is recommended sticking to `keep_if_equal` and `drop_if_equal` actions instead of switching to `keepequal` and `dropequal`.
* FEATURE: [csvimport](./README.md#how-to-import-csv-data): support empty values for imported metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3540).
* FEATURE: [vmalert](./vmalert.md): allow configuring the default number of stored rule's update states in memory via global `-rule.updateEntriesLimit` command-line flag or per-rule via rule's `update_entries_limit` configuration param. See [these docs](./vmalert.md#rules) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3556).
* FEATURE: improve the logic benhind `-maxConcurrentInserts` command-line flag. Previously this flag was limiting the number of concurrent connections from clients, which write data to VictoriaMetrics or [vmagent](./vmagent.md). Some of these connections could be idle for some time. These connections do not need significant amounts of CPU and memory, so there is no sense in limiting their count. The updated logic behind `-maxConcurrentInserts` limits the number of **active** insert requests, not counting idle connections.
* FEATURE: protect all the http endpoints with `-httpAuth.*` command-line flag. Previously endpoints protected by `-*AuthKey` command-line flags weren't protected by `-httpAuth.*`. This could complicate the proper security setup. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3060).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add `-maxConcurrentInserts` and `-insert.maxQueueDuration` command-line flags to `vmstorage`, so they could be tuned if needed in the same way as at `vminsert` nodes.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): limit the number of concurrently executed requests at `vmstorage` proportionally to the number of available CPU cores, since every request can saturate a single CPU core at `vmstorage`. Previously a single `vmstorage` could accept and start processing arbitrary number of concurrent requests received from big number of `vmselect` nodes. This could result in increased RAM, CPU and disk IO usage or event to out of memory crash at `vmstorage` side under high load. The limit can be fine-tuned if needed via `-search.maxConcurrentRequests` command-line flag at `vmstorage` according to [these docs](./Cluster-VictoriaMetrics.md#resource-usage-limits). `vmstorage` now [exposes](./Cluster-VictoriaMetrics.md#monitoring) the following additional metrics at `http://vmstorage:8482/metrics` page:
  - `vm_vmselect_concurrent_requests_capacity` - the maximum number of requests allowed to execute concurrently
  - `vm_vmselect_concurrent_requests_current` - the current number of concurrently executed requests
  - `vm_vmselect_concurrent_requests_limit_reached_total` - the total number of requests, which were put in the wait queue when `-search.maxConcurrentRequests` concurrent requests are being executed
  - `vm_vmselect_concurrent_requests_limit_timeout_total` - the total number of canceled requests because they were sitting in the wait queue for more than `-search.maxQueueDuration`

* BUGFIX: [vmui](./README.md#vmui): properly update the `step` value in url after the `step` input field has been manually changed. This allows preserving the proper `step` when copy-n-pasting the url to another instance of web browser. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3513).
* BUGFIX: [vmui](./README.md#vmui): properly update tooltip when quickly hovering multiple lines on the graph. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3530).
* BUGFIX: properly parse floating-point numbers without integer or fractional parts such as `.123` and `20.` during [data import](./README.md#how-to-import-time-series-data). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3544).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly parse durations with uppercase suffixes such as `10S`, `5MS`, `1W`, etc. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3589).
* BUGFIX: [vmagent](./vmagent.md): fix a panic during target discovery when `vmagent` runs with `-promscrape.dropOriginalLabels` command-line flag. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3580). The bug has been introduced in [v1.85.0](./CHANGELOG.md#v1850).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): properly encode `filters` field. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3579).
* BUGFIX: [vmagent](./vmagent.md): fix possible resource leak after hot reload of the updated [consul_sd_configs](./sd_configs.md#consul_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3468).
* BUGFIX: [vmagent](./vmagent.md): fix a panic in [gce_sd_configs](./sd_configs.md#gce_sd_configs) when the discovered instance has zero labels. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3624). The issue has been introduced in [v1.85.0](./CHANGELOG.md#v1850).
* BUGFIX: properly return label names starting from uppercase such as `CamelCaseLabel` from [/api/v1/labels](./url-examples.md#apiv1labels). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3566).
* BUGFIX: fix `opentsdb` HTTP endpoint not respecting `-httpAuth.*` flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3060)
* BUGFIX: consistently select the sample with the biggest value out of samples with identical timestamps during querying when the [deduplication](./README.md#deduplication) is enabled according to [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3333). Previously random samples could be selected during querying.

## Previous releases

See changes for older releases [here](./CHANGELOG_2022.md).
