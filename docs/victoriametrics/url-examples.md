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

## General information

This page contains copy-paste examples for the most commonly used VictoriaMetrics APIs.

The examples use the following placeholders for components:

* `http://<vmsingle>:8428` for single-node VictoriaMetrics
* `http://<vmselect>:8481` for cluster read APIs
* `http://<vminsert>:8480` for cluster write APIs
* `http://<vmstorage>:8482` for cluster storage maintenance APIs
* `http://<vmagent>:8429` for vmagent APIs
* `http://<vmalert>:8880` for vmalert APIs
* `http://<vmauth>:8427` for vmauth APIs

Every section includes a `> Supported by:` line, so it is easier to see which components support the endpoint.

### Multitenancy

[Multitenancy reads and writes](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy) 
are supported only by the cluster version of VictoriaMetrics.

The tenant ID can be specified in the following ways:
* Via URL path for reads and writes: `/select/<tenantID>/prometheus/api/v1/query` or `/insert/<tenantID>/prometheus/api/v1/import/prometheus`
* Via [HTTP headers](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-headers) for reads and writes: 
  `curl 'https://<vmselect>:8481/select/prometheus/api/v1/query' -d 'query=up' --header "AccountID: <tenantID>"`. Note, `--enableMultitenancyViaHeaders` must be set.
* Via [labels](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenant-writes) for writes: `curl -d 'metric_name{vm_account_id="42"} 123' -X POST 'http://<vminsert>:8480/insert/multitenant/prometheus/api/v1/import/prometheus'`.

The tenant ID [can be omitted for reads](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenant-reads) when using a special `/multitenant` endpoint.
```sh
curl 'http://<vmselect>:8481/select/multitenant/prometheus/api/v1/query' -d 'query=up'
```

In this case, VictoriaMetrics will query all available tenants and will return response with `vm_account_id` and `vm_project_id` labels
attached to each time series.

vmagent can serve as a gateway for accepting multitenant writes and forwarding them to the cluster version of VictoriaMetrics.
See more about [multitenancy in vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/#multitenancy).

> For clarity, API examples below don't demonstrate all multitenancy examples. It is expected that the user will read this section and use the examples accordingly.

## Writes

### /api/v1/import

**Import data in [JSON line format](https://docs.victoriametrics.com/victoriametrics/#how-to-import-data-in-json-line-format).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST 'http://<vmsingle>:8428/api/v1/import'
```

Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import'
```

vmagent:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST 'http://<vmagent>:8429/api/v1/import'
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [Multitenancy](#multitenancy)

### /api/v1/import/csv

**Import data in [CSV format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-csv-data).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

You must specify the desired `format`. Suppose you want to import `demo` metric exported via [/api/v1/export/csv](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportcsv).
The following command imports all time series of the `demo` metric in CSV format, including the `job` and `instance` labels.

Single-node VictoriaMetrics:
```sh
curl -X POST 'http://<vmsingle>:8428/api/v1/import/csv?format=2:label:job,3:label:instance,4:metric:demo,5:time:unix_s' -T demo.csv
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
curl -d "GOOG,1.23,4.56,NYSE" 'http://<vmsingle>:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [Multitenancy](#multitenancy)

### /api/v1/import/native

**Import data in [native format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-native-format).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

It is expected that `filename.bin` was received by exporting data via [/api/v1/export/native](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportnative).

Single-node VictoriaMetrics:

```sh
curl -X POST 'http://<vmsingle>:8428/api/v1/import/native' -T filename.bin
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/native' -T filename.bin
```

vmagent:

```sh
curl -X POST 'http://<vmagent>:8429/api/v1/import/native' -T filename.bin
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [Multitenancy](#multitenancy)

### /api/v1/import/prometheus

**Import data in [Prometheus text exposition format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST 'http://<vmsingle>:8428/api/v1/import/prometheus'
```

Cluster version of VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/prometheus'
```

vmagent:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST 'http://<vmagent>:8429/api/v1/import/prometheus'
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [Multitenancy](#multitenancy)

### /api/v1/write

**Ingest data via [Prometheus v1 remote write protocol](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Configure Prometheus or any other component that supports [Prometheus Remote Write v1 protocol](https://prometheus.io/docs/specs/prw/remote_write_spec/)
to send data to VictoriaMetrics via `/api/v1/write` path.

Single-node VictoriaMetrics:

```sh
remote_write:
  - url: http://<vmsingle>:8428/api/v1/write
```

Cluster version of VictoriaMetrics:

```sh
remote_write:
  - url: http://<vminsert>:8480/insert/0/prometheus/api/v1/write
```

vmagent:

```sh
remote_write:
  - url: http://<vmagent>:8429/api/v1/write
```

Additional information:

* [Prometheus integration](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/)
* [Multitenancy](#multitenancy)

### /datadog

**Ingest data via [Datadog agent, DogStatsD, Datadog Lambda Extension, "submit metrics" API, "sketches" API](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Configure Datadog agent to send data to VictoriaMetrics via `/datadog` path.

Single-node VictoriaMetrics:
```sh
http://<vmsingle>:8428/datadog
```

Cluster version of VictoriaMetrics:

```sh
http://<vminsert>:8480/insert/0/datadog
```

vmagent:

```sh
http://<vmagent>:8429/datadog
```

VictoriaMetrics components also support the following Datadog-specific paths:
* `/datadog/api/v1/series`
* `/datadog/api/v2/series`
* `/datadog/api/beta/sketches`

Additional information:

* [Datadog integration](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/)
* [Multitenancy](#multitenancy)

### /opentelemetry/v1/metrics

**Ingest data via [OpenTelemetry Protocol (OTLP)](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Configure OpenTelemetry Collector to send data to VictoriaMetrics using HTTP via `/opentelemetry/v1/metrics` path.

Single-node VictoriaMetrics:

```sh
exporters:
  otlphttp/victoriametrics:
    metrics_endpoint: http://<vmsingle>:8428/opentelemetry/v1/metrics
```

Cluster version of VictoriaMetrics:

```sh
exporters:
  otlphttp/victoriametrics:
    metrics_endpoint: http://<vminsert>:8480/insert/0/opentelemetry/v1/metrics
```

vmagent:

```sh
exporters:
  otlphttp/victoriametrics:
    metrics_endpoint: http://<vmagent>:8429/opentelemetry/v1/metrics
```

Additional information:

* [OpenTelemetry integration](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/)
* [OpenTelemetry Collector](https://docs.victoriametrics.com/victoriametrics/data-ingestion/opentelemetry-collector/)
* [Multitenancy](#multitenancy)

### /influx/write

**Ingest data via [InfluxDB line protocol](https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/).**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

VictoriaMetrics accepts `/write`, `/influx/write`, `/influx/api/v2/write`, and `/api/v2/write` paths for ingesting data in Influx line protocol.
The examples below use the shortest path for each component.

Single-node VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<vmsingle>:8428/write'
```

Cluster version of VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<vminsert>:8480/insert/0/influx/write'
```

vmagent:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<vmagent>:8429/write'
```

Additional information:

* [How to send Influx data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/)
* [Multitenancy](#multitenancy)

### TCP and UDP

#### How to send data from OpenTSDB-compatible agents to VictoriaMetrics

**TCP/UDP via -opentsdbListenAddr. Turned off by default.**

Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr=:4242` command-line flag.
If run from Docker, `-opentsdbListenAddr` port should be exposed. 
Multitenancy is not supported for this receiver - use `-opentsdbHTTPListenAddr` or any other protocol instead.

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Works the same for vmagent, single-node or cluster version of VictoriaMetrics:

```sh
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N '<vmsingle/vminsert/vmagent>' 4242
```

**HTTP via -opentsdbHTTPListenAddr. Turned off by default.**

> Supported by: `vmsingle`, `vminsert`, `vmagent`

For HTTP server OpenTSDB `/api/put` requests enable `-opentsdbHTTPListenAddr=:4242` command-line flag.

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' 'http://<vmsingle>:4242/api/put'
```

Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' 'http://<vminsert>:4242/insert/0/opentsdb/api/put'
```

vmagent:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' 'http://<vmagent>:4242/api/put'
```

Additional information:

* [OpenTSDB http put API](http://opentsdb.net/docs/build/html/api_http/put.html)
* [How to send OpenTSDB data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/opentsdb/)
* [Multitenancy](#multitenancy)

#### How to send Graphite data to VictoriaMetrics

**TCP/UDP via -graphiteListenAddr. Turned off by default.**

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr=:2003` command-line flag.
If run from Docker, `-graphiteListenAddr` port should be exposed.
Multitenancy is not supported for this receiver.

> Supported by: `vmsingle`, `vminsert`, `vmagent`

Works the same for vmagent, single-node or cluster version of VictoriaMetrics:

```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N '<vmsingle/vminsert/vmagent>' 2003
```

Additional information:

* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting)

## Reads

### /api/v1/export

**Exports raw samples from VictoriaMetrics in [JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-json-line-format).**

> Supported by: `vmsingle`, `vmselect`

The following command exports time series matching `vm_http_request_errors_total` metric name for the last `1d` in JSON format.

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/export' -d 'match[]=vm_http_request_errors_total' -d 'start=-1d' > filename.json
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/export' -d 'match[]=vm_http_request_errors_total' -d 'start=-1d' > filename.json
```

Additional information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [Timestamp formats](https://docs.victoriametrics.com/victoriametrics/#timestamp-formats)
* [Multitenancy](#multitenancy)

### /api/v1/export/csv

**Exports raw samples from VictoriaMetrics in [CSV format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-csv-data).**

> Supported by: `vmsingle`, `vmselect`

You must specify the desired `format` and optionally `match[]` selectors.
Suppose you have a `demo` metric with `job` and `instance` labels.
The following command exports all time series of the `demo` metric for the last `1d` in CSV format, including the `job` and `instance` labels.

Single-node VictoriaMetrics:
```sh
curl 'http://<vmsingle>:8428/api/v1/export/csv' -d 'format=__name__,job,instance,__value__,__timestamp__:unix_s' -d 'match[]=demo' -d 'start=-1d' > demo.csv
```

Cluster version of VictoriaMetrics:
```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv' -d 'format=__name__,job,instance,__value__,__timestamp__:unix_s' -d 'match[]=demo' -d 'start=-1d' > demo.csv
```

Additional information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [Timestamp formats](https://docs.victoriametrics.com/victoriametrics/#timestamp-formats)
* [Multitenancy](#multitenancy)

### /api/v1/export/native

**Exports raw samples from VictoriaMetrics in [native format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format).**

> Supported by: `vmsingle`, `vmselect`

The following command exports time series matching `vm_http_request_errors_total` metric name for the last `1d` in native binary format.

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/export/native' -d 'match[]=vm_http_request_errors_total' -d 'start=-1d' > filename.bin
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/native' -d 'match[]=vm_http_request_errors_total' -d 'start=-1d' > filename.bin
```

Additional information:

* [How to export time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
* [Timestamp formats](https://docs.victoriametrics.com/victoriametrics/#timestamp-formats)
* [Multitenancy](#multitenancy)

### /api/v1/query

**Executes an [instant query](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#instant-query).**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/query' -d 'query=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/query' -d 'query=vm_http_request_errors_total'
```

Additional information:

* [Querying data](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#metricsql)
* [Multitenancy](#multitenancy)

### /api/v1/query_range

**Executes a [range query](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#range-query).**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/query_range' -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/query_range' -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

Additional information:

* [Querying data](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#metricsql)
* [Timestamp formats](https://docs.victoriametrics.com/victoriametrics/#timestamp-formats)

### /api/v1/labels

**Get a list of label names at the given time range.**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/labels'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/labels'
```

By default, VictoriaMetrics returns labels from the last day, starting at 00:00 UTC, for performance reasons.
An arbitrary time range can be set via [`start` and `end` query args](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats).
The specified `start..end` time range is rounded to UTC day granularity for performance reasons.

Additional information:

* [Getting label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Multitenancy](#multitenancy)

### /api/v1/label/.../values

**Get a list of values for a particular label on the given time range.**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/label/job/values'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/label/job/values'
```

By default, VictoriaMetrics returns label values seen during the last day, starting at 00:00 UTC, for performance reasons.
An arbitrary time range can be set via `start` and `end` query args.
The specified `start..end` time range is rounded to UTC day granularity for performance reasons.

Additional information:

* [Querying label values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
* [Multitenancy](#multitenancy)

### /api/v1/series

**Returns series names with their labels on the given time range.**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/series' -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/series' -d 'match[]=vm_http_request_errors_total'
```

By default, VictoriaMetrics returns time series from the last day, starting at 00:00 UTC, for performance reasons.
An arbitrary time range can be set via `start` and `end` query args.
The specified `start..end` time range is rounded to UTC day granularity for performance reasons.
VictoriaMetrics accepts `limit` query arg for `/api/v1/series` handlers for limiting the number of returned entries. For example, the query to `/api/v1/series?limit=5` returns a sample of up to 5 series, while ignoring the rest. If the provided `limit` value exceeds the corresponding `-search.maxSeries` command-line flag values, then limits specified in the command-line flags are used.

Additional information:

* [Finding series by label matchers](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
* [Prometheus querying API usage](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)

### /api/v1/series/count

**Returns the total number of series.**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/series/count'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/series/count'
```

Additional information:

* [Prometheus querying API enhancements](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-enhancements)

### /api/v1/metadata

**Returns [metrics metadata](https://docs.victoriametrics.com/victoriametrics/#metrics-metadata).**

`metric` query arg can be used to filter metadata for specific metrics.
`limit` query arg can be used to limit the number of returned metadata entries.

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/metadata' -d 'metric=node_os_version'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/metadata' -d 'metric=node_os_version'
```

Additional information:

* [Metrics metadata](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#metrics-metadata)
* [Multitenancy](#multitenancy)

### /federate

**Returns federated metrics**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/federate' -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/federate' -d 'match[]=vm_http_request_errors_total'
```

Additional information:

* [Federation](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#federation)
* [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation)
* [Multitenancy](#multitenancy)

### /graphite/metrics/find

**Searches Graphite metrics in VictoriaMetrics**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/graphite/metrics/find' -d 'query=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/graphite/metrics/find' -d 'query=vm_http_request_errors_total'
```

Additional information:

* [Metrics find API in Graphite](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [Graphite API in VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#graphite-api-usage)
* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting)
* [Multitenancy](#multitenancy)

## Status

### /api/v1/status/tsdb

**Returns [TSDB stats](https://docs.victoriametrics.com/victoriametrics/#tsdb-stats).**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/prometheus/api/v1/status/tsdb'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/status/tsdb'
```

By default, the stats are returned for the current day. For the other date specify `date=YYYY-MM-DD` where `YYYY-MM-DD`
is the date for collecting the stats.

Additional information:

* [Cardinality explorer](https://docs.victoriametrics.com/victoriametrics/#cardinality-explorer)
* [Track ingested metrics usage](https://docs.victoriametrics.com/victoriametrics/#track-ingested-metrics-usage)
* [Multitenancy](#multitenancy)

### /api/v1/status/active_queries

**Returns currently executed [active queries](https://docs.victoriametrics.com/victoriametrics/#active-queries).**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/status/active_queries'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/status/active_queries'
```

Note that every vmselect maintains an independent list of active queries, which is returned in the response.

Additional information:

* [Active queries](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#active-queries)
* [Query stats](https://docs.victoriametrics.com/victoriametrics/query-stats/)
* [Prometheus querying API enhancements](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-enhancements)
* [Multitenancy](#multitenancy)

### /api/v1/status/top_queries

**Returns the [most frequently executed and the slowest queries](https://docs.victoriametrics.com/victoriametrics/#top-queries).**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/status/top_queries'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/select/0/prometheus/api/v1/status/top_queries'
```

Additional information:

* [Query stats](https://docs.victoriametrics.com/victoriametrics/query-stats/)
* [Multitenancy](#multitenancy)

### /targets

**Shows the current status of active [scrape targets](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter).**

> Supported by: `vmsingle`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/targets'
```

vmagent:

```sh
curl 'http://<vmagent>:8429/targets'
```

### /api/v1/targets

**Returns scrape target status in [Prometheus-compatible JSON format](https://prometheus.io/docs/prometheus/latest/querying/api/#targets).**

> Supported by: `vmsingle`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/targets'
```

vmagent:

```sh
curl 'http://<vmagent>:8429/api/v1/targets'
```

Additional information:

* [/targets](#targets)
* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)

### /service-discovery

**Shows [discovered targets](https://docs.victoriametrics.com/victoriametrics/sd_configs/) together with labels before and after relabeling.**

> Supported by: `vmsingle`, `vmagent`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/service-discovery'
```

vmagent:

```sh
curl 'http://<vmagent>:8429/service-discovery'
```

Additional information:

* [vmagent monitoring](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring)
* [Relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/)

### /health

**Returns component health status.**

> Supported by: `vmsingle`, `vmselect`, `vminsert`, `vmstorage`, `vmagent`, `vmalert`, `vmauth`

Returns **non-OK** response during grace period before shutting down the server.
Is supposed to be respected by load balancers for re-routing new requests to other servers.

```sh
curl 'http://<vm>:<http-port>/health'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

### /-/healthy

**Returns `VictoriaMetrics is Healthy.` if the HTTP server is healthy.**

> Supported by: `vmsingle`, `vmselect`, `vminsert`, `vmstorage`, `vmagent`, `vmalert`, `vmauth`

```sh
curl 'http://<vm>:<http-port>/-/healthy'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

### /-/ready

**Returns `VictoriaMetrics is Ready.` if the HTTP server is ready.**

> Supported by: `vmsingle`, `vmselect`, `vminsert`, `vmstorage`, `vmagent`, `vmalert`, `vmauth`

```sh
curl 'http://<vm>:<http-port>/-/ready'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

### /metrics

**Exports [service metrics](https://docs.victoriametrics.com/victoriametrics/#monitoring) in Prometheus format.**

> Supported by: `vmsingle`, `vmselect`, `vminsert`, `vmstorage`, `vmagent`, `vmalert`, `vmauth`

```sh
curl 'http://<vm>:<http-port>/metrics'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

Additional information:

* [Monitoring](https://docs.victoriametrics.com/victoriametrics/quick-start/#monitoring)

### /flags

**Returns the list of all command-line flags for the running component**

> Supported by: `vmsingle`, `vmselect`, `vminsert`, `vmstorage`, `vmagent`, `vmalert`, `vmauth`

```sh
curl 'http://<vm>:<http-port>/flags'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

Access to flags endpoint can be protected by setting auth key via command-line flag `-flagsAuthKey`.

Additional information:

* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

## Administration

### /-/reload

**Reloads configuration without restarting the server.**

> Supported by: `vmsingle`, `vmagent`, `vmalert`, `vmauth`

```sh
curl 'http://<vm>:<http-port>/-/reload'
```

Where `<vm>` is any of VictoriaMetrics services. And `<http-port>` is `-httpListenAddr` value.

Use `-reloadAuthKey` for protecting the /-/reload endpoint.

Additional information:

* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /api/v1/admin/tsdb/delete_series

**[Deletes time series](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-delete-time-series) from VictoriaMetrics.**

> Supported by: `vmsingle`, `vmselect`

Note that the handler accepts any HTTP method, so sending a `GET` request to `/api/v1/admin/tsdb/delete_series` will delete the time series.

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/api/v1/admin/tsdb/delete_series' -d 'match[]=vm_http_request_errors_total'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series' -d 'match[]=vm_http_request_errors_total'
```

Use `-deleteAuthKey` command-line flag for protecting the delete endpoint.

Additional information:

* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /admin/tenants

**Lists registered tenants in a VictoriaMetrics cluster.**

> Supported by: `vmselect`

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/admin/tenants'
```

The optional `start` and `end` query args can be used to return only tenants with ingested data in the given time range:

```sh
curl 'http://<vmselect>:8481/admin/tenants?start=-1d&end=now'
```

Additional information:

* [Multitenancy in cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)

### /internal/resetRollupResultCache

**Resets the response cache for previously served queries. It is recommended to invoke after [backfilling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling) procedure.**

> Supported by: `vmsingle`, `vmselect`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/internal/resetRollupResultCache'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmselect>:8481/internal/resetRollupResultCache?propagate=1'
```

vmselect will propagate this call to the rest of the vmselects listed in its `-selectNode` cmd-line flag when `propagate=1` argument is set.
If this flag or the `propagate` argument isn't set, then the cache needs to be purged from each vmselect individually.

If `-search.resetCacheAuthKey` is set, it will be attached to the propagation request as query argument.

Additional information:

* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /internal/force_flush

**Flushes the recently ingested samples from in-memory buffers to persistent storage, so they become visible for querying.**

> Supported by: `vmsingle`, `vmstorage`

This handler is mostly needed for testing and debugging purposes.

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/internal/force_flush'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/internal/force_flush'
```

Use `-forceFlushAuthKey` for protecting the /internal/force_flush endpoint.

Additional information:

* [Forced flush](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-latency)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /internal/force_merge

**Starts forced compaction**

> Supported by: `vmsingle`, `vmstorage`

The `partition_prefix` query arg is used for matching a subset of partitions.

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/internal/force_merge?partition_prefix=2025_'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/internal/force_merge?partition_prefix=2025_'
```

Use `-forceMergeAuthKey` for protecting the `/internal/force_merge` endpoint.

Additional information:

* [Forced merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /snapshot/create

**Creates a snapshot**

> Supported by: `vmsingle`, `vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/snapshot/create'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/create'
```

Use `-snapshotAuthKey` for protecting the `/snapshot*` endpoints.

Additional information:

* [How to work with snapshots](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots)
* [vmbackup](https://docs.victoriametrics.com/victoriametrics/vmbackup/)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /snapshot/list

**Lists existing snapshots**

> Supported by: `vmsingle`, `vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/snapshot/list?authKey=<snapshot-auth-key>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/list?authKey=<snapshot-auth-key>'
```

Use `-snapshotAuthKey` for protecting the `/snapshot*` endpoints.

Additional information:

* [/snapshot/create](#snapshotcreate)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /snapshot/delete

**Deletes the selected snapshot**

> Supported by: `vmsingle`, `vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/snapshot/delete?snapshot=<snapshot-name>'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/delete?snapshot=<snapshot-name>'
```

Use `-snapshotAuthKey` for protecting the `/snapshot*` endpoints.

Additional information:

* [/snapshot/create](#snapshotcreate)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

### /snapshot/delete_all

**Deletes all snapshots**

> Supported by: `vmsingle`, `vmstorage`

Single-node VictoriaMetrics:

```sh
curl 'http://<vmsingle>:8428/snapshot/delete_all'
```

Cluster version of VictoriaMetrics:

```sh
curl 'http://<vmstorage>:8482/snapshot/delete_all'
```

Use `-snapshotAuthKey` for protecting the `/snapshot*` endpoints.

Additional information:

* [/snapshot/create](#snapshotcreate)
* [Protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/#protecting-service-endpoints)

## Alerting

Alerting is supported by [vmalert](https://github.com/VictoriaMetrics/vmalert).

### /api/v1/rules

**Returns loaded groups and rules.**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/rules'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/alerts

**Returns list of active alerts**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/alerts'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/alert

**Returns alert status in JSON format.**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/alert?group_id=<group_id>&alert_id=<alert_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/rule

**Returns rule status in JSON format**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/rule?group_id=<group_id>&rule_id=<rule_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/group

**Returns group status in JSON format**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/group?group_id=<group_id>'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

### /api/v1/notifiers

**Returns configured vmalert notifiers**

> Supported by: `vmalert`; also reachable via `vmsingle` and `vmselect` when `-vmalert.proxyURL` is configured.

```sh
curl 'http://<vmalert>:8880/api/v1/notifiers'
```

Additional information:

* [vmalert web API](https://docs.victoriametrics.com/victoriametrics/vmalert/#web)
* [vmalert proxying through cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmalert)

---

Section below contains backward-compatible anchors for links that were moved or renamed.

###### /datadog/api/v1/series

Moved to [/datadog](https://docs.victoriametrics.com/victoriametrics/url-examples/#datadog).

###### /datadog/api/v2/series

Moved to [/datadog](https://docs.victoriametrics.com/victoriametrics/url-examples/#datadog).