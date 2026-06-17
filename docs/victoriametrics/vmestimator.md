---
weight: 3
menu:
  docs:
    parent: victoriametrics
    weight: 13
title: vmestimator
tags:
  - metrics
  - cardinality
aliases:
  - /vmestimator.html
  - /vmestimator/index.html
  - /vmestimator/
---

`cestimator` is a cardinality estimator that receives Prometheus remote write streams and exposes approximate time series cardinality as metrics.

It is useful for tracking how many unique time series are flowing through across all metrics, metric name, or broken down by specific labels.

## How it works

Running:
```
go run ./app/cestimator/... -config=streams.yaml -httpListenAddr=:8490
```

Configuration:

```yaml
streams:
  # Track total cardinality with no grouping.
  - interval: '1h'

  # Track cardinality grouped by metric name.
  - interval: '1h'
    group_by: ["__name__"]

  # Track cardinality grouped by job label.
  - interval: '1m'
    group_by: ["job"]

  # Track cardinality grouped by tenant info
  - group_by: ["vm_account_id", "vm_project_id"]

  # Track cardinality of jobs, with extra labels on the output metrics.
  - group_by: ["job"]
    labels:
      region: 'eu-central-1'
      env: 'production'
```

Fields:
- `group_by` (optional): list of label names to split cardinality by; each distinct combination gets its own estimate
- `group_limit` (optional): maximum number of distinct groups to track; excess groups are counted in a rejected sketch but not individually; defaults to `10000`
- `buckets` (optional): number of internal shards for parallel ingestion; defaults to `min(20, availableCPUs)`
- `labels` (optional): extra labels attached to all output metrics for this estimator
- `interval` (optional): how often to rotate (reset) counters; defaults to `5m`
- `hll_precision` (optional): HyperLogLog precision, must be in range `[4, 18]`; higher values yield more accurate estimates at the cost of more memory; defaults to `14`
- `hll_sparse` (optional): whether to use sparse HyperLogLog representation, which reduces memory for low-cardinality groups; defaults to `true`

Cardinality generator:

```
go run ./app/cegen/main.go -cardI=100 -cardY=20 -template="foo{instance=\"127.0.0.[cardI]\",job=\"ametric[cardY]\"}"
```

## Metrics

By default, cardinality estimates are merged with regular metrics and exposed at `/metrics`.

This behavior is controlled by the `-cardinalityMetrics.exposeAt` flag:
- `-cardinalityMetrics.exposeAt=/metrics` (default): cardinality metrics merged with regular metrics at `/metrics`
- `-cardinalityMetrics.exposeAt=/cardinality/metrics`: only cardinality metrics exposed at that path
- `-cardinalityMetrics.exposeAt=`: cardinality metrics not exposed via HTTP

All metrics include `interval`, `group_by_keys`, and `group_by_values` labels. Extra labels from the `labels` config field are inserted between `interval` and `group_by_keys` (sorted alphabetically).

**Without grouping** (`group_by_keys` is `__global__` and `group_by_values` is not set):
```
cardinality_estimate{interval="1h0m0s",group_by_keys="__global__"} 142300
```

**With grouping** — one summary line (total distinct group count) plus one line per distinct label value combination. Each per-group line also includes individual `by_{key}="{val}"` labels for each group key:
```
cardinality_estimate{interval="5m0s",group_by_keys="__group__",group_by_values="instance,job"} 2
cardinality_estimate{interval="5m0s",group_by_keys="instance,job",group_by_values="host1:9090,prometheus",by_instance="host1:9090",by_job="prometheus"} 312
cardinality_estimate{interval="5m0s",group_by_keys="instance,job",group_by_values="host2:9100,node",by_instance="host2:9100",by_job="node"} 87
```

**With extra labels:**
```
cardinality_estimate{interval="5m0s",env="production",region="eu-central-1",group_by_keys="job",group_by_values="prometheus",by_job="prometheus"} 312
```

## Operational metrics

When grouping is enabled, cestimator exposes per-bucket operational metrics at `/metrics`:

- `cestimator_group_estimator_size{groupBy, bucket}` — number of active groups in this bucket after the last rotation
- `cestimator_group_estimator_rejected_size{groupBy, bucket}` — estimated number of distinct group values rejected since the last rotation because `group_limit` was reached
- `cestimator_group_limit{groupBy, bucket}` — configured `group_limit` for this bucket

