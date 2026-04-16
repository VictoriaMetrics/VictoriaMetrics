# cestorage

`cestorage` is a cardinality estimator that receives Prometheus remote write streams
and exposes approximate time series cardinality as metrics (TODO: support remote write).

It is useful for tracking how many unique time series are flowing through across all metrics, metric name, or broken down by specific labels.

## How it works

Running:
```
go run ./app/cestorage/... -config=./app/cestorage/streams.yaml -httpListenAddr=:8490
```

Configuration:

```yaml
streams:
  # Track total cardinality with no grouping.
  - interval: '1h'

  # Track cardinality grouped by metric name.
  - interval: '1h'
    group: ["__name__"]

  # Track cardinality grouped by job label.
  - interval: '1m'
    group: ["job"]

  # Track cardinality grouped by tenant info
  - group: ["vm_account_id", "vm_project_id"]

  # Track cardinality of tens jobs, with extra labels on the output metrics.
  - filter: '{job=~"1\d+"}'
    group: ["job"]
    labels:
      region: 'eu-central-1'
      env: 'production'
```

Fields:
- `filter` (optional): MetricsQL selector to restrict which time series are counted
- `group` (optional): list of label names to split cardinality by; each distinct combination gets its own estimate
- `labels` (optional): extra labels attached to all output metrics for this estimator
- `interval` (optional): how often to rotate (reset) counters; defaults to `5m`

Cardinality generator:

```
go run ./app/cegen/main.go -cardI=100 -cardY=20 -template="foo{instance=\"127.0.0.[cardI]\",job=\"ametric[cardY]\"}"
```


## Metrics

Cardinality estimates are written to `/metrics` in Prometheus text format.

All metrics include `interval`, `filter`, and `group_by_keys` labels. Extra labels from the `labels` config field are appended next (sorted alphabetically). When grouping is configured, a `group_by_values` label holds the comma-separated values for each distinct combination.

**Without grouping:**
```
cardinality_estimate{interval="1h0m0s",filter="",group_by_keys=""} 142300
```

**With filter and single group:**
```
cardinality_estimate{interval="5m0s",filter="{region=\"eu-central-1\"}",group_by_keys="instance",group_by_values="host1:9090"} 312
```

**With multiple group labels** (one line per distinct label value combination):
```
cardinality_estimate{interval="5m0s",filter="",group_by_keys="instance,job",group_by_values="host1:9090,prometheus"} 312
cardinality_estimate{interval="5m0s",filter="",group_by_keys="instance,job",group_by_values="host2:9100,node"} 87
```

**With extra labels:**
```
cardinality_estimate{interval="5m0s",filter="{region=\"eu-central-1\"}",group_by_keys="instance",env="production",region="eu-central-1",group_by_values="host1:9090"} 312
```
