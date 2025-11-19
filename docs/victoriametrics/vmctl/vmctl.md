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
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.130.0/vmutils-darwin-arm64-v1.130.0.tar.gz

tar xzf vmutils-darwin-arm64-v1.130.0.tar.gz
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

Commands:
```shellhelp
  influx
     Migrate time series from InfluxDB
  opentsdb
     Migrate time series from OpenTSDB.
  prometheus
     Migrate time series from Prometheus.
  remote-read
     Migrate time series via Prometheus remote-read protocol.
  verify-block
     Verifies exported block with VictoriaMetrics Native format.
  vm-native
     Migrate time series between VictoriaMetrics installations.
```

Flags available for all commands:

```shellhelp
  -s
     Whether to run in silent mode. If set to true no confirmation prompts will appear. (default false)
  -verbose
     Whether to enable verbosity in logs output. (default false)
  -disable-progress-bar
     Whether to disable progress bar during the import. (default false)
     
   --vm-addr vmctl
     VictoriaMetrics address to perform import requests. 
     Should be the same as --httpListenAddr value for single-node version or vminsert component. 
     When importing into the clustered version do not forget to set additionally --vm-account-id flag. 
     Please note, that vmctl performs initial readiness check for the given address by checking `/health` endpoint. (default: "http://localhost:8428")
   --vm-user value
     VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value
     VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value
     AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). 
     AccountID is required when importing into the clustered version of VictoriaMetrics. 
     It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. 
     If projectID isn't set, then it equals to 0
   --vm-concurrency value
     Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress
     Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value
     How many samples importer collects before sending the import request to VM (default: 200000)
   --vm-significant-figures value
     The number of significant figures to leave in metric values before importing. See https://en.wikipedia.org/wiki/Significant_figures.
     Zero value saves all the significant figures. This option may be used for increasing on-disk compression level for the stored metrics.
     See also --vm-round-digits option (default: 0)
   --vm-round-digits value
     Round metric values to the given number of decimal digits after the point. This option may be used for increasing 
     on-disk compression level for the stored metrics (default: 100)
   --vm-extra-label value [ --vm-extra-label value ]
     Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flagwill have priority.
     Flag can be set multiple times, to add few additional labels.
   --vm-rate-limit value
     Optional data transfer rate limit in bytes per second.
     By default, the rate limit is disabled. It can be useful for limiting load on configured via '--vmAddr' destination. (default: 0)
   --vm-cert-file value
     Optional path to client-side TLS certificate file to use when connecting to '--vmAddr'
   --vm-key-file value
     Optional path to client-side TLS key to use when connecting to '--vmAddr'
   --vm-CA-file value
     Optional path to TLS CA file to use for verifying connections to '--vmAddr'. By default, system CA is used
   --vm-server-name value
     Optional TLS server name to use for connections to '--vmAddr'. By default, the server name from '--vmAddr' is used
   --vm-insecure-skip-verify
     Whether to skip tls verification when connecting to '--vmAddr' (default: false)
   --vm-backoff-retries value
     How many import retries to perform before giving up. (default: 10)
   --vm-backoff-factor value
     Factor to multiply the base duration after each failed import retry. Must be greater than 1.0 (default: 1.8)
   --vm-backoff-min-duration value
     Minimum duration to wait before the first import retry. Each subsequent import retry will be multiplied by the '--vm-backoff-factor'. (default: 2s)
```

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
