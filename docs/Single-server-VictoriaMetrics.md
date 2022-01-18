---
sort: 1
---

# VictoriaMetrics

[![Latest Release](https://img.shields.io/github/release/VictoriaMetrics/VictoriaMetrics.svg?style=flat-square)](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/victoriametrics/victoria-metrics.svg?maxAge=604800)](https://hub.docker.com/r/victoriametrics/victoria-metrics)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)
[![GitHub license](https://img.shields.io/github/license/VictoriaMetrics/VictoriaMetrics.svg)](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/VictoriaMetrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/VictoriaMetrics)
[![Build Status](https://github.com/VictoriaMetrics/VictoriaMetrics/workflows/main/badge.svg)](https://github.com/VictoriaMetrics/VictoriaMetrics/actions)
[![codecov](https://codecov.io/gh/VictoriaMetrics/VictoriaMetrics/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/VictoriaMetrics)

<img src="logo.png" width="300" alt="VictoriaMetrics logo">

VictoriaMetrics is a fast, cost-effective and scalable monitoring solution and time series database.

VictoriaMetrics is available in [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases),
[Docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/), [Snap packages](https://snapcraft.io/victoriametrics)
and [source code](https://github.com/VictoriaMetrics/VictoriaMetrics). Just download VictoriaMetrics and follow [these instructions](#how-to-start-victoriametrics).
Then read [Prometheus setup](#prometheus-setup) and [Grafana setup](#grafana-setup) docs.

Cluster version of VictoriaMetrics is available [here](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html).

[Contact us](mailto:info@victoriametrics.com) if you need enterprise support for VictoriaMetrics. See [features available in enterprise package](https://victoriametrics.com/products/enterprise/). Enterprise binaries can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).


## Prominent features

VictoriaMetrics has the following prominent features:

* It can be used as long-term storage for Prometheus. See [these docs](#prometheus-setup) for details.
* It can be used as drop-in replacement for Prometheus in Grafana, because it supports [Prometheus querying API](#prometheus-querying-api-usage).
* It can be used as drop-in replacement for Graphite in Grafana, because it supports [Graphite API](#graphite-api-usage).
* It features easy setup and operation:
  * VictoriaMetrics consists of a single [small executable](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d) without external dependencies.
  * All the configuration is done via explicit command-line flags with reasonable defaults.
  * All the data is stored in a single directory pointed by `-storageDataPath` command-line flag.
  * Easy and fast backups from [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282) to S3 or GCS can be done with [vmbackup](https://docs.victoriametrics.com/vmbackup.html) / [vmrestore](https://docs.victoriametrics.com/vmrestore.html) tools. See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for more details.
* It implements PromQL-based query language - [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html), which provides improved functionality on top of PromQL.
* It provides global query view. Multiple Prometheus instances or any other data sources may ingest data into VictoriaMetrics. Later this data may be queried via a single query.
* It provides high performance and good vertical and horizontal scalability for both [data ingestion](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b) and [data querying](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4). It [outperforms InfluxDB and TimescaleDB by up to 20x](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).
* It [uses 10x less RAM than InfluxDB](https://medium.com/@valyala/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) and [up to 7x less RAM than Prometheus, Thanos or Cortex](https://valyala.medium.com/prometheus-vs-victoriametrics-benchmark-on-node-exporter-metrics-4ca29c75590f) when dealing with millions of unique time series (aka [high cardinality](https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality)).
* It is optimized for time series with [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
* It provides high data compression, so [up to 70x more data points](https://medium.com/@valyala/when-size-matters-benchmarking-victoriametrics-vs-timescale-and-influxdb-6035811952d4) may be crammed into limited storage comparing to TimescaleDB and [up to 7x less storage space is required compared to Prometheus, Thanos or Cortex](https://valyala.medium.com/prometheus-vs-victoriametrics-benchmark-on-node-exporter-metrics-4ca29c75590f).
* It is optimized for storage with high-latency IO and low IOPS (HDD and network storage in AWS, Google Cloud, Microsoft Azure, etc). See [disk IO graphs from these benchmarks](https://medium.com/@valyala/high-cardinality-tsdb-benchmarks-victoriametrics-vs-timescaledb-vs-influxdb-13e6ee64dd6b).
* A single-node VictoriaMetrics may substitute moderately sized clusters built with competing solutions such as Thanos, M3DB, Cortex, InfluxDB or TimescaleDB. See [vertical scalability benchmarks](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae), [comparing Thanos to VictoriaMetrics cluster](https://medium.com/@valyala/comparing-thanos-to-victoriametrics-cluster-b193bea1683) and [Remote Write Storage Wars](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) talk from [PromCon 2019](https://promcon.io/2019-munich/talks/remote-write-storage-wars/).
* It protects the storage from data corruption on unclean shutdown (i.e. OOM, hardware reset or `kill -9`) thanks to [the storage architecture](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
* It supports metrics' scraping, ingestion and [backfilling](#backfilling) via the following protocols:
  * [Metrics scraping from Prometheus exporters](#how-to-scrape-prometheus-exporters-such-as-node-exporter).
  * [Prometheus remote write API](#prometheus-setup).
  * [Prometheus exposition format](#how-to-import-data-in-prometheus-exposition-format).
  * [InfluxDB line protocol](#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf) over HTTP, TCP and UDP.
  * [Graphite plaintext protocol](#how-to-send-data-from-graphite-compatible-agents-such-as-statsd) with [tags](https://graphite.readthedocs.io/en/latest/tags.html#carbon).
  * [OpenTSDB put message](#sending-data-via-telnet-put-protocol).
  * [HTTP OpenTSDB /api/put requests](#sending-opentsdb-data-via-http-apiput-requests).
  * [JSON line format](#how-to-import-data-in-json-line-format).
  * [Arbitrary CSV data](#how-to-import-csv-data).
  * [Native binary format](#how-to-import-data-in-native-format).
* It supports metrics' relabeling. See [these docs](#relabeling) for details.
* It can deal with [high cardinality issues](https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality) and [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate) issues via [series limiter](#cardinality-limiter).
* It ideally works with big amounts of time series data from APM, Kubernetes, IoT sensors, connected cars, industrial telemetry, financial data and various [Enterprise workloads](https://victoriametrics.com/products/enterprise/).
* It has open source [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

See also [various Articles about VictoriaMetrics](https://docs.victoriametrics.com/Articles.html).


## Case studies and talks

Case studies:

* [AbiosGaming](https://docs.victoriametrics.com/CaseStudies.html#abiosgaming)
* [adidas](https://docs.victoriametrics.com/CaseStudies.html#adidas)
* [Adsterra](https://docs.victoriametrics.com/CaseStudies.html#adsterra)
* [ARNES](https://docs.victoriametrics.com/CaseStudies.html#arnes)
* [Brandwatch](https://docs.victoriametrics.com/CaseStudies.html#brandwatch)
* [CERN](https://docs.victoriametrics.com/CaseStudies.html#cern)
* [COLOPL](https://docs.victoriametrics.com/CaseStudies.html#colopl)
* [Dreamteam](https://docs.victoriametrics.com/CaseStudies.html#dreamteam)
* [Fly.io](https://docs.victoriametrics.com/CaseStudies.html#flyio)
* [German Research Center for Artificial Intelligence](https://docs.victoriametrics.com/CaseStudies.html#german-research-center-for-artificial-intelligence)
* [Grammarly](https://docs.victoriametrics.com/CaseStudies.html#grammarly)
* [Groove X](https://docs.victoriametrics.com/CaseStudies.html#groove-x)
* [Idealo.de](https://docs.victoriametrics.com/CaseStudies.html#idealode)
* [MHI Vestas Offshore Wind](https://docs.victoriametrics.com/CaseStudies.html#mhi-vestas-offshore-wind)
* [Razorpay](https://docs.victoriametrics.com/CaseStudies.html#razorpay)
* [Percona](https://docs.victoriametrics.com/CaseStudies.html#percona)
* [Sensedia](https://docs.victoriametrics.com/CaseStudies.html#sensedia)
* [Smarkets](https://docs.victoriametrics.com/CaseStudies.html#smarkets)
* [Synthesio](https://docs.victoriametrics.com/CaseStudies.html#synthesio)
* [Wedos.com](https://docs.victoriametrics.com/CaseStudies.html#wedoscom)
* [Wix.com](https://docs.victoriametrics.com/CaseStudies.html#wixcom)
* [Zerodha](https://docs.victoriametrics.com/CaseStudies.html#zerodha)
* [zhihu](https://docs.victoriametrics.com/CaseStudies.html#zhihu)

See also [articles and slides about VictoriaMetrics from our users](https://docs.victoriametrics.com/Articles.html#third-party-articles-and-slides-about-victoriametrics)


## Operation

## How to start VictoriaMetrics

Just download [VictoriaMetrics executable](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) or [Docker image](https://hub.docker.com/r/victoriametrics/victoria-metrics/) and start it with the desired command-line flags.

The following command-line flags are used the most:

* `-storageDataPath` - VictoriaMetrics stores all the data in this directory. Default path is `victoria-metrics-data` in the current working directory.
* `-retentionPeriod` - retention for stored data. Older data is automatically deleted. Default retention is 1 month. See [these docs](#retention) for more details.

Other flags have good enough default values, so set them only if you really need this. Pass `-help` to see [all the available flags with description and default values](#list-of-command-line-flags).

See how to [ingest data to VictoriaMetrics](#how-to-import-time-series-data), how to [query VictoriaMetrics via Grafana](#grafana-setup), how to [query VictoriaMetrics via Graphite API](#graphite-api-usage) and how to [handle alerts](#alerting).

VictoriaMetrics accepts [Prometheus querying API requests](#prometheus-querying-api-usage) on port `8428` by default.

It is recommended setting up [monitoring](#monitoring) for VictoriaMetrics.


### Environment variables

Each flag value can be set via environment variables according to these rules:

* The `-envflag.enable` flag must be set.
* Each `.` char in flag name must be substituted with `_` (for example `-insert.maxQueueDuration <duration>` will translate to `insert_maxQueueDuration=<duration>`).
* For repeating flags an alternative syntax can be used by joining the different values into one using `,` char as separator (for example `-storageNode <nodeA> -storageNode <nodeB>` will translate to `storageNode=<nodeA>,<nodeB>`).
* Environment var prefix can be set via `-envflag.prefix` flag. For instance, if `-envflag.prefix=VM_`, then env vars must be prepended with `VM_`.


### Configuration with snap package


Snap package for VictoriaMetrics is available [here](https://snapcraft.io/victoriametrics).

Command-line flags for Snap package can be set with following command:

```text
echo 'FLAGS="-selfScrapeInterval=10s -search.logSlowQueryDuration=20s"' > $SNAP_DATA/var/snap/victoriametrics/current/extra_flags
snap restart victoriametrics
```

Do not change value for `-storageDataPath` flag, because snap package has limited access to host filesystem.


Changing scrape configuration is possible with text editor:

```text
vi $SNAP_DATA/var/snap/victoriametrics/current/etc/victoriametrics-scrape-config.yaml
```

After changes were made, trigger config re-read with the command `curl 127.0.0.1:8248/-/reload`.


## Prometheus setup

Add the following lines to Prometheus config file (it is usually located at `/etc/prometheus/prometheus.yml`) in order to send data to VictoriaMetrics:

```yml
remote_write:
  - url: http://<victoriametrics-addr>:8428/api/v1/write
```

Substitute `<victoriametrics-addr>` with hostname or IP address of VictoriaMetrics.
Then apply new config via the following command:

```bash
kill -HUP `pidof prometheus`
```

Prometheus writes incoming data to local storage and replicates it to remote storage in parallel.
This means that data remains available in local storage for `--storage.tsdb.retention.time` duration
even if remote storage is unavailable.

If you plan sending data to VictoriaMetrics from multiple Prometheus instances, then add the following lines into `global` section
of [Prometheus config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file):

```yml
global:
  external_labels:
    datacenter: dc-123
```

This instructs Prometheus to add `datacenter=dc-123` label to each sample before sending it to remote storage.
The label name can be arbitrary - `datacenter` is just an example. The label value must be unique
across Prometheus instances, so time series could be filtered and grouped by this label.

For highly loaded Prometheus instances (200k+ samples per second) the following tuning may be applied:

```yaml
remote_write:
  - url: http://<victoriametrics-addr>:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
      capacity: 20000
      max_shards: 30
```

Using remote write increases memory usage for Prometheus by up to ~25%. If you are experiencing issues with
too high memory consumption of Prometheus, then try to lower `max_samples_per_send` and `capacity` params. Keep in mind that these two params are tightly connected.
Read more about tuning remote write for Prometheus [here](https://prometheus.io/docs/practices/remote_write).

It is recommended upgrading Prometheus to [v2.12.0](https://github.com/prometheus/prometheus/releases) or newer, since previous versions may have issues with `remote_write`.

Take a look also at [vmagent](https://docs.victoriametrics.com/vmagent.html) and [vmalert](https://docs.victoriametrics.com/vmalert.html),
which can be used as faster and less resource-hungry alternative to Prometheus.


## Grafana setup

Create [Prometheus datasource](http://docs.grafana.org/features/datasources/prometheus/) in Grafana with the following url:

```url
http://<victoriametrics-addr>:8428
```

Substitute `<victoriametrics-addr>` with the hostname or IP address of VictoriaMetrics.

Then build graphs and dashboards for the created datasource using [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) or [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html).


## How to upgrade VictoriaMetrics

It is safe upgrading VictoriaMetrics to new versions unless [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) say otherwise. It is safe skipping multiple versions during the upgrade unless [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) say otherwise. It is recommended performing regular upgrades to the latest version, since it may contain important bug fixes, performance optimizations or new features.

It is also safe downgrading to older versions unless [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) say otherwise.

The following steps must be performed during the upgrade / downgrade procedure:

* Send `SIGINT` signal to VictoriaMetrics process in order to gracefully stop it.
* Wait until the process stops. This can take a few seconds.
* Start the upgraded VictoriaMetrics.

Prometheus doesn't drop data during VictoriaMetrics restart. See [this article](https://grafana.com/blog/2019/03/25/whats-new-in-prometheus-2.8-wal-based-remote-write/) for details. The same applies also to [vmagent](https://docs.victoriametrics.com/vmagent.html).


## How to apply new config to VictoriaMetrics

VictoriaMetrics is configured via command-line flags, so it must be restarted when new command-line flags should be applied:

* Send `SIGINT` signal to VictoriaMetrics process in order to gracefully stop it.
* Wait until the process stops. This can take a few seconds.
* Start VictoriaMetrics with the new command-line flags.

Prometheus doesn't drop data during VictoriaMetrics restart. See [this article](https://grafana.com/blog/2019/03/25/whats-new-in-prometheus-2.8-wal-based-remote-write/) for details. The same applies alos to [vmagent](https://docs.victoriametrics.com/vmagent.html).


## How to scrape Prometheus exporters such as [node-exporter](https://github.com/prometheus/node_exporter)

VictoriaMetrics can be used as drop-in replacement for Prometheus for scraping targets configured in `prometheus.yml` config file according to [the specification](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Just set `-promscrape.config` command-line flag to the path to `prometheus.yml` config - and VictoriaMetrics should start scraping the configured targets. Currently the following [scrape_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) types are supported:

* [static_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config)
* [file_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config)
* [kubernetes_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config)
* [ec2_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config)
* [gce_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config)
* [consul_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config)
* [dns_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config)
* [openstack_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config)
* [docker_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config)
* [dockerswarm_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config)
* [eureka_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#eureka_sd_config)
* [digitalocean_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config)
* [http_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config)


File a [feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you need support for other `*_sd_config` types.

The file pointed by `-promscrape.config` may contain `%{ENV_VAR}` placeholders, which are substituted by the corresponding `ENV_VAR` environment variable values.

VictoriaMetrics also supports [importing data in Prometheus exposition format](#how-to-import-data-in-prometheus-exposition-format).

See also [vmagent](https://docs.victoriametrics.com/vmagent.html), which can be used as drop-in replacement for Prometheus.


## How to send data from DataDog agent

VictoriaMetrics accepts data from [DataDog agent](https://docs.datadoghq.com/agent/) or [DogStatsD]() via ["submit metrics" API](https://docs.datadoghq.com/api/latest/metrics/#submit-metrics) at `/datadog/api/v1/series` path.

Run DataDog agent with `DD_DD_URL=http://victoriametrics-host:8428/datadog` environment variable in order to write data to VictoriaMetrics at `victoriametrics-host` host. Another option is to set `dd_url` param at [DataDog agent configuration file](https://docs.datadoghq.com/agent/guide/agent-configuration-files/) to `http://victoriametrics-host:8428/datadog`.

Example on how to send data to VictoriaMetrics via DataDog "submit metrics" API from command line:

```bash
echo '
{
  "series": [
    {
      "host": "test.example.com",
      "interval": 20,
      "metric": "system.load.1",
      "points": [[
        0,
        0.5
      ]],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}
' | curl -X POST --data-binary @- http://localhost:8428/datadog/api/v1/series
```

The imported data can be read via [export API](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format):

```bash
curl http://localhost:8428/api/v1/export -d 'match[]=system.load.1'
```

This command should return the following output if everything is OK:

```
{"metric":{"__name__":"system.load.1","environment":"test","host":"test.example.com"},"values":[0.5],"timestamps":[1632833641000]}
```

Extra labels may be added to all the written time series by passing `extra_label=name=value` query args.
For example, `/datadog/api/v1/series?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.


## How to send data from InfluxDB-compatible agents such as [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/)

Use `http://<victoriametric-addr>:8428` url instead of InfluxDB url in agents' configs.
For instance, put the following lines into `Telegraf` config, so it sends data to VictoriaMetrics instead of InfluxDB:

```toml
[[outputs.influxdb]]
  urls = ["http://<victoriametrics-addr>:8428"]
```

Another option is to enable TCP and UDP receiver for InfluxDB line protocol via `-influxListenAddr` command-line flag
and stream plain InfluxDB line protocol data to the configured TCP and/or UDP addresses.

VictoriaMetrics performs the following transformations to the ingested InfluxDB data:

* [`db` query arg](https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint) is mapped into `db` label value
  unless `db` tag exists in the InfluxDB line.
* Field names are mapped to time series names prefixed with `{measurement}{separator}` value, where `{separator}` equals to `_` by default. It can be changed with `-influxMeasurementFieldSeparator` command-line flag. See also `-influxSkipSingleField` command-line flag. If `{measurement}` is empty or if `-influxSkipMeasurement` command-line flag is set, then time series names correspond to field names.
* Field values are mapped to time series values.
* Tags are mapped to Prometheus labels as-is.

For example, the following InfluxDB line:

```raw
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following Prometheus data points:

```raw
foo_field1{tag1="value1", tag2="value2"} 12
foo_field2{tag1="value1", tag2="value2"} 40
```

Example for writing data with [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/)
to local VictoriaMetrics using `curl`:

```bash
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://localhost:8428/write'
```

An arbitrary number of lines delimited by '\n' (aka newline char) can be sent in a single request.
After that the data may be read via [/api/v1/export](#how-to-export-data-in-json-line-format) endpoint:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match={__name__=~"measurement_.*"}'
```

The `/api/v1/export` endpoint should return the following response:

```jsonl
{"metric":{"__name__":"measurement_field1","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560272508147]}
{"metric":{"__name__":"measurement_field2","tag1":"value1","tag2":"value2"},"values":[1.23],"timestamps":[1560272508147]}
```

Note that InfluxDB line protocol expects [timestamps in *nanoseconds* by default](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/#timestamp),
while VictoriaMetrics stores them with *milliseconds* precision.

Extra labels may be added to all the written time series by passing `extra_label=name=value` query args.
For example, `/write?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.

Some plugins for Telegraf such as [fluentd](https://github.com/fangli/fluent-plugin-influxdb), [Juniper/open-nti](https://github.com/Juniper/open-nti)
or [Juniper/jitmon](https://github.com/Juniper/jtimon) send `SHOW DATABASES` query to `/query` and expect a particular database name in the response.
Comma-separated list of expected databases can be passed to VictoriaMetrics via `-influx.databaseNames` command-line flag.

## How to send data from Graphite-compatible agents such as [StatsD](https://github.com/etsy/statsd)

Enable Graphite receiver in VictoriaMetrics by setting `-graphiteListenAddr` command line flag. For instance,
the following command will enable Graphite receiver in VictoriaMetrics on TCP and UDP port `2003`:

```bash
/path/to/victoria-metrics-prod -graphiteListenAddr=:2003
```

Use the configured address in Graphite-compatible agents. For instance, set `graphiteHost`
to the VictoriaMetrics host in `StatsD` configs.

Example for writing data with Graphite plaintext protocol to local VictoriaMetrics using `nc`:

```bash
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```

VictoriaMetrics sets the current time if the timestamp is omitted.
An arbitrary number of lines delimited by `\n` (aka newline char) can be sent in one go.
After that the data may be read via [/api/v1/export](#how-to-export-data-in-json-line-format) endpoint:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match=foo.bar.baz'
```

The `/api/v1/export` endpoint should return the following response:

```bash
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277406000]}
```

## Querying Graphite data

Data sent to VictoriaMetrics via `Graphite plaintext protocol` may be read via the following APIs:

* [Graphite API](#graphite-api-usage)
* [Prometheus querying API](#prometheus-querying-api-usage). See also [selecting Graphite metrics](#selecting-graphite-metrics).
* [go-graphite/carbonapi](https://github.com/go-graphite/carbonapi/blob/main/cmd/carbonapi/carbonapi.example.victoriametrics.yaml)

## Selecting Graphite metrics

VictoriaMetrics supports `__graphite__` pseudo-label for selecting time series with Graphite-compatible filters in [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html). For example, `{__graphite__="foo.*.bar"}` is equivalent to `{__name__=~"foo[.][^.]*[.]bar"}`, but it works faster and it is easier to use when migrating from Graphite to VictoriaMetrics. See [docs for Graphite paths and wildcards](https://graphite.readthedocs.io/en/latest/render_api.html#paths-and-wildcards). VictoriaMetrics also supports [label_graphite_group](https://docs.victoriametrics.com/MetricsQL.html#label_graphite_group) function for extracting the given groups from Graphite metric name.

The `__graphite__` pseudo-label supports e.g. alternate regexp filters such as `(value1|...|valueN)`. They are transparently converted to `{value1,...,valueN}` syntax [used in Graphite](https://graphite.readthedocs.io/en/latest/render_api.html#paths-and-wildcards). This allows using [multi-value template variables in Grafana](https://grafana.com/docs/grafana/latest/variables/formatting-multi-value-variables/) inside `__graphite__` pseudo-label. For example, Grafana expands `{__graphite__=~"foo.($bar).baz"}` into `{__graphite__=~"foo.(x|y).baz"}` if `$bar` template variable contains `x` and `y` values. In this case the query is automatically converted into `{__graphite__=~"foo.{x,y}.baz"}` before execution.

## How to send data from OpenTSDB-compatible agents

VictoriaMetrics supports [telnet put protocol](http://opentsdb.net/docs/build/html/api_telnet/put.html)
and [HTTP /api/put requests](http://opentsdb.net/docs/build/html/api_http/put.html) for ingesting OpenTSDB data.
The same protocol is used for [ingesting data in KairosDB](https://kairosdb.github.io/docs/build/html/PushingData.html).

### Sending data via `telnet put` protocol

Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command line flag. For instance,
the following command enables OpenTSDB receiver in VictoriaMetrics on TCP and UDP port `4242`:

```bash
/path/to/victoria-metrics-prod -opentsdbListenAddr=:4242
```

Send data to the given address from OpenTSDB-compatible agents.

Example for writing data with OpenTSDB protocol to local VictoriaMetrics using `nc`:

```bash
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

An arbitrary number of lines delimited by `\n` (aka newline char) can be sent in one go.
After that the data may be read via [/api/v1/export](#how-to-export-data-in-json-line-format) endpoint:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match=foo.bar.baz'
```

The `/api/v1/export` endpoint should return the following response:

```bash
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277292000]}
```

### Sending OpenTSDB data via HTTP `/api/put` requests

Enable HTTP server for OpenTSDB `/api/put` requests by setting `-opentsdbHTTPListenAddr` command line flag. For instance,
the following command enables OpenTSDB HTTP server on port `4242`:

```bash
/path/to/victoria-metrics-prod -opentsdbHTTPListenAddr=:4242
```

Send data to the given address from OpenTSDB-compatible agents.

Example for writing a single data point:

```bash
curl -H 'Content-Type: application/json' -d '{"metric":"x.y.z","value":45.34,"tags":{"t1":"v1","t2":"v2"}}' http://localhost:4242/api/put
```

Example for writing multiple data points in a single request:

```bash
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```

After that the data may be read via [/api/v1/export](#how-to-export-data-in-json-line-format) endpoint:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match[]=x.y.z' -d 'match[]=foo' -d 'match[]=bar'
```

The `/api/v1/export` endpoint should return the following response:

```bash
{"metric":{"__name__":"foo"},"values":[45.34],"timestamps":[1566464846000]}
{"metric":{"__name__":"bar"},"values":[43],"timestamps":[1566464846000]}
{"metric":{"__name__":"x.y.z","t1":"v1","t2":"v2"},"values":[45.34],"timestamps":[1566464763000]}
```

Extra labels may be added to all the imported time series by passing `extra_label=name=value` query args.
For example, `/api/put?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.


## Prometheus querying API usage

VictoriaMetrics supports the following handlers from [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/):

* [/api/v1/query](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries)
* [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries)
* [/api/v1/series](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
* [/api/v1/labels](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
* [/api/v1/label/.../values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)
* [/api/v1/status/tsdb](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats). See [these docs](#tsdb-stats) for details.
* [/api/v1/targets](https://prometheus.io/docs/prometheus/latest/querying/api/#targets) - see [these docs](#how-to-scrape-prometheus-exporters-such-as-node-exporter) for more details.

These handlers can be queried from Prometheus-compatible clients such as Grafana or curl.
All the Prometheus querying API handlers can be prepended with `/prometheus` prefix. For example, both `/prometheus/api/v1/query` and `/api/v1/query` should work.


### Prometheus querying API enhancements

VictoriaMetrics accepts optional `extra_label=<label_name>=<label_value>` query arg, which can be used for enforcing additional label filters for queries. For example,
`/api/v1/query_range?extra_label=user_id=123&extra_label=group_id=456&query=<query>` would automatically add `{user_id="123",group_id="456"}` label filters to the given `<query>`. This functionality can be used for limiting the scope of time series visible to the given tenant. It is expected that the `extra_label` query args are automatically set by auth proxy sitting in front of VictoriaMetrics. See [vmauth](https://docs.victoriametrics.com/vmauth.html) and [vmgateway](https://docs.victoriametrics.com/vmgateway.html) as examples of such proxies.

VictoriaMetrics accepts optional `extra_filters[]=series_selector` query arg, which can be used for enforcing arbitrary label filters for queries. For example,
`/api/v1/query_range?extra_filters[]={env=~"prod|staging",user="xyz"}&query=<query>` would automatically add `{env=~"prod|staging",user="xyz"}` label filters to the given `<query>`. This functionality can be used for limiting the scope of time series visible to the given tenant. It is expected that the `extra_filters[]` query args are automatically set by auth proxy sitting in front of VictoriaMetrics. See [vmauth](https://docs.victoriametrics.com/vmauth.html) and [vmgateway](https://docs.victoriametrics.com/vmgateway.html) as examples of such proxies.

VictoriaMetrics accepts relative times in `time`, `start` and `end` query args additionally to unix timestamps and [RFC3339](https://www.ietf.org/rfc/rfc3339.txt).
For example, the following query would return data for the last 30 minutes: `/api/v1/query_range?start=-30m&query=...`.

VictoriaMetrics accepts `round_digits` query arg for `/api/v1/query` and `/api/v1/query_range` handlers. It can be used for rounding response values to the given number of digits after the decimal point. For example, `/api/v1/query?query=avg_over_time(temperature[1h])&round_digits=2` would round response values to up to two digits after the decimal point.

By default, VictoriaMetrics returns time series for the last 5 minutes from `/api/v1/series`, while the Prometheus API defaults to all time.  Use `start` and `end` to select a different time range.

Additionally VictoriaMetrics provides the following handlers:

* `/vmui` - Basic Web UI. See [these docs](#vmui).
* `/api/v1/series/count` - returns the total number of time series in the database. Some notes:
  * the handler scans all the inverted index, so it can be slow if the database contains tens of millions of time series;
  * the handler may count [deleted time series](#how-to-delete-time-series) additionally to normal time series due to internal implementation restrictions;
* `/api/v1/labels/count` - returns a list of `label: values_count` entries. It can be used for determining labels with the maximum number of values.
* `/api/v1/status/active_queries` - returns a list of currently running queries.
* `/api/v1/status/top_queries` - returns the following query lists:
  * the most frequently executed queries - `topByCount`
  * queries with the biggest average execution duration - `topByAvgDuration`
  * queries that took the most time for execution - `topBySumDuration`

  The number of returned queries can be limited via `topN` query arg. Old queries can be filtered out with `maxLifetime` query arg.
  For example, request to `/api/v1/status/top_queries?topN=5&maxLifetime=30s` would return up to 5 queries per list, which were executed during the last 30 seconds.
  VictoriaMetrics tracks the last `-search.queryStats.lastQueriesCount` queries with durations at least `-search.queryStats.minQueryDuration`.


## Graphite API usage

VictoriaMetrics supports the following Graphite APIs, which are needed for [Graphite datasource in Grafana](https://grafana.com/docs/grafana/latest/datasources/graphite/):

* Render API - see [these docs](#graphite-render-api-usage).
* Metrics API - see [these docs](#graphite-metrics-api-usage).
* Tags API - see [these docs](#graphite-tags-api-usage).

All the Graphite handlers can be pre-pended with `/graphite` prefix. For example, both `/graphite/metrics/find` and `/metrics/find` should work.

VictoriaMetrics accepts optional query args: `extra_label=<label_name>=<label_value>` and `extra_filters[]=series_selector` query args for all the Graphite APIs. These args can be used for limiting the scope of time series visible to the given tenant. It is expected that the `extra_label` query arg is automatically set by auth proxy sitting in front of VictoriaMetrics. See [vmauth](https://docs.victoriametrics.com/vmauth.html) and [vmgateway](https://docs.victoriametrics.com/vmgateway.html) as examples of such proxies.

[Contact us](mailto:sales@victoriametrics.com) if you need assistance with such a proxy.

VictoriaMetrics supports `__graphite__` pseudo-label for filtering time series with Graphite-compatible filters in [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html). See [these docs](#selecting-graphite-metrics).


### Graphite Render API usage

[VictoriaMetrics Enterprise](https://victoriametrics.com/products/enterprise/) supports [Graphite Render API](https://graphite.readthedocs.io/en/stable/render_api.html) subset
at `/render` endpoint, which is used by [Graphite datasource in Grafana](https://grafana.com/docs/grafana/latest/datasources/graphite/).
When configuring Graphite datasource in Grafana, the `Storage-Step` http request header must be set to a step between Graphite data points stored in VictoriaMetrics. For example, `Storage-Step: 10s` would mean 10 seconds distance between Graphite datapoints stored in VictoriaMetrics.
Enterprise binaries can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).


### Graphite Metrics API usage

VictoriaMetrics supports the following handlers from [Graphite Metrics API](https://graphite-api.readthedocs.io/en/latest/api.html#the-metrics-api):

* [/metrics/find](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [/metrics/expand](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-expand)
* [/metrics/index.json](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-index-json)

VictoriaMetrics accepts the following additional query args at `/metrics/find` and `/metrics/expand`:
  * `label` - for selecting arbitrary label values. By default `label=__name__`, i.e. metric names are selected.
  * `delimiter` - for using different delimiters in metric name hierachy. For example, `/metrics/find?delimiter=_&query=node_*` would return all the metric name prefixes
    that start with `node_`. By default `delimiter=.`.


### Graphite Tags API usage

VictoriaMetrics supports the following handlers from [Graphite Tags API](https://graphite.readthedocs.io/en/stable/tags.html):

* [/tags/tagSeries](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb)
* [/tags/tagMultiSeries](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb)
* [/tags](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/{tag_name}](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/findSeries](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/autoComplete/tags](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support)
* [/tags/autoComplete/values](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support)
* [/tags/delSeries](https://graphite.readthedocs.io/en/stable/tags.html#removing-series-from-the-tagdb)


## vmui

VictoriaMetrics provides UI for query troubleshooting and exploration. The UI is available at `http://victoriametrics:8428/vmui`.
The UI allows exploring query results via graphs and tables. Graphs support scrolling and zooming:

* Drag the graph to the left / right in order to move the displayed time range into the past / future.
* Hold `Ctrl` (or `Cmd` on MacOS) and scroll up / down in order to zoom in / out the graph.

Query history can be navigated by holding `Ctrl` (or `Cmd` on MacOS) and pressing `up` or `down` arrows on the keyboard while the cursor is located in the query input field.

When querying the [backfilled data](https://docs.victoriametrics.com/#backfilling), it may be useful disabling response cache by clicking `Enable cache` checkbox.

VMUI automatically adjusts the interval between datapoints on the graph depending on the horizontal resolution and on the selected time range. The step value can be customized by clickhing `Override step value` checkbox.

VMUI allows investigating correlations between two queries on the same graph. Just click `+Query` button, enter the second query in the newly appeared input field and press `Ctrl+Enter`. Results for both queries should be displayed simultaneously on the same graph. Every query has its own vertical scale, which is displayed on the left and the right side of the graph. Lines for the second query are dashed.

See the [example VMUI at VictoriaMetrics playground](https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/prometheus/graph/?g0.expr=100%20*%20sum(rate(process_cpu_seconds_total))%20by%20(job)&g0.range_input=1d).


## How to build from sources

We recommend using either [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) or
[docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/) instead of building VictoriaMetrics
from sources. Building from sources is reasonable when developing additional features specific
to your needs or when testing bugfixes.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make victoria-metrics` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `victoria-metrics` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make victoria-metrics-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `victoria-metrics-prod` binary and puts it into the `bin` folder.

### ARM build

ARM build may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

### Development ARM build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make victoria-metrics-arm` or `make victoria-metrics-arm64` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `victoria-metrics-arm` or `victoria-metrics-arm64` binary respectively and puts it into the `bin` folder.

### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make victoria-metrics-arm-prod` or `make victoria-metrics-arm64-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `victoria-metrics-arm-prod` or `victoria-metrics-arm64-prod` binary respectively and puts it into the `bin` folder.

### Pure Go build (CGO_ENABLED=0)

`Pure Go` mode builds only Go code without [cgo](https://golang.org/cmd/cgo/) dependencies.

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make victoria-metrics-pure` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `victoria-metrics-pure` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-victoria-metrics`. It builds `victoriametrics/victoria-metrics:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-victoria-metrics`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable.
For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package-victoria-metrics
```

## Start with docker-compose

[Docker-compose](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/docker-compose.yml)
helps to spin up VictoriaMetrics, [vmagent](https://docs.victoriametrics.com/vmagent.html) and Grafana with one command.
More details may be found [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#folder-contains-basic-images-and-tools-for-building-and-running-victoria-metrics-in-docker).


## Setting up service

Read [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/43) on how to set up VictoriaMetrics as a service in your OS.
There is also [snap package for Ubuntu](https://snapcraft.io/victoriametrics).


## How to work with snapshots

VictoriaMetrics can create [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for all the data stored under `-storageDataPath` directory.
Navigate to `http://<victoriametrics-addr>:8428/snapshot/create` in order to create an instant snapshot.
The page will return the following JSON response:

```json
{"status":"ok","snapshot":"<snapshot-name>"}
```

Snapshots are created under `<-storageDataPath>/snapshots` directory, where `<-storageDataPath>`
is the command-line flag value. Snapshots can be archived to backup storage at any time
with [vmbackup](https://docs.victoriametrics.com/vmbackup.html).

The `http://<victoriametrics-addr>:8428/snapshot/list` page contains the list of available snapshots.

Navigate to `http://<victoriametrics-addr>:8428/snapshot/delete?snapshot=<snapshot-name>` in order
to delete `<snapshot-name>` snapshot.

Navigate to `http://<victoriametrics-addr>:8428/snapshot/delete_all` in order to delete all the snapshots.

Steps for restoring from a snapshot:

1. Stop VictoriaMetrics with `kill -INT`.
2. Restore snapshot contents from backup with [vmrestore](https://docs.victoriametrics.com/vmrestore.html)
   to the directory pointed by `-storageDataPath`.
3. Start VictoriaMetrics.

## How to delete time series

Send a request to `http://<victoriametrics-addr>:8428/api/v1/admin/tsdb/delete_series?match[]=<timeseries_selector_for_delete>`,
where `<timeseries_selector_for_delete>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to delete. After that all the time series matching the given selector are deleted. Storage space for
the deleted time series isn't freed instantly - it is freed during subsequent [background merges of data files](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
Note that background merges may never occur for data from previous months, so storage space won't be freed for historical data.
In this case [forced merge](#forced-merge) may help freeing up storage space.

It is recommended verifying which metrics will be deleted with the call to `http://<victoria-metrics-addr>:8428/api/v1/series?match[]=<timeseries_selector_for_delete>`
before actually deleting the metrics.  By default this query will only scan series in the past 5 minutes, so you may need to
adjust `start` and `end` to a suitable range to achieve match hits.

The `/api/v1/admin/tsdb/delete_series` handler may be protected with `authKey` if `-deleteAuthKey` command-line flag is set.

The delete API is intended mainly for the following cases:

* One-off deleting of accidentally written invalid (or undesired) time series.
* One-off deleting of user data due to [GDPR](https://en.wikipedia.org/wiki/General_Data_Protection_Regulation).

It isn't recommended using delete API for the following cases, since it brings non-zero overhead:

* Regular cleanups for unneeded data. Just prevent writing unneeded data into VictoriaMetrics.
  This can be done with [relabeling](#relabeling).
  See [this article](https://www.robustperception.io/relabelling-can-discard-targets-timeseries-and-alerts) for details.
* Reducing disk space usage by deleting unneeded time series. This doesn't work as expected, since the deleted
  time series occupy disk space until the next merge operation, which can never occur when deleting too old data.
  [Forced merge](#forced-merge) may be used for freeing up disk space occupied by old data.

It is better using `-retentionPeriod` command-line flag for efficient pruning of old data.


## Forced merge

VictoriaMetrics performs [data compactions in background](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
in order to keep good performance characteristics when accepting new data. These compactions (merges) are performed independently on per-month partitions.
This means that compactions are stopped for per-month partitions if no new data is ingested into these partitions.
Sometimes it is necessary to trigger compactions for old partitions. For instance, in order to free up disk space occupied by [deleted time series](#how-to-delete-time-series).
In this case forced compaction may be initiated on the specified per-month partition by sending request to `/internal/force_merge?partition_prefix=YYYY_MM`,
where `YYYY_MM` is per-month partition name. For example, `http://victoriametrics:8428/internal/force_merge?partition_prefix=2020_08` would initiate forced
merge for August 2020 partition. The call to `/internal/force_merge` returns immediately, while the corresponding forced merge continues running in background.

Forced merges may require additional CPU, disk IO and storage space resources. It is unnecessary to run forced merge under normal conditions,
since VictoriaMetrics automatically performs [optimal merges in background](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
when new data is ingested into it.


## How to export time series

VictoriaMetrics provides the following handlers for exporting data:

* `/api/v1/export` for exporing data in JSON line format. See [these docs](#how-to-export-data-in-json-line-format) for details.
* `/api/v1/export/csv` for exporting data in CSV. See [these docs](#how-to-export-csv-data) for details.
* `/api/v1/export/native` for exporting data in native binary format. This is the most efficient format for data export.
  See [these docs](#how-to-export-data-in-native-format) for details.


### How to export data in JSON line format

Send a request to `http://<victoriametrics-addr>:8428/api/v1/export?match[]=<timeseries_selector_for_export>`,
where `<timeseries_selector_for_export>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to export. Use `{__name__!=""}` selector for fetching all the time series.
The response would contain all the data for the selected time series in [JSON streaming format](https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON).
Each JSON line contains samples for a single time series. An example output:

```jsonl
{"metric":{"__name__":"up","job":"node_exporter","instance":"localhost:9100"},"values":[0,0,0],"timestamps":[1549891472010,1549891487724,1549891503438]}
{"metric":{"__name__":"up","job":"prometheus","instance":"localhost:9090"},"values":[1,1,1],"timestamps":[1549891461511,1549891476511,1549891491511]}
```

Optional `start` and `end` args may be added to the request in order to limit the time frame for the exported data. These args may contain either
unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values.

Optional `max_rows_per_line` arg may be added to the request for limiting the maximum number of rows exported per each JSON line.
Optional `reduce_mem_usage=1` arg may be added to the request for reducing memory usage when exporting big number of time series.
In this case the output may contain multiple lines with samples for the same time series.

Pass `Accept-Encoding: gzip` HTTP header in the request to `/api/v1/export` in order to reduce network bandwidth during exporing big amounts
of time series data. This enables gzip compression for the exported data. Example for exporting gzipped data:

```bash
curl -H 'Accept-Encoding: gzip' http://localhost:8428/api/v1/export -d 'match[]={__name__!=""}' > data.jsonl.gz
```

The maximum duration for each request to `/api/v1/export` is limited by `-search.maxExportDuration` command-line flag.

Exported data can be imported via POST'ing it to [/api/v1/import](#how-to-import-data-in-json-line-format).

The [deduplication](#deduplication) is applied to the data exported via `/api/v1/export` by default. The deduplication
isn't applied if `reduce_mem_usage=1` query arg is passed to the request.


### How to export CSV data

Send a request to `http://<victoriametrics-addr>:8428/api/v1/export/csv?format=<format>&match=<timeseries_selector_for_export>`,
where:

* `<format>` must contain comma-delimited label names for the exported CSV. The following special label names are supported:
  * `__name__` - metric name
  * `__value__` - sample value
  * `__timestamp__:<ts_format>` - sample timestamp. `<ts_format>` can have the following values:
    * `unix_s` - unix seconds
    * `unix_ms` - unix milliseconds
    * `unix_ns` - unix nanoseconds
    * `rfc3339` - [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) time
    * `custom:<layout>` - custom layout for time that is supported by [time.Format](https://golang.org/pkg/time/#Time.Format) function from Go.

* `<timeseries_selector_for_export>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to export.

Optional `start` and `end` args may be added to the request in order to limit the time frame for the exported data. These args may contain either
unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values.

The exported CSV data can be imported to VictoriaMetrics via [/api/v1/import/csv](#how-to-import-csv-data).

The [deduplication](#deduplication) is applied for the data exported in CSV by default. It is possible to export raw data without de-duplication by passing `reduce_mem_usage=1` query arg to `/api/v1/export/csv`.


### How to export data in native format

Send a request to `http://<victoriametrics-addr>:8428/api/v1/export/native?match[]=<timeseries_selector_for_export>`,
where `<timeseries_selector_for_export>` may contain any [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
for metrics to export. Use `{__name__=~".*"}` selector for fetching all the time series.

On large databases you may experience problems with limit on unique timeseries (default value is 300000). In this case you need to adjust `-search.maxUniqueTimeseries` parameter:

```bash
# count unique timeseries in database
wget -O- -q 'http://your_victoriametrics_instance:8428/api/v1/series/count' | jq '.data[0]'

# relaunch victoriametrics with search.maxUniqueTimeseries more than value from previous command
```

Optional `start` and `end` args may be added to the request in order to limit the time frame for the exported data. These args may contain either
unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values.

The exported data can be imported to VictoriaMetrics via [/api/v1/import/native](#how-to-import-data-in-native-format).
The native export format may change in incompatible way between VictoriaMetrics releases, so the data exported from the release X
can fail to be imported into VictoriaMetrics release Y.

The [deduplication](#deduplication) isn't applied for the data exported in native format. It is expected that the de-duplication is performed during data import.


## How to import time series data

Time series data can be imported via any supported ingestion protocol:

* [Prometheus remote_write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write). See [these docs](#prometheus-setup) for details.
* DataDog `submit metrics` API. See [these docs](#how-to-send-data-from-datadog-agent) for details.
* InfluxDB line protocol. See [these docs](#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf) for details.
* Graphite plaintext protocol. See [these docs](#how-to-send-data-from-graphite-compatible-agents-such-as-statsd) for details.
* OpenTSDB telnet put protocol. See [these docs](#sending-data-via-telnet-put-protocol) for details.
* OpenTSDB http `/api/put` protocol. See [these docs](#sending-opentsdb-data-via-http-apiput-requests) for details.
* `/api/v1/import` for importing data obtained from [/api/v1/export](#how-to-export-data-in-json-line-format).
  See [these docs](#how-to-import-data-in-json-line-format) for details.
* `/api/v1/import/native` for importing data obtained from [/api/v1/export/native](#how-to-export-data-in-native-format).
  See [these docs](#how-to-import-data-in-native-format) for details.
* `/api/v1/import/csv` for importing arbitrary CSV data. See [these docs](#how-to-import-csv-data) for details.
* `/api/v1/import/prometheus` for importing data in Prometheus exposition format. See [these docs](#how-to-import-data-in-prometheus-exposition-format) for details.


### How to import data in JSON line format

Example for importing data obtained via [/api/v1/export](#how-to-export-data-in-json-line-format):

```bash
# Export the data from <source-victoriametrics>:
curl http://source-victoriametrics:8428/api/v1/export -d 'match={__name__!=""}' > exported_data.jsonl

# Import the data to <destination-victoriametrics>:
curl -X POST http://destination-victoriametrics:8428/api/v1/import -T exported_data.jsonl
```

Pass `Content-Encoding: gzip` HTTP request header to `/api/v1/import` for importing gzipped data:

```bash
# Export gzipped data from <source-victoriametrics>:
curl -H 'Accept-Encoding: gzip' http://source-victoriametrics:8428/api/v1/export -d 'match={__name__!=""}' > exported_data.jsonl.gz

# Import gzipped data to <destination-victoriametrics>:
curl -X POST -H 'Content-Encoding: gzip' http://destination-victoriametrics:8428/api/v1/import -T exported_data.jsonl.gz
```

Extra labels may be added to all the imported time series by passing `extra_label=name=value` query args.
For example, `/api/v1/import?extra_label=foo=bar` would add `"foo":"bar"` label to all the imported time series.

Note that it could be required to flush response cache after importing historical data. See [these docs](#backfilling) for detail.

VictoriaMetrics parses input JSON lines one-by-one. It loads the whole JSON line in memory, then parses it and then saves the parsed samples into persistent storage. This means that VictoriaMetrics can occupy big amounts of RAM when importing too long JSON lines. The solution is to split too long JSON lines into smaller lines. It is OK if samples for a single time series are split among multiple JSON lines.


### How to import data in native format

The specification of VictoriaMetrics' native format may yet change and is not formally documented yet. So currently we do not recommend that external clients attempt to pack their own metrics in native format file.

If you have a native format file obtained via [/api/v1/export/native](#how-to-export-data-in-native-format) however this is the most efficient protocol for importing data in.

```bash
# Export the data from <source-victoriametrics>:
curl http://source-victoriametrics:8428/api/v1/export/native -d 'match={__name__!=""}' > exported_data.bin

# Import the data to <destination-victoriametrics>:
curl -X POST http://destination-victoriametrics:8428/api/v1/import/native -T exported_data.bin
```

Extra labels may be added to all the imported time series by passing `extra_label=name=value` query args.
For example, `/api/v1/import/native?extra_label=foo=bar` would add `"foo":"bar"` label to all the imported time series.

Note that it could be required to flush response cache after importing historical data. See [these docs](#backfilling) for detail.


### How to import CSV data

Arbitrary CSV data can be imported via `/api/v1/import/csv`. The CSV data is imported according to the provided `format` query arg.
The `format` query arg must contain comma-separated list of parsing rules for CSV fields. Each rule consists of three parts delimited by a colon:

```
<column_pos>:<type>:<context>
```

* `<column_pos>` is the position of the CSV column (field). Column numbering starts from 1. The order of parsing rules may be arbitrary.
* `<type>` describes the column type. Supported types are:
  * `metric` - the corresponding CSV column at `<column_pos>` contains metric value, which must be integer or floating-point number.
    The metric name is read from the `<context>`. CSV line must have at least a single metric field. Multiple metric fields per CSV line is OK.
  * `label` - the corresponding CSV column at `<column_pos>` contains label value. The label name is read from the `<context>`.
    CSV line may have arbitrary number of label fields. All these labels are attached to all the configured metrics.
  * `time` - the corresponding CSV column at `<column_pos>` contains metric time. CSV line may contain either one or zero columns with time.
    If CSV line has no time, then the current time is used. The time is applied to all the configured metrics.
    The format of the time is configured via `<context>`. Supported time formats are:
    * `unix_s` - unix timestamp in seconds.
    * `unix_ms` - unix timestamp in milliseconds.
    * `unix_ns` - unix timestamp in nanoseconds. Note that VictoriaMetrics rounds the timestamp to milliseconds.
    * `rfc3339` - timestamp in [RFC3339](https://tools.ietf.org/html/rfc3339) format, i.e. `2006-01-02T15:04:05Z`.
    * `custom:<layout>` - custom layout for the timestamp. The `<layout>` may contain arbitrary time layout according to [time.Parse rules in Go](https://golang.org/pkg/time/#Parse).

Each request to `/api/v1/import/csv` may contain arbitrary number of CSV lines.

Example for importing CSV data via `/api/v1/import/csv`:

```bash
curl -d "GOOG,1.23,4.56,NYSE" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
curl -d "MSFT,3.21,1.67,NASDAQ" 'http://localhost:8428/api/v1/import/csv?format=2:metric:ask,3:metric:bid,1:label:ticker,4:label:market'
```

After that the data may be read via [/api/v1/export](#how-to-export-data-in-json-line-format) endpoint:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match[]={ticker!=""}'
```

The following response should be returned:
```bash
{"metric":{"__name__":"bid","market":"NASDAQ","ticker":"MSFT"},"values":[1.67],"timestamps":[1583865146520]}
{"metric":{"__name__":"bid","market":"NYSE","ticker":"GOOG"},"values":[4.56],"timestamps":[1583865146495]}
{"metric":{"__name__":"ask","market":"NASDAQ","ticker":"MSFT"},"values":[3.21],"timestamps":[1583865146520]}
{"metric":{"__name__":"ask","market":"NYSE","ticker":"GOOG"},"values":[1.23],"timestamps":[1583865146495]}
```

Extra labels may be added to all the imported lines by passing `extra_label=name=value` query args.
For example, `/api/v1/import/csv?extra_label=foo=bar` would add `"foo":"bar"` label to all the imported lines.

Note that it could be required to flush response cache after importing historical data. See [these docs](#backfilling) for detail.


### How to import data in Prometheus exposition format

VictoriaMetrics accepts data in [Prometheus exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format)
and in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md)
via `/api/v1/import/prometheus` path. For example, the following line imports a single line in Prometheus exposition format into VictoriaMetrics:

```bash
curl -d 'foo{bar="baz"} 123' -X POST 'http://localhost:8428/api/v1/import/prometheus'
```

The following command may be used for verifying the imported data:

```bash
curl -G 'http://localhost:8428/api/v1/export' -d 'match={__name__=~"foo"}'
```

It should return something like the following:

```
{"metric":{"__name__":"foo","bar":"baz"},"values":[123],"timestamps":[1594370496905]}
```

Pass `Content-Encoding: gzip` HTTP request header to `/api/v1/import/prometheus` for importing gzipped data:

```bash
# Import gzipped data to <destination-victoriametrics>:
curl -X POST -H 'Content-Encoding: gzip' http://destination-victoriametrics:8428/api/v1/import/prometheus -T prometheus_data.gz
```

Extra labels may be added to all the imported metrics by passing `extra_label=name=value` query args.
For example, `/api/v1/import/prometheus?extra_label=foo=bar` would add `{foo="bar"}` label to all the imported metrics.

If timestamp is missing in `<metric> <value> <timestamp>` Prometheus exposition format line, then the current timestamp is used during data ingestion.
It can be overriden by passing unix timestamp in *milliseconds* via `timestamp` query arg. For example, `/api/v1/import/prometheus?timestamp=1594370496905`.

VictoriaMetrics accepts arbitrary number of lines in a single request to `/api/v1/import/prometheus`, i.e. it supports data streaming.

Note that it could be required to flush response cache after importing historical data. See [these docs](#backfilling) for detail.

VictoriaMetrics also may scrape Prometheus targets - see [these docs](#how-to-scrape-prometheus-exporters-such-as-node-exporter).



## Relabeling

VictoriaMetrics supports Prometheus-compatible relabeling for all the ingested metrics if `-relabelConfig` command-line flag points
to a file containing a list of [relabel_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config) entries.
The `-relabelConfig` also can point to http or https url. For example, `-relabelConfig=https://config-server/relabel_config.yml`.
See [this article with relabeling tips and tricks](https://valyala.medium.com/how-to-use-relabeling-in-prometheus-and-victoriametrics-8b90fc22c4b2).

Example contents for `-relabelConfig` file:
```yml
# Add {cluster="dev"} label.
- target_label: cluster
  replacement: dev

# Drop the metric (or scrape target) with `{__meta_kubernetes_pod_container_init="true"}` label.
- action: drop
  source_labels: [__meta_kubernetes_pod_container_init]
  regex: true
```

See [these docs](https://docs.victoriametrics.com/vmagent.html#relabeling) for more details about relabeling in VictoriaMetrics.


## Federation

VictoriaMetrics exports [Prometheus-compatible federation data](https://prometheus.io/docs/prometheus/latest/federation/)
at `http://<victoriametrics-addr>:8428/federate?match[]=<timeseries_selector_for_federation>`.

Optional `start` and `end` args may be added to the request in order to scrape the last point for each selected time series on the `[start ... end]` interval.
`start` and `end` may contain either unix timestamp in seconds or [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) values. By default, the last point
on the interval `[now - max_lookback ... now]` is scraped for each time series. The default value for `max_lookback` is `5m` (5 minutes), but it can be overridden.
For instance, `/federate?match[]=up&max_lookback=1h` would return last points on the `[now - 1h ... now]` interval. This may be useful for time series federation
with scrape intervals exceeding `5m`.


## Capacity planning

VictoriaMetrics uses lower amounts of CPU, RAM and storage space on production workloads compared to competing solutions (Prometheus, Thanos, Cortex, TimescaleDB, InfluxDB, QuestDB, M3DB) according to [our case studies](https://docs.victoriametrics.com/CaseStudies.html).

VictoriaMetrics capacity scales linearly with the available resources. The needed amounts of CPU and RAM highly depends on the workload - the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series), series [churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate), query types, query qps, etc. It is recommended setting up a test VictoriaMetrics for your production workload and iteratively scaling CPU and RAM resources until it becomes stable according to [troubleshooting docs](#troubleshooting). A single-node VictoriaMetrics works perfectly with the following production workload according to [our case studies](https://docs.victoriametrics.com/CaseStudies.html):

* Ingestion rate: 1.5+ million samples per second
* Active time series: 50+ million
* Total time series: 5+ billion
* Time series churn rate: 150+ million of new series per day
* Total number of samples: 10+ trillion
* Queries: 200+ qps
* Query latency (99th percentile): 1 second

The needed storage space for the given retention (the retention is set via `-retentionPeriod` command-line flag) can be extrapolated from disk space usage in a test run. For example, if `-storageDataPath` directory size becomes 10GB after a day-long test run on a production workload, then it will need at least `10GB*100=1TB` of disk space for `-retentionPeriod=100d` (100-days retention period).

It is recommended leaving the following amounts of spare resources:

* 50% of free RAM for reducing the probability of OOM (out of memory) crashes and slowdowns during temporary spikes in workload.
* 50% of spare CPU for reducing the probability of slowdowns during temporary spikes in workload.
* At least 30% of free storage space at the directory pointed by `-storageDataPath` command-line flag. See also `-storage.minFreeDiskSpaceBytes` command-line flag description [here](#list-of-command-line-flags).


## High availability

* Install multiple VictoriaMetrics instances in distinct datacenters (availability zones).
* Pass addresses of these instances to [vmagent](https://docs.victoriametrics.com/vmagent.html) via `-remoteWrite.url` command-line flag:

```bash
/path/to/vmagent -remoteWrite.url=http://<victoriametrics-addr-1>:8428/api/v1/write -remoteWrite.url=http://<victoriametrics-addr-2>:8428/api/v1/write
```

Alternatively these addresses may be passed to `remote_write` section in Prometheus config:

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

* Apply the updated config:

```bash
kill -HUP `pidof prometheus`
```

It is recommended to use [vmagent](https://docs.victoriametrics.com/vmagent.html) instead of Prometheus for highly loaded setups.

* Now Prometheus should write data into all the configured `remote_write` urls in parallel.
* Set up [Promxy](https://github.com/jacksontj/promxy) in front of all the VictoriaMetrics replicas.
* Set up Prometheus datasource in Grafana that points to Promxy.

If you have Prometheus HA pairs with replicas `r1` and `r2` in each pair, then configure each `r1`
to write data to `victoriametrics-addr-1`, while each `r2` should write data to `victoriametrics-addr-2`.

Another option is to write data simultaneously from Prometheus HA pair to a pair of VictoriaMetrics instances
with the enabled de-duplication. See [this section](#deduplication) for details.


## Deduplication

VictoriaMetrics de-duplicates data points if `-dedup.minScrapeInterval` command-line flag
is set to positive duration. For example, `-dedup.minScrapeInterval=60s` would de-duplicate data points
on the same time series if they fall within the same discrete 60s bucket.  The earliest data point will be kept.  In the case of equal timestamps, an arbitrary data point will be kept.

The recommended value for `-dedup.minScrapeInterval` must equal to `scrape_interval` config from Prometheus configs. It is recommended to have a single `scrape_interval` across all the scrape targets. See [this article](https://www.robustperception.io/keep-it-simple-scrape_interval-id) for details.

The de-duplication reduces disk space usage if multiple identically configured [vmagent](https://docs.victoriametrics.com/vmagent.html) or Prometheus instances in HA pair
write data to the same VictoriaMetrics instance. These vmagent or Prometheus instances must have identical
`external_labels` section in their configs, so they write data to the same time series.


## Retention

Retention is configured with `-retentionPeriod` command-line flag. For instance, `-retentionPeriod=3` means
that the data will be stored for 3 months and then deleted.
Data is split in per-month partitions inside `<-storageDataPath>/data/small` and `<-storageDataPath>/data/big` folders.
Data partitions outside the configured retention are deleted on the first day of new month.

Each partition consists of one or more data parts with the following name pattern `rowsCount_blocksCount_minTimestamp_maxTimestamp`.
Data parts outside of the configured retention are eventually deleted during [background merge](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).

In order to keep data according to `-retentionPeriod` max disk space usage is going to be `-retentionPeriod` + 1 month.
For example if `-retentionPeriod` is set to 1, data for January is deleted on March 1st.

VictoriaMetrics supports retention smaller than 1 month. For example, `-retentionPeriod=5d` would set data retention for 5 days.
Please note, time range covered by data part is not limited by retention period unit. Hence, data part may contain data
for multiple days and will be deleted only when fully outside of the configured retention.

It is safe to extend `-retentionPeriod` on existing data. If `-retentionPeriod` is set to lower
value than before then data outside the configured period will be eventually deleted.

## Multiple retentions

A single instance of VictoriaMetrics supports only a single retention, which can be configured via `-retentionPeriod` command-line flag. If you need multiple retentions, then you may start multiple VictoriaMetrics instances with distinct values for the following flags:

* `-retentionPeriod`
* `-storageDataPath`, so the data for each retention period is saved in a separate directory
* `-httpListenAddr`, so clients may reach VictoriaMetrics instance with proper retention

Then set up [vmauth](https://docs.victoriametrics.com/vmauth.html) in front of VictoriaMetrics instances,
so it could route requests from particular user to VictoriaMetrics with the desired retention.
The same scheme could be implemented for multiple tenants in [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html).
See [these docs](https://docs.victoriametrics.com/guides/guide-vmcluster-multiple-retention-setup.html) for multi-retention setup details.


## Downsampling

[VictoriaMetrics Enterprise](https://victoriametrics.com/products/enterprise/) supports multi-level downsampling with `-downsampling.period` command-line flag. For example:

* `-downsampling.period=30d:5m` instructs VictoriaMetrics to [deduplicate](#deduplication) samples older than 30 days with 5 minutes interval.

* `-downsampling.period=30d:5m,180d:1h` instructs VictoriaMetrics to deduplicate samples older than 30 days with 5 minutes interval and to deduplicate samples older than 180 days with 1 hour interval.

Downsampling is applied independently per each time series. It can reduce disk space usage and improve query performance if it is applied to time series with big number of samples per each series. The downsampling doesn't improve query performance if the database contains big number of time series with small number of samples per each series (aka [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate)), since downsampling doesn't reduce the number of time series. So the majority of time is spent on searching for the matching time series. It is possible to use recording rules in [vmalert](https://docs.victoriametrics.com/vmalert.html) in order to reduce the number of time series. See [these docs](https://docs.victoriametrics.com/vmalert.html#downsampling-and-aggregation-via-vmalert).

The downsampling can be evaluated for free by downloading and using enterprise binaries from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).


## Multi-tenancy

Single-node VictoriaMetrics doesn't support multi-tenancy. Use [cluster version](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy) instead.


## Scalability and cluster version

Though single-node VictoriaMetrics cannot scale to multiple nodes, it is optimized for resource usage - storage size / bandwidth / IOPS, RAM, CPU.
This means that a single-node VictoriaMetrics may scale vertically and substitute a moderately sized cluster built with competing solutions
such as Thanos, Uber M3, InfluxDB or TimescaleDB. See [vertical scalability benchmarks](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae).

So try single-node VictoriaMetrics at first and then [switch to cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) if you still need
horizontally scalable long-term remote storage for really large Prometheus deployments.
[Contact us](mailto:info@victoriametrics.com) for enterprise support.


## Alerting

It is recommended using [vmalert](https://docs.victoriametrics.com/vmalert.html) for alerting.

Additionally, alerting can be set up with the following tools:

* With Prometheus - see [the corresponding docs](https://prometheus.io/docs/alerting/overview/).
* With Promxy - see [the corresponding docs](https://github.com/jacksontj/promxy/blob/master/README.md#how-do-i-use-alertingrecording-rules-in-promxy).
* With Grafana - see [the corresponding docs](https://grafana.com/docs/alerting/rules/).


## Security

Do not forget protecting sensitive endpoints in VictoriaMetrics when exposing it to untrusted networks such as the internet.
Consider setting the following command-line flags:

* `-tls`, `-tlsCertFile` and `-tlsKeyFile` for switching from HTTP to HTTPS.
* `-httpAuth.username` and `-httpAuth.password` for protecting all the HTTP endpoints
  with [HTTP Basic Authentication](https://en.wikipedia.org/wiki/Basic_access_authentication).
* `-deleteAuthKey` for protecting `/api/v1/admin/tsdb/delete_series` endpoint. See [how to delete time series](#how-to-delete-time-series).
* `-snapshotAuthKey` for protecting `/snapshot*` endpoints. See [how to work with snapshots](#how-to-work-with-snapshots).
* `-forceMergeAuthKey` for protecting `/internal/force_merge` endpoint. See [force merge docs](#forced-merge).
* `-search.resetCacheAuthKey` for protecting `/internal/resetRollupResultCache` endpoint. See [backfilling](#backfilling) for more details.
* `-configAuthKey` for protecting `/config` endpoint, since it may contain sensitive information such as passwords.
- `-pprofAuthKey` for protecting `/debug/pprof/*` endpoints, which can be used for [profiling](#profiling).

Explicitly set internal network interface for TCP and UDP ports for data ingestion with Graphite and OpenTSDB formats.
For example, substitute `-graphiteListenAddr=:2003` with `-graphiteListenAddr=<internal_iface_ip>:2003`.

Prefer authorizing all the incoming requests from untrusted networks with [vmauth](https://docs.victoriametrics.com/vmauth.html)
or similar auth proxy.


## Tuning

* There is no need for VictoriaMetrics tuning since it uses reasonable defaults for command-line flags,
  which are automatically adjusted for the available CPU and RAM resources.
* There is no need for Operating System tuning since VictoriaMetrics is optimized for default OS settings.
  The only option is increasing the limit on [the number of open files in the OS](https://medium.com/@muhammadtriwibowo/set-permanently-ulimit-n-open-files-in-ubuntu-4d61064429a).
  The recommendation is not specific for VictoriaMetrics only but also for any service which handles many HTTP connections and stores data on disk.
* VictoriaMetrics is a write-heavy application and its performance depends on disk performance. So be careful with other
  applications or utilities (like [fstrim](http://manpages.ubuntu.com/manpages/bionic/man8/fstrim.8.html))
  which could [exhaust disk resources](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1521).
* The recommended filesystem is `ext4`, the recommended persistent storage is [persistent HDD-based disk on GCP](https://cloud.google.com/compute/docs/disks/#pdspecs),
  since it is protected from hardware failures via internal replication and it can be [resized on the fly](https://cloud.google.com/compute/docs/disks/add-persistent-disk#resize_pd).
  If you plan to store more than 1TB of data on `ext4` partition or plan extending it to more than 16TB,
  then the following options are recommended to pass to `mkfs.ext4`:

```bash
mkfs.ext4 ... -O 64bit,huge_file,extent -T huge
```

## Monitoring

VictoriaMetrics exports internal metrics in Prometheus format at `/metrics` page.
These metrics may be collected by [vmagent](https://docs.victoriametrics.com/vmagent.html)
or Prometheus by adding the corresponding scrape config to it.
Alternatively they can be self-scraped by setting `-selfScrapeInterval` command-line flag to duration greater than 0.
For example, `-selfScrapeInterval=10s` would enable self-scraping of `/metrics` page with 10 seconds interval.

There are officials Grafana dashboards for [single-node VictoriaMetrics](https://grafana.com/dashboards/10229) and [clustered VictoriaMetrics](https://grafana.com/grafana/dashboards/11176). There is also an [alternative dashboard for clustered VictoriaMetrics](https://grafana.com/grafana/dashboards/11831).

Graphs on these dashboard contain useful hints - hover the `i` icon at the top left corner of each graph in order to read it.

It is recommended setting up alerts in [vmalert](https://docs.victoriametrics.com/vmalert.html) or in Prometheus from [this config](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts.yml).

The most interesting metrics are:

* `vm_cache_entries{type="storage/hour_metric_ids"}` - the number of time series with new data points during the last hour
  aka [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series).
* `increase(vm_new_timeseries_created_total[1h])` - time series [churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate) during the previous hour.
* `sum(vm_rows{type=~"storage/.*"})` - total number of `(timestamp, value)` data points in the database.
* `sum(rate(vm_rows_inserted_total[5m]))` - ingestion rate, i.e. how many samples are inserted int the database per second.
* `vm_free_disk_space_bytes` - free space left at `-storageDataPath`.
* `sum(vm_data_size_bytes)` - the total size of data on disk.
* `increase(vm_slow_row_inserts_total[5m])` - the number of slow inserts during the last 5 minutes.
  If this number remains high during extended periods of time, then it is likely more RAM is needed for optimal handling
  of the current number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series).
* `increase(vm_slow_metric_name_loads_total[5m])` - the number of slow loads of metric names during the last 5 minutes.
  If this number remains high during extended periods of time, then it is likely more RAM is needed for optimal handling
  of the current number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series).

VictoriaMetrics also exposes currently running queries with their execution times at `/api/v1/status/active_queries` page.

See the example of alerting rules for VM components [here](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/alerts.yml).


## TSDB stats

VictoriaMetrics returns TSDB stats at `/api/v1/status/tsdb` page in the way similar to Prometheus - see [these Prometheus docs](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats). VictoriaMetrics accepts the following optional query args at `/api/v1/status/tsdb` page:
  * `topN=N` where `N` is the number of top entries to return in the response. By default top 10 entries are returned.
  * `date=YYYY-MM-DD` where `YYYY-MM-DD` is the date for collecting the stats. By default the stats is collected for the current day.
  * `match[]=SELECTOR` where `SELECTOR` is an arbitrary [time series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) for series to take into account during stats calculation. By default all the series are taken into account.
  * `extra_label=LABEL=VALUE`. See [these docs](#prometheus-querying-api-enhancements) for more details.


## Cardinality limiter

By default VictoriaMetrics doesn't limit the number of stored time series. The limit can be enforced by setting the following command-line flags:

* `-storage.maxHourlySeries` - limits the number of time series that can be added during the last hour. Useful for limiting the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series).
* `-storage.maxDailySeries` - limits the number of time series that can be added during the last day. Useful for limiting daily [churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).

Both limits can be set simultaneously. If any of these limits is reached, then incoming samples for new time series are dropped. A sample of dropped series is put in the log with `WARNING` level.

The exceeded limits can be [monitored](#monitoring) with the following metrics:

* `vm_hourly_series_limit_rows_dropped_total` - the number of metrics dropped due to exceeded hourly limit on the number of unique time series.
* `vm_daily_series_limit_rows_dropped_total` - the number of metrics dropped due to exceeded daily limit on the number of unique time series.

These limits are approximate, so VictoriaMetrics can underflow/overflow the limit by a small percentage (usually less than 1%).

See also more advanced [cardinality limiter in vmagent](https://docs.victoriametrics.com/vmagent.html#cardinality-limiter).


## Troubleshooting

* It is recommended to use default command-line flag values (i.e. don't set them explicitly) until the need
  of tweaking these flag values arises.

* It is recommended inspecting logs during troubleshooting, since they may contain useful information.

* It is recommended upgrading to the latest available release from [this page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases),
  since the encountered issue could be already fixed there.

* It is recommended to have at least 50% of spare resources for CPU, disk IO and RAM, so VictoriaMetrics could handle short spikes in the workload without performance issues.

* VictoriaMetrics requires free disk space for [merging data files to bigger ones](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282).
  It may slow down when there is no enough free space left. So make sure `-storageDataPath` directory
  has at least 20% of free space. The remaining amount of free space
  can be [monitored](#monitoring) via `vm_free_disk_space_bytes` metric. The total size of data
  stored on the disk can be monitored via sum of `vm_data_size_bytes` metrics.
  See also `vm_merge_need_free_disk_space` metrics, which are set to values higher than 0
  if background merge cannot be initiated due to free disk space shortage. The value shows the number of per-month partitions,
  which would start background merge if they had more free disk space.

* VictoriaMetrics buffers incoming data in memory for up to a few seconds before flushing it to persistent storage.
  This may lead to the following "issues":
  * Data becomes available for querying in a few seconds after inserting. It is possible to flush in-memory buffers to persistent storage
    by requesting `/internal/force_flush` http handler. This handler is mostly needed for testing and debugging purposes.
  * The last few seconds of inserted data may be lost on unclean shutdown (i.e. OOM, `kill -9` or hardware reset).
    See [this article for technical details](https://valyala.medium.com/wal-usage-looks-broken-in-modern-time-series-databases-b62a627ab704).

* If VictoriaMetrics works slowly and eats more than a CPU core per 100K ingested data points per second,
  then it is likely you have too many [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series) for the current amount of RAM.
  VictoriaMetrics [exposes](#monitoring) `vm_slow_*` metrics such as `vm_slow_row_inserts_total` and `vm_slow_metric_name_loads_total`, which could be used
  as an indicator of low amounts of RAM. It is recommended increasing the amount of RAM on the node with VictoriaMetrics in order to improve
  ingestion and query performance in this case.

* If the order of labels for the same metrics can change over time (e.g. if `metric{k1="v1",k2="v2"}` may become `metric{k2="v2",k1="v1"}`),
  then it is recommended running VictoriaMetrics with `-sortLabels` command-line flag in order to reduce memory usage and CPU usage.

* VictoriaMetrics prioritizes data ingestion over data querying. So if it has no enough resources for data ingestion,
  then data querying may slow down significantly.

* If VictoriaMetrics doesn't work because of certain parts are corrupted due to disk errors,
  then just remove directories with broken parts. It is safe removing subdirectories under `<-storageDataPath>/data/{big,small}/YYYY_MM` directories
  when VictoriaMetrics isn't running. This recovers VictoriaMetrics at the cost of data loss stored in the deleted broken parts.
  In the future, `vmrecover` tool will be created for automatic recovering from such errors.

* If you see gaps on the graphs, try resetting the cache by sending request to `/internal/resetRollupResultCache`.
  If this removes gaps on the graphs, then it is likely data with timestamps older than `-search.cacheTimestampOffset`
  is ingested into VictoriaMetrics. Make sure that data sources have synchronized time with VictoriaMetrics.

  If the gaps are related to irregular intervals between samples, then try adjusting `-search.minStalenessInterval` command-line flag
  to value close to the maximum interval between samples.

* If you are switching from InfluxDB or TimescaleDB, then take a look at `-search.maxStalenessInterval` command-line flag.
  It may be needed in order to suppress default gap filling algorithm used by VictoriaMetrics - by default it assumes
  each time series is continuous instead of discrete, so it fills gaps between real samples with regular intervals.

* Metrics and labels leading to [high cardinality](https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality) or [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate) can be determined at `/api/v1/status/tsdb` page. See [these docs](#tsdb-stats) for details.

* New time series can be logged if `-logNewSeries` command-line flag is passed to VictoriaMetrics.

* VictoriaMetrics limits the number of labels per each metric with `-maxLabelsPerTimeseries` command-line flag.
  This prevents from ingesting metrics with too many labels. It is recommended [monitoring](#monitoring) `vm_metrics_with_dropped_labels_total`
  metric in order to determine whether `-maxLabelsPerTimeseries` must be adjusted for your workload.

* If you store Graphite metrics like `foo.bar.baz` in VictoriaMetrics, then `{__graphite__="foo.*.baz"}` filter can be used for selecting such metrics. See [these docs](#selecting-graphite-metrics) for details.

* VictoriaMetrics ignores `NaN` values during data ingestion.


## Cache removal

VictoriaMetrics uses various internal caches. These caches are stored to `<-storageDataPath>/cache` directory during graceful shutdown (e.g. when VictoriaMetrics is stopped by sending `SIGINT` signal). The caches are read on the next VictoriaMetrics startup. Sometimes it is needed to remove such caches on the next startup. This can be performed by placing `reset_cache_on_startup` file inside the `<-storageDataPath>/cache` directory before the restart of VictoriaMetrics. See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1447) for details.


## Data migration

Use [vmctl](https://docs.victoriametrics.com/vmctl.html) for data migration. It supports the following data migration types:

* From Prometheus to VictoriaMetrics
* From InfluxDB to VictoriaMetrics
* From VictoriaMetrics to VictoriaMetrics
* From OpenTSDB to VictoriaMetrics

See [vmctl docs](https://docs.victoriametrics.com/vmctl.html) for more details.


## Backfilling

VictoriaMetrics accepts historical data in arbitrary order of time via [any supported ingestion method](#how-to-import-time-series-data).
See [how to backfill data with recording rules in vmalert](https://docs.victoriametrics.com/vmalert.html#rules-backfilling).
Make sure that configured `-retentionPeriod` covers timestamps for the backfilled data.

It is recommended disabling query cache with `-search.disableCache` command-line flag when writing
historical data with timestamps from the past, since the cache assumes that the data is written with
the current timestamps. Query cache can be enabled after the backfilling is complete.

An alternative solution is to query `/internal/resetRollupResultCache` url after backfilling is complete. This will reset
the query cache, which could contain incomplete data cached during the backfilling.

Yet another solution is to increase `-search.cacheTimestampOffset` flag value in order to disable caching
for data with timestamps close to the current time. Single-node VictoriaMetrics automatically resets response
cache when samples with timestamps older than `now - search.cacheTimestampOffset` are ingested to it.


## Data updates

VictoriaMetrics doesn't support updating already existing sample values to new ones. It stores all the ingested data points
for the same time series with identical timestamps. While it is possible substituting old time series with new time series via
[removal of old time series](#how-to-delete-timeseries) and then [writing new time series](#backfilling), this approach
should be used only for one-off updates. It shouldn't be used for frequent updates because of non-zero overhead related to data removal.


## Replication

Single-node VictoriaMetrics doesn't support application-level replication. Use cluster version instead.
See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#replication-and-data-safety) for details.

Storage-level replication may be offloaded to durable persistent storage such as [Google Cloud disks](https://cloud.google.com/compute/docs/disks#pdspecs).

See also [high availability docs](#high-availability) and [backup docs](#backups).


## Backups

VictoriaMetrics supports backups via [vmbackup](https://docs.victoriametrics.com/vmbackup.html)
and [vmrestore](https://docs.victoriametrics.com/vmrestore.html) tools.
We also provide [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager.html) tool for enterprise subscribers.
Enterprise binaries can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).


## Benchmarks

Note, that vendors (including VictoriaMetrics) are often biased when doing such tests. E.g. they try highlighting 
the best parts of their product, while highlighting the worst parts of competing products. 
So we encourage users and all independent third parties to conduct their becnhmarks for various products 
they are evaluating in production and publish the results.

As a reference, please see [benchmarks](https://docs.victoriametrics.com/Articles.html#benchmarks) conducted by
VictoriaMetrics team. Please also see the [helm chart](https://github.com/VictoriaMetrics/benchmark) 
for running ingestion benchmarks based on node_exporter metrics.


## Profiling

VictoriaMetrics provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile. It can be collected with the following command:

```bash
curl -s http://<victoria-metrics-host>:8428/debug/pprof/heap > mem.pprof
```

* CPU profile. It can be collected with the following command:

```bash
curl -s http://<victoria-metrics-host>:8428/debug/pprof/profile > cpu.pprof
```

The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).


## Integrations

* [Helm charts for single-node and cluster versions of VictoriaMetrics](https://github.com/VictoriaMetrics/helm-charts).
* [Kubernetes operator for VictoriaMetrics](https://github.com/VictoriaMetrics/operator).
* [netdata](https://github.com/netdata/netdata) can push data into VictoriaMetrics via `Prometheus remote_write API`.
  See [these docs](https://github.com/netdata/netdata#integrations).
* [go-graphite/carbonapi](https://github.com/go-graphite/carbonapi) can use VictoriaMetrics as time series backend.
  See [this example](https://github.com/go-graphite/carbonapi/blob/main/cmd/carbonapi/carbonapi.example.victoriametrics.yaml).
* [Ansible role for installing single-node VictoriaMetrics](https://github.com/dreamteam-gg/ansible-victoriametrics-role).
* [Ansible role for installing cluster VictoriaMetrics](https://github.com/Slapper/ansible-victoriametrics-cluster-role).
* [Snap package for VictoriaMetrics](https://snapcraft.io/victoriametrics).
* [vmalert-cli](https://github.com/aorfanos/vmalert-cli) - a CLI application for managing [vmalert](https://docs.victoriametrics.com/vmalert.html).


## Third-party contributions

* [Unofficial yum repository](https://copr.fedorainfracloud.org/coprs/antonpatsev/VictoriaMetrics/) ([source code](https://github.com/patsevanton/victoriametrics-rpm))
* [Prometheus -> VictoriaMetrics exporter #1](https://github.com/ryotarai/prometheus-tsdb-dump)
* [Prometheus -> VictoriaMetrics exporter #2](https://github.com/AnchorFree/tsdb-remote-write)
* [Prometheus Oauth proxy](https://gitlab.com/optima_public/prometheus_oauth_proxy) - see [this article](https://medium.com/@richard.holly/powerful-saas-solution-for-detection-metrics-c67b9208d362) for details.


## Contacts

Contact us with any questions regarding VictoriaMetrics at [info@victoriametrics.com](mailto:info@victoriametrics.com).


## Community and contributions

Feel free asking any questions regarding VictoriaMetrics:

* [slack](https://slack.victoriametrics.com/)
* [linkedin](https://www.linkedin.com/company/victoriametrics/)
* [reddit](https://www.reddit.com/r/VictoriaMetrics/)
* [telegram-en](https://t.me/VictoriaMetrics_en)
* [telegram-ru](https://t.me/VictoriaMetrics_ru1)
* [articles and talks about VictoriaMetrics in Russian](https://github.com/denisgolius/victoriametrics-ru-links)
* [google groups](https://groups.google.com/forum/#!forum/victorametrics-users)

If you like VictoriaMetrics and want to contribute, then we need the following:

* Filing issues and feature requests [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
* Spreading a word about VictoriaMetrics: conference talks, articles, comments, experience sharing with colleagues.
* Updating documentation.

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle):

* Prefer simple code and architecture.
* Avoid complex abstractions.
* Avoid magic code and fancy algorithms.
* Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
* Minimize the number of moving parts in the distributed system.
* Avoid automated decisions, which may hurt cluster availability, consistency or performance.

Adhering `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and verified by many people.

## Reporting bugs

Report bugs and propose new features [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).


## VictoriaMetrics Logo

[Zip](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/VM_logo.zip) contains three folders with different image orientations (main color and inverted version).

Files included in each folder:

* 2 JPEG Preview files
* 2 PNG Preview files with transparent background
* 2 EPS Adobe Illustrator EPS10 files

### Logo Usage Guidelines

#### Font used

* Lato Black
* Lato Regular

#### Color Palette

* HEX [#110f0f](https://www.color-hex.com/color/110f0f)
* HEX [#ffffff](https://www.color-hex.com/color/ffffff)

### We kindly ask

* Please don't use any other font instead of suggested.
* There should be sufficient clear space around the logo.
* Do not change spacing, alignment, or relative locations of the design elements.
* Do not change the proportions of any of the design elements or the design itself. You    may resize as needed but must retain all proportions.


## List of command-line flags

Pass `-help` to VictoriaMetrics in order to see the list of supported command-line flags with their description:

```
  -bigMergeConcurrency int
    	The maximum number of CPU cores to use for big merges. Default value is used if set to 0
  -configAuthKey string
    	Authorization key for accessing /config page. It must be passed via authKey query arg
  -csvTrimTimestamp duration
    	Trim timestamps when importing csv data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -datadog.maxInsertRequestSize size
    	The maximum size in bytes of a single DataDog POST request to /api/v1/series
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 67108864)
  -dedup.minScrapeInterval duration
    	Leave only the first sample in every time series per each discrete interval equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication and https://docs.victoriametrics.com/#downsampling
  -deleteAuthKey string
    	authKey for metrics' deletion via /api/v1/admin/tsdb/delete_series and /tags/delSeries
  -denyQueriesOutsideRetention
    	Whether to deny queries outside of the configured -retentionPeriod. When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. This may be useful when multiple data sources with distinct retentions are hidden behind query-tee
  -downsampling.period array
    	Comma-separated downsampling periods in the format 'offset:period'. For example, '30d:10m' instructs to leave a single sample per 10 minutes for samples older than 30 days. See https://docs.victoriametrics.com/#downsampling for details
    	Supports an array of values separated by comma or specified via multiple flags.
  -dryRun
    	Whether to check only -promscrape.config and then exit. Unknown config entries are allowed in -promscrape.config by default. This can be changed with -promscrape.config.strictParse
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -eula
    	By specifying this flag, you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -finalMergeDelay duration
    	The delay before starting final merge for per-month partition after no new data is ingested into it. Final merge may require additional disk IO and CPU resources. Final merge may increase query speed and reduce disk space usage in some cases. Zero value disables final merge
  -forceFlushAuthKey string
    	authKey, which must be passed in query string to /internal/force_flush pages
  -forceMergeAuthKey string
    	authKey, which must be passed in query string to /internal/force_merge pages
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -graphiteListenAddr string
    	TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty
  -graphiteTrimTimestamp duration
    	Trim timestamps for Graphite data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
  -http.connTimeout duration
    	Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
    	Disable compression of HTTP responses to save CPU resources. By default compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
    	Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
    	The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
    	An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
    	Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
    	Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
    	Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
    	TCP address to listen for http connections (default ":8428")
  -import.maxLineLen size
    	The maximum length in bytes of a single line accepted by /api/v1/import; the line length can be limited with 'max_rows_per_line' query arg passed to /api/v1/export
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 104857600)
  -influx.databaseNames array
    	Comma-separated list of database names to return from /query and /influx/query API. This can be needed for accepting data from Telegraf plugins such as https://github.com/fangli/fluent-plugin-influxdb
    	Supports an array of values separated by comma or specified via multiple flags.
  -influx.maxLineSize size
    	The maximum size in bytes for a single InfluxDB line during parsing
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 262144)
  -influxListenAddr string
    	TCP and UDP address to listen for InfluxDB line protocol data. Usually :8189 must be set. Doesn't work if empty. This flag isn't needed when ingesting data over HTTP - just send it to http://<victoriametrics>:8428/write
  -influxMeasurementFieldSeparator string
    	Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol (default "_")
  -influxSkipMeasurement
    	Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'
  -influxSkipSingleField
    	Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if InfluxDB line contains only a single field
  -influxTrimTimestamp duration
    	Trim timestamps for InfluxDB line protocol data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -insert.maxQueueDuration duration
    	The maximum duration for waiting in the queue for insert requests due to -maxConcurrentInserts (default 1m0s)
  -logNewSeries
    	Whether to log new series. This option is for debug purposes only. It can lead to performance issues when big number of new series are ingested into VictoriaMetrics
  -loggerDisableTimestamps
    	Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
    	Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit
  -loggerFormat string
    	Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
    	Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
    	Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
    	Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxConcurrentInserts int
    	The maximum number of concurrent inserts. Default value should work for most cases, since it minimizes the overhead for concurrent inserts. This option is tigthly coupled with -insert.maxQueueDuration (default 16)
  -maxInsertRequestSize size
    	The maximum size in bytes of a single Prometheus remote_write API request
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 33554432)
  -maxLabelValueLen int
    	The maximum length of label values in the accepted time series. Longer label values are truncated. In this case the vm_too_long_label_values_total metric at /metrics page is incremented (default 16384)
  -maxLabelsPerTimeseries int
    	The maximum number of labels accepted per time series. Superfluous labels are dropped. In this case the vm_metrics_with_dropped_labels_total metric at /metrics page is incremented (default 30)
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
    	Auth key for /metrics. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -opentsdbHTTPListenAddr string
    	TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty
  -opentsdbListenAddr string
    	TCP and UDP address to listen for OpentTSDB metrics. Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. Usually :4242 must be set. Doesn't work if empty
  -opentsdbTrimTimestamp duration
    	Trim timestamps for OpenTSDB 'telnet put' data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
  -opentsdbhttp.maxInsertRequestSize size
    	The maximum size of OpenTSDB HTTP put request
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 33554432)
  -opentsdbhttpTrimTimestamp duration
    	Trim timestamps for OpenTSDB HTTP data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -pprofAuthKey string
    	Auth key for /debug/pprof. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -precisionBits int
    	The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss (default 64)
  -promscrape.cluster.memberNum int
    	The number of number in the cluster of scrapers. It must be an unique value in the range 0 ... promscrape.cluster.membersCount-1 across scrapers in the cluster
  -promscrape.cluster.membersCount int
    	The number of members in a cluster of scrapers. Each member must have an unique -promscrape.cluster.memberNum in the range 0 ... promscrape.cluster.membersCount-1 . Each member then scrapes roughly 1/N of all the targets. By default cluster scraping is disabled, i.e. a single scraper scrapes all the targets
  -promscrape.cluster.replicationFactor int
    	The number of members in the cluster, which scrape the same targets. If the replication factor is greater than 2, then the deduplication must be enabled at remote storage side. See https://docs.victoriametrics.com/#deduplication (default 1)
  -promscrape.config string
    	Optional path to Prometheus config file with 'scrape_configs' section containing targets to scrape. The path can point to local file and to http url. See https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter for details
  -promscrape.config.dryRun
    	Checks -promscrape.config file for errors and unsupported fields and then exits. Returns non-zero exit code on parsing errors and emits these errors to stderr. See also -promscrape.config.strictParse command-line flag. Pass -loggerLevel=ERROR if you don't need to see info messages in the output.
  -promscrape.config.strictParse
    	Whether to allow only supported fields in -promscrape.config . By default unsupported fields are silently skipped
  -promscrape.configCheckInterval duration
    	Interval for checking for changes in '-promscrape.config' file. By default the checking is disabled. Send SIGHUP signal in order to force config check for changes
  -promscrape.consul.waitTime duration
    	Wait time used by Consul service discovery. Default value is used if not set
  -promscrape.consulSDCheckInterval duration
    	Interval for checking for changes in Consul. This works only if consul_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config for details (default 30s)
  -promscrape.digitaloceanSDCheckInterval duration
    	Interval for checking for changes in digital ocean. This works only if digitalocean_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config for details (default 1m0s)
  -promscrape.disableCompression
    	Whether to disable sending 'Accept-Encoding: gzip' request headers to all the scrape targets. This may reduce CPU usage on scrape targets at the cost of higher network bandwidth utilization. It is possible to set 'disable_compression: true' individually per each 'scrape_config' section in '-promscrape.config' for fine grained control
  -promscrape.disableKeepAlive
    	Whether to disable HTTP keep-alive connections when scraping all the targets. This may be useful when targets has no support for HTTP keep-alive connection. It is possible to set 'disable_keepalive: true' individually per each 'scrape_config' section in '-promscrape.config' for fine grained control. Note that disabling HTTP keep-alive may increase load on both vmagent and scrape targets
  -promscrape.discovery.concurrency int
    	The maximum number of concurrent requests to Prometheus autodiscovery API (Consul, Kubernetes, etc.) (default 100)
  -promscrape.discovery.concurrentWaitTime duration
    	The maximum duration for waiting to perform API requests if more than -promscrape.discovery.concurrency requests are simultaneously performed (default 1m0s)
  -promscrape.dnsSDCheckInterval duration
    	Interval for checking for changes in dns. This works only if dns_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config for details (default 30s)
  -promscrape.dockerSDCheckInterval duration
    	Interval for checking for changes in docker. This works only if docker_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config for details (default 30s)
  -promscrape.dockerswarmSDCheckInterval duration
    	Interval for checking for changes in dockerswarm. This works only if dockerswarm_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config for details (default 30s)
  -promscrape.dropOriginalLabels
    	Whether to drop original labels for scrape targets at /targets and /api/v1/targets pages. This may be needed for reducing memory usage when original labels for big number of scrape targets occupy big amounts of memory. Note that this reduces debuggability for improper per-target relabeling configs
  -promscrape.ec2SDCheckInterval duration
    	Interval for checking for changes in ec2. This works only if ec2_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config for details (default 1m0s)
  -promscrape.eurekaSDCheckInterval duration
    	Interval for checking for changes in eureka. This works only if eureka_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#eureka_sd_config for details (default 30s)
  -promscrape.fileSDCheckInterval duration
    	Interval for checking for changes in 'file_sd_config'. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config for details (default 30s)
  -promscrape.gceSDCheckInterval duration
    	Interval for checking for changes in gce. This works only if gce_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config for details (default 1m0s)
  -promscrape.httpSDCheckInterval duration
    	Interval for checking for changes in http endpoint service discovery. This works only if http_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config for details (default 1m0s)
  -promscrape.kubernetes.apiServerTimeout duration
    	How frequently to reload the full state from Kuberntes API server (default 30m0s)
  -promscrape.kubernetesSDCheckInterval duration
    	Interval for checking for changes in Kubernetes API server. This works only if kubernetes_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config for details (default 30s)
  -promscrape.maxDroppedTargets int
    	The maximum number of droppedTargets to show at /api/v1/targets page. Increase this value if your setup drops more scrape targets during relabeling and you need investigating labels for all the dropped targets. Note that the increased number of tracked dropped targets may result in increased memory usage (default 1000)
  -promscrape.maxResponseHeadersSize size
    	The maximum size of http response headers from Prometheus scrape targets
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 4096)
  -promscrape.maxScrapeSize size
    	The maximum size of scrape response in bytes to process from Prometheus targets. Bigger responses are rejected
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 16777216)
  -promscrape.minResponseSizeForStreamParse size
    	The minimum target response size for automatic switching to stream parsing mode, which can reduce memory usage. See https://docs.victoriametrics.com/vmagent.html#stream-parsing-mode
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 1000000)
  -promscrape.noStaleMarkers
    	Whether to disable sending Prometheus stale markers for metrics when scrape target disappears. This option may reduce memory usage if stale markers aren't needed for your setup. This option also disables populating the scrape_series_added metric. See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series
  -promscrape.openstackSDCheckInterval duration
    	Interval for checking for changes in openstack API server. This works only if openstack_sd_configs is configured in '-promscrape.config' file. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config for details (default 30s)
  -promscrape.seriesLimitPerTarget int
    	Optional limit on the number of unique time series a single scrape target can expose. See https://docs.victoriametrics.com/vmagent.html#cardinality-limiter for more info
  -promscrape.streamParse
    	Whether to enable stream parsing for metrics obtained from scrape targets. This may be useful for reducing memory usage when millions of metrics are exposed per each scrape target. It is posible to set 'stream_parse: true' individually per each 'scrape_config' section in '-promscrape.config' for fine grained control
  -promscrape.suppressDuplicateScrapeTargetErrors
    	Whether to suppress 'duplicate scrape target' errors; see https://docs.victoriametrics.com/vmagent.html#troubleshooting for details
  -promscrape.suppressScrapeErrors
    	Whether to suppress scrape errors logging. The last error for each target is always available at '/targets' page even if scrape errors logging is suppressed
  -relabelConfig string
    	Optional path to a file with relabeling rules, which are applied to all the ingested metrics. The path can point either to local file or to http url. See https://docs.victoriametrics.com/#relabeling for details. The config is reloaded on SIGHUP signal
  -relabelDebug
    	Whether to log metrics before and after relabeling with -relabelConfig. If the -relabelDebug is enabled, then the metrics aren't sent to storage. This is useful for debugging the relabeling configs
  -retentionPeriod value
    	Data with timestamps outside the retentionPeriod is automatically deleted
    	The following optional suffixes are supported: h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 1)
  -search.cacheTimestampOffset duration
    	The maximum duration since the current time for response data, which is always queried from the original raw data, without using the response cache. Increase this value if you see gaps in responses due to time synchronization issues between VictoriaMetrics and data sources. See also -search.disableAutoCacheReset (default 5m0s)
  -search.disableAutoCacheReset
    	Whether to disable automatic response cache reset if a sample with timestamp outside -search.cacheTimestampOffset is inserted into VictoriaMetrics
  -search.disableCache
    	Whether to disable response caching. This may be useful during data backfilling
  -search.graphiteMaxPointsPerSeries int
    	The maximum number of points per series Graphite render API can return (default 1000000)
  -search.graphiteStorageStep duration
    	The interval between datapoints stored in the database. It is used at Graphite Render API handler for normalizing the interval between datapoints in case it isn't normalized. It can be overriden by sending 'storage_step' query arg to /render API or by sending the desired interval via 'Storage-Step' http header during querying /render API (default 10s)
  -search.latencyOffset duration
    	The time when data points become visible in query results after the collection. Too small value can result in incomplete last points for query results (default 30s)
  -search.logSlowQueryDuration duration
    	Log queries with execution time exceeding this value. Zero disables slow query logging (default 5s)
  -search.maxConcurrentRequests int
    	The maximum number of concurrent search requests. It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration (default 8)
  -search.maxExportDuration duration
    	The maximum duration for /api/v1/export call (default 720h0m0s)
  -search.maxLookback duration
    	Synonym to -search.lookback-delta from Prometheus. The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. See also '-search.maxStalenessInterval' flag, which has the same meaining due to historical reasons
  -search.maxPointsPerTimeseries int
    	The maximum points per a single timeseries returned from /api/v1/query_range. This option doesn't limit the number of scanned raw samples in the database. The main purpose of this option is to limit the number of per-series points returned to graphing UI such as Grafana. There is no sense in setting this limit to values bigger than the horizontal resolution of the graph (default 30000)
  -search.maxQueryDuration duration
    	The maximum duration for query execution (default 30s)
  -search.maxQueryLen size
    	The maximum search query length in bytes
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 16384)
  -search.maxQueueDuration duration
    	The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached; see also -search.maxQueryDuration (default 10s)
  -search.maxSamplesPerQuery int
    	The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries (default 1000000000)
  -search.maxSamplesPerSeries int
    	The maximum number of raw samples a single query can scan per each time series. This option allows limiting memory usage (default 30000000)
  -search.maxStalenessInterval duration
    	The maximum interval for staleness calculations. By default it is automatically calculated from the median interval between samples. This flag could be useful for tuning Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. See also '-search.maxLookback' flag, which has the same meaning due to historical reasons
  -search.maxStatusRequestDuration duration
    	The maximum duration for /api/v1/status/* requests (default 5m0s)
  -search.maxStepForPointsAdjustment duration
    	The maximum step when /api/v1/query_range handler adjusts points with timestamps closer than -search.latencyOffset to the current time. The adjustment is needed because such points may contain incomplete data (default 1m0s)
  -search.maxTagKeys int
    	The maximum number of tag keys returned from /api/v1/labels (default 100000)
  -search.maxTagValueSuffixesPerSearch int
    	The maximum number of tag value suffixes returned from /metrics/find (default 100000)
  -search.maxTagValues int
    	The maximum number of tag values returned from /api/v1/label/<label_name>/values (default 100000)
  -search.maxUniqueTimeseries int
    	The maximum number of unique time series each search can scan. This option allows limiting memory usage (default 300000)
  -search.minStalenessInterval duration
    	The minimum interval for staleness calculations. This flag could be useful for removing gaps on graphs generated from time series with irregular intervals between samples. See also '-search.maxStalenessInterval'
  -search.noStaleMarkers
    	Set this flag to true if the database doesn't contain Prometheus stale markers, so there is no need in spending additional CPU time on its handling. Staleness markers may exist only in data obtained from Prometheus scrape targets
  -search.queryStats.lastQueriesCount int
    	Query stats for /api/v1/status/top_queries is tracked on this number of last queries. Zero value disables query stats tracking (default 20000)
  -search.queryStats.minQueryDuration duration
    	The minimum duration for queries to track in query stats at /api/v1/status/top_queries. Queries with lower duration are ignored in query stats (default 1ms)
  -search.resetCacheAuthKey string
    	Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call
  -search.treatDotsAsIsInRegexps
    	Whether to treat dots as is in regexp label filters used in queries. For example, foo{bar=~"a.b.c"} will be automatically converted to foo{bar=~"a\\.b\\.c"}, i.e. all the dots in regexp filters will be automatically escaped in order to match only dot char instead of matching any char. Dots in ".+", ".*" and ".{n}" regexps aren't escaped. This option is DEPRECATED in favor of {__graphite__="a.*.c"} syntax for selecting metrics matching the given Graphite metrics filter
  -selfScrapeInstance string
    	Value for 'instance' label, which is added to self-scraped metrics (default "self")
  -selfScrapeInterval duration
    	Interval for self-scraping own metrics at /metrics page
  -selfScrapeJob string
    	Value for 'job' label, which is added to self-scraped metrics (default "victoria-metrics")
  -smallMergeConcurrency int
    	The maximum number of CPU cores to use for small merges. Default value is used if set to 0
  -snapshotAuthKey string
    	authKey, which must be passed in query string to /snapshot* pages
  -sortLabels
    	Whether to sort labels for incoming samples before writing them to storage. This may be needed for reducing memory usage at storage when the order of labels in incoming samples is random. For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}. Enabled sorting for labels can slow down ingestion performance a bit
  -storage.maxDailySeries int
    	The maximum number of unique series can be added to the storage during the last 24 hours. Excess series are logged and dropped. This can be useful for limiting series churn rate. See also -storage.maxHourlySeries
  -storage.maxHourlySeries int
    	The maximum number of unique series can be added to the storage during the last hour. Excess series are logged and dropped. This can be useful for limiting series cardinality. See also -storage.maxDailySeries
  -storage.minFreeDiskSpaceBytes size
    	The minimum free disk space at -storageDataPath after which the storage stops accepting new data
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 10000000)
  -storageDataPath string
    	Path to storage data (default "victoria-metrics-data")
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
```
