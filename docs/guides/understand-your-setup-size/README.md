This guide is based on capacity planning for [Single-Node](https://docs.victoriametrics.com/single-server-victoriametrics/#capacity-planning),
[Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning)
and [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/) docs.

## Terminology

- [Active Time Series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series) - a [time series](https://docs.victoriametrics.com/keyconcepts/#time-series)
  that was update at least one time during the last hour;
- Ingestion Rate - how many [samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) are ingest into the database per second;
- [Churn Rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) - how frequently a new [time series](https://docs.victoriametrics.com/keyconcepts/#time-series)
  is created. For example, changing pod name in Kubernetes is a common source of time series churn;
- Queries per Second - how many [read queries](https://docs.victoriametrics.com/keyconcepts/#query-data) are executed per second;
- [Retention Period](https://docs.victoriametrics.com/#retention) - for how long data is stored in the database.

### Active Time Series

Time series [exposed](https://docs.victoriametrics.com/keyconcepts/#push-model) by applications on `/metrics` page
during the last 1h are considered as Active Time Series. For example, [Node exporter](https://prometheus.io/docs/guides/node-exporter/)
exposes **1000** time series per instance. Therefore, if you collect metrics from 50 node exporters, the approximate
amount of Active Time Series is **1000 * 50 = 50,000** series.

For Prometheus, get the max number of Active Time Series over last 24h by running the following query:
```metricsql
sum(max_over_time(prometheus_tsdb_head_series[24h]))
```

For VictoriaMetrics, the query will be the following:
```metricsql
sum(max_over_time(vm_cache_entries{type="storage/hour_metric_ids"}[24h]))
```

_Note: if you have more than one Prometheus, you need to run this query across all of them and summarise the results._

For [pushed](https://docs.victoriametrics.com/keyconcepts/#push-model) metrics the math is the same. Find the average
amount of unique [time series](https://docs.victoriametrics.com/keyconcepts/#time-series) pushed by one application
and multiply it by the number of applications.

Applying [replication Factor](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety)
multiplies the number of Active Time Series since each series will be stored ReplicationFactor times.

### Churn Rate

The higher the Churn Rate, the more compute resources are needed for the efficient work of VictoriaMetrics.
Churn rate affects Active Time Series, efficiency of cache, query performance and on-disk compression.
It is recommended to lower the churn rate as much as possible.

High Churn Rate can be caused by using high-volatile labels, such as `client_id`, `url`, `checksum`, `timestamp`, etc.
In Kubernetes, the pod's name is also a volatile label because it changes each time the pod is redeployed.
For example, a service exposes 1000 time series. If we deploy 100 replicas of this service, the total amount of
Active Time Series will be **1000*100 = 100,000**. If we redeploy this service, each replica's pod name will change
and will create a **100,000** of new time series.

To see the Churn Rate in VictoriaMetrics over last 24h use the following query:
```metricsql
sum(increase(vm_new_timeseries_created_total[24h]))
```

The metrics with the highest number of time series can be tracked via VictoriaMetrics [Cardinality Explorer](https://docs.victoriametrics.com/#cardinality-explorer).

### Ingestion Rate

Ingestion rate is how many time series are pulled (scraped) or pushed per second into the database. For example,
if you scrape a service that exposes **1000** time series with an interval of **15s**, the Ingestion Rate would be
**1000/15 = 66** [samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) per second. The more services are
scraped or the lower scrape interval is, the higher would be the Ingestion Rate.

For Prometheus, get the Ingestion Rate by running the following query:
```metricsql
sum(rate(prometheus_tsdb_head_samples_appended_total[24h]))
```

_Note: if you have more than one Prometheus, you need to run this query across all of them and summarise the results._

For VictoriaMetrics, use the following query:
```metricsql
sum(rate(vm_rows_inserted_total[24h]))
```

This query shows how many samples are inserted in VictoriaMetrics before replication.
If you want to know ingestion rate including replication factor, use the following query:
```metricsql
sum(rate(vm_vminsert_metrics_read_total[24h]))
```

### Queries per Second

There are two types of queries **light** and **heavy**:
* queries calculated over 5m intervals or selecting low number of time series are **light**;
* queries calculated over 30d or selecting big number of time series are **heavy**.

The larger the time range and the more series are needed to be scanned - the more heavy and expensive query is.

To scale VictoriaMetrics cluster for high RPS, consider deploying more [vmselect](https://docs.victoriametrics.com/cluster-victoriametrics/#architecture-overview)
replicas (scale horizontally).
To improve latency of **heavy** queries, consider giving more compute resources to vmselects (scale vertically).

### Compute resources

It is hard to predict the amount of compute resources (CPU, Mem) or cluster size only knowing Ingestion Rate and
Active Time Series. The much better approach is to run tests for your type of load (ingestion and reads) and extrapolate
from there.

For example, if you already run [Prometheus](https://docs.victoriametrics.com/#prometheus-setup)
or [Telegraf](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)
for metrics collection then just configure them (or a part of them) to replicate data to VictoriaMetrics. In this way,
you'd have the most precise simulation of your production environment.

For synthetic tests we recommend using [Prometheus benchmark suite](https://github.com/VictoriaMetrics/prometheus-benchmark)
that runs configured number of [Node exporter](https://prometheus.io/docs/guides/node-exporter/) hosts and writes their
metrics to the configured destination. With this suite you can not only test VictoriaMetrics, but also compare it with
any other compatible solution. See [Grafana Mimir and VictoriaMetrics: performance tests](https://victoriametrics.com/blog/mimir-benchmark/).

As a reference, see resource consumption of VictoriaMetrics cluster on our [playground](https://play-grafana.victoriametrics.com/d/oS7Bi_0Wz_vm/victoriametrics-cluster-vm).

### Retention Period/Disk Space

The Retention Period is the number of days or months for storing data. It affects the disk space usage.
The formula for calculating required disk space is the following:
```
Bytes Per Sample * Ingestion rate * Replication Factor * (Retention Period in Seconds +1 Retention Cycle(day or month)) * 1.2 (recommended 20% of dree space for merges ) 
```

The **Retention Cycle** is one **day** or one **month**. If the retention period is higher than 30 days cycle is a month; otherwise day.

On average, sample size requires less or around **1 byte** after compression:
```metricsql
sum(vm_data_size_bytes) / sum(vm_rows{type!~"indexdb.*"})
```
_Please note, High Churn Rate could negatively impact compression efficiency._

Keep at least **20%** of free space for VictoriaMetrics to remain efficient with compression and read performance.


#### Calculation Example

A Kubernetes environment that produces 5k time series per second with 1-year of the Retention Period and ReplicationFactor=2:

`(1 byte-per-sample * 5000 time series * 2 replication factor * 34128000 seconds) * 1.2 ) / 2^30 = 381 GB`

VictoriaMetrics requires additional disk space for the index. The lower Churn Rate, the lower is disk space usage for the index.
Usually, index takes about **20%** of the disk space for storing data. High cardinality setups may use **>50%** of storage size for index.

You can significantly reduce the amount of disk usage by using [Downsampling](https://docs.victoriametrics.com/#downsampling)
and [Retention Filters](https://docs.victoriametrics.com/#retention-filters). These settings are available in VictoriaMetrics Cloud and Enterprise.
See a blog post about [reducing expenses on monitoring](https://victoriametrics.com/blog/reducing-costs-p2/) for more techniques.

### Cluster size

It is [recommended](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-setup) to run many small vmstorage
nodes over a few big vmstorage nodes. This reduces the workload increase on the remaining vmstorage nodes when some of
vmstorage nodes become temporarily unavailable. Prefer giving at least 2 vCPU per each vmstorage node.

In general, the optimal number of vmstorage nodes is between 10 and 50. Please note, while adding more vmstorage nodes
is a straightforward process, decreasing number of vmstorage nodes is a very complex process that should be avoided.

vminsert and vmselect components are stateless, and can be easily scaled up or down. Scale them accordingly to your load.

## Align Terms with VictoriaMetrics setups

### VictoriaMetrics Cloud

Every deployment (Single-Node or Cluster) contains the expected load in Ingestion Rate and Active Time Series.
We assume that the Churn Rate is no more than **30%**. You may need to choose a more extensive deployment if you have a higher Churn Rate.

#### Example

Deployment type: **s.medium ~100k samples/s Ingestion Rate, ~2.5M of Active Time Series**

You can collect metrics from

- 10x Kubernetes cluster with 50 nodes each - 4200 * 10 * 50 = 2.1M
- 500 node exporters - 0.5M
- With metrics collection interval - 30s

### On-Premise

Please follow these capacity planning documents ([Single-Node](https://docs.victoriametrics.com/single-server-victoriametrics/#capacity-planning),
[Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning)). It contains the number of CPUs
and Memory required to handle the Ingestion Rate, Active Time Series, Churn Rate, QPS and Retention Period.
