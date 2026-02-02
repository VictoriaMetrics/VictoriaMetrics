---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
VictoriaMetrics command-line tool (**vmctl**) provides the following migration modes:
- [Prometheus](https://docs.victoriametrics.com/victoriametrics/vmctl/prometheus/) to VictoriaMetrics via [snapshot](https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot)
- [InfluxDB](https://docs.victoriametrics.com/victoriametrics/vmctl/influxdb/) to VictoriaMetrics
- [OpenTSDB](https://docs.victoriametrics.com/victoriametrics/vmctl/opentsdb/) to VictoriaMetrics
- [Thanos](https://docs.victoriametrics.com/victoriametrics/vmctl/thanos/) to VictoriaMetrics via [snapshot](https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot)
- migrate data between [VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/) single or cluster version.
- migrate data via [Prometheus remote read protocol](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) to VictoriaMetrics:
    - [Thanos](https://docs.victoriametrics.com/victoriametrics/vmctl/thanos/#remote-read-protocol)
    - [Cortex](https://docs.victoriametrics.com/victoriametrics/vmctl/cortex/)
    - [Mimir](https://docs.victoriametrics.com/victoriametrics/vmctl/mimir/)
    - [Promscale](https://docs.victoriametrics.com/victoriametrics/vmctl/promscale/)

Additionally, vmctl supports [verify](#verifying-exported-blocks-from-victoriametrics) mode for exported blocks from
VictoriaMetrics single or cluster version.

## Articles 

- [How to migrate data from Prometheus](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-d44a6728f043)
- [How to migrate data from Prometheus. Filtering and modifying time series](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-filtering-and-modifying-time-series-6d40cea4bf21)

## Quick Start

vmctl command-line tool is available as:
* docker images at [Docker Hub](https://hub.docker.com/r/victoriametrics/vmctl/) and [Quay](https://quay.io/repository/victoriametrics/vmctl?tab=tags)
* [Binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) as part of `vmutils` package

Download and unpack vmctl:
```sh
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.134.0/vmutils-darwin-arm64-v1.134.0.tar.gz

tar xzf vmutils-darwin-arm64-v1.134.0.tar.gz
```

Once binary is unpacked, see the full list of supported modes by running the following command:
```sh
$ ./vmctl-prod --help
NAME:
   vmctl - VictoriaMetrics command-line tool

USAGE:
   vmctl [global options] command [command options] [arguments...]

COMMANDS:
   opentsdb      Migrate time series from OpenTSDB
   influx        Migrate time series from InfluxDB
   remote-read   Migrate time series via Prometheus remote-read protocol
   prometheus    Migrate time series from Prometheus
   vm-native     Migrate time series between VictoriaMetrics installations
   verify-block  Verifies exported block with VictoriaMetrics Native format
```

vmctl acts as a proxy between **source** (where to fetch data from) and **destination** (where to migrate data to).

As a **source**, user needs to specify one of the migration modes (`prometheus`, `influx`, etc.). Each command has its
own unique set of flags (e.g. prefixed with `influx-` for influx) and a common list of flags for destination (prefixed with `vm-` for VictoriaMetrics):

```sh
$ ./vmctl-prod influx --help
OPTIONS:
   --influx-addr value              InfluxDB server addr (default: "http://localhost:8086")
   --influx-user value              InfluxDB user [$INFLUX_USERNAME]
...
   --vm-addr value        VictoriaMetrics address to perform import requests.
   --vm-user value        VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value    VictoriaMetrics password for basic auth [$VM_PASSWORD]
```

See detailed documentation about each migration mode in the main menu under `vmctl` section.

See how to [add extra labels](#adding-extra-labels) or [improve compression](#significant-figures).

## Configuring VictoriaMetrics

For every migration mode, user needs to specify **destination** - Victoriametrics `--vm-addr` address.
For example:
```sh
./vmctl prometheus \
  --vm-addr=<victoriametrics-addr>:8428 \
  --prom-snapshot=/path/to/snapshot
```
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For cluster version you need to additionally specify the `--vm-account-id` flag and use vminsert address:
```
http://<vminsert-addr>:8480
```
_Replace `<vminsert-addr>` with the hostname or IP address of vminsert service._

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).

## Migration tips

Migration speed heavily depends on the following factors:
1. Network bandwidth. Since vmctl acts as a proxy, it receives data from **source** and forward it to **destination**
   via the network.
2. **source** performance, or how quick source can return requested data
3. **destination** performance, or how quick destination can accept and store data.

> See the expected migration speed [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5366#issuecomment-1854251938).

Importing speed can be adjusted via `--vm-concurrency` cmd-line flag, which controls the number of concurrent
workers busy with processing. Please note, that each worker can load up to a single vCPU core on VictoriaMetrics.
So try to set it according to allocated CPU resources of your VictoriaMetrics destination installation.

Migration is a backfilling process, so it is recommended to read [Backfilling tips](https://github.com/VictoriaMetrics/VictoriaMetrics#backfilling) section.

`vmctl` doesn't provide relabeling or other types of labels management. Instead, use [relabeling in VictoriaMetrics](https://github.com/VictoriaMetrics/vmctl/issues/4#issuecomment-683424375).

### Importer stats

After successful import `vmctl` prints some statistics for details.
The important numbers to watch are following:

- `idle duration` - shows time that importer spent while waiting for data from InfluxDB/Prometheus
  to fill up `--vm-batch-size` batch size. Value shows total duration across all workers configured
  via `--vm-concurrency`. High value may be a sign of too slow InfluxDB/Prometheus fetches or too
  high `--vm-concurrency` value. Try to improve it by increasing `--<mode>-concurrency` value or
  decreasing `--vm-concurrency` value.
- `import requests` - shows how many import requests were issued to VM server.
  The import request is issued once the batch size(`--vm-batch-size`) is full and ready to be sent.
  Please prefer big batch sizes (50k-500k) to improve performance.
- `import requests retries` - shows number of unsuccessful import requests. Non-zero value may be
  a sign of network issues or VM being overloaded. See the logs during import for error messages.

### Silent mode

By default, `vmctl` waits confirmation from user before starting the import. If this is unwanted
behavior and no user interaction required - pass `-s` flag to enable "silence" mode.

Setting `-disable-progress-bar` cmd-line flag disables the progress bar during import.

### Significant figures

`vmctl` allows to limit the number of [significant figures](https://en.wikipedia.org/wiki/Significant_figures)
before importing to improve data compression:

- `--vm-round-digits` flag for rounding processed values to the given number of decimal digits after the point.
  For example, `--vm-round-digits=2` would round `1.2345` to `1.23`. By default, the rounding is disabled.

- `--vm-significant-figures` flag for limiting the number of significant figures in processed values. It takes no effect if set
  to 0 (by default), but set `--vm-significant-figures=5` and `102.342305` will be rounded to `102.34`.

The most common case for using these flags is to improve data compression for time series storing aggregation
results such as `average`, `rate`, etc.

See [How to migrate data from Prometheus. Filtering and modifying time series](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-filtering-and-modifying-time-series-6d40cea4bf21)
for examples of compression improvement.

### Adding extra labels

`vmctl` allows to add extra labels to all imported series. It can be achieved with flag `--vm-extra-label label=value`.
If multiple labels needs to be added, set flag for each label, for example, `--vm-extra-label label1=value1 --vm-extra-label label2=value2`.
If timeseries already have label, that must be added with `--vm-extra-label` flag, flag has priority and will override label value from timeseries.

### Rate limiting

Limiting the rate of data transfer could help to reduce pressure on disk or on destination database.
The rate limit may be set in bytes-per-second via `--vm-rate-limit` flag. Note that the rate limit is applied per worker,
see `--vm-concurrency` flag.

Please note, you can also use [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/)
as a proxy between `vmctl` and destination with `-remoteWrite.rateLimit` flag enabled.

### Monitoring the migration process

`vmctl` can push internal metrics {{% available_from "v1.135.0" %}} to a remote storage for monitoring migration progress and performance.
This is especially useful for long-running migrations where you want to track progress, detect issues,
or build dashboards to visualize the migration status.

Example usage with VictoriaMetrics as the metrics destination:

```sh
./vmctl influx \
  --influx-addr=http://localhost:8086 \
  --influx-database=mydb \
  --vm-addr=http://localhost:8428 \
  --pushmetrics.url=http://localhost:8428/api/v1/import/prometheus \
  --pushmetrics.extraLabel='job="vmctl"' \
  --pushmetrics.extraLabel='instance="migration-1"'
```

#### Available metrics

The following metrics are exposed by `vmctl`:

General metrics (available for all migration modes):

| Metric | Description |
|--------|-------------|
| `vmctl_backoff_retries_total` | Total number of retry attempts across all operations |
| `vmctl_limiter_bytes_processed_total` | Total bytes processed through rate limiter (when `--vm-rate-limit` is set) |
| `vmctl_limiter_throttle_events_total` | Number of times rate limiting caused a pause |

Mode-specific metrics:

Each migration mode exposes its own set of metrics with the mode name embedded in the metric name:

| Mode | Metrics |
|------|---------|
| `influx` | `vmctl_influx_migration_series_total`, `vmctl_influx_migration_series_processed`, `vmctl_influx_migration_errors_total` |
| `prometheus` | `vmctl_prometheus_migration_blocks_total`, `vmctl_prometheus_migration_blocks_processed`, `vmctl_prometheus_migration_errors_total` |
| `opentsdb` | `vmctl_opentsdb_migration_series_total`, `vmctl_opentsdb_migration_series_processed`, `vmctl_opentsdb_migration_errors_total` |
| `remote-read` | `vmctl_remote_read_migration_ranges_total`, `vmctl_remote_read_migration_ranges_processed`, `vmctl_remote_read_migration_errors_total` |
| `vm-native` | `vmctl_vm_native_migration_metrics_total`, `vmctl_vm_native_migration_metrics_processed`, `vmctl_vm_native_migration_requests_planned`, `vmctl_vm_native_migration_requests_completed`, `vmctl_vm_native_migration_tenants_total`, `vmctl_vm_native_migration_tenants_processed`, `vmctl_vm_native_migration_bytes_transferred_total`, `vmctl_vm_native_migration_errors_total` |

#### Example PromQL queries

Monitor migration progress:
```promql
# Migration completion percentage for influx mode
vmctl_influx_migration_series_processed / vmctl_influx_migration_series_total * 100

# Migration completion percentage for vm-native mode
vmctl_vm_native_migration_metrics_processed / vmctl_vm_native_migration_metrics_total * 100

# Retry rate
rate(vmctl_backoff_retries_total[5m])

# Rate limiter throttling events per second
rate(vmctl_limiter_throttle_events_total[5m])

# Data transfer speed in bytes per second (when rate limiting is enabled)
rate(vmctl_limiter_bytes_processed_total[5m])

# Data transfer speed in MB per second for vm-native mode
rate(vmctl_vm_native_migration_bytes_transferred_total[5m]) / 1Mb
```

## Verifying exported blocks from VictoriaMetrics

In this mode, `vmctl` allows verifying correctness and integrity of data exported via
[native format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format)
from VictoriaMetrics.
You can verify exported data at disk before uploading it by `vmctl verify-block` command:

```sh
# export blocks from VictoriaMetrics
curl localhost:8428/api/v1/export/native -g -d 'match[]={__name__!=""}' -o exported_data_block
# verify block content
./vmctl verify-block exported_data_block
2022/03/30 18:04:50 verifying block at path="exported_data_block"
2022/03/30 18:04:50 successfully verified block at path="exported_data_block", blockCount=123786
2022/03/30 18:04:50 Total time: 100.108ms
```

## How to build

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - `vmctl` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install).
1. Run `make vmctl` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmctl-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmctl`. It builds `victoriametrics/vmctl:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmctl`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```sh
ROOT_IMAGE=scratch make package-vmctl
```

### ARM build

ARM build may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

#### Development ARM build

1. [Install Go](https://golang.org/doc/install).
1. Run `make vmctl-linux-arm` or `make vmctl-linux-arm64` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-linux-arm` or `vmctl-linux-arm64` binary respectively and puts it into the `bin` folder.

#### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmctl-linux-arm-prod` or `make vmctl-linux-arm64-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-linux-arm-prod` or `vmctl-linux-arm64-prod` binary respectively and puts it into the `bin` folder.

## Command-line flags

Run `vmctl -help` in order to see all the available options.

{{% content "vmctl_flags.md" %}}

See list of available cmd-line flags for each command in the corresponding section.

---

Section below contains backward-compatible anchors for links that were moved or renamed.

###### Migrating data from Prometheus

Moved to [vmctl/prometheus](https://docs.victoriametrics.com/victoriametrics/vmctl/prometheus/).

###### Migrating data from InfluxDB (1.x)

Moved to [vmctl/influx](https://docs.victoriametrics.com/victoriametrics/vmctl/influxdb/).

###### Migrating data from InfluxDB (2.x)

Moved to [vmctl/influx](https://docs.victoriametrics.com/victoriametrics/vmctl/influxdb/#influxdb-v2).

###### Migrating data from OpenTSDB

Moved to [vmctl/opentsdb](https://docs.victoriametrics.com/victoriametrics/vmctl/opentsdb/).

###### Migrating data from Thanos

Moved to [vmctl/thanos](https://docs.victoriametrics.com/victoriametrics/vmctl/thanos/).

###### Migrating data by remote read protocol

Moved to [vmctl/remoteread](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/).

###### Migrating data from Cortex

Moved to [vmctl/cortex](https://docs.victoriametrics.com/victoriametrics/vmctl/cortex/).

###### Migrating data from Promscale

Moved to [vmctl/promscale](https://docs.victoriametrics.com/victoriametrics/vmctl/promscale/).

###### Migrating data from Mimir

Moved to [vmctl/mimir](https://docs.victoriametrics.com/victoriametrics/vmctl/mimir/).

###### Migrating data from VictoriaMetrics

Moved to [vmctl/victoriametrics](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/).

###### Tuning

Moved to [vmctl#migration-tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).
