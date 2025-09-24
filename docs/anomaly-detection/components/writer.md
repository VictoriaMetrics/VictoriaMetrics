---
title: Writer
weight: 4
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 4
tags:
  - metrics
  - enterprise
aliases:
  - /anomaly-detection/components/writer.html
---
For exporting data, VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily employs the [VmWriter](#vm-writer), which writes produced anomaly scores **(preserving initial labelset and optionally applying additional ones)** back to VictoriaMetrics. This writer is tailored for smooth data export within the VictoriaMetrics ecosystem.

Future updates will introduce additional export methods, offering users more flexibility in data handling and integration.

## VM writer

### Config parameters

<table class="params">
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Example</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`class`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`writer.vm.VmWriter` or `vm`{{% available_from "v1.13.0" anomaly %}}
</span>
            </td>
            <td>

Name of the class needed to enable writing to VictoriaMetrics or Prometheus. VmWriter is the default option, if not specified.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`datasource_url`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`http://localhost:8481/`</span>
            </td>
            <td>

Datasource URL address
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tenant_id`</span>
            </td>
            <td>
<span>

`0:0`, `multitenant`{{% available_from "v1.16.2" anomaly %}}
</span>
            </td>
            <td>

For VictoriaMetrics Cluster version only, tenants are identified by `accountID` or `accountID:projectID`. multitenant` [endpoint](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels){{% available_from "v1.16.2" anomaly %}} is supported, to write data to multiple [tenants](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy). See VictoriaMetrics Cluster [multitenancy docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
            </td>
        </tr>
        <!-- Additional rows for metric_format -->
        <tr>
            <td rowspan="4">

<span style="white-space: nowrap;">`metric_format`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`__name__: "vmanomaly_$VAR"`</span>
            </td>
            <td rowspan="4">

Metrics to save the output (in metric names or labels). Must have `__name__` key. Must have a value with `$VAR` placeholder in it to distinguish between resulting metrics. Supported placeholders:
                <ul>
                    <li>

`$VAR` -- Variables that model provides, all models provide the following set: {"anomaly_score", "y", "yhat", "yhat_lower", "yhat_upper"}. Description of standard output is [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#vmanomaly-output). Depending on [model type](https://docs.victoriametrics.com/anomaly-detection/components/models/) it can provide more metrics, like "trend", "seasonality" etc.
                    </li>
                    <li>

`$QUERY_KEY` -- E.g. "ingestion_rate".
                    </li>
                </ul>
                Other keys are supposed to be configured by the user to help identify generated metrics, e.g., specific config file name etc.
                More details on metric formatting are [here](#metrics-formatting).
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`for: "$QUERY_KEY"`</span>
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`run: "test_metric_format"`</span>
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`config: "io_vm_single.yaml"`</span>
            </td>
        </tr>  
        <!-- End of additional rows -->
        <tr>
            <td>

<span style="white-space: nowrap;">`import_json_path`</span>
            </td>
            <td>

`/api/v1/import`{{% deprecated_from "v1.19.2" anomaly %}}
            </td>
            <td>

Optional, to override the default import path
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`health_path`</span>
            </td>
            <td>

`/health`
            </td>
            <td>

Absolute or relative URL address where to check the availability of the datasource. Optional, to override the default `/health` path.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`user`</span>
            </td>
            <td>

`USERNAME`
            </td>
            <td>

BasicAuth username
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`password`</span>
            </td>
            <td>

`PASSWORD`
            </td>
            <td>

BasicAuth password
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`timeout`</span>
            </td>
            <td>

`5s`
            </td>
            <td>

Timeout for the requests, passed as a string
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

`path/to/cert.crt`
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

<span style="white-space: nowrap;">`bearer_token`</span>
            </td>
            <td>

`token`
            </td>
            <td>
Token is passed in the standard format with header: `Authorization: bearer {token}`
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
<span>
Path to a file, which contains token, that is passed in the standard format with header: `Authorization: bearer {token}`{{% available_from "v1.15.9" anomaly %}}
</span>            </td>
        </tr>
    </tbody>
</table>

Config example:

```yaml
writer:
  class: "vm"  # or "writer.vm.VmWriter" until v1.13.0
  datasource_url: "http://localhost:8428/"
  tenant_id: "0:0"
  metric_format:
    __name__: "vmanomaly_$VAR"
    for: "$QUERY_KEY"
    run: "test_metric_format"
    config: "io_vm_single.yaml"
  import_json_path: "/api/v1/import"
  health_path: "health"
  user: "foo"
  password: "bar"
```

### Multitenancy support

> This feature applies to the VictoriaMetrics Cluster version only. Tenants are identified by either `accountID` or `accountID:projectID`. `multitenant` [endpoint](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels){{% available_from "v1.15.9" anomaly %}} is supported for writing data across multiple [tenants](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy). For more details, refer to the VictoriaMetrics Cluster [multitenancy documentation](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

Please note the different behaviors depending on the `tenant_id` value:

1. **When `writer.tenant_id != 'multitenant'` (e.g., `"0:0"`) and `reader.tenant_id != 'multitenant'` (can be different but valid, like `"0:1")**:
   - The `vm_account_id` label is **not created** in the reader, **not persisted** to the writer, and is **not expected** in the output.
   - **Result**: Data is written successfully with no logs or errors.

2. **When `writer.tenant_id = 'multitenant'` and `vm_project_id` is present in the label set**:
   - This typically happens when `reader.tenant_id` is also set to `multitenant`, meaning the `vm_account_id` label is stored in the results returned from the queries.
   - **Result**: Everything functions as expected. Data is written successfully with no logs or errors.

3. **When `writer.tenant_id = 'multitenant'` but `vm_account_id` is missing** (e.g., due to aggregation in the reader or missing `keep_metric_names` in the query):
   - **Result**: The data is still written to `"0:0"`, but a warning is raised:
    ```
    The label `vm_account_id` was not found in the label set of {query_result.key}, 
    but tenant_id='multitenant' is set in writer. The data will be written to the default tenant 0:0. 
    Ensure that the query retains the necessary multi-tenant labels, 
    or adjust the aggregation settings to preserve `vm_account_id` key in the label set.
    ```

4. **When `writer.tenant_id != 'multitenant'` (e.g., `"0:0"`) and `vm_account_id` exists in the label set**:
   - **Result**: Writing is allowed, but a warning is raised:
    ```
    The label set for the metric {query_result.key} contains multi-tenancy labels, 
    but the write endpoint is configured for single-tenant mode (tenant_id != 'multitenant'). 
    Either adjust the query in the reader to avoid multi-tenancy labels 
    or ensure that reserved key `vm_account_id` is not explicitly set for single-tenant environments.
    ```

### mTLS protection

`vmanomaly` components such as [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) support [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication) to ensure secure communication with [VictoriaMetrics Enterprise, configured with mTLS](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#mtls-protection){{% available_from "v1.16.3" anomaly %}}.

For detailed guidance on configuring mTLS parameters such as `verify_tls`, `tls_cert_file`, and `tls_key_file`, please refer to the [mTLS protection section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#mtls-protection) in the [Reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) documentation. The configuration principles apply consistently across all these `vmanomaly` components.


### Healthcheck metrics

`VmWriter` exposes [several healthchecks metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics). 

### Metrics formatting

There should be 2 mandatory parameters set in `metric_format` - `__name__` and `for`. 

```yaml
__name__: PREFIX1_$VAR
for: PREFIX2_$QUERY_KEY
```

* for `__name__` parameter it will name metrics returned by models as `PREFIX1_anomaly_score`, `PREFIX1_yhat_lower`, etc. Vmanomaly output metrics names described [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#vmanomaly-output)
* for `for` parameter will add labels `PREFIX2_query_name_1`, `PREFIX2_query_name_2`, etc. Query names are set as aliases in config `reader` section in [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters) parameter.

It is possible to specify other custom label names needed.
For example:

```yaml
custom_label_1: label_name_1
custom_label_2: label_name_2
```

Apart from specified labels, output metrics will return **labels inherited from input metrics returned by [queries](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters)**.
For example if input data contains labels such as `cpu=1, device=eth0, instance=node-exporter:9100` all these labels will be present in vmanomaly output metrics.

So if metric_format section was set up like this:

```yaml
metric_format:
    __name__: "PREFIX1_$VAR"
    for: "PREFIX2_$QUERY_KEY"
    custom_label_1: label_name_1
    custom_label_2: label_name_2
```

It will return metrics that will look like:

```promtextmetric
{__name__="PREFIX1_anomaly_score", for="PREFIX2_query_name_1", custom_label_1="label_name_1", custom_label_2="label_name_2", cpu=1, device="eth0", instance="node-exporter:9100"}
{__name__="PREFIX1_yhat_lower", for="PREFIX2_query_name_1", custom_label_1="label_name_1", custom_label_2="label_name_2", cpu=1, device="eth0", instance="node-exporter:9100"}
{__name__="PREFIX1_anomaly_score", for="PREFIX2_query_name_2", custom_label_1="label_name_1", custom_label_2="label_name_2", cpu=1, device="eth0", instance="node-exporter:9100"}
{__name__="PREFIX1_yhat_lower", for="PREFIX2_query_name_2", custom_label_1="label_name_1", custom_label_2="label_name_2", cpu=1, device="eth0", instance="node-exporter:9100"}
```
