# How to delete or replace metrics in VictoriaMetrics 

Data deletion is an operation people expect a database to have. [VictoriaMetrics](https://victoriametrics.com) also supports 
[delete operation](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-delete-time-series) but to a limited extent. Due to implementation details, VictoriaMetrics remains an [append-only database](https://en.wikipedia.org/wiki/Append-only), which perfectly fits the case for storing time series data. But the drawback of such architecture is that it is extremely expensive to mutate the data. Hence, `delete` or `update` operations support is very limited. In this guide, we'll walk through the possible workarounds for deleting or changing already written data in VictoriaMetrics.

### Precondition

- [Single-node VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html);
- [Cluster version of VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html);
- [curl](https://curl.se/docs/manual.html)
- [jq tool](https://stedolan.github.io/jq/)

## How to delete metrics

_Warning: time series deletion is not recommended to use on a regular basis. Each call to delete API could have a performance penalty. The API was provided for one-off operations for deleting malformed data or tp satisfy GDPR compliance._

[Delete API](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-delete-time-series) expects from user to specify [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). So the first thing to do before the deletion is to verify whether it matches the correct data.

To check that metrics are present in **VictoriaMetrics Cluster** run the following command:

_Warning: the response can return too many metrics, so pleas be careful with series selector_

<div class="with-copy" markdown="1">

```console
curl -s 'http://127.0.0.1:8481/select/0/prometheus/api/v1/series?match[]=process_cpu_cores_available' | jq
```

</div>

The expected output:

```json
{
  "status": "success",
  "isPartial": false,
  "data": [
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmagent",
      "instance": "vmagent:8429"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmalert",
      "instance": "vmalert:8880"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vminsert",
      "instance": "vminsert:8480"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmselect",
      "instance": "vmselect:8481"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmstorage",
      "instance": "vmstorage-1:8482"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmstorage",
      "instance": "vmstorage-2:8482"
    }
  ]
}

```

When you're sure with [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) correctness, send a POST request to [delete API](https://docs.victoriametrics.com/url-examples.html#apiv1admintsdbdelete_series) with [`match[]=<time-series-selector>`](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) argument. For example:

**Single-node VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console

curl -v http://127.0.0.1:8428/api/v1/admin/tsdb/delete_series?match[]=process_cpu_cores_available'
```

</div>

**Cluster version of VictoriaMetrics**:
<div class="with-copy" markdown="1">

```console
curl -s 'http://127.0.0.1:8481/select/0/prometheus/api/v1/series?match[]=process_cpu_cores_available'
```

</div>

If operation was successful, the deleted series will stop being [queryable](https://docs.victoriametrics.com/keyConcepts.html#query-data). Storage space for the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). The background merges may never occur for data from previous months, so storage space won't be freed for historical data. In this case [forced merge](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#forced-merge) may help freeing up storage space.

To trigger `force merge` on Single-node VictoriaMetrics run the following command:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8428/internal/force_merge
```

And for the VictoriaMetrics Cluster version please run:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8482/internal/force_merge
```

After the merge is complete, the data will be permanently deleted from the disk.

## How to replace metrics

By default, VictoriaMetrics doesn't provide a mechanism for replacing or updating data. As a workaround, take the following actions:

In short, we have to do the following:
- [export metrics to a file](https://docs.victoriametrics.com/url-examples.html#apiv1export);
- change the values of metrics in the file and save it;
- [delete metrics from a database](https://docs.victoriametrics.com/url-examples.html#apiv1admintsdbdelete_series);
- [import saved file to VictoriaMetrics](https://docs.victoriametrics.com/url-examples.html#apiv1import).

### Export metrics

In this example, we will use `node_memory_MemTotal_bytes` metrics that scrapes by [node_exporter](https://github.com/prometheus/node_exporter/) and then vmagent write it to VictoriaMetrics Single and Cluster versions and [JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-export-data-in-json-line-format) to export metrics.

For example, let's export metric for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`:

**Single-node VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g http://127.0.0.1:8428/api/v1/export -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

</div>

**Cluster version of VictoriaMetrics**:

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

The expected output will look like:

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
    33604390912
  ],
  "timestamps": [
    1656669031378,
    1656669032378,
    1656669033378,
    1656669034378
  ]
}

```

In this example, we will replace the values of `node_memory_MemTotal_bytes` from `33604390912` to `17179869184` (from 32Gb to 16Gb) via [sed](https://linux.die.net/man/1/sed), but it can be done in any of the available ways.

```console
sed -i 's/33604390912/17179869184/g' data.jsonl
```

Let's check the changes in data.jsonl with `cat`:

```console
cat data.jsonl | jq
```

The expected output will be the next:

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
    17179869184
  ],
  "timestamps": [
    1656669031378,
    1656669032378,
    1656669033378,
    1656669034378
  ]
}
```

### Delete metrics

See [How-to-delete-metrics](https://docs.victoriametrics.com/guides/guide-delete-or-replace-metrics.html#how-to-delete-metrics) from the previous paragraph

### Import metrics

Victoriametrics supports a lot of [ingestion protocols](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-time-series-data) and we will use [import from JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-json-line-format).

The next command will import metrics from `data.jsonl` to VictoriaMetrics:

**Single-node VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8428/api/v1/import -T data.jsonl
```
</div>

**Cluster version of VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console
curl -v -X POST http://127.0.0.1:8480/insert/0/prometheus/api/v1/import -T data.jsonl
```
</div>

### Check imported metrics

**Single-node VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g 'http://127.0.0.1:8428/api/v1/export -d match[]=node_memory_MemTotal_bytes
```

</div>

**Cluster version of VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console
curl -X POST -g http://127.0.0.1:8481/select/0/prometheus/api/v1/export -d match[]=node_memory_MemTotal_bytes
```

</div>

The expected output will look like:

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
    17179869184
  ],
  "timestamps": [
    1656669031378,
    1656669032378,
    1656669033378,
    1656669034378
  ]
}
```

## Summary

* We have learned how to remove metrics from VictoriaMetrics
* We have learned how to update(replace) metrics in VictoriaMetrics