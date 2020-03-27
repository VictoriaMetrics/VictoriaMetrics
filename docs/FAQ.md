# FAQ

### What is the main purpose of VictoriaMetrics?

To provide the best long-term [remote storage](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage) solution for [Prometheus](https://prometheus.io/).


### Which features does VictoriaMetrics have?

* Supports [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/), so it can be used as Prometheus drop-in replacement in Grafana.
  Additionally, VictoriaMetrics extends PromQL with opt-in [useful features](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL).
* High performance and good scalability for both [inserts](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b)
  and [selects](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4).
  [Outperforms InfluxDB and TimescaleDB by up to 20x](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
* [Uses 10x less RAM than InfluxDB](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) when working with millions of unique time series (aka high cardinality).
* High data compression, so [up to 70x more data points](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4)
  may be crammed into a limited storage comparing to TimescaleDB.
* Optimized for storage with high-latency IO and low iops (HDD and network storage in AWS, Google Cloud, Microsoft Azure, etc). See [graphs from these benchmarks](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b).
* A single-node VictoriaMetrics may substitute moderately sized clusters built with competing solutions such as Thanos, M3DB, Cortex, InfluxDB or TimescaleDB.
  See [vertical scalability benchmarks](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
  and [comparing Thanos to VictoriaMetrics](https://medium.com/@valyala/comparing-thanos-to-victoriametrics-cluster-b193bea1683).
* Easy operation:
  * VictoriaMetrics consists of a single executable without external dependencies.
  * All the configuration is done via explicit command-line flags with reasonable defaults.
  * All the data is stored in a single directory pointed by `-storageDataPath` flag.
  * Easy backups from [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
* Storage is protected from corruption on unclean shutdown (i.e. hardware reset or `kill -9`) thanks to [the storage architecture](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
* Supports metrics' ingestion and backfilling via the following protocols:
  * [Prometheus remote write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
  * [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/)
  * [Graphite plaintext protocol](https://graphite.readthedocs.io/en/latest/feeding-carbon.html) with [tags](https://graphite.readthedocs.io/en/latest/tags.html#carbon)
    if `-graphiteListenAddr` is set.
  * [OpenTSDB put message](http://opentsdb.net/docs/build/html/api_telnet/put.html) if `-opentsdbListenAddr` is set.
* Ideally works with big amounts of time series data from IoT sensors, connected car sensors and industrial sensors.
* Has open source [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).


### Which clients do you target?

The following Prometheus users may be interested in VictoriaMetrics:
- Users who don't want to bother with Prometheus' local storage operational burden - backups, replication, capacity planning, scalability, etc.
- Users with multiple Prometheus instances who want performing arbitrary queries over all the metrics collected by their Prometheus instances (aka `global querying view`).
- Users who want reducing costs for storing huge amounts of time series data.


### How to start using VictoriaMetrics?

Start with [single-node version](Single-server-VictoriaMetrics). It is easy to configure and operate. It should fit the majority of use cases.


### Is it safe to enable [remote write storage](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage) in Prometheus?

Yes. Prometheus continues writing data to local storage after enabling remote storage write, so all the existing local storage data
and new data is available for querying via Prometheus as usual.


### How does VictoriaMetrics compare to other clustered TSDBs on top of Prometheus such as [M3 from Uber](https://eng.uber.com/m3/), [Thanos](https://github.com/thanos-io/thanos), [Cortex](https://github.com/cortexproject/cortex), etc.?

VictoriaMetrics is simpler, faster, more cost-effective and it provides [MetricsQL with useful extensions for PromQL](MetricsQL). The simplicity is twofold:
- It is simpler to configure and operate. There is no need in configuring third-party [sidecars](https://github.com/thanos-io/thanos/blob/master/docs/components/sidecar.md)
  or fighting with [gossip protocol](https://github.com/thanos-io/thanos/blob/master/docs/proposals/completed/201809_gossip-removal.md).
- VictoriaMetrics has simpler architecture, which means less bugs and more useful features in the long run comparing to competing TSDBs.

See [comparing Thanos to VictoriaMetrics cluster](https://medium.com/@valyala/comparing-thanos-to-victoriametrics-cluster-b193bea1683)
and [Remote Write Storage Wars](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) talk from [PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).

VictoriaMetrics also [uses less RAM than Thanos components](https://github.com/thanos-io/thanos/issues/448).


### What is the difference between VictoriaMetrics and [Cortex](https://github.com/cortexproject/cortex)?

VictoriaMetrics is similar to Cortex in the following aspects:
- Both systems accept data from Prometheus via standard [remote_write API](https://prometheus.io/docs/practices/remote_write/),
  i.e. there is no need in running sidecars unlike in [Thanos](https://github.com/thanos-io/thanos) case.
- Both systems support multi-tenancy out of the box. See [the corresponding docs for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md#url-format).

The main differences between Corex and VictoriaMetrics:
- Cortex re-uses Prometheus source code, while VictoriaMetrics is written from scratch.
- Cortex provides [Ruler](https://github.com/cortexproject/cortex/blob/master/docs/architecture.md#ruler) and [Alertmanager](https://github.com/cortexproject/cortex/blob/master/docs/architecture.md#alertmanager) components,
  which are currently missing in VictoriaMetrics. However, these components can be substituted by [Promxy](https://github.com/jacksontj/promxy#how-do-i-use-alertingrecording-rules-in-promxy).
- Cortex heavily relies on third-party services such as Consul, Memcache, DynamoDB, BigTable, Cassandra, etc.
  This may increase operational complexity and reduce system reliability comparing to VictoriaMetrics' case,
  which doesn't use any external services. Compare [Cortex Architecture](https://github.com/cortexproject/cortex/blob/master/docs/architecture.md)
  to [VictoriaMetrics architecture](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md#architecture-overview).
- VictoriaMetrics provides [production-ready single-node solution](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md),
  which is much easier to setup and operate than Cortex cluster.
- Cortex may lose up to 12 hours of recent data on Ingestor failure - see [the corresponding docs](https://github.com/cortexproject/cortex/blob/master/docs/architecture.md#ingesters-failure-and-data-loss).
  VictoriaMetrics may lose only a few seconds of recent data, which isn't synced to persistent storage yet.
  See [this article for details](https://medium.com/@valyala/wal-usage-looks-broken-in-modern-time-series-databases-b62a627ab704).
- Cortex is usually slower and requires more CPU and RAM than VictoriaMetrics. See [this talk from Adidas at PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).


### What is the difference between VictoriaMetrics and [Thanos](https://github.com/thanos-io/thanos)?

- Thanos re-uses Prometheus source code, while VictoriaMetrics is written from scratch.
- Thanos provides [Ruler component](https://github.com/thanos-io/thanos/blob/master/docs/components/rule.md),
  while VictoriaMetrics relies on [Promxy for alerting and recording rules](https://github.com/jacksontj/promxy#how-do-i-use-alertingrecording-rules-in-promxy).
- VictoriaMetrics accepts data via [standard remote_write API for Prometheus](https://prometheus.io/docs/practices/remote_write/),
  while Thanos uses non-standard [Sidecar](https://github.com/thanos-io/thanos/blob/master/docs/components/sidecar.md), which must run alongside each Prometheus instance.
- Thanos Sidecar requires disabling data compaction in Prometheus, which may hurt Prometheus performance and increase RAM usage.
- Thanos stores data on object storage (Amazon S3 or Google GCS), while VictoriaMetrics stores data on block storage (GCP persistent disks, Amazon EBS or bare metal HDD).
- Thanos may lose up to 2 hours of recent data, which wasn't uploaded yet to object storage. VictoriaMetrics may lose only a few seconds of recent data,
  which isn't synced to persistent storage yet. See [this article for details](https://medium.com/@valyala/wal-usage-looks-broken-in-modern-time-series-databases-b62a627ab704).
- Thanos may be harder to setup and operate comparing to VictoriaMetrics, since it has more moving parts, which can be connected with less reliable networks.
  See [this article for details](https://medium.com/faun/comparing-thanos-to-victoriametrics-cluster-b193bea1683).
- Thanos is usually slower and requires more CPU and RAM than VictoriaMetrics. See [this talk from Adidas at PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).


### How does VictoriaMetrics compare to [InfluxDB](https://www.influxdata.com/time-series-platform/influxdb/)?

VictoriaMetrics requires [10x less RAM](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) and it [works faster](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
It is easier to configure and operate. It provides [better query language](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) than InfluxQL or Flux.


### How does VictoriaMetrics compare to [TimescaleDB](https://www.timescale.com/)?

TimescaleDB insists on using SQL as a query language. While SQL is more powerful than PromQL, this power is rarely required during typical TSDB usage. Real-world queries usually [look clearer and simpler when written in PromQL than in SQL](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085).
Additionally, VictoriaMetrics requires [up to 70x less storage space comparing to TimescaleDB](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4) for storing the same amount of time series data.


### Does VictoriaMetrics use Prometheus technologies like other clustered TSDBs built on top of Prometheus such as [M3 from Uber](https://eng.uber.com/m3/), [Thanos](https://github.com/thanos-io/thanos), [Cortex](https://github.com/cortexproject/cortex)?

No. VictoriaMetrics core is written in Go from scratch by [fasthttp](https://github.com/valyala/fasthttp) [author](https://github.com/valyala).
The architecture is [optimized for storing and querying large amounts of time series data with high cardinality](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac). VictoriaMetrics storage uses [certain ideas from ClickHouse](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). Special thanks to [Alexey Milovidov](https://github.com/alexey-milovidov).



### Are there performance comparisons with other solutions?

Yes:

* [Measuring vertical scalability for time series databases: VictoriaMetrics vs InfluxDB vs TimescaleDB](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
* [Measuring insert performance on high-cardinality time series: VictoriaMetrics vs InfluxDB](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893)
* [TSBS benchmark on high-cardinality time series: VictoriaMetrics vs InfluxDB vs TimescaleDB](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b)
* [Standard TSBS benchmark: VictoriaMetrics vs InfluxDB vs TimescaleDB](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4)


### What is the pricing for VictoriaMetrics?

The following versions are open source and free:
* [Single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/Single-server-VictoriaMetrics).
* [Cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

We provide commercial support for both versions. [Contact us](mailto:info@victoriametrics.com) for the pricing.

The following versions are commercial:
* Managed cluster in the Cloud.
* SaaS version.

[Contact us](mailto:info@victoriametrics.com) for the pricing.


### Why VictoriaMetrics doesn't support [Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cremote_read%3E)?

Remote read API requires transferring all the raw data for all the requested metrics over the given time range. For instance,
if a query covers 1000 metrics with 10K values each, then the remote read API had to return `1000*10K`=10M metric values to Prometheus.
This is slow and expensive.
Prometheus remote read API isn't intended for querying foreign data aka `global query view`. See [this issue](https://github.com/prometheus/prometheus/issues/4456) for details.

So just query VictoriaMetrics directly via [Prometheus Querying API](https://prometheus.io/docs/prometheus/latest/querying/api/)
or via [Prometheus datasource in Grafana](http://docs.grafana.org/features/datasources/prometheus/).


### Does VictoriaMetrics deduplicate data from Prometheus instances scraping the same targets (aka `HA pairs`)?

Yes. See [these docs](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#deduplication) for details.


### Where is the source code of VictoriaMetrics?

Source code for the following versions is available in the following places:
* [Single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics)
* [Cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster)


### Does VictoriaMetrics fit for data from IoT sensors and industrial sensors?

VictoriaMetrics is able to handle data from hundreds of millions of IoT sensors and industrial sensors.
It supports [high cardinality data](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b),
perfectly [scales up on a single node](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
and scales horizontally to multiple nodes.


### Where can I ask questions about VictoriaMetrics?

Questions about VictoriaMetrics can be asked via the following channels:

- [Slack channel](http://slack.victoriametrics.com/)
- [Telegram channel](https://t.me/VictoriaMetrics_en)
- [Google group](https://groups.google.com/forum/#!forum/victorametrics-users)


### Where can I file bugs and feature requests regarding VictoriaMetrics?

File bugs and feature requests [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).


### Are you looking for investors?

Yes. [Mail us](mailto:info@victoriametrics.com) if you are interested in.
