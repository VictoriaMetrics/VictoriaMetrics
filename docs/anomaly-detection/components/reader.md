---
# sort: 2
title: Reader
weight: 2
menu:
  docs:
    parent: "vmanomaly-components"
    # sort: 2
    weight: 2
aliases:
  - /anomaly-detection/components/reader.html
---

# Reader

<!--
There are 4 sources available to read data into VM Anomaly Detection from: VictoriaMetrics, (ND)JSON file, QueryRange, or CSV file. Depending on the data source, different parameters should be specified in the config file in the `reader` section.
-->

VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily uses [VmReader](#vm-reader) to ingest data. This reader focuses on fetching time-series data directly from VictoriaMetrics with the help of powerful [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) expressions for aggregating, filtering and grouping your data, ensuring seamless integration and efficient data handling. 

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
            <td><code>"reader.vm.VmReader"</code></td>
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
            <td>For cluster version only, tenants are identified by accountID or accountID:projectID</td>
        </tr>
        <tr>
            <td><code>sampling_period</code></td>
            <td><code>"1h"</code></td>
            <td>Optional. Frequency of the points returned. Will be converted to <code>"/query_range?step=%s"</code> param (in seconds).</td>
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
  class: "reader.vm.VmReader"
  datasource_url: "http://localhost:8428/"
  tenant_id: "0:0"
  queries:
    ingestion_rate: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
  sampling_period: '1m'
```

### Healthcheck metrics

`VmReader` exposes [several healthchecks metrics](./monitoring.html#reader-behaviour-metrics).

<!--

# TODO: uncomment and maintain after multimodel config refactor, 2nd priority

## NDJSON reader
Accepts data in the same format as <code>/export</code>. 

File content example:
```
{"metric":{"__name__":"metric1","job":"vm"},"values":[745487.56,96334.13,277822.84,159596.94],"timestamps":[1640908800000,1640908802000,1640908803000,1640908804000]}
{"metric":{"__name__":"metric2","job":"vm"},"values":[217822.84,159596.94,745487.56,96334.13],"timestamps":[1640908800000,1640908802000,1640908803000,1640908804000]}

```
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
            <td><code>"reader.ndjson.NdjsonReader"</code></td>
            <td>Name of the class needed to enable reading from JSON line format file.</td>
        </tr>
        <tr>
            <td><code>path</code></td>
            <td><code>"tests/reader/export.ndjson"</code></td>
            <td>Path to file in JSON line format</td>
        </tr>
    </tbody>
</table>

Config file example:
```yaml
reader:
  class: "reader.ndjson.NdjsonReader"
  path: "tests/reader/export.ndjson"
```


## QueryRange
This datasource is VictoriaMetrics handler for [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/).

[Range query](https://docs.victoriametrics.com/keyConcepts.html#range-query) executes the query expression at the given time range with the given step.

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
            <td><code>"reader.query_range.QueryRangeReader"</code></td>
            <td>Name of the class enabling Query Range reader.</td>
        </tr>
        <tr>
            <td><code>path</code></td>
            <td><code>"http://localhost:8428/api/v1/query_range?query=sum(rate(vm_rows_inserted_total[30])) by (type)"</code></td>
            <td>URL with query</td>
        </tr>
    </tbody>
</table>

Config file example:
```yaml
reader:
  class: "reader.query_range.QueryRangeReader"
  path: "http://localhost:8428/api/v1/query_range?query=sum(rate(vm_rows_inserted_total[30])) by (type)"
```


## CSV reader
### Data format
File should be in `.csv` format and must contain 2 columns with the names: `timestamp` and `y` - metric's datetimes and values accordinally. Order of the columns doesn't matter.

* `timestamp` can be represented eather in explicit datetime format like `2021-04-21 05:18:19` or in UNIX time in seconds like `1618982299`.

* `y` should be a numeric value. 


File content example:
```
timestamp,y
2020-07-12 23:09:05,61.0
2020-07-13 23:09:05,63.0
2020-07-14 23:09:05,63.0
2020-07-15 23:09:05,66.0
2020-07-20 23:09:05,68.0
2020-07-21 23:09:05,69.0
2020-07-22 23:09:05,69.0
```

### Config parameters
<table>
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td><code>class</code></td>
            <td>str</td>
            <td><code>"reader.csv.CsvReader"</code></td>
            <td>Name of the class enabling CSV reader</td>
        </tr>
        <tr>
            <td><code>path</code></td>
            <td>str</td>
            <td><code>"data/v1/jumpsup.csv"</code></td>
            <td>.csv file location (local path). <b>The file existence is checked during config validation</b></td>
        </tr>
        <tr>
            <td><code>metric_name</code></td>
            <td>str</td>
            <td><code>"value"</code></td>
            <td>Optional. Alias for metric. If not specified, filename without extension will be used. In this example, `jumpsup`.</td>
        </tr>
    </tbody>
</table>
Config file example:

```yaml
reader:
  class: "reader.csv.CsvReader"
  path: "data/v1/jumpsup.csv"
  metric_name: "value"
```
-->