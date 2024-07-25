---
sort: 103
weight: 103
title: CHANGELOG for the year 2022
menu:
  docs:
    parent: 'victoriametrics'
    weight: 103
aliases:
- /CHANGELOG_2022.html
---
## [v1.85.3](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.85.3)

Released at 2022-12-20

**Update note 1:** This and newer releases of VictoriaMetrics may return gaps for `rate(m[d])` queries on short time ranges if `[d]` lookbehind window is set explicitly. For example, `rate(http_requests_total[$__interval])`. This reduces confusion level when the user expects the needed results from the query with explicitly set lookbehind window. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3483). The previous gap filling behaviour can be restored by removing explicit lookbehind window `[d]` from the query, e.g. by substituting the `rate(m[d])` with `rate(m)`. See [these docs](./MetricsQL.md#implicit-query-conversions) for details.

* BUGFIX: fix `error when searching for TSIDs by metricIDs in the previous indexdb: EOF` error, which can occur during queries after unclean shutdown of VictoriaMetrics (e.g. via hardware reset, out of memory crash or `kill -9`). The error has been introduced in [v1.85.2](./CHANGELOG.md#v1852). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3515).
* BUGFIX: [VictoriaMetrics enterprise](./enterprise.md): expose proper values for `vm_downsampling_partitions_scheduled` and `vm_downsampling_partitions_scheduled_size_bytes` metrics, which were added at [v1.78.0](./CHANGELOG.md#v1780). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2612).
* BUGFIX: [MetricsQL](./MetricsQL.md): never extend explicitly set lookbehind window for [rate()](./MetricsQL.md#rate) function. This reduces the level of confusion when the user expects the needed results after explicitly seting the lookbehind window `[d]` in the query `rate(m[d])`. Previously VictoriaMetrics could silently extend the lookbehind window, so it covers at least two raw samples. Now this behavior works only if the lookbehind window in square brackets isn't set explicitly, e.g. in the case of `rate(m)`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3483) for details.
* BUGFIX: [vmagent](./vmagent.md): respect `-usePromCompatibleNaming` flag if no relabeling or extra labels were set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3511) for details.
* BUGFIX: [vmui](./README.md#vmui): fix the wrong legend when queries are hidden. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3512).
* BUGFIX: [vmui](./README.md#vmui): fix incorrect time selection after the timezone change. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3519).


## [v1.85.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.85.2)

Released at 2022-12-19

* FEATURE: support overriding of `-search.latencyOffset` value via URL param `latency_offset` when performing requests to [/api/v1/query](./keyConcepts.md#instant-query) and [/api/v1/query_range](./keyConcepts.md#range-query). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3481).
* FEATURE: allow changing field names in JSON logs if VictoriaMetrics components are started with `-loggerFormat=json` command-line flags. The field names can be changed with the `-loggerJSONFields` command-line flag. For example `-loggerJSONFields=ts:timestamp,msg:message` would rename `ts` and `msg` fields on the output JSON to `timestamp` and `message` fields. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2348). Thanks to @michal-kralik for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3488).
* FEATURE: [vmagent](./vmagent.md): expose `__meta_consul_tag_<tagname>` and `__meta_consul_tagpresent_<tagname>` labels for targets discovered via [consul_sd_configs](./sd_configs.md#consul_sd_configs). This simplifies converting [Consul service tags](https://developer.hashicorp.com/consul/docs/services/discovery/dns-overview) to target labels with a simple [relabeling rule](./vmagent.md#relabeling):

  ```yaml
  - action: labelmap
    regex: __meta_consul_tag_(.+)
  ```

  This resolves [this StackOverflow question](https://stackoverflow.com/questions/44339461/relabeling-in-prometheus).

* BUGFIX: properly return query results for time series, which stop receiving new samples after the rotation of `indexdb`. Previously such time series could be missing in query results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502). The issue has been introduced in [v1.83.0](./CHANGELOG.md#v1830).
* BUGFIX: allow specifying values bigger than 2GiB to the following command-line flag values on 32-bit architectures (`386` and `arm`): `-storage.minFreeDiskSpaceBytes` and `-remoteWrite.maxDiskUsagePerURL`. Previously values bigger than 2GiB were incorrectly truncated on these architectures.
* BUGFIX: [vmagent](./vmagent.md): stop dropping metric name by a mistake on the [/metric-relabel-debug](./vmagent.md#relabel-debug) page.


## [v1.85.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.85.1)

Released at 2022-12-14

**It is recommended upgrading to [VictoriaMetrics v1.85.2](./CHANGELOG.md#v1852) because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502), which may result in incomplete query results for historical time series.**

* FEATURE: [vmalert](./vmalert.md): support `$for` or `.For` template variables in alert's annotations. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3246).

* BUGFIX: [DataDog protocol parser](./README.md#how-to-send-data-from-datadog-agent): do not re-use `host` and `device` fields from the previously parsed messages if these fields are missing in the currently parsed message. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3432).
* BUGFIX: reduce CPU usage when the regex-based relabeling rules are applied to more than 100K unique Graphite metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3466). The issue was introduced in [v1.82.0](./CHANGELOG.md#v1820).
* BUGFIX: do not block [merges](./README.md#storage) of small parts by merges of big parts on hosts with small number of CPU cores. This issue could result in the increasing number of `storage/small` parts while big merge is in progress. This, in turn, could result in increased CPU usage and memory usage during querying, since queries need to inspect bigger number of small parts. The issue has been introduced in [v1.85.0](./CHANGELOG.md#v1850).
* BUGFIX: [vmbackup](./vmbackup.md): fix the `The source request body for synchronous copy is too large and exceeds the maximum permissible limit (256MB)` error when performing backups to Azure blob storage. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3477).


## [v1.85.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.85.0)

Released at 2022-12-11

**It is recommended upgrading to [VictoriaMetrics v1.85.2](./CHANGELOG.md#v1852) because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502), which may result in incomplete query results for historical time series.**

**Update note 1:** this release drops support for direct upgrade from VictoriaMetrics versions prior [v1.28.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.28.0). Please upgrade to `v1.84.0`, wait until `finished round 2 of background conversion` line is emitted to log by single-node VictoriaMetrics or by `vmstorage`, and then upgrade to newer releases.

**Update note 2:** this release splits `type="indexdb"` metrics into `type="indexdb/inmemory"` and `type="indexdb/file"` metrics. This may break old dashboards and alerting rules, which contain [label filter](./keyConcepts.md#filtering) on `{type="indexdb"}`. Such label filter must be substituted with `{type=~"indexdb.*"}`, so it matches `indexdb` from the previous releases and `indexdb/inmemory` + `indexdb/file` from new releases. It is recommended upgrading to the latest available dashboards and alerting rules mentioned in [these docs](./README.md#monitoring), since they already contain fixed label filters.

**Update note 3:** this release deprecates `relabel_debug` and `metric_relabel_debug` config options in [scrape_configs](./sd_configs.md#scrape_configs). The `-relabelDebug`, `-remoteWrite.relabelDebug` and `-remoteWrite.urlRelabelDebug` command-line options are also deprecated. Use more powerful target-level relabel debugging and metric-level relabel debugging instead as documented [here](./vmagent.md#relabel-debug).

* FEATURE: [vmagent](./vmagent.md): provide enhanced target-level and metric-level relabel debugging. See [these docs](./vmagent.md#relabel-debug) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3407).
* FEATURE: leave a sample with the biggest value for identical timestamps per each `-dedup.minScrapeInterval` discrete interval when the [deduplication](./README.md#deduplication) is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3333).
* FEATURE: add `-inmemoryDataFlushInterval` command-line flag, which can be used for controlling the frequency of in-memory data flush to disk. The data flush frequency can be reduced when VictoriaMetrics stores data to low-end flash device with limited number of write cycles (for example, on Raspberry PI). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3337).
* FEATURE: expose additional metrics for `indexdb` and `storage` parts stored in memory and for `indexdb` parts stored in files (see [storage docs](./README.md#storage) for technical details):
  * `vm_active_merges{type="storage/inmemory"}` - active merges for in-memory `storage` parts
  * `vm_active_merges{type="indexdb/inmemory"}` - active merges for in-memory `indexdb` parts
  * `vm_active_merges{type="indexdb/file"}` - active merges for file-based `indexdb` parts
  * `vm_merges_total{type="storage/inmemory"}` - the total merges for in-memory `storage` parts
  * `vm_merges_total{type="indexdb/inmemory"}` - the total merges for in-memory `indexdb` parts
  * `vm_merges_total{type="indexdb/file"}` - the total merges for file-based `indexdb` parts
  * `vm_rows_merged_total{type="storage/inmemory"}` - the total rows merged for in-memory `storage` parts
  * `vm_rows_merged_total{type="indexdb/inmemory"}` - the total rows merged for in-memory `indexdb` parts
  * `vm_rows_merged_total{type="indexdb/file"}` - the total rows merged for file-based `indexdb` parts
  * `vm_rows_deleted_total{type="storage/inmemory"}` - the total rows deleted for in-memory `storage` parts
  * `vm_assisted_merges_total{type="storage/inmemory"}` - the total number of assisted merges for in-memory `storage` parts
  * `vm_assisted_merges_total{type="indexdb/inmemory"}` - the total number of assisted merges for in-memory `indexdb` parts
  * `vm_parts{type="storage/inmemory"}` - the total number of in-memory `storage` parts
  * `vm_parts{type="indexdb/inmemory"}` - the total number of in-memory `indexdb` parts
  * `vm_parts{type="indexdb/file"}` - the total number of file-based `indexdb` parts
  * `vm_blocks{type="storage/inmemory"}` - the total number of in-memory `storage` blocks
  * `vm_blocks{type="indexdb/inmemory"}` - the total number of in-memory `indexdb` blocks
  * `vm_blocks{type="indexdb/file"}` - the total number of file-based `indexdb` blocks
  * `vm_data_size_bytes{type="storage/inmemory"}` - the total size of in-memory `storage` blocks
  * `vm_data_size_bytes{type="indexdb/inmemory"}` - the total size of in-memory `indexdb` blocks
  * `vm_data_size_bytes{type="indexdb/file"}` - the total size of file-based `indexdb` blocks
  * `vm_rows{type="storage/inmemory"}` - the total number of in-memory `storage` rows
  * `vm_rows{type="indexdb/inmemory"}` - the total number of in-memory `indexdb` rows
  * `vm_rows{type="indexdb/file"}` - the total number of file-based `indexdb` rows
* FEATURE: [DataDog parser](./README.md#how-to-send-data-from-datadog-agent): add `device` tag when it is passed in the `device` field is present in the `series` object of the input request. Thanks to @PerGon for the provided [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3431).
* FEATURE: [vmagent](./vmagent.md): improve [service discovery](./sd_configs.md) performance when discovering big number of targets (10K and more).
* FEATURE: [vmagent](./vmagent.md): allow using `series_limit` option for [limiting the number of series a single scrape target generates](./vmagent.md#cardinality-limiter) in [stream parsing mode](./vmagent.md#stream-parsing-mode). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3458).
* FEATURE: [vmagent](./vmagent.md): allow using `sample_limit` option for limiting the number of metrics a single scrape target can expose in every response sent over [stream parsing mode](./vmagent.md#stream-parsing-mode).
* FEATURE: [vmagent](./vmagent.md): add `exported_` prefix to metric names exported by scrape targets if these metric names clash with [automatically generated metrics](./vmagent.md#automatically-generated-metrics) such as `up`, `scrape_samples_scraped`, etc. This prevents from corruption of automatically generated metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3406).
* FEATURE: [vmagent](./vmagent.md): make the `host` label optional in [DataDog data ingestion protocol](./README.md#how-to-send-data-from-datadog-agent). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3432).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): improve error message when the requested path cannot be properly parsed, so users could identify the issue and properly fix the path. Now the error message links to [url format docs](./Cluster-VictoriaMetrics.md#url-format). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3402).
* FEATURE: [VictoriaMetrics enterprise cluster](./enterprise.md): add `-storageNode.discoveryInterval` command-line flag to `vmselect` and `vminsert` to control load on DNS servers when [automatic discovery of vmstorage nodes](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery) is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3417).
* FEATURE: [VictoriaMetrics enterprise cluster](./enterprise.md): allow reading and updating the list of `vmstorage` nodes at `vmselect` and `vminsert` nodes via file. See [automatic discovery of vmstorage](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery) for details.
* FEATURE: [vmalert](./vmalert.md): reduce memory and CPU usage by up to 50% on setups with thousands of recording/alerting groups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3464).
* FEATURE: [vmalert](./vmalert.md): add `-remoteWrite.sendTimeout` command-line flag, which allows configuring timeout for sending data to `-remoteWrite.url`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3408).
* FEATURE: [vmctl](./vmctl.md): add ability to migrate data between VictoriaMetrics clusters with automatic tenants discovery. See [these docs](./vmctl.md#cluster-to-cluster-migration-mode) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2930).
* FEATURE: [vmctl](./vmctl.md): add ability to copy data from sources via Prometheus `remote_read` protocol. See [these docs](./vmctl.md#migrating-data-by-remote-read-protocol). The related issues: [one](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3132) and [two](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1101).
* FEATURE: [vmui](./README.md#vmui): allow changing timezones for the requested data. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3075).
* FEATURE: [vmui](./README.md#vmui): provide fast path for hiding results for all the queries except the given one by clicking `eye` icon with `ctrl` key pressed. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3446).
* FEATURE: [MetricsQL](./MetricsQL.md): add `range_trim_spikes(phi, q)` function for trimming `phi` percent of the largest spikes per each time series returned by `q`. See [these docs](./MetricsQL.md#range_trim_spikes).
* FEATURE: [MetricsQL](./MetricsQL.md): allow passing `inf` arg into [limitk](./MetricsQL.md#limitk), [topk](./MetricsQL.md#topk), [bottomk](./MetricsQL.md) and other functions, which accept numeric arg, which limits the number of output time series. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3461).
* FEATURE: [vmgateway](./vmgateway.md): add support for JWT token signature verification. See [these docs](./vmgateway.md#jwt-signature-verification) for details.
* FEATURE: put the version of VictoriaMetrics in the first message of [query trace](./README.md#query-tracing). This should simplify debugging.

* BUGFIX: [vmagent](./vmagent.md): fix the `The request did not have a subscription or a valid tenant level resource provider` error when discovering Azure targets with [azure_sd_configs](./sd_configs.md#azure_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3247).
* BUGFIX: [vmalert](./vmalert.md): properly pass HTTP headers during the alert state restore procedure. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3418).
* BUGFIX: [vmalert](./vmalert.md): properly specify rule evaluation step during the [replay mode](./vmalert.md#rules-backfilling). The `step` value was previously overriden by `-datasource.queryStep` command-line flag.
* BUGFIX: [vmalert](./vmalert.md): properly return the error message from remote-write failures. Before, error was ignored and only `vmalert_remotewrite_errors_total` was incremented.
* BUGFIX: [vmui](./README.md#vmui): fix sticky tooltip sizing, which could prevent from closing the tooltip. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3427).
* BUGFIX: [vmui](./README.md#vmui): properly put multi-line queries in the url, so it could be copy-n-pasted and opened without issues in a new browser tab. Previously the url for multi-line query couldn't be opened. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3444).
* BUGFIX: [vmui](./README.md#vmui): correctly handle `up` and `down` keypresses when editing multi-line queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3445).

## [v1.84.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.84.0)

Released at 2022-11-25

**It is recommended upgrading to [VictoriaMetrics v1.85.2](./CHANGELOG.md#v1852) because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502), which may result in incomplete query results for historical time series.**

* FEATURE: add support for [Pushgateway data import format](https://github.com/prometheus/pushgateway#url) via `/api/v1/import/prometheus` url. See [these docs](./README.md#how-to-import-data-in-prometheus-exposition-format) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1415). Thanks to @PerGon for [the initial implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3360).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add `http://<vmselect>:8481/admin/tenants` API endpoint for returning a list of registered tenants. See [these docs](./Cluster-VictoriaMetrics.md#url-format) for details.
* FEATURE: [VictoriaMetrics enterprise](./enterprise.md): add `-storageNode.filter` command-line flag for filtering the [discovered vmstorage nodes](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery) with arbitrary regular expressions. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3353).
* FEATURE: [MetricsQL](./MetricsQL.md): allow using numeric values with `K`, `Ki`, `M`, `Mi`, `G`, `Gi`, `T` and `Ti` suffixes inside MetricsQL queries. For example `8Ki` equals to `8*1024`, while `8.2M` equals to `8.2*1000*1000`.
* FEATURE: [MetricsQL](./MetricsQL.md): add [range_normalize](./MetricsQL.md#range_normalize) function for normalizing multiple time series into `[0...1]` value range. This function is useful for correlation analysis of time series with distinct value ranges. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3167).
* FEATURE: [MetricsQL](./MetricsQL.md): add [range_linear_regression](./MetricsQL.md#range_linear_regression) function for calculating [simple linear regression](https://en.wikipedia.org/wiki/Simple_linear_regression) over the input time series on the selected time range. This function is useful for predictions and capacity planning. For example, `range_linear_regression(process_resident_memory_bytes)` can predict future memory usage based on the past memory usage.
* FEATURE: [MetricsQL](./MetricsQL.md): add [range_stddev](./MetricsQL.md#range_stddev) and [range_stdvar](./MetricsQL.md#range_stdvar) functions.
* FEATURE: [MetricsQL](./MetricsQL.md): optimize `expr1 op expr2` query when `expr1` returns an empty result. In this case there is no sense in executing `expr2` for `op` not equal to `or`, since the end result will be empty according to [PromQL series matching rules](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3349). Thanks to @jianglinjian for pointing to this case.
* FEATURE: [vmui](./README.md#vmui): add the ability to upload/paste JSON to investigate the trace. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3308) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3310).
* FEATURE: [vmui](./README.md#vmui): reduce JS bundle size from 200Kb to 100Kb. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3298).
* FEATURE: [vmui](./README.md#vmui): add the ability to hide results of a particular query by clicking the `eye` icon. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3359).
* FEATURE: [vmui](./README.md#vmui): add copy button to row on Table view. The button copies row in MetricQL format. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2815).
* FEATURE: [vmui](./README.md#vmui): add compact table view. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3241).
* FEATURE: [vmui](./README.md#vmui): add the ability to "stick" a tooltip on the chart by clicking on a data point. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3321) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3376)
* FEATURE: [vmui](./README.md#vmui): add the ability to set up series custom limits. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3297).
* FEATURE: [vmalert](./vmalert.md): add default alert list for vmalert's metrics. See [alerts-vmalert.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts-vmalert.yml).
* FEATURE: [vmagent](./vmagent.md): expose `vmagent_relabel_config_*`, `vm_relabel_config_*` and `vm_promscrape_config_*` metrics for tracking relabel and scrape configuration hot-reloads. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3345).

* BUGFIX: [MetricsQL](./MetricsQL.md): properly return an empty result from [limit_offset](./MetricsQL.md#limit_offset) if the `offset` arg exceeds the number of inner time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3312).
* BUGFIX: [vmagent](./vmagent.md): properly discover GCE zones when `filter` option is set at [gce_sd_configs](./sd_configs.md#gce_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3202).
* BUGFIX: [vmui](./README.md#vmui): properly display the requested graph on the requested time range when navigating from Prometheus URL in Grafana.
* BUGFIX: [vmui](./README.md#vmui): properly display wide tables. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3153).
* BUGFIX: reduce CPU usage spikes and memory usage spikes under high data ingestion rate introduced in [v1.83.0](./CHANGELOG.md#v1830). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3343).

## [v1.83.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.83.1)

Released at 2022-11-10

**It is recommended upgrading to [VictoriaMetrics v1.85.2](./CHANGELOG.md#v1852) because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502), which may result in incomplete query results for historical time series.**

* FEATURE: [vmagent](./vmagent.md): expose `__meta_consul_partition` label for targets discovered via [consul_sd_configs](./sd_configs.md#consul_sd_configs) in the same way as [Prometheus 2.40 does](https://github.com/prometheus/prometheus/pull/11482).
* FEATURE: [vmui](./README.md#vmui): show the [query trace](./README.md#query-tracing) in JSON view. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2814). Thanks to @michal-kralik for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3316).

* BUGFIX: [VictoriaMetrics enterprise](./enterprise.md): fix a panic at `vminsert` when the discovered list of `vmstorage` nodes is changed during [automatic vmstorage discovery](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3329).
* BUGFIX: properly register new time series in per-day inverted index if they were ingested during the last 10 seconds of the day. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309). Thanks to @lmarszal for the bugreport and for the [initial fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3320).
* BUGFIX: reduce the increased memory usage spikes for some workloads. The issue was introduced in [v1.83.0](./CHANGELOG.md#v1830).
* BUGFIX: properly accept [OpenTSDB telnet put lines](./README.md#sending-data-via-telnet-put-protocol) without tags without the need to specify the trailing whitespace. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3290).


## [v1.83.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.83.0)

Released at 2022-10-29

**It is recommended upgrading to [VictoriaMetrics v1.85.2](./CHANGELOG.md#v1852) because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3502), which may result in incomplete query results for historical time series.**

**Update note 1:** the `indexdb/tagFilters` cache type at [/metrics](./README.md#monitoring) has been renamed to `indexdb/tagFiltersToMetricIDs` in order to make its purpose more clear.

**Update note 2:** [vmalert](./vmalert.md): the `crlfEscape` [template function](./vmalert.md#template-functions) becomes obsolete starting from this release. It can be safely removed from alerting templates, since `\n` chars are properly escaped with other `*Escape` functions now. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3139) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890) issue for details.


* FEATURE: [VictoriaMetrics enterprise](./enterprise.md): add support for automatic `vmstorage` nodes discovering and updating at `vmselect` and `vminsert`. See [these docs](./Cluster-VictoriaMetrics.md#automatic-vmstorage-discovery).
* FEATURE: [VictoriaMetrics enterprise](./enterprise.md): allow configuring multiple retentions for distinct sets of time series. See [these docs](./README.md#retention-filters), [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/143) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/289) feature request.
* FEATURE: [VictoriaMetric cluster enterprise](./enterprise.md): add support for multiple retentions for distinct tenants - see [these docs](./Cluster-VictoriaMetrics.md#retention-filters) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/143) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/289) feature request.
* FEATURE: allow limiting memory usage on a per-query basis with `-search.maxMemoryPerQuery` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3203).
* FEATURE: allow referring environment variables inside command-line flags via `%{ENV_VAR}` syntax. For example, if `AUTH_KEY=top-secret` environment variable is set, then `-metricsAuthKey=%{AUTH_KEY}` command-line flag is automatically expanded to `-storageDataPath=top-secret` at VictoriaMetrics startup. See [these docs](./README.md#environment-variables) for details.
* FEATURE: allow referring environment variables inside other environment variables via `%{ENV_VAR}` syntax. For example, if `A=a-%{B}`, `B=b-%{C}` and `C=c` env vars are set, then VictoriaMetrics components automatically expand them to `A=a-b-c`, `B=b-c` and `C=c` on startup.
* FEATURE: [vmagent](./vmagent.md): drop all the labels with `__` prefix from discovered targets in the same way as Prometheus does according to [this article](https://www.robustperception.io/life-of-a-label/). Previously the following labels were available during [metric-level relabeling](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#metric_relabel_configs): `__address__`, `__scheme__`, `__metrics_path__`, `__scrape_interval__`, `__scrape_timeout__`, `__param_*`. Now these labels are available only during [target-level relabeling](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config). This should reduce CPU usage and memory usage for `vmagent` setups, which scrape big number of targets.
* FEATURE: [vmagent](./vmagent.md): improve the performance for metric-level [relabeling](./vmagent.md#relabeling), which can be applied via `metric_relabel_configs` section at [scrape_configs](./sd_configs.md#scrape_configs), via `-remoteWrite.relabelConfig` or via `-remoteWrite.urlRelabelConfig` command-line options.
* FEATURE: [vmagent](./vmagent.md): allow specifying full url in scrape target addresses (aka `__address__` label). This makes valid the following `-promscrape.config`:

  ```yaml
  scrape_configs:
  - job_name: abc
    metrics_path: /foo/bar
    scheme: https
    static_configs:
    - targets:
      # the following targets are scraped by the provided full urls
      - 'http://host1/metric/path1'
      - 'https://host2/metric/path2'
      - 'http://host3:1234/metric/path3?arg1=value1'
      # the following target is scraped by <scheme>://host4:1234<metrics_path>
      - host4:1234
  ```

  See [the corresponding issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3208).

* FEATURE: [vmagent](./vmagent.md): allow controlling [staleness tracking](./vmagent.md#prometheus-staleness-markers) on a per-[scrape_config](./sd_configs.md#scrape_configs) basis by specifying `no_stale_markers: true` or `no_stale_markers: false` option in the corresponding [scrape_config](./sd_configs.md#scrape_configs).
* FEATURE: [vmalert](./vmalert.md): add `strvalue` and `stripDomain` [template functions](./vmalert.md#template-functions) in order to improve compatibility with Prometheus.
* FEATURE: [vmalert](./vmalert.md): add `jsonEscape` and `htmlEscape` [template functions](./vmalert.md#template-functions).
* FEATURE: [vmui](./README.md#vmui): limit the number of plotted series. This should prevent from browser crashes or hangs when the query returns big number of time series. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3155).
* FEATURE: [vmui](./README.md#vmui): reduce memory usage when querying big number of time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3240).
* FEATURE: [vmui](./README.md#vmui): add responsive styles for small screens. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3239) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3256).
* FEATURE: log error if some environment variables referred at `-promscrape.config` via `%{ENV_VAR}` aren't found. This should prevent from silent using incorrect config files.
* FEATURE: immediately shut down VictoriaMetrics apps on the second SIGINT or SIGTERM signal if they couldn't be finished gracefully for some reason after receiving the first signal.
* FEATURE: improve the performance of [/api/v1/series](./url-examples.md#apiv1series) endpoint by eliminating loading of unused `TSID` data during the API call.
* FEATURE: [vmbackupmanager](./vmbackupmanager.md): add functionality for automated restore from backup. See [these docs](./vmbackupmanager.md#how-to-restore-backup-via-cli).

* BUGFIX: [MetricsQL](./MetricsQL.md): properly merge buckets with identical `le` values, but with different string representation of these values when calculating [histogram_quantile](./MetricsQL.md#histogram_quantile) and [histogram_share](./MetricsQL.md#histogram_share). For example, `http_request_duration_seconds_bucket{le="5"}` and `http_requests_duration_seconds_bucket{le="5.0"}`. Such buckets may be returned from distinct targets. Thanks to @647-coder for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3225).
* BUGFIX: [vmalert](./vmalert.md): change severity level for log messages about failed attempts for sending data to remote storage from `error` to `warn`. The message for about all failed send attempts remains at `error` severity level.
* BUGFIX: [vmalert](./vmalert.md): fix panic if `vmalert` runs with `-clusterMode` command-line flag in [multitenant mode](./vmalert.md#multitenancy). The issue has been introduced in [v1.82.0](./CHANGELOG.md#v1820).
* BUGFIX: [vmalert](./vmalert.md): properly escape string passed to `quotesEscape` [template function](./vmalert.md#template-functions), so it can be safely embedded into JSON string. This makes obsolete the `crlfEscape` function. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3139) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890) issue.
* BUGFIX: [vmagent](./vmagent.md): do not show invalid error message in Kubernetes service discovery: `cannot parse WatchEvent json response: EOF`. The invalid error message has been appeared in [v1.82.0](./CHANGELOG.md#v1820).
* BUGFIX: [vmagent](./vmagent.md): properly add `exported_` prefix to metric labels, which clashing with scrape target labels if `honor_labels: true` option isn't set in [scrape_config](./sd_configs.md#scrape_configs). Previously some `exported_` prefixes were missing in the resulting metric labels. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3278). The issue has been introduced in [v1.82.0](./CHANGELOG.md#v1820).
* BUGFIX: `vmselect`: expose missing metric `vm_cache_size_max_bytes{type="promql/rollupResult"}` . This metric is used for monitoring rollup cache usage with the query `vm_cache_size_bytes{type="promql/rollupResult"} / vm_cache_size_max_bytes{type="promql/rollupResult"}` in the same way as this is done for other cache types.

## [v1.82.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.82.1)

Released at 2022-10-14

* BUGFIX: [vmui](./README.md#vmui): automatically update graph, legend and url after the removal of query field. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3169) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3196#issuecomment-1269765205).
* BUGFIX: [vmalert](./vmalert.md): remove duplicate `alertname` JSON entry from generated alerts. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3053). Thanks to @Howie59 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3182)!
* BUGFIX: [vmalert](./vmalert.md): fix integration with Grafana via `-vmalert.proxyURL`, which has been broken in [v1.82.0](./CHANGELOG.md#v1820). See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/391).
* BUGFIX: [vmbackup](./vmbackup.md): set default region to `us-east-1` if `AWS_REGION` environment variable isn't set. The issue was introduced in [vmbackup v1.82.0](./CHANGELOG.md#v1820). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3224).
* BUGFIX: [vmbackupmanager](./vmbackupmanager.md): fix deletion of old backups at [Azure blob storage](https://azure.microsoft.com/en-us/products/storage/blobs/).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly apply regex filters when searching for time series. Previously unexpected time series could be returned from regex filter. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227). The issue was introduced in [v1.82.0](./CHANGELOG.md#v1820).
* BUGFIX: [vmagent](./vmagent.md): properly apply `if` section with regex filters. Previously unexpected metrics could be returned from `if` section. The issue was introduced in [v1.82.0](./CHANGELOG.md#v1820).

## [v1.82.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.82.0)

Released at 2022-10-07

**It isn't recommended to use VictoriaMetrics and vmagent v1.82.0 because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227), which may result in incorrect query results and [relabeling](./vmagent.md#relabeling) results. Upgrade to [v1.82.1](./CHANGELOG.md#v1821) instead.**

**Update note 1:** this release changes data format for [/api/v1/export/native](./README.md#how-to-export-data-in-native-format) in incompatible way, so it cannot be imported into older version of VictoriaMetrics via [/api/v1/import/native](./README.md#how-to-import-data-in-native-format).

**Update note 2:** [vmalert](./vmalert.md) changes default value for command-line flag `-datasource.queryStep` from `0s` to `5m`. The change supposed to improve reliability of the rules evaluation when evaluation interval is lower than scraping interval.

**Update note 3:** `vm_account_id` and `vm_project_id` labels must be passed to tcp-based `Graphite`, `InfluxDB` and `OpenTSDB` endpoints
at [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md) instead of undocumented
`VictoriaMetrics_AccountID` and `VictoriaMetrics_ProjectID` labels when writing samples to the needed tenant.
See [these docs](./Cluster-VictoriaMetrics.md#multitenancy-via-labels) for details.

* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): support specifying tenant ids via `vm_account_id` and `vm_project_id` labels. See [these docs](./Cluster-VictoriaMetrics.md#multitenancy-via-labels) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2970).
* FEATURE: [vmagent](./vmagent.md): improve [relabeling](./vmagent.md#relabeling) performance by up to 3x for non-trivial `regex` values such as `([^:]+):.+`, which can be used for extracting a `host` part from `host:port` label value.
* FEATURE: [MetricsQL](./MetricsQL.md): improve performance by up to 4x for queries containing non-trivial `regex` filters such as `{path=~"/foo/.+|/bar"}`.
* FEATURE: improve performance scalability on systems with many CPU cores for [/federate](./README.md#federation) and [/api/v1/export/...](./README.md#how-to-export-time-series) endpoints.
* FEATURE: sanitize metric names for data ingested via [DataDog protocol](./README.md#how-to-send-data-from-datadog-agent) according to [DataDog metric naming](https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics). The behaviour can be disabled by passing `-datadog.sanitizeMetricName=false` command-line flag. Thanks to @PerGon for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3105).
* FEATURE: add `-usePromCompatibleNaming` command-line flag to [vmagent](./vmagent.md), to single-node VictoriaMetrics and to `vminsert` component of VictoriaMetrics cluster. This flag can be used for normalizing the ingested metric names and label names to [Prometheus-compatible form](https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels). If this flag is set, then all the chars unsupported by Prometheus are replaced with `_` chars in metric names and labels of the ingested samples. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3113).
* FEATURE: accept whitespace in metric names and tags ingested via [Graphite plaintext protocol](./README.md#how-to-send-data-from-graphite-compatible-agents-such-as-statsd) according to [the specs](https://graphite.readthedocs.io/en/latest/tags.html). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3102).
* FEATURE: check the correctess of raw sample timestamps stored on disk when reading them. This reduces the probability of possible silent corruption of the data stored on disk. This should help [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2998) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3011).
* FEATURE: atomically delete directories with snapshots, parts and partitions at [storage level](./README.md#storage). Previously such directories can be left in partially deleted state when the deletion operation was interrupted by unclean shutdown. This may result in `cannot open file ...: no such file or directory` error on the next start. The probability of this error was quite high when NFS or EFS was used as persistent storage for VictoriaMetrics data. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3038).
* FEATURE: set the `start` arg to `end - 5 minutes` if isn't passed explicitly to [/api/v1/labels](./url-examples.md#apiv1labels) and [/api/v1/label/.../values](./url-examples.md#apiv1labelvalues). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3052).
* FEATURE: allow to define the minimum TLS version to use when accepting https requests to VictoriaMetrics components if `-tls` command-line flag is set. The minimum TLS version can be set via `-tlsMinVersion` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3090).
* FEATURE: [vmctl](./vmctl.md): add `vm-native-step-interval` command line flag for `vm-native` mode. New option allows splitting the import process into chunks by time interval. This helps migrating data sets with high churn rate and provides better control over the process. See [feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2733).
* FEATURE: [vmui](./README.md#vmui): add `top queries` tab, which shows various stats for recently executed queries. See [these docs](./README.md#top-queries) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2707).
* FEATURE: [vmui](./README.md#vmui): move the "Execute Query" and "Add Query" buttons below the query fields, change icon for remove query. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3101).
* FEATURE: [vmui](./README.md#vmui): set the maximum number of queries to 4, remove multi Y-axes, left one for all queries and dotted lines to indicate queries in the graph. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3169).
* FEATURE: [vmalert](./vmalert.md): add `debug` mode to the alerting rule settings for printing additional information into logs during evaluation. See `debug` param in [alerting rule config](./vmalert.md#alerting-rules).
* FEATURE: [vmalert](./vmalert.md): add experimental feature for displaying last 10 states of the rule (recording or alerting) evaluation. The state is available on the Rule page, which can be opened by clicking on `Details` link next to Rule's name on the `/groups` page.
* FEATURE: [vmalert](./vmalert.md): allow using extra labels in annotations. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3013).
* FEATURE: [vmalert](./vmalert.md): allow configuring authorization params per list of targets in vmalert's notifier config for `static_configs`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2690).
* FEATURE: [vmalert](./vmalert.md): allow using `{{$labels}}` for templating in command-line flag `-external.alert.source`. The change supposed to provide additional flexibility for generating alert's source link based on labels values.
* FEATURE: [vmalert](./vmalert.md): add `vm_account_id` and `vm_project_id` labels to results of alerting and recording rules if `-clusterMode` is enabled. This improves [multitenant support in vmalert](./vmalert.md#multitenancy).
* FEATURE: [vmagent](./vmagent.md): minimize the time needed for reading large responses from scrape targets in [stream parsing mode](./vmagent.md#stream-parsing-mode). This should reduce scrape durations for such targets as [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) running in a big Kubernetes cluster.
* FEATURE: [MetricsQL](./MetricsQL.md): add [sort_by_label_numeric](./MetricsQL.md#sort_by_label_numeric) and [sort_by_label_numeric_desc](./MetricsQL.md#sort_by_label_numeric_desc) functions for [numeric sort](https://www.gnu.org/software/coreutils/manual/html_node/Version-sort-is-not-the-same-as-numeric-sort.html) of input time series by the specified labels. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2938).
* FEATURE: [vmbackup](./vmbackup.md) and [vmrestore](./vmrestore.md): retry GCS operations for up to 3 minutes on temporary failures. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3147).
* FEATURE: [vmbackup](./vmbackup.md): add support for saving / restoring backups to / from [Azure blob storage](https://azure.microsoft.com/en-us/products/storage/blobs/). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1029).
* FEATURE: [vmbackupmanager](./vmbackupmanager.md): expose `vm_backup_in_flight` metric, which can be used for determining which backup types - latest, hourly, daily, weekly or monthly - are currently executed.
* FEATURE: [vmgateway](./vmgateway.md): add ability to extract JWT authorization token from non-standard HTTP header by passing it via `-auth.httpHeader` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3054).
* FEATURE: [vmagent](./vmagent.md): expose `__meta_ec2_region` label for [ec2_sd_config](./sd_configs.md#ec2_sd_configs) in the same way as [Prometheus 2.39 does](https://github.com/prometheus/prometheus/pull/11326).
* FEATURE: [vmagent](./vmagent.md): accept data ingestion requests via paths starting from `/prometheus` prefix in the same way as [VictoriaMetrics does](./README.md#how-to-import-time-series-data). For example, `vmagent` now accepts Prometheus `remote_write` data via both `/api/v1/write` and `/prometheus/api/v1/write`. This simplifies switching between single-node VictoriaMetrics and `vmagent`.
* FEATURE: [vmagent](./vmagent.md): add `external_labels` from `global` section at `-promscrape.config` after the [relabeling](./vmagent.md#relabeling) is applied to scraped metrics. This aligns with Prometheus behaviour. Previously the `external_labels` were added to scrape targets, so they could be modified during relabeling. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3137).
* FEATURE: [vmagent](./vmagent.md): allow specifying per-`-remoteWrite.url` limits for on-disk size for pending data via `-remoteWrite.maxDiskUsagePerURL` command-line flag. Thanks to @rbizos for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3071).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): log clear error when multiple identical `-storageNode` command-line flags are passed to `vmselect` or to `vminsert`. Previously these components were crashed with cryptic panic `metric ... is already registered` in this case. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3076).

* BUGFIX: do not export stale metrics via [/federate api](./README.md#federation) after the staleness markers. Previously such metrics were exported with `NaN` values. this could break some setups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3185).
* BUGFIX: export infinity numbers as `"Infinity"` strings at [/api/v1/export](./README.md#how-to-export-data-in-json-line-format), so they can be parsed by standard JSON parsers. Previously infinity numbers were exported as `Inf` values, which couldn't be parsed by standard JSON parsers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3161).
* BUGFIX: [vmauth](./vmauth.md): properly handle request paths ending with `/` such as `/vmui/`. Previously `vmui` was dropping the trailing `/`, which could prevent from using `vmui` via `vmauth`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1752).
* BUGFIX: [vmagent](./vmagent.md): properly encode query params for aws signed requests, use `%20` instead of `+` as api requires. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3171).
* BUGFIX: [vmagent](./vmagent.md): properly parse relabel config when regex ending with escaped `$`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3131).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate `rate_over_sum(m[d])` as `sum_over_time(m[d])/d`. Previously the `sum_over_time(m[d])` could be improperly divided by smaller than `d` time range. See [rate_over_sum() docs](./MetricsQL.md#rate_over_sum) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3045).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate `increase(m[d])` over slow-changing counters with values smaller than 100. Previously [increase](./MetricsQL.md#increase) could return unexpectedly big results in this case. See [the related issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3163).
* BUGFIX: [MetricsQL](./MetricsQL.md): ignore empty series when applying [limit_offset](./MetricsQL.md#limit_offset). It should improve queries with additional filters by value in expressions like `limit_offset(1,1, foo > 1)`.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate [quantiles_over_time](./MetricsQL.md#quantiles_over_time) when the lookbehind window contains only a single sample. Previously an empty result was incorrectly returned in this case.
* BUGFIX: [vmui](./README.md#vmui): fix `RangeError: Maximum call stack size exceeded` error when the query returns too many data points at `Table` view. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3092/files).
* BUGFIX: [vmui](./README.md#vmui): fix workaround for adding more queries via URL. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3169).
* BUGFIX: [vmalert](./vmalert.md): re-evaluate annotations per each alert evaluation. Previously, annotations were evaluated only on alert's value change. This could result in stale annotations in some cases described in [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3119).
* BUGFIX: prevent from excessive CPU usage when the storage enters [read-only mode](./Cluster-VictoriaMetrics.md#readonly-mode). The previous fix in [v1.81.0](./CHANGELOG.md#v1810) wasn't complete.
* BUGFIX: [vmalert](./vmalert.md): change default value for command-line flag `-datasource.queryStep` from `0s` to `5m`. Param `step` is added by vmalert to every rule evaluation request sent to datasource. Before this change, `step` was equal to group's evaluation interval by default. Param `step` for instant queries defines how far VM can look back for the last written data point. The change supposed to improve reliability of the rules evaluation when evaluation interval is lower than scraping interval.
* BUGFIX: properly calculate `vm_rows_scanned_per_query` histogram exported at `/metrics` page of `vmselect` and single-node VictoriaMetrics. Previously it could return misleadingly high numbers for [rollup functions](./MetricsQL.md#rollup-functions), which scan only a few samples on the provided lookbehind window in square brackets. For example, `increase(m[1d])` always scans only 2 rows (aka `raw samples`) per each returned time series.

## [v1.81.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.81.2)

Released at 2022-09-08

* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly calculate query results at `vmselect`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3067). The issue has been introduced in [v1.81.0](./CHANGELOG.md#v1810).

## [v1.81.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.81.1)

Released at 2022-09-02

**It isn't recommended to use VictoriaMetrics cluster v1.81.1 because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3067), which may result in incorrect query results. Upgrade to [v1.81.2](./CHANGELOG.md#v1812) instead.**

* FEATURE: [MetricsQL](./MetricsQL.md): evaluate `q1`, ..., `qN` in parallel when calculating `union(q1, .., qN)`. Previously [union](./MetricsQL.md#union) args were evaluated sequentially. This could result in lower than expected performance.

* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): fix potential panic at `vmselect` under high load, which has been introduced in [v1.81.0](./CHANGELOG.md#v1810). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3058).


## [v1.81.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.81.0)

**It isn't recommended to use VictoriaMetrics cluster v1.81.0 because of [the bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3058), which may result in `vmselect` crashes under high load. Upgrade to [v1.81.2](./CHANGELOG.md#v1812) instead.**

Released at 2022-08-31

**Update note 1:** [vmalert](./vmalert.md) by default hides values of `-remoteWrite.url`, `-remoteRead.url` and `-datasource.url` in logs and at `http://vmalert:8880/flags` for security reasons. See the corresponding SECURITY change in the Changelog below for additional info.

**Update note 2:** [vmalert](./vmalert.md) by default points alert source url to `/vmalert/alert?...` aka [web UI](./vmalert.md#web) instead of `/vmalert/api/v1/alert?...` aka JSON handler. The old behavior can be achieved by setting `-external.alert.source=vmalert/api/v1/alert?group_id={{.GroupID}}&alert_id={{.AlertID}}` command-line flag.

* SECURITY: [vmalert](./vmalert.md): do not expose `-remoteWrite.url`, `-remoteRead.url` and `-datasource.url` command-line flag values in logs and at `http://vmalert:8880/flags` page by default, since they may contain sensitive data such as auth keys. This aligns `vmalert` behaviour with [vmagent](./vmagent.md), which doesn't expose `-remoteWrite.url` command-line flag value in logs and at `http://vmagent:8429/flags` page by default. Specify `-remoteWrite.showURL`, `-remoteRead.showURL` and `-datasource.showURL` command-line flags for showing values for the corresponding `-*.url` flags in logs. Thanks to @mble for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2965).
* SECURITY: upgrade base docker image (alpine) from 3.16.1 to 3.16.2. See [alpine 3.16.2 release notes](https://alpinelinux.org/posts/Alpine-3.13.12-3.14.8-3.15.6-3.16.2-released.html).

* FEATURE: return shorter error messages to Grafana and to other clients requesting [/api/v1/query](./keyConcepts.md#instant-query) and [/api/v1/query_range](./keyConcepts.md#range-query) endpoints. This should simplify reading these errors by humans. The long error message with full context is still written to logs.
* FEATURE: add the ability to fine-tune the number of points, which can be generated per each matching time series during [subquery](./MetricsQL.md#subqueries) evaluation. This can be done with the `-search.maxPointsSubqueryPerTimeseries` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2922).
* FEATURE: [vmagent](./vmagent.md): improve the performance for relabeling rules with commonly used regular expressions in `regex` and `if` fields such as `some_string`, `prefix.*`, `prefix.+`, `foo|bar|baz`, `.*foo.*` and `.+foo.+`.
* FEATURE: [vmagent](./vmagent.md): reduce CPU usage when discovering big number of [Kubernetes targets](./sd_configs.md#kubernetes_sd_configs) with big number of labels and annotations.
* FEATURE: [vmagent](./vmagent.md): add ability to accept [multitenant](./Cluster-VictoriaMetrics.md#multitenancy) data via OpenTSDB `/api/put` protocol at `/insert/<tenantID>/opentsdb/api/put` http endpoint if [multitenant support](./vmagent.md#multitenancy) is enabled at `vmagent`. Thanks to @chengjianyun for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3015).
* FEATURE: [monitoring](./README.md#monitoring): expose `vm_hourly_series_limit_max_series`, `vm_hourly_series_limit_current_series`, `vm_daily_series_limit_max_series` and `vm_daily_series_limit_current_series` metrics when `-search.maxHourlySeries` or `-search.maxDailySeries` limits are set. This allows alerting when the number of unique series reaches the configured limits. See [these docs](./README.md#cardinality-limiter) for details.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): reduce the amounts of logging at `vmstorage` when `vmselect` connects/disconnects to `vmstorage`.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): improve performance for heavy queries on systems with many CPU cores.
* FEATURE: [vmagent](./vmagent.md): add ability to use `{{label_name}}` placeholders in the `replacement` option of relabeling rules. This simplifies constructing label values from multiple existing label values. See [these docs](./vmagent.md#relabeling-enhancements) for details.
* FEATURE: [vmagent](./vmagent.md): generate additional per-target metrics - `scrape_series_limit`, `scrape_series_current` and `scrape_series_limit_samples_dropped` if series limit is set according to [these docs](./vmagent.md#cardinality-limiter). This simplifies alerting on targets with the exceeded series limit. See [these docs](./vmagent.md#automatically-generated-metrics) for details on these metrics.
* FEATURE: [vmagent](./vmagent.md): add support for MX record types in [dns_sd_configs](./sd_configs.md#dns_sd_configs) in the same way as Prometheus 2.38 [does](https://github.com/prometheus/prometheus/pull/10099).
* FEATURE: [vmagent](./vmagent.md): add `__meta_kubernetes_service_port_number` meta-label for `role: service` in [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs) in the same way as Prometheus 2.38 [does](https://github.com/prometheus/prometheus/pull/11002).
* FEATURE: [vmagent](./vmagent.md): add `__meta_kubernetes_pod_container_image` meta-label for `role: pod` in [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs) in the same way as Prometheus 2.38 [does](https://github.com/prometheus/prometheus/pull/11034).
* FEATURE: [vmagent](./vmagent.md): retry HTTP requests after some wait time during service discovery and during target scrapes if the server returns 429 HTTP status code (aka `Too many requests`). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2940).
* FEATURE: [vmui](./README.md#vmui): add a legend in the top right corner for shortcut keys. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2813).
* FEATURE: [vmalert](./vmalert.md): add `toTime()` template function in the same way as Prometheus 2.38 [does](https://github.com/prometheus/prometheus/pull/10993). See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/#numbers).
* FEATURE: [vmalert](./vmalert.md): add `$alertID` and `$groupID` template variables. These variables may be used for templating annotations or `-external.alert.source` command-line flag. See the full list of supported variables [here](./vmalert.md#templating).
* FEATURE: [vmalert](./vmalert.md): add `$activeAt` template variable. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2999). See the full list of supported variables [here](./vmalert.md#templating). Thanks to @laixintao for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3000).
* FEATURE: [vmalert](./vmalert.md): point alert source to [vmalert's UI](./vmalert.md#web) at `/vmalert/alert?...` instead of JSON handler at `/vmalert/api/v1/alert?...`. This improves user experience. The old behavior can be achieved by setting `-external.alert.source=vmalert/api/v1/alert?group_id={{.GroupID}}&alert_id={{.AlertID}}` command-line flag.

* BUGFIX: prevent from excess CPU usage when the storage enters [read-only mode](./Cluster-VictoriaMetrics.md#readonly-mode).
* BUGFIX: improve performance for requests to [/api/v1/labels](./url-examples.md#apiv1labels) and [/api/v1/label/.../values](./url-examples.md#apiv1labelvalues) when the filter in the `match[]` query arg matches small number of time series. The performance for this case has been reduced in [v1.78.0](./CHANGELOG.md#v1780). See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1533) issues.
* BUGFIX: increase the default limit on the number of concurrent merges for small parts from 8 to 16. This should help resolving potential issues with heavy data ingestion. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2673#issuecomment-1218185978) from @lukepalmer .
* BUGFIX: [MetricsQL](./MetricsQL.md): fix panic when incorrect arg is passed as `phi` into [histogram_quantiles](./MetricsQL.md#histogram_quantiles) function. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3026).

## [v1.80.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.80.0)

Released at 2022-08-08

* FEATURE: [vmalert](./vmalert.md): allow configuring additional HTTP request headers for `-datasource.url`, `-remoteWrite.url` and `-remoteRead.url` via `-datasource.headers`, `-remoteWrite.headers` and `-remoteRead.headers` command-line flags. Additional HTTP request headers also can be set on group level via `headers` param - see [these docs](./vmalert.md#groups) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2860).
* FEATURE: [MetricsQL](./MetricsQL.md): execute left and right sides of certain operations in parallel. For example, `q1 or q2`, `aggr_func(q1) <op> q2`, `q1 <op> aggr_func(q1)`. This may improve query performance if VictoriaMetrics has enough free resources for parallel processing of both sides of the operation. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2886).
* FEATURE: [vmauth](./vmagent.md): allow multiple sections with duplicate `username` but with different `password` values at `-auth.config` file.
* FEATURE: add ability to push internal metrics (e.g. metrics exposed at `/metrics` page) to the configured remote storage from all the VictoriaMetrics components. See [these docs](./README.md#push-metrics).
* FEATURE: improve performance for heavy queries over big number of time series on systems with big number of CPU cores. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2896). Thanks to @zqyzyq for [the idea](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/b596ac3745314fcc170a14e3ded062971cf7ced2).
* FEATURE: improve performance for registering new time series in `indexdb` by up to 50%. Thanks to @ahfuzhang for [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2249).
* FEATURE: [vmagent](./vmagent.md): add ability to specify tenantID in target labels. In this case metrics from the given target are routed to the given `__tenant_id__`. See [these docs](./vmagent.md#multitenancy) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2943).
* FEATURE: [vmagent](./vmagent.md): add service discovery for [Yandex Cloud](https://cloud.yandex.com/en/). See [these docs](./sd_configs.md#yandexcloud_sd_configs) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1386).
* FEATURE: [vmui](./README.md#vmui). Zoom in the graph by selecting the needed time range in the same way Grafana does. Hold `ctrl` (or `cmd` on MacOS) in order to move the graph to the left/right. Hold `ctrl` (or `cmd` on MacOS) and scroll up/down in order to zoom in/out the area under the cursor. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2812).

* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): fix potential panic in [multi-level cluster setup](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup) when top-level `vmselect` is configured with `-replicationFactor` bigger than 1. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2961).
* BUGFIX: [vmagent](./vmagent.md): properly handle custom `endpoint` value in [ec2_sd_configs](./sd_configs.md#ec2_sd_configs). It was ignored since [v1.77.0](./CHANGELOG.md#v1770) because of a bug in the implementation of [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1287). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2917).
* BUGFIX: [vmagent](./vmagent.md): add missing `__meta_kubernetes_ingress_class_name` meta-label for `role: ingress` service discovery in Kubernetes. See [this commit from Prometheus](https://github.com/prometheus/prometheus/commit/7e65ad3e432bd2837c17e3e63e85dcbcc30f4a8a).
* BUGFIX: [vmagent](./vmagent.md): allow stale responses from Consul service discovery (aka [consul_sd_configs](./sd_configs.md#consul_sd_configs)) by default in the same way as Prometheus does. This should reduce load on Consul when discovering big number of targets. Stale responses can be disabled by specifying `allow_stale: false` option in `consul_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2940).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): properly set `__meta_dockerswarm_container_label_*` labels instead of `__meta_dockerswarm_task_label_*` labels as Prometheus does. See [this issue](https://github.com/prometheus/prometheus/issues/9187).
* BUGFIX: [vmagent](./vmagent.md): set `up` metric to `0` for partial scrapes in [stream parsing mode](./vmagent.md#stream-parsing-mode). Previously the `up` metric was set to `1` when at least a single metric has been scraped before the error. This aligns the behaviour of `vmselect` with Prometheus.
* BUGFIX: [vmagent](./vmagent.md): restart all the scrape jobs during [config reload](./vmagent.md#configuration-update) after `global` section is changed inside `-promscrape.config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2884).
* BUGFIX: [vmagent](./vmagent.md): properly assume role with AWS ECS credentials. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2875). Thanks to @transacid for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2876).
* BUGFIX: [vmagent](./vmagent.md): do not split regex in [relabeling rules](./vmagent.md#relabeling) into multiple lines if it contains groups. This fixes [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2928).
* BUGFIX: [MetricsQL](./MetricsQL.md): return series from `q1` if `q2` doesn't return matching time series in the query `q1 ifnot q2`. Previously series from `q1` weren't returned in this case.
* BUGFIX: [vmui](./README.md#vmui): properly show date picker at `Table` tab. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2874).
* BUGFIX: properly generate http redirects if `-http.pathPrefix` command-line flag is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2918).

## [v1.79.14](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.14)

Released at 2023-08-12

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade Go builder from Go1.20.4 to Go1.21.0.
* SECURITY: upgrade base docker image (Alpine) from 3.18.2 to 3.18.3. See [alpine 3.18.3 release notes](https://alpinelinux.org/posts/Alpine-3.15.10-3.16.7-3.17.5-3.18.3-released.html).

* BUGFIX: [vmagent](./vmagent.md): properly apply `if` filters during [relabeling](./vmagent.md#relabeling-enhancements). Previously the `if` filter could improperly work. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4806) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4816).
* BUGFIX: [vmalert](./vmalert.md): Properly form path to static assets in WEB UI if `http.pathPrefix` set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4349).
* BUGFIX: [vmalert](./vmalert.md): Properly set datasource query params. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4340). Thanks to @gsakun for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4341).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` and `/api/v1/alerts` API handlers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).
* BUGFIX: [vmalert](./vmalert.md): properly return empty slices instead of nil for `/api/v1/rules` for groups with present name but absent `rules`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221).

## [v1.79.13](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.13)

Released at 2023-05-18

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade Go builder from Go1.20.3 to Go1.20.4. See [the list of issues addressed in Go1.20.4](https://github.com/golang/go/issues?q=milestone%3AGo1.20.4+label%3ACherryPickApproved).
* SECURITY: upgrade base docker image (alpine) from 3.17.3 to 3.18.0. See [alpine 3.18.0 release notes](https://www.alpinelinux.org/posts/Alpine-3.18.0-released.html).
* SECURITY: serve `/robots.txt` content to disallow indexing of the exposed instances by search engines. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4128) for details.

## [v1.79.12](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.12)

Released at 2023-04-06

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade base docker image (alpine) from 3.17.2 to 3.17.3. See [alpine 3.17.3 release notes](https://alpinelinux.org/posts/Alpine-3.17.3-released.html).
* SECURITY: upgrade Go builder from Go1.20.2 to Go1.20.3. See [the list of issues addressed in Go1.20.3](https://github.com/golang/go/issues?q=milestone%3AGo1.20.3+label%3ACherryPickApproved).

* BUGFIX: [vmagent](./vmagent.md): fix CPU and memory usage spikes when files pointed by [file_sd_config](./sd_configs.md#file_sd_configs) cannot be re-read. See [this_issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3989).
* BUGFIX: prevent unexpected merges on start-up when `-storage.minFreeDiskSpaceBytes` is set. See [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4023).
* BUGFIX: verify response code when fetching configuration files via HTTP. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4034).

## [v1.79.11](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.11)

Released at 2023-03-12

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade Go builder from Go1.20.1 to Go1.20.2. See [the list of issues addressed in Go1.20.2](https://github.com/golang/go/issues?q=milestone%3AGo1.20.2+label%3ACherryPickApproved).

* BUGFIX: fix a bug, which could lead to incomplete or empty results for heavy queries selecting tens of thousands of time series. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3946).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly take into account `-rpc.disableCompression` command-line flag at `vmstorage`. It was ignored since [v1.78.0](./CHANGELOG.md#v1780). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3932).
* BUGFIX: prevent from possible `SIGBUS` crash on ARM architectures (Raspberry Pi), which deny unaligned access to 8-byte words. Thanks to @oliverpool for narrowing down the issue and for [the initial attempt to fix it](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927).

## [v1.79.10](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.10)

Released at 2023-02-27

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* BUGFIX: prevent from high CPU usage on the first UTC hour of the data. The issue has been introduced in [v1.79.5](./CHANGELOG.md#v1795) when fixing [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309).

## [v1.79.9](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.9)

Released at 2023-02-24

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade base docker image (alpine) from 3.17.1 to 3.17.2. See [alpine 3.17.2 release notes](https://alpinelinux.org/posts/Alpine-3.17.2-released.html).
* SECURITY: upgrade Go builder from Go1.20.0 to Go1.20.1. See [the list of issues addressed in Go1.20.1](https://github.com/golang/go/issues?q=milestone%3AGo1.20.1+label%3ACherryPickApproved).

* BUGFIX: properly parse timestamps in milliseconds when [ingesting data via OpenTSDB telnet put protocol](./README.md#sending-data-via-telnet-put-protocol). Previously timestamps in milliseconds were mistakenly multiplied by 1000. Thanks to @Droxenator for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3810).

## [v1.79.8](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.8)

Released at 2023-02-03

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* BUGFIX: fix a bug, which could prevent background merges for the previous partitions until restart if the storage didn't have enough disk space for final deduplication and down-sampling.
* BUGFIX: [vmagent](./vmagent.md): update API version for [ec2_sd_configs](./sd_configs.md#ec2_sd_configs) to fix [the issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3700) with missing `__meta_ec2_availability_zone_id` attribute.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): fix panic on top-level vmselect nodes of [multi-level setup](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup) when the `-replicationFactor` flag is set and request contains `trace` query parameter. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3734).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): apply `filters` only to objects of the specified `role`. Previously filters were applied to all the objects, which could cause errors when different types of objects were used with filters that were not compatible with them. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3579).
* BUGFIX: [vmagent](./vmagent.md): suppress all the scrape errors when `-promscrape.suppressScrapeErrors` is enabled. Previously some scrape errors were logged even if `-promscrape.suppressScrapeErrors` flag was set.
* BUGFIX: [vmagent](./vmagent.md): consistently put the scrape url with scrape target labels to all error logs for failed scrapes. Previously some failed scrapes were logged without this information.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly parse `M` and `Mi` suffixes as `1e6` multipliers in `1M` and `1Mi` numeric constants. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3664). The issue has been introduced in [v1.79.7](./CHANGELOG.md#v1797).

## [v1.79.7](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.7)

Released at 2023-01-10

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* BUGFIX: properly parse floating-point numbers without integer or fractional parts such as `.123` and `20.` during [data import](./README.md#how-to-import-time-series-data). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3544).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly parse durations with uppercase suffixes such as `10S`, `5MS`, `1W`, etc. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3589).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): properly encode `filters` field. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3579)
* BUGFIX: allow specifying values bigger than 2GiB to the following command-line flag values on 32-bit architectures (`386` and `arm`): `-storage.minFreeDiskSpaceBytes` and `-remoteWrite.maxDiskUsagePerURL`. Previously values bigger than 2GiB were incorrectly truncated on these architectures.
* BUGFIX: [VictoriaMetrics enterprise](./enterprise.md): expose proper values for `vm_downsampling_partitions_scheduled` and `vm_downsampling_partitions_scheduled_size_bytes` metrics, which were added at [v1.78.0](./CHANGELOG.md#v1780). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2612).
* BUGFIX: [DataDog protocol parser](./README.md#how-to-send-data-from-datadog-agent): do not re-use `host` and `device` fields from the previously parsed messages if these fields are missing in the currently parsed message. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3432).

## [v1.79.6](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.6)

Released at 2022-12-11

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: update Go builder from v1.19.3 to v1.19.4. See [the changelog](https://github.com/golang/go/issues?q=milestone%3AGo1.19.4+label%3ACherryPickApproved).
* SECURITY: update base Docker image for VictoriaMetrics components from Alpine 3.16.2 to Alpine v3.17.0. See [the changelog](https://alpinelinux.org/releases/).

* BUGFIX: [vmagent](./vmagent.md): fix the `The request did not have a subscription or a valid tenant level resource provider` error when discovering Azure targets with [azure_sd_configs](./sd_configs.md#azure_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3247).
* BUGFIX: [vmagent](./vmagent.md): properly discover GCE zones when `filter` option is set at [gce_sd_configs](./sd_configs.md#gce_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3202).
* BUGFIX: [vmalert](./vmalert.md): properly specify rule evaluation step during the [replay mode](./vmalert.md#rules-backfilling). The `step` value was previously overriden by `-datasource.queryStep` command-line flag.
* BUGFIX: [vmalert](./vmalert.md): properly return the error message from remote-write failures. Before, error was ignored and only `vmalert_remotewrite_errors_total` was incremented.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly return an empty result from [limit_offset](./MetricsQL.md#limit_offset) if the `offset` arg exceeds the number of inner time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3312).

## [v1.79.5](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.5)

Released at 2022-11-10

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

**Update note 1:** [vmalert](./vmalert.md): the `crlfEscape` [template function](./vmalert.md#template-functions) becomes obsolete starting from this release. It can be safely removed from alerting templates, since `\n` chars are properly escaped with other `*Escape` functions now. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3139) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890) issue for details.

* SECURITY: update Go builder to v1.19.3. This fixes [CVE-2022 security issue](https://github.com/golang/go/issues/56328). See [the changelog](https://github.com/golang/go/issues?q=milestone%3AGo1.19.3+label%3ACherryPickApproved).

* BUGFIX: properly register new time series in per-day inverted index if they were ingested during the last 10 seconds of the day. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309). Thanks to @lmarszal for the bugreport and for the [initial fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3320).
* BUGFIX: properly accept [OpenTSDB telnet put lines](./README.md#sending-data-via-telnet-put-protocol) without tags without the need to specify the trailing whitespace. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3290).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly merge buckets with identical `le` values, but with different string representation of these values when calculating [histogram_quantile](./MetricsQL.md#histogram_quantile) and [histogram_share](./MetricsQL.md#histogram_share). For example, `http_request_duration_seconds_bucket{le="5"}` and `http_requests_duration_seconds_bucket{le="5.0"}`. Such buckets may be returned from distinct targets. Thanks to @647-coder for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3225).
* BUGFIX: [vmalert](./vmalert.md): change severity level for log messages about failed attempts for sending data to remote storage from `error` to `warn`. The message for about all failed send attempts remains at `error` severity level.
* BUGFIX: [vmalert](./vmalert.md): properly escape string passed to `quotesEscape` [template function](./vmalert.md#template-functions), so it can be safely embedded into JSON string. This makes obsolete the `crlfEscape` function. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3139) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890) issue.
* BUGFIX: `vmselect`: expose missing metric `vm_cache_size_max_bytes{type="promql/rollupResult"}` . This metric is used for monitoring rollup cache usage with the query `vm_cache_size_bytes{type="promql/rollupResult"} / vm_cache_size_max_bytes{type="promql/rollupResult"}` in the same way as this is done for other cache types.

## [v1.79.4](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.4)

Released at 2022-10-07

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

**Update note 1:** [vmalert](./vmalert.md) changes default value for command-line flag `-datasource.queryStep` from `0s` to `5m`. The change supposed to improve reliability of the rules evaluation when evaluation interval is lower than scraping interval.

* FEATURE: expose `vmagent_remotewrite_queues` metric and use it in alerting rules in order to improve the detection of remote storage connection saturation. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2871).

* BUGFIX: do not export stale metrics via [/federate api](./README.md#federation) after the staleness markers. Previously such metrics were exported with `NaN` values. this could break some setups. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3185).
* BUGFIX: [vmauth](./vmauth.md): properly handle request paths ending with `/` such as `/vmui/`. Previously `vmui` was dropping the trailing `/`, which could prevent from using `vmui` via `vmauth`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1752).
* BUGFIX: [vmagent](./vmagent.md): properly encode query params for aws signed requests, use `%20` instead of `+` as api requires. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3171).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate `rate_over_sum(m[d])` as `sum_over_time(m[d])/d`. Previously the `sum_over_time(m[d])` could be improperly divided by smaller than `d` time range. See [rate_over_sum() docs](./MetricsQL.md#rate_over_sum) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3045).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate `increase(m[d])` over slow-changing counters with values smaller than 100. Previously [increase](./MetricsQL.md#increase) could return unexpectedly big results in this case. See [the related issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3163).
* BUGFIX: [MetricsQL](./MetricsQL.md): ignore empty series when applying [limit_offset](./MetricsQL.md#limit_offset). It should improve queries with additional filters by value in expressions like `limit_offset(1,1, foo > 1)`.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate [quantiles_over_time](./MetricsQL.md#quantiles_over_time) when the lookbehind window contains only a single sample. Previously an empty result was incorrectly returned in this case.
* BUGFIX: [vmui](./README.md#vmui): fix `RangeError: Maximum call stack size exceeded` error when the query returns too many data points at `Table` view. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3092/files).
* BUGFIX: [vmalert](./vmalert.md): re-evaluate annotations per each alert evaluation. Previously, annotations were evaluated only on alert's value change. This could result in stale annotations in some cases described in [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3119).
* BUGFIX: prevent from excessive CPU usage when the storage enters [read-only mode](./Cluster-VictoriaMetrics.md#readonly-mode). The previous fix in [v1.79.3](./CHANGELOG.md#v1793) wasn't complete.
* BUGFIX: [vmalert](./vmalert.md): change default value for command-line flag `-datasource.queryStep` from `0s` to `5m`. Param `step` is added by vmalert to every rule evaluation request sent to datasource. Before this change, `step` was equal to group's evaluation interval by default. Param `step` for instant queries defines how far VM can look back for the last written data point. The change supposed to improve reliability of the rules evaluation when evaluation interval is lower than scraping interval.
* BUGFIX: properly calculate `vm_rows_scanned_per_query` histogram exported at `/metrics` page of `vmselect` and single-node VictoriaMetrics. Previously it could return misleadingly high numbers for [rollup functions](./MetricsQL.md#rollup-functions), which scan only a few samples on the provided lookbehind window in square brackets. For example, `increase(m[1d])` always scans only 2 rows (aka `raw samples`) per each returned time series.

## [v1.79.3](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.3)

Released at 2022-08-30

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: [vmalert](./vmalert.md): do not expose `-remoteWrite.url`, `-remoteRead.url` and `-datasource.url` command-line flag values in logs and at `http://vmalert:8880/flags` page by default, since they may contain sensitive data such as auth keys. This aligns `vmalert` behaviour with [vmagent](./vmagent.md), which doesn't expose `-remoteWrite.url` command-line flag value in logs and at `http://vmagent:8429/flags` page by default. Specify `-remoteWrite.showURL`, `-remoteRead.showURL` and `-datasource.showURL` command-line flags for showing values for the corresponding `-*.url` flags in logs. Thanks to @mble for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2965).
* SECURITY: upgrade base docker image (alpine) from 3.16.1 to 3.16.2. See [alpine 3.16.2 release notes](https://alpinelinux.org/posts/Alpine-3.13.12-3.14.8-3.15.6-3.16.2-released.html).

* BUGFIX: prevent from excess CPU usage when the storage enters [read-only mode](./Cluster-VictoriaMetrics.md#readonly-mode).
* BUGFIX: improve performance for requests to [/api/v1/labels](./url-examples.md#apiv1labels) and [/api/v1/label/.../values](./url-examples.md#apiv1labelvalues) when the filter in the `match[]` query arg matches small number of time series. The performance for this case has been reduced in [v1.78.0](./CHANGELOG.md#v1780). See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1533) issues.
* BUGFIX: increase the default limit on the number of concurrent merges for small parts from 8 to 16. This should help resolving potential issues with heavy data ingestion. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2673#issuecomment-1218185978) from @lukepalmer .
* BUGFIX: [MetricsQL](./MetricsQL.md): fix panic when incorrect arg is passed as `phi` into [histogram_quantiles](./MetricsQL.md#histogram_quantiles) function. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3026).

## [v1.79.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.2)

Released at 2022-08-08

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): fix potential panic in [multi-level cluster setup](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup) when top-level `vmselect` is configured with `-replicationFactor` bigger than 1. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2961).
* BUGFIX: [vmagent](./vmagent.md): properly handle custom `endpoint` value in [ec2_sd_configs](./sd_configs.md#ec2_sd_configs). It was ignored since [v1.77.0](./CHANGELOG.md#v1770) because of a bug in the implementation of [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1287).
* BUGFIX: [vmagent](./vmagent.md): add missing `__meta_kubernetes_ingress_class_name` meta-label for `role: ingress` service discovery in Kubernetes. See [this commit from Prometheus](https://github.com/prometheus/prometheus/commit/7e65ad3e432bd2837c17e3e63e85dcbcc30f4a8a).
* BUGFIX: [vmagent](./vmagent.md): allow stale responses from Consul service discovery (aka [consul_sd_configs](./sd_configs.md#consul_sd_configs)) by default in the same way as Prometheus does. This should reduce load on Consul when discovering big number of targets. Stale responses can be disabled by specifying `allow_stale: false` option in `consul_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2940).
* BUGFIX: [vmagent](./vmagent.md): [dockerswarm_sd_configs](./sd_configs.md#dockerswarm_sd_configs): properly set `__meta_dockerswarm_container_label_*` labels instead of `__meta_dockerswarm_task_label_*` labels as Prometheus does. See [this issue](https://github.com/prometheus/prometheus/issues/9187).
* BUGFIX: [vmagent](./vmagent.md): set `up` metric to `0` for partial scrapes in [stream parsing mode](./vmagent.md#stream-parsing-mode). Previously the `up` metric was set to `1` when at least a single metric has been scraped before the error. This aligns the behaviour of `vmselect` with Prometheus.
* BUGFIX: [vmagent](./vmagent.md): restart all the scrape jobs during [config reload](./vmagent.md#configuration-update) after `global` section is changed inside `-promscrape.config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2884).
* BUGFIX: [vmagent](./vmagent.md): properly assume role with AWS ECS credentials. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2875). Thanks to @transacid for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2876).
* BUGFIX: [vmagent](./vmagent.md): do not split regex in [relabeling rules](./vmagent.md#relabeling) into multiple lines if it contains groups. This fixes [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2928).
* BUGFIX: [MetricsQL](./MetricsQL.md): return series from `q1` if `q2` doesn't return matching time series in the query `q1 ifnot q2`. Previously series from `q1` weren't returned in this case.
* BUGFIX: [vmui](./README.md#vmui): properly show date picker at `Table` tab. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2874).
* BUGFIX: properly generate http redirects if `-http.pathPrefix` command-line flag is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2918).


## [v1.79.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.1)

Released at 2022-08-02

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

* SECURITY: upgrade base docker image (alpine) from 3.16.0 to 3.16.1 . See [alpine 3.16.1 release notes](https://alpinelinux.org/posts/Alpine-3.16.1-released.html).


## [v1.79.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.79.0)

Released at 2022-07-14

**v1.79.x is a line of [LTS releases](./LTS-releases.md). It contains important up-to-date bugfixes.
The v1.79.x line will be supported for at least 12 months since [v1.79.0](./CHANGELOG.md#v1790) release**

**Update note 1:** this release introduces backwards-incompatible changes to `vm_partial_results_total` metric by changing its labels to be consistent with `vm_requests_total` metric. If you use alerting rules or Grafana dashboards, which rely on this metric, then they must be updated. The official dashboards for VictoriaMetrics don't use this metric.

**Update note 2:** [vmalert](./vmalert.md) adds `/vmalert/` prefix to [web urls](./vmalert.md#web) according to [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2825). This may affect `vmalert` instances with non-empty `-http.pathPrefix` command-line flag. After the update, configuring this flag is no longer needed. Here's [why](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2799#issuecomment-1171392005).

**Update note 3:** this release introduces backwards-incompatible changes to communication protocol between `vmselect` and `vmstorage` nodes in cluster version of VictoriaMetrics because of added ability to query `vmselect` data from other `vmselect` nodes - see [these docs](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup), so read requests to `vmselect` will fail until the upgrade is complete. These errors will stop after all the `vmselect` and `vmstorage` nodes are updated to the new release. It is safe to downgrade to previous releases at any time.

**Update note 4:** this release removes support of deprecated in [1.70.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.70.0) param `extra_filter_labels` from [vmalert's](./vmalert.md) groups definition. This deprecated param was replaced with [params](./vmalert.md#url-params).

**Update note 5:** this release changes naming for published linux binaries at [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). Now names for binaries for all the supported platforms match the following template - `$(APP_NAME)-$(GOOS)-$(GOARCH)-$(VERSION).tar.gz`. For example, `victoria-metrics-linux-amd64-v1.79.0.tar.gz`. Previously linux binaries didn't have `$(GOOS)` part, e.g. they had the name `victoria-metrics-amd64-v1.79.0.tar.gz`. Please update automation scripts for upgrading VictoriaMetrics releases according to this change.

* FEATURE: [vmagent](./vmagent.md): add [azure_sd_configs](./sd_configs.md#azure_sd_configs) service discovery mechanism. It allows discovering Virtual Machines at [Azure Cloud](https://azure.microsoft.com/en-us/). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1364).
* FEATURE: [vmalert](./vmalert.md): deprecate alert's status link `/api/v1/<groupID>/<alertID>/status` in favour of `api/v1/alert?group_id=<group_id>&alert_id=<alert_id>"`. The old alert's status link is still supported, but will be removed in future releases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2825).
* FEATURE: [cluster version of VictoriaMetrics](./Cluster-VictoriaMetrics.md): add support for querying lower-level `vmselect` nodes from upper-level `vmselect` nodes. This makes possible to build multi-level cluster setups for global querying view and HA purposes without the need to use [Promxy](https://github.com/jacksontj/promxy). See [these docs](./Cluster-VictoriaMetrics.md#multi-level-cluster-setup) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2778).
* FEATURE: add `-search.setLookbackToStep` command-line flag, which enables InfluxDB-like gap filling during querying. See [these docs](./guides/migrate-from-influx.md) for details.
* FEATURE: [vmui](./README.md#vmui): add an UI for [query tracing](./README.md#query-tracing). It can be enabled by clicking `trace query` checkbox and re-running the query. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2703).
* FEATURE: [vmagent](./vmagent.md): add `-remoteWrite.headers` command-line option for specifying optional HTTP headers to send to the configured `-remoteWrite.url`. For example, `-remoteWrite.headers='Foo:Bar^^Baz:x'` would send `Foo: Bar` and `Baz: x` HTTP headers with every request to `-remoteWrite.url`. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2805).
* FEATURE: [vmagent](./vmagent.md): push per-target `scrape_samples_limit` metric to the configured `-remoteWrite.url` if `sample_limit` option is set for this target in [scrape_configs](./sd_configs.md#scrape_configs). See [this feature request](https://github.com/VictoriaMetrics/operator/issues/497).
* FEATURE: [vmagent](./vmagent.md): attach node-level labels to [kubernetes_sd_config](./sd_configs.md#kubernetes_sd_configs) targets if `attach_metadata: {"node": true}` is set for `role: endpoints` and `role: endpointslice`. This is a feature backport from Prometheus 2.37 - see [this pull request](https://github.com/prometheus/prometheus/pull/10759).
* FEATURE: [vmagent](./vmagent.md): add ability to specify additional HTTP headers to send to scrape targets via `headers` section in `scrape_configs`. This can be used when the scrape target requires custom authorization and authentication like in [this stackoverflow question](https://stackoverflow.com/questions/66032498/prometheus-scrape-metric-with-custom-header). For example, the following config instructs sending `My-Auth: top-secret` and `TenantID: FooBar` headers with each request to `http://host123:8080/metrics`:

```yaml
scrape_configs:
- job_name: foo
  headers:
  - "My-Auth: top-secret"
  - "TenantID: FooBar"
  static_configs:
  - targets: ["host123:8080"]
```

* FEATURE: add ability to pass `limit` query arg to `api/v1/series` endpoint. This can be used if only a sample of up to `limit` series must be returned from the endpoint. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2841) and [these docs](./README.md#prometheus-querying-api-enhancements).
* FEATURE: [query tracing](./Single-Server-VictoriaMetrics.md#query-tracing): show timestamps in query traces in human-readable format (aka `RFC3339` in UTC timezone) instead of milliseconds since Unix epoch. For example, `2022-06-27T10:32:54.506Z` instead of `1656325974506`. This improves traces' readability.
* FEATURE: improve performance of [/api/v1/series](./url-examples.md#apiv1series) requests, which return big number of time series.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): improve query performance when [replication is enabled](./Cluster-VictoriaMetrics.md#replication-and-data-safety).
* FEATURE: [MetricsQL](./MetricsQL.md): properly handle partial counter resets in [remove_resets](./MetricsQL.md#remove_resets) function. Now `remove_resets(sum(m))` should returns the expected increasing line when some time series matching `m` disappear on the selected time range. Previously such a query would return horizontal line after the disappeared series.
* FEATURE: expose `vm_next_retention_seconds` metric at `http://victoriametrics:8428/metrics`, which shows the number of seconds left until the next `indexdb` rotation. Thanks to @guidao for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2863).
* FEATURE: expose additional histogram metrics at `http://victoriametrics:8428/metrics`, which may help understanding query workload:

  * `vm_rows_read_per_query` - the number of raw samples read per query.
  * `vm_rows_scanned_per_query` - the number of raw samples scanned per query. This number can exceed `vm_rows_read_per_query` if `step` query arg passed to [/api/v1/query_range](./keyConcepts.md#range-query) is smaller than the lookbehind window set in square brackets of [rollup function](./MetricsQL.md#rollup-functions). For example, if `increase(some_metric[1h])` is executed with the `step=5m`, then the same raw samples on a hour time range are scanned `1h/5m=12` times. See [this article](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986) for details.
  * `vm_rows_read_per_series` - the number of raw samples read per queried series.
  * `vm_series_read_per_query` - the number of series read per query.

* FEATURE: publish binaries for FreeBSD and OpenBSD at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
* FEATURE: [vmui](./README.md#vmui): allow selecting the needed columns at table view. This functionally may help when the selected time series contain many different labels. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2817) and [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2867).

* BUGFIX: consistently name binaries at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) in the form `$(APP_NAME)-$(GOOS)-$(GOARCH)-$(VERSION).tar.gz`. For example, `victoria-metrics-linux-amd64-v1.79.0.tar.gz`. Previously the `$(GOOS)` part was missing in binaries for Linux.
* BUGFIX: [vmalert](./vmalert.md): allow using `__name__` label (aka [metric name](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)) in alerting annotations. For example:

```sh
{{ $labels.__name__ }}: Too high connection number for "{{ $labels.instance }}
```

* BUGFIX: limit max memory occupied by the cache, which stores parsed regular expressions. Previously too long regular expressions passed in [MetricsQL queries](./MetricsQL.md) could result in big amounts of used memory (e.g. multiple of gigabytes). Now the max cache size for parsed regexps is limited to a a few megabytes.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly handle partial counter resets when calculating [rate](./MetricsQL.md#rate), [irate](./MetricsQL.md#irate) and [increase](./MetricsQL.md#increase) functions. Previously these functions could return zero values after partial counter resets until the counter increases to the last value before partial counter reset. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2787).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly calculate [histogram_quantile](./MetricsQL.md#histogram_quantile) over Prometheus buckets with unexpected values. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2819).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly evaluate [timezone_offset](./MetricsQL.md#timezone_offset) function over time range covering time zone offset switches. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2771).
* BUGFIX: [vmagent](./vmagent.md): properly add service-level labels (`__meta_kubernetes_service_*`) to discovered targets for `role: endpointslice` in [kubernetes_sd_config](./sd_configs.md#kubernetes_sd_configs). Previously these labels were missing. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2823).
* BUGFIX: [vmagent](./vmagent.md): make sure that [stale markers](./vmagent.md#prometheus-staleness-markers) are generated with the actual timestamp when unsuccessful scrape occurs. This should prevent from possible time series overlap on scrape target restart in dynamic environments such as Kubernetes.
* BUGFIX: [vmagent](./vmagent.md): properly reload changed `-promscrape.config` file when `-promscrape.configCheckInterval` option is set. The changed config file wasn't reloaded in this case since [v1.69.0](./CHANGELOG_2021.md#v1690). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2786). Thanks to @ttyv for the fix.
* BUGFIX: [vmagent](./vmagent.md): properly set `Host` header during target scraping when `proxy_url` is set to http proxy. Previously the `Host` header was set to the proxy hostname instead of the target hostname. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2794).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): assume that the response is complete if `-search.denyPartialResponse` is enabled and up to `-replicationFactor - 1` `vmstorage` nodes are unavailable. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1767).
* BUGFIX: [vmselect](./README.md#vmselect): update `vm_partial_results_total` metric labels to be consistent with `vm_requests_total` labels.
* BUGFIX: accept tags without values when reading data in [DataDog format](./Single-Server-VictoriaMetrics.md#how-to-send-data-from-datadog-agent). Thanks to @PerGon for the [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2839).
* BUGFIX: [vmui](./README.md#vmui): properly pass the end of the selected time range to `time` query arg to [/api/v1/query](./keyConcepts.md#instant-query) when displaying the requested data in JSON and table views. Previously the `time` query arg wasn't set, so `/api/v1/query` was always returning query results for the current time regardless of the selected time range. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2781).
* BUGFIX: [vmui](./README.md#vmui): allow clicking on the suggestion from autocomplete list. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2804).
* BUGFIX: [vmui](./README.md#vmui): apply the selected time range in date picker only after clicking the `Apply` button. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2811).

## [v1.78.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.78.1)

Released at 2022-07-08

**Update notes:** it is recommended [clearing caches](./README.md#cache-removal) after the upgrade from [v1.78.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.78.0) in order to immediately fix the issue for newly ingested data. Otherwise the issue may exist for newly ingested data for up to a day after the upgrade.

* BUGFIX: properly register time series in per-day inverted index. Previously some series could miss registration in the per-day inverted index. This could result in missing time series during querying. The issue has been introduced in [v1.78.0](#v1780). See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2798) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2793) issues.

## [v1.78.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.78.0)

Released at 2022-06-20

**Warning (2022-07-03):** VictoriaMetrics v1.78.0 contains a bug, which may result in missing time series during queries. It is recommended upgrading to [v1.78.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.78.1), which fixes the bug.

**Update notes:** this release introduces backwards-incompatible changes to communication protocol between `vmselect` and `vmstorage` nodes in cluster version of VictoriaMetrics because of added [query tracing](./Single-Server-VictoriaMetrics.md#query-tracing), so read requests to `vmselect` will fail until the upgrade is complete. These errors will stop after all the `vmselect` and `vmstorage` nodes are updated to the new release. It is safe to downgrade to previous releases.

* SECURITY: add `-flagsAuthKey` command-line flag for protecting `/flags` endpoint from unauthorized access. Though this endpoint already hides values for command-line flags with `key` and `password` substrings in their names, other sensitive information could be exposed there. See [This issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2753).

* FEATURE: support query tracing, which allows determining bottlenecks during query processing. See [these docs](./Single-Server-VictoriaMetrics.md#query-tracing) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1403).
* FEATURE: [vmui](./README.md#vmui): add `cardinality` tab, which can help identifying the source of [high cardinality](./FAQ.md#what-is-high-cardinality) and [high churn rate](./FAQ.md#what-is-high-churn-rate) issues. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2233) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2730) feature requests and [these docs](./README.md#cardinality-explorer).
* FEATURE: [vmui](./README.md#vmui): small UX enhancements according to [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2638).
* FEATURE: allow overriding default limits for in-memory cache `indexdb/tagFilters` via flag `-storage.cacheSizeIndexDBTagFilters`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2663).
* FEATURE: add support of `lowercase` and `uppercase` relabeling actions in the same way as [Prometheus 2.36.0 does](https://github.com/prometheus/prometheus/releases/tag/v2.36.0). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2664).
* FEATURE: add ability to change the `indexdb` rotation timezone offset via `-retentionTimezoneOffset` command-line flag. Previously it was performed at 4am UTC time. This could lead to performance degradation in the middle of the day when VictoriaMetrics runs in time zones located too far from UTC. Thanks to @cnych for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574).
* FEATURE: limit the number of background merge threads on systems with big number of CPU cores by default. This increases the max size of parts, which can be created during background merge when `-storageDataPath` directory has limited free disk space. This may improve on-disk data compression efficiency and query performance. The limits can be tuned if needed with `-smallMergeConcurrency` and `-bigMergeConcurrency` command-line flags. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2673).
* FEATURE: accept optional `limit` query arg at [/api/v1/labels](./url-examples.md#apiv1labels) and [/api/v1/label/.../values](./url-examples.md#apiv1labelvalues) for limiting the number of sample entries returned from these endpoints. See [these docs](./README.md#prometheus-querying-api-enhancements).
* FEATURE: optimize performance for [/api/v1/labels](./url-examples.md#apiv1labels) and [/api/v1/label/.../values](./url-examples.md#apiv1labelvalues) endpoints when `match[]`, `extra_label` or `extra_filters[]` query args are passed to these endpoints. This should help with [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1533).
* FEATURE: [vmalert](./vmalert.md): support `limit` param per-group for limiting number of produced samples per each rule. Thanks to @Howie59 for [implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2676).
* FEATURE: [vmalert](./vmalert.md): remove dependency on Internet access at [web API pages](./vmalert.md#web). Previously the functionality and the layout of these pages was broken without Internet access. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2594).
* FEATURE: [vmalert](./vmalert.md): send alerts to the configured notifiers in parallel. Previously alerts were sent to notifiers sequentially. This could delay sending pending alerts when notifier blocks on the currently sent alert.
* FEATURE: [vmagent](./vmagent.md): implement the `http://vmagent:8429/service-discovery` page in the same way as Prometheus does. This page shows the original labels for all the discovered targets alongside the resulting labels after the relabeling. This simplifies service discovery debugging.
* FEATURE: [vmagent](./vmagent.md): remove dependency on Internet access at `http://vmagent:8429/targets` page. Previously the page layout was broken without Internet access. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2594).
* FEATURE: [vmagent](./vmagent.md): add support for `kubeconfig_file` option at [kubernetes_sd_configs](./sd_configs.md#kubernetes_sd_configs). It may be useful for Kubernetes monitoring by `vmagent` outside Kubernetes cluster. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1464).
* FEATURE: [vmagent](./vmagent.md): expose `/api/v1/status/config` endpoint in the same way as Prometheus does. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/api/#config).
* FEATURE: [vmagent](./vmagent.md): add `-promscrape.suppressScrapeErrorsDelay` command-line flag, which can be used for delaying and aggregating the logging of per-target scrape errors. This may reduce the amounts of logs when `vmagent` scrapes many unreliable targets. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2575). Thanks to @jelmd for [the initial implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2576).
* FEATURE: [vmagent](./vmagent.md): add `-promscrape.cluster.name` command-line flag, which allows proper data de-duplication when the same target is scraped from multiple [vmagent clusters](./vmagent.md#scraping-big-number-of-targets). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2679).
* FEATURE: [vmagent](./vmagent.md): add `action: graphite` relabeling rules optimized for extracting labels from Graphite-style metric names. See [these docs](./vmagent.md#graphite-relabeling) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2737).
* FEATURE: [VictoriaMetrics enterprise](./enterprise.md): expose `vm_downsampling_partitions_scheduled` and `vm_downsampling_partitions_scheduled_size_bytes` metrics, which can be used for tracking the progress of initial [downsampling](./README.md#downsampling) for historical data. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2612).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): do not spend up to 5 seconds when trying to connect to unavailable `vmstorage` nodes. This should improve query latency when some of `vmstorage` nodes aren't available. Expose `vm_tcpdialer_addr_available{addr="..."}` metric at `http://vmselect:8481/metrics` for determining whether the given `addr` is available for establishing new connections. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711#issuecomment-1160363187).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add `-vmstorageDialTimeout` command-line flags to `vmselect` and `vminsert` for tuning the maximum duration for connection establishing to `vmstorage` nodes. This should help resolving [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711).

* BUGFIX: support for data ingestion in [DataDog format](./Single-Server-VictoriaMetrics.md#how-to-send-data-from-datadog-agent) from legacy clients / agents. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2670). Thanks to @elProxy for the fix.
* BUGFIX: [vmagent](./vmagent.md): do not expose `vm_promscrape_service_discovery_duration_seconds_bucket` metric for unused service discovery types. This reduces the number of metrics exported at `http://vmagent:8429/metrics`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2671).
* BUGFIX: [vmalert](./vmalert.md): properly apply `alert_relabel_configs` relabeling rules to `-notifier.config` according to [these docs](./vmalert.md#notifier-configuration-file). Thanks to @spectvtor for [the bugfix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2633).
* BUGFIX: [vmalert](./vmalert.md): properly add `Content-Encoding: snappy`, `Content-Type: application/x-protobuf` and `X-Prometheus-Remote-Write-Version: 0.1.0` request headers when `vmalert` sends [evaluated recording rules' data](./vmalert.md#recording-rules) to `-remoteWrite.url`. These headers are needed by some remote storage systems in order to properly decode snappy-encoded request body. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2685) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2701) pull requests. Thanks to @manji-0 for th fix.
* BUGFIX: deny [background merge](https://valyala.medium.com/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282) when the storage enters read-only mode, e.g. when free disk space becomes lower than `-storage.minFreeDiskSpaceBytes`. Background merge needs additional disk space, so it could result in `no space left on device` errors. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2603).
* BUGFIX: [vmui](./README.md#vmui): properly apply the selected time range when auto-refresh is enabled. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2693).
* BUGFIX: [vmui](./README.md#vmui): properly update the url with vmui state when new query is entered. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2692).
* BUGFIX: [Graphite render API](./README.md#graphite-render-api-usage): properly calculate sample timestamps when `moving*()` functions such as [movingAverage()](https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingAverage) are applied over [summarize()](https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.summarize).
* BUGFIX: limit the `end` query arg value to `+2 days` in the future at `/api/v1/*` endpoints, because VictoriaMetrics doesn't allow storing samples with timestamps bigger than +2 days in the future. This should help resolving [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2669).
* BUGFIX: properly register time series in per-day inverted index during the first hour after `indexdb` rotation. Previously this could lead to missing time series during querying if these time series stopped receiving new samples during the first hour after `indexdb` rotation. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2698).
* BUGFIX: do not register new series when `-storage.maxHourlySeries` or `-storage.maxDailySeries` limits were reached. Previously samples for new series weren't added to the database when the [cardinality limit](./README.md#cardinality-limiter) was reached, but series were still registered in the inverted index (aka `indexdb`). This could lead to unbound `indexdb` growth during [high churn rate](./FAQ.md#what-is-high-churn-rate).

## [v1.77.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.77.2)

Released at 2022-05-21

* FEATURE: [vmalert](./vmalert.md): support [reusable templates](https://prometheus.io/docs/prometheus/latest/configuration/template_examples/#defining-reusable-templates) for rules annotations. The path to the template files can be specified via `-rule.templates` flag. See more about this feature [here](./vmalert.md#reusable-templates). Thanks to @AndrewChubatiuk for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2532). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2510).
* FEATURE: [vmalert](./vmalert.md): expose `vmalert_iteration_interval_seconds` metric at `http://vmalert:8880/metrics`. This metric shows the configured per-group evaluation interval. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2618).
* FEATURE: [vmctl](./vmctl.md): add `influx-prometheus-mode` command-line flag, which allows to restore the original time series written from Prometheus into InfluxDB during data migration from InfluxDB to VictoriaMetrics. See [this feature request](https://github.com/VictoriaMetrics/vmctl/issues/8). Thanks to @mback2k for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2545).
* FEATURE: [vmagent](./vmagent.md): add ability to specify AWS service name when issuing requests to AWS api. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2605). Thanks to @transacid for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2604).

* BUGFIX: [vmagent](./vmagent.md): fix a bug, which could lead to incomplete discovery of scrape targets in Kubernetes (aka `kubernetes_sd_config`). the bug has been introduced in [v1.77.0](./CHANGELOG.md#v1770).
* BUGFIX: [vmalert](./vmalert.md): support `scalar` result type in response. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2607).
* BUGFIX: [vmalert](./vmalert.md): support strings in `humanize.*` template function in the same way as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2569).
* BUGFIX: [vmalert](./vmalert.md): proxy `/rules` requests to vmalert from Grafana's alerting UI. This removes errors in Grafana's UI for Grafana versions older than `8.5.*`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2583)
* BUGFIX: [vmalert](./vmalert.md): do not add `/api/v1/query` suffix to `-datasource.url` if `-remoteRead.disablePathAppend` command-line flag is set. Previously this flag was applied only to `-remoteRead.url`, which could confuse users.
* BUGFIX: [vmalert](./vmalert.md): prevent from possible resource leak on config update, which could lead to the slowdown of `vmalert` over time. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2577).
* BUGFIX: [MetricsQL](./MetricsQL.md): do not return values from [label_value()](./MetricsQL.md#label_value) function if the original time series has no values at the selected timestamps.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): limit the number of concurrently established connections from vmselect to vmstorage. This should prevent from potentially high spikes in the number of established connections after temporary slowdown in connection handshake procedure between vmselect and vmstorage because of spikes in workload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2552).
* BUGFIX: [vmctl](./vmctl.md): fix build for Solaris / SmartOS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1322#issuecomment-1120276146).

## [v1.77.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.77.1)

Released at 2022-05-07

* FEATURE: [vmagent](./vmagent.md): add ability to specify filters for Availability Zones in [ec2_sd_config](./sd_configs.md#ec2_sd_configs) via `az_filters` section. This section can contain AZ-specific set of filters in the same way as the existing `filters` section, which is used for filtering EC2 instances. The list of supported AZ-specific filters is available [here](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html).
* FEATURE: [vmagent](./vmagent.md): expose `vmagent_remotewrite_global_rows_pushed_before_relabel_total` and `vmagent_remotewrite_rows_pushed_after_relabel_total` metrics at `http://vmagent:8429/metrics`, which can be used for monitoring the rate of rows (aka samples) pushed to remote storage before and after the relabeling via `-remoteWrite.relabelConfig` and `-remoteWrite.urlRelabelConfig`. See [relabeling docs](./vmagent.md#relabeling) for details.
* FEATURE: [vmctl](./vmctl.md): add ability to skip `db` label during InfluxDB data import when `influx-skip-database-label` option is used. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2544). Thanks to @mback2k .

* BUGFIX: [vmagent](./vmagent.md): properly process passwords and secrets specified in the file pointed by `-promscrape.config` command-line flag. All the passwords and secrets were mistakenly replaced with `<secret>` string in `v1.77.0`. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2551) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2550).
* BUGFIX: [vmagent](./vmagent.md): rename `vmagent_remote_write_rate_limit_reached_total` metric to `vmagent_remotewrite_rate_limit_reached_total`, so its name is consistent with the rest of `vmagent_remotewrite_` metrics.
* BUGFIX: [vmagent](./vmagent.md): rename `promscrape_stale_samples_created_total` metric to `vm_promscrape_stale_samples_created_total`, so its name is consistent with the rest of `vm_promscrape_` metrics.
* BUGFIX: [vmctl](./vmctl.md): properly import InfluxDB measurements if they contain `db` tag. Previously this could result in incomplete import of measurement tags. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2536). Thanks to @mback2k for the bugfix.
* BUGFIX: [vmui](./README.md#vmui): do not reset the selected relative time range when entering new query. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2402#issuecomment-1115817302).
* BUGFIX: [vmbackup](./vmbackup.md): disallow writing backups to `-storageDataPath` directory, since this directory is managed solely by VictoriaMetrics or `vmstorage`. Other apps shouldn't write into this directory. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2503).
* BUGFIX: do not allow setting `-retentionPeriod` smaller than one day, since VictoriaMetrics doesn't support properly such small retention periods. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2496).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): do not drop samples routed to readonly `vmstorage` nodes if `-dropSamplesOnOverload` command-line flag is set. Try re-routing them to healthy `vmstorage` nodes instead. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2478).


## [v1.77.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.77.0)

Released at 2022-05-05

* FEATURE: [vmagent](./vmagent.md): add support for sending data to remote storage with AWS sigv4 authorization. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1287).
* FEATURE: [vmagent](./vmagent.md): allow filtering targets by target url and by target labels with [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) on `http://vmagent:8429/targets` page. This may be useful when `vmagent` scrapes big number of targets. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1796).
* FEATURE: [vmagent](./vmagent.md): reduce `-promscrape.config` reload duration when the config contains big number of jobs (aka [scrape_configs](./sd_configs.md#scrape_configs) sections) and only a few of them are changed. Previously all the jobs were restarted. Now only the jobs with changed configs are restarted. This should reduce the probability of data miss because of slow config reload. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2270).
* FEATURE: [vmagent](./vmagent.md): improve service discovery speed for big number of scrape targets. This should help when `vmagent` discovers big number of targets (e.g. thousands) in Kubernetes cluster. The service discovery speed now should scale with the number of CPU cores available to `vmagent`.
* FEATURE: [vmagent](./vmagent.md): add ability to attach node-level labels and annotations to discovered Kubernetes pod targets in the same way as Prometheus 2.35 does. See [this feature request](https://github.com/prometheus/prometheus/issues/9510) and [this pull request](https://github.com/prometheus/prometheus/pull/10080).
* FEATURE: [vmagent](./vmagent.md): add support for `tls_config` and `proxy_url` options at `oauth2` section in the same way as Prometheus does. See [oauth2 docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2).
* FEATURE: [vmagent](./vmagent.md): add support for `min_version` option at `tls_config` section in the same way as Prometheus does. See [tls_config docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config).
* FEATURE: [vmagent](./vmagent.md): expose `vmagent_remotewrite_rate_limit` metric at `http://vmagent:8429/metrics`, which can be used for alerting rules such as `rate(vmagent_remotewrite_conn_bytes_written_total) / vmagent_remotewrite_rate_limit > 0.8` when `-remoteWrite.rateLimit` command-line flag is set. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2521).
* FEATURE: [vmalert](./vmalert.md): add support for DNS-based discovery for notifiers in the same way as Prometheus does (aka `dns_sd_configs`). See [these docs](./vmalert.md#notifier-configuration-file) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2460).
* FEATURE: [vmalert](./vmalert.md): add `-replay.disableProgressBar` command-line flag, which allows disabling progressbar in [rules' backfilling mode](./vmalert.md#rules-backfilling). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1761).
* FEATURE: allow specifying TLS cipher suites for incoming https requests via `-tlsCipherSuites` command-line flag. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2404).
* FEATURE: allow specifying TLS cipher suites for mTLS connections between cluster components via `-cluster.tlsCipherSuites` command-line flag. See [these docs](./Cluster-VictoriaMetrics.md#mtls-protection).
* FEATURE: vmstorage: add `-snapshotsMaxAge` command-line flag for automatic removal of snapshots older than the given age.
* FEATURE: [vmui](./README.md#vmui): show an empty graph on the selected time range when there is no data on it. Previously `No data to show` placeholder was shown instead of the graph in this case. This prevented from zooming and scrolling of such a graph.
* FEATURE: [vmui](./README.md#vmui): show the selected `last N minutes/hours/days` in the top right corner. Previously the `start - end` duration was shown instead, which could be hard to interpret. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2402).
* FEATURE: [vmui](./README.md#vmui): execute the query when `enter` button is pressed in the same way as Prometheus does. Multi-line query can be entered by pressing `shift-enter` in the query input field.
* FEATURE: expose `vm_indexdb_items_added_total` and `vm_indexdb_items_added_size_bytes_total` counters at `/metrics` page, which can be used for monitoring the rate for addition of new entries in `indexdb` (aka `inverted index`) alongside the total size in bytes for the added entries. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2471).
* FEATURE: [vmctl](./vmctl.md): show data processing speed during data migration.
* FEATURE: [MetricsQL](./MetricsQL.md): add `drop_common_labels()` function, which drops common `label="name"` pairs from the passed time series. See [these docs](./MetricsQL.md#drop_common_labels).
* FEATURE: [MetricsQL](./MetricsQL.md): add `tlast_change_over_time(m[d])` function, which returns the timestamp of the last change of `m` on the given lookbehind window `d`. See [these docs](./MetricsQL.md#tlast_change_over_time).
* FEATURE: leave the last raw sample per each `-dedup.minScrapeInterval` discrete interval when the [deduplication](./README.md#deduplication) is enabled. This aligns better with the [staleness rules in Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness) comparing to the previous behaviour when the first sample per each `-dedup.minScrapeInterval` was left.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add ability to disable peer TLS certificate verification with `-cluster.tlsInsecureSkipVerify` command-line flag. See [mTLS docs](./Cluster-VictoriaMetrics.md#mtls-protection) for details. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2490).
* FEATURE: add a handler for `/api/v1/status/buildinfo` endpoint, which is used by Grafana starting from v8.5.0 . See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2515).
* FEATURE: add ability to proxy alerting API requests from Grafana to vmalert by passing `-vmalert.proxyURL` command-line flag to single-node VictoriaMetrics or to `vmselect` at cluster version of VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1739).

* BUGFIX: export staleness markers as `null` values from [JSON export API](./README.md#how-to-export-data-in-json-line-format). Previously they were exported as `NaN` values. This could break the exported JSON parsing, since `NaN` values aren't supported by [JSON specification](https://www.json.org/).
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): close `vmselect->vmstorage` connections if they were idle for more than 30 seconds. Expose `vm_tcpdialer_conns_idle` metric at `http://vmselect:8481/metrics` with the number of idle connections to `vmstorage`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2508).
* BUGFIX: [vmctl](./vmctl.md): return non-zero exit code on error. This allows handling `vmctl` errors in shell scripts. Previously `vmctl` was returning 0 exit code on error. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2322).
* BUGFIX: [vmctl](./vmctl.md): prevent from indefinite hang on `Ctrl+C`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2491).
* BUGFIX: [vmagent](./vmagent.md): properly show `scrape_timeout` and `scrape_interval` options at `http://vmagent:8429/config` page. Previously these options weren't displayed even if they were set in `-promscrape.config`.
* BUGFIX: [vmagent](./vmagent.md): handle non-standard http redirect status codes, which may be returned by scrape targets, in the same way as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2482).
* BUGFIX: [vmalert](./vmalert.md): skip template execution during rules' validation. This should prevent from `error evaluating annotation template` errors when some template functions expect non-empty args. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2514).
* BUGFIX: [vmalert](./vmalert.md): fixed truncating alerts expression in table, updated table cell layout. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2484).
* BUGFIX: [MetricsQL](./MetricsQL.md): properly handle joins on time series filtered by values. For example, `kube_pod_container_resource_requests{resource="cpu"} * on (namespace,pod) group_left() (kube_pod_status_phase{phase=~"Pending|Running"}==1)`. This query could result in `duplicate time series on the right side` error even if `==1` filter leaves only a single time series per `(namespace,pod)` labels. Now such query is properly executed.
* BUGFIX: [MetricsQL](./MetricsQL.md): properly handle `scalar default vector`, `scalar if vector` and `scalar ifnot vector` queries. Previously such queries could return unexpected results from the `vector` part.
* BUGFIX: [Official Grafana dashboards for VictoriaMetrics](https://grafana.com/orgs/victoriametrics): take into account `indexdb` when calculating disk space usage. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2368).


## [v1.76.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.76.1)

Released at 2022-04-12

**Update notes:** this release introduces backwards-incompatible changes to communication protocol between `vmselect` and `vmstorage` nodes in cluster version of VictoriaMetrics, so read requests to `vmselect` will fail until the upgrade is complete. These errors will stop after all the `vmselect` and `vmstorage` nodes are updated to the new release. It is safe to downgrade to previous releases.

* FEATURE: [vmalert](./vmalert.md): add support for `alert_relabel_configs` option at `-notifier.config`. This option allows configuring relabeling rules for alerts before sending them to configured notifiers. See [these docs](./vmalert.md#notifier-configuration-file) for details.
* FEATURE: [vmagent](./vmalert.md): allow passing StatefulSet pod names to `-promscrape.cluster.memberNum` command-line flag. In this case the member number is automatically extracted from the pod name, which must end with the number in the range `0 ... promscrape.cluster.membersCount-1`. For example, `vmagent-0`, `vmagent-1`, etc. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2359) and [these docs](./vmagent.md#scraping-big-number-of-targets).

* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): properly propagate limits at `-search.max*` command-line flags from `vminsert` to `vmstorage`. The limits are `-search.maxUniqueTimeseries`, `-search.maxSeries`, `-search.maxFederateSeries`, `-search.maxExportSeries`, `-search.maxGraphiteSeries` and `-search.maxTSDBStatusSeries`. They weren't propagated to `vmstorage` because of the bug. These limits were introduced in [v1.76.0](./CHANGELOG.md#v1760). See [this bug](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2450).
* BUGFIX: fix goroutine leak and possible deadlock when importing invalid data via [native binary format](./README.md#how-to-import-data-in-native-format). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2423).
* BUGFIX: [Graphite Render API](./README.md#graphite-render-api-usage): properly calculate [hitCount](https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.hitcount) function. Previously it could return empty results if there were no original samples in some parts of the selected time range.
* BUGFIX: [MetricsQL](./MetricsQL.md): allow overriding built-in function names inside [WITH templates](https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/expand-with-exprs). For example, `WITH (sum(a,b) = a + b + 1) sum(x,y)` now expands into `x + y + 1`. Previously such a query would fail with `cannot use reserved name` error. See [this bugreport](https://github.com/VictoriaMetrics/metricsql/issues/5).
* BUGFIX: [vmui](./README.md#vmui): properly display values greater than 1000 on Y axis. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2409).


## [v1.76.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.76.0)

Released at 2022-04-07

**Update notes:** this release introduces backwards-incompatible changes to communication protocol between `vmselect` and `vmstorage` nodes in cluster version of VictoriaMetrics, so read requests to `vmselect` will fail until the upgrade is complete. These errors will stop after all the `vmselect` and `vmstorage` nodes are updated to the new release. It is safe to downgrade to previous releases.

* FEATURE: [vmctl](./vmctl.md): add ability to verify files obtained via [native export](./README.md#how-to-export-data-in-native-format). See [these docs](./vmctl.md#verifying-exported-blocks-from-victoriametrics) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2362).
* FEATURE: [vmui](./README.md#vmui): add pre-defined dashboards for per-job CPU usage, memory usage and disk IO usage. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2243) for details.
* FEATURE: [vmalert](./vmalert.md): improve compatibility with [Prometheus Alert Generator specification](https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2340).
* FEATURE: [vmalert](./vmalert.md): add `-datasource.disableKeepAlive` command-line flag, which can be used for disabling [HTTP keep-alive connections](https://en.wikipedia.org/wiki/HTTP_persistent_connection) to datasources. This option can be useful for distributing load among multiple datasources behind TCP proxy such as [HAProxy](http://www.haproxy.org/).
* FEATURE: [Cluster version of VictoriaMetrics](./Cluster-VictoriaMetrics.md): reduce memory usage by up to 50% for `vminsert` and `vmstorage` under high ingestion rate.
* FEATURE: [vmgateway](./vmgateway.md): Allow to read `-ratelimit.config` file from URL. Also add `-ratelimit.configCheckInterval` command-line option. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2241).
* FEATURE: add the following command-line flags, which can be used for fine-grained limiting of CPU and memory usage during various API calls:

  * `-search.maxFederateSeries` for limiting the number of time series, which can be returned from [/federate](./README.md#federation).
  * `-search.maxExportSeries` for limiting the number of time series, which can be returned from [/api/v1/export](./README.md#how-to-export-time-series).
  * `-search.maxSeries` for limiting the number of time series, which can be returned from [/api/v1/series](./url-examples.md#apiv1series).
  * `-search.maxTSDBStatusSeries` for limiting the number of time series, which can be scanned during the request to [/api/v1/status/tsdb](./README.md#tsdb-stats).
  * `-search.maxGraphiteSeries` for limiting the number of time series, which can be scanned during the request to [Graphite Render API](./README.md#graphite-render-api-usage).

Previously the `-search.maxUniqueTimeseries` command-line flag was used as a global limit for all these APIs. Now the `-search.maxUniqueTimeseries` is used only for limiting the number of time series, which can be scanned during requests to [/api/v1/query](./url-examples.md#apiv1query) and [/api/v1/query_range](./url-examples.md#apiv1query_range).

When using [cluster version of VictoriaMetrics](./Cluster-VictoriaMetrics.md), these command-line flags (including `-search.maxUniqueTimeseries`) must be passed to `vmselect` instead of `vmstorage`.

* BUGFIX: [vmagent](./vmagent.md) and [vmauth](./vmauth.md): reduce the probability of `TLS handshake error from XX.XX.XX.XX: EOF` errors when `-remoteWrite.url` points to HTTPS url at `vmauth`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1699).
* BUGFIX: return `Content-Type: text/html` response header when requesting `/` HTTP path at VictoriaMetrics components. Previously `text/plain` response header was returned, which could lead to broken page formatting. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2323).
* BUGFIX: [Graphite Render API](./README.md#graphite-render-api-usage): accept floating-point values for [maxDataPoints](https://graphite.readthedocs.io/en/stable/render_api.html#maxdatapoints) query arg, since some clients send floating-point values instead of integer values for this arg.

## [v1.75.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.75.1)

Released at 2022-03-28

* BUGFIX: update base image for VictoriaMetrics from `alpine-3.15.0` to `alpine-3.15.2`. This fixes [CVE-2022-0778](https://nvd.nist.gov/vuln/detail/CVE-2022-0778). See [alpine 3.15.2 release docs](https://alpinelinux.org/posts/Alpine-3.15.2-released.html).

## [v1.75.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.75.0)

Released at 2022-03-18

**Update notes:** release contains breaking change to vmalert's API introduced in [ee396b5](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2320/commits/ee396b5750d0bcb98233d624f88fa6cf92a8253b).
It replaces the `api/v1/groups` API handler with `api/v1/rules` handler in order to become compatible
with [alerts generator specification](https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md).
See other changes introduced to vmalert [here](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2320).

* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add support for mTLS communications between cluster components. See [these docs](./Cluster-VictoriaMetrics.md#mtls-protection) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/550).
* FEATURE: [vmalert](./vmalert.md): add ability to use OAuth2 for `-datasource.url`, `-notifier.url` and `-remoteRead.url`. See the corresponding command-line flags containing `oauth2` in their names [here](./vmalert.md#flags).
* FEATURE: [vmalert](./vmalert.md): add ability to use Bearer Token for `-notifier.url` via `-notifier.bearerToken` and `-notifier.bearerTokenFile` command-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1824).
* FEATURE: [vmalert](./vmalert.md): add `sortByLabel` template function in order to be consistent with Prometheus. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/#functions) for more details.
* FEATURE: [vmalert](./vmalert.md): improve compliance with [Prometheus Alert Generator Specification](https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md).
* FEATURE: [vmalert](./vmalert.md): add `-rule.resendDelay` command-line flag, which specifies the minumum amount of time to wait before resending an alert to Alertmanager (e.g. this is equivalent to `-rules.alert.resend-delay` option from Prometheus. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1665).
* FEATURE: [vmauth](./vmauth.md): transparently treat `Authorization: Token ...` request headers as `Authorization: Bearer ...` request headers. This allows sending requests to `vmauth` from InfluxDB clients. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1897). Thanks to @dcircelli for the pull request.
* FEATURE: do not log trivial network errors such as `broken pipe` and `connection reset by peer`. This error could occur when writing data to the client, which closes the connection to VictoriaMetrics due to request timeout or similar reason. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2334).

* BUGFIX: [Graphite Render API](./README.md#graphite-render-api-usage): return an additional point after `until` timestamp in the same way as Graphite does. Previously VictoriaMetrics didn't return this point, which could result in missing last point on the graph.
* BUGFIX: properly locate series with the given `name` and without the given `label` when using the `name{label=~"foo|"}` series selector. Previously such series could be skipped. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2255). Thanks to @jduncan0000 for discovering and fixing the issue.
* BUGFIX: properly free up memory occupied by deleted cache entries for the following caches: `indexdb/dataBlocks`, `indexdb/indexBlocks`, `storage/indexBlocks`. This should reduce the increased memory usage starting from v1.73.0. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2242) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2007) issue.
* BUGFIX: reduce the interval for checking for free disk space from 30 seconds to 1 second. This should reduce the probability of `no space left on device` panics when `-storage.minFreeDiskSpaceBytes` is set to too low values. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2305).
* BUGFIX: [vmagent](./vmagent.md): prevent from panic at vmagent when importing a time series with big number of samples. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2335). Thanks to @bleedfish for discovering and fixing the issue.

## [v1.74.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.74.0)

Released at 2022-03-03

**Update notes:** In this release VictoriaMetrics may use some extra memory due to issues [#2242](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2242) and [#2007](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2007). These issues were addressed in [v1.75.0](#v1750), so we recommend updating straight to it.

* FEATURE: add support for conditional relabeling via `if` filter. The `if` filter can contain arbitrary [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). For example, the following rule drops targets matching `foo{bar="baz"}` series selector:

```yaml
- action: drop
  if: 'foo{bar="baz"}'
```

This rule is equivalent to less clear traditional one:

```yaml
- action: drop
  source_labels: [__name__, bar]
  regex: 'foo;baz'
```

  See [relabeling docs](./vmagent.md#relabeling) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1998) for more details.

* FEATURE: reduce memory usage for various caches under [high churn rate](./FAQ.md#what-is-high-churn-rate).
* FEATURE: [vmagent](./vmagent.md): re-use Kafka client when pushing data from [many tenants](./vmagent.md#multitenancy) to Kafka. Previously a separate Kafka client was created per each tenant. This could lead to increased load on Kafka. See [how to push data from vmagent to Kafka](./vmagent.md#writing-metrics-to-kafka).
* FEATURE: improve performance when registering new time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2247). Thanks to @ahfuzhang .

* BUGFIX: return the proper number of datapoints from `moving*()` functions such as `movingAverage()` in [Graphite Render API](./README.md#graphite-render-api-usage). Previously these functions could return too big number of samples if [maxDataPoints query arg](https://graphite.readthedocs.io/en/stable/render_api.html#maxdatapoints) is explicitly passed to `/render` API.
* BUGFIX: properly handle [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) containing a filter for multiple metric names plus a negative filter. For example, `{__name__=~"foo|bar",job!="baz"}` . Previously VictoriaMetrics could return series with `foo` or `bar` names and with `job="baz"`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2238).
* BUGFIX: [vmgateway](./vmgateway.md): properly parse JWT tokens if they are encoded with [URL-safe base64 encoding](https://datatracker.ietf.org/doc/html/rfc4648#section-5).

## [v1.73.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.73.1)

Released at 2022-02-22

**Update notes:** In this release VictoriaMetrics may use some extra memory due to issues [#2242](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2242) and [#2007](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2007). These issues were addressed in [v1.75.0](#v1750), so we recommend updating straight to it.

* FEATURE: allow overriding default limits for the following in-memory caches, which usually occupy the most memory:
  * `storage/tsid` - the cache speeds up lookups of internal metric ids by `metric_name{labels...}` during data ingestion. The size for this cache can be tuned with `-storage.cacheSizeStorageTSID` command-line flag.
  * `indexdb/dataBlocks` - the cache speeds up data lookups in `<-storageDataPath>/indexdb` files. The size for this cache can be tuned with `-storage.cacheSizeIndexDBDataBlocks` command-line flag.
  * `indexdb/indexBlocks` - the cache speeds up index lookups in `<-storageDataPath>/indexdb` files. The size for this cache can be tuned with `-storage.cacheSizeIndexDBIndexBlocks` command-line flag.
  See also [cache tuning docs](./README.md#cache-tuning). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1940).
* FEATURE: add `-influxDBLabel` command-line flag for overriding `db` label name for the data [imported into VictoriaMetrics via InfluxDB line protocol](./README.md#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf). Thanks to @johnatannvmd for [the pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2203).
* FEATURE: return `X-Influxdb-Version` HTTP header in responses to [InfluxDB write requests](./README.md#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf). This is needed for some InfluxDB clients. See [this comment](https://github.com/ntop/ntopng/issues/5449#issuecomment-1005347597) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2209).

* BUGFIX: reduce memory usage during the first three hours after the upgrade from versions older than v1.73.0. The memory usage spike was related to the need of in-memory caches' re-population after the upgrade because of the fix for [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401). Now cache size limits are reduced in order to occupy less memory during the upgrade.
* BUGFIX: fix a bug, which could significantly slow down requests to `/api/v1/labels` and `/api/v1/label/<label_name>/values`. These APIs are used by Grafana for auto-completion of label names and label values. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2200).
* BUGFIX: vmalert: add support for `$externalLabels` and `$externalURL` template vars in the same way as Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2193).
* BUGFIX: vmalert: make sure notifiers are discovered during initialization if they are configured via `consul_sd_configs`. Previously they could be discovered in 30 seconds (the default value for `-promscrape.consulSDCheckInterval` command-line flag) after the initialization. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2202).
* BUGFIX: update default value for `-promscrape.fileSDCheckInterval`, so it matches default duration used by Prometheus for checking for updates in `file_sd_configs`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2187). Thanks to @corporate-gadfly for the fix.
* BUGFIX: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): do not return partial responses from `vmselect` if at least a single `vmstorage` node was reachable and returned an app-level error. Such errors are usually related to cluster mis-configuration, so they must be returned to the caller instead of being masked by [partial responses](./Cluster-VictoriaMetrics.md#cluster-availability). Partial responses can be returned only if some of `vmstorage` nodes are unreachable during the query. This may help the following issues: [one](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1941), [two](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/678).

## [v1.73.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.73.0)

Released at 2022-02-14

**Update notes:** In this release VictoriaMetrics may use some extra memory described in issues [#2242](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2242) and [#2007](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2007). These issues were addressed in [v1.75.0](#v1750), so we recommend updating straight to it.

* FEATURE: publish VictoriaMetrics binaries for MacOS amd64 and MacOS arm64 (aka MacBook M1) at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1896) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1851).
* FEATURE: reduce CPU and disk IO usage during `indexdb` rotation once per `-retentionPeriod`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401).
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): add `-dropSamplesOnOverload` command-line flag for `vminsert`. If this flag is set, then `vminsert` drops incoming data if the destination `vmstorage` is temporarily unavailable or cannot keep up with the ingestion rate. The number of dropped rows can be [monitored](./Cluster-VictoriaMetrics.md#monitoring) via `vm_rpc_rows_dropped_on_overload_total` metric at `vminsert`.
* FEATURE: [VictoriaMetrics cluster](./Cluster-VictoriaMetrics.md): improve re-routing logic, so it re-routes incoming data more evenly if some of `vmstorage` nodes are temporarily unavailable and/or accept data at slower rate than other `vmstorage` nodes. Also significantly reduce possible re-routing storm when `vminsert` runs with `-disableRerouting=false` command-line flag. This should help the following issues: [one](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1337), [two](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1165), [three](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1054), [four](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791), [five](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1544).
* FEATURE: [MetricsQL](./MetricsQL.md): cover more cases with the [label filters' propagation optimization](https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization). This should improve the average performance for practical queries. The following cases are additionally covered:
  * Multi-level [transform functions](./MetricsQL.md#transform-functions). For example, `abs(round(foo{a="b"})) + bar{x="y"}` is now optimized to `abs(round(foo{a="b",x="y"})) + bar{a="b",x="y"}`
  * Binary operations with `on()`, `without()`, `group_left()` and `group_right()` modifiers. For example, `foo{a="b"} on (a) + bar` is now optimized to `foo{a="b"} on (a) + bar{a="b"}`
  * Multi-level binary operations. For example, `foo{a="b"} + bar{x="y"} + baz{z="q"}` is now optimized to `foo{a="b",x="y",z="q"} + bar{a="b",x="y",z="q"} + baz{a="b",x="y",z="q"}`
  * Aggregate functions. For example, `sum(foo{a="b"}) by (c) + bar{c="d"}` is now optimized to `sum(foo{a="b",c="d"}) by (c) + bar{c="d"}`
* FEATURE [MetricsQL](./MetricsQL.md): optimize joining with `*_info` labels. For example: `kube_pod_created{namespace="prod"} * on (uid) group_left(node) kube_pod_info` now automatically adds the needed filters on `uid` label to `kube_pod_info` before selecting series for the right side of `*` operation. This may save CPU, RAM and disk IO resources. See [this article](https://www.robustperception.io/exposing-the-software-version-to-prometheus) for details on `*_info` labels. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1827).
* FEATURE: all: improve performance for arm64 builds of VictoriaMetrics components by up to 15%. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2102).
* FEATURE: all: expose `process_cpu_cores_available` metric, which shows the number of CPU cores available to the app. The number can be fractional if the corresponding cgroup limit is set to a fractional value. This metric is useful for alerting on CPU saturation. For example, the following query alerts when the app uses more than 90% of CPU during the last 5 minutes: `rate(process_cpu_seconds_total[5m]) / process_cpu_cores_available > 0.9` . See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2107).
* FEATURE: [vmalert](./vmalert.md): add ability to configure notifiers (e.g. alertmanager) via a file in the way similar to Prometheus. See [these docs](./vmalert.md#notifier-configuration-file), [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2127).
* FEATURE: [vmalert](./vmalert.md): add support for Consul service discovery for notifiers. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1947).
* FEATURE: [vmalert](./vmalert.md): add support for specifying Basic Auth password for notifiers via a file. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1567).
* FEATURE: [vmagent](./vmagent.md): provide the ability to fetch target responses on behalf of `vmagent` by clicking the `response` link for the needed target at `/targets` page. This feature may be useful for debugging responses from targets located in isolated environments.
* FEATURE: [vmagent](./vmagent.md): show the total number of scrapes and the total number of scrape errors per target at `/targets` page. This information may be useful when debugging unreliable scrape targets.
* FEATURE: vmagent and single-node VictoriaMetrics: disallow unknown fields at `-promscrape.config` file. Previously unknown fields were allowed. This could lead to long-living silent config errors. The previous behaviour can be returned by passing `-promscrape.config.strictParse=false` command-line flag.
* FEATURE: vmagent: add `__meta_kubernetes_endpointslice_label*` and `__meta_kubernetes_endpointslice_annotation*` labels for `role: endpointslice` targets in [kubernetes_sd_config](./sd_configs.md#kubernetes_sd_configs) to be consistent with other `role` values. See [this issue](https://github.com/prometheus/prometheus/issues/10284).
* FEATURE: vmagent: add `collapse all` and `expand all` buttons to `http://vmagent:8429/targets` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2021).
* FEATURE: vmagent: support Prometheus-like durations in `-promscrape.config`. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/817#issuecomment-1033384766).
* FEATURE: automatically re-read `-tlsCertFile` and `-tlsKeyFile` files, so their contents can be updated without the need to restart VictoriaMetrics apps. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2171).

* BUGFIX: calculate [absent_over_time()](./MetricsQL.md#absent_over_time) in the same way as Prometheus does. Previously it could return multiple time series instead of at most one time series like Prometheus does. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2130).
* BUGFIX: return proper results from `highestMax()` function at [Graphite render API](./README.md#graphite-render-api-usage). Previously it was incorrectly returning timeseries with min peaks instead of max peaks.
* BUGFIX: properly limit indexdb cache sizes. Previously they could exceed values set via `-memory.allowedPercent` and/or `-memory.allowedBytes` when `indexdb` contained many data parts. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2007).
* BUGFIX: [vmui](./README.md#vmui): fix a bug, which could break time range picker when editing `From` or `To` input fields. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2080).
* BUGFIX: [vmui](./README.md#vmui): fix a bug, which could break switching between `graph`, `json` and `table` views. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2084).
* BUGFIX: [vmui](./README.md#vmui): fix possible UI freeze after querying `node_uname_info` time series. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2115).
* BUGFIX: show the original location of the warning or error message when logging throttled messages. Previously the location inside `lib/logger/throttler.go` was shown. This could increase the complexity of debugging.
* BUGFIX: vmalert: fix links at web UI. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2167).
* BUGFIX: vmagent: properly discover pods without exposed ports for the given service for `role: endpoints` and `role: endpointslice` in [kubernetes_sd_config](./sd_configs.md#kubernetes_sd_configs). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2134).
* BUGFIX: vmagent: properly display `zone` contents for `gce_sd_configs` section at `http://vmagent:8429/config` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2179). Thanks to @artifactori for the bugfix.
* BUGFIX: vmagent: properly handle `all_tenants: true` config option at `openstack_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2182).

## [v1.72.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.72.0)

Released at 2022-01-18

* FEATURE: [MetricsQL](./MetricsQL.md): add support for `@` modifier, which is enabled by default in Prometheus starting from [Prometheus v2.33.0](https://github.com/prometheus/prometheus/pull/10121). See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#modifier) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1348). VictoriaMetrics extends `@` modifier with the following additional features:
  * It can contain arbitrary expression. For example, `foo @ (end() - 1h)` would return `foo` value at `end - 1 hour` timestamp on the selected time range `[start ... end]`. Another example: `foo @ (now() - 10m)` would return `foo` value 10 minutes ago from the current time.
  * It can be put everywhere in the query. For example, `sum(foo) @ start()` would calculate `sum(foo)` at `start` timestamp on the selected time range `[start ... end]`.
* FEATURE: [MetricsQL](./MetricsQL.md): add support for optional `keep_metric_names` modifier, which can be applied to all the [rollup functions](./MetricsQL.md#rollup-functions) and [transform functions](./MetricsQL.md#transform-functions). This modifier prevents from deleting metric names from function results. For example, `rate({__name__=~"foo|bar"}[5m]) keep_metric_names` leaves `foo` and `bar` metric names in `rate()` results. This feature provides an additional workaround for [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/949).
* FEATURE: [vmagent](./vmagent.md): add support for Kubernetes service discovery in the current namespace in the same way as [Prometheus does](https://github.com/prometheus/prometheus/pull/9881). For example, the following config limits pod discovery to the namespace where vmagent runs:

```yaml
  scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
    - role: pod
      namespaces:
        own_namespace: true
```

* FEATURE: [vmagent](./vmagent.md): add `__meta_kubernetes_node_provider_id` label for discovered Kubernetes nodes in the same way as [Prometheus does](https://github.com/prometheus/prometheus/pull/9603).
* FEATURE: [vmagent](./vmagent.md): log error message when remote storage returns 400 or 409 http errors. This should simplify detection and debugging of this case. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1911).
* FEATURE: [vmagent](./vmagent.md): expose `promscrape_stale_samples_created_total` metric for monitoring the total number of created stale samples when scraping Prometheus targets. See [these docs](./vmagent.md#prometheus-staleness-markers) for the information on when stale samples (aka staleness markers) can be created.
* FEATURE: [vmrestore](./vmrestore.md): store `restore-in-progress` file in `-dst` directory while `vmrestore` is running. This file is automatically deleted when `vmrestore` is successfully finished. This helps detecting incompletely restored data on VictoriaMetrics start. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1958).
* FEATURE: [vmctl](./vmctl.md): print the last sample timestamp when the data migration is interrupted either by user or by error. This helps continuing the data migration from the interruption moment. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1236).
* FEATURE: [vmalert](./vmalert.md): expose `vmalert_remotewrite_total` metric at `/metrics` page. This makes possible calculating SLOs for error rate during writing recording rules and alert state to `-remoteWrite.url` with the query `vmalert_remotewrite_errors_total / vmalert_remotewrite_total`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2040). Thanks to @afoninsky .
* FEATURE: [vmalert](./vmalert.md): add `stripPort` template function in the same way as [Prometheus does](https://github.com/prometheus/prometheus/pull/10002).
* FEATURE: [vmalert](./vmalert.md): add `parseDuration` template function in the same way as [Prometheus does](https://github.com/prometheus/prometheus/pull/8817).
* FEATURE: [MetricsQL](./MetricsQL.md): add `stale_samples_over_time(m[d])` function for calculating the number of [staleness marks](./vmagent.md#prometheus-staleness-markers) for time series `m` over the duration `d`. This function may be useful for detecting flapping metrics at scrape targets, which periodically disappear and then appear again.
* FEATURE: [vmgateway](./vmgateway.md): add support for `extra_filters` option. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1863).
* FEATURE: [vmui](./README.md#vmui): improve UX according to [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1960). Thanks to @Loori-R .
* FEATURE: [vmui](./README.md#vmui): limit the number of requests sent to VictoriaMetrics during zooming / scrolling. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2064).

* BUGFIX: [vmagent](./vmagent.md): make sure that `vmagent` replicas scrape the same targets at different time offsets when [replication is enabled in vmagent clustering mode](./vmagent.md#scraping-big-number-of-targets). This guarantees that the [deduplication](./README.md#deduplication) consistently leaves samples from the same `vmagent` replica.
* BUGFIX: return the proper response stub from `/api/v1/query_exemplars` handler, which is needed for Grafana v8+. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1999).
* BUGFIX: [vmctl](./vmctl.md): fix a few edge cases and improve migration speed for OpenTSDB importer. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2019).
* BUGFIX: fix possible data race when searching for time series matching `{key=~"value|"}` filter over time range covering multiple days. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2032). Thanks to @waldoweng for the provided fix.
* BUGFIX: [vmagent](./vmagent.md): do not send staleness markers on graceful shutdown. This follows Prometheus behavior. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2013#issuecomment-1006994079).
* BUGFIX: [vmagent](./vmagent.md): properly set `__address__` label in `dockerswarm_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2038). Thanks to @ashtuchkin for the fix.
* BUGFIX: [vmui](./README.md#vmui): fix incorrect calculations for graph limits on y axis. This could result in incorrect graph rendering in some cases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2037).
* BUGFIX: [vmui](./README.md#vmui): fix handling for multi-line queries. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2039).

## Previous releases

See changes for older releases [here](./CHANGELOG_2021.md).
