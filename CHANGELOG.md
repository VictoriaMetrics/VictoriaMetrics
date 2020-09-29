# tip


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


# Previous versions

See [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
