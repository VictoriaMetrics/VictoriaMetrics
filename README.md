[![Latest Release](https://img.shields.io/github/release/VictoriaMetrics/VictoriaMetrics.svg?style=flat-square)](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
[![GitHub license](https://img.shields.io/github/license/VictoriaMetrics/VictoriaMetrics.svg)](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/VictoriaMetrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/VictoriaMetrics)
[![Build Status](https://travis-ci.org/VictoriaMetrics/VictoriaMetrics.svg?branch=master)](https://travis-ci.org/VictoriaMetrics/VictoriaMetrics)
[![codecov](https://codecov.io/gh/VictoriaMetrics/VictoriaMetrics/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/VictoriaMetrics)

<img alt="Victoria Metrics" src="logo.png">

## Single-node VictoriaMetrics

VictoriaMetrics is fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.
It is available in [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases),
[docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/) and
in [source code](https://github.com/VictoriaMetrics/VictoriaMetrics).

Cluster version is available [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).


## Prominent features

* Supports [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/), so it can be used as Prometheus drop-in replacement in Grafana.
  Additionally, VictoriaMetrics extends PromQL with opt-in [useful features](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/ExtendedPromQL).
* Global query view. Multiple Prometheus instances may write data into VictoriaMetrics. Later this data may be used in a single query.
* High performance and good scalability for both [inserts](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b)
  and [selects](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4).
  [Outperforms InfluxDB and TimescaleDB by up to 20x](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
* [Uses 10x less RAM than InfluxDB](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) when working with millions of unique time series (aka high cardinality).
* High data compression, so [up to 70x more data points](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4)
  may be crammed into a limited storage comparing to TimescaleDB.
* Optimized for storage with high-latency IO and low iops (HDD and network storage in AWS, Google Cloud, Microsoft Azure, etc). See [graphs from these benchmarks](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b).
* A single-node VictoriaMetrics may substitute moderately sized clusters built with competing solutions such as Thanos, Uber M3, Cortex, InfluxDB or TimescaleDB.
  See [vertical scalability benchmarks](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
  and [comparing Thanos to VictoriaMetrics cluster](https://medium.com/@valyala/comparing-thanos-to-victoriametrics-cluster-b193bea1683).
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
* Ideally works with big amounts of time series data from Kubernetes, IoT sensors, connected cars and industrial telemetry.
* Has open source [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).


## Operation


### Table of contents

  - [How to start VictoriaMetrics](#how-to-start-victoriametrics)
  - [Prometheus setup](#prometheus-setup)
  - [Grafana setup](#grafana-setup)
  - [How to upgrade VictoriaMetrics?](#how-to-upgrade-victoriametrics)
  - [How to apply new config to VictoriaMetrics?](#how-to-apply-new-config-to-victoriametrics)
  - [How to send data from InfluxDB-compatible agents such as Telegraf?](#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)
  - [How to send data from Graphite-compatible agents such as StatsD?](#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
  - [Querying Graphite data](#querying-graphite-data)
  - [How to send data from OpenTSDB-compatible agents?](#how-to-send-data-from-opentsdb-compatible-agents)
  - [How to build from sources](#how-to-build-from-sources)
    - [Development build](#development-build)
    - [Production build](#production-build)
    - [ARM build](#arm-build)
    - [Pure Go build (CGO_ENABLED=0)](#pure-go-build-cgo_enabled0)
    - [Building docker images](#building-docker-images)
  - [Start with docker-compose](#start-with-docker-compose)
  - [Setting up service](#setting-up-service)
  - [Third-party contributions](#third-party-contributions)
  - [How to work with snapshots?](#how-to-work-with-snapshots)
  - [How to delete time series?](#how-to-delete-time-series)
  - [How to export time series?](#how-to-export-time-series)
  - [Federation](#federation)
  - [Capacity planning](#capacity-planning)
  - [High availability](#high-availability)
  - [Multiple retentions](#multiple-retentions)
  - [Downsampling](#downsampling)
  - [Multi-tenancy](#multi-tenancy)
  - [Scalability and cluster version](#scalability-and-cluster-version)
  - [Alerting](#alerting)
  - [Security](#security)
  - [Tuning](#tuning)
  - [Monitoring](#monitoring)
  - [Troubleshooting](#troubleshooting)
- [Contacts](#contacts)
- [Community and contributions](#community-and-contributions)
- [Reporting bugs](#reporting-bugs)
- [Victoria Metrics Logo](#victoria-metrics-logo)
  - [Logo Usage Guidelines](#logo-usage-guidelines)
    - [Font used:](#font-used)
    - [Color Palette:](#color-palette)
  - [We kindly ask:](#we-kindly-ask)


### How to start VictoriaMetrics

Just start VictoriaMetrics [executable](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
or [docker image](https://hub.docker.com/r/victoriametrics/victoria-metrics/) with the desired command-line flags.

The following command line flags are used the most:

* `-storageDataPath` - path to data directory. VictoriaMetrics stores all the data in this directory.
* `-retentionPeriod` - retention period in months for the data. Older data is automatically deleted.
* `-httpListenAddr` - TCP address to listen to for http requests. By default it listens port `8428` on all the network interfaces.
* `-graphiteListenAddr` - TCP and UDP address to listen to for Graphite data. By default it is disabled.
* `-opentsdbListenAddr` - TCP and UDP address to listen to for OpenTSDB data. By default it is disabled.

Pass `-help` to see all the available flags with description and default values.


### Prometheus setup

Add the following lines to Prometheus config file (it is usually located at `/etc/prometheus/prometheus.yml`):

```yml
remote_write:
  - url: http://<victoriametrics-addr>:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
      max_shards: 100
```

Substitute `<victoriametrics-addr>` with the hostname or IP address of VictoriaMetrics.
Then apply the new config via the following command:

```
kill -HUP `pidof prometheus`
```

Prometheus writes incoming data to local storage and replicates it to remote storage in parallel.
This means the data remains available in local storage for `--storage.tsdb.retention.time` duration
even if remote storage is unavailable.

If you plan sending data to VictoriaMetrics from multiple Prometheus instances, then add the following lines into `global` section
of [Prometheus config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file):

```yml
global:
  external_labels:
    datacenter: dc-123
```

This instructs Prometheus to add `datacenter=dc-123` label to each time series sent to remote storage.
The label name may be arbitrary - `datacenter` is just an example. The label value must be unique
across Prometheus instances, so time series may be filtered and grouped by this label.


It is recommended upgrading Prometheus to [v2.10.0](https://github.com/prometheus/prometheus/releases) or newer,
since the previous versions may have issues with `remote_write`.


### Grafana setup

Create [Prometheus datasource](http://docs.grafana.org/features/datasources/prometheus/) in Grafana with the following Url:

```
http://<victoriametrics-addr>:8428
```

Substitute `<victoriametrics-addr>` with the hostname or IP address of VictoriaMetrics.

Then build graphs with the created datasource using [Prometheus query language](https://prometheus.io/docs/prometheus/latest/querying/basics/).
VictoriaMetrics supports native PromQL and [extends it with useful features](ExtendedPromQL).


### How to upgrade VictoriaMetrics?

It is safe upgrading VictoriaMetrics to new versions unless [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
say otherwise. It is recommended performing regular upgrades to the latest version,
since it may contain important bug fixes, performance optimizations or new features.

Follow the following steps during the upgrade:

1) Send `SIGINT` signal to VictoriaMetrics process in order to gracefully stop it.
2) Wait until the process stops. This can take a few seconds.
3) Start the upgraded VictoriaMetrics.


### How to apply new config to VictoriaMetrics?

VictoriaMetrics must be restarted for applying new config:

1) Send `SIGINT` signal to VictoriaMetrics process in order to gracefully stop it.
2) Wait until the process stops. This can take a few seconds.
3) Start VictoriaMetrics with new config.


### How to send data from InfluxDB-compatible agents such as [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/)?

Just use `http://<victoriametric-addr>:8428` url instead of InfluxDB url in agents' configs.
For instance, put the following lines into `Telegraf` config, so it sends data to VictoriaMetrics instead of InfluxDB:

```
[[outputs.influxdb]]
  urls = ["http://<victoriametrics-addr>:8428"]
```

Do not forget substituting `<victoriametrics-addr>` with the real address where VictoriaMetrics runs.

VictoriaMetrics maps Influx data using the following rules:
* [`db` query arg](https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint) is mapped into `db` label value.
* Field names are mapped to time series names prefixed with `{measurement}{separator}` value,
  where `{separator}` equals to `_` by default. It can be changed with `-influxMeasurementFieldSeparator` command-line flag.
  See also `-influxSkipSingleField` command-line flag.
* Field values are mapped to time series values.
* Tags are mapped to Prometheus labels as-is.

For example, the following Influx line:

```
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following Prometheus data points:

```
foo.field1{tag1="value1", tag2="value2"} 12
foo.field2{tag1="value1", tag2="value2"} 40
```

Example for writing data with [Influx line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/)
to local VictoriaMetrics using `curl`:

```
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://localhost:8428/write'
```

Arbitrary number of lines delimited by '\n' may be sent in a single request.
After that the data may be read via [/api/v1/export](#how-to-export-time-series) endpoint:

```
curl -G 'http://localhost:8428/api/v1/export' --data-urlencode 'match={__name__!=""}'
```

The `/api/v1/export` endpoint should return the following response:

```
{"metric":{"__name__":"measurement.field1","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560272508147]}
{"metric":{"__name__":"measurement.field2","tag1":"value1","tag2":"value2"},"values":[1.23],"timestamps":[1560272508147]}
```


### How to send data from Graphite-compatible agents such as [StatsD](https://github.com/etsy/statsd)?

1) Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command line flag. For instance,
the following command will enable Graphite receiver in VictoriaMetrics on TCP and UDP port `2003`:

```
/path/to/victoria-metrics-prod -graphiteListenAddr=:2003
```

2) Use the configured address in Graphite-compatible agents. For instance, set `graphiteHost`
to the VictoriaMetrics host in `StatsD` configs.


Example for writing data with Graphite plaintext protocol to local VictoriaMetrics using `nc`:

```
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```

VictoriaMetrics sets the current time if timestamp is omitted.
Arbitrary number of lines delimited by `\n` may be sent in one go.
After that the data may be read via [/api/v1/export](#how-to-export-time-series) endpoint:

```
curl -G 'http://localhost:8428/api/v1/export' --data-urlencode 'match={__name__!=""}'
```

The `/api/v1/export` endpoint should return the following response:

```
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277406000]}
```


### Querying Graphite data

Data sent to VictoriaMetrics via `Graphite plaintext protocol` may be read either via
[Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/)
or via [go-graphite/carbonapi](https://github.com/go-graphite/carbonapi/blob/master/cmd/carbonapi/carbonapi.example.prometheus.yaml).



### How to send data from OpenTSDB-compatible agents?

1) Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command line flag. For instance,
the following command will enable OpenTSDB receiver in VictoriaMetrics on TCP and UDP port `4242`:

```
/path/to/victoria-metrics-prod -opentsdbListenAddr=:4242
```

2) Send data to the given address from OpenTSDB-compatible agents.


Example for writing data with OpenTSDB protocol to local VictoriaMetrics using `nc`:

```
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

Arbitrary number of lines delimited by `\n` may be sent in one go.
After that the data may be read via [/api/v1/export](#how-to-export-time-series) endpoint:

```
curl -G 'http://localhost:8428/api/v1/export' --data-urlencode 'match={__name__!=""}'
```

The `/api/v1/export` endpoint should return the following response:

```
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277292000]}
```


### How to build from sources

We recommend using either [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) or
[docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/) instead of building VictoriaMetrics
from sources. Building from sources is reasonable when developing an additional features specific
to your needs.


#### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make victoria-metrics` from the root folder of the repository.
   It will build `victoria-metrics` binary and put it into the `bin` folder.

#### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make victoria-metrics-prod` from the root folder of the repository.
   It will build `victoria-metrics-prod` binary and put it into the `bin` folder.

#### ARM build

ARM builds may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make victoria-metrics-arm` or `make victoria-metrics-arm64` from the root folder of the repository.
   It will build `victoria-metrics-arm` or `victoria-metrics-arm64` binary respectively and put it into the `bin` folder.

#### Pure Go build (CGO_ENABLED=0)

`Pure Go` mode builds only Go code without [cgo](https://golang.org/cmd/cgo/) dependencies.
This is experimental mode, which may result in lower compression ratio and slower decompression performance.
Use it with caution!

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make victoria-metrics-pure` from the root folder of the repository.
   It will build `victoria-metrics-pure` binary and put it into the `bin` folder.

#### Building docker images

Run `make package-victoria-metrics`. It will build `victoriametrics/victoria-metrics:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-victoria-metrics`.


### Start with docker-compose

[Docker-compose](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/docker-compose.yml)
helps to spin up VictoriaMetrics, Prometheus and Grafana with one command.
More details may be found [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#folder-contains-basic-images-and-tools-for-building-and-running-victoria-metrics-in-docker).


### Setting up service

Read [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/43) on how to set up VictoriaMetrics as a service in your OS.


### Third-party contributions

* [Unofficial yum repository](https://copr.fedorainfracloud.org/coprs/antonpatsev/VictoriaMetrics/) ([source code](https://github.com/patsevanton/victoriametrics-rpm))


### How to work with snapshots?

VictoriaMetrics is able to create [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for all the data stored under `-storageDataPath` directory.
Navigate to `http://<victoriametrics-addr>:8428/snapshot/create` in order to create an instant snapshot.
The page will return the following JSON response:

```
{"status":"ok","snapshot":"<snapshot-name>"}
```

Snapshots are created under `<-storageDataPath>/snapshots` directory, where `<-storageDataPath>`
is the command-line flag value. Snapshots can be archived to backup storage via `cp -L`, `rsync -L`, `scp -r`
or any similar tool that follows symlinks during copying.

The `http://<victoriametrics-addr>:8428/snapshot/list` page contains the list of available snapshots.

Navigate to `http://<victoriametrics-addr>:8428/snapshot/delete?snapshot=<snapshot-name>` in order
to delete `<snapshot-name>` snapshot.

Navigate to `http://<victoriametrics-addr>:8428/snapshot/delete_all` in order to delete all the snapshots.

Steps for restoring from a snapshot:
1. Stop VictoriaMetrics with `kill -INT`.
2. Remove the entire contents of the directory pointed by `-storageDataPath` command-line flag.
3. Copy snapshot contents to the directory pointed by `-storageDataPath`.
4. Start VictoriaMetrics.


### How to delete time series?

Send a request to `http://<victoriametrics-addr>:8428/api/v1/admin/tsdb/delete_series?match[]=<timeseries_selector_for_delete>`,
where `<timeseries_selector_for_delete>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to delete. After that all the time series matching the given selector are deleted. Storage space for
the deleted time series isn't freed instantly - it is freed during subsequent merges of data files.


### How to export time series?

Send a request to `http://<victoriametrics-addr>:8428/api/v1/export?match[]=<timeseries_selector_for_export>`,
where `<timeseries_selector_for_export>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to export. The response would contain all the data for the selected time series in [JSON streaming format](https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON).
Each JSON line would contain data for a single time series. An example output:

```
{"metric":{"__name__":"up","job":"node_exporter","instance":"localhost:9100"},"values":[0,0,0],"timestamps":[1549891472010,1549891487724,1549891503438]}
{"metric":{"__name__":"up","job":"prometheus","instance":"localhost:9090"},"values":[1,1,1],"timestamps":[1549891461511,1549891476511,1549891491511]}
```

Optional `start` and `end` args may be added to the request in order to limit the time frame for the exported data. These args may contain either
unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values.


### Federation

VictoriaMetrics exports [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/)
at `http://<victoriametrics-addr>:8428/federate?match[]=<timeseries_selector_for_federation>`.

Optional `start` and `end` args may be added to the request in order to scrape the last point for each selected time series on the `[start ... end]` interval.
`start` and `end` may contain either unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values. By default the last point
on the interval `[now - max_lookback ... now]` is scraped for each time series. Default value for `max_lookback` is `5m` (5 minutes), but can be overridden.
For instance, `/federate?match[]=up&max_lookback=1h` would return last points on the `[now - 1h ... now]` interval. This may be useful for time series federation
with scrape intervals exceeding `5m`.


### Capacity planning

Rough estimation of the required resources for ingestion path:

* RAM size: less than 1KB per active time series. So, ~1GB of RAM is required for 1M active time series.
  Time series is considered active if new data points have been added to it recently or if it has been recently queried.
  The number of active time series may be obtained from `vm_cache_entries{type="storage/hour_metric_ids"}` metric
  exproted on the `/metrics` page.
  VictoriaMetrics stores various caches in RAM. Memory size for these caches may be limited by `-memory.allowedPercent` flag.

* CPU cores: a CPU core per 300K inserted data points per second. So, ~4 CPU cores are required for processing
  the insert stream of 1M data points per second. The ingestion rate may be lower for high cardinality data.
  See [this article](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) for details.
  If you see lower numbers per CPU core, then it is likely active time series info doesn't fit caches,
  so you need more RAM for lowering CPU usage.

* Storage space: less than a byte per data point on average. So, ~260GB is required for storing a month-long insert stream
  of 100K data points per second.
  The actual storage size heavily depends on data randomness (entropy). Higher randomness means higher storage size requirements.
  Read [this article](https://medium.com/faun/victoriametrics-achieving-better-compression-for-time-series-data-than-gorilla-317bc1f95932)
  for details.

* Network usage: outbound traffic is negligible. Ingress traffic is ~100 bytes per ingested data point via
  [Prometheus remote_write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write).
  The actual ingress bandwitdh usage depends on the average number of labels per ingested metric and the average size
  of label values. Higher number of per-metric lables and longer label values mean higher ingress bandwidth.


The required resources for query path:

* RAM size: depends on the number of time series to scan in each query and the `step`
  argument passed to [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).
  Higher number of scanned time series and lower `step` argument results in higher RAM usage.

* CPU cores: a CPU core per 30 millions of scanned data points per second.

* Network usage: depends on the frequency and the type of incoming requests. Typical Grafana dashboards usually
  require negligible network bandwidth.


### High availability

1) Install multiple VictoriaMetrics instances in distinct datacenters (availability zones).
2) Add addresses of these instances to `remote_write` section in Prometheus config:

```yml
remote_write:
  - url: http://<victoriametrics-addr-1>:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
  # ...
  - url: http://<victoriametrics-addr-N>:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
```

3) Apply the updated config:

```
kill -HUP `pidof prometheus`
```

4) Now Prometheus should write data into all the configured `remote_write` urls in parallel.
5) Set up [Promxy](https://github.com/jacksontj/promxy) in front of all the VictoriaMetrics replicas.
6) Set up Prometheus datasource in Grafana that points to Promxy.


If you have Prometheus HA pairs with replicas `r1` and `r2` in each pair, then configure each `r1`
to write data to `<victoriametrics-addr-1`, while each `r2` should write data to `victoriametrics-addr-2`.


### Multiple retentions

Just start multiple VictoriaMetrics instances with distinct values for the following flags:

* `-retentionPeriod`
* `-storageDataPath`, so the data for each retention period is saved in a separate directory
* `-httpListenAddr`, so clients may reach VictoriaMetrics instance with proper retention


### Downsampling

There is no downsampling support at the moment, but:
- VictoriaMetrics is optimized for querying big amounts of raw data. See benchmark results for heavy queries
  in [this article](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
- VictoriaMetrics has good compression for on-disk data. See [this article](https://medium.com/@valyala/victoriametrics-achieving-better-compression-for-time-series-data-than-gorilla-317bc1f95932)
  for details.

These properties reduce the need in downsampling. We plan implementing downsampling in the future.
See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/36) for details.


### Multi-tenancy

Single-node VictoriaMetrics doesn't support multi-tenancy. Use [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) instead.


### Scalability and cluster version

Though single-node VictoriaMetrics cannot scale to multiple nodes, it is optimized for resource usage - storage size / bandwidth / IOPS, RAM, CPU.
This means that a single-node VictoriaMetrics may scale vertically and substitute moderately sized cluster built with competing solutions
such as Thanos, Uber M3, InfluxDB or TimescaleDB. See [vertical scalability benchmarks](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).

So try single-node VictoriaMetrics at first and then [switch to cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) if you still need
horizontally scalable long-term remote storage for really large Prometheus deployments.
[Contact us](mailto:info@victoriametrics.com) for paid support.


### Alerting

VictoriaMetrics doesn't support rule evaluation and alerting yet, so these actions must be performed either
on [Prometheus side](https://prometheus.io/docs/alerting/overview/) or on [Grafana side](https://grafana.com/docs/alerting/rules/).


### Security

Do not forget protecting sensitive endpoints in VictoriaMetrics when exposing it to untrusted networks such as internet.
Consider setting the following command-line flags:

* `-tls`, `-tlsCertFile` and `-tlsKeyFile` for switching from HTTP to HTTPS.
* `-httpAuth.username` and `-httpAuth.password` for protecting all the HTTP endpoints
  with [HTTP Basic Authentication](https://en.wikipedia.org/wiki/Basic_access_authentication).
* `-deleteAuthKey` for protecting `/api/v1/admin/tsdb/delete_series` endpoint. See [how to delete time series](#how-to-delete-time-series).
* `-snapshotAuthKey` for protecting `/snapshot*` endpoints. See [how to work with snapshots](#how-to-work-with-snapshots).

Explicitly set internal network interface for TCP and UDP ports for data ingestion with Graphite and OpenTSDB formats.
For example, substitute `-graphiteListenAddr=:2003` with `-graphiteListenAddr=<internal_iface_ip>:2003`.


### Tuning

* There is no need in VictoriaMetrics tuning, since it uses reasonable defaults for command-line flags,
  which are automatically adjusted for the available CPU and RAM resources.
* There is no need in Operating System tuning, since VictoriaMetrics is optimized for default OS settings.
  The only option is increasing the limit on [the number open files in the OS](https://medium.com/@muhammadtriwibowo/set-permanently-ulimit-n-open-files-in-ubuntu-4d61064429a),
  so Prometheus instances could establish more connections to VictoriaMetrics.


### Monitoring

VictoriaMetrics exports internal metrics in Prometheus format on the `/metrics` page.
Add this page to Prometheus' scrape config in order to collect VictoriaMetrics metrics.
There is [an official Grafana dashboard for single-node VictoriaMetrics](https://grafana.com/dashboards/10229).

The most interesting metrics are:

* `vm_cache_entries{type="storage/hour_metric_ids"}` - the number of time series with new data points during the last hour
  aka active time series.
* `vm_rows{type="indexdb"}` - the number of rows in inverted index. Each label in each unique time series adds a single
  row into the inverted index. An approximate number of time series in the database may be calculated as
  `vm_rows{type="indexdb"} / (avg_labels_per_series + 1)`, where `avg_labels_per_series` is the average number of labels
  per each time series.
* Sum of `vm_rows{type="storage/big"}` and `vm_rows{type="storage/small"}` - total number of `(timestamp, value)` data points
  in the database.
* Sum of all the `vm_cache_size_bytes` metrics - the total size of all the caches in the database.
* `vm_allowed_memory_bytes` - the maximum allowed size for caches in the database. It is calculated as `system_memory * <-memory.allowedPercent> / 100`,
  where `system_memory` is the amount of system memory and `-memory.allowedPercent` is the corresponding flag value.
* `vm_rows_inserted_total` - the total number of inserted rows since VictoriaMetrics start.


### Troubleshooting

* If VictoriaMetrics works slowly and eats more than a CPU core per 100K ingested data points per second,
  then it is likely you have too many active time series for the current amount of RAM.
  It is recommended increasing the amount of RAM on the node with VictoriaMetrics in order to improve
  ingestion performance.
  Another option is to increase `-memory.allowedPercent` command-line flag value. Be careful with this
  option, since too big value for `-memory.allowedPercent` may result in high I/O usage.

* If VictoriaMetrics doesn't work because of certain parts are corrupted due to disk errors,
  then just remove directoreis with broken parts. This will recover VictoriaMetrics at the cost
  of data loss stored in the broken parts. In the future `vmrecover` tool will be created
  for automatic recovering from such errors.


## Contacts

Contact us with any questions regarding VictoriaMetrics at [info@victoriametrics.com](mailto:info@victoriametrics.com).


## Community and contributions

Feel free asking any questions regarding VictoriaMetrics:

- [slack](http://slack.victoriametrics.com/)
- [telergam-en](https://t.me/VictoriaMetrics_en)
- [telergam-ru](https://t.me/VictoriaMetrics_ru1)
- [google groups](https://groups.google.com/forum/#!forum/victorametrics-users)


If you like VictoriaMetrics and want contributing, then we need the following:

- Filing issues and feature requests [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
- Spreading a word about VictoriaMetrics: conference talks, articles, comments, experience sharing with colleagues.
- Updating documentation.

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle):

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency or performance.

Adhering `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and verified by many people.


## Reporting bugs

Report bugs and propose new features [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).


## Victoria Metrics Logo

[Zip](VM_logo.zip) contains three folders with different image orientation (main color and inverted version).

Files included in each folder:

* 2 JPEG Preview files
* 2 PNG Preview files with transparent background
* 2 EPS Adobe Illustrator EPS10 files


### Logo Usage Guidelines

#### Font used:

* Lato Black
* Lato Regular

#### Color Palette:

* HEX [#110f0f](https://www.color-hex.com/color/110f0f)
* HEX [#ffffff](https://www.color-hex.com/color/ffffff)

### We kindly ask:

- Please don't use any other font instead of suggested.
- There should be sufficient clear space around the logo.
- Do not change spacing, alignment, or relative locations of the design elements.
- Do not change the proportions of any of the design elements or the design itself. You    may resize as needed but must retain all proportions.
