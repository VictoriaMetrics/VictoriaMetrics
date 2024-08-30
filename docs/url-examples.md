---
weight: 33
title: API examples
menu:
  docs:
    parent: 'victoriametrics'
    weight: 33
    identifier: vm-api-examples

aliases:
  - /url-examples.html
---
### /api/v1/admin/tsdb/delete_series

**Deletes time series from VictoriaMetrics**

Note that handler accepts any HTTP method, so sending a `GET` request to `/api/v1/admin/tsdb/delete_series` will result in deletion of time series.

Single-node VictoriaMetrics:

```sh
curl -v http://localhost:8428/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```


The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#page-53) and will look like:


```sh
*   Trying 127.0.0.1:8428...
* Connected to 127.0.0.1 (127.0.0.1) port 8428 (#0)
> GET /api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total HTTP/1.1
> Host: 127.0.0.1:8428
> User-Agent: curl/7.81.0
> Accept: */*
>
* Mark bundle as not supporting multiuse
< HTTP/1.1 204 No Content
< X-Server-Hostname: eba075fb0e1a
< Date: Tue, 21 Jun 2022 07:33:35 GMT
<
* Connection #0 to host 127.0.0.1 left intact
```


Cluster version of VictoriaMetrics:

```sh
curl -v http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```


The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#page-53) and will look like:


```sh
*   Trying 127.0.0.1:8481...
* Connected to 127.0.0.1 (127.0.0.1) port 8481 (#0)
> GET /delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total HTTP/1.1
> Host: 127.0.0.1:8481
> User-Agent: curl/7.81.0
> Accept: */*
>
* Mark bundle as not supporting multiuse
< HTTP/1.1 204 No Content
< X-Server-Hostname: 101ed7a45c94
< Date: Tue, 21 Jun 2022 07:21:36 GMT
<
* Connection #0 to host 127.0.0.1 left intact
```


Additional information:

* [How to delete time series](https://docs.victoriametrics.com/#how-to-delete-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/export

**Exports raw samples from VictoriaMetrics in JSON line format**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```


Additional information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export data in JSON line format](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/export/csv

**Exports raw samples from VictoriaMetrics in CSV format**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/export/csv -d 'format=__name__,__value__,__timestamp__:unix_s' -d 'match[]=vm_http_request_errors_total' > filename.csv
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv -d 'format=__name__,__value__,__timestamp__:unix_s' -d 'match[]=vm_http_request_errors_total' > filename.csv
```


Additional information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/export/native

**Exports raw samples from VictoriaMetrics in native format**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```


More information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/import

**Imports data to VictoriaMetrics in JSON line format**

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST http://localhost:8428/api/v1/import
```


Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' --data-binary "@filename.json" -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import
```


More information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/import/csv

**Imports CSV data to VictoriaMetrics**

Single-node VictoriaMetrics:

```sh
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```


Cluster version of VictoriaMetrics:

```sh
curl -d "GOOG,1.23,4.56,NYSE" 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```


Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/import/native

**Imports data to VictoriaMetrics in native format**

Single-node VictoriaMetrics:

```sh
curl -X POST http://localhost:8428/api/v1/import/native -T filename.bin
```

Cluster version of VictoriaMetrics:

```sh
curl -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/native -T filename.bin
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/import/prometheus

**Imports data to VictoriaMetrics in Prometheus text exposition format**

Single-node VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST http://localhost:8428/api/v1/import/prometheus
```


Cluster version of VictoriaMetrics:

```sh
curl -d 'metric_name{foo="bar"} 123' -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/prometheus
```

Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/labels

**Get a list of label names at the given time range**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/labels
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/labels
```

By default, VictoriaMetrics returns labels seen during the last day starting at 00:00 UTC because of performance reasons.
An arbitrary time range can be set via [`start` and `end` query args](https://docs.victoriametrics.com/#timestamp-formats).
The specified `start..end` time range is rounded to UTC day granularity because of performance reasons.

Additional information:
* [Getting label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/label/.../values

**Get a list of values for a particular label on the given time range**

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
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/query

**Performs PromQL/MetricsQL instant query**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```


Additional information:
* [Instant queries](https://docs.victoriametrics.com/keyconcepts/#instant-query)
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/keyconcepts/#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/query_range

**Performs PromQL/MetricsQL range query**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```


Additional information:
* [Range queries](https://docs.victoriametrics.com/keyconcepts/#range-query)
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Query language](https://docs.victoriametrics.com/keyconcepts/#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /api/v1/series

**Returns series names with their labels on the given time range**

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

Additional information:
* [Finding series by label matchers](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)
VictoriaMetrics accepts `limit` query arg for `/api/v1/series` handlers for limiting the number of returned entries. For example, the query to `/api/v1/series?limit=5` returns a sample of up to 5 series, while ignoring the rest. If the provided `limit` value exceeds the corresponding `-search.maxSeries` command-line flag values, then limits specified in the command-line flags are used.

### /api/v1/status/tsdb

**Cardinality statistics**

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
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /datadog

**DataDog URL for Single-node VictoriaMetrics**


```
http://victoriametrics:8428/datadog
```


**DataDog URL for Cluster version of VictoriaMetrics**


```
http://vminsert:8480/insert/0/datadog
```


### /datadog/api/v1/series

**Imports data in DataDog v1 format into VictoriaMetrics**

Single-node VictoriaMetrics:

```sh
echo '
{
  "series": [
    {
      "host": "test.example.com",
      "interval": 20,
      "metric": "system.load.1",
      "points": [[
        0,
        0.5
      ]],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}
' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://localhost:8428/datadog/api/v1/series
```


Cluster version of VictoriaMetrics:

```sh
echo '
{
  "series": [
    {
      "host": "test.example.com",
      "interval": 20,
      "metric": "system.load.1",
      "points": [[
        0,
        0.5
      ]],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}
' | curl -X POST -H 'Content-Type: application/json' --data-binary @- 'http://<vminsert>:8480/insert/0/datadog/api/v1/series'
```


Additional information:

* [How to send data from DataDog agent](https://docs.victoriametrics.com/#how-to-send-data-from-datadog-agent)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)


### /datadog/api/v2/series

**Imports data in [DataDog v2](https://docs.datadoghq.com/api/latest/metrics/#submit-metrics) format into VictoriaMetrics**

Single-node VictoriaMetrics:

```sh
echo '
{
  "series": [
    {
      "metric": "system.load.1",
      "type": 0,
      "points": [
        {
          "timestamp": 0,
          "value": 0.7
        }
      ],
      "resources": [
        {
          "name": "dummyhost",
          "type": "host"
        }
      ],
      "tags": ["environment:test"]
    }
  ]
}
' | curl -X POST -H 'Content-Type: application/json' --data-binary @- http://localhost:8428/datadog/api/v2/series
```


Cluster version of VictoriaMetrics:

```sh
echo '
{
  "series": [
    {
      "metric": "system.load.1",
      "type": 0,
      "points": [
        {
          "timestamp": 0,
          "value": 0.7
        }
      ],
      "resources": [
        {
          "name": "dummyhost",
          "type": "host"
        }
      ],
      "tags": ["environment:test"]
    }
  ]
}
' | curl -X POST -H 'Content-Type: application/json' --data-binary @- 'http://<vminsert>:8480/insert/0/datadog/api/v2/series'
```


Additional information:

* [How to send data from DataDog agent](https://docs.victoriametrics.com/#how-to-send-data-from-datadog-agent)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /federate

**Returns federated metrics**

Single-node VictoriaMetrics:

```sh
curl http://localhost:8428/federate -d 'match[]=vm_http_request_errors_total'
```


Cluster version of VictoriaMetrics:

```sh
curl http://<vmselect>:8481/select/0/prometheus/federate -d 'match[]=vm_http_request_errors_total'
```


Additional information:

* [Federation](https://docs.victoriametrics.com/#federation)
* [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /graphite/metrics/find

**Searches Graphite metrics in VictoriaMetrics**

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
* [Graphite API in VictoriaMetrics](https://docs.victoriametrics.com/#graphite-api-usage)
* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [URL Format](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /influx/write

**Writes data with InfluxDB line protocol to VictoriaMetrics**

Single-node VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://localhost:8428/write
```


Cluster version of VictoriaMetrics:

```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://<vminsert>:8480/insert/0/influx/write
```


Additional information:

* [How to send Influx data to VictoriaMetrics](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)
* [URL Format](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format)

### /internal/resetRollupResultCache

**Resets the response cache for previously served queries. It is recommended to invoke after [backfilling](https://docs.victoriametrics.com/#backfilling) procedure.**

Single-node VictoriaMetrics:

```sh
curl -Is http://localhost:8428/internal/resetRollupResultCache
```

Cluster version of VictoriaMetrics:

```sh
curl -Is http://<vmselect>:8481/internal/resetRollupResultCache
```

vmselect will propagate this call to the rest of the vmselects listed in its `-selectNode` cmd-line flag. If this
flag isn't set, then cache need to be purged from each vmselect individually.



### TCP and UDP

#### How to send data from OpenTSDB-compatible agents to VictoriaMetrics

Turned off by default. Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command-line flag.
*If run from docker, '-opentsdbListenAddr' port should be exposed*

Single-node VictoriaMetrics:

```sh
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```


Cluster version of VictoriaMetrics:

```sh
echo "put foo.bar.baz `date +%s` 123  tag1=value1 tag2=value2" | nc -N http://<vminsert> 4242
```


Enable HTTP server for OpenTSDB /api/put requests by setting `-opentsdbHTTPListenAddr` command-line flag.

Single-node VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```


Cluster version of VictoriaMetrics:

```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://<vminsert>:8480/insert/42/opentsdb/api/put
```


Additional information:

* [OpenTSDB http put API](http://opentsdb.net/docs/build/html/api_http/put.html)
* [How to send data OpenTSDB data to VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-opentsdb-compatible-agents)

#### How to send Graphite data to VictoriaMetrics

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command-line flag.

Single-node VictoriaMetrics:

```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```


Cluster version of VictoriaMetrics:

```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N http://<vminsert> 2003
```


Additional information:

* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [Multitenancy in cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy)
