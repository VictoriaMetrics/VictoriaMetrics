# cestorage

`cestorage` is a cardinality estimator that receives Prometheus remote write streams 
and exposes approximate time series cardinality as metrics (TODO: support remote write).

It is useful for tracking how many unique time series are flowing through across all metrics, metric name, or broken down by a specific label.

## How it works

Running:
```
go run ./app/cestimator/... -config=./app/streams.yaml -httpListenAddr=:8490
```

Configuration:

```yaml
streams:
  # Track cardinality of all time series, grouped by metric name.
  - name: 'global_by_metric_name'
    interval: '1h'
    group: '__name__'

  # Track cardinality of all time series, grouped by instance label.
  - name: 'global_by_instance'
    interval: '5m'
    group: 'instance'

  # Track cardinality only for the eu-central-1 region, grouped by instance.
  - name: 'eu_region_by_instance'
    filter: '{region="eu-central-1"}'
    group: 'instance'

  # Track total cardinality with no grouping.
  - name: 'global_total'
    interval: '1h'
```

Cardinality generator:

```
go run ./app/cegen/main.go -cardI=100 -cardY=20 -template="foo{instance=\"127.0.0.[cardI]\",job=\"ametric[cardY]\"}"
```


## Metrics

Cardinality estimates are written to `/metrics` in Prometheus text format.

**Without grouping:**
```
cardinality_estimate{stream="global_total"} 142300
```

**With grouping** (one line per distinct label value):
```
cardinality_estimate{stream="global_by_metric_name",ce___name__="cpu_usage_seconds_total"} 4200
cardinality_estimate{stream="global_by_metric_name",ce___name__="http_requests_total"} 8750
```

The group label is prefixed with `ce_` to avoid collisions with built-in labels.