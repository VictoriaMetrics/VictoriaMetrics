---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

## Purpose

Data deletion in VictoriaMetrics should be used only in specific, one-off cases, such as correcting malformed data or satisfying GDPR requirements. VictoriaMetrics architecture is optimized for appending data, not deleting or modifying existing metrics, which can cause a significant performance penalty. As a result, VictoriaMetrics provides a limited API for data deletion.

In addition, the data deletion API is not a reliable way to free up storage. You can use storage more efficiently by:

- Setting up [relabeling](https://docs.victoriametrics.com/victoriametrics/#relabeling) to drop unwanted targets and metrics before they reach storage. See [this post](https://www.robustperception.io/relabelling-can-discard-targets-timeseries-and-alerts/) for more details
- Changing the [retention period](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention) to automatically prune old metrics

### Precondition

- [curl](https://curl.se/docs/manual.html)
- [jq tool](https://stedolan.github.io/jq/)

This guide works with:
- [VictoriaMetrics single node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
- [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/)
- [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/)

## Identify API endpoints {#endpoints}

VictoriaMetrics provides the following endpoints to manage metrics:

- [series](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series): returns series names and their labels
- [export](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export): exports raw samples in JSON line format
- [import](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1import): imports samples in JSON line format
- [delete_series](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series): deletes time series
- [force_merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge): forces data compaction in VictoriaMetrics storage

The [endpoints change depending on whether you are running single-node or cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format
). Use the tables below as a reference.

### Single-node version

Below are the API endpoints for the single-node version of VictoriaMetrics.

| Type            | Endpoint                                                          | 
|-----------------|-------------------------------------------------------------------|
| `series`          | http://localhost:8428/prometheus/api/v1/series                    | 
| `export`          | http://localhost:8428/prometheus/api/v1/export                    | 
| `import`          | http://localhost:8428/prometheus/api/v1/import                    |
| `delete_series`   | http://localhost:8428/prometheus/api/v1/admin/tsdb/delete_series  |
| `force_merge`     | http://localhost:8428/internal/force_merge                        |

The table assumes that:
- You are logged into the machine running the single-node VictoriaMetrics process
- or, if on Kubernetes, that you have port-forwarded the VictoriaMetrics service to `localhost:8428`

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

To select, import, export, and delete series from a VictoriaMetrics cluster, you need to make the API request to the correct service. The table shows the service and its API endpoints for a VictoriaMetrics cluster.

| Type            | Service   | Endpoint                                                                   | 
|-----------------|-----------|----------------------------------------------------------------------------|
| `series`          | vmselect  | http://localhost:8481/select/0/prometheus/api/v1/series                    | 
| `export`          | vmselect  | http://localhost:8481/select/0/prometheus/api/v1/export                    | 
| `import`          | vminsert  | http://localhost:8480/insert/0/prometheus/api/v1/import                    |
| `delete_series`   | vmselect  | http://localhost:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series  |
| `force_merge`     | vmstorage | http://localhost:8482/internal/force_merge                                |


The table assumes that:
- the [Tenant ID](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy) is 0; adjust this value as needed
- You are logged into the machine running the VictoriaMetrics processes
- or, if on Kubernetes, that you have port-forwarded the VictoriaMetrics services to localhost

{{% collapse name="Expand to see how to port-forward the VictoriaMetrics cluster services in Kubernetes" %}}

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

The [delete API](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-delete-time-series) expects a [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) to be supplied. For example:

- `match[]=process_cpu_cores_available` selects the entire time series in VictoriaMetrics (including all label combinations)
- `match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}` selects only the time series with the provided labels

As a first step, query the `series` endpoint to confirm the series selector before deleting anything. For example, if we want to retrieve the `process_cpu_cores_available` series in a single-node VictoriaMetrics, send a GET request as follows:

```sh
curl -s 'http://localhost:8428/prometheus/api/v1/series' -d 'match[]=process_cpu_cores_available' | jq
```

> [!NOTE] Warning
> The response can return many metrics, so be careful with the series selector.

To do the same on the cluster version:

```sh
curl -s 'http://localhost:8481/select/0/prometheus/api/v1/series' -d 'match[]=process_cpu_cores_available' | jq
```

If no records are returned, you should increase the time window (by default, only the last 5 minutes of data are returned). The following example adds `-d 'start=-30d'` to show the last 30 days:

```sh
curl -s 'http://localhost:8428/prometheus/api/v1/series' \
  -d 'match[]=process_cpu_cores_available' \
  -d 'start=-30d' | jq
```

The output should show the matching time series found in VictoriaMetrics:

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
If you are using VictoriaMetrics Cloud, you need to:

- replace the base URL with your [Access Endpoint](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/quickstart/#start-writing-and-reading-data) (e.g., `https://<xxxx>.cloud.victoriametrics.com`)
- add an Authorization Header with your [Access Token](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/quickstart/#start-writing-and-reading-data) 
- modify the endpoint path based on your [deployment type](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/single-or-cluster/) depending on your Cloud deployment type

The following example works with VictoriaMetrics Cloud single:

```sh
curl -s -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://<xxxx>.cloud.victoriametrics.com/prometheus/api/v1/series' \
   -d 'match[]=process_cpu_cores_available' | jq
```


### Delete data

> [!NOTE] Warning
> This operation cannot be undone. Consider [exporting your metrics](#export-metrics) for backup purposes.

Once you have confirmed the time series selector, send a POST request to the `delete_series` endpoint and supply the selector with the format [`match[]=<time-series-selector>`](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors).

For example, to delete the `process_cpu_cores_available` time series in single-node VictoriaMetrics:

```sh
curl -s -X POST 'http://localhost:8428/prometheus/api/v1/admin/tsdb/delete_series' -d 'match[]=process_cpu_cores_available'
```

To do the same on the cluster version:

```sh
curl -s -X POST 'http://localhost:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series' -d 'match[]=process_cpu_cores_available'
```

On VictoriaMetrics Cloud single node, the command is:

```sh
curl -s -X POST -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/admin/tsdb/delete_series' \
   -d 'match[]=process_cpu_cores_available'
```

If the operation was successful, the deleted series will stop being [queryable](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data). 

### Storage

The storage used by the deleted time series isn't freed immediately. There is done during [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282), which may never happen for historical data. In this case, you can trigger a [forced merge](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#forced-merge) to free up storage. After the merge is complete, the data will be permanently deleted from the disk.

To force a merge on VictoriaMetrics single node, run the following command:

```sh
curl -v -X POST http://localhost:8428/internal/force_merge
```

To do the same on the cluster version:

```sh
curl -v -X POST http://localhost:8482/internal/force_merge
```

Forced merging is not available on VictoriaMetrics Cloud. If you need help managing storage after deleting a time series, please contact support.

## How to update metrics

VictoriaMetrics doesn't provide a mechanism for replacing or updating data. As a workaround, you can  take the following actions:

1. [Export time series to a file](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export)
2. Change the values in the exported file
3. [Delete time series from a database](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series)
4. [Import saved file back into VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1import)

### Export metrics

For example, let's export metric for `node_memory_MemTotal_bytes` with labels `instance="node-exporter:9100"` and `job="hostname.com"`.

For the single-node version, run:

```sh
curl -s -X POST -g \
  http://localhost:8428/prometheus/api/v1/export \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

On the cluster version, the command is:

```sh
curl -s -X POST -g \
  http://localhost:8481/select/0/prometheus/api/v1/export \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl
```

On VictoriaMetrics Cloud single, the command is:

```sh
curl -s -X POST -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/export' \
   -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' > data.jsonl

```

You can use [jq](https://stedolan.github.io/jq/download/) to more easily verify the exported data:

```sh
jq < data.jsonl
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
We can replace the value of `node_memory_MemTotal_bytes` from `33604390912` to `17179869184` (from 32Gb to 16Gb) using [sed](https://linux.die.net/man/1/sed) or any other text-processing tool:

```sh
sed -i 's/33604390912/17179869184/g' data.jsonl
```

Check the changes in `data.jsonl`:

```sh
jq < data.jsonl
```

The expected output should look like this:

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

Delete the metrics as explained above in [how to delete metrics](https://docs.victoriametrics.com/guides/guide-delete-or-replace-metrics/#how-to-delete-metrics).

### Import metrics

VictoriaMetrics supports many [ingestion protocols](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data). In this case, we can directly [import from JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-json-line-format).

The next command imports metrics from `data.jsonl` to VictoriaMetrics single node:

```sh
curl -v -X POST http://localhost:8428/prometheus/api/v1/import -T data.jsonl
```

On the cluster version, the command is:

```sh
curl -v -X POST http://localhost:8480/insert/0/prometheus/api/v1/import -T data.jsonl
```

For VictoriaMetrics Cloud single node, the command is:

```sh
curl -s -X POST -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/import' \
   -T data.jsonl
```

Please note that importing data with old timestamps is called **backfilling** and may require resetting caches, as described [here](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling). 

### Check imported metrics

The final step is to validate that the data was imported correctly.

To query the series on a single-node VictoriaMetrics:

```sh
curl -s -X POST -g 'http://localhost:8428/prometheus/api/v1/export' \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' | jq
```

On the cluster version, the command is:

```sh
curl -s -X POST -g 'http://localhost:8481/select/0/prometheus/api/v1/export' \
  -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' | jq
```

On VictoriaMetrics Cloud single node, use:

```sh
curl -s -X POST -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  'https://YOUR_CLOUD_ENDPOINT.victoriametrics.com/prometheus/api/v1/export' \
   -d 'match[]=node_memory_MemTotal_bytes{instance="node-exporter:9100", job="hostname.com"}' | jq
```

The output should look like:

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

## Troubleshooting

If you have problems interacting with the API, try the following:
- Remove the `-s` from the curl command to see any errors
- Add `-v` to the curl command for verbose output
- Check that curl is sending the correct HTTP request: all requests except [series](#endpoints) should use POST
- Check that you are using the correct endpoint and port for your VictoriaMetrics deployment
- On Kubernetes, you might need to port-forward the services in order to reach the API endpoints 

## See also

- [API Examples](https://docs.victoriametrics.com/victoriametrics/url-examples/)
- [Relabeling cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/)
- [Retention period configuration](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention)

