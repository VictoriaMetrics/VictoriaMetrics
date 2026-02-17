---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

TODO: rewrite intro

Data deletion is an operation people expect a database to have. [VictoriaMetrics](https://victoriametrics.com) supports 
[delete operation](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-delete-time-series) but to a limited extent. Due to implementation details, VictoriaMetrics remains an [append-only database](https://en.wikipedia.org/wiki/Append-only), which perfectly fits the case for storing time series data. But the drawback of such architecture is that it is extremely expensive to mutate the data. Hence, `delete` or `update` operations support is very limited. In this guide, we'll walk through the possible workarounds for deleting or changing already written data in VictoriaMetrics.

TODO: link here
URL FORMAT single vs cluster: https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format

### Precondition

- [Single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) or
- [Cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/)
- [curl](https://curl.se/docs/manual.html)
- [jq tool](https://stedolan.github.io/jq/)

## Identify API endpoints

The first step is to identify the API endpoints to interact with your metric data:

- [series](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series): returns series names and their labels
- [export](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export): exports raw samples in JSON line format
- [import](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1import): imports samples in JSON line format
- [delete series](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series): deletes a time series
- [force_merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge): forces data compaction in VictoriaMetrics storage

The endpoints URLs are different for the [single-node] and [cluster] versions. Use the tables below as reference.

### Single-node version

Below are the API endpoints for the single-node VictoriaMetrics version.

| Type            | Endpoint                                                          | 
|-----------------|-------------------------------------------------------------------|
| `series`          | http://localhost:8428/prometheus/api/v1/series                    | 
| `export`          | http://localhost:8428/prometheus/api/v1/export                    | 
| `import`          | http://localhost:8428/prometheus/api/v1/import                    |
| `delete_series`   | http://localhost:8428/prometheus/api/v1/admin/tsdb/delete_series  |
| `force_merge`     | http://localhost:8428/internal/force_merge                        |

The table assumes that:
- you are logged in the machine running the single-node VictoriaMetrics process
- or that you have port-forwarded the single-node VictoriaMetrics service to `localhost:8428`

{{% collapse name="Expand to see how to port-forward the VictoriaMetrics services in Kubernetes" %}}

Find the name of the VictoriaMetrics service:

```sh
kubectl get svc -l app.kubernetes.io/instance=vmsingle

NAME                                      TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE
vmsingle-victoria-metrics-single-server   ClusterIP   None         <none>        8428/TCP   24s
```

Port-forward the service to localhost with:

```sh
kubectl port-forward svc/vmsingle-victoria-metrics-single-server 8428 &
```

{{% /collapse %}}

### Cluster version

To select, import, export, and delete series from a VictoriaMetrics cluster you need to target the correct service.

Below are the API endpoints for VictoriaMetrics cluster version.

| Type            | Service  | Endpoint                                                                             | 
|-----------------|----------|--------------------------------------------------------------------------------------|
| `series`          | vmselect | http://localhost:8481/select/0/prometheus/api/v1/series                    | 
| `export`          | vmselect | http://localhost:8481/select/0/prometheus/api/v1/export                    | 
| `import`          | vminsert | http://localhost:8480/insert/0/prometheus/api/v1/import                     |
| `delete_series`   | vmselect | http://localhost:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series  |
| `force_merge`     | vmstorage | http://localhost:8482/internal/force_merge                                          |


The table assumes that:
- the [Tenant ID] is 0; adjust this value as needed
- you are logged in the machine running the VictoriaMetrics processes
- or that you have port-forwarded the VictoriaMetrics services to localhost

{{% collapse name="Expand to see how to port-forward the VictoriaMetrics services in Kubernetes" %}}

Find the name of the services:

```sh
kubectl get svc -l app.kubernetes.io/instance=vmcluster

NAME                                           TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
vmcluster-victoria-metrics-cluster-vminsert    ClusterIP   10.43.177.139   <none>        8480/TCP                     5d7h
vmcluster-victoria-metrics-cluster-vmselect    ClusterIP   10.43.41.195    <none>        8481/TCP                     5d7h
vmcluster-victoria-metrics-cluster-vmstorage   ClusterIP   None            <none>        8482/TCP,8401/TCP,8400/TCP   5d7h
```

Port-forward the services to localhost:

```sh
kubectl port-forward svc/vmcluster-victoria-metrics-cluster-vminsert 8480 &
kubectl port-forward svc/vmcluster-victoria-metrics-cluster-vmselect 8481 &
kubectl port-forward svc/vmcluster-victoria-metrics-cluster-vmstorage 8482 &
```

{{% /collapse %}}

## How to delete metrics

### Select data to be deleted

> [!NOTE] Warning
> Deletion of a time series is not recommended to use on a regular basis. Each call to delete API could have a performance penalty. The API was provided for one-off operations to deleting malformed data or to satisfy GDPR compliance.


TODO: rewrite this
[Delete API](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-delete-time-series) expects from user to specify [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors). So the first thing to do before the deletion is to verify whether the selector matches the correct series.

> [!NOTE] Warning
> Response can return many metrics, so be careful with series selector.

Query the `series` endpoint to confirm the series selector before deleting anything. For example, if we want to delete the `process_cpu_cores_available` series in a single-node VictoriaMetrics:

```sh
curl -s 'http://localhost:8428/prometheus/api/v1/series' -d 'match[]=node_cpu_seconds_total' | jq
```

To do the same on the cluster version:

```sh
curl -s 'http://localhost:8481/select/0/prometheus/api/v1/series' -d 'match[]=node_cpu_seconds_total' | jq
```

If no records are returned you might need increase the time window (by default, only the data from the last 5 minutes are returned). For example, the following command adds `-d 'start=-30d'` to filter the last 30 days:

```sh
curl -s 'http://localhost:8428/prometheus/api/v1/series' \
  -d 'match[]=node_cpu_seconds_total' \
  -d 'start=-30d' | jq
```

The output should look like this:

```json
{
  "status": "success",
  "isPartial": false,
  "data": [
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
      "instance": "vmstorage:8482"
    }
  ]
}

```
If you are using VictoriaMetrics Cloud, you need to
- replace the base URL with your [Cloud endpoint]
- add an [access token] with read permissions
- use either the single-node or cluster endpoints depending on your Cloud deployment type

The following example applies to VictoriaMetrics Cloud single node:

```sh
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/series' \
   -d 'match[]=node_cpu_seconds_total' | jq
```


### Delete data

> [!NOTE] Warning
> This operation cannot be undone. You might want to [export your metrics](#export-metrics) first for backup purposes.

TODO: rewrite this
When you're sure [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) is correct, send a POST request to [delete API](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series) with [`match[]=<time-series-selector>`](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) argument.

To delete the series on single-node VictoriaMetrics:

```sh
curl -s 'http://localhost:8428/prometheus/api/v1/admin/tsdb/delete_series' -d 'match[]=process_cpu_cores_available'
```

To do the same on the cluster version:

```sh
curl -s 'http://localhost:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series' -d 'match[]=process_cpu_cores_available'
```

On VictoriaMetrics Cloud single node, the command is:

```sh
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/admin/tsdb/delete_series' \
   -d 'match[]=node_cpu_seconds_total'
```

If operation was successful, the deleted series will stop being [queryable](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data). 

Storage space for the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). The background merges may never occur for data from previous months, so storage space won't be freed for historical data. In this case [forced merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge) may help freeing up storage space.

To trigger [forced merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge) on VictoriaMetrics single node, run the following command:

```sh

curl -v -X POST http://localhost:8428/internal/force_merge
```

To do the same on the cluster version:

```sh
curl -v -X POST http://localhost:8482/internal/force_merge
```

And on VictoriaMetrics Cloud single node:

```sh
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/internal/force_merge' 
```

After the merge is complete, the data will be permanently deleted from the disk.

## How to update metrics

By default, VictoriaMetrics doesn't provide a mechanism for replacing or updating data. As a workaround, take the following actions:

1. [export time series to a file](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export)
2. change the values of time series in the file and save it
3. [delete time series from a database](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series)
4. [import saved file to VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1import)

### Export metrics

For example, let's export metric for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`.

For the single-node version, run:

```sh
curl -X POST -g \
  http://localhost:8428/prometheus/api/v1/export \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

On the cluster version, the command is:

```sh
curl -X POST -g \
  http://localhost:8481/select/0/prometheus/api/v1/export \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

To check that exported file contains time series we can use [cat](https://man7.org/linux/man-pages/man1/cat.1.html) and [jq](https://stedolan.github.io/jq/download/):

```sh
cat data.jsonl | jq
```

The expected output will look like the following:

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

In this example, we will replace the values of `node_memory_MemTotal_bytes` from `33604390912` to `17179869184` (from 32Gb to 16Gb) via [sed](https://linux.die.net/man/1/sed), but it can be done in any of the available ways:

```sh
sed -i 's/33604390912/17179869184/g' data.jsonl
```

Let's check the changes in data.jsonl with `cat`:

```sh
cat data.jsonl | jq
```

The expected output will be the following:

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

Delete the metrics as explained above in [How to delete metrics](https://docs.victoriametrics.com/guides/guide-delete-or-replace-metrics/#how-to-delete-metrics).

### Import metrics

VictoriaMetrics supports a lot of [ingestion protocols](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data) and we will use [import from JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-json-line-format).

The next command imports metrics from `data.jsonl` to VictoriaMetrics single node:

```sh
curl -v -X POST http://localhost:8428/prometheus/api/v1/import -T data.jsonl
```

On the cluster version, the command is:

```sh
curl -v -x post http://localhost:8480/insert/0/prometheus/api/v1/import -t data.jsonl
```

For VictoriaMetrics Cloud single node, the command is:

```sh
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/import' \
   -t data.jsonl
```

Please note, importing data with old timestamps is called **backfilling** and may require resetting caches as described [here](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling). 

### Check imported metrics

The final step is to validate that the data was imported correctly.

To query the series on a single-node VictoriaMetrics:

```sh
curl -X POST -g 'http://localhost:8428/prometheus/api/v1/export' \
  -d 'match[]=node_memory_MemTotal_bytes' | jq
```

On the cluster version, the command is:

```sh
curl -X POST -g 'http://localhost:8481/select/0/prometheus/api/v1/export' \
  -d 'match[]=node_memory_MemTotal_bytes' | jq
```

On VictoriaMetrics Cloud single node, use:

```sh
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/export' \
   -d 'match[]=node_memory_MemTotal_bytes' | jq
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

## See also

TODO: write
