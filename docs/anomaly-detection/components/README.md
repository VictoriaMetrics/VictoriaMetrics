---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
This chapter describes different components, that correspond to respective sections of a config to launch VictoriaMetrics Anomaly Detection (or simply [`vmanomaly`](https://docs.victoriametrics.com/anomaly-detection/) service:

- [Model(s) section](https://docs.victoriametrics.com/anomaly-detection/components/models/) - Required
- [Reader section](https://docs.victoriametrics.com/anomaly-detection/components/reader/) - Required
- [Scheduler(s) section](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) - Required
- [Writer section](https://docs.victoriametrics.com/anomaly-detection/components/writer/) - Required
- [Monitoring section](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/) -  Optional
- [Settings section](https://docs.victoriametrics.com/anomaly-detection/components/settings/) - Optional

> Once the service starts, automated config validation is performed{{% available_from "v1.7.2" anomaly %}}. Please see container logs for errors that need to be fixed to create fully valid config, visiting sections above for examples and documentation.

> Components' class{{% available_from "v1.13.0" anomaly %}} can be referenced by a short alias instead of a full class path - i.e. `model.zscore.ZscoreModel` becomes `zscore`, `reader.vm.VmReader` becomes `vm`, `scheduler.periodic.PeriodicScheduler` becomes `periodic`, etc. Please see according sections for the details.

> `preset` modes are available{{% available_from "v1.13.0" anomaly %}} for `vmanomaly`. Please find the guide [here](https://docs.victoriametrics.com/anomaly-detection/presets/).

## Components interaction

Below, you will find an example illustrating how the components of `vmanomaly` interact with each other and with a single-node VictoriaMetrics setup.

> [Reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [Writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) also support [multitenancy](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy), so you can read/write from/to different locations - see `tenant_id` param description.

![vmanomaly-components](vmanomaly-components.webp)

## Example config

Here's a minimalistic full config example, demonstrating many-to-many configuration (actual for [latest version](https://docs.victoriametrics.com/anomaly-detection/changelog/)):

```yaml
settings:
  n_workers: 4  # number of workers to run models in parallel
  anomaly_score_outside_data_range: 5.0  # default anomaly score for anomalies outside expected data range
  restore_state: True  # restore state from previous run, if available

# how and when to run the models is defined by schedulers
# https://docs.victoriametrics.com/anomaly-detection/components/scheduler/
schedulers:
  periodic_1d:  # alias
    class: 'periodic' # scheduler class
    infer_every: "30s"
    fit_every: "1d"
    fit_window: "24h"
    start_from: "00:00"  # start from specified time, i.e. 00:00 given timezone and do daily fits as `fit_every` is 1 day
    tz: "Europe/Kyiv"  # timezone to use for start_from
  periodic_1w:
    class: 'periodic'
    infer_every: "15m"
    fit_every: "1h"
    fit_window: "7d"
    # if no start_from is specified, jobs will start immediately after service starts

# what model types and with what hyperparams to run on your data
# https://docs.victoriametrics.com/anomaly-detection/components/models/
models:
  zscore:  # we can set up alias for model
    class: 'zscore'  # model class
    z_threshold: 3.5
    provide_series: ['anomaly_score']  # what series to produce
    queries: ['host_network_receive_errors']  # what queries to run particular model on
    schedulers: ['periodic_1d']  # will be attached to 1-day schedule, fit every 10m and infer every 30s
    min_dev_from_expected: 0.0  # turned off. if |y - yhat| < min_dev_from_expected, anomaly score will be 0
    detection_direction: 'above_expected' # detect anomalies only when y > yhat, "peaks"
    clip_predictions: True  # clip predictions to expected data range, i.e. [0, inf] for this query `host_network_receive_errors
  prophet: # we can set up alias for model
    class: 'prophet'
    provide_series: ['anomaly_score', 'yhat', 'yhat_lower', 'yhat_upper']
    queries: ['cpu_seconds_total']
    schedulers: ['periodic_1w']  # will be attached to 1-week schedule, fit every 1h and infer every 15m
    min_dev_from_expected: [0.01, 0.01]  # minimum deviation from expected value to be even considered as anomaly
    anomaly_score_outside_data_range: 1.5  # override default anomaly score outside expected data range
    detection_direction: 'above_expected'
    clip_predictions: True  # clip predictions to expected data range, i.e. [0, inf] for this query `cpu_seconds_total`
    args:  # model-specific arguments
      interval_width: 0.98
      yearly_seasonality: False  # disable yearly seasonality, since we have only 7 days of data

# where to read data from
# https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader
reader:
  class: 'vm'
  datasource_url: "https://play.victoriametrics.com/"
  tenant_id: "0:0"
  sampling_period: "30s"  # what data resolution to fetch from VictoriaMetrics' /query_range endpoint
  latency_offset: '1ms'
  query_from_last_seen_timestamp: False
  tz: "UTC"  # timezone to use for queries without explicit timezone
  "offset": "0s"  # offset to apply to all queries, e.g. to account for data delays, can be overridden on per-query basis
  queries:  # aliases to MetricsQL expressions
    cpu_seconds_total:
      expr: 'avg(rate(node_cpu_seconds_total[5m])) by (mode)' 
      # step: '30s'  # if not set, will be equal to reader-level sampling_period
      data_range: [0, 'inf']  # expected value range, anomaly_score = anomaly_score_outside_data_range if y (real value) is outside
    host_network_receive_errors:
      expr: 'rate(node_network_receive_errs_total[3m]) / rate(node_network_receive_packets_total[3m])'
      step: '15m'  # here we override per-query `sampling_period` to request way less data from VM TSDB
      data_range: [0, 'inf']

# where to write data to
# https://docs.victoriametrics.com/anomaly-detection/components/writer/
writer:
  datasource_url: "http://victoriametrics:8428/"
  # tenant_id: "0:0"  # for VictoriaMetrics cluster, can support "multitenant"

# enable self-monitoring in pull and/or push mode
# https://docs.victoriametrics.com/anomaly-detection/components/monitoring/
monitoring:
  pull: # Enable /metrics endpoint.
    addr: "0.0.0.0"
    port: 8490

  push: # Enable pushing self-monitoring metrics
    url: "http://victoriametrics:8428"
    push_frequency: "15m"  # how often to push self-monitoring metrics
```

## Hot reload

> This feature is better used in conjunction with [stateful service](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) to preserve the state of the models and schedulers between restarts and reuse what can be reused, thus avoiding unnecessary re-training of models, re-initialization of schedulers and re-reading of data.

{{% available_from "v1.25.0" anomaly %}} Service supports hot reload of configuration files, which allows for automatic reloading of configurations on config files change filesystem events without the need of explicit service restart. This can be enabled via the `--watch` [CLI argument](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments). `vmanomaly_hot_reload_enabled` flag in [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#startup-metrics) will be set to 1 (if enabled) or 0 (if disabled).

### How it works

It works by watching for file system events, such as modifications, creations, or deletions of `.yml|.yaml` files in the specified directories. When a change is detected, the service will attempt to reload the configuration files, rebuild the [global config](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#global-config) and reinitialize the components. If the reload is successful, the `vmanomaly_hot_reload_events_total` metric will be incremented for `status="success"` label, otherwise it will be incremented with `status="failure"` label and a respective error message on config validation failure(s) will be logged.

> If the reload fails, the service will log an error message indicating the reason for the failure, and the **previous configuration will remain active until a successful reload occurs** to preserve the service's stability. This means that if there are errors in the new configuration, the service will continue to operate with the last valid configuration until the issues are resolved.

If used on [sharded setup](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability), upon [global config](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#global-config) change, all shards will be reinitialized with the new configurations.

> Please note, that even if [state restoration](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) is enabled, the models, queries and schedulers might "migrate" to new shards if the order or the amount of [sub-configs](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#sub-configuration) changes after new config is hot-reloaded, so the state restoration won't be **fully** efficient in this case.

### Example

For simplicity, let a service be run on a config file named `config.yaml` with the following content:

```yaml
settings:
  n_workers: 4  # number of workers to run models in parallel
  anomaly_score_outside_data_range: 5.0  # default anomaly score for anomalies outside expected data range
  restore_state: True  # restore state from previous run, if available

schedulers:
  periodic:
    class: 'periodic'
    infer_every: "30s"
    fit_every: "24h"
    fit_window: "24h"

reader:
  datasource_url: "https://play.victoriametrics.com/"
  tenant_id: "0:0"
  class: 'vm'
  sampling_period: "30s"
  queries:
    cpu_seconds_total:
      expr: 'avg(rate(node_cpu_seconds_total[5m])) by (mode)'
      data_range: [0, 'inf']
      # step: '30s'  # if not set, will be equal to reader-level sampling_period
    host_network_receive_errors:
      expr: 'rate(node_network_receive_errs_total[3m]) / rate(node_network_receive_packets_total[3m])'
      step: '15s'
      data_range: [0, 'inf']
  
models:
  zscore:
    class: 'zscore'
    z_threshold: 3.5
    provide_series: ['anomaly_score']
    # if queries are not specified, all queries from reader will be used
    # if schedulers are not specified, all schedulers will be used

writer:
  datasource_url: "http://victoriametrics:8428/"

monitoring:
  push:
    url: "http://victoriametrics:8428"
    push_frequency: "15m"
```

Suppose after 15m since service startup, there was a change to the query expression and frequency for `node_cpu_seconds_total` query in `reader.queries`:

```yaml
# ... (rest of the config remains unchanged)
reader:
  # ... (rest of the reader config remains unchanged)
  queries:
    cpu_seconds_total:
      expr: 'avg(rate(node_cpu_seconds_total[10m])) by (mode)'  # changed lookback period
      data_range: [0, 'inf']
      step: '60s'  # changed step
# ... (rest of the config remains unchanged)
```

After saving the changes, hot reload will automatically detect the changes in `config.yaml` and attempt to reload the configuration. As the changes are valid, the service will log a success message and increment the `vmanomaly_hot_reload_events_total` metric with `status="success"` label:

- All the model instances of class `zscore`, that were trained on `host_network_receive_errors` can be reused as they are still valid and "fresh" for making inference on new datapoints until the next `fit_every` happens (10m - 5m).
- All the model instances of class `zscore`, that were trained on `cpu_seconds_total` will be re-trained with the new query expression and frequency, as old model instances are not valid anymore.


## Environment variables

{{% available_from "v1.25.0" anomaly %}} Environment variables can be referenced directly in the configuration files using scalar string placeholders `%{ENV_NAME}`. This allows for dynamic configuration based on environment variables, which is particularly useful for managing sensitive information like API keys or database credentials while still making it accessible to the service.

For example, `VMANOMALY_URL` environment variable can be set to `http://localhost:8428` and then used in the configuration file in reader [section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) as `datasource_url: %{VMANOMALY_URL}`.

> If referenced environment variable is not set or there is a typo, **the placeholder will not be replaced** which may lead to errors during configuration validation or endpoints probing failure. It is recommended to ensure that all required environment variables are set before starting the service.

### Example

```yaml
reader:
  class: 'vm'
  datasource_url: %{VMANOMALY_URL}  # will be replaced with the value of VMANOMALY_URL environment variable
  tenant_id: %{VMANOMALY_TENANT_ID}  # will be replaced with the value of VMANOMALY_TENANT_ID environment variable
  bearer_token: %{VMANOMALY_BEARER_TOKEN}  # will be replaced with the value of VMANOMALY_BEARER_TOKEN environment variable
  sampling_period: "30s"

writer:
  datasource_url: %{VMANOMALY_URL}  # will be replaced with the value of VMANOMALY_URL environment variable
  tenant_id: %{VMANOMALY_TENANT_ID}  # will be replaced with the value of VMANOMALY_TENANT_ID environment variable
  bearer_token: %{VMANOMALY_BEARER_TOKEN}  # will be replaced with the value of VMANOMALY_BEARER_TOKEN environment variable

# other config sections ...
```
