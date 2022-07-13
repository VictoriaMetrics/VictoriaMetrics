---
sort: 21
---

# VictoriaMetrics API examples

## /api/v1/admin/tsdb/delete_series

**Deletes time series from VictoriaMetrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -v http://localhost:8428/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```

</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#page-53) and will look like:

<div class="with-copy" markdown="1">

```console
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

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -v http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series -d 'match[]=vm_http_request_errors_total'
```

</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#page-53) and will look like:

<div class="with-copy" markdown="1">

```console
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

</div>

Additional information:

* [How to delete time series](https://docs.victoriametrics.com/#how-to-delete-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/export

**Exports raw samples from VictoriaMetrics in JSON line format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export -d 'match[]=vm_http_request_errors_total' > filename.json
```

</div>

Additional information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/export/csv

**Exports raw samples from VictoriaMetrics in CSV format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/api/v1/export/csv -d 'format=__name__,__value__,__timestamp__:unix_s' -d 'match[]=vm_http_request_errors_total' > filename.csv
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv -d 'format=__name__,__value__,__timestamp__:unix_s' -d 'match[]=vm_http_request_errors_total' > filename.csv
```

</div>

Additional information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/export/native

**Exports raw samples from VictoriaMetrics in native format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/export/native -d 'match[]=vm_http_request_errors_total' > filename.bin
```

</div>

More information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/import

**Imports data to VictoriaMetrics in JSON line format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl --data-binary "@filename.json" -X POST http://localhost:8428/api/v1/import
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl --data-binary "@filename.json" -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import
```

</div>

More information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/import/csv

**Imports CSV data to VictoriaMetrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d "GOOG,1.23,4.56,NYSE" 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

</div>

Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/import/native

**Imports data to VictoriaMetrics in native format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -X POST http://localhost:8428/api/v1/import/native -T filename.bin
```

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/native -T filename.bin
```
</div>

Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/import/prometheus

**Imports data to VictoriaMetrics in Prometheus text exposition format**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d 'metric_name{foo="bar"} 123' -X POST http://localhost:8428/api/v1/import/prometheus
```

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d 'metric_name{foo="bar"} 123' -X POST http://<vminsert>:8480/insert/0/prometheus/api/v1/import/prometheus
```
</div>

Additional information:

* [How to import time series](https://docs.victoriametrics.com/#how-to-import-time-series-data)
* [How to export time series](https://docs.victoriametrics.com/#how-to-export-time-series)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/labels

**Get a list of label names**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/labels
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/labels
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Querying label values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/label/.../values

**Get a list of values for a particular label**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/label/job/values
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/label/job/values
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Getting label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/query

**Performs PromQL/MetricsQL instant query**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query -d 'query=vm_http_request_errors_total'
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Instant queries](https://docs.victoriametrics.com/keyConcepts.html#instant-query)
* [Query language](https://docs.victoriametrics.com/keyConcepts.html#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/query_range

**Performs PromQL/MetricsQL range query**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/query_range -d 'query=sum(increase(vm_http_request_errors_total{job="foo"}[5m]))' -d 'start=-1d' -d 'step=1h'
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Range queries](https://docs.victoriametrics.com/keyConcepts.html#range-query)
* [Query language](https://docs.victoriametrics.com/keyConcepts.html#metricsql)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/series

**Returns series names with their labels**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/series -d 'match[]=vm_http_request_errors_total'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/series -d 'match[]=vm_http_request_errors_total'
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Finding series by label matchers](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)
VictoriaMetrics accepts `limit` query arg for `/api/v1/series` handlers for limiting the number of returned entries. For example, the query to `/api/v1/series?limit=5` returns a sample of up to 5 series, while ignoring the rest. If the provided `limit` value exceeds the corresponding `-search.maxSeries` command-line flag values, then limits specified in the command-line flags are used.

## /api/v1/status/tsdb

**Cardinality statistics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/prometheus/api/v1/status/tsdb
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/api/v1/status/tsdb
```

</div>

Additional information:
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [TSDB Stats](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /datadog/api/v1/series

**Imports data in DataDog format into VictoriaMetrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
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
' | curl -X POST --data-binary @- http://localhost:8428/datadog/api/v1/series
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
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
' | curl -X POST --data-binary @- 'http://<vminsert>:8480/insert/0/datadog/api/v1/series'
```

</div>

Additional information:

* [How to send data from datadog agent](https://docs.victoriametrics.com/#how-to-send-data-from-datadog-agent)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /federate

**Returns federated metrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/federate -d 'match[]=vm_http_request_errors_total'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/prometheus/federate -d 'match[]=vm_http_request_errors_total'
```

</div>

Additional information:

* [Federation](https://docs.victoriametrics.com/#federation)
* [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation)
* [URL format for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /graphite/metrics/find

**Searches Graphite metrics in VictoriaMetrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://localhost:8428/graphite/metrics/find -d 'query=vm_http_request_errors_total'
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl http://<vmselect>:8481/select/0/graphite/metrics/find -d 'query=vm_http_request_errors_total'
```

</div>

Additional information:

* [Metrics find API in Graphite](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [Graphite API in VictoriaMetrics](https://docs.victoriametrics.com/#graphite-api-usage)
* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [URL Format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /influx/write

**Writes data with InfluxDB line protocol to VictoriaMetrics**

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://localhost:8428/write
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST http://<vminsert>:8480/insert/0/influx/write
```

</div>

Additional information:

* [How to send Influx data to VictoriaMetrics](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)
* [URL Format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## TCP and UDP

**How to send data from OpenTSDB-compatible agents to VictoriaMetrics**

Turned off by default. Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command-line flag.
*If run from docker, '-opentsdbListenAddr' port should be exposed*

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
echo "put foo.bar.baz `date +%s` 123  tag1=value1 tag2=value2 VictoriaMetrics_AccountID=0" | nc -N http://<vminsert> 4242
```

</div>

Enable HTTP server for OpenTSDB /api/put requests by setting `-opentsdbHTTPListenAddr` command-line flag.

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://<vminsert>:8480/insert/42/opentsdb/api/put
```

</div>

Additional information:

* [OpenTSDB http put API](http://opentsdb.net/docs/build/html/api_http/put.html)
* [How to send data OpenTSDB data to VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-opentsdb-compatible-agents)

**How to send Graphite data to VictoriaMetrics**

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command-line flag.

Single-node VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```

</div>

Cluster version of VictoriaMetrics:
<div class="with-copy" markdown="1">

```console
echo "foo.bar.baz;tag1=value1;tag2=value2;VictoriaMetrics_AccountID=42 123 `date +%s`" | nc -N http://<vminsert> 2003
```

</div>

Additional information:

`VictoriaMetrics_AccountID=42` - [tenant ID](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy) in cluster version of VictoriaMetrics

* [How to send Graphite data to VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [Multitenancy in cluster version of VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy)
