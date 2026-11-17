---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
This guide is based on capacity planning for [Single-Node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#capacity-planning),
[Cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning)
and [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/) docs.

## Terminology

- [Active Time Series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series) - a [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series)
  that was updated at least once during the last hour;
- Ingestion Rate - how many [samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) are ingested into the database per second;
- [Churn Rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate) - how many new [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) are registered over a period of time, usually measured over the last 24h. For example, changing a pod name in Kubernetes is a common source of time series churn;
- Queries per Second (QPS) - how many [read queries](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data) are executed per second;
- [Retention Period](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention) - for how long data is stored in the database.

### Active Time Series

A time series is considered active if the database received at least one sample during the last hour. This definition applies to both scraped (or pulled) and pushed metrics.

For example, given a [Node exporter](https://prometheus.io/docs/guides/node-exporter/)
that exposes 1000 time series per instance and a total of 50 instances, the approximate number of Active Time series is `1000 * 50 = 50,000`.

For VictoriaMetrics, get the max number of Active Time Series over the last 24h by running the following query:
```metricsql
sum(max_over_time(vm_cache_entries{type="storage/hour_metric_ids"}[24h]))
```

For Prometheus, get the max number of Active Time Series over the last 24h by running the following query:
```metricsql
sum(max_over_time(prometheus_tsdb_head_series[24h]))
```

_Note: if you have more than one Prometheus, you need to run this query across all of them and add up the results._

For [pushed](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model) metrics, the math is the same. Find the average
amount of unique [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) pushed by one application
and multiply it by the number of applications.

[Replication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety) increases the physical series load across vmstorage nodes because samples for each logical series are stored on multiple nodes.

> In a cluster, replication and re-routing can cause the same logical series to be present on multiple vmstorage nodes. Summing the active series across all nodes in the cluster only estimates the physical series load across storage nodes, not the number of unique logical series.

### Churn Rate

The higher the Churn Rate, the more compute resources are needed for VictoriaMetrics to work efficiently.
Churn rate affects Active Time Series, cache efficiency, query performance, and on-disk compression.
It is recommended to lower the churn rate as much as possible.

High Churn Rate can be caused by using high-volatility labels, such as `client_id`, `url`, `checksum`, `timestamp`, etc.
In Kubernetes, the pod's name is also a volatile label because it changes each time the pod is redeployed.

For example, given a service that exposes 1000 time series and is deployed with 100 replicas, this raises the total number of Active Time Series to `1000 * 100 = 100,000`. If we redeploy this service, each replica's pod name changes, creating 100,000 new time series.

To see the Churn Rate in VictoriaMetrics over the last 24h, use the following query:
```metricsql
sum(increase(vm_new_timeseries_created_total[24h]))
```

The metrics with the highest number of time series can be found using the VictoriaMetrics [Cardinality Explorer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-explorer). For monitoring and alerting on cardinality spikes, see [vmestimator](https://docs.victoriametrics.com/victoriametrics/vmestimator/).

### Ingestion Rate

Ingestion rate is how many [samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) are scraped or pushed into the database per second. Every scrape produces one sample per exposed time series. In other words, the Ingestion Rate can be estimated with:

```text
Ingestion Rate ≈ number of currently scraped or pushed series / scrape interval
```

For example, a service exposing 1,000 time series scraped every 15 seconds contributes about 1,000 / 15 ≈ 67 samples per second.

For series sampled at a constant interval, divide the number of currently sampled series by that interval to estimate their ingestion rate. Adding more services or lowering the scrape (or push) interval increases the Ingestion Rate.

For VictoriaMetrics, use the following query to obtain the Ingestion Rate over the last 24 hours:
```metricsql
sum(rate(vm_rows_inserted_total[24h]))
```

The previous query shows how many samples were inserted in VictoriaMetrics before replication.
If you want to know Ingestion Rate including the replication factor, use the following query:
```metricsql
sum(rate(vm_vminsert_metrics_read_total[24h]))
```

For Prometheus, get the Ingestion Rate over the last 24 hours by running the following query:
```metricsql
sum(rate(prometheus_tsdb_head_samples_appended_total[24h]))
```

_Note: if you have more than one Prometheus, you need to run this query across all of them and add up the results._

### Queries per Second

There are two types of queries: light and heavy. For example:
* queries calculated over 5m (5 minutes) intervals or selecting a low number of time series are **light**;
* queries calculated over 30d or selecting a large number of time series are **heavy**.

The larger the time range and the more series are needed to be scanned, the heavier and more expensive the query is.

To scale VictoriaMetrics cluster for high QPS, consider deploying more [vmselect](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview)
replicas (scale horizontally).
To reduce latency for heavy queries, consider giving vmselects more compute resources (scale vertically).

### Compute resources

The Ingestion Rate and Active Time Series alone are not very helpful for estimating cluster size or the compute (CPU and memory) required. The preferred approach is to run tests for your load type (ingestion and reads) and extrapolate from there.

For example, if you already run [Prometheus](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/)
or [Telegraf](https://docs.victoriametrics.com/victoriametrics/data-ingestion/telegraf/) for metrics collection, then just configure them (or a part of them) to replicate data to VictoriaMetrics. In this way,
you'd have the most precise simulation of your production environment.

For synthetic tests, we recommend using the [Prometheus benchmark suite](https://github.com/VictoriaMetrics/prometheus-benchmark)
that runs a configured number of [Node exporter](https://prometheus.io/docs/guides/node-exporter/) hosts and writes their
metrics to the configured destination. With this suite you can not only test VictoriaMetrics, but also compare it with
any other compatible solution. See [Grafana Mimir and VictoriaMetrics: performance tests](https://victoriametrics.com/blog/mimir-benchmark/).

As a reference, see resource consumption of the VictoriaMetrics cluster on our [playground](https://play-grafana.victoriametrics.com/d/oS7Bi_0Wz_vm/victoriametrics-cluster-vm).

### Retention Period/Disk Space

The Retention Period is the number of days or months during which the data is stored and persisted on disk. It directly affects the disk space usage. Use the following formulas to estimate the required disk space:

```
Data size = Bytes Per Sample * Ingestion Rate * Replication Factor * (Retention Period in seconds + 1 month in seconds)
Disk size = (Data size + Index size) * 1.25
```

The `1.25` multiplier keeps at least 20% of free disk space, which VictoriaMetrics needs for background merges and efficient compression.

VictoriaMetrics stores data in monthly partitions. Entire partitions outside the configured retention are deleted on the first day of a new month. Data parts fully outside retention can also be deleted during background merges. For conservative capacity planning, allow for up to one additional month of data beyond the configured retention period.

See [Bytes per sample](#bytes-per-sample) and [Index size](#index-size) below for more information.

> Before v1.133.0, IndexDB retention differed from the per-partition behavior described here. For older deployments, verify the storage and retention behavior for the installed version before applying this estimate.

#### Bytes per sample

On average, a sample takes 1 byte or less on disk after compression. Check the actual value for your setup with the following query, which counts on-disk data parts only and excludes in-memory parts and the index:

```metricsql
sum(vm_data_size_bytes{type=~"storage/(small|big)"}) / sum(vm_rows{type=~"storage/(small|big)"})
```

The size per sample depends on how random the values are. Values with many significant digits compress worse. Limiting precision with `-remoteWrite.significantFigures` or `-remoteWrite.roundDigits` in [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) improves compression. High cardinality and a high Churn Rate also increase the size per sample.

#### Index size

VictoriaMetrics requires additional disk space for the [index](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#indexdb). The index size grows with the number of time series registered in each monthly partition and with the total length of their labels, not with the number of samples. So it depends mostly on the Churn Rate and on label size:

- With a low Churn Rate and short label sets, the index stays small compared to the data.
- In Kubernetes, every pod restart creates a new set of time series with ~30-40 labels and ~1KB of `label=value` pairs per series. In such setups, the index may be up to 2x the size of the data.

Use this query to get the total IndexDB size in bytes:

```metricsql
sum(vm_data_size_bytes{type="indexdb/file"})
```

Measure the index-to-data ratio of your setup with the following query:

```metricsql
sum(vm_data_size_bytes{type="indexdb/file"}) / sum(vm_data_size_bytes{type=~"storage/(small|big)"})
```

The index growth can be reduced by [dropping unneeded labels](https://docs.victoriametrics.com/victoriametrics/relabeling/#how-to-remove-labels-from-metrics-subset), by aggregating series with [stream aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#reducing-the-number-of-stored-series), or, for setups where the set of series rarely changes, by [turning off the per-day index](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#index-tuning-for-low-churn-rate). 

Deduplication and downsampling reduce the number of samples, but not the number of series, so they don't reduce the index size. 

See [FAQ: Why is IndexDB so large?](https://docs.victoriametrics.com/victoriametrics/faq/#why-indexdb-size-is-so-large) for details.

#### Calculation example

Let's assume we have a Kubernetes environment that ingests 5,000 samples per second with a 1-year Retention Period and ReplicationFactor=2.

The Retention Period plus one extra month (30 days) is 365 + 30 = 395 days, or 34,128,000 seconds. Assuming 1 byte per sample, the data size for the whole year is:

```
Data size = 1 byte * 5,000 samples/s * 2 * 34,128,000 s = 341,280,000,000 bytes ≈ 318 GiB
```

Index size depends primarily on the number of indexed time series, the number and length of their labels, and the per-day index entries created for days when those series receive samples. It does not scale directly with the number of samples. See [index size](#index-size) to calculate the size of the index.

For example, if the query shows that the index is about the same size as the data, the total disk size required will be (leaving 25% of headroom free):

```
Disk size = (318 GiB + 318 GiB) * 1.25 ≈ 795 GiB
```

Kubernetes workloads can produce large label sets and frequent series turnover. A production cluster with roughly 30 to 40 labels and around 1 KB of label=value pairs per series can make the IndexDB reach twice the size of the sample data. In the example above, such a large IndexDB would take the total disk usage to:

```text
Disk size = (318 GiB + 636 GiB) * 1.25 ≈ 1,192 GiB
```

If [deduplication](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication) is enabled, make sure the free disk space is at least one month of ingested data: 1 byte * 5,000 samples/s * 2 * 2,592,000 s ≈ 24 GiB. In this example, the 20% headroom (≈ 159 GiB) already covers it. For small setups or short retentions, check it explicitly.

You can significantly reduce disk usage by using [Downsampling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#downsampling) and [Retention Filters](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention-filters). These settings are available in VictoriaMetrics Cloud and Enterprise. Note that downsampling reduces the data size but not the index size, since it doesn't reduce the number of stored time series. See a blog post about [reducing expenses on monitoring](https://victoriametrics.com/blog/reducing-costs-p2/) for more techniques.

### Cluster size

> For ingestion rates lower than a million samples per second, consider the [single-node version](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) first. It is easier to operate and can be set up in [High Availability](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#high-availability) mode. See [which VictoriaMetrics type is recommended](https://docs.victoriametrics.com/victoriametrics/faq/#which-victoriametrics-type-is-recommended-for-use-in-production---single-node-or-cluster).

We [recommend](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup) running many small vmstorage nodes instead of a few big vmstorage nodes. A good lower bound is at least 10 vmstorage nodes. This reduces the workload increase on the remaining nodes when some of them become temporarily unavailable. E.g., losing 1 node out of 3 increases the load on each remaining node by 50%, while losing 1 node out of 10 increases it by only ~11%. 

With [replication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety) enabled, the cluster needs at least `2*N-1` vmstorage nodes to keep replication factor `N` for new data when `N-1` nodes are unavailable.

Once the cluster has 30 or more vmstorage nodes, prefer scaling vertically (adding CPU, RAM and disk to existing nodes) instead of adding more nodes, unless you're limited by CPU, RAM or disk space.

Adding vmstorage nodes is straightforward, but existing data isn't [rebalanced](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#rebalancing) to the new nodes. [Removing vmstorage nodes](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#scaling-down) requires either keeping the removed nodes readable until their data leaves the retention period or migrating their data with [vmctl](https://docs.victoriametrics.com/victoriametrics/vmctl/), which is time- and resource-consuming. Choose the number of vmstorage nodes carefully from the beginning.

vminsert and vmselect components are stateless and can be easily scaled up or down. Adding vmselect nodes increases the maximum query rate, while adding CPU and RAM to vmselect nodes speeds up heavy queries. 

For vminsert, dedicate about 2 GiB of RAM per vCPU. Size vmselect memory according to the expected query workload and the `-search.maxMemoryPerQuery` and `-search.maxConcurrentRequests` limits, and allocate at least 10 GiB of disk space for its local caches. Determine vmstorage CPU and memory requirements by testing with a representative workload. All components read CPU and memory limits only at startup, so restart them after changing their limits instead of resizing them in place.

See also [Capacity planning docs for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning).

## Align Terms with VictoriaMetrics setups

### VictoriaMetrics Cloud

Capacity Tiers size [VictoriaMetrics Cloud](https://victoriametrics.cloud) deployments. Each tier is defined by Active Time Series, Churn Rate, Ingestion Rate, Data Read Rate, and Series Read per Query, using the same terms as this guide. The recommended storage is calculated with the [formula above](#retention-perioddisk-space), assuming 0.8 bytes per sample. Storage can be increased later but not downsized, so it is recommended to start small and grow as needed.

See [VictoriaMetrics Cloud Deployments](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/victoriametrics-tiers/) for the current Capacity Tiers and limits.

### On-Premise

Follow the [single-node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#capacity-planning) or [cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning) capacity-planning documentation for deployment-specific sizing guidance. CPU and memory requirements depend on active time series, ingestion rate, churn rate, query workload, and concurrency. Test with representative ingestion and read workloads, then adjust resources based on observed usage.

