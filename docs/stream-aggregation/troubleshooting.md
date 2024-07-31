---
sort: 5
weight: 5
title: Troubleshooting
menu:
  docs:
    identifier: stream-aggregation-troubleshooting
    parent: 'stream-aggregation'
    weight: 5
aliases:
- /stream-aggregation/troubleshooting/
- /stream-aggregation/troubleshooting/index.html
---

## Known scenarios

- [Unexpected spikes for `total` or `increase` outputs](#staleness).
- [Lower than expected values for `total_prometheus` and `increase_prometheus` outputs](#staleness).
- [High memory usage and CPU usage](#high-resource-usage).
- [Unexpected results in vmagent cluster mode](#cluster-mode).

### Staleness

The following outputs track the last seen per-series values in order to properly calculate output values:

- [rate_sum](./configuration/outputs/#rate_sum)
- [rate_avg](./configuration/outputs/#rate_avg)
- [total](./configuration/outputs/#total)
- [total_prometheus](./configuration/outputs/#total_prometheus)
- [increase](./configuration/outputs/#increase)
- [increase_prometheus](./configuration/outputs/#increase_prometheus)
- [histogram_bucket](./configuration/outputs/#histogram_bucket)

The last seen per-series value is dropped if no new samples are received for the given time series during two consecutive aggregation
intervals specified in [stream aggregation config](./configuration/README.md) via `interval` option.
If a new sample for the existing time series is received after that, then it is treated as the first sample for a new time series.
This may lead to the following issues:

- Lower than expected results for [total_prometheus](./configuration/outputs/#total_prometheus) and [increase_prometheus](./configuration/outputs/#increase_prometheus) outputs,
  since they ignore the first sample in a new time series.
- Unexpected spikes for [total](./configuration/outputs/#total) and [increase](./configuration/outputs/#increase) outputs, since they assume that new time series start from 0.

These issues can be fixed in the following ways:

- By increasing the `interval` option at [stream aggregation config](./configuration/README.md), so it covers the expected
  delays in data ingestion pipelines.
- By specifying the `staleness_interval` option at [stream aggregation config](./configuration/README.md), so it covers the expected
  delays in data ingestion pipelines. By default, the `staleness_interval` equals to `2 x interval`.

### High resource usage

The following solutions can help reducing memory usage and CPU usage durting streaming aggregation:

- To use more specific `match` filters at [streaming aggregation config](./configuration/README.md), so only the really needed
  [raw samples](https://docs.victoriametrics.com/keyconcepts#raw-samples) are aggregated.
- To increase aggregation interval by specifying bigger duration for the `interval` option at [streaming aggregation config](./configuration/README.md).
- To generate lower number of output time series by using less specific [`by` list](#aggregating-by-labels) or more specific [`without` list](#aggregating-by-labels).
- To drop unneeded long labels in input samples via [input_relabel_configs](#relabeling).

### Cluster mode

If you use [vmagent in cluster mode](https://docs.victoriametrics.com/vmagent#scraping-big-number-of-targets) for streaming aggregation
then be careful when using [`by` or `without` options](#aggregating-by-labels) or when modifying sample labels
via [relabeling](#relabeling), since incorrect usage may result in duplicates and data collision.
   
For example, if more than one `vmagent` instance calculates [increase](./configuration/outputs/#increase) for `http_requests_total` metric
with `by: [path]` option, then all the `vmagent` instances will aggregate samples to the same set of time series with different `path` labels.
The proper fix would be [adding an unique label](https://docs.victoriametrics.com/vmagent#adding-labels-to-metrics) for all the output samples
produced by each `vmagent`, so they are aggregated into distinct sets of [time series](https://docs.victoriametrics.com/keyconcepts#time-series).
These time series then can be aggregated later as needed during querying.
   
If `vmagent` instances run in Docker or Kubernetes, then you can refer `POD_NAME` or `HOSTNAME` environment variables
as an unique label value per each `vmagent` via `-remoteWrite.label=vmagent=%{HOSTNAME}` command-line flag.
See [these docs](https://docs.victoriametrics.com/#environment-variables) on how to refer environment variables in VictoriaMetrics components.
