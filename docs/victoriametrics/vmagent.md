---
weight: 3
menu:
  docs:
    parent: victoriametrics
    weight: 3
title: vmagent
tags:
  - metrics
aliases:
  - /vmagent.html
  - /vmagent/index.html
  - /vmagent/
---

`vmagent` is a tiny agent which helps you collect metrics from various sources,
[relabel and filter the collected metrics](https://docs.victoriametrics.com/victoriametrics/relabeling/)
and store them in [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)
or any other storage systems via Prometheus `remote_write` protocol
or via [VictoriaMetrics `remote_write` protocol](#victoriametrics-remote-write-protocol).

See [Quick Start](#quick-start) for details.

![vmagent](vmagent.webp)

## Motivation

While VictoriaMetrics provides an efficient solution to store and observe metrics, our users needed something fast
and RAM friendly to scrape metrics from Prometheus-compatible exporters into VictoriaMetrics.
Also, we found that our user's infrastructure are like snowflakes in that no two are alike. Therefore, we decided to add more flexibility
to `vmagent` such as the ability to [accept metrics via popular push protocols](#how-to-push-data-to-vmagent)
additionally to [discovering Prometheus-compatible targets and scraping metrics from them](#how-to-collect-metrics-in-prometheus-format).

## Features

* Can be used as a drop-in replacement for Prometheus for discovering and scraping targets such as [node_exporter](https://github.com/prometheus/node_exporter).
  Note that single-node VictoriaMetrics can also discover and scrape Prometheus-compatible targets in the same way as `vmagent` does -
  see [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter).
* Can add, remove and modify labels (aka tags) via Prometheus relabeling. Can filter data before sending it to remote storage. See [these docs](https://docs.victoriametrics.com/victoriametrics/relabeling/) for details.
* Can accept data via all the ingestion protocols supported by VictoriaMetrics - see [these docs](#how-to-push-data-to-vmagent).
* Can aggregate incoming samples by time and by labels before sending them to remote storage - see [these docs](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/).
* Can replicate collected metrics simultaneously to multiple Prometheus-compatible remote storage systems - see [these docs](#replication-and-high-availability).
* Can save egress network bandwidth usage costs when [VictoriaMetrics remote write protocol](#victoriametrics-remote-write-protocol)
  is used for sending the data to VictoriaMetrics.
* Works smoothly in environments with unstable connections to remote storage. If the remote storage is unavailable, the collected metrics
  are buffered at `-remoteWrite.tmpDataPath`. The buffered metrics are sent to remote storage as soon as the connection
  to the remote storage is repaired. The maximum disk usage for the buffer can be limited with `-remoteWrite.maxDiskUsagePerURL`.
* Uses much lower amounts of RAM, CPU, disk IO and network bandwidth than Prometheus. The RAM usage and CPU usage can be reduced further
  if needed according to [these docs](#performance-optimizations).
* Scrape targets can be spread among multiple `vmagent` instances when big number of targets must be scraped. See [these docs](#scraping-big-number-of-targets).
* Can load scrape configs from multiple files. See [these docs](#loading-scrape-configs-from-multiple-files).
* Can efficiently scrape targets that expose millions of time series such as [/federate endpoint in Prometheus](https://prometheus.io/docs/prometheus/latest/federation/).
  See [these docs](#stream-parsing-mode).
* Can deal with [high cardinality](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-cardinality)
  and [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate) issues by limiting the number of unique time series at scrape time
  and before sending them to remote storage systems. See [these docs](#cardinality-limiter).
* Can write collected metrics to multiple tenants. See [these docs](#multitenancy).
* Can read and write data from / to Kafka. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/).
* Can read and write data from / to Google PubSub. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/).

## Quick Start

Please download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) (
`vmagent` is also available in docker images [Docker Hub](https://hub.docker.com/r/victoriametrics/vmagent/tags) and [Quay](https://quay.io/repository/victoriametrics/vmagent?tab=tags)),
unpack it and pass the following flags to the `vmagent` binary in order to start scraping Prometheus-compatible targets
and sending the data to the Prometheus-compatible remote storage:

* `-promscrape.config` with the path to [Prometheus config file](https://docs.victoriametrics.com/victoriametrics/sd_configs/) (usually located at `/etc/prometheus/prometheus.yml`).
  The path can point either to local file or to http url. See [scrape config examples](https://docs.victoriametrics.com/victoriametrics/scrape_config_examples/).
  `vmagent` doesn't support some sections of Prometheus config file, so you may need either to delete these sections or
  to run `vmagent` with `-promscrape.config.strictParse=false` command-line flag.
  In this case `vmagent` ignores unsupported sections. See [the list of unsupported sections](#unsupported-prometheus-config-sections).
* `-remoteWrite.url` with Prometheus-compatible remote storage endpoint such as VictoriaMetrics, where to send the data to.
  The `-remoteWrite.url` may refer to [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) address. See [these docs](#srv-urls) for details.

Example command for writing the data received via [supported push-based protocols](#how-to-push-data-to-vmagent)
to [single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) located at `victoria-metrics-host:8428`:

```sh
/path/to/vmagent -remoteWrite.url=https://victoria-metrics-host:8428/api/v1/write
```

See [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format) if you need writing
the data to [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/).

Example command for scraping Prometheus targets and writing the data to single-node VictoriaMetrics:

```sh
/path/to/vmagent -promscrape.config=/path/to/prometheus.yml -remoteWrite.url=https://victoria-metrics-host:8428/api/v1/write
```

See [how to scrape Prometheus-compatible targets](#how-to-collect-metrics-in-prometheus-format) for more details.

If you use single-node VictoriaMetrics, then you can discover and scrape Prometheus-compatible targets directly from VictoriaMetrics
without the need to use `vmagent` - see [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter).

`vmagent` can save network bandwidth usage costs under high load when [VictoriaMetrics remote write protocol is used](#victoriametrics-remote-write-protocol).

See [troubleshooting docs](#troubleshooting) if you encounter common issues with `vmagent`.

See [various use cases](#use-cases) for vmagent.

Pass `-help` to `vmagent` in order to see [the full list of supported command-line flags with their descriptions](#advanced-usage).

## Use cases

### IoT and Edge monitoring

`vmagent` can run and collect metrics in IoT environments and industrial networks with unreliable or scheduled connections to their remote storage.
It buffers the collected data in local files until the connection to remote storage becomes available and then sends the buffered
data to the remote storage. It re-tries sending the data to remote storage until errors are resolved.
The maximum on-disk size for the buffered metrics can be limited with `-remoteWrite.maxDiskUsagePerURL`.

`vmagent` works on various architectures from the IoT world - 32-bit arm, 64-bit arm, ppc64, 386, amd64.

The `vmagent` can save network bandwidth usage costs by using [VictoriaMetrics remote write protocol](#victoriametrics-remote-write-protocol).

### Drop-in replacement for Prometheus

If you use Prometheus only for scraping metrics from various targets and forwarding these metrics to remote storage
then `vmagent` can replace Prometheus. Typically, `vmagent` requires lower amounts of RAM, CPU and network bandwidth compared with Prometheus.
See [these docs](#how-to-collect-metrics-in-prometheus-format) for details.

### Statsd alternative

`vmagent` can be used as an alternative to [statsd](https://github.com/statsd/statsd)
when [stream aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/) is enabled.
See [these docs](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#statsd-alternative) for details.

### Flexible metrics relay

`vmagent` can accept metrics in [various popular data ingestion protocols](#how-to-push-data-to-vmagent), apply [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/)
to the accepted metrics (for example, change metric names/labels or drop unneeded metrics) and then forward the relabeled metrics
to other remote storage systems, which support Prometheus `remote_write` protocol (including other `vmagent` instances).

### Replication and high availability

`vmagent` replicates the collected metrics among multiple remote storage instances configured via `-remoteWrite.url` args.
If a single remote storage instance temporarily is out of service, then the collected data remains available in another remote storage instance.
`vmagent` buffers the collected data in files at `-remoteWrite.tmpDataPath` until the remote storage becomes available again,
and then it sends the buffered data to the remote storage in order to prevent data gaps.

[VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) already supports replication,
so there is no need in specifying multiple `-remoteWrite.url` flags when writing data to the same cluster.
See [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety).

### Sharding among remote storages

By default `vmagent` replicates data among remote storage systems enumerated via `-remoteWrite.url` command-line flag.
If the `-remoteWrite.shardByURL` command-line flag is set, then `vmagent` spreads evenly
the outgoing [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) among all the remote storage systems
enumerated via `-remoteWrite.url`.

It is possible to replicate samples among remote storage systems by passing `-remoteWrite.shardByURLReplicas=N`
command-line flag to `vmagent` additionally to `-remoteWrite.shardByURL` command-line flag.
This instructs `vmagent` writing every outgoing sample to `N` distinct remote storage systems enumerated via `-remoteWrite.url`
in addition to sharding.

Samples for the same time series are routed to the same remote storage system if `-remoteWrite.shardByURL` flag is specified.
This allows building scalable data processing pipelines when a single remote storage cannot keep up with the data ingestion workload.
For example, this allows building horizontally scalable [stream aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/)
by routing outgoing samples for the same time series of [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter)
and [histogram](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#histogram) types from top-level `vmagent` instances
to the same second-level `vmagent` instance, so they are aggregated properly.

If `-remoteWrite.shardByURL` command-line flag is set, then all the metric labels are used for even sharding
among remote storage systems specified in `-remoteWrite.url`.

Sometimes it may be needed to use only a particular set of labels for sharding. For example, it may be needed to route all the metrics with the same `instance` label
to the same `-remoteWrite.url`. In this case you can specify comma-separated list of these labels in the `-remoteWrite.shardByURL.labels`
command-line flag. For example, `-remoteWrite.shardByURL.labels=instance,__name__` would shard metrics with the same name and `instance`
label to the same `-remoteWrite.url`.

Sometimes, it may be necessary to ignore some labels when sharding samples across multiple `-remoteWrite.url` backends.
For example, if all the [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) with the same set of labels
except of `instance` and `pod` labels must be routed to the same backend. In this case the list of ignored labels must be passed to
`-remoteWrite.shardByURL.ignoreLabels` command-line flag: `-remoteWrite.shardByURL.ignoreLabels=instance,pod`.

See also [how to scrape big number of targets](#scraping-big-number-of-targets).

### Relabeling and filtering

`vmagent` can add, remove or update labels on the collected data before sending it to the remote storage. Additionally,
it can remove unwanted samples via Prometheus-like relabeling before sending the collected data to remote storage.
Please see [Relabeling cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/) for details.

### Splitting data streams among multiple systems

`vmagent` supports splitting the collected data between multiple destinations with the help of `-remoteWrite.urlRelabelConfig`,
which is applied independently for each configured `-remoteWrite.url` destination. For example, it is possible to replicate or split
data among long-term remote storage, short-term remote storage and a real-time analytical system [built on top of Kafka](https://github.com/Telefonica/prometheus-kafka-adapter).
Note that each destination can receive its own subset of the collected data due to per-destination relabeling via `-remoteWrite.urlRelabelConfig`.

For example, let's assume all the scraped or received metrics by `vmagent` have label `env` with values `dev` or `prod`.
To route metrics `env=dev` to destination `dev` and metrics with `env=prod` to destination `prod` apply the following config:
1. Create relabeling config file `relabelDev.yml` to drop all metrics that don't have label `env=dev`:
```yaml
- action: keep
  source_labels: [env]
  regex: "dev"
```
2. Create relabeling config file `relabelProd.yml` to drop all metrics that don't have label `env=prod`:
```yaml
- action: keep
  source_labels: [env]
  regex: "prod"
```
3. Configure `vmagent` with 2 `-remoteWrite.url` flags pointing to destinations `dev` and `prod` with corresponding
`-remoteWrite.urlRelabelConfig` configs:
```sh
./vmagent \
  -remoteWrite.url=http://<dev-url> -remoteWrite.urlRelabelConfig=relabelDev.yml \
  -remoteWrite.url=http://<prod-url> -remoteWrite.urlRelabelConfig=relabelProd.yml 
```
With this configuration `vmagent` will forward to `http://<dev-url>` only metrics that have `env=dev` label.
And to `http://<prod-url>` it will forward only metrics that have `env=prod` label.

Please note, order of flags is important: 1st mentioned `-remoteWrite.urlRelabelConfig` will be applied to the
1st mentioned `-remoteWrite.url`, and so on.

### Prometheus remote_write proxy

`vmagent` can be used as a proxy for Prometheus data sent via Prometheus `remote_write` protocol. It can accept data via the `remote_write` API
at the`/api/v1/write` endpoint. Then apply relabeling and filtering and proxy it to another `remote_write` system .
The `vmagent` can be configured to encrypt the incoming `remote_write` requests with `-tls*` command-line flags.
Also, Basic Auth can be enabled for the incoming `remote_write` requests with `-httpAuth.*` command-line flags.

### remote_write for clustered version

While `vmagent` can accept data in several supported protocols (OpenTSDB, Influx, Prometheus, Graphite) and scrape data from various targets,
writes are always performed in Prometheus remote_write protocol. Therefore, for the [clustered version](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/),
the `-remoteWrite.url` command-line flag should be configured as `<schema>://<vminsert-host>:8480/insert/<accountID>/prometheus/api/v1/write`
according to [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format).
There is also support for multitenant writes. See [these docs](#multitenancy).

### Flexible deduplication

[Deduplication at stream aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#deduplication) allows setting up arbitrary complex de-duplication schemes
for the collected samples. Examples:

- The following command instructs `vmagent` to send only the last sample per each seen [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) per every 60 seconds:
  ```sh
  ./vmagent -remoteWrite.url=http://remote-storage/api/v1/write -streamAggr.dedupInterval=60s
  ```

- The following command instructs `vmagent` to merge [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) with different `replica` label values
  and then to send only the last sample per each merged series per every 60 seconds:
  ```sh
  ./vmagent -remoteWrite=http://remote-storage/api/v1/write -streamAggr.dropInputLabels=replica -streamAggr.dedupInterval=60s
  ```

## How to push data to vmagent

`vmagent` supports [the same set of push-based data ingestion protocols as VictoriaMetrics does](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data)
in addition to the pull-based Prometheus-compatible targets' scraping:

* DataDog "submit metrics" API. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/datadog/).
* InfluxDB line protocol via `http://<vmagent>:8429/write`. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/).
* Graphite plaintext protocol if `-graphiteListenAddr` command-line flag is set. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting).
* OpenTelemetry http API. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#sending-data-via-opentelemetry).
* NewRelic API. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/newrelic/#sending-data-from-agent).
* OpenTSDB telnet and http protocols if `-opentsdbListenAddr` command-line flag is set. See [these docs](https://docs.victoriametrics.com/victoriametrics/integrations/opentsdb/).
* Prometheus remote write protocol via `http://<vmagent>:8429/api/v1/write`.
* JSON lines import protocol via `http://<vmagent>:8429/api/v1/import`. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-json-line-format).
* Native data import protocol via `http://<vmagent>:8429/api/v1/import/native`. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-native-format).
* Prometheus exposition format via `http://<vmagent>:8429/api/v1/import/prometheus`. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format) for details.
* Arbitrary CSV data via `http://<vmagent>:8429/api/v1/import/csv`. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-csv-data).

## How to collect metrics in Prometheus format

Specify the path to `prometheus.yml` file via `-promscrape.config` command-line flag. `vmagent` takes into account the following
sections from [Prometheus config file](https://prometheus.io/docs/prometheus/latest/configuration/configuration/):

* `global`
* `scrape_configs`

All other sections are ignored, including the [remote_write](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write) section.
Use `-remoteWrite.*` command-line flag instead for configuring remote write settings. See [the list of unsupported config sections](#unsupported-prometheus-config-sections).

The file pointed by `-promscrape.config` may contain `%{ENV_VAR}` placeholders which are substituted by the corresponding `ENV_VAR` environment variable values.

See also:

- [scrape config examples](https://docs.victoriametrics.com/victoriametrics/scrape_config_examples/)
- [the list of supported service discovery types for Prometheus scrape targets](https://docs.victoriametrics.com/victoriametrics/sd_configs/).


### scrape_config enhancements

`vmagent` supports the following additional options in [scrape_configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) section:

* `headers` - a list of HTTP headers to send to scrape target with each scrape request. This can be used when the scrape target
  needs custom authorization and authentication. For example:

```yaml
scrape_configs:
- job_name: custom_headers
  headers:
  - "TenantID: abc"
  - "My-Auth: TopSecret"
```

* `disable_compression: true` for disabling response compression on a per-job basis. By default, `vmagent` requests compressed responses
  from scrape targets for saving network bandwidth.
* `disable_keepalive: true` for disabling [HTTP keep-alive connections](https://en.wikipedia.org/wiki/HTTP_persistent_connection)
  on a per-job basis. By default, `vmagent` uses keep-alive connections to scrape targets for reducing overhead on connection re-establishing.
* `series_limit: N` for limiting the number of unique time series a single scrape target can expose. See [these docs](#cardinality-limiter).
* `stream_parse: true` for scraping targets in a streaming manner. This may be useful when targets export big number of metrics. See [these docs](#stream-parsing-mode).
* `scrape_align_interval: duration` for aligning scrapes to the given interval instead of using random offset
  in the range `[0 ... scrape_interval]` for scraping each target. The random offset helps to spread scrapes evenly in time.
* `scrape_offset: duration` for specifying the exact offset for scraping instead of using random offset in the range `[0 ... scrape_interval]`.

See [scrape_configs docs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) for more details on all the supported options.

### Loading scrape configs from multiple files

`vmagent` supports loading [scrape configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) from multiple files specified
in the `scrape_config_files` section of `-promscrape.config` file. For example, the following `-promscrape.config` instructs `vmagent`
loading scrape configs from all the `*.yml` files under `configs` directory, from `single_scrape_config.yml` local file
and from `https://config-server/scrape_config.yml` url:

```yaml
scrape_config_files:
- configs/*.yml
- single_scrape_config.yml
- https://config-server/scrape_config.yml
```

Every referred file can contain arbitrary number of [supported scrape configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs).
There is no need in specifying top-level `scrape_configs` section in these files. For example:

```yaml
- job_name: foo
  static_configs:
  - targets: ["vmagent:8429"]
- job_name: bar
  kubernetes_sd_configs:
  - role: pod
```

`vmagent` is able to dynamically reload these files - see [these docs](#configuration-update).

### Unsupported Prometheus config sections

`vmagent` doesn't support the following sections in Prometheus config file passed to `-promscrape.config` command-line flag:

* [remote_write](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write). This section is substituted
  with various `-remoteWrite*` command-line flags. See [the full list of flags](#advanced-usage). The `remote_write` section isn't supported
  in order to reduce possible confusion when `vmagent` is used for accepting incoming metrics via [supported push protocols](#how-to-push-data-to-vmagent).
  In this case the `-promscrape.config` file isn't needed.
* `remote_read`. This section isn't supported at all, since `vmagent` doesn't provide Prometheus querying API.
  It is expected that the querying API is provided by the remote storage specified via `-remoteWrite.url` such as VictoriaMetrics.
  See [Prometheus querying API docs for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage).
* `rule_files` and `alerting`. These sections are supported by [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/).

The list of supported service discovery types is available [here](#how-to-collect-metrics-in-prometheus-format).

Additionally, `vmagent` doesn't support `refresh_interval` option at service discovery sections.
This option is substituted with `-promscrape.*CheckInterval` command-line flags, which are specific per each service discovery type.
See [the full list of command-line flags for vmagent](#advanced-usage).

## Configuration update

`vmagent` should be restarted in order to update config options set via command-line args.
`vmagent` supports multiple approaches for reloading configs from updated config files such as
`-promscrape.config`, `-remoteWrite.relabelConfig`, `-remoteWrite.urlRelabelConfig`, `-streamAggr.config`
and `-remoteWrite.streamAggr.config`:

* Sending `SIGHUP` signal to `vmagent` process:

  ```sh
  kill -SIGHUP `pidof vmagent`
  ```

* Sending HTTP request to `http://vmagent:8429/-/reload` endpoint. This endpoint can be protected with `-reloadAuthKey` command-line flag.

There is also `-promscrape.configCheckInterval` command-line flag, which can be used for automatic reloading configs from updated `-promscrape.config` file.

## SRV urls

If `vmagent` encounters urls with `srv+` prefix in hostname (such as `http://srv+some-addr/some/path`), then it resolves `some-addr` [DNS SRV](https://en.wikipedia.org/wiki/SRV_record)
record into TCP address with hostname and TCP port, and then uses the resulting url when it needs connecting to it.

SRV urls are supported in the following places:

- In `-remoteWrite.url` command-line flag. For example, if `victoria-metrics` [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) record contains
  `victoria-metrics-host:8428` TCP address, then `-remoteWrite.url=http://srv+victoria-metrics/api/v1/write` is automatically resolved into
  `-remoteWrite.url=http://victoria-metrics-host:8428/api/v1/write`. If the DNS SRV record is resolved into multiple TCP addresses, then `vmagent`
  uses randomly chosen address per each connection it establishes to the remote storage.

- In scrape target addresses aka `__address__` label - see [these docs](https://docs.victoriametrics.com/victoriametrics/relabeling/#how-to-modify-scrape-urls-in-targets) for details.

- In urls used for [service discovery](https://docs.victoriametrics.com/victoriametrics/sd_configs/).

SRV urls are useful when HTTP services run on different TCP ports or when they can change TCP ports over time (for instance, after the restart).

## VictoriaMetrics remote write protocol

`vmagent` supports sending data to the configured `-remoteWrite.url` either via Prometheus remote write protocol
or via VictoriaMetrics remote write protocol.

VictoriaMetrics remote write protocol provides the following benefits comparing to Prometheus remote write protocol:

- Reduced network bandwidth usage by 2x-5x. This allows saving network bandwidth usage costs when `vmagent` and
  the configured remote storage systems are located in different datacenters, availability zones or regions.

- Reduced disk read/write IO and disk space usage at `vmagent` when the remote storage is temporarily unavailable.
  In this case `vmagent` buffers the incoming data to disk using the VictoriaMetrics remote write format.
  This reduces disk read/write IO and disk space usage by 2x-5x comparing to Prometheus remote write format.

`vmagent` uses VictoriaMetrics remote write protocol by default {{% available_from "v1.116.0" %}} when it sends data to VictoriaMetrics components such as other `vmagent` instances,
[single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
or `vminsert` at [cluster version](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/). If needed, It can automatically downgrade to a Prometheus protocol at runtime.
It is possible to force switch to VictoriaMetrics remote write protocol by specifying `-remoteWrite.forceVMProto`
command-line flag for the corresponding `-remoteWrite.url`.
It is possible to tune the compression level for VictoriaMetrics remote write protocol with `-remoteWrite.vmProtoCompressLevel` command-line flag.
Bigger values reduce network usage at the cost of higher CPU usage. Negative values reduce CPU usage at the cost of higher network usage.
The default value for the compression level is `0`, the minimum value is `-22` and the maximum value is `22`. The default value works optimally
in most cases, so it isn't recommended changing it.

`vmagent` automatically switches to Prometheus remote write protocol when it sends data to old versions of VictoriaMetrics components
or to other Prometheus-compatible remote storage systems. It is possible to force switch to Prometheus remote write protocol
by specifying `-remoteWrite.forcePromProto` command-line flag for the corresponding `-remoteWrite.url`.

## Multitenancy

By default `vmagent` collects the data without [tenant](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy) identifiers
and routes it to the remote storage specified via `-remoteWrite.url` command-line flag. The `-remoteWrite.url` can point to `/insert/<tenant_id>/prometheus/api/v1/write` path
at `vminsert` according to [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format). In this case all the metrics are written to the given `<tenant_id>` tenant.

The easiest way to write data to multiple distinct tenants is to specify the needed tenants via `vm_account_id` and `vm_project_id` labels
and then to push metrics with these labels to [multitenant url at VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels).
The `vm_account_id` and `vm_project_id` labels can be specified via [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/) before sending the metrics to `-remoteWrite.url`.

For example, the following relabeling rule instructs sending metrics to `<account_id>:0` [tenant](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
defined in the `prometheus.io/account_id` annotation of Kubernetes pod deployment:

```yaml
scrape_configs:
- kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_account_id]
    target_label: vm_account_id
```

`vmagent` can accept data via the same multitenant endpoints (`/insert/<accountID>/<suffix>`) as `vminsert` at [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/)
does according to [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format) if `-enableMultitenantHandlers` command-line flag is set.
In this case, vmagent automatically converts tenant identifiers from the URL to `vm_account_id` and `vm_project_id` labels.
These tenant labels are added before applying [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/) specified via `-remoteWrite.relabelConfig`
and `-remoteWrite.urlRelabelConfig` command-line flags. Metrics with `vm_account_id` and `vm_project_id` labels can be routed to the corresponding tenants
when specifying `-remoteWrite.url` to [multitenant url at VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels).

## Adding labels to metrics

Extra labels can be added to metrics collected by `vmagent` via the following mechanisms:

* The `global -> external_labels` section in `-promscrape.config` file. These labels are added only to metrics scraped from targets configured
  in the `-promscrape.config` file. They aren't added to metrics collected via other [data ingestion protocols](#how-to-push-data-to-vmagent).
* The `-remoteWrite.label` command-line flag. These labels are added to all the collected metrics before sending them to `-remoteWrite.url`.
  For example, the following command starts `vmagent`, which adds `{datacenter="foobar"}` label to all the metrics pushed
  to all the configured remote storage systems (all the `-remoteWrite.url` flag values):

  ```sh
  /path/to/vmagent -remoteWrite.label=datacenter=foobar ...
  ```

* Via relabeling. See [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/).

## Automatically generated metrics

`vmagent` automatically generates the following metrics per each scrape of every [Prometheus-compatible target](#how-to-collect-metrics-in-prometheus-format)
and attaches `instance`, `job` and other target-specific labels to these metrics:

* `up` - this metric exposes `1` value on successful scrape and `0` value on unsuccessful scrape. This allows monitoring
  failing scrapes with the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/):

  ```metricsql
  up == 0
  ```

* `scrape_duration_seconds` - the duration of the scrape for the given target. This allows monitoring slow scrapes.
  For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/) returns scrapes,
  which take more than 1.5 seconds to complete:

  ```metricsql
  scrape_duration_seconds > 1.5
  ```

* `scrape_timeout_seconds` - the configured timeout for the current scrape target (aka `scrape_timeout`).
  This allows detecting targets with scrape durations close to the configured scrape timeout.
  For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/) returns targets (identified by `instance` label),
  which take more than 80% of the configured `scrape_timeout` during scrapes:

  ```metricsql
  scrape_duration_seconds / scrape_timeout_seconds > 0.8
  ```

* `scrape_response_size_bytes` - response size in bytes for the given target. This allows to monitor amount of data scraped
  and to adjust [`max_scrape_size` option](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) for scraped targets.
  For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/) returns targets with scrape response
  bigger than `10MiB`:

  ```metricsql
  scrape_response_size_bytes > 10MiB
  ```

* `scrape_samples_scraped` - the number of [samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) parsed per each scrape. This allows detecting targets,
  which expose too many [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series). For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/)
  returns targets, which expose more than 10000 metrics:

  ```metricsql
  scrape_samples_scraped > 10000
  ```

* `scrape_samples_limit` - the configured limit on the number of [samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) the given target can expose.
  The limit can be set via `sample_limit` option at [scrape_configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs).
  This metric is exposed only if the `sample_limit` is set. This allows detecting targets,
  which expose too many metrics compared to the configured `sample_limit`. For example, the following query
  returns targets (identified by `instance` label), which expose more than 80% metrics compared to the configured `sample_limit`:

  ```metricsql
  scrape_samples_scraped / scrape_samples_limit > 0.8
  ```

* `scrape_samples_post_metric_relabeling` - the number of [samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) left after applying metric-level relabeling
  from `metric_relabel_configs` section (see [relabeling docs](https://docs.victoriametrics.com/victoriametrics/relabeling/) for more details).
  This allows detecting targets with too many [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) after the relabeling.
  For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/) returns targets
  with more than 10000 metrics after the relabeling:

  ```metricsql
  scrape_samples_post_metric_relabeling > 10000
  ```

* `scrape_labels_limit` - the configured limit on the number of [labels](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels) the given target can expose
  per [sample](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples).
  The limit can be set via `label_limit` option at [scrape_configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs).
  This metric is exposed only if the `label_limit` is set. 

* `scrape_series_added` - **an approximate** number of new [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) the given target generates during the current scrape.
  This metric allows detecting targets (identified by `instance` label),
  which lead to [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).
  For example, the following [MetricsQL query](https://docs.victoriametrics.com/victoriametrics/metricsql/) returns targets,
  which generate more than 1000 new series during the last hour:

  ```metricsql
  sum_over_time(scrape_series_added[1h]) > 1000
  ```

  `vmagent` sets `scrape_series_added` to zero when it runs with `-promscrape.noStaleMarkers` command-line flag
  or when it scrapes target with `no_stale_markers: true` option, e.g. when [staleness markers](#prometheus-staleness-markers) are disabled.

* `scrape_series_limit` - the limit on the number of unique [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) the given target can expose according to [these docs](#cardinality-limiter).
  This metric is exposed only if the series limit is set.

* `scrape_series_current` - the number of unique [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) the given target exposed so far.
  This metric is exposed only if the series limit is set according to [these docs](#cardinality-limiter).
  This metric allows alerting when the number of exposed series by the given target reaches the limit.
  For example, the following query would alert when the target exposes more than 90% of unique series compared to the configured limit.

  ```metricsql
  scrape_series_current / scrape_series_limit > 0.9
  ```

* `scrape_series_limit_samples_dropped` - exposes the number of dropped samples during the scrape because of the exceeded limit
  on the number of unique [series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series). This metric is exposed only if the series limit is set according to [these docs](#cardinality-limiter).
  This metric allows alerting when scraped samples are dropped because of the exceeded limit.
  For example, the following query alerts when at least a single sample is dropped because of the exceeded limit during the last hour:

  ```metricsql
  sum_over_time(scrape_series_limit_samples_dropped[1h]) > 0
  ```

If the target exports metrics with names clashing with the automatically generated metric names, then `vmagent` automatically
adds `exported_` prefix to these metric names, so they don't clash with automatically generated metric names.

Relabeling defined in `relabel_configs` or `metric_relabel_configs` of scrape config isn't applied to automatically
generated metrics. But they still can be relabeled via `-remoteWrite.relabelConfig` before sending metrics to remote address.

## Prometheus staleness markers

`vmagent` sends [Prometheus staleness markers](https://www.robustperception.io/staleness-and-promql) to `-remoteWrite.url` in the following cases:

* If they are passed to `vmagent` via [Prometheus remote_write protocol](#prometheus-remote_write-proxy).
* If the metric disappears from the list of scraped metrics, then stale marker is sent to this particular metric.
* If the scrape target becomes temporarily unavailable, then stale markers are sent for all the metrics scraped from this target.
* If the scrape target is removed from the list of targets, then stale markers are sent for all the metrics scraped from this target.

Prometheus staleness markers' tracking needs additional memory, since it must store the previous response body per each scrape target
in order to compare it to the current response body. The memory usage may be reduced by disabling staleness tracking in the following ways:

* By passing `-promscrape.noStaleMarkers` command-line flag to `vmagent`. This disables staleness tracking across all the targets.
* By specifying `no_stale_markers: true` option in the [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) for the corresponding target.

When staleness tracking is disabled, then `vmagent` doesn't track the number of new time series per each scrape,
e.g. it sets `scrape_series_added` metric to zero. See [these docs](#automatically-generated-metrics) for details.

## Metric metadata

By default, `vmagent` ignores metric metadata exposed by scrape targets in [Prometheus exposition format](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md), received via [Prometheus remote write v1](https://prometheus.io/docs/specs/prw/remote_write_spec/) or [OpenTelemetry protocol](https://github.com/open-telemetry/opentelemetry-proto/blob/v1.7.0/opentelemetry/proto/metrics/v1/metrics.proto). Set `-enableMetadata=true` to enable metadata processing{{% available_from "v1.125.1" %}}.
During processing, metadata won't be dropped or modified by [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/) or [streaming aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/).

When `-enableMultitenantHandlers` is enabled, vmagent adds tenant info to metadata received via the [multitenant endpoints](https://docs.victoriametrics.com/victoriametrics/vmagent/#multitenancy) (/insert/<accountID>/<suffix>). However, if `vm_account_id` or `vm_project_id` labels are added directly to metrics before reaching vmagent, and vmagent writes to the [vminsert multitenant endpoints](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels), the tenant info won't be attached and the metadata will be stored under the default tenant of VictoriaMetrics cluster.

>Enabling metadata requires extra memory, disk space, and network traffic.

## Stream parsing mode

By default, `vmagent` parses the full response from the scrape target, applies [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/)
and then pushes the resulting metrics to the configured `-remoteWrite.url` in one go. This mode works good for the majority of cases
when the scrape target exposes small number of metrics (e.g. less than 10K). But this mode may take big amounts of memory
when the scrape target exposes big number of metrics (for example, when `vmagent` scrapes [`kube-state-metrics`](https://github.com/kubernetes/kube-state-metrics)
in large Kubernetes cluster). It is recommended enabling stream parsing mode for such targets.
When this mode is enabled, `vmagent` processes the response from the scrape target in chunks.
This allows saving memory when scraping targets that expose millions of metrics.

Stream parsing mode is automatically enabled for scrape targets returning response bodies with sizes bigger than
the `-promscrape.minResponseSizeForStreamParse` command-line flag value. Additionally,
stream parsing mode can be explicitly enabled in the following places:

* Via `-promscrape.streamParse` command-line flag. In this case all the scrape targets defined
  in the file pointed by `-promscrape.config` are scraped in stream parsing mode.
* Via `stream_parse: true` option at `scrape_configs` section. In this case all the scrape targets defined
  in this section are scraped in stream parsing mode.
* Via `__stream_parse__=true` label, which can be set via [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/) at `relabel_configs` section.
  In this case stream parsing mode is enabled for the corresponding scrape targets.
  Typical use case: to set the label via [Kubernetes annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)
  for targets exposing big number of metrics.

Examples:

```yaml
scrape_configs:
- job_name: 'big-federate'
  stream_parse: true
  static_configs:
  - targets:
    - big-prometheus1
    - big-prometheus2
  honor_labels: true
  metrics_path: /federate
  params:
    'match[]': ['{__name__!=""}']
```

Note that `vmagent` in stream parsing mode stores up to `sample_limit` samples to the configured `-remoteStorage.url`
instead of dropping all the samples read from the target, because the parsed data is sent to the remote storage
as soon as it is parsed in stream parsing mode.

## Scraping big number of targets

A single `vmagent` instance can scrape tens of thousands of scrape targets. Sometimes this isn't enough due to limitations on CPU, network, RAM, etc.
In this case scrape targets can be split among multiple `vmagent` instances (aka `vmagent` horizontal scaling, sharding and clustering).
The number of `vmagent` instances in the cluster must be passed to `-promscrape.cluster.membersCount` command-line flag.
Each `vmagent` instance in the cluster must use identical `-promscrape.config` files with distinct `-promscrape.cluster.memberNum` values
in the range `0 ... N-1`, where `N` is the number of `vmagent` instances in the cluster specified via `-promscrape.cluster.membersCount`.
For example, the following commands spread scrape targets among a cluster of two `vmagent` instances:

```sh
/path/to/vmagent -promscrape.cluster.membersCount=2 -promscrape.cluster.memberNum=0 -promscrape.config=/path/to/config.yml ...
/path/to/vmagent -promscrape.cluster.membersCount=2 -promscrape.cluster.memberNum=1 -promscrape.config=/path/to/config.yml ...
```

The `-promscrape.cluster.memberNum` can be set to a StatefulSet pod name when `vmagent` runs in Kubernetes.
The pod name must end with a number in the range `0 ... promscrape.cluster.membersCount-1`. For example, `-promscrape.cluster.memberNum=vmagent-0`.

By default, each scrape target is scraped only by a single `vmagent` instance in the cluster. If there is a need for replicating scrape targets among multiple `vmagent` instances,
then `-promscrape.cluster.replicationFactor` command-line flag must be set to the desired number of replicas. For example, the following commands
start a cluster of three `vmagent` instances, where each target is scraped by two `vmagent` instances:

```sh
/path/to/vmagent -promscrape.cluster.membersCount=3 -promscrape.cluster.replicationFactor=2 -promscrape.cluster.memberNum=0 -promscrape.config=/path/to/config.yml ...
/path/to/vmagent -promscrape.cluster.membersCount=3 -promscrape.cluster.replicationFactor=2 -promscrape.cluster.memberNum=1 -promscrape.config=/path/to/config.yml ...
/path/to/vmagent -promscrape.cluster.membersCount=3 -promscrape.cluster.replicationFactor=2 -promscrape.cluster.memberNum=2 -promscrape.config=/path/to/config.yml ...
```

Every `vmagent` in the cluster exposes all the discovered targets at `http://vmagent:8429/service-discovery` page.
Each discovered target on this page contains its status (`UP`, `DOWN` or `DROPPED` with the reason why the target has been dropped).
If the target is dropped because of sharding to other `vmagent` instances in the cluster, then the status column contains
`-promscrape.cluster.memberNum` values for `vmagent` instances where the given target is scraped.

The `/service-discovery` page provides links to the corresponding `vmagent` instances if `-promscrape.cluster.memberURLTemplate` command-line flag is set.
Every occurrence of `%d` inside the `-promscrape.cluster.memberURLTemplate` is substituted with the `-promscrape.cluster.memberNum`
for the corresponding `vmagent` instance. For example, `-promscrape.cluster.memberURLTemplate='http://vmagent-instance-%d:8429/targets'`
generates `http://vmagent-instance-42:8429/targets` url for `vmagent` instance, which runs with `-promscrape.cluster.memberNum=42`.

Note that `vmagent` shows up to `-promscrape.maxDroppedTargets` dropped targets on the `/service-discovery` page.
Increase the `-promscrape.maxDroppedTargets` command-line flag value if the `/service-discovery` page misses some dropped targets.

If each target is scraped by multiple `vmagent` instances, then data deduplication must be enabled at remote storage pointed by `-remoteWrite.url`.
The `-dedup.minScrapeInterval` must be set to the `scrape_interval` configured at `-promscrape.config`.
See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication) for details.

The `-promscrape.cluster.memberLabel` command-line flag allows specifying a name for `member num` label to add to all the scraped metrics.
The value of the `member num` label is set to `-promscrape.cluster.memberNum`. For example, the following config instructs adding `vmagent_instance="0"` label
to all the metrics scraped by the given `vmagent` instance:

```sh
/path/to/vmagent -promscrape.cluster.membersCount=2 -promscrape.cluster.memberNum=0 -promscrape.cluster.memberLabel=vmagent_instance
```

See also [how to shard data among multiple remote storage systems](#sharding-among-remote-storages).

## High availability

It is possible to run multiple **identically configured** `vmagent` instances or `vmagent` 
[clusters](#scraping-big-number-of-targets), so they [scrape](#how-to-collect-metrics-in-prometheus-format) 
the same set of targets and push the collected data to the same set of VictoriaMetrics remote storage systems. 
Two **identically configured** vmagent instances or clusters is usually called an HA pair.

When running HA pairs, [deduplication](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication) must be configured 
at VictoriaMetrics side in order to de-duplicate received samples.
See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication) for details.

It is also recommended passing different values to `-promscrape.cluster.name` command-line flag per each `vmagent` 
instance or per each `vmagent` cluster in HA setup. This is needed for proper data de-duplication. 
See [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2679) for details.

## Scraping targets via a proxy

`vmagent` supports scraping targets via http, https and socks5 proxies. Proxy address must be specified in `proxy_url` option. For example, the following scrape config instructs
target scraping via https proxy at `https://proxy-addr:1234`:

```yaml
scrape_configs:
- job_name: foo
  proxy_url: https://proxy-addr:1234
```

Proxy can be configured with the following optional settings:

* `proxy_authorization` for generic token authorization. See [these docs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#http-api-client-options).
* `proxy_basic_auth` for Basic authorization. See [these docs](https://docs.victoriametrics.com/victoriametrics/sd_configs/#http-api-client-options).
* `proxy_bearer_token` and `proxy_bearer_token_file` for Bearer token authorization
* `proxy_oauth2` for OAuth2 config. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2).
* `proxy_tls_config` for TLS config. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config).
* `proxy_headers` for passing additional HTTP headers in requests to proxy.

For example:

```yaml
scrape_configs:
- job_name: foo
  proxy_url: https://proxy-addr:1234
  proxy_basic_auth:
    username: foobar
    password: secret
  proxy_tls_config:
    insecure_skip_verify: true
    cert_file: /path/to/cert
    key_file: /path/to/key
    ca_file: /path/to/ca
    server_name: real-server-name
  proxy_headers:
  - "Proxy-Auth: top-secret"
```

## On-disk persistence

`vmagent` stores pending data that cannot be sent to the configured remote storage systems in a timely manner.
By default, `vmagent` writes all the pending data to folder configured via `-remoteWrite.tmpDataPath` cmd-line flag
until this data is sent to the configured `-remoteWrite.url` systems or until the folder becomes full. 
The maximum data size that can be saved to `-remoteWrite.tmpDataPath` per every configured `-remoteWrite.url` can be 
limited via `-remoteWrite.maxDiskUsagePerURL` command-line flag. When this limit is reached, `vmagent` drops the oldest 
data from disk in order to save newly ingested data.

The folder structure of persistence data is as follows:
```
<remoteWrite.tmpDataPath>
└── persistent-queue
    └── 1_B9EB7BE220B91E9D
```
Each remote write URL corresponds to a folder similar to `1_B9EB7BE220B91E9D`.

It's generated based on the following information:
1. The **sequence order** of the remote write URL cmd-line flags, starting from **1**.
2. The **hash result** of the remote write URL itself, excluding query parameters and fragments.

For example, for the remote write configs:
```
-remoteWrite.url=http://example-1:8428/prometheus/api/v1/write?foo=bar#baz
-remoteWrite.url=http://user:pass@example-2:8428/prometheus/api/v1/write?qux=quux#quuz
```

vmagent will generate the following persistent queue folders:
```bash
# 1_<hash(http://example-1:8428/prometheus/api/v1/write)>, query parameters foo=bar and fragment baz are removed.
1_BA6E4303DCFA0D45
# 2_<hash(http://user:pass@example-2:8428/prometheus/api/v1/write)>, query parameters qux=quux and fragment quuz are removed.
2_0AAFDF53E314A72A
```

### Disabling On-disk persistence

There are cases when it is better disabling on-disk persistence for pending data at `vmagent` side:

- When the persistent disk performance isn't enough for the given data processing rate.
- When it is better to buffer pending data at the client side instead of buffering it at `vmagent` side in the `-remoteWrite.tmpDataPath` folder.
- When the data is already buffered at [Kafka side](#reading-metrics-from-kafka) or at [Google PubSub side](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics).
- When it is better to drop pending data instead of buffering it.

In this case `-remoteWrite.disableOnDiskQueue` command-line flag can be passed to `vmagent` per each configured `-remoteWrite.url`.
`vmagent` works in the following way if the corresponding remote storage system at `-remoteWrite.url` cannot keep up with the data ingestion rate
and the `-remoteWrite.disableOnDiskQueue` command-line flag is set:

- It returns `429 Too Many Requests` HTTP error to clients, which send data to `vmagent` via [supported HTTP endpoints](#how-to-push-data-to-vmagent).
  If `-remoteWrite.dropSamplesOnOverload` command-line flag is set or if multiple `-remoteWrite.url` command-line flags are set, 
  then the ingested samples are silently dropped instead of returning the error to clients.
- It suspends consuming data from [Kafka side](#reading-metrics-from-kafka) or [Google PubSub side](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/) until the remote storage becomes available.
  If `-remoteWrite.dropSamplesOnOverload` command-line flag is set or if multiple `-remoteWrite.disableOnDiskQueue` command-line flags are set
  for different `-remoteWrite.url` options, then the fetched samples are silently dropped instead of suspending data consumption from Kafka or Google PubSub.
- It drops samples pushed to `vmagent` via non-HTTP protocols and logs the error. Pass `-remoteWrite.dropSamplesOnOverload` command-line flag in order
  to suppress error messages in this case.
- It drops samples [scraped from Prometheus-compatible targets](#how-to-collect-metrics-in-prometheus-format), because it is better from operations perspective
  to drop samples instead of blocking the scrape process.
- It drops [stream aggregation](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/) output samples, because it is better from operations perspective
  to drop output samples instead of blocking the stream aggregation process.

The number of dropped samples because of overloaded remote storage can be [monitored](#monitoring) via `vmagent_remotewrite_samples_dropped_total` metric.
The number of unsuccessful attempts to send data to overloaded remote storage can be [monitored](#monitoring) via `vmagent_remotewrite_push_failures_total` metric.

Running `vmagent` on hosts with more RAM or increasing the value for `-memory.allowedPercent` may reduce the number of unsuccessful attempts or dropped samples
on spiky workloads, since `vmagent` may buffer more data in memory before returning the error or dropping data.

`vmagent` still may write pending in-memory data to `-remoteWrite.tmpDataPath` on graceful shutdown
if `-remoteWrite.disableOnDiskQueue` command-line flag is specified. It may also read buffered data from `-remoteWrite.tmpDataPath`
on startup.

When `-remoteWrite.disableOnDiskQueue` command-line flag is set, `vmagent` may send the same samples multiple times to the configured remote storage
if it cannot keep up with the data ingestion rate. In this case the [deduplication](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication)
must be enabled on all the configured remote storage systems.

## Cardinality limiter

By default, `vmagent` doesn't limit the number of time series each scrape target can expose.
The limit can be enforced in the following places:

* Via `-promscrape.seriesLimitPerTarget` command-line flag. This limit is applied individually
  to all the scrape targets defined in the file pointed by `-promscrape.config`.
* Via `series_limit` config option at [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs) section.
  The `series_limit` allows overriding the `-promscrape.seriesLimitPerTarget` on a per-`scrape_config` basis.
  If `series_limit` is set to `0` or to negative value, then it isn't applied to the given `scrape_config`,
  even if `-promscrape.seriesLimitPerTarget` command-line flag is set.
* Via `__series_limit__` label, which can be set with [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/) at `relabel_configs` section.
  The `__series_limit__` allows overriding the `series_limit` on a per-target basis.
  If `__series_limit__` is set to `0` or to negative value, then it isn't applied to the given target.
  Typical use case: to set the limit via [Kubernetes annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)
  for targets, which may expose too high number of time series.

Scraped metrics are dropped for time series exceeding the given limit on the time window of 24h.
`vmagent` creates the following additional per-target metrics for targets with non-zero series limit:

- `scrape_series_limit_samples_dropped` - the number of dropped samples during the scrape when the unique series limit is exceeded.
- `scrape_series_limit` - the series limit for the given target.
- `scrape_series_current` - the current number of series for the given target.

These metrics are automatically sent to the configured `-remoteWrite.url` alongside with the scraped per-target metrics.

These metrics allow building the following alerting rules:

- `scrape_series_current / scrape_series_limit > 0.9` - alerts when the number of series exposed by the target reaches 90% of the limit.
- `sum_over_time(scrape_series_limit_samples_dropped[1h]) > 0` - alerts when some samples are dropped because the series limit on a particular target is reached.

See also `sample_limit` option at [scrape_config section](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs).

By default, `vmagent` doesn't limit the number of time series written to remote storage systems specified at `-remoteWrite.url`.
The limit can be enforced by setting the following command-line flags:

* `-remoteWrite.maxHourlySeries` - limits the number of unique time series `vmagent` can write to remote storage systems during the last hour.
  Useful for limiting the number of active time series.
* `-remoteWrite.maxDailySeries` - limits the number of unique time series `vmagent` can write to remote storage systems during the last day.
  Useful for limiting daily churn rate.

Both limits can be set simultaneously. If any of these limits is reached, then samples for new time series are dropped instead of sending
them to remote storage systems. A sample of dropped series is put in the log with `WARNING` level.

`vmagent` exposes the following metrics at `http://vmagent:8429/metrics` page (see [monitoring docs](#monitoring) for details):

* `vmagent_hourly_series_limit_rows_dropped_total` - the number of metrics dropped due to exceeded hourly limit on the number of unique time series.
* `vmagent_hourly_series_limit_max_series` - the hourly series limit set via `-remoteWrite.maxHourlySeries`.
* `vmagent_hourly_series_limit_current_series` - the current number of unique series registered during the last hour.
* `vmagent_daily_series_limit_rows_dropped_total` - the number of metrics dropped due to exceeded daily limit on the number of unique time series.
* `vmagent_daily_series_limit_max_series` - the daily series limit set via `-remoteWrite.maxDailySeries`.
* `vmagent_daily_series_limit_current_series` - the current number of unique series registered during the last day.

These limits are approximate, so `vmagent` can underflow/overflow the limit by a small percentage (usually less than 1%).

See also [cardinality explorer docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-explorer).

## Monitoring

`vmagent` exports various metrics in Prometheus exposition format at `http://vmagent-host:8429/metrics` page.
We recommend setting up regular scraping of this page either through `vmagent` itself or by Prometheus-compatible scraper,
so that the exported metrics may be analyzed later.

If you use Google Cloud Managed Prometheus for scraping metrics from VictoriaMetrics components, then pass `-metrics.exposeMetadata`
command-line to them, so they add `TYPE` and `HELP` comments per each exposed metric at `/metrics` page.
See [these docs](https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type) for details.

Use official [Grafana dashboard](https://grafana.com/grafana/dashboards/12683) for `vmagent` state overview.
Graphs on this dashboard contain useful hints - hover the `i` icon at the top left corner of each graph in order to read it.
If you have suggestions for improvements or have found a bug - please open an issue on github or add a review to the dashboard.

`vmagent` also exports the status for various targets at the following pages:

* `http://vmagent-host:8429/targets`. This pages shows the current status for every active target.
* `http://vmagent-host:8429/service-discovery`. This pages shows the list of discovered targets with the discovered `__meta_*` labels
  according to [these docs](https://docs.victoriametrics.com/victoriametrics/sd_configs/).
  This page may help debugging target [relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/).
* `http://vmagent-host:8429/api/v1/targets`. This handler returns JSON response
  compatible with [the corresponding page from Prometheus API](https://prometheus.io/docs/prometheus/latest/querying/api/#targets).
* `http://vmagent-host:8429/ready`. This handler returns http 200 status code when `vmagent` finishes
  its initialization for all the [service_discovery configs](https://docs.victoriametrics.com/victoriametrics/sd_configs/).
  It may be useful to perform `vmagent` rolling update without any scrape loss.

## Troubleshooting

* It is recommended [setting up the official Grafana dashboard](#monitoring) in order to monitor the state of `vmagent`.

* It is recommended increasing the maximum number of open files in the system (`ulimit -n`) when scraping a big number of targets,
  as `vmagent` establishes at least a single TCP connection per target.

* If `vmagent` uses too much RAM or CPU, then follow [these recommendations](#performance-optimizations).

* When `vmagent` scrapes many unreliable targets, it can flood the error log with scrape errors. It is recommended investigating and fixing these errors.
  If it is unfeasible to fix all the reported errors, then they can be suppressed by passing `-promscrape.suppressScrapeErrors` command-line flag to `vmagent`.
  The most recent scrape error per each target can be observed at `http://vmagent-host:8429/targets` and `http://vmagent-host:8429/api/v1/targets`.

* The `http://vmagent-host:8429/service-discovery` page could be useful for debugging relabeling process for scrape targets.
  This page contains original labels for targets dropped during relabeling.
  By default, the `-promscrape.maxDroppedTargets` targets are shown here. If your setup drops more targets during relabeling,
  then increase `-promscrape.maxDroppedTargets` command-line flag value to see all the dropped targets.
  Note that tracking each dropped target requires up to 10Kb of RAM. Therefore, big values for `-promscrape.maxDroppedTargets`
  may result in increased memory usage if a big number of scrape targets are dropped during relabeling.

* It is recommended increasing `-remoteWrite.queues` if `vmagent_remotewrite_pending_data_bytes` [metric](#monitoring)
  grows constantly. It is also recommended increasing `-remoteWrite.maxBlockSize` and `-remoteWrite.maxRowsPerBlock` command-line flags in this case.
  This can improve data ingestion performance to the configured remote storage systems at the cost of higher memory usage.

* If you see gaps in the data pushed by `vmagent` to remote storage when `-remoteWrite.maxDiskUsagePerURL` is set,
  try increasing `-remoteWrite.queues`. Such gaps may appear because `vmagent` cannot keep up with sending the collected data to remote storage.
  Therefore, it starts dropping the buffered data if the on-disk buffer size exceeds `-remoteWrite.maxDiskUsagePerURL`.

* `vmagent` drops data blocks if remote storage replies with `400 Bad Request` and `409 Conflict` HTTP responses.
  The number of dropped blocks can be monitored via `vmagent_remotewrite_packets_dropped_total` metric exported at [/metrics page](#monitoring).

* Use `-remoteWrite.queues=1` when `-remoteWrite.url` points to remote storage, which doesn't accept out-of-order samples (aka data backfilling).
  Such storage systems include Prometheus, Mimir, Cortex and Thanos, which typically emit `out of order sample` errors.
  The best solution is to use remote storage with [backfilling support](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling) such as VictoriaMetrics.

* `vmagent` buffers scraped data at the `-remoteWrite.tmpDataPath` directory until it is sent to `-remoteWrite.url`.
  The directory can grow large when remote storage is unavailable for extended periods of time and if the maximum directory size isn't limited
  with `-remoteWrite.maxDiskUsagePerURL` command-line flag.
  If you don't want to send all the buffered data from the directory to remote storage then simply stop `vmagent` and delete the directory.

* If `vmagent` runs on a host with slow persistent storage, which cannot keep up with the volume of processed samples, then it is possible to disable
  the persistent storage with `-remoteWrite.disableOnDiskQueue` command-line flag. See [these docs](#disabling-on-disk-persistence) for more details.

* By default `vmagent` masks `-remoteWrite.url` with `secret-url` values in logs and at `/metrics` page because
  the url may contain sensitive information such as auth tokens or passwords.
  Pass `-remoteWrite.showURL` command-line flag when starting `vmagent` in order to see all the valid urls.

* By default `vmagent` evenly spreads scrape load in time. If a particular scrape target must be scraped at the beginning of some interval,
  then `scrape_align_interval` option  must be used. For example, the following config aligns hourly scrapes to the beginning of hour:

  ```yaml
  scrape_configs:
  - job_name: foo
    scrape_interval: 1h
    scrape_align_interval: 1h
  ```

* By default `vmagent` evenly spreads scrape load in time. If a particular scrape target must be scraped at specific offset, then `scrape_offset` option must be used.
  For example, the following config instructs `vmagent` to scrape the target at 10 seconds of every minute:

  ```yaml
  scrape_configs:
  - job_name: foo
    scrape_interval: 1m
    scrape_offset: 10s
  ```

* If you see `skipping duplicate scrape target with identical labels` errors when scraping Kubernetes pods, then it is likely these pods listen to multiple ports
  or they use an init container. These errors can either be fixed or suppressed with the `-promscrape.suppressDuplicateScrapeTargetErrors` command-line flag.
  See the available options below if you prefer fixing the root cause of the error:

  The following relabeling rule may be added to `relabel_configs` section in order to filter out pods with unneeded ports:

  ```yaml
  - action: keep_if_equal
    source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_container_port_number]
  ```

  The following relabeling rule may be added to `relabel_configs` section in order to filter out init container pods:

  ```yaml
  - action: drop
    source_labels: [__meta_kubernetes_pod_container_init]
    regex: true
  ```

See also:

- [Relabel Debugging](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabel-debugging)
- [General Troubleshooting](https://docs.victoriametrics.com/victoriametrics/troubleshooting/)

## Calculating disk space for persistence queue

`vmagent` buffers collected metrics on disk at the directory specified via `-remoteWrite.tmpDataPath` command-line flag
until the metrics are sent to remote storage configured via `-remoteWrite.url` command-line flag.
The `-remoteWrite.tmpDataPath` directory can grow large when remote storage is unavailable for extended
periods of time and if the maximum directory size isn't limited with `-remoteWrite.maxDiskUsagePerURL` command-line flag.

To estimate the allocated disk size for persistent queue, or to estimate `-remoteWrite.maxDiskUsagePerURL` command-line flag value,
take into account the following attributes:

1. The **size in bytes** of data stream sent by vmagent:

   Run the query `sum(rate(vmagent_remotewrite_bytes_sent_total[1h])) by(instance,url)` in [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui)
   or Grafana to get the amount of bytes sent by each vmagent instance per second.

1. The amount of **time** a persistent queue should keep the data before starting to drop it.

   For example, if `vmagent` should be able to buffer the data for at least 6 hours, then the following query
   can be used for estimating the needed amounts of disk space in gigabytes:

   ```metricsql
   sum(rate(vmagent_remotewrite_bytes_sent_total[1h])) by(instance,url) * 6h / 1Gi
   ```

Additional notes:

1. Ensure that `vmagent` [monitoring](#monitoring) is configured properly.
1. Re-evaluate the estimation each time when:
    * there is an increase in the vmagent's workload
    * there is a change in [relabeling rules](https://docs.victoriametrics.com/victoriametrics/relabeling/) which could increase the amount metrics to send
    * there is a change in number of configured `-remoteWrite.url` addresses
1. The minimum disk size to allocate for the persistent queue is 500Mi per each `-remoteWrite.url`.
1. On-disk persistent queue can be disabled if needed. See [these docs](https://docs.victoriametrics.com/victoriametrics/vmagent/#disabling-on-disk-persistence).

## Security

See general recommendations regarding security [here](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#security).

### mTLS protection

By default `vmagent` accepts http requests at `8429` port (this port can be changed via `-httpListenAddr` command-line flags),
since it is expected it runs in an isolated trusted network.
[Enterprise version of vmagent](https://docs.victoriametrics.com/victoriametrics/enterprise/) supports the ability to accept [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication)
requests at this port, by specifying `-tls` and `-mtls` command-line flags. For example, the following command runs `vmagent`, which accepts only mTLS requests at port `8429`:

```
./vmagent -tls -mtls -remoteWrite.url=...
```

By default, system-wide [TLS Root CA](https://en.wikipedia.org/wiki/Root_certificate) is used for verifying client certificates if `-mtls` command-line flag is specified.
It is possible to specify custom TLS Root CA via `-mtlsCAFile` command-line flag.

## Performance optimizations

`vmagent` is optimized for low CPU usage and low RAM usage without the need to tune any configs. Sometimes it is needed to optimize CPU / RAM usage of `vmagent` even more.
For example, if `vmagent` needs to scrape thousands of targets in resource-constrained environments. Then the following options may help reducing CPU usage and RAM usage of `vmagent`:

- Set [GOGC](https://pkg.go.dev/runtime#hdr-Environment_Variables) environment variable to `100`. This reduces CPU usage at the cost of higher RAM usage.

- Set [GOMAXPROCS](https://pkg.go.dev/runtime#hdr-Environment_Variables) environment variable to the value slightly bigger than the number of CPU cores used by `vmagent`.
  Another option is to set CPU limit in Kubernetes / Docker to the integer value bigger than the number of CPU cores used by `vmagent`.
  This reduces RAM usage and CPU usage when `vmagent` runs in an environment with bigger number of available CPU cores. Note that it may be needed to increase the `-remoteWrite.queues`
  command-line flag to bigger values if `GOMAXPROCS` is set to too small values, since by default `-remoteWrite.queues` is proportional to `GOMAXPROCS`.

- Disable response compression at scrape targets via `-promscrape.disableCompression` command-line flag or via `disable_compression: true` option
  in the [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs). This reduces CPU usage at the cost of higher network bandwidth usage
  between `vmagent` and scrape targets.

- Disable tracking of original labels for the discovered targets via `-promscrape.dropOriginalLabels` command-line flag. This helps reducing RAM usage when `vmagent`
  discovers big number of scrape targets and the majority of these targets are [dropped](https://docs.victoriametrics.com/victoriametrics/relabeling/#how-to-drop-discovered-targets).
  This is a typical case when `vmagent` discovers Kubernetes targets. The downside of using `-promscrape.dropOriginalLabels` command-line flag
  is the reduced [debuggability](#debugging-scrape-targets) for improperly configured per-target relabeling.

- Disable [staleness markers](https://docs.victoriametrics.com/victoriametrics/vmagent/#prometheus-staleness-markers) via `-promscrape.noStaleMarkers` command-line flag
  or via `no_stale_markers: true` option in the [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs). This reduces RAM usage and CPU usage.
  Note that disabling staleness markers may result in unexpected query results when scrape targets are frequently rotated (this is a typical case in Kubernetes).

- Set `-memory.allowedBytes` command-line flag to the value close to the actual memory usage of `vmagent`. Another option is to set memory limit in Kubernetes / Docker
  to the value 50% bigger than the actual memory usage of `vmagent`. This should reduce memory usage spikes for `vmagent` running in the environment with bigger available memory
  when the remote storage cannot keep up with the data ingestion rate. Increasing `-remoteWrite.queues` command-line flag value may help in this case too.

- In extreme cases it may be useful to set `-promscrape.disableKeepAlive` command-line flag in order to save RAM on HTTP keep-alive connections to thousands of scrape targets.

- Increase `scrape_interval` option in the `global` section of the `-promscrape.config` and/or at every [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs)
  for reducing CPU usage. For example, increasing the `scrape_interval` from `10s` to `30s` across all the targets decreases CPU usage at `vmagent` by up to 3x.

Example command, which runs `vmagent` in an optimized mode:

```
GOGC=100 GOMAXPROCS=1 ./vmagent -promscrape.disableCompression -promscrape.dropOriginalLabels -promscrape.noStaleMarkers -memory.allowedBytes=1GiB -promscrape.disableKeepAlive ...
```

## How to build from sources

We recommend using [official binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - `vmagent` is located in the `vmutils-...` archives.

It may be needed to build `vmagent` from source code when developing or testing new feature or bugfix.

### Development build

1. [Install Go](https://golang.org/doc/install).
1. Run `make vmagent` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds the `vmagent` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmagent-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmagent-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmagent`. It builds `victoriametrics/vmagent:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is an auto-generated image tag, which depends on source code in [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmagent`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```sh
ROOT_IMAGE=scratch make package-vmagent
```

### ARM build

ARM build may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

### Development ARM build

1. [Install Go](https://golang.org/doc/install).
1. Run `make vmagent-linux-arm` or `make vmagent-linux-arm64` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics)
   It builds `vmagent-linux-arm` or `vmagent-linux-arm64` binary respectively and puts it into the `bin` folder.

### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmagent-linux-arm-prod` or `make vmagent-linux-arm64-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmagent-linux-arm-prod` or `vmagent-linux-arm64-prod` binary respectively and puts it into the `bin` folder.

## Profiling

`vmagent` provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile can be collected with the following command (replace `0.0.0.0` with hostname if needed):


```sh
curl http://0.0.0.0:8429/debug/pprof/heap > mem.pprof
```


* CPU profile can be collected with the following command (replace `0.0.0.0` with hostname if needed):


```sh
curl http://0.0.0.0:8429/debug/pprof/profile > cpu.pprof
```


The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).

It is safe sharing the collected profiles from security point of view, since they do not contain sensitive information.

## Advanced usage

`vmagent` can be fine-tuned with various command-line flags. Run `./vmagent -help` in order to see the full list of these flags with their descriptions and default values:

{{% content "vmagent_flags.md" %}}

---

Section below contains backward-compatible anchors for links that were moved or renamed.

###### Relabeling

Moved to [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/).

###### Relabeling enhancements

Moved to [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabeling-enhancements).

###### Graphite relabeling

Moved to [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/#graphite-relabeling).

###### Relabel debug

Moved to [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabel-debugging).

###### Debugging scrape targets

Moved to [Relabeling Cookbook](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabel-debugging).

###### Google PubSub integration

Moved to [integrations/pubsub](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/).

###### Reading metrics from PubSub

Moved to [integrations/pubsub/#reading-metrics](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics).

###### Consume metrics from multiple topics

Moved to [integrations/pubsub/#multiple-topics](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#multiple-topics).

###### Command-line flags for PubSub consumer

Moved to [integrations/pubsub/#consumer-command-line-flags](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#consumer-command-line-flags).

###### Writing metrics to PubSub

Moved to [integrations/pubsub/#writing-metrics](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics).

###### Command-line flags for PubSub producer

Moved to [integrations/pubsub/#producer-command-line-flags](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#producer-command-line-flags).

###### Kafka integration

Moved to [integrations/kafka](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/).

###### Reading metrics from Kafka

Moved to [integrations/kafka/#reading-metrics](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics).

###### Command-line flags for Kafka consumer

Moved to [integrations/kafka/#consumer-command-line-flags](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#consumer-command-line-flags).

###### Writing metrics to Kafka

Moved to [integrations/kafka/#writing-metrics](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#writing-metrics).

###### Estimating message size and rate

Moved to [integrations/kafka/#estimating-message-size-and-rate](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#estimating-message-size-and-rate).

###### Kafka broker authorization and authentication

Moved to [integrations/kafka/#kafka-broker-authorization-and-authentication](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#kafka-broker-authorization-and-authentication).

