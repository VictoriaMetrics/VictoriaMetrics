---
weight: 7
title: How to delete or replace metrics in VictoriaMetrics
menu:
  docs:
    parent: "guides"
    weight: 7
aliases:
- /guides/guide-delete-or-replace-metrics.html
---
# How to delete or replace metrics in VictoriaMetrics 

Data deletion is an operation people expect a database to have. [VictoriaMetrics](https://victoriametrics.com) supports 
[delete operation](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-delete-time-series) but to a limited extent. Due to implementation details, VictoriaMetrics remains an [append-only database](https://en.wikipedia.org/wiki/Append-only), which perfectly fits the case for storing time series data. But the drawback of such architecture is that it is extremely expensive to mutate the data. Hence, `delete` or `update` operations support is very limited. In this guide, we'll walk through the possible workarounds for deleting or changing already written data in VictoriaMetrics.

### Precondition

- [Single-node VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/);
- [Cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/);
- [curl](https://curl.se/docs/manual.html)
- [jq tool](https://stedolan.github.io/jq/)

## How to delete metrics

_Warning: time series deletion is not recommended to use on a regular basis. Each call to delete API could have a performance penalty. The API was provided for one-off operations to deleting malformed data or to satisfy GDPR compliance._

[Delete API](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-delete-time-series) expects from user to specify [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). So the first thing to do before the deletion is to verify whether the selector matches the correct series.

To check that metrics are present in **VictoriaMetrics Cluster** run the following command:

_Warning: response can return many metrics, so be careful with series selector._


```curl
curl -s 'http://vmselect:8481/select/0/prometheus/api/v1/series?match[]=process_cpu_cores_available' | jq
```


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

When you're sure [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) is correct, send a POST request to [delete API](https://docs.victoriametrics.com/url-examples/#apiv1admintsdbdelete_series) with [`match[]=<time-series-selector>`](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) argument. For example:


```curl
curl -s 'http://vmselect:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series?match[]=process_cpu_cores_available'
```


If operation was successful, the deleted series will stop being [queryable](https://docs.victoriametrics.com/keyconcepts/#query-data). Storage space for the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). The background merges may never occur for data from previous months, so storage space won't be freed for historical data. In this case [forced merge](https://docs.victoriametrics.com/single-server-victoriametrics/#forced-merge) may help freeing up storage space.

To trigger [forced merge](https://docs.victoriametrics.com/single-server-victoriametrics/#forced-merge) on VictoriaMetrics Cluster run the following command:


```curl
curl -v -X POST http://vmstorage:8482/internal/force_merge
```

After the merge is complete, the data will be permanently deleted from the disk.

## How to update metrics

By default, VictoriaMetrics doesn't provide a mechanism for replacing or updating data. As a workaround, take the following actions:

- [export time series to a file](https://docs.victoriametrics.com/url-examples/#apiv1export);
- change the values of time series in the file and save it;
- [delete time series from a database](https://docs.victoriametrics.com/url-examples/#apiv1admintsdbdelete_series);
- [import saved file to VictoriaMetrics](https://docs.victoriametrics.com/url-examples/#apiv1import).

### Export metrics

For example, let's export metric for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`:


```curl
curl -X POST -g http://vmselect:8481/select/0/prometheus/api/v1/export -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```


To check that exported file contains time series we can use [cat](https://man7.org/linux/man-pages/man1/cat.1.html) and [jq](https://stedolan.github.io/jq/download/)


```curl
cat data.jsonl | jq
```


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

```sh
sed -i 's/33604390912/17179869184/g' data.jsonl
```

Let's check the changes in data.jsonl with `cat`:

```sh
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

Victoriametrics supports a lot of [ingestion protocols](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-time-series-data) and we will use [import from JSON line format](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-data-in-json-line-format).

The next command will import metrics from `data.jsonl` to VictoriaMetrics:


```curl
curl -v -X POST http://vminsert:8480/insert/0/prometheus/api/v1/import -T data.jsonl
```

### Check imported metrics


```curl
curl -X POST -g http://vmselect:8481/select/0/prometheus/api/v1/export -d match[]=node_memory_MemTotal_bytes
```


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
