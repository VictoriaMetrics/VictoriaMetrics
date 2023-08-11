# VictoriaLogs changelog

The following `tip` changes can be tested by building VictoriaLogs from the latest commit of [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/) repository
according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html#building-from-source-code)

## tip

* FEATURE: add `-elasticsearch.version` command-line flag, which can be used for specifying Elasticsearch version returned by VictoriaLogs to Filebeat at [elasticsearch bulk API](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#elasticsearch-bulk-api). This helps resolving [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4777).
* FEATURE: expose the following metrics at [/metrics](monitoring) page:
  * `vl_data_size_bytes{type="storage"}` - on-disk size for data excluding [log stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) indexes.
  * `vl_data_size_bytes{type="indexdb"}` - on-disk size for [log stream](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) indexes.

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
