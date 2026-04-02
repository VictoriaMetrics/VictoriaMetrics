---
title: Thanos
weight: 5
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-thanos"
    weight: 5
---

`vmctl` provides a dedicated `thanos` mode for migrating data from Thanos blocks to VictoriaMetrics.
This mode supports both raw blocks (resolution=0) and downsampled blocks with all aggregate types.

> **Important:** Before copying Thanos blocks from object storage to local disk, stop the **Thanos Compactor**.
> The Compactor is [not concurrency-safe](https://thanos.io/tip/components/compact.md/) and may compact,
> downsample, or delete blocks while you are copying, resulting in an inconsistent snapshot.
> Thanos Sidecar does not need to be stopped — it only uploads new blocks and does not modify existing ones.

## Migration via Thanos mode

The `thanos` mode reads Thanos snapshot blocks directly from disk, including:
- **Raw blocks** (resolution=0): Original metrics data from Prometheus
- **Downsampled blocks** (resolution=5m or 1h): Aggregated data with count, sum, min, max, and counter aggregates

### Downsampled data handling

When importing downsampled blocks, each aggregate type is imported as a separate metric with resolution and aggregate type suffixes.
For example, a metric `http_requests_total` from a 5-minute downsampled block will be imported as:
- `http_requests_total:5m:count` - number of raw samples aggregated into the 5-minute window
- `http_requests_total:5m:sum` - sum of values in the 5-minute window
- `http_requests_total:5m:min` - minimum value in the 5-minute window
- `http_requests_total:5m:max` - maximum value in the 5-minute window
- `http_requests_total:5m:counter` - cumulative counter value per window, intended for `rate()`/`increase()` calculations over downsampled data

You can filter which aggregate types to import using the `--thanos-aggr-types` flag:
```sh
vmctl thanos --thanos-snapshot thanos-data --vm-addr http://victoria-metrics:8428 --thanos-aggr-types=count --thanos-aggr-types=sum
```

If `--thanos-aggr-types` is not specified, all aggregate types will be imported.

### Migration steps

We assume you're using **Thanos Sidecar** on your Prometheus pods, and that you have a separate Thanos Store installation.
To migrate the data we need to start writing fresh (current) data to VictoriaMetrics, and migrate historical data in background.

#### Current data

1. For now, keep your Thanos Sidecar and Thanos-related Prometheus configuration, but make it to stream metrics to VictoriaMetrics:
    ```yaml
    remote_write:
    - url: http://<victoriametrics-addr>:8428/api/v1/write
    ```
   
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For cluster version use vminsert address:
```
http://<vminsert-addr>:8480/insert/<tenant>/prometheus
```
_Replace `<vminsert-addr>` with the hostname or IP address of vminsert service._

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

2. Check the logs to make sure that Prometheus is sending and VM is receiving.
   In Prometheus, make sure there are no errors. On the VM side, you should see messages like this:
    ```sh
    2020-04-27T18:38:46.474Z info VictoriaMetrics/lib/storage/partition.go:207 creating a partition "2025_04" with smallPartsPath="/victoria-metrics-data/data/small/2025_04", bigPartsPath="/victoria-metrics-data/data/big/2025_04"
    2020-04-27T18:38:46.506Z info VictoriaMetrics/lib/storage/partition.go:222 partition "2025_04" has been created
    ```

3. Within two hours, Prometheus should finish its current data file and hand it off to Thanos Store for long term
   storage.

#### Historical data

Let's assume your data is stored on S3 served by minio. You first need to copy that out to a local filesystem,
then import it into VM using `vmctl thanos` mode.

1. Copy data from minio.
    1. Run the `minio/mc` Docker container.
    1. `mc config host add minio http://minio:9000 accessKey secretKey`, substituting appropriate values for the last 3 items.
    1. `mc cp -r minio/prometheus thanos-data`
    1. When you copy the data from the minio be sure to copy the entire `thanos-data` directory, which contains all the blocks.
       The directory structure should look like this:

    ```sh
      thanos-data
      ├── 01JWS713P2E4MQW7T643GYGD69
      │    ├── chunks
      │    │    └── 000001
      │    ├── index
      │    ├── meta.json
      │    └── tombstones
   ```
   If you have multiple blocks, they will be in the same directory, e.g.:

    ```sh
      thanos-data
      ├── 01JWS713P2E4MQW7T643GYGD69
      ├── 01JWS713P2E4MQW7T643GYGD70
      ├── 01JWS713P2E4MQW7T643GYGD71
      └── ...
    ```
    1. Import data into VictoriaMetrics with `vmctl`.

2. Import using `vmctl`.
    1. Follow the [Quick Start instructions](https://docs.victoriametrics.com/victoriametrics/vmctl/#quick-start) to get `vmctl` on your machine.
    1. Use `thanos` mode to import data:
    ```sh
    vmctl thanos --thanos-snapshot thanos-data --vm-addr http://victoria-metrics:8428
    ```

   The output will look like this:
   ```sh
   Thanos import mode
   2026/03/16 15:52:22 Processing blocks with aggregate types: [count sum min max counter]
   2026/03/16 15:52:22 Found 1 raw blocks and 2 downsampled blocks
   Thanos snapshot stats:
     blocks found: 3;
     blocks skipped by time filter: 0;
     min time: 1735689600000 (2025-01-01T01:00:00+01:00);
     max time: 1735696740001 (2025-01-01T02:59:00+01:00);
     samples: 488;
     series: 12.
   2026/03/16 15:52:22 Processing raw blocks (resolution=0)...
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 480 samples | Total: 1/1 blocks, 4 series, 480 samples
   2026/03/16 15:52:22 Processing downsampled blocks with aggregate type: count
   2026/03/16 15:52:22 Processing 2 blocks for aggregate type: count
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 96 samples | Total: 1/2 blocks, 4 series, 96 samples
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 8 samples | Total: 2/2 blocks, 8 series, 104 samples
   2026/03/16 15:52:22 Processing downsampled blocks with aggregate type: sum
   2026/03/16 15:52:22 Processing 2 blocks for aggregate type: sum
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 96 samples | Total: 1/2 blocks, 4 series, 96 samples
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 8 samples | Total: 2/2 blocks, 8 series, 104 samples
   2026/03/16 15:52:22 Processing downsampled blocks with aggregate type: min
   2026/03/16 15:52:22 Processing 2 blocks for aggregate type: min
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 96 samples | Total: 1/2 blocks, 4 series, 96 samples
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 8 samples | Total: 2/2 blocks, 8 series, 104 samples
   2026/03/16 15:52:22 Processing downsampled blocks with aggregate type: max
   2026/03/16 15:52:22 Processing 2 blocks for aggregate type: max
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 96 samples | Total: 1/2 blocks, 4 series, 96 samples
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 8 samples | Total: 2/2 blocks, 8 series, 104 samples
   2026/03/16 15:52:22 Processing downsampled blocks with aggregate type: counter
   2026/03/16 15:52:22 Processing 2 blocks for aggregate type: counter
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 96 samples | Total: 1/2 blocks, 4 series, 96 samples
   2026/03/16 15:52:22 [Worker 0] Block 01KKS78P: 4 series, 8 samples | Total: 2/2 blocks, 8 series, 104 samples
   2026/03/16 15:52:22 Migration statistics (1 raw blocks, 2 downsampled blocks):
   2026/03/16 15:52:22   raw: 4 series, 480 samples
   2026/03/16 15:52:22   count: 8 series, 104 samples
   2026/03/16 15:52:22   sum: 8 series, 104 samples
   2026/03/16 15:52:22   min: 8 series, 104 samples
   2026/03/16 15:52:22   max: 8 series, 104 samples
   2026/03/16 15:52:22   counter: 8 series, 104 samples
   2026/03/16 15:52:22   total: 44 series, 1000 samples
   2026/03/16 15:52:22 Import finished!
   2026/03/16 15:52:22 VictoriaMetrics importer stats:
     idle duration: 0s;
     time spent while importing: 10.314541ms;
     total samples: 1000;
     samples/s: 96950.51;
     total bytes: 23.5 kB;
     bytes/s: 2.3 MB;
     import requests: 2;
     import requests retries: 0;
   2026/03/16 15:52:22 Total time: 13.729792ms
   ```

### Filtering data

You can filter the data to import using the following flags:
- `--thanos-filter-time-start` - import only data with timestamps after this time (RFC3339 format)
- `--thanos-filter-time-end` - import only data with timestamps before this time (RFC3339 format)
- `--thanos-filter-label` - filter timeseries by label name (e.g., `__name__`)
- `--thanos-filter-label-value` - regular expression to filter label values

Example with time filtering:
```sh
vmctl thanos --thanos-snapshot thanos-data --vm-addr http://victoria-metrics:8428 \
  --thanos-filter-time-start=2024-01-01T00:00:00Z \
  --thanos-filter-time-end=2024-12-31T23:59:59Z
```

Example with label filtering (import only specific metrics):
```sh
vmctl thanos --thanos-snapshot thanos-data --vm-addr http://victoria-metrics:8428 \
  --thanos-filter-label=__name__ \
  --thanos-filter-label-value="http_requests_.*"
```

## Thanos mode flags

| Flag | Description | Default |
|------|-------------|---------|
| `--thanos-snapshot` | Path to Thanos snapshot directory containing raw and/or downsampled blocks | (required) |
| `--thanos-concurrency` | Number of concurrently running snapshot readers | 1 |
| `--thanos-filter-time-start` | Time filter to select timeseries with timestamp equal or higher than provided value (RFC3339 format) | |
| `--thanos-filter-time-end` | Time filter to select timeseries with timestamp equal or lower than provided value (RFC3339 format) | |
| `--thanos-filter-label` | Label name to filter timeseries by | |
| `--thanos-filter-label-value` | Regular expression to filter label values | `.*` |
| `--thanos-aggr-types` | Aggregate types to import from downsampled blocks (count, sum, min, max, counter). Can be specified multiple times. If not set, all types are imported | |

## Remote read protocol

> Migration via remote read protocol allows to fetch data via API. This is usually a resource intensive operation
for Thanos and may be slow or expensive in terms of resources. 

Currently, Thanos doesn't support streaming remote read protocol. It is [recommended](https://thanos.io/tip/thanos/integrations.md/#storeapi-as-prometheus-remote-read)
to use [thanos-remote-read](https://github.com/G-Research/thanos-remote-read). It is a proxy, that allows exposing any Thanos
service (or anything that exposes gRPC StoreAPI e.g. Querier) via Prometheus remote read protocol.

Run the proxy and define the Thanos store address `./thanos-remote-read -store 127.0.0.1:19194`.
It is important to know that `store` flag is Thanos Store API gRPC endpoint.
The proxy exposes port to serve HTTP on `10080 by default`.

The importing process example for local installation of Thanos and single-node VictoriaMetrics(`http://localhost:8428`):
```sh
./vmctl remote-read \
--remote-read-src-addr=http://127.0.0.1:10080 \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://127.0.0.1:8428 \
```

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

On the [thanos-remote-read](https://github.com/G-Research/thanos-remote-read) proxy side you will see logs like:
```sh
ts=2022-10-19T15:05:04.193916Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666180800000 max_time:1666184399999 matchers:<type:RE value:\".*\" > aggregates:RAW "
ts=2022-10-19T15:05:04.468852Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666184400000 max_time:1666187999999 matchers:<type:RE value:\".*\" > aggregates:RAW "
ts=2022-10-19T15:05:04.553914Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666188000000 max_time:1666191364863 matchers:<type:RE value:\".*\" > aggregates:RAW "
```

And when process will finish you will see:
```sh
Split defined times into 8799 ranges to import. Continue? [Y/n]
VM worker 0:↓ 98183 samples/s
VM worker 1:↓ 114640 samples/s
VM worker 2:↓ 131710 samples/s
VM worker 3:↓ 114256 samples/s
VM worker 4:↓ 105671 samples/s
VM worker 5:↓ 124000 samples/s
Processing ranges: 8799 / 8799 [██████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/19 18:05:07 Import finished!
2022/10/19 18:05:07 VictoriaMetrics importer stats:
  idle duration: 52m13.987637229s;
  time spent while importing: 9m1.728983776s;
  total samples: 70836111;
  samples/s: 130759.32;
  total bytes: 2.2 GB;
  bytes/s: 4.0 MB;
  import requests: 356;
  import requests retries: 0;
2022/10/19 18:05:07 Total time: 9m2.607521618s
```

## Configuration

See [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) for more details.

See also general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).
