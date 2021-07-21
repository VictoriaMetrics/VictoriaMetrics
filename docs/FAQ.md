---
sort: 14
---

# FAQ

### What is the main purpose of VictoriaMetrics?

To provide the best monitoring solution.


### Who uses VictoriaMetrics?

See [case studies](https://docs.victoriametrics.com/CaseStudies.html).


### Which features does VictoriaMetrics have?

See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prominent-features).


### Are there performance comparisons with other solutions?

Yes. See [these benchmarks](https://docs.victoriametrics.com/Articles.html#benchmarks).


### How to start using VictoriaMetrics?

See [these docs](https://docs.victoriametrics.com/Quick-Start.html).


### Does VictoriaMetrics support replication?

Yes. See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#replication-and-data-safety) for details.


### Can I use VictoriaMetrics instead of Prometheus?

Yes in most cases. VictoriaMetrics can substitute Prometheus in the following aspects:

* Prometheus-compatible service discovery and target scraping can be done with [vmagent](https://docs.victoriametrics.com/vmagent.html) and with single-node VictoriaMetrics - see [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter).
* Prometheus-compatible alerting rules and recording rules can be processed with [vmalert](https://docs.victoriametrics.com/vmalert.html).
* Prometheus-compatible querying in Grafana is supported by VictoriaMetrics. See [these docs](https://docs.victoriametrics.com/#grafana-setup).


### What is the difference between vmagent and Prometheus?

While both [vmagent](https://docs.victoriametrics.com/vmagent.html) and Prometheus may scrape Prometheus targets (aka `/metrics` pages)
according to the provided Prometheus-compatible [scrape configs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config)
and send data to multiple remote storage systems, vmagent has the following additional features:

- vmagent usually requires lower amounts of CPU, RAM and disk IO comparing to Prometheus when scraping big number of targets (more than 1000)
  or targets with big number of exposed metrics.
- vmagent provides independent disk-backed buffers per each configured remote storage (aka `-remoteWrite.url`). This means that slow or temporarily unavailable storage
  doesn't prevent from sending data to healthy storage in parallel. Prometheus uses a single shared buffer for all the configured remote storage systems (aka `remote_write->url`)
  with the hardcoded retention of 2 hours.
- vmagent may accept, relabel and filter data obtained via multiple data ingestion protocols additionally to data scraped from Prometheus targets.
  I.e. it supports both `pull` and `push` protocols for data ingestion.
  See [these docs](https://docs.victoriametrics.com/vmagent.html#features) for details.
- vmagent may be used in different use cases:
  - [IoT and edge monitoring](https://docs.victoriametrics.com/vmagent.html#iot-and-edge-monitoring)
  - [Drop-in replacement for Prometheus](https://docs.victoriametrics.com/vmagent.html#drop-in-replacement-for-prometheus)
  - [Replication and High Availability](https://docs.victoriametrics.com/vmagent.html#replication-and-high-availability)
  - [Relabeling and Filtering](https://docs.victoriametrics.com/vmagent.html#relabeling-and-filtering)
  - [Splitting data streams among multiple systems](https://docs.victoriametrics.com/vmagent.html#splitting-data-streams-among-multiple-systems)
  - [Prometheus remote_write proxy](https://docs.victoriametrics.com/vmagent.html#prometheus-remote_write-proxy)


### Is it safe to enable [remote write](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage) in Prometheus?

Yes. Prometheus continues writing data to local storage after enabling remote write, so all the existing local storage data
and new data is available for querying via Prometheus as usual.

It is recommended using [vmagent](https://docs.victoriametrics.com/vmagent.html) for scraping Prometheus targets
and writing data to VictoriaMetrics.


### How does VictoriaMetrics compare to other remote storage solutions for Prometheus such as [M3 from Uber](https://eng.uber.com/m3/), [Thanos](https://github.com/thanos-io/thanos), [Cortex](https://github.com/cortexproject/cortex), etc.?

VictoriaMetrics is simpler, faster, more cost-effective and it provides [MetricsQL query language](MetricsQL) based on PromQL. The simplicity is twofold:
- It is simpler to configure and operate. There is no need in configuring [sidecars](https://github.com/thanos-io/thanos/blob/master/docs/components/sidecar.md),
  fighting [gossip protocol](https://github.com/improbable-eng/thanos/blob/030bc345c12c446962225221795f4973848caab5/docs/proposals/completed/201809_gossip-removal.md)
  or setting up third-party systems such as [Consul](https://github.com/cortexproject/cortex/issues/157), [Cassandra](https://cortexmetrics.io/docs/production/cassandra/),
  [DynamoDB](https://cortexmetrics.io/docs/production/aws/) or [Memcached](https://cortexmetrics.io/docs/production/caching/).
- VictoriaMetrics has simpler architecture. This means less bugs and more useful features in the long run comparing to competing TSDBs.

See [comparing Thanos to VictoriaMetrics cluster](https://medium.com/@valyala/comparing-thanos-to-victoriametrics-cluster-b193bea1683)
and [Remote Write Storage Wars](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) talk from [PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).

VictoriaMetrics also [uses less RAM than Thanos components](https://github.com/thanos-io/thanos/issues/448).


### What is the difference between VictoriaMetrics and [QuestDB](https://questdb.io/)?

- QuestDB needs more than 20x storage space than VictoriaMetrics. This translates to higher storage costs and slower queries over historical data, which must be read from the disk.
- QuestDB is much harder to setup and operate than VictoriaMetrics. Compare [setup instructions for QuestDB](https://questdb.io/docs/get-started/binaries) to [setup instructions for VictoriaMetrics](https://docs.victoriametrics.com/#how-to-start-victoriametrics).
- VictoriaMetrics provides [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) query language, which is better suited for typical queries over time series data than SQL-like query language provided by QuestDB. See [this article](https://valyala.medium.com/promql-tutorial-for-beginners-9ab455142085) for details.
- Thanks to PromQL support, VictoriaMetrics [can be used as a drop-in replacement for Prometheus in Grafana](https://docs.victoriametrics.com/#grafana-setup), while QuestDB needs full rewrite of existing dashboards in Grafana.
- Thanks to Prometheus remote_write API support, VictoriaMetrics can be used as a long-term storage for Prometheus or for [vmagent](https://docs.victoriametrics.com/vmagent.html), while QuestDB has no integration with Prometheus.
- QuestDB [supports smaller range of popular data ingestion protocols](https://questdb.io/docs/develop/insert-data) compared to VictoriaMetrics (compare to [the list of supported data ingestion protocols for VictoriaMetrics](https://docs.victoriametrics.com/#how-to-import-time-series-data)).
- [VictoriaMetrics supports backfilling (e.g. storing historical data) out of the box](https://docs.victoriametrics.com/#backfilling), while QuestDB provides [very limited support for backfilling](https://questdb.io/blog/2021/05/10/questdb-release-6-0-tsbs-benchmark#the-problem-with-out-of-order-data).


### What is the difference between VictoriaMetrics and [Cortex](https://github.com/cortexproject/cortex)?

VictoriaMetrics is similar to Cortex in the following aspects:
- Both systems accept data from [vmagent](https://docs.victoriametrics.com/vmagent.html) or Prometheus
  via standard [remote_write API](https://prometheus.io/docs/practices/remote_write/), i.e. there is no need in running sidecars
  unlike in [Thanos](https://github.com/thanos-io/thanos) case.
- Both systems support multi-tenancy out of the box. See [the corresponding docs for VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy).
- Both systems support data replication. See [replication in Cortex](https://github.com/cortexproject/cortex/blob/fe56f1420099aa1bf1ce09316c186e05bddee879/docs/architecture.md#hashing) and [replication in VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#replication-and-data-safety).
- Both systems scale horizontally to multiple nodes. See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-resizing-and-scalability) for details.
- Both systems support alerting and recording rules via the corresponding tools such as [vmalert](https://docs.victoriametrics.com/vmalert.html).


The main differences between Cortex and VictoriaMetrics:
- Cortex re-uses Prometheus source code, while VictoriaMetrics is written from scratch.
- Cortex heavily relies on third-party services such as Consul, Memcache, DynamoDB, BigTable, Cassandra, etc.
  This may increase operational complexity and reduce system reliability comparing to VictoriaMetrics' case,
  which doesn't use any external services. Compare [Cortex Architecture](https://github.com/cortexproject/cortex/blob/master/docs/architecture.md)
  to [VictoriaMetrics architecture](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#architecture-overview).
- VictoriaMetrics provides [production-ready single-node solution](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html),
  which is much easier to setup and operate than Cortex cluster.
- Cortex may lose up to 12 hours of recent data on Ingestor failure - see [the corresponding docs](https://github.com/cortexproject/cortex/blob/fe56f1420099aa1bf1ce09316c186e05bddee879/docs/architecture.md#ingesters-failure-and-data-loss).
  VictoriaMetrics may lose only a few seconds of recent data, which isn't synced to persistent storage yet.
  See [this article for details](https://medium.com/@valyala/wal-usage-looks-broken-in-modern-time-series-databases-b62a627ab704).
- Cortex is usually slower and requires more CPU and RAM than VictoriaMetrics. See [this talk from adidas at PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) and [other case studies](https://docs.victoriametrics.com/CaseStudies.html).
- VictoriaMetrics accepts data in multiple popular data ingestion protocols additionally to Prometheus remote_write protocol - InfluxDB, OpenTSDB, Graphite, CSV, JSON, native binary.
  See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-time-series-data) for details.


### What is the difference between VictoriaMetrics and [Thanos](https://github.com/thanos-io/thanos)?

- Thanos re-uses Prometheus source code, while VictoriaMetrics is written from scratch.
- VictoriaMetrics accepts data via [standard remote_write API for Prometheus](https://prometheus.io/docs/practices/remote_write/),
  while Thanos uses non-standard [Sidecar](https://github.com/thanos-io/thanos/blob/master/docs/components/sidecar.md), which must run alongside each Prometheus instance.
- Thanos Sidecar requires disabling data compaction in Prometheus, which may hurt Prometheus performance and increase RAM usage. See [these docs](https://thanos.io/components/sidecar.md/) for more details.
- Thanos stores data in object storage (Amazon S3 or Google GCS), while VictoriaMetrics stores data in block storage
  ([GCP persistent disks](https://cloud.google.com/compute/docs/disks#pdspecs), Amazon EBS or bare metal HDD).
  While object storage is usually less expensive, block storage provides much lower latencies and higher throughput.
  VictoriaMetrics works perfectly with HDD-based block storage - there is no need in using more expensive SSD or NVMe disks in most cases.
- Thanos may lose up to 2 hours of recent data, which wasn't uploaded yet to object storage. VictoriaMetrics may lose only a few seconds of recent data,
  which isn't synced to persistent storage yet. See [this article for details](https://medium.com/@valyala/wal-usage-looks-broken-in-modern-time-series-databases-b62a627ab704).
- VictoriaMetrics provides [production-ready single-node solution](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html),
  which is much easier to setup and operate than Thanos components.
- Thanos may be harder to setup and operate comparing to VictoriaMetrics, since it has more moving parts, which can be connected with less reliable networks.
  See [this article for details](https://medium.com/faun/comparing-thanos-to-victoriametrics-cluster-b193bea1683).
- Thanos is usually slower and requires more CPU and RAM than VictoriaMetrics. See [this talk from adidas at PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).
- VictoriaMetrics accepts data in multiple popular data ingestion protocols additionally to Prometheus remote_write protocol - InfluxDB, OpenTSDB, Graphite, CSV, JSON, native binary.
  See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-time-series-data) for details.


### How does VictoriaMetrics compare to [InfluxDB](https://www.influxdata.com/time-series-platform/influxdb/)?

- VictoriaMetrics requires [10x less RAM](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) and it [works faster](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
- VictoriaMetrics needs lower amounts of storage space than InfluxDB on production data.
- VictoriaMetrics provides better query language - [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) - than InfluxQL or Flux. See [this tutorial](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) for details.
- VictoriaMetrics accepts data in multiple popular data ingestion protocols additionally to InfluxDB - Prometheus remote_write, OpenTSDB, Graphite, CSV, JSON, native binary.
  See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-time-series-data) for details.


### How does VictoriaMetrics compare to [TimescaleDB](https://www.timescale.com/)?

- TimescaleDB insists on using SQL as a query language. While SQL is more powerful than PromQL, this power is rarely required during typical TSDB usage. Real-world queries usually [look clearer and simpler when written in PromQL than in SQL](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085).
- VictoriaMetrics requires [up to 70x less storage space comparing to TimescaleDB](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4) for storing the same amount of time series data. The gap in storage space usage can be lowered from 70x to 3x if [compression in TimescaleDB is properly configured](https://docs.timescale.com/latest/using-timescaledb/compression) (it isn't an easy task in general case :)).
- TimescaleDB is [harder to setup, configure and operate](https://docs.timescale.com/timescaledb/latest/how-to-guides/install-timescaledb/self-hosted/ubuntu/installation-apt-ubuntu/) than VictoriaMetrics (see [how to run VictoriaMetrics](https://docs.victoriametrics.com/#how-to-start-victoriametrics)).
- VictoriaMetrics accepts data in multiple popular data ingestion protocols - InfluxDB, OpenTSDB, Graphite, CSV, while TimescaleDB supports only SQL inserts.


### Does VictoriaMetrics use Prometheus technologies like other clustered TSDBs built on top of Prometheus such as [Thanos](https://github.com/thanos-io/thanos) or [Cortex](https://github.com/cortexproject/cortex)?

No. VictoriaMetrics core is written in Go from scratch by [fasthttp](https://github.com/valyala/fasthttp) [author](https://github.com/valyala).
The architecture is [optimized for storing and querying large amounts of time series data with high cardinality](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac). VictoriaMetrics storage uses [certain ideas from ClickHouse](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282). Special thanks to [Alexey Milovidov](https://github.com/alexey-milovidov).


### What is the pricing for VictoriaMetrics?

The following versions are open source and free:
* [Single-node version](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html).
* [Cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

We provide commercial support for both versions. [Contact us](mailto:info@victoriametrics.com) for the pricing.

The following commercial versions of VictoriaMetrics are planned:
* Managed cluster in the Cloud.
* SaaS version.

[Contact us](mailto:info@victoriametrics.com) for more information on our plans.


### Why VictoriaMetrics doesn't support [Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cremote_read%3E)?

Remote read API requires transferring all the raw data for all the requested metrics over the given time range. For instance,
if a query covers 1000 metrics with 10K values each, then the remote read API had to return `1000*10K`=10M metric values to Prometheus.
This is slow and expensive.
Prometheus remote read API isn't intended for querying foreign data aka `global query view`. See [this issue](https://github.com/prometheus/prometheus/issues/4456) for details.

So just query VictoriaMetrics directly via [Prometheus Querying API](https://docs.victoriametrics.com/#prometheus-querying-api-usage)
or via [Prometheus datasource in Grafana](https://docs.victoriametrics.com/#grafana-setup).


### Does VictoriaMetrics deduplicate data from Prometheus instances scraping the same targets (aka `HA pairs`)?

Yes. See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#deduplication) for details.


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

- [Slack channel](https://slack.victoriametrics.com/)
- [Telegram channel](https://t.me/VictoriaMetrics_en)
- [Google group](https://groups.google.com/forum/#!forum/victorametrics-users)


### Where can I file bugs and feature requests regarding VictoriaMetrics?

File bugs and feature requests [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).


### Where I can find information about multi-tenancy?

See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy). Multitenancy is supported only by [cluster version](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) of VictoriaMetrics.


### How to set a memory limit for VictoriaMetrics components?

All the VictoriaMetrics component provide command-line flags to control the size of internal buffers and caches: `-memory.allowedPercent` and `-memory.allowedBytes` (pass `-help` to any VictoriaMetrics component in order to see the description for these flags). These limits don't take into account additional memory, which may be needed for processing incoming queries. Hard limits may be enforced only by the OS via [cgroups](https://en.wikipedia.org/wiki/Cgroups), Docker (see [these docs](https://docs.docker.com/config/containers/resource_constraints)) or Kubernetes (see [these docs](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers)).

Memory usage for VictoriaMetrics components can be tuned according to the following docs:

* [Capacity planning for single-node VictoriaMetrics](https://docs.victoriametrics.com/#capacity-planning)
* [Capacity planning for cluster VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#capacity-planning)
* [Troubleshooting for vmagent](https://docs.victoriametrics.com/vmagent.html#troubleshooting)
* [Troubleshooting for single-node VictoriaMetrics](https://docs.victoriametrics.com/#troubleshooting)


### How can I run VictoriaMetrics on FreeBSD?

VictoriaMetrics is included in FreeBSD ports, so just install it from there. See [this link](https://www.freebsd.org/cgi/ports.cgi?query=victoria&stype=all).


### Does VictoriaMetrics support Graphite query language?

Yes. See [these docs](https://docs.victoriametrics.com/#graphite-api-usage).
