# URL Examples



# /api/v1/admin/tsdb/delete_series

**Delete time series**
 
Single:
```bash
curl 'http://<victoriametrics-addr>:8428/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

Cluster:
```bash
curl 'http://<vmselect>:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

Additional information:
* https://docs.victoriametrics.com/?highlight=delete%20api#how-to-delete-time-series 



# /api/v1/export/csv

**Exports CSV data from VictoriaMetrics**
 
Single:
```bash
curl 'http://<victoriametrics-addr>:8428/api/v1/export/csv?format=__name__,__value__,__timestamp__:unix_s&match=vm_http_request_errors_total' > filename.txt
```
 
Cluster:
```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/csv?format=__name__,__value__,__timestamp__:unix_s&match=vm_http_request_errors_total' > filename.txt
```

Additional information: 
* https://docs.victoriametrics.com/#how-to-export-csv-data 
* https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format 

If run from docker, <vminsert> address can be found via: “docker ps” command
The assigned port might be different


# /api/v1/export/native
  
**Exporting in native format**

Single:
```bash
curl -G 'http://<victoriametrics-addr>:8428/api/v1/export/native?match[]=vm_http_request_errors_total' > filename.txt
```

Cluster:
```bash
curl -G 'http://<vmselect>:8481/select/0/prometheus/api/v1/export/native?match=vm_http_request_errors_total' > filename.txt
```

More information:
* https://docs.victoriametrics.com/?highlight=echo#how-to-export-data-in-native-format



# /api/v1/import/csv 

**Imports CSV data to VictoriaMetrics**
 
Single:
```bash
curl --data-binary "@import.txt" -X POST 'http://localhost:8428/api/v1/import/prometheus'
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

Cluster:
```bash
curl --data-binary "@import.txt" -X POST  'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv'
curl -d "GOOG,1.23,4.56,NYSE" 'http://<vminsert>:8480/insert/0/prometheus/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

Additional information: 
* https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format
* https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-csv-data 



# /datadog/api/v1/series

**Sends data from DataDog agent to VM**
 
Single:
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

Cluster:
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

Additional information:
* https://docs.victoriametrics.com/?highlight=post#how-to-send-data-from-datadog-agent 



# /graphite/metrics/find

**Searches Graphite metrics**

Single:
```bash
curl -G 'http://localhost:8428/graphite/metrics/find?query=vm_http_request_errors_total'
```
 
Cluster:
```bash
curl -G 'http://<vmselect>:8481/select/0/graphite/metrics/find?query=vm_http_request_errors_total'
```
 
Additional information:
* https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
* https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd 
* https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html?highlight=url%20format#url-format



# /influx/write

**Writes data with InfluxDB line protocol to local VictoriaMetrics**

Single:
```bash
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://localhost:8428/write'
```
 
Cluster:
```bash
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<vminsert>:8480/insert/0/influx/write'
```
 
Additional information:
* https://docs.victoriametrics.com/?highlight=post#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf 




# /prometheus/api/v1/import

**Importing data obtained via api/v1/export at vmselect**

Single:
```bash
curl --data-binary "@import.txt" -X POST 'http://destination-victoriametrics:8428/api/v1/import'
```
 
Cluster:
```bash
curl --data-binary "@import.txt" -X POST 'http://<vminsert>:8480/insert/prometheus/api/v1/import'
```
 
Additional information:
* https://docs.victoriametrics.com/?highlight=echo#how-to-import-time-series-data



# TCP and UDP

**Sends data from OpenTSDB-compatible agents**

Turned off by default.  
Enable OpenTSDB receiver in VictoriaMetrics by setting -opentsdbListenAddr command line flag.
*If run from docker, '-opentsdbListenAddr' port should be exposed*

Single:
```bash
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```
 
Cluster:
```bash
echo "put foo.bar.baz `date +%s` 123  tag1=value1 tag2=value2 VictoriaMetrics_AccountID=0" | nc -N http://<vminsert> 4242
```
 

**Enable HTTP server for OpenTSDB /api/put requests by setting -opentsdbHTTPListenAddr**
 
Single:
```bash
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```
 
Cluster:
```bash
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]'
 'http://<vminsert>:8480/insert/42/opentsdb/api/put'
```
 
Additional information:
* http://opentsdb.net/docs/build/html/api_http/put.html
* https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-opentsdb-compatible-agents



**Writes data with Graphite plaintext protocol to local VictoriaMetrics using nc**

Enable Graphite receiver in VictoriaMetrics by setting -graphiteListenAddr command line flag
 
Single:
```bash
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" |
 nc -N localhost 2003
```
 
Cluster:
```bash
echo "foo.bar.baz;tag1=value1;tag2=value2;VictoriaMetrics_AccountID=42 123 `date +%s`" | nc -N http://<vminsert> 2003
```

Additional information:

VictoriaMetrics_AccountID=42 - tag that indicated tennant ID.
* https://github.com/VictoriaMetrics/VictoriaMetrics/blob/a3eafd2e7fc75776dfc19d3c68c85589454d9dce/app/vminsert/opentsdb/request_handler.go#L47
* https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd




