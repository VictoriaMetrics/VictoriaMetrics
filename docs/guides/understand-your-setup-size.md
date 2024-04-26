---
weight: 9
title: Understand Your Setup Size
menu:
  docs:
    parent: "guides"
    weight: 9
aliases:
- /guides/understand-your-setup-size.html
---
# Understand Your Setup Size

The docs provide a simple and high-level overview of Ingestion Rate, Active Time Series, and Query per Second. These terms are a part of capacity planning ([Single-Node](https://docs.victoriametrics.com/single-server-victoriametrics/#capacity-planning), [Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning)) and [Managed VictoriaMetrics](https://docs.victoriametrics.com/managed-victoriametrics/) pricing.

## Terminology

- [Active Time Series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series) - the [time series](https://docs.victoriametrics.com/keyconcepts/#time-series) that receive at least one sample for the latest hour;
- Ingestion Rate - how many [data points](https://docs.victoriametrics.com/keyconcepts/#raw-samples) you ingest into the database per second;
- [Churn Rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) - how frequently a new time series is registered. For example, in the Kubernetes ecosystem, the pod name is a part of time series labels. And when the pod is re-created, its name changes and affects all the exposed metrics, which results in high cardinality and Churn Rate problems;
- Query per Second - the number of [read queries](https://docs.victoriametrics.com/keyconcepts/#query-data) per second;
- Retention Period - for how long data is stored in the database.

## Calculation

### Active Time Series

Time series exported by your applications during the last 1h are considered as Active Time Series. For example, on average, a [Node exporter](https://prometheus.io/docs/guides/node-exporter/) exposes 1000 time series per instance. Therefore, if you collect metrics from 50 node exporters, the approximate amount of Active Time Series is 1000 * 50 = 50000 series.

If you already use Prometheus, you can get the number of Active Time Series by running the following query:

**`sum(max_over_time(prometheus_tsdb_head_series[24h]))`**

For VictoriaMetrics, the query will be the following:

**`sum(max_over_time(vm_cache_entries{type="storage/hour_metric_ids"}[24h]))`**

_Note: if you have more than one Prometheus, you need to run this query across all of them and summarise the results._

[CollectD](https://collectd.org/) exposes 346 series per host. The number of exposed series heavily depends on the installed plugins (`cgroups`, `conntrack`, `contextswitch`, `CPU`, `df`, `disk`, `ethstat`, `fhcount`, `interface`, `load`, `memory`, `processes`, `python`, `tcpconns`, `write_graphite`)

[Replication Factor](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety) multiplies the number of Active Time Series since each series will be stored ReplicationFactor times.


### Churn Rate

The higher the Churn Rate, the more compute resources are needed for the efficient work of VictoriaMetrics. It is recommended to lower the churn rate as much as possible.

The high Churn Rate is commonly a result of using high-volatile labels, such as `client_id`, `url`, `checksum`, `timestamp`, etc. In Kubernetes, the pod's name is also a volatile label because it changes each time pod is redeployed. For example, a service exposes 1000 time series. If we deploy 100 replicas of the service, the total amount of Active Time Series will be 1000*100 = 100000. If we redeploy the service, each replica's pod name will change, and the number of Active Time Series will double because all the time series will update the pod's name label.

To track the Churn Rate in VictoriaMetrics, use the following query:

**`sum(rate(vm_new_timeseries_created_total))`**


### Ingestion Rate

Ingestion rate is how many time series are pulled (scraped) or pushed per second into the database. For example, if you scrape a service that exposes 1000 time series with an interval of 15s, the Ingestion Rate would be 1000/15 = 66 [samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) per second. The more services you scrape or the lower is scrape interval the higher would be the Ingestion Rate.
For Ingestion Rate calculation, you need to know how many time series you pull or push and how often you save them into VictoriaMetrics. To be more specific, the formula is the Number Of Active Time Series / Metrics Collection Interval.

If you run the Prometheus, you can get the Ingestion Rate by running the following query:

**`sum(rate(prometheus_tsdb_head_samples_appended_total[24h]))`**

_Note: if you have more than one Prometheus, you need to run this query across all of them and summarise the results._

For the VictoriaMetrics, use the following query:

**`sum(rate(vm_rows_inserted_total[24h])) > 0`**

This query shows how many datapoints are inserted into vminsert, i.e. this metric does not take into account the increase in data due to replication.
If you want to know ingestion rate including replication factor, use the following query:

**`sum(rate(vm_vminsert_metrics_read_total[24h])) > 0`**

This query shows how many datapoints are read by vmstorage from vminsert.


### Queries per Second

There are two types of queries light and heavy.

The larger the time range and the more series need to be scanned - the more expensive the query is

Typically, queries calculating the rollups on the last 5m are lightweight.
Queries which calculate metrics over the last 30d are more expensive in terms of resources.

The typical distribution between light vs heavy queries is 80%:20%. So when you see that setup supports X RPS, you can understand the expected distribution.

### Retention Period/Disk Space

The Retention Period is the number of days or months you store the metrics. It affects the disk space. The formula for calculating required disk space is **Replication Factor * Datapoint Size * Ingestion rate * Retention Period in Seconds + Free Space for Merges (20%) + 1 Retention Cycle**.

The Retention Cycle is one day or one month. If the retention period is higher than 30 days cycle is a month; otherwise day.

The typical data point size requires less or around 1 byte of disk space. Keep at least 20% of free space for VictoriaMetrics to remain efficient with compression and read performance..

### Calculation Example

You have a Kubernetes environment that produces 5k time series per second with 1-year of the retention period and Replication Factor 2 in VictoriaMetrics:

`(RF2 * 1 byte/sample * 5000 time series * 34128000 seconds) * 1.2 ) / 2^30 = 381 GB`

VictoriaMetrics requires additional disk space for the index. The lower Churn Rate means lower disk space usage for the index because of better compression.
Usually, the index takes about 20% of the disk space for storing data. High cardinality setups may use >50% of datapoints storage size for index.

You can significantly reduce the amount of disk usage by specifying [Downsampling](https://docs.victoriametrics.com/#downsampling) and [Retention Filters](https://docs.victoriametrics.com/#retention-filters) that are lower than the Retention Period. Both two settings are available in Managed VictoriaMetrics and Enterprise.


## Align Terms with VictoriaMetrics setups

### Managed VictoriaMetrics

Every deployment (Single-Node or Cluster) contains the expected load in Ingestion Rate and Active Time Series. We assume that the Churn Rate is no more than 30%. You may need to choose a more extensive deployment if you have a higher Churn Rate.

#### Example

Deployment type: **s.medium ~100k samples/s Ingestion Rate, ~2.5M of Active Time Series**

You can collect metrics from

- 10x Kubernetes cluster with 50 nodes each - 4200 * 10 * 50 = 2.1M
- 500 node exporters - 0.5M
- With metrics collection interval - 30s

### On-Premise

Please follow these capacity planning documents ([Single-Node](https://docs.victoriametrics.com/single-server-victoriametrics/#capacity-planning), [Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning)). It contains the number of CPUs and Memory required to handle the Ingestion Rate, Active Time Series, Churn Rate, QPS and Retention Period.
