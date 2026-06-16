---
weight: 33
title: API examples
menu:
  docs:
    parent: 'victoriametrics'
    weight: 33
    identifier: vm-api-examples
tags:
  - metrics
  - guide
aliases:
  - /url-examples.html
  - /url-examples/index.html
  - /url-examples/
---
This page contains copy-paste examples for the most commonly used VictoriaMetrics APIs.

The examples below use these placeholders:

- `http://localhost:8428` for single-node VictoriaMetrics
- `http://<vmselect>:8481` for cluster read APIs
- `http://<vminsert>:8480` for cluster write APIs
- `http://<vmstorage>:8482` for cluster storage maintenance APIs
- `http://<vmagent>:8429` for vmagent APIs
- `http://<vmalert-addr>:8880` for vmalert APIs
- `http://<vmauth-addr>:8427` for vmauth APIs

Every section includes a `Supported by:` line so it is easier to see where the endpoint is available.

### /api/v1/admin/tsdb/delete_series

**Deletes time series from VictoriaMetrics**

Supported by: `single-node`, `cluster`

Note that handler accepts any HTTP method, so sending a `GET` request to `/api/v1/admin/tsdb/delete_series` will result in deletion of time series.

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```

Expected response: [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#page-53).

Additional information:

* [How to delete time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-delete-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/export

**Exports raw samples from VictoriaMetrics in JSON line format**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```

Additional information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export data in JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-json-line-format)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/export/csv

**Exports raw samples from VictoriaMetrics in CSV format**

Supported by: `single-node`, `cluster`

You must specify the desired `format` and optionally `match[]` selectors.
Suppose you have a `demo` metric with `job` and `instance` labels.
The following command exports all time series of the `demo` metric in CSV format including the `job` and `instance` labels.

Single-node VictoriaMetrics:
```sh
curl http://localhost:8428/api/v1/export/csv -d 'format=__name__,job,instance,__value__,__timestamp__:unix_s' -d 'match[]=demo' > demo.csv
```

Cluster version of VictoriaMetrics:
```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv -d 'format=__name__,job,instance,__value__,__timestamp__:unix_s' -d 'match[]=demo' > demo.csv
```

More information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/export/native

**Exports raw samples from VictoriaMetrics in native format**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```

Additional information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/import

**Imports data to VictoriaMetrics in JSON line format**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST http://localhost:8428/api/v1/import
```

Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import
```

vmagent:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST http://<vmagent>:8429/api/v1/import
```

Expected response: HTTP 204 No Content.

More information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/import/csv

**Imports CSV data to VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

You must specify the desired `format`. Suppose you want to import `demo` metric exported with [/api/v1/export/csv](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportcsv).
The following command imports all time series of the `demo` metric in CSV format including the `job` and `instance` labels.

Single-node VictoriaMetrics:
```sh
curl -X POST 'http://localhost:8428/api/v1/import/csv?format=2:label:job,3:label:instance,4:metric:demo,5:time:unix_s' -T demo.csv
```

Cluster version of VictoriaMetrics:
```sh
curl -X POST 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv?format=2:label:job,3:label:instance,4:metric:demo,5:time:unix_s' -T demo.csv
```

vmagent:
```sh
curl -X POST 'http://<vmagent>:8429/api/v1/import/csv?format=2:label:job,3:label:instance,4:metric:demo,5:time:unix_s' -T demo.csv
```

A single CSV line can contain multiple metrics. For example, this command imports two metrics `ask{ticker="GOOG",market="NYSE"} 1.23` and `bid{ticker="GOOG",market="NYSE"} 4.56`:
```
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

Expected response: HTTP 204 No Content.

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/import/native

**Imports data to VictoriaMetrics in native format**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -X POST http://localhost:8428/api/v1/import/native -T filename.bin
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/native -T filename.bin
```

vmagent:

```sh
curl -X POST http://<vmagent>:8429/api/v1/import/native -T filename.bin
```

Expected response: HTTP 204 No Content.

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/import/prometheus

**Imports data to VictoriaMetrics in Prometheus text exposition format**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST http://localhost:8428/api/v1/import/prometheus
```

Cluster version of VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/prometheus
```

vmagent:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST http://<vmagent>:8429/api/v1/import/prometheus
```

Pushgateway-compatible paths under `/api/v1/import/prometheus/metrics/job/...` return HTTP 200 OK. Other paths return HTTP 204 No Content.

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/write

**Ingests data via Prometheus remote write protocol**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -X POST http://localhost:8428/api/v1/write --data-binary @request.bin
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/write --data-binary @request.bin
```

vmagent:

```sh
curl -X POST http://<vmagent>:8429/api/v1/write --data-binary @request.bin
```

Expected response: HTTP 204 No Content.

Additional information:

* [Prometheus integration](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/)
* [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/labels

**Get a list of label names at the given time range**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/labels
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/labels
```

By default, VictoriaMetrics returns labels seen during the last day starting at 00:00 UTC because of performance reasons.
An arbitrary time range can be set via [`start` and `end` query args](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats).
The specified `start..end` time range is rounded to UTC day granularity because of performance reasons.

Additional information:

* [Getting label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/label/.../values

**Get a list of values for a particular label on the given time range**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/label/job/values
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/label/job/values
```

By default, VictoriaMetrics returns labels values seen during the last day starting at 00:00 UTC because of performance reasons.
An arbitrary time range can be set via `start` and `end` query args.
The specified `start..end` time range is rounded to UTC day granularity because of performance reasons.

Additional information:

* [Querying label values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/query

**Performs PromQL/MetricsQL instant query**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```

Additional information:

* [Instant queries](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#instant-query)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/query_range

**Performs PromQL/MetricsQL range query**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

Additional information:

* [Range queries](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#range-query)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/query_exemplars

**Returns exemplars for the selected query and time range**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/query_exemplars -d 'query=vm_http_request_duration_seconds_bucket' -d 'start=-1h' -d 'end=now'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query_exemplars -d 'query=vm_http_request_duration_seconds_bucket' -d 'start=-1h' -d 'end=now'
```

Additional information:

* [Prometheus exemplars API](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-exemplars)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/series

**Returns series names with their labels on the given time range**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/series -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/series -d 'match[]=vm_http_request_errors_total'
```

By default, VictoriaMetrics returns time series seen during the last day starting at 00:00 UTC because of performance reasons.
An arbitrary time range can be set via `start` and `end` query args.
The specified `start..end` time range is rounded to UTC day granularity because of performance reasons.
VictoriaMetrics accepts `limit` query arg for `/api/v1/series` handlers for limiting the number of returned entries. For example, the query to `/api/v1/series?limit=5` returns a sample of up to 5 series, while ignoring the rest. If the provided `limit` value exceeds the corresponding `-search.maxSeries` command-line flag values, then limits specified in the command-line flags are used.

Additional information:

* [Finding series by label matchers](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/series/count

**Returns the total number of series**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/series/count
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/series/count
```

Additional information:

* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/status/tsdb

**Cardinality statistics**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/status/tsdb
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/status/tsdb
```

Additional information:

* [TSDB Stats](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/status/active_queries

**Returns currently executed active queries**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/status/active_queries
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/status/active_queries
```

Additional information:

* [Query stats](https://docs.victoriametrics.com/victoriametrics/query-stats/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/status/top_queries

**Returns the most frequently executed and the slowest queries**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/status/top_queries
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/status/top_queries
```

Additional information:

* [Query stats](https://docs.victoriametrics.com/victoriametrics/query-stats/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/status/buildinfo

**Returns build information**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/status/buildinfo
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/status/buildinfo
```

Additional information:

* [Prometheus buildinfo API](https://prometheus.io/docs/prometheus/latest/querying/api/#build-information)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /api/v1/metadata

**Returns stored metrics metadata**.
`metric` query arg can be used to filter metadata for specific metrics.
`limit` query arg can be used to limit the number of returned metadata entries.

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/metadata
```

Cluster version of VictoriaMetrics:

```sh
curl -X GET http://<vmselect>:8481/select/0/prometheus/api/v1/metadata
```

vmagent:

```sh
curl http://<vmagent>:8429/api/v1/metadata
```

Additional information:

* [Single-node - Metrics Metadata](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#metric-metadata)
* [Cluster - Metrics Metadata](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#metric-metadata)
* [VMAgent - Metrics Metadata](https://docs.victoriametrics.com/victoriametrics/vmagent/#metric-metadata)

### /admin/tenants

**Lists registered tenants in a VictoriaMetrics cluster**

Supported by: `cluster`

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/admin/tenants
```

The optional `start` and `end` query args can be used to return only tenants with ingested data in the given time range:

```sh
curl 'http://<vmselect>:8481/admin/tenants?start=-1d&end=now'
```

Additional information:

* [Multitenancy in cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /datadog

**DataDog URL for Single-node VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

```
http://victoriametrics:8428/datadog
```

**DataDog URL for Cluster version of VictoriaMetrics**

```
http://vminsert:8480/insert/0/datadog
```

**DataDog URL for vmagent**

```
http://vmagent:8429/datadog
```

### /datadog/api/v1/series

**Imports data in DataDog v1 format into VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
echo '{"series":[{"host":"test.example.com","interval":20,"metric":"system.load.1","points":[[0,0.5]],"tags":["environment:test"],"type":"rate"}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://localhost:8428/datadog/api/v1/series
```

Cluster version of VictoriaMetrics:

```sh
echo '{"series":[{"host":"test.example.com","interval":20,"metric":"system.load.1","points":[[0,0.5]],"tags":["environment:test"],"type":"rate"}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- 'http://<vminsert>:8480/insert/0/datadog/api/v1/series'
```

vmagent:

```sh
echo '{"series":[{"host":"test.example.com","interval":20,"metric":"system.load.1","points":[[0,0.5]],"tags":["environment:test"],"type":"rate"}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://<vmagent>:8429/datadog/api/v1/series
```

Expected response: HTTP 202 Accepted.

Additional information:

* [How to send data from DataDog agent](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /datadog/api/v2/series

**Imports data in [DataDog v2](https://docs.datadoghq.com/api/latest/metrics/#submit-metrics) format into VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
echo '{"series":[{"metric":"system.load.1","type":0,"points":[{"timestamp":0,"value":0.7}],"resources":[{"name":"dummyhost","type":"host"}],"tags":["environment:test"]}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://localhost:8428/datadog/api/v2/series
```

Cluster version of VictoriaMetrics:

```sh
echo '{"series":[{"metric":"system.load.1","type":0,"points":[{"timestamp":0,"value":0.7}],"resources":[{"name":"dummyhost","type":"host"}],"tags":["environment:test"]}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- 'http://<vminsert>:8480/insert/0/datadog/api/v2/series'
```

vmagent:

```sh
echo '{"series":[{"metric":"system.load.1","type":0,"points":[{"timestamp":0,"value":0.7}],"resources":[{"name":"dummyhost","type":"host"}],"tags":["environment:test"]}]}' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://<vmagent>:8429/datadog/api/v2/series
```

Expected response: HTTP 202 Accepted.

Additional information:

* [How to send data from DataDog agent](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /datadog/api/beta/sketches

**Imports data in DataDog sketches format into VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -X POST -H 'Content-Type: application/json' --data-binary @sketches.json http://localhost:8428/datadog/api/beta/sketches
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST -H 'Content-Type: application/json' --data-binary @sketches.json http://<vminsert>:8480/insert/0/datadog/api/beta/sketches
```

vmagent:

```sh
curl -X POST -H 'Content-Type: application/json' --data-binary @sketches.json http://<vmagent>:8429/datadog/api/beta/sketches
```

Expected response: HTTP 202 Accepted.

Additional information:

* [How to send data from DataDog agent](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /opentelemetry/v1/metrics

**Ingests metrics via OpenTelemetry Protocol (OTLP)**

Supported by: `single-node`, `cluster`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -X POST -H 'Content-Type: application/x-protobuf' --data-binary @metrics.pb http://localhost:8428/opentelemetry/v1/metrics
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST -H 'Content-Type: application/x-protobuf' --data-binary @metrics.pb http://<vminsert>:8480/insert/0/opentelemetry/v1/metrics
```

vmagent:

```sh
curl -X POST -H 'Content-Type: application/x-protobuf' --data-binary @metrics.pb http://<vmagent>:8429/opentelemetry/v1/metrics
```

If request body is gzip-compressed, add `Content-Encoding: gzip`.

Additional information:

* [OpenTelemetry integration](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/)
* [OpenTelemetry Collector](https://docs.victoriametrics.com/victoriametrics/data-ingestion/opentelemetry-collector/)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /federate

**Returns federated metrics**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/federate -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/federate -d 'match[]=vm_http_request_errors_total'
```

Additional information:

* [Federation](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#federation)
* [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /graphite/metrics/find

**Searches Graphite metrics in VictoriaMetrics**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/graphite/metrics/find -d 'query=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/graphite/metrics/find -d 'query=vm_http_request_errors_total'
```

Additional information:

* [Metrics find API in Graphite](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [Graphite API in VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#graphite-api-usage)
* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting)
* [URL Format](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /influx/write

**Writes data with InfluxDB line protocol to VictoriaMetrics**

Supported by: `single-node`, `cluster`, `vmagent`

VictoriaMetrics also accepts `/write`, `/influx/api/v2/write` and `/api/v2/write` paths. The examples below use the shortest path for each component.

Single-node VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://localhost:8428/write
```

Cluster version of VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://<vminsert>:8480/insert/0/influx/write
```

vmagent:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://<vmagent>:8429/write
```

Expected response: HTTP 204 No Content.

Additional information:

* [How to send Influx data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/)
* [URL Format](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format)

### /internal/resetRollupResultCache

**Resets the response cache for previously served queries. It is recommended to invoke after [backfilling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling) procedure.**

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
curl -Is http://localhost:8428/internal/resetRollupResultCache
```

Cluster version of VictoriaMetrics:

```sh
curl -Is http://<vmselect>:8481/internal/resetRollupResultCache?propagate=1
```

vmselect will propagate this call to the rest of the vmselects listed in its `-selectNode` cmd-line flag when `propagate=1` argument is set.
If this flag or the `propagate` argument isn't set, then cache need to be purged from each vmselect individually.

If `-search.resetCacheAuthKey` is set, it will be attached to the propagation request as query argument.

### /internal/force_flush

**Flushes the recently ingested samples from in-memory buffers to persistent storage, so they become visible for querying**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/internal/force_flush?authKey=<force-flush-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/internal/force_flush?authKey=<force-flush-auth-key>'
```

Additional information:

* [Forced flush](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-latency)

### /internal/force_merge

**Starts forced compaction**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/internal/force_merge?authKey=<force-merge-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/internal/force_merge?authKey=<force-merge-auth-key>'
```

The `partition_prefix` query arg can be used for limiting the merge to selected partitions:

```sh
curl 'http://<vmstorage>:8482/internal/force_merge?authKey=<force-merge-auth-key>&partition_prefix=2025_'
```

Additional information:

* [Forced merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge)

### /snapshot/create

**Creates a snapshot**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/snapshot/create?authKey=<snapshot-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/create?authKey=<snapshot-auth-key>'
```

Additional information:

* [How to work with snapshots](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots)
* [vmbackup](https://docs.victoriametrics.com/victoriametrics/vmbackup/)

### /snapshot/list

**Lists existing snapshots**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/snapshot/list?authKey=<snapshot-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/list?authKey=<snapshot-auth-key>'
```

### /snapshot/delete

**Deletes the selected snapshot**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/snapshot/delete?authKey=<snapshot-auth-key>&snapshot=<snapshot-name>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/delete?authKey=<snapshot-auth-key>&snapshot=<snapshot-name>'
```

### /snapshot/delete_all

**Deletes all snapshots**

Supported by: `single-node`, `cluster via vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://localhost:8428/snapshot/delete_all?authKey=<snapshot-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/delete_all?authKey=<snapshot-auth-key>'
```

### /targets

**Shows the current status of active scrape targets**

Supported by: `single-node`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/targets
```

vmagent:

```sh
curl http://<vmagent>:8429/targets
```

### /service-discovery

**Shows discovered targets together with labels before and after relabeling**

Supported by: `single-node`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/service-discovery
```

vmagent:

```sh
curl http://<vmagent>:8429/service-discovery
```

Additional information:

* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)
* [Relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/)

### /api/v1/targets

**Returns scrape target status in Prometheus-compatible JSON format**

Supported by: `single-node`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/targets
```

vmagent:

```sh
curl http://<vmagent>:8429/api/v1/targets
```

Additional information:

* [Prometheus targets API](https://prometheus.io/docs/prometheus/latest/querying/api/#targets)
* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)

### /api/v1/rules

**Returns loaded groups and rules from vmalert**

Supported by: `vmalert`; also reachable via `single-node` and `cluster` when `-vmalert.proxyURL` is configured

```sh
curl http://<vmalert-addr>:8880/api/v1/rules
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/alerts

**Returns active alerts from vmalert**

Supported by: `vmalert`; also reachable via `single-node` and `cluster` when `-vmalert.proxyURL` is configured

```sh
curl http://<vmalert-addr>:8880/api/v1/alerts
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /vmalert/api/v1/alert

**Returns alert status in JSON format**

Supported by: `vmalert`

```sh
curl 'http://<vmalert-addr>:8880/vmalert/api/v1/alert?group_id=<group_id>&alert_id=<alert_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)

### /vmalert/api/v1/rule

**Returns rule status in JSON format**

Supported by: `vmalert`

```sh
curl 'http://<vmalert-addr>:8880/vmalert/api/v1/rule?group_id=<group_id>&rule_id=<rule_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)

### /vmalert/api/v1/group

**Returns group status in JSON format**

Supported by: `vmalert`

```sh
curl 'http://<vmalert-addr>:8880/vmalert/api/v1/group?group_id=<group_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)

### /api/v1/notifiers

**Returns configured vmalert notifiers**

Supported by: `vmalert`; also reachable via `single-node` and `cluster` when `-vmalert.proxyURL` is configured

```sh
curl http://<vmalert-addr>:8880/api/v1/notifiers
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /ready

**Returns readiness status for vmagent scrape initialization**

Supported by: `vmagent`

vmagent:

```sh
curl http://<vmagent>:8429/ready
```

Additional information:

* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)

### /-/ready

**Returns Prometheus-compatible readiness status**

Supported by: `single-node`, `cluster via vmselect`

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/-/ready
```

Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/-/ready
```

### /-/reload

**Reloads configuration**

Supported by: `vmagent`, `vmalert`, `vmauth`

vmagent:

```sh
curl 'http://<vmagent>:8429/-/reload?authKey=<reload-auth-key>'
```

vmalert:

```sh
curl 'http://<vmalert-addr>:8880/-/reload?authKey=<reload-auth-key>'
```

vmauth:

```sh
curl 'http://<vmauth-addr>:8427/-/reload?authKey=<reload-auth-key>'
```

This endpoint is commonly protected with `-reloadAuthKey`.

### /metrics

**Exports Prometheus-format metrics for the running component**

Supported by: `vmagent`, `vmalert`, `vmauth`

vmagent:

```sh
curl http://<vmagent>:8429/metrics
```

vmalert:

```sh
curl http://<vmalert-addr>:8880/metrics
```

vmauth:

```sh
curl http://<vmauth-addr>:8427/metrics
```

Additional information:

* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)
* [vmalert web and monitoring](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmauth monitoring](https://docs.victoriametrics.com/victoriametrics/vmauth/#monitoring)

### TCP and UDP

#### How to send data from OpenTSDB-compatible agents to VictoriaMetrics

Turned off by default. Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command-line flag.
*If run from docker, '-opentsdbListenAddr' port should be exposed*

Supported by: `single-node`, `cluster`

Single-node VictoriaMetrics:

```sh
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

Cluster version of VictoriaMetrics:

```sh
echo "put foo.bar.baz `date +%s` 123  tag1=value1 tag2=value2" | nc -N <vminsert> 4242
```

Enable HTTP server for OpenTSDB /api/put requests by setting `-opentsdbHTTPListenAddr` command-line flag.

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```

Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://<vminsert>:4242/insert/42/opentsdb/api/put
```

Additional information:

* [OpenTSDB http put API](http://opentsdb.net/docs/build/html/api_http/put.html)
* [How to send data OpenTSDB data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/opentsdb/)

#### How to send Graphite data to VictoriaMetrics

Supported by: `single-node`, `cluster`

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command-line flag.

Single-node VictoriaMetrics:

```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```

Cluster version of VictoriaMetrics:

```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N <vminsert> 2003
```

Additional information:

* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting)
* [Multitenancy in cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
