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

VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily uses [VmReader](#vm-reader) to ingest data. This reader focuses on fetching time-series data directly from VictoriaMetrics with the help of powerful [MetricsQL](../../MetricsQL.md) expressions for aggregating, filtering and grouping your data, ensuring seamless integration and efficient data handling.

Future updates will introduce additional readers, expanding the range of data sources `vmanomaly` can work with.


## VM reader

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

`reader.vm.VmReader` (or `vm` starting from [v1.13.0](../CHANGELOG.md#v1130))
            </td>
            <td>

Name of the class needed to enable reading from VictoriaMetrics or Prometheus. VmReader is the default option, if not specified.
            </td>
        </tr>
        <tr>
            <td>

`queries`
            </td>
            <td>

`ingestion_rate: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'`
            </td>
            <td>

PromQL/MetricsQL query to select data in format: `QUERY_ALIAS: "QUERY"`. As accepted by `/query_range?query=%s`.
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

For VictoriaMetrics Cluster version only, tenants are identified by accountID or accountID:projectID. See VictoriaMetrics Cluster [multitenancy docs](../../Cluster-VictoriaMetrics.md#multitenancy)
            </td>
        </tr>
        <tr>
            <td>

`sampling_period`
            </td>
            <td>

`1h`
            </td>
            <td>

Frequency of the points returned. Will be converted to `/query_range?step=%s` param (in seconds). **Required** since [v1.9.0](../CHANGELOG.md#v190).
            </td>
        </tr>
        <tr>
            <td>

`query_range_path`
            </td>
            <td>

`/api/v1/query_range`
            </td>
            <td>

Performs PromQL/MetricsQL range query
            </td>
        </tr>
        <tr>
            <td>

`health_path`
            </td>
            <td>

`health`
            </td>
            <td>

Absolute or relative URL address where to check availability of the datasource.
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

`30s`
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

Token is passed in the standard format with header: `Authorization: bearer {token}`
            </td>
        </tr>
        <tr>
            <td>

`extra_filters`
            </td>
            <td>

`[]`
            </td>
            <td>

List of strings with series selector. See: [Prometheus querying API enhancements](../../README.md##prometheus-querying-api-enhancements)
            </td>
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

`VmReader` exposes [several healthchecks metrics](./monitoring.md#reader-behaviour-metrics).
