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
<!--
There are 4 sources available to read data into VM Anomaly Detection from: VictoriaMetrics, (ND)JSON file, QueryRange, or CSV file. Depending on the data source, different parameters should be specified in the config file in the `reader` section.
-->

VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily uses [VmReader](#vm-reader) to ingest data. This reader focuses on fetching time-series data directly from VictoriaMetrics with the help of powerful [MetricsQL](../../MetricsQL.md) expressions for aggregating, filtering and grouping your data, ensuring seamless integration and efficient data handling.

Future updates will introduce additional readers, expanding the range of data sources `vmanomaly` can work with.


## VM reader

> **Note**: Starting from [v1.13.0](/anomaly-detection/changelog#v1130) there is backward-compatible change of [`queries`](/anomaly-detection/components/reader?highlight=queries#vm-reader) arg of [VmReader](#vm-reader). New format allows to specify per-query parameters, like `step` to reduce amount of data read from VictoriaMetrics TSDB and to allow config flexibility. Please see [per-query parameters](#per-query-parameters) section for the details.

Old format like

```yaml
# other config sections ...
reader:
  class: 'vm'
  datasource_url: 'http://localhost:8428'  # source victoriametrics/prometheus
  sampling_period: "10s"  # set it <= min(infer_every) in schedulers section
  queries:
    # old format {query_alias: query_expr}, prior to 1.13, will be converted to a new format automatically
    vmb: 'avg(vm_blocks)'
```

will be converted to a new one with a warning raised in logs:

```yaml
# other config sections ...
reader:
  class: 'vm'
  datasource_url: 'http://localhost:8428'  # source victoriametrics/prometheus
  sampling_period: '10s'
  queries:
    # old format {query_alias: query_expr}, prior to 1.13, will be converted to a new format automatically
    vmb:
      expr: 'avg(vm_blocks)'  # initial MetricsQL expression
      step: '10s'  # individual step for this query, will be filled with `sampling_period` from the root level
      # new query-level arguments will be added in backward-compatible way in future releases
```

### Per-query parameters

Starting from [v1.13.0](/anomaly-detection/changelog#v1130) there is change of [`queries`](/anomaly-detection/components/reader?highlight=queries#vm-reader) arg format. Now each query alias supports the next (sub)fields:

- `expr` (string): MetricsQL/PromQL expression that defines an input for VmReader. As accepted by `/query_range?query=%s`. i.e. `avg(vm_blocks)`

- `step` (string): query-level frequency of the points returned, i.e. `30s`. Will be converted to `/query_range?step=%s` param (in seconds). Useful to optimize total amount of data read from VictoriaMetrics, where different queries may have **different frequencies for different [machine learning models](/anomaly-detection/components/models)** to run on.

    > **Note**: if not set explicitly (or if older config style prior to [v1.13.0](/anomaly-detection/changelog#v1130)) is used, then it is set to reader-level `sampling_period` arg.

    > **Note**: having **different** individual `step` args for queries (i.e. `30s` for `q1` and `2m` for `q2`) is not yet supported for [multivariate model](/anomaly-detection/components/models/index.html#multivariate-models) if you want to run it on several queries simultaneously (i.e. setting [`queries`](/anomaly-detection/components/models/#queries) arg of a model to [`q1`, `q2`]).

### Per-query config example
```yaml
reader:
  class: 'vm'
  sampling_period: '1m'
  # other reader params ...
  queries:
    ingestion_rate:
      expr: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
      step: '2m'  # overrides global `sampling_period` of 1m
```

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
                See [per-query config example](#per-query-config-example) above
            </td>

<td>

See [per-query config section](#per-query-parameters) above
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
