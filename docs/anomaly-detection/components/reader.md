---
sort: 2
title: Reader
weight: 2
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 2
aliases:
  - /anomaly-detection/components/reader.html
---

# Reader

<!--
There are 4 sources available to read data into VM Anomaly Detection from: VictoriaMetrics, (ND)JSON file, QueryRange, or CSV file. Depending on the data source, different parameters should be specified in the config file in the `reader` section.
-->

VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily uses [VmReader](#vm-reader) to ingest data. This reader focuses on fetching time-series data directly from VictoriaMetrics with the help of powerful [MetricsQL](https://docs.victoriametrics.com/metricsql/) expressions for aggregating, filtering and grouping your data, ensuring seamless integration and efficient data handling. 

Future updates will introduce additional readers, expanding the range of data sources `vmanomaly` can work with.


## VM reader

### Config parameters

<table>
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td><code>class</code></td>
            <td><code>"reader.vm.VmReader" (or "vm" starting from <a href="https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130">v1.13.0</a>)</code></td>
            <td>Name of the class needed to enable reading from VictoriaMetrics or Prometheus. VmReader is the default option, if not specified.</td>
        </tr>
        <tr>
            <td><code>queries</code></td>
            <td><code>"ingestion_rate: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'"</code></td>
            <td>PromQL/MetricsQL query to select data in format: <code>QUERY_ALIAS: "QUERY"</code>. As accepted by <code>"/query_range?query=%s"</code>.</td>
        </tr>
        <tr>
            <td><code>datasource_url</code></td>
            <td><code>"http://localhost:8481/"</code></td>
            <td>Datasource URL address</td>
        </tr>
        <tr>
            <td><code>tenant_id</code></td>
            <td><code>"0:0"</code></td>
            <td>For VictoriaMetrics Cluster version only, tenants are identified by accountID or accountID:projectID. See VictoriaMetrics Cluster <a href="https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy">multitenancy docs</a></td>
        </tr>
        <tr>
            <td><code>sampling_period</code></td>
            <td><code>"1h"</code></td>
            <td>Frequency of the points returned. Will be converted to <code>"/query_range?step=%s"</code> param (in seconds). <b>Required</b> since <a href="https://docs.victoriametrics.com/anomaly-detection/changelog/#v190">v1.9.0</a>.</td>
        </tr>
        <tr>
            <td><code>query_range_path</code></td>
            <td><code>"api/v1/query_range"</code></td>
            <td>Performs PromQL/MetricsQL range query. Default <code>"api/v1/query_range"</code></td>
        </tr>
        <tr>
            <td><code>health_path</code></td>
            <td><code>"health"</code></td>
            <td>Absolute or relative URL address where to check availability of the datasource. Default is <code>"health"</code>.</td>
        </tr>
        <tr>
            <td><code>user</code></td>
            <td><code>"USERNAME"</code></td>
            <td>BasicAuth username</td>
        </tr>
        <tr>
            <td><code>password</code></td>
            <td><code>"PASSWORD"</code></td>
            <td>BasicAuth password</td>
        </tr>
        <tr>
            <td><code>timeout</code></td>
            <td><code>"30s"</code></td>
            <td>Timeout for the requests, passed as a string. Defaults to "30s"</td>
        </tr>
        <tr>
            <td><code>verify_tls</code></td>
            <td><code>"false"</code></td>
            <td>Allows disabling TLS verification of the remote certificate.</td>
        </tr>
        <tr>
            <td><code>bearer_token</code></td>
            <td><code>"token"</code></td>
            <td>Token is passed in the standard format with header: "Authorization: bearer {token}"</td>
        </tr>
        <tr>
            <td><code>extra_filters</code></td>
            <td><code>"[]"</code></td>
            <td>List of strings with series selector. See: <a href="https://docs.victoriametrics.com/#prometheus-querying-api-enhancements">Prometheus querying API enhancements</a></td>
        </tr>
    </tbody>
</table>

Config file example:

```yaml
reader:
  class: "vm"  # or "reader.vm.VmReader" until v1.13.0
  datasource_url: "http://localhost:8428/"
  tenant_id: "0:0"
  queries:
    ingestion_rate: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
  sampling_period: '1m'
```

### Healthcheck metrics

`VmReader` exposes [several healthchecks metrics](./monitoring.html#reader-behaviour-metrics).
