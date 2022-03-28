---
sort: 21
---

# VictoriaMetrics API examples

## /api/v1/admin/tsdb/delete_series

**Deletes time series from VictoriaMetrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl 'http://<victoriametrics-addr>:8428/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl 'http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

</div>

Additional information:

* [How to delete time series](https://docs.victoriametrics.com/#how-to-delete-time-series)

## /api/v1/export/csv

**Exports CSV data from VictoriaMetrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl 'http://<victoriametrics-addr>:8428/api/v1/export/csv?format=__name__,__value__,__timestamp__:unix_s&match=vm_http_request_errors_total' > filename.txt
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv?format=__name__,__value__,__timestamp__:unix_s&match=vm_http_request_errors_total' > filename.txt
```

</div>

Additional information:

* [How to export time series](https://docs.victoriametrics.com/#how-to-export-csv-data)
* [URL Format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /api/v1/export/native
  
**Exports data from VictoriaMetrics in native format**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<victoriametrics-addr>:8428/api/v1/export/native?match[]=vm_http_request_errors_total' > filename.txt
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/native?match=vm_http_request_errors_total' > filename.txt
```

</div>

More information:

* [How to export data in native format](https://docs.victoriametrics.com/#how-to-export-data-in-native-format)

## /api/v1/import

**Imports data obtained via /api/v1/export**

Single:
<div class="with-copy" markdown="1">

```bash
curl --data-binary "@import.txt" -X POST 'http://destination-victoriametrics:8428/api/v1/import'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl --data-binary "@import.txt" -X POST 'http://<vminsert>:8480/insert/prometheus/api/v1/import'
```

</div>

Additional information:

* [How to import time series data](https://docs.victoriametrics.com/#how-to-import-time-series-data)

## /api/v1/import/csv

**Imports CSV data to VictoriaMetrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl --data-binary "@import.txt" -X POST 'http://localhost:8428/api/v1/import/prometheus'
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl --data-binary "@import.txt" -X POST  'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv'
curl -d "GOOG,1.23,4.56,NYSE" 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

</div>

Additional information:

* [URL format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)
* [How to import CSV data](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-csv-data)

## /api/v1/labels

**Get a list of label names**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/labels'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/labels'
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Querying label values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)

## /api/v1/label/\<label_name\>/values

**Querying label values**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/label/job/values'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/label/job/values'
```

</div>

Additional information: 
* [Getting label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)

## /api/v1/query

**Performs PromQL/MetricsQL instant query**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/query?query=vm_http_request_errors_total&time=2021-02-22T19:10:30.781Z'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/query?query=vm_http_request_errors_total&time=2021-02-22T19:10:30.781Z'
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Instant queries](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries)
* [Instant vector selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#instant-vector-selectors)

## /api/v1/query_range

**Performs PromQL/MetricsQL range_query**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/query_range?query=vm_http_request_errors_total&start=2021-02-22T19:10:30.781Z&step=20m'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/query_range?query=vm_http_request_errors_total&start=2021-02-22T19:10:30.781Z&step=20m'
```

</div>

<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/query_range?query=vm_http_request_errors_total&start=-1h&step=10m'
```

```bash
curl -G http://<vmselect>:8481/select/0/prometheus/api/v1/query_range --data-urlencode 'query=sum(increase(vm_http_request_errors_total{status=""}[5m])) by (status)'
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Range queries](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries)
* [Range Vector Selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors)

## /api/v1/series

**Finding series by label matchers**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/series?match[]=vm_http_request_errors_total&start=-1h'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/series?match[]=vm_http_request_errors_total&start=-1h'
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Finding series by label matchers](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)

## /api/v1/status/tsdb

**Cardinality statistics**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/prometheus/api/v1/status/tsdb'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/status/tsdb'
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [TSDB Stats](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats)

## /api/v1/targets  

**Checking targets**

Should be sent to vmagent/VMsingle

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmsingle>:8428/api/v1/targets' 
```

</div>

cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmagent>:8429/api/v1/targets' 
```

</div>

Additional information: 
* [Prometheus querying API usage](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
* [Targets](https://prometheus.io/docs/prometheus/latest/querying/api/#targets)

## /datadog/api/v1/series

**Sends data from DataDog agent to VM**

Single:
<div class="with-copy" markdown="1">

```bash
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

Cluster:
<div class="with-copy" markdown="1">

```bash
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

## /federate

**Returns federated metrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/federate?match[]=vm_http_request_errors_total&start=2021-02-22T19:10:30.781Z'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/federate?match[]=vm_http_request_errors_total&start=2021-02-22T19:10:30.781Z'
```

</div>

Additional information:

* [Federation](https://docs.victoriametrics.com/#federation)
* [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation)

## /graphite/metrics/find

**Searches Graphite metrics in VictoriaMetrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://localhost:8428/graphite/metrics/find?query=vm_http_request_errors_total'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -G 'http://<vmselect>:8481/select/0/graphite/metrics/find?query=vm_http_request_errors_total'
```

</div>

Additional information:

* [Metrics find](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [How to send data from graphite compatible agents such as statsd](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [URL Format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)

## /influx/write

**Writes data with InfluxDB line protocol to VictoriaMetrics**

Single:
<div class="with-copy" markdown="1">

```bash
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://localhost:8428/write'
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<vminsert>:8480/insert/0/influx/write'
```

</div>

Additional information:

* [How to send data from influxdb compatible agents such as telegraf](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)

## TCP and UDP

**How to send data from OpenTSDB-compatible agents to VictoriaMetrics**

Turned off by default. Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command-line flag.
*If run from docker, '-opentsdbListenAddr' port should be exposed*

Single:
<div class="with-copy" markdown="1">

```bash
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
echo "put foo.bar.baz `date +%s` 123  tag1=value1 tag2=value2 VictoriaMetrics_AccountID=0" | nc -N http://<vminsert> 4242
```

</div>

Enable HTTP server for OpenTSDB /api/put requests by setting `-opentsdbHTTPListenAddr` command-line flag.

Single:
<div class="with-copy" markdown="1">

```bash
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]'
 'http://<vminsert>:8480/insert/42/opentsdb/api/put'
```

</div>

Additional information:

* [Api http put](http://opentsdb.net/docs/build/html/api_http/put.html)
* [How to send data from opentsdb compatible agents](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-opentsdb-compatible-agents)

**How to write data with Graphite plaintext protocol to VictoriaMetrics**

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command-line flag.

Single:
<div class="with-copy" markdown="1">

```bash
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" |
 nc -N localhost 2003
```

</div>

Cluster:
<div class="with-copy" markdown="1">

```bash
echo "foo.bar.baz;tag1=value1;tag2=value2;VictoriaMetrics_AccountID=42 123 `date +%s`" | nc -N http://<vminsert> 2003
```

</div>

Additional information:

`VictoriaMetrics_AccountID=42` - tag that indicated tenant ID.

* [Request handler](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/a3eafd2e7fc75776dfc19d3c68c85589454d9dce/app/vminsert/opentsdb/request_handler.go#L47)
* [How to send data from graphite compatible agents such as statsd](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
