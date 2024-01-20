---
sort: 7
weight: 7
title: VictoriaLogs changelog
menu:
  docs:
    identifier: "victorialogs-changelog"
    parent: "victorialogs"
    weight: 7
    title: CHANGELOG
aliases:
- /VictoriaLogs/CHANGELOG.html
---

# VictoriaLogs changelog

The following `tip` changes can be tested by building VictoriaLogs from the latest commit of [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/) repository
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html#building-from-source-code)

## tip

## [v0.4.2](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.2-victorialogs)

Released at 2023-11-15

* BUGFIX: properly locate logs for the [requested streams](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stream-filter). Previously logs for some streams may be missing in query results. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4856). Thanks to @XLONG96 for [the fix](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5295)!
* BUGFIX: [web UI](https://docs.victoriametrics.com/VictoriaLogs/querying/#web-ui): properly sort found logs by time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5300).

## [v0.4.1](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.1-victorialogs)

Released at 2023-10-04

* BUGFIX: fix the free space verification process in VictoriaLogs that was erroneously shifting to read-only mode, despite there being sufficient free space available. See [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5112) issue.

## [v0.4.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.4.0-victorialogs)

Released at 2023-10-03

* FEATURE: add `-elasticsearch.version` command-line flag, which can be used for specifying Elasticsearch version returned by VictoriaLogs to Filebeat at [elasticsearch bulk API](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#elasticsearch-bulk-api). This helps resolving [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4777).
* FEATURE: expose the following metrics at [/metrics](https://docs.victoriametrics.com/VictoriaLogs/#monitoring) page:
  * `vl_data_size_bytes{type="storage"}` - on-disk size for data excluding [log stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) indexes.
  * `vl_data_size_bytes{type="indexdb"}` - on-disk size for [log stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) indexes.
* FEATURE: add `-insert.maxFieldsPerLine` command-line flag, which can be used for limiting the number of fields per line in logs sent to VictoriaLogs via ingestion protocols. This helps to avoid issues like [this](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762).
* FEATURE: expose `vl_http_request_duration_seconds` histogram at the [/metrics](https://docs.victoriametrics.com/VictoriaLogs/#monitoring) page. Thanks to @crossoverJie for [this pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4934).
* FEATURE: add support of `-storage.minFreeDiskSpaceBytes` command-line flag to allow switching to read-only mode when running out of disk space at `-storageDataPath`. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4737).

* BUGFIX: fix possible panic when no data is written to VictoriaLogs for a long time. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4895). Thanks to @crossoverJie for filing and fixing the issue.
* BUGFIX: add `/insert/loky/ready` endpoint, which is used by Promtail for healthchecks. This should remove `unsupported path requested: /insert/loki/ready` warning logs. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762#issuecomment-1690966722).
* BUGFIX: prevent from panic during background merge when the number of columns in the resulting block exceeds the maximum allowed number of columns per block. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4762).

## [v0.3.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.3.0-victorialogs)

Released at 2023-07-20

* FEATURE: add support for data ingestion via Promtail (aka default log shipper for Grafana Loki). See [these](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/Promtail.html) and [these](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#loki-json-api) docs.

## [v0.2.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.2.0-victorialogs)

Released at 2023-07-17

* FEATURE: support short form of `_time` filters over the last X minutes/hours/days/etc. For example, `_time:5m` is a short form for `_time:(now-5m, now]`, which matches logs with [timestamps](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field) for the last 5 minutes. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#time-filter) for details.
* FEATURE: add ability to specify offset for the selected time range. For example, `_time:5m offset 1h` is equivalent to `_time:(now-5m-1h, now-1h]`. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#time-filter) for details.
* FEATURE: [LogsQL](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html): replace `exact_prefix("...")` with `exact("..."*)`. This makes it consistent with [i()](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#case-insensitive-filter) filter, which can accept phrases and prefixes, e.g. `i("phrase")` and `i("phrase"*)`. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#exact-prefix-filter).

## [v0.1.0](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v0.1.0-victorialogs)

Released at 2023-06-21

Initial release
