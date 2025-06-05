---
title: Settings
weight: 6
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 6
    identifier: "vmanomaly-settings"
tags:
  - metrics
  - enterprise
aliases:
  - ./settings.html
---

Through the **Settings** section of a config, you can configure the following parameters of the anomaly detection service:

## Anomaly Score Outside Data Range

This argument allows you to override the anomaly score for anomalies that are caused by values outside the expected **data range** of particular [query](https://docs.victoriametrics.com/anomaly-detection/components/models#queries). The reasons for such anomalies can be various, such as improperly constructed metricsQL queries, sensor malfunctions, or other issues that lead to unexpected values in the data and reqire investigation.

> If not set, the [anomaly score](https://docs.victoriametrics.com/anomaly-detection/faq#what-is-anomaly-score) for such anomalies defaults to `1.01` for backward compatibility, however, it is recommended to set it to a higher value, such as `5.0`, to better reflect the severity of anomalies that fall outside the expected data range to catch them faster and check the query for correctness and underlying data for potential issues.

Here's an example configuration that sets default anomaly score outside expected data range to `5.0` and overrides it for a specific model to `1.5`:

```yaml
settings:
  n_workers: 4
  anomaly_score_outside_data_range: 5.0

schedulers:
  periodic:
    class: periodic
    fit_every: 5m
    fit_window: 3h
    infer_every: 30s
  # other schedulers

models:
  zscore_online_override:
    class: zscore_online
    z_threshold: 3.5
    clip_predictions: True
    # will be inherited from settings.anomaly_score_outside_data_range
    # anomaly_score_outside_data_range: 5.0
  zscore_online_override:
    class: zscore_online
    z_threshold: 3.5
    clip_predictions: True
    anomaly_score_outside_data_range: 1.5  # will override settings.anomaly_score_outside_data_range
  # other models

reader:
  class: vm
  datasource_url: 'https://play.victoriametrics.com'
  tenant_id: "0"
  queries:
    error_rate:
      expr: 'rand()*100 + rand()'  # example query that generates values between 1 and 100 and sometimes exceeds 100
      data_range: [0., 100.]  # expected data range for the underlying query and business logic
    # other queries
  sampling_period: 30s
  latency_offset: 10ms
  query_from_last_seen_timestamp: False
  verify_tls: False
  # other reader settings
  
writer:
  class: "vm"
  datasource_url: http://localhost:8428
  metric_format:
    __name__: "$VAR"
    for: "$QUERY_KEY"
  # other writer settings

monitoring:
  push:
    url: http://localhost:8428
    push_frequency: 1m
  # other monitoring settings
```

## Parallelization

The `n_workers` argument allows you to explicitly specify the number of workers for internal parallelization of the service. This can help improve performance on multicore systems by allowing the service to process multiple tasks in parallel. For backward compatibility, it's set to `1` by default, meaning that the service will run in a single-threaded mode. It should be an integer greater than or equal to `-1`, where `-1` and `0` means that the service will automatically inherit the number of workers based on the number of available CPU cores.

Increasing the number can be particularly useful when dealing with a high volume of queries returning many (long) timeseries.
Decreasing the number can be useful when running the service on a system with limited resources or when you want to reduce the load on the system.

Here's an example configuration that uses 4 workers for service's internal parallelization:

```yaml
settings:
  n_workers: 4

schedulers:
  periodic:
    class: periodic
    fit_every: 5m
    fit_window: 3h
    infer_every: 30s
  # other schedulers

models:
  zscore_online_override:
    class: zscore_online
    z_threshold: 3.5
    clip_predictions: True
  # other models

reader:
  class: vm
  datasource_url: 'https://play.victoriametrics.com'
  tenant_id: "0"
  queries:
    example_query:
      expr: 'rand() + 1'  # example query that generates random values between 1 and 2
      data_range: [1., 2.]
    # other queries
  sampling_period: 30s
  latency_offset: 10ms
  query_from_last_seen_timestamp: False
  verify_tls: False
  # other reader settings
  
writer:
  class: "vm"
  datasource_url: http://localhost:8428
  metric_format:
    __name__: "$VAR"
    for: "$QUERY_KEY"
  # other writer settings

monitoring:
  push:
    url: http://localhost:8428
    push_frequency: 1m
  # other monitoring settings
```
