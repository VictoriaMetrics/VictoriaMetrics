---
title: Writer
weight: 4
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 4
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
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`class`
            </td>
            <td>

`writer.vm.VmWriter` or `vm` starting from [`v1.13.0`](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130)
            </td>
            <td>

Name of the class needed to enable writing to VictoriaMetrics or Prometheus. VmWriter is the default option, if not specified.
            </td>
        </tr>
        <tr>
            <td>

`datasource_url`
            </td>
            <td>

`http://localhost:8481/`
            </td>
            <td>

Datasource URL address
            </td>
        </tr>
        <tr>
            <td>

`tenant_id`
            </td>
            <td>

`0:0`
            </td>
            <td>

For VictoriaMetrics Cluster version only, tenants are identified by accountID or accountID:projectID. See VictoriaMetrics Cluster [multitenancy docs](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy)
            </td>
        </tr>
        <!-- Additional rows for metric_format -->
        <tr>
            <td rowspan="4">

`metric_format`
            </td>
            <td>

`__name__: "vmanomaly_$VAR"`
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

`for: "$QUERY_KEY"`
            </td>
        </tr>
        <tr>
            <td>

`run: "test_metric_format"`
            </td>
        </tr>
        <tr>
            <td>

`config: "io_vm_single.yaml"`
            </td>
        </tr>  
        <!-- End of additional rows -->
        <tr>
            <td>

`import_json_path`
            </td>
            <td>

`/api/v1/import`
            </td>
            <td>

Optional, to override the default import path
            </td>
        </tr>
        <tr>
            <td>

`health_path`
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

`user`
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

`password`
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

`timeout`
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

`verify_tls`
            </td>
            <td>

`false`
            </td>
            <td>

Allows disabling TLS verification of the remote certificate.
            </td>
        </tr>
        <tr>
            <td>

`bearer_token`
            </td>
            <td>

`token`
            </td>
            <td>

Token is passed in the standard format with the header: `Authorization: bearer {token}`
            </td>
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
