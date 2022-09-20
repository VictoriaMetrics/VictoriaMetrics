# How to delete or replace metrics in VictoriaMetrics 

**Scenario**

Today there are a lot of monitoring software (Enterprise SaaS or self-hosted services) that help users and companies to stay informed and watch what's going on with their servers, devices, machines, applications, etc. Some of this software can collect many different metrics out-of-the-box. Some of these metrics are useful, but some could be unneeded or mistakenly recorded in the database. In such cases, you may need to remove junky metrics or clear up all space after tests. As a time series database, VictoriaMetrics also provides a mechanism for deleting metrics, but for architecture reasons, it is not suitable for frequent use.

This guide will cover deletion and replacing metrics in the [Single-node](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html) and [Cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) versions of [VictoriaMetrics](https://victoriametrics.com).

**Precondition**

We will use:
- [Single-node VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html);
- [Cluster version of VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html);
- [curl](https://curl.se/docs/manual.html)
- [jq](https://stedolan.github.io/jq/)

## How to delete metrics

According to official [documentation](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-delete-time-series), metrics can be deleted via `/api/v1/admin/tsdb/delete_series` for both VictoriaMetrics Single and Cluster. Before deleting we need to check that the required metrics are present in the database.

To do this we need to send a query request with curl to **Single-node VictoriaMetrics** [api](https://docs.victoriametrics.com/url-examples.html#apiv1series):

<div class="with-copy" markdown="1">

```console
curl -s 'http://127.0.0.1:8428/api/v1/series?match[]=process_cpu_cores_available' | jq
```

</div>

The expected output will look like:
```json
{
  "status": "success",
  "data": [
    {
      "__name__": "process_cpu_cores_available",
      "job": "victoriametrics",
      "instance": "victoriametrics:8428"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmagent",
      "instance": "vmagent:8429"
    },
    {
      "__name__": "process_cpu_cores_available",
      "job": "vmalert",
      "instance": "vmalert:8880"
    }
  ]
}
```

To check that metrics are present in **VictoriaMetrics Cluster** run the following command:

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

To delete series, send a POST request to [delete API](https://docs.victoriametrics.com/url-examples.html#apiv1admintsdbdelete_series) with [`match[]=<time-series-selector>`](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) argument. For example:

**Single-node VictoriaMetrics**:

<div class="with-copy" markdown="1">

```console

curl -v http://127.0.0.1:8428/api/v1/admin/tsdb/delete_series?match[]=process_cpu_cores_available' | jq
```

</div>

**Cluster version of VictoriaMetrics**:
<div class="with-copy" markdown="1">

```console
curl -s 'http://127.0.0.1:8481/select/0/prometheus/api/v1/series?match[]=process_cpu_cores_available' | jq
```

</div>

After that all the time series matching the given selector are deleted. Storage space for the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). The background merges may never occur for data from previous months, so storage space won't be freed for historical data. In this case [forced merge](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#forced-merge) may help freeing up storage space.

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

After the merge is complete, the data will be permanently deleted.

## How to replace metrics

By default, VictoriaMetrics doesn't provide a mechanism that can be used to replace or update metrics. This task looks non-trivial, but there is a workaround.

In short, we have to do the following:
- export metrics to a file;
- change the values of metrics in the file and save it;
- delete metrics from a database;
- import saved file to VictoriaMetrics.

### Export metrics

In this example, we will use `node_memory_MemTotal_bytes` metrics that scrapes by [node_exporter](https://github.com/prometheus/node_exporter/) and then vmagent write it to VictoriaMetrics Single and Cluster versions and [JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-export-data-in-json-line-format) to export metrics.

Let's export metrics for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`. To do this, please run the following command:

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

See [How to delete metrics](https://docs.victoriametrics.com/guides/guide-delete-and-replace-metrics.html#How-to-delete-metrics) from the previous paragraph

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