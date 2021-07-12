---
sort: 15
---

# CHANGELOG

## tip

* FEATURE: vmagent: dynamically reload client TLS certificates from disk on every [mTLS connection](https://developers.cloudflare.com/cloudflare-one/identity/devices/mutual-tls-authentication). This should allow using `vmagent` with [Istio service mesh](https://istio.io/latest/about/service-mesh/). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1420).
* FEATURE: reduce memory usage by up to 30% on production workloads.
* FEATURE: log http request path plus all the query args on errors during request processing. Previously only http request path was logged without query args, so it could be hard debugging such errors.
* FEATURE: export `vmselect_request_duration_seconds` and `vminsert_request_duration_seconds` [VictoriaMetrics histograms](https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350) at `/metrics` page. These histograms can be used for determining latency distribution for the served requests.
* FEATURE: vmselect: embed [vmui](https://github.com/VictoriaMetrics/vmui) into a single-node VictoriaMetrics and into `vmselect` component of cluster version. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1413). The web interface is available at the following paths:
  * `/vmui/` for a single-node VictoriaMetrics
  * `/select/<accountID>/vmui/` for `vmselect` at cluster version of VictoriaMetrics
* FEATURE: support durations anywhere in [MetricsQL queries](https://docs.victoriametrics.com/MetricsQL.html). For example, `sum_over_time(m[1h]) / 1h` is a valid query, which is equivalent to `sum_over_time(m[1h]) / 3600`.
* FEATURE: support durations without suffxies in [MetricsQL queries](https://docs.victoriametrics.com/MetricsQL.html). For example, `rate(m[3600])` is a valid query, which is equivalent to `rate(m[1h])`.

* BUGFIX: vmagent: remove `{ %space %}` typo in `/targets` output. The typo has been introduced in v1.62.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1408).
* BUGFIX: vmagent: fix CSS styles on `/targets` page. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1422).
* BUGFIX: vmalert: accept Prometheus-like durations in `interval` config option inside `group` section. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1444).
* BUGFIX: properly update `vm_merge_need_free_disk_space` metric at `/metrics` page when there is no enough free disk space for performing optimal merges. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1373).


## [v1.62.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.62.0)

* FEATURE: vmagent: add service discovery for Docker (aka [docker_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config)). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1402).
* FEATURE: vmagent: add service discovery for DigitalOcean (aka [digitalocean_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config)). See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1367).
* FEATURE: vmagent: change the default value for `-remoteWrite.queues` from 4 to `2 * numCPUs`. This should reduce scrape duration for highly loaded vmagent, which scrapes tens of thousands of targets. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1385).
* FEATURE: vmagent: show the number of samples the target returns during the last scrape on `/targets` and `/api/v1/targets` pages. This should simplify debugging targets, which may return too big or too low number of samples. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1377).
* FEATURE: vmagent: show jobs with zero discovered targets on `/targets` page. This should help debugging improperly configured scrape configs.
* FEATURE: vmagent: support for http-based service discovery (aka [http_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config)), which has been added since Prometheus 2.28. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1392).
* FEATURE: vmagent: support namespace in Consul serive discovery in the same way as Prometheus 2.28 does. See [this issue](https://github.com/prometheus/prometheus/issues/8894) for details.
* FEATURE: vmagent: support generic auth configs in `consul_sd_configs` in the same way as Prometheus 2.28 does. See [this issue](https://github.com/prometheus/prometheus/issues/8924) for details.
* FEATURE: [vmctl](https://docs.victoriametrics.com/vmctl.html): limit the number of samples per each imported JSON line. This should limit the memory usage at VictoriaMetrics side when importing time series with big number of samples.
* FEATURE: vmselect: log slow queries across all the `/api/v1/*` handlers (aka [Prometheus query API](https://prometheus.io/docs/prometheus/latest/querying/api)) if their execution duration exceeds `-search.logSlowQueryDuration`. This should simplify debugging slow requests to such handlers as `/api/v1/labels` or `/api/v1/series` additionally to `/api/v1/query` and `/api/v1/query_range`, which were logged in the previous releases.
* FEATURE: vminsert: sort the `-storageNode` list in order to guarantee the identical `series -> vmstorage` mapping across all the `vminsert` nodes. This should reduce resource usage (RAM, CPU and disk IO) at `vmstorage` nodes if `vmstorage` addresses are passed in random order to `vminsert` nodes.
* FEATURE: vmstorage: reduce memory usage on a system with many CPU cores under high ingestion rate.

* BUGFIX: prevent from adding new samples to deleted time series after the rotation of the inverted index (the rotation is performed once per `-retentionPeriod`). See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347#issuecomment-861232136) for details.
* BUGFIX: vmstorage: reduce high disk write IO usage on systems with big number of CPU cores. The issue has been introduced in the release [v1.59.0](#v1590). See [this commit](aa9b56a046b6ae8083fa659df35dd5e994bf9115) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1338#issuecomment-863046999) for details.
* BUGFIX: vmstorage: prevent from incorrect stats collection when multiple concurrent queries execute the same tag filter. This may help reducing CPU usage under certain workloads. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1338).
* BUGFIX: vmselect: return the last timestamp for the max / min value from `tmax_over_time(m[d])` and `tmin_over_time(m[d])` [MetricsQL functions](https://docs.victoriametrics.com/MetricsQL.html) as most users expect. See also [this issue](https://github.com/prometheus/prometheus/issues/8966).
* BUGFIX: vmselect: return the expected value for `increase_pure()` [MetricsQL function](https://docs.victoriametrics.com/MetricsQL.html) after a gap in a time series. Previously incorrect too big value could be returned after the gap from `increase_pure()`.


## [v1.61.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.61.1)

* BUGFIX: vmalert: fix recording rules, which were broken in v1.61.0. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1369).
* BUGFIX: reset the on-disk cache for mapping from the full metric name to an internal metric id (e.g. `metric_name{labels} -> internal_metric_id`) after deleting metrics via [delete API](https://docs.victoriametrics.com/#how-to-delete-time-series). This should prevent from possible inconsistent state after unclean shutdown. This [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347).


## [v1.61.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.61.0)

* FEATURE: vmalert: add support for backfilling (aka replay) of recording and alerting rules. See [these docs](https://docs.victoriametrics.com/vmalert.html#rules-backfilling) and [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/836).
* FEATURE: vmalert: add a command-line flag `-rule.configCheckInterval` for automatic re-reading of `-rule` files without the need to send SIGHUP signal. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/512).
* FEATURE: vmagent: respect the `sample_limit` and `-promscrape.maxScrapeSize` values when scraping targets in [stream parsing mode](https://docs.victoriametrics.com/vmagent.html#stream-parsing-mode). See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1331).
* FEATURE: vmauth: add ability to specify mutliple `url_prefix` entries for balancing the load among multiple `vmselect` and/or `vminsert` nodes in a cluster. See [these docs](https://docs.victoriametrics.com/vmauth.html#load-balancing).
* FEATURE: vminsert: add `-disableRerouting` command-line flag for forcibly disabling the rerouting. This should help resolving [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1054) issues.
* FEATURE: vminsert: reduce the probability of global re-routing storm if all the vmstorage nodes cannot keep up with the given ingestion rate for some time. This should improve cluster stability in such cases. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791) and [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1054) issues.
* FEATURE: allow building VictoriaMetrics components for Solaris / SmartOS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1322).
* FEATURE: vmagent: add ability to debug relabeling rules. See [these docs](https://docs.victoriametrics.com/vmagent.html#relabeling) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1343).

* BUGFIX: reduce CPU usage by up to 2x during querying a database with big number of active daily time series. The issue has been introduced in `v1.59.0`.
* BUGFIX: vmagent: properly apply auth and tls configs in `eureka_sd_configs`. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1350).
* BUGFIX: vmauth: do not panic on aborted http requests. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353).
* BUGFIX: properly generate `target` property for `*Series(foo.*.bar)` responses returned from [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage). Previously the `target` contained the expanded list of series for `foo.*.bar`, e.g. `sumSeries(foo.a.bar,foo.b.bar,...foo.z.bar)`. Now VictoriaMetrics returns `sumSeries(foo.*.bar)` as a target in the same way as Graphite does.


## [v1.60.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.60.0)

* FEATURE: add ability to limit the number of unique time series, which can be added to storage per hour and per day. This can help dealing with high cardinality and high churn rate issues. See [these docs](https://docs.victoriametrics.com/#cardinality-limiter).
* FEATURE: vmagent: add ability to limit the number of unique time series, which can be sent to remote storage systems per hour and per day. This can help dealing with high cardinality and high churn rate issues. See [these docs](https://docs.victoriametrics.com/vmagent.html#cardinality-limiter).
* FEATURE: vmalert: add ability to run alerting and recording rules for multiple tenants. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/740) and [these docs](https://docs.victoriametrics.com/vmalert.html#multitenancy).
* FEATURE: vminsert: add support for data ingestion via other `vminsert` nodes. This allows building multi-level data ingestion paths in VictoriaMetrics cluster by writing data from one level of `vminsert` nodes to another level of `vminsert` nodes. See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multi-level-cluster-setup) and [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/541#issuecomment-835487858) for details.
* FEATURE: vmagent: reload `bearer_token_file`, `credentials_file` and `password_file` contents every second. This allows dynamically changing the contents of these files during target scraping and service discovery without the need to restart `vmagent`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1297).
* FEATURE: vmalert: add a flag to control behaviour on startup for state restore errors. Such errors were returned and logged before as well. Now user can specify whether to just log these errors (`-remoteRead.ignoreRestoreErrors=true`) or to stop the process (`-remoteRead.ignoreRestoreErrors=false`). The latter is important when VM isn't ready yet to serve queries from vmalert and it needs to wait. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1252).
* FEATURE: vmalert: add ability to pass `round_digits` query arg to datasource via `-datasource.roundDigits` command-line flag. This can be used for limiting the number of decimal digits after the point in recording rule results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/525).
* FEATURE: return `X-Server-Hostname` header in http responses of all the VictoriaMetrics components. This should simplify tracing the origin server behind a load balancer or behind auth proxy during troubleshooting.
* FEATURE: vmselect: allow to use 2x more memory for query processing at `vmselect` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html). This should allow processing heavy queries without the need to increase RAM size at `vmselect` nodes.
* FEATURE: add ability to filter `/api/v1/status/tsdb` output with arbitrary [time series selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) passed via `match[]` query args. See [these docs](https://docs.victoriametrics.com/#tsdb-stats) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1168) for details.
* FEATURE: automatically detect memory and cpu limits for VictoriaMetrics components running under [cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) environments such as [HashiCorp Nomad](https://www.nomadproject.io/). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1269).
* FEATURE: vmauth: allow `-auth.config` reloading via `/-/reload` http endpoint. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1194).
* FEATURE: add `timezone_offset(tz)` function. It returns offset in seconds for the given timezone `tz` relative to UTC. This can be useful when combining with datetime-related functions. For example, `day_of_week(time()+timezone_offset("America/Los_Angeles"))` would return weekdays for `America/Los_Angeles` time zone. Special `Local` time zone can be used for returning an offset for the time zone set on the host where VictoriaMetrics runs. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1306) and [MetricsQL docs](https://docs.victoriametrics.com/MetricsQL.html) for more details.
* FEATURE: vmagent: add support for OAuth2 authorization for scrape targets and service discovery in the same way as Prometheus does. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2).
* FEATURE: vmagent: add support for OAuth2 authorization when writing data to `-remoteWrite.url`. See `-remoteWrite.oauth2.*` config params in `/path/to/vmagent -help` output.
* FEATURE: vmalert: add ability to set `extra_filter_labels` at alerting and recording group configs. See [these docs](https://docs.victoriametrics.com/vmalert.html#groups).
* FEATURE: vmstorage: reduce memory usage by up to 30% when ingesting big number of active time series.

* BUGFIX: vmagent: do not retry scraping targets, which don't support HTTP. This should reduce CPU load and network usage at `vmagent` and at scrape target. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1289).
* BUGFIX: vmagent: fix possible race when refreshing `role: endpoints` and `role: endpointslices` scrape targets in `kubernetes_sd_config`. Prevoiusly `pod` objects could be updated after the related `endpoints` object update. This could lead to missing scrape targets. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240).
* BUGFIX: vmagent: properly spread scrape targets among `vmagent` replicas if `-promscrape.cluster.replicationFactor` exceeds 1. See [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1292).
* BUGFIX: vmagent: limit `scrape_timeout` by `scrape_interval`. This guarantees that only a single sample is lost during the configured `scrape_interval` when scrape target responds slowly. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1281#issuecomment-840538907) for details.
* BUGFIX: properly remove stale parts outside the configured retention if `-retentionPeriod` is smaller than one month. Previously stale parts could remain active for up to a month after they go outside the retention.
* BUGFIX: stop the process on panic errors, since such errors may leave the process in inconsistent state. Previously panics could be recovered, which could result in unexpected hard-to-debug further behavior of running process.
* BUGFIX: vminsert, vmagent: make sure data ingestion connections are closed before completing graceful shutdown. Previously the connection may remain open, which could result in trailing samples loss.
* BUGFIX: vmauth, vmalert: properly re-use HTTP keep-alive connections to backends and datasources. Previously only 2 keep-alive connections per backend could be re-used. Other connections were closed after the first request. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1300) for details.
* BUGFIX: vmalert: fix false positive error `result contains metrics with the same labelset after applying rule labels`, which could be triggered when recording rules generate unique metrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1293).
* BUGFIX: vmctl: properly import InfluxDB rows if they have a field and a tag with identical names. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1299).
* BUGFIX: properly reload configs if `SIGHUP` signal arrives during service initialization. Previously such `SIGHUP` signal could be ingonred and configs weren't reloaded.
* BUGFIX: vmalert: properly import default rules from OpenShift. See [this issue](https://github.com/VictoriaMetrics/operator/issues/243).
* BUGFIX: reduce the probability of `the removal queue is full` panic when highly loaded VictoriaMetrics stores data on NFS. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1313).


## [v1.59.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.59.0)

* FEATURE: improved new time series registration speed on systems with many CPU cores. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1244). Thanks to @waldoweng for the idea and [draft implementation](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1243).
* FEATURE: vmalert: use the same technique as Grafana for determining evaluation timestamps for recording rules. This should make consistent graphs for series generated by recording rules compared to graphs generated for queries from recording rules in Grafana. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1232).
* FEATURE: vmauth: add ability to set madatory query args in `url_prefix`. For example, `url_prefix: http://vm:8428/?extra_label=team=dev` would add `extra_label=team=dev` query arg to all the incoming requests. See [the example](https://docs.victoriametrics.com/vmauth.html#auth-config) for more details.
* FEATURE: vmctl: add OpenTSDB migration option. See more details [here](https://docs.victoriametrics.com/vmctl#migrating-data-from-opentsdb).
Thanks to @johnseekins!
* FEATURE: log metrics with dropped labels if the number of labels in the ingested metric exceeds `-maxLabelsPerTimeseries`. This should simplify debugging for this case.
* FEATURE: vmagent: list user-visible endpoints at `http://vmagent:8429/`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1251).

* BUGFIX: vmagent: properly update `role: endpoints` and `role: endpointslices` scrape targets if the underlying service objects are updated in `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240).
* BUGFIX: vmagent: apply `scrape_timeout` on receiving the first response byte from `stream_parse: true` scrape targets. Previously it was applied to receiving and *processing* the full response stream. This could result in false timeout errors when scrape target exposes millions of metrics as described [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1017#issuecomment-767235047).
* BUGFIX: vmagent: eliminate possible data race when obtaining value for the metric `vm_persistentqueue_bytes_pending`. The data race could result in incorrect value for this metric.
* BUGFIX: vmstorage: remove empty directories on startup. Such directories can be left after unclean shutdown on NFS storage. Previously such directories could lead to crashloop until manually removed. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1142).


## [v1.58.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.58.0)

* FEATURE: vminsert and vmagent: add `-sortLabels` command-line flag for sorting metric labels before pushing them to `vmstorage`. This should reduce the size of `MetricName -> internal_series_id` cache (aka `vm_cache_size_bytes{type="storage/tsid"}`) when ingesting samples for the same time series with distinct order of labels. For example, `foo{k1="v1",k2="v2"}` and `foo{k2="v2",k1="v1"}` represent a single time series. Labels sorting is disabled by default, since the majority of established exporters preserve the order of labels for the exported metrics.
* FEATURE: allow specifying label value alongside label name for the `others sum` time series returned from `topk_*` and `bottomk_*` functions from [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html). For example, `topk_avg(3, max(process_resident_memory_bytes) by (instance), "instance=other_sum")` would return top 3 series from `max(process_resident_memory_bytes) by (instance)` plus a series containing the sum of other series. The `others sum` series will have `{instance="other_sum"}` label.
* FEATURE: do not delete `dst_label` when applying `label_copy(q, "src_label", "dst_label")` and `label_move(q, "src_label", "dst_label")` to series without `src_label` and with non-empty `dst_label`. See more details at [MetricsQL docs](https://docs.victoriametrics.com/MetricsQL.html).
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
* FEATURE: vmauth: add support for authorization via [bearer token](https://swagger.io/docs/specification/authentication/bearer-authentication/). See [the docs](https://docs.victoriametrics.com/vmauth.html#auth-config) for details.
* FEATURE: publish `arm64` and `amd64` binaries for cluster version of VictoriaMetrics at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).

* BUGFIX: properly handle `/api/v1/labels` and `/api/v1/label/<label_name>/values` queries on big `start ... end` time range. This should fix big resource usage when VictoriaMetrics is queried with [Promxy](https://github.com/jacksontj/promxy) v0.0.62 or newer versions.
* BUGFIX: do not break sort order for series returned from `topk*`, `bottomk*` and `outliersk` [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) functions. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1189).
* BUGFIX: vmagent: properly work with simple HTTP proxies which don't support `CONNECT` method. For example, [PushProx](https://github.com/prometheus-community/PushProx). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1179).
* BUGFIX: vmagent: properly discover targets if multiple namespace selectors are put inside `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1170).
* BUGFIX: vmagent: properly discover `role: endpoints` and `role: endpointslices` targets in `kubernetes_sd_config`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1182).
* BUGFIX: properly generate filename for `*.tar.gz` archive inside `_checksums.txt` file posted at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1171).


## [v1.57.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.57.1)

* FEATURE: publish vmutils for `GOOS=arm` on [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).

* BUGFIX: prevent from possible incomplete query results after timed out query.
* BUGFIX: vmselect: remove `-search.storageTimeout` command-line flag, since it has the same meaning as `-search.maxQueryDuration`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711#issuecomment-808884995).
* BUGFIX: vminsert: return back `type` label to per-tenant metric `vm_tenant_inserted_rows_total`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/932).


## [v1.57.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.57.0)

* FEATURE: optimize query performance by up to 10x on systems with many CPU cores. See [this tweet](https://twitter.com/MetricsVictoria/status/1375064484860067840).
* FEATURE: add the following metrics at `/metrics` page for every VictoraMetrics app:
  * `process_resident_memory_anon_bytes` - RSS share for memory allocated by the process itself.  This share cannot be freed by the OS, so it must be taken into account by OOM killer.
  * `process_resident_memory_file_bytes` - RSS share for page cache memory (aka memory-mapped files). This share can be freed by the OS at any time, so it must be ignored by OOM killer.
  * `process_resident_memory_shared_bytes` - RSS share for memory shared with other processes (aka shared memory). This share can be freed by the OS at any time, so it must be ignored by OOM killer.
  * `process_resident_memory_peak_bytes` - peak RSS usage for the process.
  * `process_virtual_memory_peak_bytes` - peak virtual memory usage for the process.
* FEATURE: accept and enforce `extra_label=<label_name>=<label_value>` query arg at [Graphite APIs](https://docs.victoriametrics.com/#graphite-api-usage).
* FEATURE: use Influx field as metric name if measurement is empty and `-influxSkipSingleField` command-line is set. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1139).
* FEATURE: vmagent: add `-promscrape.consul.waitTime` command-line flag for tuning the maximum wait time for Consul service discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1144).
* FEATURE: vmagent: add `vm_promscrape_discovery_kubernetes_stale_resource_versions_total` metric for monitoring the frequency of `too old resource version` errors during Kubernetes service discovery.
* FEATURE: single-node VictoriaMetrics: log metrics with timestamps older than `-search.cacheTimestampOffset` compared to the current time. See [these docs](https://docs.victoriametrics.com/#backfilling) for details.

* BUGFIX: prevent from infinite loop on `{__graphite__="..."}` filters when a metric name contains `*`, `{` or `[` chars.
* BUGFIX: prevent from infinite loop in `/metrics/find` and `/metrics/expand` [Graphite Metrics API handlers](https://docs.victoriametrics.com/#graphite-metrics-api-usage) when they match metric names or labels with `*`, `{` or `[` chars.
* BUGFIX: do not merge duplicate time series during requests to `/api/v1/query`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1141
* BUGFIX: vmagent: properly handle `too old resource version` error messages from Kubernetes watch API. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1150
* BUGFIX: vmagent: do not retry sending data blocks if remote storage returns `400 Bad Request` error. The number of dropped blocks due to such errors can be monitored with `vmagent_remotewrite_packets_dropped_total` metrics. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1149
* BUGFIX: properly calculate `summarize` and `*Series` functions in [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage).


## [v1.56.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.56.0)

* FEATURE: add the following functions to [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html):
  - `histogram_avg(buckets)` - returns the average value for the given buckets.
  - `histogram_stdvar(buckets)` - returns standard variance for the given buckets.
  - `histogram_stddev(buckets)` - returns standard deviation for the given buckets.
* FEATURE: export `vm_available_memory_bytes` and `vm_available_cpu_cores` metrics, which show the number of available RAM and available CPU cores for VictoriaMetrics apps.
* FEATURE: export `vm_index_search_duration_seconds` histogram, which can be used for troubleshooting time series search performance.
* FEATURE: vmagent: add ability to replicate scrape targets among `vmagent` instances in the cluster with `-promscrape.cluster.replicationFactor` command-line flag. See [these docs](https://docs.victoriametrics.com/vmagent.html#scraping-big-number-of-targets).
* FEATURE: vmagent: accept `scrape_offset` option at `scrape_config`. This option may be useful when scrapes must start at the specified offset of every scrape interval. See [these docs](https://docs.victoriametrics.com/vmagent.html#troubleshooting) for details.
* FEATURE: vmagent: support `proxy_tls_config`, `proxy_basic_auth`, `proxy_bearer_token` and `proxy_bearer_token_file` options at `scrape_config` section for configuring proxies specified via `proxy_url`. See [these docs](https://docs.victoriametrics.com/vmagent.html#scraping-targets-via-a-proxy).
* FEATURE: vmauth: allow using regexp paths in `url_map`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1112) for details.
* FEATURE: accept `round_digits` query arg at `/api/v1/query` and `/api/v1/query_range` handlers. This option can be set at Prometheus datasource in Grafana for limiting the number of digits after the decimal point in response values.
* FEATURE: add `-influx.databaseNames` command-line flag, which can be used for accepting data from some Telegraf plugins such as [fluentd plugin](https://github.com/fangli/fluent-plugin-influxdb). See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1124).
* FEATURE: add `-logNewSeries` command-line flag, which can be used for debugging the source of time series churn rate.
* FEATURE: publish Windows builds for [vmagent](https://docs.victoriametrics.com/vmagent.html), [vmalert](https://docs.victoriametrics.com/vmalert.html), [vmauth](https://docs.victoriametrics.com/vmauth.html) and [vmctl](https://docs.victoriametrics.com/vmctl.html) at `vmutils-windows-*.zip` archives at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
* FEATURE: listen for IPv6 UDP if `-enableTCP6` command-line flag is passed to VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1131).

* BUGFIX: vmagent: prevent from high CPU usage bug during failing scrapes with small `scrape_timeout` (less than a few seconds).
* BUGFIX: vmagent: reduce memory usage when Kubernetes service discovery is used in big number of distinct scrape config jobs by sharing Kubernetes object cache. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1113
* BUGFIX: vmagent: apply `sample_limit` only after `metric_relabel_configs` are applied as Prometheus does. Previously the `sample_limit` was applied before metrics relabeling.
* BUGFIX: vmagent: properly apply `tls_config`, `basic_auth` and `bearer_token` to proxy connections if `proxy_url` option is set. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1116
* BUGFIX: vmagent: properly scrape targets via https proxy specified in `proxy_url` if `insecure_skip_verify` flag isn't set in `tls_config` section. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1116
* BUGFUX: avoid `duplicate time series` error if `prometheus_buckets()` covers a time range with distinct set of buckets.
* BUGFIX: prevent exponent overflow when processing extremely small values close to zero such as `2.964393875E-314`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1114
* BUGFIX: do not include datapoints with a timestamp `t-d` when returning results from `/api/v1/query?query=m[d]&time=t` as Prometheus does.
* BUGFIX: do not crash if a query contains `histogram_over_time()` function name with uppercase chars. For example, `Histogram_Over_Time(m[5m])`.


## [v1.55.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.55.1)

* BUGFIX: vmagent: fix a panic in Kubernetes service discovery when a target is filtered out with relabeling. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1107
* BUGFIX: vmagent: fix Kubernetes service discovery for `role: ingress`. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1110


## [v1.55.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.55.0)


* FEATURE: add `sign(q)` and `clamp(q, min, max)` functions, which are planned to be added in [the upcoming Prometheus release](https://twitter.com/roidelapluie/status/1363428376162295811) . The `last_over_time(m[d])` function is already supported in [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html).
* FEATURE: vmagent: add `scrape_align_interval` config option, which can be used for aligning scrapes to the beginning of the configured interval. See [these docs](https://docs.victoriametrics.com/vmagent.html#troubleshooting) for details.
* FEATURE: expose io-related metrics at `/metrics` page for every VictoriaMetrics component:
  * `process_io_read_bytes_total` - the number of bytes read via io syscalls such as read and pread
  * `process_io_written_bytes_total` - the number of bytes written via io syscalls such as write and pwrite
  * `process_io_read_syscalls_total` - the number of read syscalls such as read and pread
  * `process_io_write_syscalls_total` - the number of write syscalls such as write and pwrite
  * `process_io_storage_read_bytes_total` - the number of bytes read from storage layer
  * `process_io_storage_written_bytes_total` - the number of bytes written to storage layer
* FEATURE: vmagent: add ability to spread scrape targets among multiple `vmagent` instances. See [these docs](https://docs.victoriametrics.com/vmagent.html#scraping-big-number-of-targets) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1084) for details.
* FEATURE: vmagent: use watch API for Kuberntes service discovery. This should reduce load on Kuberntes API server when it tracks big number of objects (for example, 10K pods). This should also reduce the time needed for k8s targets discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1057) for details.
* FEATURE: vmagent: export `vm_promscrape_target_relabel_duration_seconds` metric, which can be used for monitoring the time spend on relabeling for discovered targets.
* FEATURE: vmagent: optimize [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling) performance for common cases.
* FEATURE: add `increase_pure(m[d])` function to MetricsQL. It works the same as `increase(m[d])` except of various edge cases. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962) for details.
* FEATURE: increase accuracy for `buckets_limit(limit, buckets)` results for small `limit` values. See [MetricsQL docs](https://docs.victoriametrics.com/MetricsQL.html) for details.
* FEATURE: vmagent: initial support for Windows build with `CGO_ENABLED=0 GOOS=windows go build -mod=vendor ./app/vmagent`. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/70) and [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1036).
* FEATURE: vmagent: support WebIdentityToken auth in EC2 service discovery. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1080) for details.
* FEATURE: vmalert: properly process query params in `-datasource.url` and `-remoteRead.url` command-line flags. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1087) for details.

* BUGFIX: vmagent: properly apply `-remoteWrite.rateLimit` when `-remoteWrite.queues` is greater than 1. Previously there was a data race, which could prevent from proper rate limiting.
* BUGFIX: vmagent: properly perform graceful shutdown on `SIGINT` and `SIGTERM` signals. The graceful shutdown has been broken in `v1.54.0`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1065
* BUGFIX: reduce the probability of `duplicate time series` errors when querying Kubernetes metrics.
* BUGFIX: properly calculate `histogram_quantile()` over time series with only a single non-zero bucket with `{le="+Inf"}`. Previously `NaN` was returned, now the value for the last bucket before `{le="+Inf"}` is returned like Prometheus does.
* BUGFIX: vmselect: do not cache partial query results on timeout when receiving data from `vmstorage` nodes. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1085
* BUGFIX: properly handle `stale NFS file handle` error.
* BUGFIX: properly cache query results when `extra_label` query arg is used. Previously the cached results could clash for different `extra_label` values. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1095
* BUGFIX: fix `http: superfluous response.WriteHeader call` issue. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1078
* BUGFIX: fix arm64 builds due to the issue in `github.com/golang/snappy`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1074
* BUGFIX: fix `index out of range [1024819115206086200] with length 27` panic, which could occur when `1e-9` value is passed to VictoriaMetrics histogram. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1096
* BUGFIX: fix parsing for Graphite line with empty tags such as `foo; 123 456`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1100
* BUGFIX: unescape only `\\`, `\n` and `\"` in label names when parsing Prometheus text exposition format as Prometheus does. Previously other escape sequences could be improperly unescaped.


## [v1.54.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.1)

* BUGFIX: properly handle queries containing a filter on metric name plus any number of negative filters and zero non-negative filters. For example, `node_cpu_seconds_total{mode!="idle"}`. The bug was introduced in [v1.54.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.0).


## [v1.54.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.54.0)

* FEATURE: optimize searching for matching metrics for `metric{<label_filters>}` queries if `<label_filters>` contains at least a single filter. For example, the query `up{job="foobar"}` should find the matching time series much faster than previously.
* FEATURE: reduce execution times for `q1 <binary_op> q2` queries by executing `q1` and `q2` in parallel.
* FEATURE: switch from Go1.15 to [Go1.16](https://golang.org/doc/go1.16) for building prod binaries.
* FEATURE: single-node VictoriaMetrics now accepts requests to handlers with `/prometheus` and `/graphite` prefixes such as `/prometheus/api/v1/query`. This improves compatibility with [handlers from VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format).
* FEATURE: expose `process_open_fds` and `process_max_fds` metrics. These metrics can be used for alerting when `process_open_fds` reaches `process_max_fds`. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/402 and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1037
* FEATURE: vmalert: add `-datasource.appendTypePrefix` command-line option for querying both Prometheus and Graphite datasource in cluster version of VictoriaMetrics. See [these docs](https://docs.victoriametrics.com/vmalert.html#graphite) for details.
* FEATURE: vmauth: add ability to route requests from a single user to multiple destinations depending on the requested paths. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1064
* FEATURE: remove dependency on external programs such as `cat`, `grep` and `cut` when detecting cpu and memory limits inside Docker or LXC container.
* FEATURE: vmagent: add `__meta_kubernetes_endpoints_label_*`, `__meta_kubernetes_endpoints_labelpresent_*`, `__meta_kubernetes_endpoints_annotation_*` and `__meta_kubernetes_endpoints_annotationpresent_*` labels for `role: endpoints` in Kubernetes service discovery. These labels where added in Prometheus 2.25.
* FEATURE: reduce the minimum supported retention period for inverted index (aka `indexdb`) from one month to one day. This should reduce disk space usage for `<-storageDataPath>/indexdb` folder if `-retentionPeriod` is set to values smaller than one month.
* FEATURE: vmselect: export per-tenant metrics `vm_vmselect_http_requests_total` and `vm_vmselect_http_requests_duration_ms_total` . Other per-tenant metrics are available as a part of [enterprise package](https://victoriametrics.com/enterprise.html). See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/932 for details.

* BUGFIX: properly convert regexp tag filters containing escaped dots to non-regexp tag filters. For example, `{foo=~"bar\.baz"}` should be converted to `{foo="bar.baz"}`. Previously it was incorrectly converted to `{foo="bar\.baz"}`, which could result in missing time series for this tag filter.
* BUGFIX: do not spam error logs when discovering Docker Swarm targets without dedicated IP. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1028 .
* BUGFIX: properly embed timezone data into VictoriaMetrics apps. This should fix `-loggerTimezone` usage inside Docker containers.
* BUGFIX: properly build Docker images for non-amd64 architectures (arm, arm64, ppc64le, 386) on [Docker hub](https://hub.docker.com/u/victoriametrics/). Previously these images were incorrectly based on amd64 base image, so they didn't work.
* BUGFIX: vmagent: return back unsent block to the queue during graceful shutdown. Previously this block could be dropped if remote storage is unavailable during vmagent shutdown. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1065 .


## [v1.53.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.53.1)

* BUGFIX: vmselect: fix the bug peventing from proper searching by Graphite filter with wildcards such as `{__graphite__="foo.*.bar"}`.


## [v1.53.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.53.0)

* FEATURE: added [vmctl tool](https://docs.victoriametrics.com/vmctl.html) to VictoriaMetrics release process. Now it is packaged in `vmutils-*.tar.gz` archive on [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases). Source code for `vmctl` tool has been moved from [github.com/VictoriaMetrics/vmctl](https://github.com/VictoriaMetrics/vmctl) to [github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmctl).
* FEATURE: added `-loggerTimezone` command-line flag for adjusting time zone for timestamps in log messages. By default UTC is used.
* FEATURE: added `-search.maxStepForPointsAdjustment` command-line flag, which can be used for disabling adjustment for points returned by `/api/v1/query_range` handler if such points have timestamps closer than `-search.latencyOffset` to the current time. Such points may contain incomplete data, so they are substituted by the previous values for `step` query args smaller than one minute by default.
* FEATURE: vmselect: added ability to use Graphite-compatible filters in MetricsQL via `{__graphite__="foo.*.bar"}` syntax. This expression is equivalent to `{__name__=~"foo[.][^.]*[.]bar"}`, but it works faster and it is easier to use when migrating from Graphite to VictoriaMetrics. This feature deprecates the usage of `-search.treatDotsAsIsInRegexps` command-line flag.
* FEATURE: vmselect: added ability to set additional label filters, which must be applied during queries. Such label filters can be set via optional `extra_label` query arg, which is accepted by [querying API](https://docs.victoriametrics.com/#prometheus-querying-api-usage) handlers. For example, the request to `/api/v1/query_range?extra_label=tenant_id=123&query=<query>` adds `{tenant_id="123"}` label filter to the given `<query>`. It is expected that the `extra_label` query arg is automatically set by auth proxy sitting
in front of VictoriaMetrics. [Contact us](mailto:sales@victoriametrics.com) if you need assistance with such a proxy. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1021 .
* FEATURE: vmalert: added `-datasource.queryStep` command-line flag for passing optional `step` query arg to `/api/v1/query` endpoint. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1025
* FEATURE: vmalert: added ability to query Graphite datasource when evaluating alerting and recording rules. See [these docs](https://docs.victoriametrics.com/vmalert.html#graphite) for details.
* FEATURE: vmagent: added `-remoteWrite.roundDigits` command-line option for rounding metric values to the given number of decimal digits after the point before sending the metric to the corresponding `-remoteWrite.url`. This option can be used for improving data compression on the remote storage, because values with lower number of decimal digits can be compressed better than values with bigger number of decimal digits.
* FEATURE: vmagent: added `-remoteWrite.rateLimit` command-line flag for limiting data transfer rate to `-remoteWrite.url`. This may be useful when big amounts of buffered data is sent after temporarily unavailability of the remote storage. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1035
* FEATURE: vmagent: export the following additional metrics, which may be useful during troubleshooting:
  - `vm_promscrape_scrapes_failed_per_url_total`
  - `vm_promscrape_scrapes_skipped_by_sample_limit_per_url_total`
  - `vm_promscrape_discovery_requests_total`
  - `vm_promscrape_discovery_retries_total`
  - `vm_promscrape_scrape_retries_total`
  - `vm_promscrape_service_discovery_duration_seconds`
* FEATURE: vmselect: initial implementation for [Graphite Render API](https://docs.victoriametrics.com/#graphite-render-api-usage).

* BUGFIX: vmagent: reduce HTTP reconnection rate for scrape targets. Previously vmagent could errorneusly close HTTP keep-alive connections more frequently than needed.
* BUGFIX: vmagent: retry scrape and service discovery requests when the remote server closes HTTP keep-alive connection. Previously `disable_keepalive: true` option could be used under `scrape_configs` section when working with such servers.


## [v1.52.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.52.0)

* FEATURE: provide a sample list of alerting rules for VictoriaMetrics components. It is available [here](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts.yml).
* FEATURE: disable final merge for data for the previous month at the beginning of new month, since it may result in high disk IO and CPU usage. Final merge can be enabled by setting `-finalMergeDelay` command-line flag to positive duration.
* FEATURE: add `tfirst_over_time(m[d])` and `tlast_over_time(m[d])` functions to [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) for returning timestamps for the first and the last data point in `m` over `d` duration.
* FEATURE: add ability to pass multiple labels to `sort_by_label()` and `sort_by_label_desc()` functions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/992 .
* FEATURE: enforce at least TLS v1.2 when accepting HTTPS requests if `-tls`, `-tlsCertFile` and `-tlsKeyFile` command-line flags are set, because older TLS protocols such as v1.0 and v1.1 have been deprecated due to security vulnerabilities.
* FEATURE: support `extra_label` query arg for all HTTP-based [data ingestion protocols](https://docs.victoriametrics.com/#how-to-import-time-series-data). This query arg can be used for specifying extra labels which should be added for the ingested data.
* FEATURE: vmbackup: increase backup chunk size from 128MB to 1GB. This should reduce the number of Object storage API calls during backups by 8x. This may also reduce costs, since object storage API calls usually have non-zero costs. See https://aws.amazon.com/s3/pricing/ and https://cloud.google.com/storage/pricing#operations-pricing .

* BUGFIX: properly parse escaped unicode chars in MetricsQL metric names, label names and function names. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/990
* BUGFIX: override user-provided labels with labels set in `extra_label` query args during data ingestion over HTTP-based protocols.
* BUGFIX: vmagent: prevent from `dialing to the given TCP address time out` error when scraping big number of unavailable targets. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/987
* BUGFIX: vmagent: properly show scrape duration on `/targets` page. Previously it was incorrectly shown as 0.000s.
* BUGFIX: vmagent: properly log errors when `-promscrape.streamParse` command-line flag is set. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1009
* BUGFIX: vmagent: properly suppress errors when both `-promscrape.suppressScrapeErrors` and `-promscrape.streamParse` command-line flags are set. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1009 .
* BUGFIX: vmalert: return non-empty result in template func `query` stub to pass validation. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/989 .
* BUGFIX: upgrade base image for Docker packages from Alpine 3.12.1 to Alpine 3.12.3 in order to fix potential security issues. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1010


## [v1.51.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.51.0)

* FEATURE: add `/api/v1/status/top_queries` handler, which returns the most frequently executed queries and queries that took the most time for execution. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/907
* FEATURE: vmagent: add support for `proxy_url` config option in Prometheus scrape configs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/503
* FEATURE: remove parts with stale data as soon as they go outside the configured `-retentionPeriod`. Previously such parts may remain active for long periods of time. This should help reducing disk usage for `-retentionPeriod` smaller than one month.
* FEATURE: vmalert: allow setting multiple values for `-notifier.tlsInsecureSkipVerify` command-line flag per each `-notifier.url`.

* BUGFIX: vmalert: properly escape multiline queries when passing them to Grafana. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890
* BUGFIX: vmagent: set missing `__meta_kubernetes_service_*` labels in `kubernetes_sd_config` for `endpoints` and `endpointslices` roles. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/982
* BUGFIX: do not adjust `offset` value provided in MetricsQL query. Previously it could be modified in order to improve response cache hit ratio. This is unneeded, since cache hit ratio should remain good because the query time range should be already aligned to multiple of `step` values. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/976


## [v1.50.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.2)

* FEATURE: do not publish duplicate Docker images with `-cluster` tag suffix for [vmagent](https://docs.victoriametrics.com/vmagent.html), [vmalert](https://docs.victoriametrics.com/vmalert.html), [vmauth](https://docs.victoriametrics.com/vmauth.html), [vmbackup](https://docs.victoriametrics.com/vmbackup.html) and [vmrestore](https://docs.victoriametrics.com/vmrestore.html), since they are identical to images without `-cluster` tag suffix.

* BUGFIX: vmalert: properly populate template variables. This has been broken in v1.50.0. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/974
* BUGFIX: properly parse negative combined duration in MetricsQL such as `-1h3m4s`. It must be parsed as `-(1h + 3m + 4s)`. Prevsiously it was parsed as `-1h + 3m + 4s`.
* BUGFIX: properly parse lines in [Prometheus exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md) and in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md) with whitespace after the timestamp. For example, `foo 123 456 ## some comment here`. See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/970


## [v1.50.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.1)

* FEATURE: vmagent: export `vmagent_remotewrite_blocks_sent_total` and `vmagent_remotewrite_blocks_sent_total` metrics for each `-remoteWrite.url`.

* BUGFIX: vmagent: properly delete unregistered scrape targets from `/targets` and `/api/v1/targets` pages. They weren't deleted due to the bug in `v1.50.0`.


## [v1.50.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.50.0)

* FEATURE: automatically reset response cache when samples with timestamps older than `now - search.cacheTimestampOffset` are ingested to VictoriaMetrics. This makes unnecessary disabling response cache during data backfilling or resetting it after backfilling is complete as described [in these docs](https://docs.victoriametrics.com/#backfilling). This feature applies only to single-node VictoriaMetrics. It doesn't apply to cluster version of VictoriaMetrics because `vminsert` nodes don't know about `vmselect` nodes where the response cache must be reset.
* FEATURE: vmalert: add `query`, `first` and `value` functions to alert templates. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/539
* FEATURE: vmagent: return user-friendly HTML page when requesting `/targets` page from web browser. The page is returned in the old plaintext format when requesting via curl or similar tool.
* FEATURE: allow multiple whitespace chars between measurements, fields and timestamp when parsing InfluxDB line protocol.
  Though [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.8/write_protocols/line_protocol_tutorial/) denies multiple whitespace chars between these entities,
  some apps improperly put multiple whitespace chars. This workaround allows accepting data from such apps.
* FEATURE: export `vm_promscrape_active_scrapers{type="<sd_type>"}` metric for tracking the number of active scrapers per each service discovery type.
* FEATURE: export `vm_promscrape_scrapers_started_total{type="<sd_type>"}` and `vm_promscrape_scrapers_stopped_total{type="<sd_type>"}` metrics for tracking churn rate for scrapers
  per each service discovery type.
* FEATURE: vmagent: allow setting per-`-remoteWrite.url` command-line flags for `-remoteWrite.sendTimeout` and `-remoteWrite.tlsInsecureSkipVerify`.

* BUGFIX: properly handle `*` and `[...]` inside curly braces in query passed to Graphite Metrics API. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/952
* BUGFIX: vmagent: fix memory leak when big number of targets is discovered via service discovery.
* BUGFIX: vmagent: properly pass `datacenter` filter to Consul API server. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/574#issuecomment-740454170
* BUGFIX: properly handle CPU limits set on the host system or host container. The bugfix may result in lower memory usage on systems with CPU limits. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/946
* BUGFIX: prevent from duplicate `name` tag returned from `/tags/autoComplete/tags` handler. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/942
* BUGFIX: do not enable strict parsing for `-promscrape.config` if `-promscrape.config.dryRun` comand-line flag is set. Strict parsing can be enabled with `-promscrape.config.strictParse` command-line flag. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/944
* BUGFIX: vminsert: properly update `vm_rpc_rerouted_rows_processed_total` metric. Previously it wasn't updated. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/955
* BUGFIX: vmagent: properly recover when opening incorrectly stored persistent queue. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/964
* BUGFIX: vmagent: properly handle scrape errors when stream parsing is enabled with `-promscrape.streamParse` command-line flag or with `stream_parse: true` per-target config option. Previously such errors weren't reported at `/targets` page. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/967
* BUGFIX: assume the previous value is 0 when calculating `increase()` for the first point on the graph if its value doesn't exceed 100 and the delta between two first points equals to 0. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/962


## [v1.49.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.49.0)

* FEATURE: optimize Consul service discovery speed when discovering big number of services. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/574
* FEATURE: add `label_uppercase(q, label1, ... labelN)` and `label_lowercase(q, label1, ... labelN)` function to [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html)
  for uppercasing and lowercasing values for the given labels. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/936
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
* FEATURE: add `/tags/delSeries` handler from Graphite Tags API. See https://docs.victoriametrics.com/#graphite-tags-api-usage
* FEATURE: log metric name plus all its labels when the metric timestamp is out of the configured retention. This should simplify detecting the source of metrics with unexpected timestamps.
* FEATURE: add `-dryRun` command-line flag to single-node VictoriaMetrics in order to check config file pointed by `-promscrape.config`.

* BUGFIX: properly parse Prometheus metrics with [exemplars](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#exemplars-1) such as `foo 123 ## {bar="baz"} 1`.
* BUGFIX: properly parse "infinity" values in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#abnf).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/924


## [v1.47.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.47.0)

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
  See [troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting) for more details.
* FEATURE: added [Graphite Tags API](https://graphite.readthedocs.io/en/stable/tags.html) support.
  See [these docs](https://docs.victoriametrics.com/#graphite-tags-api-usage) for details.

* BUGFIX: do not return data points in the end of the selected time range for time series ending in the middle of the selected time range.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/887 and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/845
* BUGFIX: remove spikes at the end of time series gaps for `increase()` or `delta()` functions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/894
* BUGFIX: vminsert: properly return HTTP 503 status code when all the vmstorage nodes are unavailable. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/896


## [v1.46.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.46.0)

* FEATURE: optimize requests to `/api/v1/labels` and `/api/v1/label/<name>/values` when `start` and `end` args are set.
* FEATURE: reduce memory usage when query touches big number of time series.
* FEATURE: vmagent: reduce memory usage when `kubernetes_sd_config` discovers big number of scrape targets (e.g. hundreds of thousands) and the majority of these targets (99%)
  are dropped during relabeling. Previously labels for all the dropped targets were displayed at `/api/v1/targets` page. Now only up to `-promscrape.maxDroppedTargets` such
  targets are displayed. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/878 for details.
* FEATURE: vmagent: reduce memory usage when scraping big number of targets with big number of temporary labels starting with `__`.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825
* FEATURE: vmagent: add `/ready` HTTP endpoint, which returns 200 OK status code when all the service discovery has been initialized.
  This may be useful during rolling upgrades. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/875

* BUGFIX: vmagent: eliminate data race when `-promscrape.streamParse` command-line is set. Previously this mode could result in scraped metrics with garbage labels.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-723198247 for details.
* BUGFIX: properly calculate `topk_*` and `bottomk_*` functions from [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) for time series with gaps.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/883


## [v1.45.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.45.0)

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
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 and [troubleshooting docs for vmagent](https://docs.victoriametrics.com/vmagent.html#troubleshooting).
* FEATURE: vmalert: add `-dryRun` command-line option for validating the provided config files without the need to start `vmalert` service.
* FEATURE: accept optional third argument of string type at `topk_*` and `bottomk_*` functions. This is label name for additional time series to return with the sum of time series outside top/bottom K. See [MetricsQL docs](https://docs.victoriametrics.com/MetricsQL.html) for more details.
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


## [v1.44.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.44.0)

* FEATURE: automatically add missing label filters to binary operands as described at https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization .
  This should improve performance for queries with missing label filters in binary operands. For example, the following query should work faster now, because it shouldn't
  fetch and discard time series for `node_filesystem_files_free` metric without matching labels for the left side of the expression:
  ```
     node_filesystem_files{ host="$host", mountpoint="/" } - node_filesystem_files_free
  ```
* FEATURE: vmagent: add Docker Swarm service discovery (aka [dockerswarm_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config)).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/656
* FEATURE: add ability to export data in CSV format. See [these docs](https://docs.victoriametrics.com/#how-to-export-csv-data) for details.
* FEATURE: vmagent: add `-promscrape.suppressDuplicateScrapeTargetErrors` command-line flag for suppressing `duplicate scrape target` errors.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651 and https://docs.victoriametrics.com/vmagent.html#troubleshooting .
* FEATURE: vmagent: show original labels before relabeling is applied on `duplicate scrape target` errors. This should simplify debugging for incorrect relabeling.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651
* FEATURE: vmagent: `/targets` page now accepts optional `show_original_labels=1` query arg for displaying original labels for each target before relabeling is applied.
  This should simplify debugging for target relabeling configs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/651
* FEATURE: add `-finalMergeDelay` command-line flag for configuring the delay before final merge for per-month partitions.
  The final merge is started after no new data is ingested into per-month partition during `-finalMergeDelay`.
* FEATURE: add `vm_rows_added_to_storage_total` metric, which shows the total number of rows added to storage since app start.
  The `sum(rate(vm_rows_added_to_storage_total))` can be smaller than `sum(rate(vm_rows_inserted_total))` if certain metrics are dropped
  due to [relabeling](https://docs.victoriametrics.com/#relabeling). The `sum(rate(vm_rows_added_to_storage_total))` can be bigger
  than `sum(rate(vm_rows_inserted_total))` if [replication](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#replication-and-data-safety) is enabled.
* FEATURE: keep metric name after applying [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) functions, which don't change time series meaning.
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


## [v1.43.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.43.0)

* FEATURE: reduce CPU usage for repeated queries over sliding time window when no new time series are added to the database.
  Typical use cases: repeated evaluation of alerting rules in [vmalert](https://docs.victoriametrics.com/vmalert.html) or dashboard auto-refresh in Grafana.
* FEATURE: vmagent: add OpenStack service discovery aka [openstack_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config).
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/728 .
* FEATURE: vmalert: make `-maxIdleConnections` configurable for datasource HTTP client. This option can be used for minimizing connection churn.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/795 .
* FEATURE: add `-influx.maxLineSize` command-line flag for configuring the maximum size for a single Influx line during parsing.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/807

* BUGFIX: properly handle `inf` values during [background merge of LSM parts](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
  Previously `Inf` values could result in `NaN` values for adjacent samples in time series. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/805 .
* BUGFIX: fill gaps on graphs for `range_*` and `running_*` functions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/806 .
* BUGFIX: make a copy of label with new name during relabeling with `action: labelmap` in the same way as Prometheus does.
  Previously the original label name has been replaced. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/812 .
* BUGFIX: support parsing floating-point timestamp like Graphite Carbon does. Such timestmaps are truncated to seconds.


## [v1.42.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0)

* FEATURE: use all the available CPU cores when accepting data via a single TCP connection
  for [all the supported protocols](https://docs.victoriametrics.com/#how-to-import-time-series-data).
  Previously data ingested via a single TCP connection could use only a single CPU core. This could limit data ingestion performance.
  The main benefit of this feature is that data can be imported at max speed via a single connection - there is no need to open multiple concurrent
  connections to VictoriaMetrics or [vmagent](https://docs.victoriametrics.com/vmagent.html) in order to achieve the maximum data ingestion speed.
* FEATURE: cluster: improve performance for data ingestion path from `vminsert` to `vmstorage` nodes. The maximum data ingestion performance
  for a single connection between `vminsert` and `vmstorage` node scales with the number of available CPU cores on `vmstorage` side.
  This should help with https://github.com/VictoriaMetrics/VictoriaMetrics/issues/791 .
* FEATURE: add ability to export / import data in native format via `/api/v1/export/native` and `/api/v1/import/native`.
  This is the most optimized approach for data migration between VictoriaMetrics instances. Both single-node and cluster instances are supported.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/787#issuecomment-700632551 .
* FEATURE: add `reduce_mem_usage` query option to `/api/v1/export` in order to reduce memory usage during data export / import.
  See [these docs](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format) for details.
* FEATURE: improve performance for `/api/v1/series` handler when it returns big number of time series.
* FEATURE: add `vm_merge_need_free_disk_space` metric, which can be used for estimating the number of deferred background data merges due to the lack of free disk space.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/686 .
* FEATURE: add OpenBSD support. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/785 .

* BUGFIX: properly apply `-search.maxStalenessInterval` command-line flag value. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/784 .
* BUGFIX: fix displaying data in Grafana tables. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/720 .
* BUGFIX: do not adjust the number of detected CPU cores found at `/sys/devices/system/cpu/online`.
  The adjustment was increasing the resulting GOMAXPROC by 1, which looked confusing to users.
  See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-698595309 .
* BUGFIX: vmagent: do not show `-remoteWrite.url` in initial logs if `-remoteWrite.showURL` isn't set. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/773 .
* BUGFIX: properly handle case when [/metrics/find](https://docs.victoriametrics.com/#graphite-metrics-api-usage) finds both a leaf and a node for the given `query=prefix.*`.
  In this case only the node must be returned with stripped dot in the end of id as carbonapi does.


## Previous releases

See [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
