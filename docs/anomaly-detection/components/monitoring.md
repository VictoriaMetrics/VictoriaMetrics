---
title: Monitoring
weight: 5
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 5
    identifier: "vmanomaly-monitoring"
tags:
  - metrics
  - enterprise
aliases:
  - ./monitoring.html
---
There are 2 models to monitor VictoriaMetrics Anomaly Detection behavior - [push](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model) and [pull](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#pull-model). Parameters for each of them should be specified in the config file, `monitoring` section.

> There was an enhancement of [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) metrics for consistency across the components ([v.1.17.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1170)). Documentation was updated accordingly. Key changes included:
- Converting several [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) metrics from `Summary` to `Histogram` to enable quantile calculation. This addresses the limitation of the `prometheus_client`'s [Summary](https://prometheus.github.io/client_python/instrumenting/summary/) implementation, which does not support quantiles. The change ensures metrics are more informative for performance analysis. Affected metrics are:
    - `vmanomaly_reader_request_duration_seconds` ([VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics))
    - `vmanomaly_reader_response_parsing_seconds` ([VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics))
    - `vmanomaly_writer_request_duration_seconds` ([VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics))
    - `vmanomaly_writer_request_serialize_seconds` ([VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics))
- Adding a `query_key` label to the `vmanomaly_reader_response_parsing_seconds` [metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) to provide finer granularity in tracking the performance of individual queries. This metric has also been switched from `Summary` to `Histogram` to align with the other metrics and support quantile calculations.
- Adding `preset` and `scheduler_alias` keys to [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) and [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics) metrics for consistency in multi-[scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) setups.
- Renaming [Counters](https://prometheus.io/docs/concepts/metric_types/#counter) `vmanomaly_reader_response_count` to `vmanomaly_reader_responses` and `vmanomaly_writer_response_count` to `vmanomaly_writer_responses`.

## Pull Model Config parameters

<table class="params">
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Default</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`addr`
            </td>
            <td>

`"0.0.0.0"`
            </td>
            <td>Server IP Address</td>
        </tr>
        <tr>
            <td>

`port`
            </td>
            <td>

`8080`
            </td>
            <td>Port</td>
        </tr>
    </tbody>
</table>

## Push Config parameters

By default, metrics are pushed only after the completion of specific stages, e.g., `fit`, `infer`, or `fit_infer` (for each [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) if using a multi-scheduler configuration).

The `push_frequency` parameter{{% available_from "v1.18.7" anomaly %}} (default value: `15m`) can be configured to initiate *additional* periodic metric pushes at consistent intervals. This enhances the self-monitoring capabilities of `vmanomaly` by aligning more closely with pull-based monitoring behavior, especially in setups with infrequent schedules (e.g., long `fit_every` or `infer_every` intervals in [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler)), mitigating data staleness. To disable scheduled metric pushes, set the `push_frequency` parameter to an empty string in the configuration file, as demonstrated in the examples below.

<table class="params">
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Default</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`url`</span>
            </td>
            <td></td>
            <td>

Link where to push metrics to. Example: `"http://localhost:8480/"`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tenant_id`</span>
            </td>
            <td></td>
            <td>

Tenant ID for cluster version. Example: `"0:0"`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`health_path`</span>
            </td>
            <td>

`"health"`
            </td>
            <td>

{{% deprecated_from "v1.8.0" anomaly %}}. Absolute, to override `/health` path
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`user`</span>
            </td>
            <td></td>
            <td>BasicAuth username</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`password`</span>
            </td>
            <td></td>
            <td>BasicAuth password</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`bearer_token`</span>
            </td>
            <td>

`token`
            </td>
            <td>
Token is passed in the standard format with header: `Authorization: bearer {token}`{{% available_from "v1.15.9" anomaly %}}. 
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`bearer_token_file`</span>
            </td>
            <td>

`path_to_file`
            </td>
            <td>
Path to a file, which contains token, that is passed in the standard format with header: `Authorization: bearer {token}`{{% available_from "v1.15.9" anomaly %}}.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`verify_tls`</span>
            </td>
            <td>

`false`
            </td>
            <td>
Verify TLS certificate. If `False`, it will not verify the TLS certificate. 
If `True`, it will verify the certificate using the system's CA store. 
If a path to a CA bundle file (like `ca.crt`), it will verify the certificate using the provided CA bundle.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tls_cert_file`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`path/to/cert.crt`</span>
            </td>
            <td>
Path to a file with the client certificate, i.e. `client.crt`{{% available_from "v1.16.3" anomaly %}}. 
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tls_key_file`</span>
            </td>
            <td>

`path/to/key.crt`
            </td>
            <td>
Path to a file with the client certificate key, i.e. `client.key`{{% available_from "v1.16.3" anomaly %}}.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`timeout`</span>
            </td>
            <td>

`"5s"`
            </td>
            <td>Stop waiting for a response after a given number of seconds.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`push_frequency`</span>
            </td>
            <td>

`"15m"`
            </td>
            <td>Frequency for scheduled pushing of metrics, e.g., '30m'. Suggested to be less than the staleness interval `-search.maxStalenessInterval` Set to empty string to disable *scheduled* pushing{{% available_from "v1.18.7" anomaly %}}.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`extra_labels`</span>
            </td>
            <td></td>
            <td>Section for custom labels specified by user.</td>
        </tr>
    </tbody>
</table>

## Monitoring section config example

``` yaml
monitoring:
  pull: # Enable /metrics endpoint.
    addr: "0.0.0.0"
    port: 8080
  push:
    url: "http://localhost:8480/"
    tenant_id: "0:0" # For cluster version only
    user: "USERNAME"
    password: "PASSWORD"
    verify_tls: False
    timeout: "5s"
    push_frequency: "15m"  # set to "" to disable scheduled pushes and leave only fit/infer based
    extra_labels:
      job: "vmanomaly-push"
      test: "test-1"
```

## mTLS protection

`vmanomaly` components such as [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) support [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication){{% available_from "v1.16.3" anomaly %}} to ensure secure communication with [VictoriaMetrics Enterprise, configured with mTLS](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#mtls-protection).

For detailed guidance on configuring mTLS parameters such as `verify_tls`, `tls_cert_file`, and `tls_key_file`, please refer to the [mTLS protection section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#mtls-protection) in the [Reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) documentation. The configuration principles apply consistently across all these `vmanomaly` components.

## Metrics generated by vmanomaly

- [Startup metrics](#startup-metrics)
- [Reader metrics](#reader-behaviour-metrics)
- [Model metrics](#models-behaviour-metrics)
- [Writer metrics](#writer-behaviour-metrics)

### Startup metrics

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th><span style="white-space: nowrap;">Type</span></th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_start_time_seconds`</span>
            </td>
            <td>

<span style="white-space: nowrap;">Gauge</span>
        </td>
            <td>vmanomaly start time in UNIX time</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_version_info`</span>
            </td>
            <td>Gauge</td>
            <td>vmanomaly version information, contained in `version` label{{% available_from "v1.17.2" anomaly %}}.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_ui_version_info`</span>
            </td>
            <td>Gauge</td>
            <td>vmanomaly UI version information, contained in `version` label{{% available_from "v1.17.2" anomaly %}}.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_available_memory_bytes`</span>
            </td>
            <td>Gauge</td>
            <td>Virtual memory size in bytes, available to the process{{% available_from "v1.18.4" anomaly %}}.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_cpu_cores_available`</span>
            </td>
            <td>Gauge</td>
            <td>Number of (logical) CPU cores available to the process{{% available_from "v1.18.4" anomaly %}}.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_config_entities`</span>
            </td>
            <td>Gauge</td>
            <td>Number of [sub-configs](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#sub-configuration) **available** (`{scope="total"}`) and **used** for particular [shard](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability) (`{scope="shard"}`) {{% available_from "v1.21.0" anomaly %}}</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_config_reload_enabled`</span>
(was `vmanomaly_hot_reload_enabled` {{% deprecated_from "v1.25.1" anomaly %}})
            </td>
            <td>Gauge</td>
            <td>Whether particular vmanomaly instance is run in [config hot-reload mode](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) {{% available_from "v1.25.0" anomaly %}}</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vmanomaly_config_reloads_total`</span> (was `vmanomaly_hot_reload_events_total`{{% deprecated_from "v1.25.1" anomaly %}})
            </td>
            <td>Counter</td>
            <td>How many config [hot-reloads](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) were made since service started {{% available_from "v1.25.0" anomaly %}}</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vmanomaly_config_last_reload_successful`</span>
            </td>
            <td>Gauge</td>
            <td>Whether last config [hot-reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) was successful (1) or not (0) {{% available_from "v1.25.1" anomaly %}}</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vmanomaly_config_last_reload_success_timestamp_seconds`</span>
            </td>
            <td>Gauge</td>
            <td>Timestamp of the last successful config [hot-reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) in seconds since epoch {{% available_from "v1.25.1" anomaly %}}</td>
        </tr>
    </tbody>
</table>

[Back to metric sections](#metrics-generated-by-vmanomaly)

### Reader behaviour metrics
Label names [description](#labelnames)

> To improve consistency across the components additional labels (`scheduler_alias`, `preset`) were added to writer and reader metrics{{% available_from "v1.17.0" anomaly %}}. Also, metrics `vmanomaly_reader_request_duration_seconds` and `vmanomaly_reader_response_parsing_seconds` changed their type to `Histogram` (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}}).

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th><span style="white-space: nowrap;">Description</span></th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_request_duration_seconds`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`Histogram`</span> (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}})</td>
            <td>The total time (in seconds) taken by queries to VictoriaMetrics `url` for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_responses`</span> (named `vmanomaly_reader_response_count`{{% deprecated_from "v1.17.0" anomaly %}})
            </td>
            <td>

`Counter`
            </td>
            <td>The count of responses received from VictoriaMetrics `url` for the `query_key` query, categorized by `code`, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `code`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_received_bytes`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of bytes received in responses for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, <span style="white-space: nowrap;">`scheduler_alias`</span>, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_response_parsing_seconds`</span>
            </td>
            <td>

`Histogram` (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}})
            </td>
            <td>The total time (in seconds) taken for data parsing at each `step` (json, dataframe) for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`step`, `url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_timeseries_received`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of timeseries received from VictoriaMetrics for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_datapoints_received`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of datapoints received from VictoriaMetrics for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
    </tbody>
</table>

[Back to metric sections](#metrics-generated-by-vmanomaly)

### Models behaviour metrics
Label names [description](#labelnames)

> There is a new label key `model_alias` introduced in multi-model support{{% available_from "v1.10.0" anomaly %}}. This label key adjustment was made to preserve unique label set production during writing produced metrics back to VictoriaMetrics.

> As a part of [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) metrics enhancement{{% available_from "v1.17.0" anomaly %}}, new metrics, like `vmanomaly_model_run_errors`, was added. Some of them changed the type (`Summary` -> `Histogram`), like `vmanomaly_model_run_duration_seconds`.

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th><span style="white-space: nowrap;">Description</span></th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_runs`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>How many successful `stage` (`fit`, `infer`, `fit_infer`) runs occurred for models of class `model_alias` based on results from the `query_key` query, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_run_duration_seconds`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`Histogram`</span> (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}}) </td>
            <td>The total time (in seconds) taken by model invocations during the `stage` (`fit`, `infer`, `fit_infer`), based on the results of the `query_key` query, for models of class `model_alias`, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_datapoints_accepted`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The number of datapoints accepted (excluding NaN or Inf values) by models of class `model_alias` from the results of the `query_key` query during the `stage` (`infer`, `fit_infer`), within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_datapoints_produced`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The number of datapoints generated by models of class `model_alias` during the `stage` (`infer`, `fit_infer`) based on results from the `query_key` query, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_models_active`</span>
            </td>
            <td>

`Gauge`
            </td>
            <td>The number of model instances of class `model_alias` currently available for inference for the `query_key` query, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_runs_skipped`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The number of times model runs (of class `model_alias`) were skipped in expected situations (e.g., no data for fitting/inference, or no new data to infer on) during the `stage` (`fit`, `infer`, `fit_infer`), based on results from the `query_key` query, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_model_run_errors`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The number of times model runs (of class `model_alias`) failed due to internal service errors during the `stage` (`fit`, `infer`, `fit_infer`), based on results from the `query_key` query, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`stage`, `query_key`, `model_alias`, <span style="white-space: nowrap;">`scheduler_alias`</span>, `preset`
            </td>
        </tr>
    </tbody>
</table>

[Back to metric sections](#metrics-generated-by-vmanomaly)

### Writer behaviour metrics
Label names [description](#labelnames)

> Additional labels (`scheduler_alias`, `preset`){{% available_from "v1.17.0" anomaly %}} were added to writer and reader metrics to improve consistency across the components. Also, metrics `vmanomaly_writer_request_duration_seconds` and `vmanomaly_writer_request_serialize_seconds` changed their type to `Histogram` (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}}).

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th><span style="white-space: nowrap;">Description</span></th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_request_duration_seconds`</span>
            </td>
            <td>

`Histogram` (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}})
            </td>
            <td>The total time (in seconds) taken by write requests to VictoriaMetrics `url` for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.
</td>
            <td>

`url`, `query_key`, <span style="white-space: nowrap;">`scheduler_alias`</span>, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_responses`</span> (named `vmanomaly_reader_response_count`{{% deprecated_from "v1.17.0" anomaly %}})
            </td>
            <td>

`Counter`
            </td>
            <td>The count of response codes received from VictoriaMetrics `url` for the `query_key` query, categorized by `code`, within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.
</td>
            <td>

`url`, `code`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_sent_bytes`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of bytes sent to VictoriaMetrics `url` for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_request_serialize_seconds`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`Histogram`</span> (was `Summary`{{% deprecated_from "v1.17.0" anomaly %}})</td>
            <td>The total time (in seconds) taken for serializing data for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_datapoints_sent`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of datapoints sent to VictoriaMetrics for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_writer_timeseries_sent`</span>
            </td>
            <td>

`Counter`
            </td>
            <td>The total number of timeseries sent to VictoriaMetrics for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
            <td>

`url`, `query_key`, `scheduler_alias`, `preset`
            </td>
        </tr>
    </tbody>
</table>

[Back to metric sections](#metrics-generated-by-vmanomaly)

### Labelnames

* `stage` - stage of model - 'fit', 'infer' or 'fit_infer' for models that do it simultaneously, see [model types](https://docs.victoriametrics.com/anomaly-detection/components/models/#model-types).
* `query_key` - query alias from [`reader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/) config section.
* `model_alias` - model alias from [`models`](https://docs.victoriametrics.com/anomaly-detection/components/models/) config section{{% available_from "v1.10.0" anomaly %}}.
* `scheduler_alias` - scheduler alias from [`schedulers`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) config section{{% available_from "v1.11.0" anomaly %}}.
* `preset` - preset alias for [`preset`](https://docs.victoriametrics.com/anomaly-detection/presets/) mode of `vmanomaly`{{% available_from "v1.12.0" anomaly %}}.
* `url` - writer or reader url endpoint.
* `code` - response status code or `connection_error`, `timeout`.
* `step` - json or dataframe reading step.

[Back to metric sections](#metrics-generated-by-vmanomaly)


## Logs generated by vmanomaly

The `vmanomaly` service logs operations, errors, and performance for its components (service, reader, writer), alongside [self-monitoring metrics](#metrics-generated-by-vmanomaly) updates. Below is a description of key logs {{% available_from "v1.17.1" anomaly %}} for each component and the related metrics affected.

`{{X}}` indicates a placeholder in the log message templates described below, which will be replaced with the appropriate entity during logging.


> By default, `vmanomaly` uses the `INFO` logging level. You can change this by specifying the `--loggerLevel` argument. See command-line arguments [here](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments).

- [Startup logs](#startup-logs)
- [Reader  logs](#reader-logs)
- [Service logs](#service-logs)
- [Writer  logs](#writer-logs)


### Startup logs

The `vmanomaly` service logs important information during the startup process. This includes checking for the license, validating configurations, and setting up schedulers, readers, and writers. Below are key logs that are generated during startup, which can help troubleshoot issues with the service's initial configuration or license validation.

---

**License check**. If no license key or file is provided, the service will fail to start and log an error message. If a license file is provided but cannot be read, the service logs a failure. Log messages:

```text
Please provide a license code using --license or --licenseFile arg, or as VM_LICENSE_FILE env. See https://victoriametrics.com/products/enterprise/trial/ to obtain a trial license.
```

```text
failed to read file {{args.license_file}}: {{error_message}}
```

---

**Config validation**. If the service's configuration fails to load or does not meet validation requirements, an error message is logged and the service will exit. If the configuration is loaded successfully, a message confirming the successful load is logged. Log messages:

```text
Config validation failed, please fix these errors: {{error_details}}
```

```text
Config has been loaded successfully.
```

---

**Model and data directory setup**. The service checks the environment variables `VMANOMALY_MODEL_DUMPS_DIR` and `VMANOMALY_DATA_DUMPS_DIR` to determine where to store models and data. If these variables are not set, models and data will be stored in memory. Please find the [on-disk mode details here](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode). Log messages:

```text
Using ENV MODEL_DUMP_DIR=`{{model_dump_dir}}` to store anomaly detection models.
```
```text
ENV MODEL_DUMP_DIR is not set. Models will be kept in RAM between consecutive `fit` calls.
```
```text
Using ENV DATA_DUMP_DIR=`{{data_dump_dir}}` to store anomaly detection data.
```
```text
ENV DATA_DUMP_DIR is not set. Models' training data will be stored in RAM.
```

---

**Scheduler and service initialization**. After configuration is successfully loaded, the service initializes [schedulers](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) and services for each defined `scheduler_alias`. If there are issues with a specific scheduler (e.g., no models or queries found to attach to a scheduler), a warning is logged. When schedulers are initialized, the service logs a list of active schedulers. Log messages:

```text
Scheduler {{scheduler_alias}} wrapped and initialized with {{N}} model spec(s).
```
```text
No model spec(s) found for scheduler `{{scheduler_alias}}`, skipping setting it up.
```
```text
Active schedulers: {{list_of_schedulers}}.
```

[Back to logging sections](#logs-generated-by-vmanomaly)

---

### Reader logs

The `reader` component logs events during the process of querying VictoriaMetrics and retrieving the data necessary for anomaly detection. This includes making HTTP requests, handling SSL, parsing responses, and processing data into formats like DataFrames. The logs help to troubleshoot issues such as connection problems, timeout errors, or misconfigured queries.

---

**Starting a healthcheck request**. When the `reader` component initializes, it checks whether the VictoriaMetrics endpoint is accessible by sending a request for `_vmanomaly_healthcheck`. Log messages:

```text
[Scheduler {{scheduler_alias}}] Max points per timeseries set as: {{vm_max_datapoints_per_ts}}
```
```text
[Scheduler {{scheduler_alias}}] Reader endpoint SSL error {{url}}: {{error_message}}
```
```text
[Scheduler {{scheduler_alias}}] Reader endpoint inaccessible {{url}}: {{error_message}}
```
```text
[Scheduler {{scheduler_alias}}] Reader endpoint timeout {{url}}: {{error_message}}
```

---


**No data found (False)**. Based on [`query_from_last_seen_timestamp`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters) VmReader flag. A `warning` log is generated when no data is found in the requested range. This could indicate that the query was misconfigured or that no new data exists for the time period requested. Log message format:

```text
[Scheduler {{scheduler_alias}}] No data between {{start_s}} and {{end_s}} for query "{{query_key}}"
```

---

**No unseen data found (True)**. Based on [`query_from_last_seen_timestamp`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters) VmReader flag. A `warning` log is generated when no new data is returned (i.e., all data has already been seen in a previous inference step(s)). This helps in identifying situations where data for inference has already been processed. Based on VmReader's `adjust` flag. Log messages:

```text
[Scheduler {{scheduler_alias}}] No unseen data between {{start_s}} and {{end_s}} for query "{{query_key}}"
```

---

**Connection or timeout errors**. When the reader fails to retrieve data due to connection or timeout errors, a `warning` log is generated. These errors could result from network issues, incorrect query endpoints, or VictoriaMetrics being temporarily unavailable. Log message format:

```text
[Scheduler {{scheduler_alias}}] Error querying {{query_key}} for {{url}}: {{error_message}}
```

---

**Max datapoints warning**. If the requested query range (defined by `fit_every` or `infer_every` [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#parameters-1) args) exceeds the maximum number of datapoints allowed by VictoriaMetrics, a `warning` log is generated, and the request is split into multiple intervals{{% available_from "v1.14.1" anomaly %}}. This ensures that the request does not violate VictoriaMetrics’ constraints. Log messages:

```text
[Scheduler {{scheduler_alias}}] Query "{{query_key}}" from {{start_s}} to {{end_s}} with step {{step}} may exceed max datapoints per timeseries and will be split...
```

---

**Multi-tenancy warnings**. If the reader detects any issues related to missing or misconfigured multi-tenancy labels (a `warning` log{{% available_from "v1.16.2" anomaly %}} is generated to indicate the issue. See additional details [here](https://docs.victoriametrics.com/anomaly-detection/components/writer/#multitenancy-support). Log message format:

```text
The label vm_account_id was not found in the label set of {{query_key}}, but tenant_id='multitenant' is set in reader configuration...
```

---

**Metrics updated in read operations**. During successful query execution process, the following reader [self-monitoring metrics](#reader-behaviour-metrics) are updated:

- `vmanomaly_reader_request_duration_seconds`: Records the time (in seconds) taken to complete the query request.
  
- `vmanomaly_reader_responses`: Tracks the number of response codes received from VictoriaMetrics.

- `vmanomaly_reader_received_bytes`: Counts the number of bytes received in the response.

- `vmanomaly_reader_response_parsing_seconds`: Records the time spent parsing the response into different formats (e.g., JSON or DataFrame).

- `vmanomaly_reader_timeseries_received`: Tracks how many timeseries were retrieved in the query result.

- `vmanomaly_reader_datapoints_received`: Counts the number of datapoints retrieved in the query result.

---

**Metrics skipped in case of failures**. If an error occurs (connection or timeout), `vmanomaly_reader_received_bytes`, `vmanomaly_reader_timeseries_received`, and `vmanomaly_reader_datapoints_received` are not incremented because no valid data was received.

[Back to logging sections](#logs-generated-by-vmanomaly)

### Service logs

The `model` component (wrapped in service) logs operations during the fitting and inference stages for each model spec attached to particular [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) `scheduler_alias`. These logs inform about skipped runs, connection or timeout issues, invalid data points, and successful or failed model operations.

---

**Skipped runs**. When there are insufficient valid data points to fit or infer using a model, the run is skipped and a `warning` log is generated. This can occur when the query returns no new data or when the data contains invalid values (e.g., `NaN`, `INF`). The skipped run is also reflected in the `vmanomaly_model_runs_skipped` metric. Log messages:

When there are insufficient valid data points (at least 1 for [online models](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) and 2 for [offline models](https://docs.victoriametrics.com/anomaly-detection/components/models/#offline-models))
```text
[Scheduler {{scheduler_alias}}] Skipping run for stage 'fit' for model '{{model_alias}}' (query_key: {{query_key}}): Not enough valid data to fit: {{valid_values_cnt}}
```

When all the received timestamps during an `infer` call have already been processed, meaning the [`anomaly_score`](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) has already been produced for those points
```text
[Scheduler {{scheduler_alias}}] Skipping run for stage 'infer' for model '{{model_alias}}' (query_key: {{query_key}}): No unseen data to infer on.
```
When the model fails to produce any valid or finite outputs (such as [`anomaly_score`](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score))
```text
[Scheduler {{scheduler_alias}}] Skipping run for stage 'infer' for model '{{model_alias}}' (query_key: {{query_key}}): No (valid) datapoints produced.
```

---

**Errors during model execution**. If the model fails to fit or infer data due to internal service errors or model spec misconfigurations, an `error` log is generated and the error is also reflected in the `vmanomaly_model_run_errors` metric. This can occur during both `fit` and `infer` stages. Log messages:
```text
[Scheduler {{scheduler_alias}}] Error during stage 'fit' for model '{{model_alias}}' (query_key: {{query_key}}): {{error_message}}
```
```text
[Scheduler {{scheduler_alias}}] Error during stage 'infer' for model '{{model_alias}}' (query_key: {{query_key}}): {{error_message}}
```

---

**Model instance created during inference**. In cases where an [online model](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) instance is created during the inference stage (without a prior fit{{% available_from "v1.15.2" anomaly %}}), a `debug` log is produced. This helps track models that are created dynamically based on incoming data. Log messages:

```text
[Scheduler {{scheduler_alias}}] Model instance '{{model_alias}}' created for '{{query_key}}' during inference.
```
---

**Successful model runs**. When a model successfully fits, logs track the number of valid datapoints processed and the time taken for the operation. These logs are accompanied by updates to [self-monitoring metrics](#models-behaviour-metrics) like `vmanomaly_model_runs`, `vmanomaly_model_run_duration_seconds`, `vmanomaly_model_datapoints_accepted`, and `vmanomaly_model_datapoints_produced`. Log messages:

For [non-rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#non-rolling-models)
```text
[Scheduler {{scheduler_alias}}] Fitting on {{valid_values_cnt}}/{{total_values_cnt}} valid datapoints for "{{query_key}}" using model "{{model_alias}}".
```
```text
[Scheduler {{scheduler_alias}}] Model '{{model_alias}}' fit completed in {{model_run_duration}} seconds for {{query_key}}.
```
For [rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models) (combined stage)
```text
[Scheduler {{scheduler_alias}}] Fit-Infer on {{datapoint_count}} points for "{{query_key}}" using model "{{model_alias}}".
```

---

**Metrics updated in model runs**. During successful fit or infer operations, the following [self-monitoring metrics](#models-behaviour-metrics) are updated for each run:

- `vmanomaly_model_runs`: Tracks how many times the model ran (`fit`, `infer`, or `fit_infer`) for a specific `query_key`.

- `vmanomaly_model_run_duration_seconds`: Records the total time (in seconds) for the model invocation, based on the results of the `query_key`.

- `vmanomaly_model_datapoints_accepted`: The number of valid datapoints processed by the model during the run.

- `vmanomaly_model_datapoints_produced`: The number of datapoints generated by the model during inference.

- `vmanomaly_models_active`: Tracks the number of models currently **available for infer** for a specific `query_key`.

---

**Metrics skipped in case of failures**. If a model run fails due to an error or if no valid data is available, the metrics such as `vmanomaly_model_datapoints_accepted`, `vmanomaly_model_datapoints_produced`, and `vmanomaly_model_run_duration_seconds` are not updated.

---

[Back to logging sections](#logs-generated-by-vmanomaly)

### Writer logs

The `writer` component logs events during the process of sending produced data (like `anomaly_score` [metrics](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score)) to VictoriaMetrics. This includes data preparation, serialization, and network requests to VictoriaMetrics endpoints. The logs can help identify issues in data transmission, such as connection errors, invalid data points, and track the performance of write requests.

---

**Starting a write request**. A `debug` level log is produced when the `writer` component starts the process of writing data to VictoriaMetrics. It includes details like the number of datapoints, bytes of payload, and the query being written. This is useful for tracking the payload size and performance at the start of the request. Log messages:

```text
[Scheduler {{scheduler_alias}}] POST {{url}} with {{N}} datapoints, {{M}} bytes of payload, for {{query_key}}
```

---

**No valid data points**. A `warning` log is generated if there are no valid datapoints to write (i.e., all are `NaN` or unsupported like `INF`). This indicates that the writer will not send any data to VictoriaMetrics. Log messages:

```text
[Scheduler {{scheduler_alias}}] No valid datapoints to save for metric: {{query_key}}
```

---

**Connection, timeout, or I/O errors**. When the writer fails to send data due to connection, timeout, or I/O errors, an `error` log is generated. These errors often arise from network problems, incorrect URLs, or VictoriaMetrics being unavailable. The log includes details of the failed request and the reason for the failure. Log messages:

```text
[Scheduler {{scheduler_alias}}] Cannot write {{N}} points for {{query_key}}: connection error {{url}} {{error_message}}
```
```text
[Scheduler {{scheduler_alias}}] Cannot write {{N}} points for {{query_key}}: timeout for {{url}} {{error_message}}
```
```text
[Scheduler {{scheduler_alias}}] Cannot write {{N}} points for {{query_key}}: I/O error for {{url}} {{error_message}}
```

---

**Multi-tenancy warnings**. If the `tenant_id` is set to `multitenant` but the `vm_account_id` label is missing from the query result, or vice versa, a `warning` log is produced{{% available_from "v1.16.2" anomaly %}}. This helps in debugging label set issues that may occur due to the multi-tenant configuration - see [this section for details](https://docs.victoriametrics.com/anomaly-detection/components/writer/#multitenancy-support). Log messages:

```text
The label vm_account_id was not found in the label set of {{query_key}}, but tenant_id='multitenant' is set in writer...
```
```text
The label set for the metric {{query_key}} contains multi-tenancy labels, but the write endpoint is configured for single-tenant mode (tenant_id != 'multitenant')...
```

---

**Metrics updated in write operations**. During the successful write process of *non-empty data*, the following [self-monitoring metrics](#writer-behaviour-metrics) are updated:

- `vmanomaly_writer_request_duration_seconds`: Records the time (in seconds) taken to complete the write request.

- `vmanomaly_writer_sent_bytes`: Tracks the number of bytes sent in the request.

- `vmanomaly_writer_responses`: Captures the HTTP response code returned by VictoriaMetrics. In case of connection, timeout, or I/O errors, a specific error code (`connection_error`, `timeout`, or `io_error`) is recorded instead.

- `vmanomaly_writer_request_serialize_seconds`: Records the time taken for data serialization.

- `vmanomaly_writer_datapoints_sent`: Counts the number of valid datapoints that were successfully sent.

- `vmanomaly_writer_timeseries_sent`: Tracks the number of timeseries sent to VictoriaMetrics.

**Metrics skipped in case of failures**. If an error occurs (connection, timeout, or I/O error), only `vmanomaly_writer_request_duration_seconds` is updated with appropriate error code. 

[Back to logging sections](#logs-generated-by-vmanomaly)
