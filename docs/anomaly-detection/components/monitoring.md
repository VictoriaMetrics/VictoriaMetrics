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
            <td>Effective memory capacity available to the process in bytes{{% available_from "v1.18.4" anomaly %}}. The value honors cgroup limits when available, then process address-space limits, and otherwise reports host physical memory. It does not represent currently unused memory.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_cpu_cores_available`</span>
            </td>
            <td>Gauge</td>
            <td>Effective CPU capacity available to the process{{% available_from "v1.18.4" anomaly %}}, constrained by host logical CPUs, process affinity, and cgroup quota. The value can be fractional when a fractional CPU quota is configured.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_config_entities`</span>
            </td>
            <td>Gauge</td>
            <td>Number of [sub-configs](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#sub-configuration) **available** (`scope="total"`) and **used** by the current [shard](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability) (`scope="shard"`){{% available_from "v1.21.0" anomaly %}}, labeled by `preset` and `scope`.</td>
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
        <tr>
            <td>
<span style="white-space: nowrap;">`vmanomaly_scheduler_alive`</span>
            </td>
            <td>Gauge</td>
            <td>Whether the scheduler worker thread identified by `scheduler_alias` and `preset` is alive (`1`) or not (`0`) {{% available_from "v1.30.0" anomaly %}}.</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vmanomaly_scheduler_restarts_total`</span>
            </td>
            <td>Counter</td>
            <td>Number of bounded scheduler restart attempts {{% available_from "v1.30.0" anomaly %}}, labeled by `scheduler_alias`, `preset`, and `status` (`success` or `failure`).</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vm_license_expires_at`</span>
            </td>
            <td>Gauge</td>
            <td>License expiration time as a Unix timestamp in seconds. See the [licensing section](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing) for example alerts.</td>
        </tr>
        <tr>
            <td>
<span style="white-space: nowrap;">`vm_license_expires_in_seconds`</span>
            </td>
            <td>Gauge</td>
            <td>Time remaining until license expiration in seconds. See the [licensing section](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing) for warning and critical alert examples.</td>
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
            <td>The total time (in seconds) taken for data parsing at each `step` (`json` or `df`) for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode.</td>
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
        <tr>
            <td>

<span style="white-space: nowrap;">`vmanomaly_reader_processing_tasks_queued`</span>
            </td>
            <td>

`Gauge`
            </td>
            <td>The total number of queued processing tasks {{% available_from "v1.29.7" anomaly %}} (timeseries batches of size `series_processing_batch_size`) for the `query_key` query within the specified scheduler `scheduler_alias`, in the `vmanomaly` service running in `preset` mode. If continuously >0, it may lead to skipped infer runs due to resource contention and timeouts.</td>
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
            <td>The model-service stage duration in seconds for `fit`, `infer`, or combined `fit_infer` execution, based on the results of the `query_key` query for `model_alias`. Reader and writer I/O durations are reported by their respective metrics.</td>
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
            <td>The number of valid datapoints accepted by `model_alias`, excluding NaN and Inf values, during `fit`, `infer`, or combined `fit_infer` execution for the `query_key` query.</td>
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

<span style="white-space: nowrap;">`vmanomaly_writer_responses`</span> (named `vmanomaly_writer_response_count`{{% deprecated_from "v1.17.0" anomaly %}})
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

* `stage` - model execution stage: `fit`, `infer`, or `fit_infer` for a combined fit/inference scheduler run. See [model types](https://docs.victoriametrics.com/anomaly-detection/components/models/#model-types).
* `query_key` - query alias from [`reader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/) config section.
* `model_alias` - model alias from [`models`](https://docs.victoriametrics.com/anomaly-detection/components/models/) config section{{% available_from "v1.10.0" anomaly %}}.
* `scheduler_alias` - scheduler alias from [`schedulers`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) config section{{% available_from "v1.11.0" anomaly %}}.
* `preset` - preset alias for [`preset`](https://docs.victoriametrics.com/anomaly-detection/presets/) mode of `vmanomaly`{{% available_from "v1.12.0" anomaly %}}.
* `url` - writer or reader url endpoint.
* `code` - HTTP response status code or `connection_error`, `timeout`, `ssl_error`, or `io_error`.
* `step` - reader parsing step: `json` or `df`.

[Back to metric sections](#metrics-generated-by-vmanomaly)


## Logs generated by vmanomaly

The `vmanomaly` service logs important lifecycle, I/O, model, and recovery events alongside
[self-monitoring metrics](#metrics-generated-by-vmanomaly). The fragments below are stable prefixes for
recognizing log families, rather than byte-for-byte message contracts; entity values and exception details follow
the prefix.

By default, `vmanomaly` uses the `INFO` level. Use the global `--loggerLevel` command-line argument or
`settings.logger_levels`{{% available_from "v1.30.0" anomaly %}} for prefix-based component overrides:

```yaml
settings:
  logger_levels:
    reader: DEBUG       # also applies to reader.vm, reader.vlogs, and other child loggers
    writer.vm: ERROR
    copilot: WARNING
```

More-specific prefixes override their parent. Changes limited to `settings.logger_levels` can be
[hot-reloaded](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) without restarting
services. See [`settings.logger_levels`](https://docs.victoriametrics.com/anomaly-detection/components/settings/#logger-levels)
and the [command-line arguments](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments).

- [Startup logs](#startup-logs)
- [Reader logs](#reader-logs)
- [Service logs](#service-logs)
- [Writer logs](#writer-logs)
- [Scheduler supervision logs](#scheduler-supervision-logs)
- [Hot-reload logs](#hot-reload-logs)
- [Persisted-state logs](#persisted-state-logs)
- [Query server and task logs](#query-server-and-task-logs)
- [AI Copilot logs](#ai-copilot-logs)


### Startup logs

Startup logs summarize the version, license, effective storage mode, state restoration, process-pool mode,
server addresses, hot-reload state, and active schedulers. The most useful prefixes are:

- **License check**: `Please provide a license code`, `failed to read file`, and `Licensed to`.
- **Config validation**: `Config validation failed`, `Config read failed`, and the fatal
  `Config validation failed, shutting down`. Successful startup ends with `Config has been loaded successfully`.
- **Model and data directory setup**: `Using ENV VMANOMALY_MODEL_DUMPS_DIR`,
  `Using ENV VMANOMALY_DATA_DUMPS_DIR`, or their `is not set` in-memory variants. See
  [on-disk mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).
- **Scheduler and service initialization**: `Version:`, `Using process pool executor`, `Listening on`,
  `Serving /metrics`, `Hot reload enabled`, and `Active schedulers`. `Process pool health check failed, falling
  back to sequential mode` reports a safe runtime fallback. Per-scheduler wrapping and omitted empty schedulers are
  `DEBUG` diagnostics.

[Back to logging sections](#logs-generated-by-vmanomaly)

### Reader logs

Reader logs cover endpoint checks, request splitting, network failures, response parsing, and coordination between
queries used by the same model.

**Starting a healthcheck request**. The reader probes each configured tenant and discovers
`search.maxPointsPerTimeseries`. `Max points per timeseries set as` is a `DEBUG` diagnostic. A warning beginning
`Could not get constraints` means the reader uses its built-in limit. Endpoint initialization errors identify SSL,
connection, or timeout failures.

**No data found (False)**. A fit/read range with no results uses this form, showing both local and Unix times:

```text
[Scheduler `SCHEDULER`] No data for query_key `QUERY` between LOCAL_START and LOCAL_END timezone TZ (START_EPOCH to END_EPOCH)
```

Check the query, tenant, offsets, and selected range.

**No unseen data found (True)**. An inference read whose timestamps were already processed uses:

```text
[Scheduler `SCHEDULER`] No unseen data for query_key `QUERY` between LOCAL_START and LOCAL_END, timezone TZ (START_EPOCH to END_EPOCH)
```

This can be expected for overlapping scheduler windows, UI range navigation, or retries. Investigate when it
persists while the datasource continues receiving newer samples.

**Connection or timeout errors**. `Error querying URL for QUERY with PARAMS` includes the effective endpoint,
query alias, request parameters, and nested SSL, connection, timeout, or I/O reason. The corresponding
`vmanomaly_reader_responses` code is `ssl_error`, `connection_error`, `timeout`, or `io_error`.

At `DEBUG`, reader request lines start with `[Scheduler ...] GET` or `OPTIONS` and show the effective URL;
token query parameters are redacted. `Cancellation requested for query` records cooperative cancellation.
`Failed queries detected`, `Timeout waiting for queries`, and `Auto-marking pending queries as failed` identify
coordination failures for related query sets.

**Max datapoints warning**. `Query "QUERY" from START to END with step ... may exceed max datapoints per
timeseries (LIMIT)` means the range will be split{{% available_from "v1.14.1" anomaly %}}. The message reports the
effective limit and suggests reducing the range, increasing the step, or raising
`search.maxPointsPerTimeseries`. A `DEBUG` message reports the resulting interval count.

**Multi-tenancy warnings**. Messages starting with `The label vm_account_id was not found` indicate that a
multitenant query lost routing labels. Preserve `vm_account_id` and `vm_project_id` through query aggregation; see
[multitenancy support](https://docs.victoriametrics.com/anomaly-detection/components/writer/#multitenancy-support).

**Metrics updated in read operations**. Requests update duration and response-code metrics even on handled
failures. Bytes, time series, datapoints, and parsing durations are recorded only when those values were received
or parsed. See [reader behaviour metrics](#reader-behaviour-metrics).

[Back to logging sections](#logs-generated-by-vmanomaly)

### Service logs

The service logs `fit`, `infer`, and combined `fit_infer`/backtesting work for each model alias and scheduler.
The `query_key` value may be a composite key containing source labels and an internal hash, rather than only the
configured query alias.

**Skipped runs**. Warnings start with `Skipping run for stage 'STAGE' for model 'MODEL'`. Common reasons are no
fit or inference partition, no data to infer, no unseen valid data, a missing model instance or on-disk model, an
unsupported exact-batch path, or no valid output. The service attempts fitting when at least one valid row exists;
individual models may require more history and report their own error. Skips increment
`vmanomaly_model_runs_skipped`.

**Errors during model execution**. Errors start with `Error during stage 'STAGE' for model 'MODEL'` and include
the composite query key and exception. They increment `vmanomaly_model_run_errors`.

**Model instance created during inference**. `Model instance 'MODEL' created ... during inference` is a `DEBUG`
message for an online model cold start{{% available_from "v1.15.2" anomaly %}}.

**Successful model runs**. `Fitting on VALID/TOTAL valid datapoints` is emitted at `INFO`. At `DEBUG`,
`Model ... fit completed`, `Inference ran in`, and `Fit-Infer ran in` report stage duration. Combined
`fit_infer` is used by applicable backtesting/scheduler execution and is not a separate “rolling model” class.

**Metrics updated in model runs**. Successful stages update runs, duration, accepted/produced datapoints, and
active-model gauges. Skips and failures update their respective counters; success-only values are not recorded for
an unsuccessful stage. See [models behaviour metrics](#models-behaviour-metrics).

[Back to logging sections](#logs-generated-by-vmanomaly)

### Writer logs

Writer logs cover serialization and delivery of produced series such as
[`anomaly_score`](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score).

**Starting a write request**. At `DEBUG`, `[Scheduler ...] POST URL with N datapoints, M bytes of payload`
includes the composite query key and dataframe shape.

**No valid data points**. `No valid datapoints to save for metric` includes the query key and original dataframe
shape; no request is sent.

**Connection, timeout, or I/O errors**. `Cannot write N points for QUERY` ends with an SSL, connection, timeout,
or I/O reason. A retriable connection failure first emits `Connection error while writing ... reinitializing
session and retrying`; the final failed attempt is logged as an error.

**Multi-tenancy warnings**. `The label vm_account_id was not found` means a `multitenant` writer will fall back to
tenant `0:0`. `The label set for the metric ... contains multi-tenancy labels` means labels disagree with the
configured single tenant. Preserve or align tenant labels and `writer.tenant_id`; see
[multitenancy support](https://docs.victoriametrics.com/anomaly-detection/components/writer/#multitenancy-support).

**Metrics updated in write operations**. Request duration is observed for successful and handled failed requests.
`vmanomaly_writer_responses` records the HTTP status or `ssl_error`, `connection_error`, `timeout`, or `io_error`.
Serialization duration and prepared time-series count may already be recorded before a failed request; sent bytes
and datapoints are recorded only after a successful response. See [writer behaviour metrics](#writer-behaviour-metrics).

[Back to logging sections](#logs-generated-by-vmanomaly)

### Scheduler supervision logs

Scheduler supervision{{% available_from "v1.30.0" anomaly %}} logs a dead worker, automatic restart, successful
recovery, failed-attempt backoff, and removal after the retry limit. Stable prefixes include `Scheduler ... is not
alive`, `Scheduler ... restarted successfully`, `restart attempt ... failed`, and `reached max restart attempts`.
Correlate them with `vmanomaly_scheduler_alive` and `vmanomaly_scheduler_restarts_total`.

[Back to logging sections](#logs-generated-by-vmanomaly)

### Hot-reload logs

Hot reload logs config-change detection, validation, staged service restart, success, and rollback. `Reload aborted
– invalid config` keeps the current runtime unchanged; `Reload apply failed; attempting rollback` starts recovery.
`Rollback failed` is critical and requests shutdown. A logger-only change emits `Applied component log level changes
without restarting services`.

[Back to logging sections](#logs-generated-by-vmanomaly)

### Persisted-state logs

With `settings.restore_state`, startup logs the stored/runtime version assessment, reusable components, required
model or reader-data purges, and restored jobs/services. `Persisted state is incompatible` followed by `Dropping
stored artifacts completely` indicates a full reset; missing or unreadable model files are reported separately.

[Back to logging sections](#logs-generated-by-vmanomaly)

### Query server and task logs

The query server logs its listening address and datasource-proxy timeouts/failures. Background anomaly-detection
and autotune failures use `Error in task` and `Error in autotune task`; canceled client requests may still leave a
background raw query finishing cleanly.

[Back to logging sections](#logs-generated-by-vmanomaly)

### AI Copilot logs

AI Copilot{{% available_from "v1.30.0" anomaly %}} reports whether it is initialized, disabled, misconfigured, or
unable to mount. `Invalid Copilot request state` identifies an incomplete/canceled tool-call history, `Copilot
request failed` identifies provider execution failure, and `MCP server unreachable` identifies unavailable MCP
guidance tools.

[Back to logging sections](#logs-generated-by-vmanomaly)
