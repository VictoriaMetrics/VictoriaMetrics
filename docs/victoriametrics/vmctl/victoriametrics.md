---
title: VictoriaMetrics
weight: 9
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-victoriametrics" 
    weight: 9
---

The simplest way to migrate data between VictoriaMetrics installations is [to copy data between instances](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#data-migration).
But when simple copy isn't possible (migration between single-node and cluster, or re-sharding) or when data need to be
modified - use `vmctl vm-native` to migrate data.

See `./vmctl vm-native --help` for details and full list of flags.

vmctl uses [native binary protocol](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format)
to migrate data between VictoriaMetrics installations:
* single-node to single-node
* cluster to cluster
* single to cluster and vice versa.

Migration in `vm-native` mode takes two steps:
1. Explore the list of the metrics to migrate via `api/v1/label/__name__/values` API;
1. Migrate explored metrics one-by-one with specified `--vm-concurrency`.

```sh
./vmctl vm-native \
    --vm-native-src-addr=http://127.0.0.1:8481/select/0/prometheus \ # migrate from
    --vm-native-dst-addr=http://localhost:8428 \                     # migrate to
    --vm-native-filter-time-start='2022-11-20T00:00:00Z' \           # starting from
    --vm-native-filter-match='{__name__!~"vm_.*"}'                   # match only metrics without `vm_` prefix
VictoriaMetrics Native import mode

2023/03/02 09:22:02 Initing import process from "http://127.0.0.1:8481/select/0/prometheus/api/v1/export/native" 
                    to "http://localhost:8428/api/v1/import/native" with filter 
        filter: match[]={__name__!~"vm_.*"}
        start: 2022-11-20T00:00:00Z
2023/03/02 09:22:02 Exploring metrics...
Found 9 metrics to import. Continue? [Y/n] 
2023/03/02 09:22:04 Requests to make: 9
Requests to make: 9 / 9 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2023/03/02 09:22:06 Import finished!
2023/03/02 09:22:06 VictoriaMetrics importer stats:
  time spent while importing: 3.632638875s;
  total bytes: 7.8 MB;
  bytes/s: 2.1 MB;
  requests: 9;
  requests retries: 0;
2023/03/02 09:22:06 Total time: 3.633127625s
```

## Time intervals

It is possible to split the migration process into steps based on time via `--vm-native-step-interval` cmd-line flag.
It allows to reduce amount of matched series per each request and significantly reduced the load on installations 
with [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).

Supported values for `--vm-native-step-interval` are: `month`, `week`, `day`, `hour`, `minute`. 
For example, when migrating 1 year of data with `--vm-native-step-interval=month` vmctl will execute it in 12 separate 
requests from the beginning of the interval to its end. 

> To reverse the order set `--vm-native-filter-time-reverse` and migration will start from the newest to the
oldest data. `--vm-native-filter-time-start` is required to be set when using `--vm-native-step-interval`.

It is recommended using default `month` step when migrating the data over the long time intervals. If you hit query
limits on `--vm-native-src-addr` and can't or don't want to change them, try lowering the step interval to `week`, `day` or `hour`.

Usage example:
```sh
./vmctl vm-native \
    --vm-native-src-addr=http://127.0.0.1:8481/select/0/prometheus \ 
    --vm-native-dst-addr=http://localhost:8428 \
    --vm-native-filter-time-start='2022-11-20T00:00:00Z' \
    --vm-native-step-interval=month \
    --vm-native-filter-match='{__name__!~"vm_.*"}'    
VictoriaMetrics Native import mode

2023/03/02 09:18:05 Initing import process from "http://127.0.0.1:8481/select/0/prometheus/api/v1/export/native" to "http://localhost:8428/api/v1/import/native" with filter 
        filter: match[]={__name__!~"vm_.*"}
        start: 2022-11-20T00:00:00Z
2023/03/02 09:18:05 Exploring metrics...
Found 9 metrics to import. Continue? [Y/n] 
2023/03/02 09:18:07 Selected time range will be split into 5 ranges according to "month" step. Requests to make: 45.
```

## Cluster to cluster

Using cluster-to-cluster migration mode helps to migrate all tenants data in a single `vmctl` run.

Cluster-to-cluster uses `/admin/tenants` endpoint {{% available_from "v1.84.0" %}} to discover list of tenants from source cluster.

To use this mode set `--vm-intercluster` flag to `true`, `--vm-native-src-addr` flag to 'http://vmselect:8481/' 
and `--vm-native-dst-addr` value to http://vminsert:8480/:
```sh
  ./vmctl vm-native --vm-native-src-addr=http://127.0.0.1:8481/ \
  --vm-native-dst-addr=http://127.0.0.1:8480/ \
  --vm-native-filter-match='{__name__="vm_app_uptime_seconds"}' \
  --vm-native-filter-time-start='2023-02-01T00:00:00Z' \
  --vm-native-step-interval=day \  
  --vm-intercluster
  
VictoriaMetrics Native import mode
2023/02/28 10:41:42 Discovering tenants...
2023/02/28 10:41:42 The following tenants were discovered: [0:0 1:0 2:0 3:0 4:0]
2023/02/28 10:41:42 Initing import process from "http://127.0.0.1:8481/select/0:0/prometheus/api/v1/export/native" to "http://127.0.0.1:8480/insert/0:0/prometheus/api/v1/import/native" with filter 
        filter: match[]={__name__="vm_app_uptime_seconds"}
        start: 2023-02-01T00:00:00Z for tenant 0:0 
2023/02/28 10:41:42 Exploring metrics...
2023/02/28 10:41:42 Found 1 metrics to import 
2023/02/28 10:41:42 Selected time range will be split into 28 ranges according to "day" step. 
Requests to make for tenant 0:0: 28 / 28 [████████████████████████████████████████████████████████████████████] 100.00%

2023/02/28 10:41:45 Initing import process from "http://127.0.0.1:8481/select/1:0/prometheus/api/v1/export/native" to "http://127.0.0.1:8480/insert/1:0/prometheus/api/v1/import/native" with filter 
        filter: match[]={__name__="vm_app_uptime_seconds"}
        start: 2023-02-01T00:00:00Z for tenant 1:0 
2023/02/28 10:41:45 Exploring metrics...
2023/02/28 10:41:45 Found 1 metrics to import 
2023/02/28 10:41:45 Selected time range will be split into 28 ranges according to "day" step. Requests to make: 28 
Requests to make for tenant 1:0: 28 / 28 [████████████████████████████████████████████████████████████████████] 100.00%

...

2023/02/28 10:42:49 Import finished!
2023/02/28 10:42:49 VictoriaMetrics importer stats:
  time spent while importing: 1m6.714210417s;
  total bytes: 39.7 MB;
  bytes/s: 594.4 kB;
  requests: 140;
  requests retries: 0;
2023/02/28 10:42:49 Total time: 1m7.147971417s
```

## Configuration

Migrating big volumes of data may result in reaching the safety limits on `src` side.
Please verify that `-search.maxExportDuration` and `-search.maxExportSeries` were set with proper values for `src`. 
If hitting the limits, follow the recommendations [here](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format).
If hitting `the number of matching timeseries exceeds...` error, adjust filters to match less time series or
update `-search.maxSeries` command-line flag on vmselect/vmsingle;

Migrating all the metrics from one VM to another may collide with existing application metrics
(prefixed with `vm_`) at destination and lead to confusion when using
[official Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).
To avoid such situation try to filter out VM process metrics via `--vm-native-filter-match='{__name__!~"vm_.*"}'` flag.

Migrating data with overlapping time range or via unstable network can produce duplicates series at destination.
To avoid duplicates set `-dedup.minScrapeInterval=1ms` for `vmselect`/`vmstorage` at the destination.
This will instruct `vmselect`/`vmstorage` to ignore duplicates with identical timestamps. Ignore this recommendation
if you already have `-dedup.minScrapeInterval` set to 1ms or higher values at destination.
 
When migrating data from one VM cluster to another, consider using [cluster-to-cluster mode](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/#cluster-to-cluster).
Or manually specify addresses according to [URL format](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format):
   ```sh
   # Migrating from cluster specific tenantID to single
   --vm-native-src-addr=http://<src-vmselect>:8481/select/0/prometheus
   --vm-native-dst-addr=http://<dst-vmsingle>:8428
    
   # Migrating from single to cluster specific tenantID
   --vm-native-src-addr=http://<src-vmsingle>:8428
   --vm-native-dst-addr=http://<dst-vminsert>:8480/insert/0/prometheus
    
   # Migrating single to single
   --vm-native-src-addr=http://<src-vmsingle>:8428
   --vm-native-dst-addr=http://<dst-vmsingle>:8428
    
   # Migrating cluster to cluster for specific tenant ID
   --vm-native-src-addr=http://<src-vmselect>:8481/select/0/prometheus
   --vm-native-dst-addr=http://<dst-vminsert>:8480/insert/0/prometheus
   ```

When migrating data from VM cluster to Single-node VictoriaMetrics, vmctl will use the `/api/v1/export/native` API of the VM cluster,
which attaches `vm_account_id` and `vm_project_id` labels to each time series. If you don't need to distinguish between tenants
or simply want to remove these labels, try setting the `--vm-native-disable-binary-protocol` flag, which will use the `/api/v1/export` API,
exporting and importing data in JSON format. Deduplication should be enabled at `-vm-native-src-addr` side if needed.

Migrating data from VM cluster which had replication (`-replicationFactor` > 1) enabled won't produce the same amount
of data copies for the destination database, and will result only in creating duplicates. To remove duplicates,
destination database need to be configured with `-dedup.minScrapeInterval=1ms`. To restore the replication factor
the destination `vminsert` component need to be configured with the according `-replicationFactor` value.
See more about replication [here](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety).

`vmctl` supports `--vm-native-src-headers` and `--vm-native-dst-headers` to define headers sent with each request
to the corresponding source address.

`vmctl` supports `--vm-native-disable-http-keep-alive` to allow `vmctl` to use non-persistent HTTP connections to avoid
error `use of closed network connection` when running a heavy export requests.

See general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).

See `./vmctl vm-native --help` for details and full list of flags:

{{% content "vmctl_vm-native_flags.md" %}}