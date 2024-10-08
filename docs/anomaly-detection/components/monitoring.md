---
title: Monitoring
weight: 5
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 5
    identifier: "vmanomaly-monitoring"
aliases:
  - ./monitoring.html
---
There are 2 models to monitor VictoriaMetrics Anomaly Detection behavior - [push](https://docs.victoriametrics.com/keyconcepts/#push-model) and [pull](https://docs.victoriametrics.com/keyconcepts/#pull-model). Parameters for each of them should be specified in the config file, `monitoring` section.

## Pull Model Config parameters

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

`url`
            </td>
            <td></td>
            <td>

Link where to push metrics to. Example: `"http://localhost:8480/"`
            </td>
        </tr>
        <tr>
            <td>

`tenant_id`
            </td>
            <td></td>
            <td>

Tenant ID for cluster version. Example: `"0:0"`
            </td>
        </tr>
        <tr>
            <td>

`health_path`
            </td>
            <td>

`"health"`
            </td>
            <td>

Deprecated since [v1.8.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v180). Absolute, to override `/health` path
            </td>
        </tr>
        <tr>
            <td>

`user`
            </td>
            <td></td>
            <td>BasicAuth username</td>
        </tr>
        <tr>
            <td>

`password`
            </td>
            <td></td>
            <td>BasicAuth password</td>
        </tr>
        <tr>
            <td>
`bearer_token`
            </td>
            <td>
`token`
            </td>
            <td>
Token is passed in the standard format with header: `Authorization: bearer {token}`. Available since [v1.15.9](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1159)
            </td>
        </tr>
        <tr>
            <td>
`bearer_token_file`
            </td>
            <td>
`path_to_file`
            </td>
            <td>
Path to a file, which contains token, that is passed in the standard format with header: `Authorization: bearer {token}`. Available since [v1.15.9](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1159)
            </td>
        </tr>
        <tr>
            <td>
`verify_tls`
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
`tls_cert_file`
            </td>
            <td>
`path/to/cert.crt`
            </td>
            <td>
Path to a file with the client certificate, i.e. `client.crt`. Available since [v1.16.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1163).
            </td>
        </tr>
        <tr>
            <td>
`tls_key_file`
            </td>
            <td>
`path/to/key.crt`
            </td>
            <td>
Path to a file with the client certificate key, i.e. `client.key`. Available since [v1.16.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1163).
            </td>
        </tr>
        <tr>
            <td>

`timeout`
            </td>
            <td>

`"5s"`
            </td>
            <td>Stop waiting for a response after a given number of seconds.</td>
        </tr>
        <tr>
            <td>

`extra_labels`
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
    extra_labels:
      job: "vmanomaly-push"
      test: "test-1"
```

## mTLS protection

Starting from [v1.16.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1163), `vmanomaly` components such as [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) support [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication) to ensure secure communication with [VictoriaMetrics Enterprise, configured with mTLS](https://docs.victoriametrics.com/#mtls-protection).

For detailed guidance on configuring mTLS parameters such as `verify_tls`, `tls_cert_file`, and `tls_key_file`, please refer to the [mTLS protection section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#mtls-protection) in the [Reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) documentation. The configuration principles apply consistently across all these `vmanomaly` components.

## Metrics generated by vmanomaly

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`vmanomaly_start_time_seconds`
            </td>
            <td>Gauge</td>
            <td>vmanomaly start time in UNIX time</td>
        </tr>
    </tbody>
</table>

### Models Behaviour Metrics
Label names [description](#labelnames)

> **Note**: There is a new label key `model_alias` introduced in multi-model support [v1.10.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1100). This label key adjustment was made to preserve unique label set production during writing produced metrics back to VictoriaMetrics.

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th>Description</th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`vmanomaly_model_runs`
            </td>
            <td>Counter</td>
            <td>How many times models ran (per model)</td>
            <td>

`stage, query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_model_run_duration_seconds`
            </td>
            <td>Summary</td>
            <td>How much time (in seconds) model invocations took</td>
            <td>

`stage, query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_model_datapoints_accepted`
            </td>
            <td>Counter</td>
            <td>How many datapoints did models accept</td>
            <td>

`stage, query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_model_datapoints_produced`
            </td>
            <td>Counter</td>
            <td>How many datapoints were generated by models</td>
            <td>

`stage, query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_models_active`
            </td>
            <td>Gauge</td>
            <td>How many models are currently inferring</td>
            <td>

`query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_model_runs_skipped`
            </td>
            <td>Counter</td>
            <td>How many times a run was skipped (per model)</td>
            <td>

`stage, query_key, model_alias, scheduler_alias, preset`
            </td>
        </tr>
    </tbody>
</table>

### Writer Behaviour Metrics
Label names [description](#labelnames)

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th>Description</th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`vmanomaly_writer_request_duration_seconds`
            </td>
            <td>Summary</td>
            <td>How much time (in seconds) did requests to VictoriaMetrics take</td>
            <td>

`url, query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_writer_response_count`
            </td>
            <td>Counter</td>
            <td>Response code counts we got from VictoriaMetrics</td>
            <td>

`url, query_key, code`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_writer_sent_bytes`
            </td>
            <td>Counter</td>
            <td>How much bytes were sent to VictoriaMetrics</td>
            <td>

`url, query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_writer_request_serialize_seconds`
            </td>
            <td>Summary</td>
            <td>How much time (in seconds) did serializing take</td>
            <td>

`query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_writer_datapoints_sent`
            </td>
            <td>Counter</td>
            <td>How many datapoints were sent to VictoriaMetrics</td>
            <td>

`query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_writer_timeseries_sent`
            </td>
            <td>Counter</td>
            <td>How many timeseries were sent to VictoriaMetrics</td>
            <td>

`query_key`
            </td>
        </tr>
    </tbody>
</table>

### Reader Behaviour Metrics
Label names [description](#labelnames)

<table class="params">
    <thead>
        <tr>
            <th>Metric</th>
            <th>Type</th>
            <th>Description</th>
            <th>Labelnames</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`vmanomaly_reader_request_duration_seconds`
            </td>
            <td>Summary</td>
            <td>How much time (in seconds) did queries to VictoriaMetrics take</td>
            <td>

`url, query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_reader_response_count`
            </td>
            <td>Counter</td>
            <td>Response code counts we got from VictoriaMetrics</td>
            <td>

`url, query_key, code`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_reader_received_bytes`
            </td>
            <td>Counter</td>
            <td>How much bytes were received in responses</td>
            <td>

`query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_reader_response_parsing_seconds`
            </td>
            <td>Summary</td>
            <td>How much time (in seconds) did parsing take for each step</td>
            <td>

`step`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_reader_timeseries_received`
</td>
            <td>Counter</td>
            <td>How many timeseries were received from VictoriaMetrics</td>
            <td>

`query_key`
            </td>
        </tr>
        <tr>
            <td>

`vmanomaly_reader_datapoints_received`
            </td>
            <td>Counter</td>
            <td>How many rows were received from VictoriaMetrics</td>
            <td>

`query_key`
            </td>
        </tr>
    </tbody>
</table>

### Labelnames

* `stage` - stage of model - 'fit', 'infer' or 'fit_infer' for models that do it simultaneously, see [model types](https://docs.victoriametrics.com/anomaly-detection/components/models/#model-types).
* `query_key` - query alias from [`reader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/) config section.
* `model_alias` - model alias from [`models`](https://docs.victoriametrics.com/anomaly-detection/components/models/) config section. **Introduced in [v1.10.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1100).**
* `scheduler_alias` - scheduler alias from [`schedulers`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler) config section. **Introduced in [v1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110).**
* `preset` - preset alias for forthcoming `preset` section compatibility. **Introduced in [v1.12.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1120).**
* `url` - writer or reader url endpoint.
* `code` - response status code or `connection_error`, `timeout`.
* `step` - json or dataframe reading step.
