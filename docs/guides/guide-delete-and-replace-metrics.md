# How to delete or replace metrics in VictoriaMetrics 

## Scenario

* You have working [VictoriaMetrics Single](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker) or [VictoriaMetrics Cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster/deployment/docker).
* You want to delete the stored metrics to clear up space.
* You have mistakenly recorded metrics that should be deleted or updated.

## How to delete metrics

According to [officital documentation](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-delete-time-series) metrics can be deleted via `/api/v1/admin/tsdb/delete_series` for both version of VictoriaMetrics Single and Cluster. Before deleting we need to check that metrics which needs to be deleted is present in database.

To check that metrics are present in VictoriaMetrics run the following command:

**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl 'http://<victoria-metrics-addr>:8428/api/v1/series?match[]=vm_http_request_errors_total'
```

</div>

<details>
	<summary>The expected output will look like:</summary>

```json
{"status":"success","data":[{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/export/csv"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/delSeries"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/export"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/import","protocol":"vmimport"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/import/csv","protocol":"csvimport"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/query_range"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/status/top_queries"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/export/native"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/tagMultiSeries"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/series"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/query"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/tagSeries"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/federate"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/findSeries"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/label/{}/values"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/admin/tsdb/delete_series"},{"__name__":"vm_http_request_errors_total","job":"vmalert","instance":"vmalert:8880","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/series/count"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/import/native","protocol":"nativeimport"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/status/tsdb"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/write","protocol":"promremotewrite"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/metrics/find"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/datadog/api/v1/series","protocol":"datadog"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/metrics/expand"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/autoComplete/tags"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/import/prometheus","protocol":"prometheusimport"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/api/v1/labels"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/influx/write","protocol":"influx"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/metrics/index.json"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/autoComplete/values"},{"__name__":"vm_http_request_errors_total","job":"vmagent","instance":"vmagent:8429","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/tags/\u003ctag_name>"},{"__name__":"vm_http_request_errors_total","job":"victoriametrics","instance":"victoriametrics:8428","path":"/target_response"}]}
```

</details>

**Cluster version of VictoriaMetrics:**:

To check that metrics are present in VictoriaMetrics Cluster run the following command:

<div class="with-copy" markdown="1">

```console
curl 'http://<vmselect-addr>:8428/api/v1/series?match[]=vm_http_request_errors_total'
```

</div>

<details>
	<summary>The expected output will look like:</summary>

```json
{"status":"success","isPartial":false,"data":[{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/export"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/delete/{}/prometheus/api/v1/admin/tsdb/delete_series"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/metrics/expand"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/api/v1/status/top_queries"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/metrics/index.json"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/tagMultiSeries"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/\u003ctag_name>"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/series/count"},{"__name__":"vm_http_request_errors_total","job":"vmagent","instance":"vmagent:8429","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/autoComplete/values"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/tagSeries"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/prometheus/api/v1/import","protocol":"vmimport"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/status/top_queries"},{"__name__":"vm_http_request_errors_total","job":"vmstorage","instance":"vmstorage-2:8482","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/prometheus/api/v1/import/prometheus","protocol":"prometheusimport"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/label/{}/values"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/autoComplete/tags"},{"__name__":"vm_http_request_errors_total","job":"vmstorage","instance":"vmstorage-1:8482","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/influx/write","protocol":"influx"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/query"},{"__name__":"vm_http_request_errors_total","job":"vmalert","instance":"vmalert:8880","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/query_range"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/datadog/api/v1/series","protocol":"datadog"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"*","reason":"unsupported"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/delSeries"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/export/csv"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/federate"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/metrics/find"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/export/native"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/series"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/prometheus/","protocol":"promremotewrite"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/prometheus/api/v1/import/csv","protocol":"csvimport"},{"__name__":"vm_http_request_errors_total","job":"vminsert","instance":"vminsert:8480","path":"/insert/{}/prometheus/api/v1/import/native","protocol":"nativeimport"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/graphite/tags/findSeries"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/la* Connection #0 to host 127.0.0.1 left intact
bels"},{"__name__":"vm_http_request_errors_total","job":"vmselect","instance":"vmselect:8481","path":"/select/{}/prometheus/api/v1/status/tsdb"}]}
```

</details>

In order to delete a series based on label, send a POST request with the label selectors of your choice to delete the resulting time series from starting date till now, i.e, everything for that time series.

**Single-node VictoriaMetrics:**

The below command will delete `vm_http_request_errors_total` from database:

<div class="with-copy" markdown="1">

```console
curl -v 'http://<victoriametrics-addr>:8428/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

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

**VictoriaMetrics Cluster**

The below command will delete `vm_http_request_errors_total` from database:

<div class="with-copy" markdown="1">

```console
curl -v 'http://127.0.0.1:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=vm_http_request_errors_total'
```

</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

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

After that all the time series matching the given selector are deleted. Storage space for the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). The background merges may never occur for data from previous months, so storage space won't be freed for historical data. In this case [forced merge](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#forced-merge) may help freeing up storage space.

## How to replace metrics

By default VictoriaMetrics do not provide a mechanism that can be use to replace or update metrics. This task looks non-trivial but there is a workaround.

In short, we have to do the following: 
- export metrics to a file;
- change the values of metrics in the file and save it;
- delete metrics from database;
- import metrics from saved file to VictoriaMetrics.

### Export metrics

In this example we will use `node_memory_MemTotal_bytes` metrics that scrapes by [node_exporter](https://github.com/prometheus/node_exporter/) and then vmagent write it to VictoriaMetrics Single and Cluster versions and [JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-export-data-in-json-line-format) to export metrics.

Let's export metrics for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`. To do it run the next command:

**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g http://127.0.0.1:8428/api/v1/export -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

</div>

**Cluster version of VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g http://127.0.0.1:8481/select/0/prometheus/api/v1/export -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

</div>

To check that exported file contains metrics we can use [cat](https://man7.org/linux/man-pages/man1/cat.1.html) and [jq](https://stedolan.github.io/jq/download/)

<div class="with-copy" markdown="1">

```console
cat data.jsonl | jq
```

</div>

<details>
	<summary>The expeted output will looks like:</summary>

```json
{
  "metric": {
    "__name__": "node_memory_MemTotal_bytes",
    "job": "hostname.com",
    "instance": "node-exporter:9100"
  },
  "values": [
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    33604390912,
    null
  ],
  "timestamps": [
    1656669031378,
    1656669032378,
    1656669033378,
    1656669034378,
    1656669035378,
    1656669036378,
    1656669037378,
    1656669038378,
    1656669039378,
    1656669040378,
    1656669041378,
    1656669042378,
    1656669043378,
    1656669044378,
    1656669045378,
    1656669046378,
    1656669047378,
    1656669048378,
    1656669049378,
    1656669050378,
    1656669051378,
    1656669052378,
    1656669053378,
    1656669054378,
    1656669055378,
    1656669056378,
    1656669057378,
    1656669058378,
    1656669059378,
    1656669060378,
    1656669061378,
    1656669062378,
    1656669063378,
    1656669064378,
    1656669065378,
    1656669066378,
    1656669067378,
    1656669068378,
    1656669069378,
    1656669070378,
    1656669071378,
    1656669072378,
    1656669073378,
    1656669074378,
    1656669075378,
    1656669076378,
    1656669077378,
    1656669078378,
    1656669079378,
    1656669080378,
    1656669081378,
    1656669082378,
    1656669083378,
    1656669084378,
    1656669085378,
    1656669086378,
    1656669087378,
    1656669088378,
    1656669089378,
    1656669090378,
    1656669091378,
    1656669092378,
    1656669093378,
    1656669094378,
    1656669095378,
    1656669096378,
    1656669097378,
    1656669098378,
    1656669099378,
    1656669100378,
    1656669101378,
    1656669102378,
    1656669103378,
    1656669104378,
    1656669105378,
    1656669106378,
    1656669107378,
    1656669108378,
    1656669109378,
    1656669110378,
    1656669111378,
    1656669112378,
    1656669113378,
    1656669114378,
    1656669115378,
    1656669116378,
    1656669117378,
    1656669118378,
    1656669119378,
    1656669120378,
    1656669121378,
    1656669122378,
    1656669123378,
    1656669124378,
    1656669125378,
    1656669126378,
    1656669127378,
    1656669128378,
    1656669129378,
    1656669130378,
    1656669131378,
    1656669132378,
    1656669133378,
    1656669134378,
    1656669135378,
    1656669136378,
    1656669137378,
    1656669138378,
    1656669139378,
    1656669140378
  ]
}

```
</details>

For this example we will to change the values of `node_memory_MemTotal_bytes` from `33604390912` to `17179869184` (from 32Gb to 16Gb) with [sed](https://linux.die.net/man/1/sed), but it can be done in any of the available ways.

```console
sed -i 's/33604390912/17179869184/g' data.jsonl
```

Let's check the changes in data.jsonl with `cat`:

```console
cat data.jsonl | jq
```

<details>
	<summary>The expeted output will looks like:</summary>

```json
{
  "metric": {
    "__name__": "node_memory_MemTotal_bytes",
    "job": "hostname.com",
    "instance": "node-exporter:9100"
  },
  "values": [
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    17179869184,
    null
  ],
  "timestamps": [
    1656669031378,
    1656669032378,
    1656669033378,
    1656669034378,
    1656669035378,
    1656669036378,
    1656669037378,
    1656669038378,
    1656669039378,
    1656669040378,
    1656669041378,
    1656669042378,
    1656669043378,
    1656669044378,
    1656669045378,
    1656669046378,
    1656669047378,
    1656669048378,
    1656669049378,
    1656669050378,
    1656669051378,
    1656669052378,
    1656669053378,
    1656669054378,
    1656669055378,
    1656669056378,
    1656669057378,
    1656669058378,
    1656669059378,
    1656669060378,
    1656669061378,
    1656669062378,
    1656669063378,
    1656669064378,
    1656669065378,
    1656669066378,
    1656669067378,
    1656669068378,
    1656669069378,
    1656669070378,
    1656669071378,
    1656669072378,
    1656669073378,
    1656669074378,
    1656669075378,
    1656669076378,
    1656669077378,
    1656669078378,
    1656669079378,
    1656669080378,
    1656669081378,
    1656669082378,
    1656669083378,
    1656669084378,
    1656669085378,
    1656669086378,
    1656669087378,
    1656669088378,
    1656669089378,
    1656669090378,
    1656669091378,
    1656669092378,
    1656669093378,
    1656669094378,
    1656669095378,
    1656669096378,
    1656669097378,
    1656669098378,
    1656669099378,
    1656669100378,
    1656669101378,
    1656669102378,
    1656669103378,
    1656669104378,
    1656669105378,
    1656669106378,
    1656669107378,
    1656669108378,
    1656669109378,
    1656669110378,
    1656669111378,
    1656669112378,
    1656669113378,
    1656669114378,
    1656669115378,
    1656669116378,
    1656669117378,
    1656669118378,
    1656669119378,
    1656669120378,
    1656669121378,
    1656669122378,
    1656669123378,
    1656669124378,
    1656669125378,
    1656669126378,
    1656669127378,
    1656669128378,
    1656669129378,
    1656669130378,
    1656669131378,
    1656669132378,
    1656669133378,
    1656669134378,
    1656669135378,
    1656669136378,
    1656669137378,
    1656669138378,
    1656669139378,
    1656669140378
  ]
}
```
</details>

### Delete metrics

The next command will delete metrics with name `node_memory_MemTotal_bytes` which has labels `instance="node-exporter:9100"` and `job="hostname.com"` from database:

**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST 'http://127.0.0.1:8428/api/v1/admin/tsdb/delete_series?match[]=node_memory_MemTotal_bytes\{instance="node-exporter:9100",job="hostname.com"\}'
```
</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

```console
*   Trying 127.0.0.1:8428...
* Connected to 127.0.0.1 (127.0.0.1) port 8428 (#0)
> POST /api/v1/admin/tsdb/delete_series?match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100",job="hostname.com"} HTTP/1.1
> Host: 127.0.0.1:8428
> User-Agent: curl/7.81.0
> Accept: */*
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 204 No Content
< X-Server-Hostname: 677ca3de6cd5
< Date: Thu, 30 Jun 2022 07:23:26 GMT
< 
* Connection #0 to host 127.0.0.1 left intact
```

**Cluster version of VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST 'http://127.0.0.1:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=node_memory_MemTotal_bytes\{instance="node-exporter:9100",job="hostname.com"\}'
```
</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

```console
*   Trying 127.0.0.1:8481...
* Connected to 127.0.0.1 (127.0.0.1) port 8481 (#0)
> POST /delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100",job="hostname.com"} HTTP/1.1
> Host: 127.0.0.1:8481
> User-Agent: curl/7.81.0
> Accept: */*
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 204 No Content
< X-Server-Hostname: 990b3df2eece
< Date: Tue, 30 Jun 2022 07:29:58 GMT
< 
* Connection #0 to host 127.0.0.1 left intact
```

After `soft` deleting metric from database we need to run [force merge](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#forced-merge) in order to free up disk space occupied by deleted time series. The next command will triger `force merge`:

**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8428/internal/force_merge
```

The expected output should return [HTTP Status 200](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.1) and will look like:

```console
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to 127.0.0.1 (127.0.0.1) port 8428 (#0)
> GET /internal/force_merge HTTP/1.1
> Host: 127.0.0.1:8428
> User-Agent: curl/7.58.0
> Accept: */*
> 
< HTTP/1.1 200 OK
< X-Server-Hostname: 92e1057b222f
< Date: Wed, 29 Jun 2022 07:25:10 GMT
< Content-Length: 0
< 
* Connection #0 to host 127.0.0.1 left intact
```

**Cluster version of VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8482/internal/force_merge
```

The expected output should return [HTTP Status 200](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.1) and will look like:

```console
*   Trying 127.0.0.1:8482...
* Connected to 127.0.0.1 (127.0.0.1) port 8482 (#0)
> GET /internal/force_merge HTTP/1.1
> Host: 127.0.0.1:8482
> User-Agent: curl/7.81.0
> Accept: */*
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 200 OK
< X-Server-Hostname: f5f11c905b25
< Date: Tue, 30 Jun 2022 07:36:14 GMT
< Content-Length: 0
< 
* Connection #0 to host 127.0.0.1 left intact
```

### Import metrics

Victoriametrics supports a lot of [ingestion protocols](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-time-series-data) and we will use [import from JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-json-line-format).

The next command will import metrics from `data.jsonl` to VictoriaMetrics:

**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8428/api/v1/import -T data.jsonl
```
</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

```console
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to 127.0.0.1 (127.0.0.1) port 8428 (#0)
> POST /api/v1/import HTTP/1.1
> Host: 127.0.0.1:8428
> User-Agent: curl/7.58.0
> Accept: */*
> Content-Length: 2085388
> Expect: 100-continue
> 
< HTTP/1.1 100 Continue
* We are completely uploaded and fine
< HTTP/1.1 204 No Content
< X-Server-Hostname: 92e1057b222f
< Date: Wed, 30 Jun 2022 07:31:25 GMT
< 
* Connection #0 to host 127.0.0.1 left intact
```

**Cluster version of VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8480/insert/0/prometheus/api/v1/import -T data.jsonl
```
</div>

The expected output should return [HTTP Status 204](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3.5) and will look like:

```console
*   Trying 127.0.0.1:8480...
* Connected to 127.0.0.1 (127.0.0.1) port 8480 (#0)
> POST /insert/0/prometheus/api/v1/import HTTP/1.1
> Host: 127.0.0.1:8480
> User-Agent: curl/7.81.0
> Accept: */*
> Content-Length: 366
> Expect: 100-continue
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 100 Continue
* We are completely uploaded and fine
* Mark bundle as not supporting multiuse
< HTTP/1.1 204 No Content
< X-Server-Hostname: 757ab90c0647
< Date: Tue, 30 Jun 2022 08:02:00 GMT
< 
* Connection #0 to host 127.0.0.1 left intact
```

### Check imported metrics


**Single-node VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g 'http://127.0.0.1:8428/api/v1/export -d match[]=node_memory_MemTotal_bytes
```

</div>

**Cluster version of VictoriaMetrics:**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g http://127.0.0.1:8481/select/0/prometheus/api/v1/export -d match[]=node_memory_MemTotal_bytes
```

</div>

<details>
  <summary>The expected output will look like:</summary>

```json
{"metric":{"__name__":"node_memory_MemTotal_bytes","job":"hostname.com","instance":"node-exporter:9100"},"values":[17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,17179869184,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912,33604390912],"timestamps":[1656570483784,1656570493784,1656570503784,1656570513784,1656570523784,1656570533784,1656570543784,1656570553784,1656570563784,1656570573784,1656570583784,1656570593784,1656570603784,1656570613784,1656570623784,1656570633784,1656570643784,1656570653784,1656570663784,1656570673784,1656570683784,1656570693784,1656570703784,1656570713784,1656570723784,1656570733784,1656570743784,1656570753784,1656570763784,1656570773784,1656570783784,1656570793784,1656570803784,1656570813784,1656570823784,1656570833784,1656570843784,1656570853784,1656570863784,1656570873784,1656570883784,1656570893784,1656570903784,1656570913784,1656570923784,1656570933784,1656570943784,1656570953784,1656570963784,1656570973784,1656570983784,1656570993784,1656571003784,1656571013784,1656571023784,1656571033784,1656571043784,1656571053784,1656571063784,1656571073784,1656571083784,1656571093784,1656571103784,1656571113784,1656571123784,1656571133784,1656571143784,1656571153784,1656571163784,1656571173784,1656571183784,1656571193784,1656571203784,1656571213784,1656571223784,1656571233784,1656571243784,1656571253784,1656571263784,1656571273784,1656571283784,1656571293784,1656571303784,1656571313784,1656571323784,1656571333784,1656571343784,1656571353784,1656571363784,1656571373784,1656571383784,1656571393784,1656571403784,1656571413784,1656571423784,1656571433784,1656571443784,1656571453784,1656571463784,1656571473784,1656571483784,1656571493784,1656571503784,1656571513784,1656571523784,1656571533784,1656571543784,1656571553784,1656571563784,1656571573784,1656571583784,1656571593784,1656571603784,1656571613784,1656571623784,1656571633784,1656571643784,1656571653784,1656571663784,1656571673784,1656571683784,1656571693784,1656571703784,1656571713784,1656571723784,1656571733784,1656571743784,1656571753784,1656571763784,1656571773784,1656571783784,1656571793784,1656571803784,1656571813784,1656571823784,1656571833784,1656571843784,1656571853784,1656571863784,1656571873784,1656571883784,1656571893784,1656571903784,1656571913784,1656571923784,1656571933784,1656571943784,1656571953784,1656571963784,1656571973784,1656571983784,1656571993784,1656572003784,1656572013784,1656572023784,1656572033784,1656572043784,1656572053784,1656572063784,1656572073784,1656572083784,1656572093784,1656572103784,1656572113784,1656572123784,1656572133784,1656572143784,1656572153784,1656572163784,1656572173784,1656572183784,1656572193784,1656572203784,1656572213784,1656572223784,1656572233784,1656572243784,1656572253784,1656572263784,1656572273784,1656572283784,1656572293784,1656572303784,1656572313784,1656572323784,1656572333784,1656572343784,1656572353784,1656572363784,1656572373784,1656572383784,1656572393784,1656572403784,1656572413784,1656572423784,1656572433784,1656572443784,1656572453784,1656572463784,1656572473784,1656572483784,1656572493784,1656572503784,1656572513784,1656572523784,1656572533784,1656572543784,1656572553784,1656572563784,1656572573784,1656572583784,1656572593784,1656572603784,1656572613784,1656572623784,1656572633784,1656572643784,1656572653784,1656572663784,1656572673784,1656572683784,1656572693784,1656572703784,1656572713784,1656572723784,1656572733784,1656572743784,1656572753784,1656572763784,1656572773784,1656572783784,1656572793784,1656572803784,1656572813784,1656572823784,1656572833784,1656572843784,1656572853784,1656572863784,1656572873784,1656572883784,1656572893784,1656572903784,1656572913784,1656572923784,1656572933784,1656572943784,1656572953784,1656572963784,1656572973784,1656572983784,1656572993784,1656573003784,1656573013784,1656573023784,1656573033784,1656573043784,1656573053784,1656573063784,1656573073784,1656573083784,1656573093784,1656573103784,1656573113784,1656573123784,1656573133784,1656573143784,1656573153784,1656573163784,1656573173784,1656573183784,1656573193784,1656573203784,1656573213784,1656573223784,1656573233784,1656573243784,1656573253784,1656573263784,1656573273784,1656573283784,1656573293784,1656573303784,1656573313784,1656573323784,1656573333784,1656573343784,1656573353784,1656573363784,1656573373784,1656573383784,1656573393784,1656573403784,1656573413784,1656573423784,1656573433784,1656573443784,1656573453784,1656573463784,1656573473784,1656573483784,1656573493784,1656573503784,1656573513784,1656573523784,1656573533784,1656573543784,1656573553784,1656573563784,1656573573784,1656573583784,1656573593784,1656573603784,1656573613784,1656573623784,1656573633784,1656573643784,1656573653784,1656573663784,1656573673784,1656573683784,1656573693784,1656573703784,1656573713784,1656573723784,1656573733784,1656573743784,1656573753784,1656573763784,1656573773784,1656573783784,1656573793784,1656573803784,1656573813784,1656573823784,1656573833784,1656573843784,1656573853784,1656573863784,1656573873784,1656573883784,1656573893784,1656573903784,1656573913784,1656573923784,1656573933784,1656573943784,1656573953784,1656573963784,1656573973784,1656573983784,1656573993784,1656574003784,1656574013784,1656574023784,1656574033784,1656574043784,1656574053784,1656574063784,1656574073784,1656574083784,1656574093784,1656574103784,1656574113784,1656574123784,1656574133784,1656574143784,1656574153784,1656574163784,1656574173784,1656574183784,1656574193784,1656574203784,1656574213784,1656574223784,1656574233784,1656574243784,1656574253784,1656574263784,1656574273784,1656574283784,1656574293784,1656574303784,1656574313784,1656575253783,1656575263783,1656575273783,1656575283783,1656575293783,1656575303783,1656575313783,1656575323783,1656575333783,1656575343783,1656575353783,1656575363783,1656575373783,1656575383783,1656575393783,1656575403783,1656575413783,1656575423783,1656575433783,1656575443783,1656575453783,1656575463783,1656575473783,1656575483783,1656575493783,1656575503783,1656575513783,1656575523783,1656575533783,1656575543783,1656575553783,1656575563783,1656575573783,1656575583783,1656575593783,1656575603783,1656575613783,1656575623783,1656575633783,1656575643783,1656575653783,1656575663783,1656575673783,1656575683783,1656575693783,1656575703783,1656575713783,1656575723783,1656575733783,1656575743783,1656575753783,1656575763783,1656575773783,1656575783783,1656575793783,1656575803783,1656575813783,1656575823783,1656575833783,1656575843783,1656575853783,1656575863783,1656575873783,1656575883783,1656575893783,1656575903783,1656575913783,1656575923783,1656575933783,1656575943783,1656575953783,1656575963783,1656575973783,1656575983783,1656575993783,1656576003783,1656576013783,1656576023783,1656576033783,1656576043783,1656576053783,1656576063783,1656576073783,1656576083783,1656576093783,1656576103783,1656576113783,1656576123783,1656576133783,1656576143783,1656576153783,1656576163783,1656576173783,1656576183783,1656576193783,1656576203783,1656576213783,1656576223783,1656576233783,1656576243783,1656576253783,1656576263783,1656576273783,1656576283783,1656576293783,1656576303783,1656576313783,1656576323783,1656576333783,1656576343783,1656576353783,1656576363783,1656576373783,1656576383783,1656576393783,1656576403783,1656576413783,1656576423783,1656576433783,1656576443783,1656576453783,1656576463783,1656576473783,1656576483783,1656576493783,1656576503783,1656576513783]}
```
</details>

## Summary

* We have laerned how to remove metrics from VictoriaMetrics
* We have laerned how to update(replace) metrics in VictoriaMetrics