---
sort: 5
weight: 5
title: Common mistakes
menu:
  docs:
    identifier: stream-aggregation-common-mistakes
    parent: 'stream-aggregation'
    weight: 5
aliases:
- /stream-aggregation/common-mistakes/
- /stream-aggregation/common-mistakes/index.html
---

## Place aggregation agents behind a load balancer

Partial aggregation (only subset of all data, which satisfies [`match`](./configuration/#match) expression was pushed to an aggreagation agent) is not acceptable.
It produces wrong aggregations which are not usable and not comparable to equivalent [recording rules](https://docs.victoriametrics.com/vmalert#recording-rules).

To keep aggregation results consistent
it should be either fully processed on a single VMAgent or data can be sharded accross multiple VMAgents by metric name

## Create separate aggregator for each recording rule

As is was mentioned in [use case scenarios](./use-cases.md#recording-rules-alternative), stream aggregation can be considered as a substitution for
[recording rules](https://docs.victoriametrics.com/vmalert#recording-rules), but straightforward conversion of recording rules to [stream aggregation config](./configuration/#configuration-file-reference)
can lead to inefficient resource usage on component it's configured on ([VMAgent](https://docs.victoriametrics.com/vmagent) or [VMSingle](https://docs.victoriametrics.com/vmsingle)).

To optimize this we recommend to merge together aggregations which only differ in match expressions. E.g:

Given list of recording rules:

```yaml
- expr: sum(rate(node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}[3m])) BY (instance)
  record: instance:node_cpu:rate:sum
- expr: sum(rate(node_network_receive_bytes_total[3m])) BY (instance)
  record: instance:node_network_receive_bytes:rate:sum
- expr: sum(rate(node_network_transmit_bytes_total[3m])) BY (instance)
  record: instance:node_network_transmit_bytes:rate:sum
- expr: sum(rate(node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}[5m]))
  record: cluster:node_cpu:sum_rate5m
```

can be converted to aggregation rules:

```yaml
- match: node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}
  interval: 3m
  by:
  - instance
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    replacement: instance:node_cpu:rate:sum
- match: node_network_receive_bytes_total
  interval: 3m
  by:
  - instance
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    replacement: instance:node_network_receive_bytes:rate:sum
- match: node_network_transmit_bytes_total
  interval: 3m
  by:
  - instance
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    replacement: instance:node_network_transmit_bytes:rate:sum
- match: node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}
  interval: 5m
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    replacement: cluster:node_cpu:sum_rate5m
```

note, that first 3 aggregation rules differ only in [`match`](./configuration/#match), so they can be merged together:

```yaml
- match:
  - node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}
  - node_network_receive_bytes_total
  - node_network_transmit_bytes_total
  interval: 3m
  by:
  - instance
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    regex: regex: "(.+)(_seconds)?(_total)?:.+"
    replacement: cluster:node_cpu:sum_rate5m
- match: node_cpu_seconds_total{mode!="idle",mode!="iowait",mode!="steal"}
  interval: 5m
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    replacement: cluster:node_cpu:sum_rate5m
```

**Note**: having separate aggregator for a certain [`match`](./configuration/#match) expression can only be justified when aggregator cannot keep up with all
the data pushed to an aggregator within an aggregation interval

## Use identical --remoteWrite.streamAggr.config for all remote writes

As it's described in [previous](#create-separate-aggregator-for-each-recording-rule) case having many aggregators leads to increased resource usage so having `n`
identical aggregation configurations `-remoteWrite.streamAggr.config` for multiple `-remoteWrite.url` requires `n * x` resources.

As an optimization we suggest using `-streamAggr.config` as a replacement for `-remoteWrite.streamAggr.config`.
It places global aggregator in front of all remote writes, which helps to reduce resource usage.

## Treat aggregated metrics in the same manner as original ones

Stream aggregation allows to keep for aggregation result the name of a source metric using [`keep_metric_names:`](./configuration/#keep-metric-names) `true`.
But graphs and alerts, which were previously used for a raw metric can become incorrect for aggregated one.

Dashboards and alerts should be updated according to aggregation configurations.
