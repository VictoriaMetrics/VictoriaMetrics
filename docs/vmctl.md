---
sort: 8
weight: 8
menu:
  docs:
    parent: 'victoriametrics'
    weight: 8
title: vmctl
aliases:
  - /vmctl.html
---
# vmctl

VictoriaMetrics command-line tool (vmctl) provides the following list of actions:
- migrate data from [Prometheus](#migrating-data-from-prometheus) to VictoriaMetrics using snapshot API
- migrate data from [Thanos](#migrating-data-from-thanos) to VictoriaMetrics
- migrate data from [Cortex](#migrating-data-from-cortex) to VictoriaMetrics
- migrate data from [Mimir](#migrating-data-from-mimir) to VictoriaMetrics
- migrate data from [InfluxDB](#migrating-data-from-influxdb-1x) to VictoriaMetrics
- migrate data from [OpenTSDB](#migrating-data-from-opentsdb) to VictoriaMetrics
- migrate data from [Promscale](#migrating-data-from-promscale)
- migrate data between [VictoriaMetrics](#migrating-data-from-victoriametrics) single or cluster version.
- migrate data by [Prometheus remote read protocol](#migrating-data-by-remote-read-protocol) to VictoriaMetrics
- [verify](#verifying-exported-blocks-from-victoriametrics) exported blocks from VictoriaMetrics single or cluster version.

To see the full list of supported actions run the following command:

```sh
$ ./vmctl --help
NAME:
   vmctl - VictoriaMetrics command-line tool

USAGE:
   vmctl [global options] command [command options] [arguments...]

COMMANDS:
   opentsdb    Migrate timeseries from OpenTSDB
   influx      Migrate timeseries from InfluxDB
   prometheus  Migrate timeseries from Prometheus
   vm-native   Migrate time series between VictoriaMetrics installations via native binary format
   remote-read Migrate timeseries by Prometheus remote read protocol
   verify-block  Verifies correctness of data blocks exported via VictoriaMetrics Native format. See https://docs.victoriametrics.com/#how-to-export-data-in-native-format
```

Each command has its own unique set of flags specific (e.g. prefixed with `influx-` for [influx](https://docs.victoriametrics.com/vmctl/#migrating-data-from-influxdb-1x))
to the data source and common list of flags for destination (prefixed with `vm-` for VictoriaMetrics):

```sh
$ ./vmctl influx --help
OPTIONS:
   --influx-addr value              InfluxDB server addr (default: "http://localhost:8086")
   --influx-user value              InfluxDB user [$INFLUX_USERNAME]
...
   --vm-addr vmctl                             VictoriaMetrics address to perform import requests.
Should be the same as --httpListenAddr value for single-node version or vminsert component.
When importing into the clustered version do not forget to set additionally --vm-account-id flag.
Please note, that vmctl performs initial readiness check for the given address by checking `/health` endpoint. (default: "http://localhost:8428")
   --vm-user value        VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value    VictoriaMetrics password for basic auth [$VM_PASSWORD]
```

When doing a migration user needs to specify flags for **source** (where and how to fetch data) and for
**destination** (where to migrate data). Every command has additional details and nuances, please see
them below in corresponding sections.

For the **destination** flags see the full description by running the following command:
```
$ ./vmctl influx --help | grep vm-
```

Some flags like [--vm-extra-label](#adding-extra-labels) or [--vm-significant-figures](#significant-figures)
has additional sections with description below. Details about tweaking and adjusting settings
are explained in [Tuning](#tuning) section.

Please note, that if you're going to import data into VictoriaMetrics cluster do not
forget to specify the `--vm-account-id` flag. See more details for cluster version
[here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

## Articles

- [How to migrate data from Prometheus](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-d44a6728f043)
- [How to migrate data from Prometheus. Filtering and modifying time series](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-filtering-and-modifying-time-series-6d40cea4bf21)

## Migrating data from OpenTSDB

`vmctl` supports the `opentsdb` mode to migrate data from OpenTSDB to VictoriaMetrics time-series database.

See `./vmctl opentsdb --help` for details and full list of flags.

**Important:** OpenTSDB migration is not possible without a functioning [meta](http://opentsdb.net/docs/build/html/user_guide/metadata.html) table to search for metrics/series. Check in OpenTSDB config that appropriate options are [activated]( https://github.com/OpenTSDB/opentsdb/issues/681#issuecomment-177359563) and HBase meta tables are present. W/o them migration won't work.

OpenTSDB migration works like so:

1. Find metrics based on selected filters (or the default filter set `['a','b','c','d','e','f','g','h','i','j','k','l','m','n','o','p','q','r','s','t','u','v','w','x','y','z']`):

   `curl -Ss "http://opentsdb:4242/api/suggest?type=metrics&q=sys"`

1. Find series associated with each returned metric:

   `curl -Ss "http://opentsdb:4242/api/search/lookup?m=system.load5&limit=1000000"`

   Here `results` return field should not be empty. Otherwise, it means that meta tables are absent and needs to be turned on previously.

1. Download data for each series in chunks defined in the CLI switches:

   `-retention=sum-1m-avg:1h:90d` means:
   - `curl -Ss "http://opentsdb:4242/api/query?start=1h-ago&end=now&m=sum:1m-avg-none:system.load5\{host=host1\}"`
   - `curl -Ss "http://opentsdb:4242/api/query?start=2h-ago&end=1h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`
   - `curl -Ss "http://opentsdb:4242/api/query?start=3h-ago&end=2h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`
   - ...
   - `curl -Ss "http://opentsdb:4242/api/query?start=2160h-ago&end=2159h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`

This means that we must stream data from OpenTSDB to VictoriaMetrics in chunks. This is where concurrency for OpenTSDB comes in. We can query multiple chunks at once, but we shouldn't perform too many chunks at a time to avoid overloading the OpenTSDB cluster.

```sh
$ ./vmctl opentsdb --otsdb-addr http://opentsdb:4242/ --otsdb-retentions sum-1m-avg:1h:1d --otsdb-filters system --otsdb-normalize --vm-addr http://victoria:8428/
OpenTSDB import mode
2021/04/09 11:52:50 Will collect data starting at TS 1617990770
2021/04/09 11:52:50 Loading all metrics from OpenTSDB for filters:  [system]
Found 9 metrics to import. Continue? [Y/n]
2021/04/09 11:52:51 Starting work on system.load1
23 / 402200 [>____________________________________________________________________________________________] 0.01% 2 p/s
```
Where `:8428` is Prometheus port of VictoriaMetrics.

For clustered VictoriaMetrics setup `--vm-account-id` flag needs to be added, for example:

```
$ ./vmctl opentsdb --otsdb-addr http://opentsdb:4242/ --otsdb-retentions sum-1m-avg:1h:1d --otsdb-filters system --otsdb-normalize --vm-addr http://victoria:8480/ --vm-account-id 0
```
This time `:8480` port is vminsert/Prometheus input port.

### Retention strings

Starting with a relatively simple retention string (`sum-1m-avg:1h:30d`), let's describe how this is converted into actual queries.

There are two essential parts of a retention string:

1. [aggregation](#aggregation)
1. [windows/time ranges](#windows)

#### Aggregation

Retention strings essentially define the two levels of aggregation for our collected series.

`sum-1m-avg` would become:

- First order: `sum`
- Second order: `1m-avg-none`

##### First Order Aggregations

First-order aggregation addresses how to aggregate any un-mentioned tags.

This is, conceptually, directly opposite to how PromQL deals with tags. In OpenTSDB, if a tag isn't explicitly mentioned, all values associated with that tag will be aggregated.

It is recommended to use `sum` for the first aggregation because it is relatively quick and should not cause any changes to the incoming data (because we collect each individual series).

##### Second Order Aggregations

Second-order aggregation (`1m-avg` in our example) defines any windowing that should occur before returning the data

It is recommended to match the stat collection interval, so we again avoid transforming incoming data.

We do not allow for defining the "null value" portion of the rollup window (e.g. in the aggregation, `1m-avg-none`, the user cannot change `none`), as the goal of this tool is to avoid modifying incoming data.

#### Windows

There are two important windows we define in a retention string:

1. the "chunk" range of each query
1. The time range we will be querying on with that "chunk"

From our example, our windows are `1h:30d`.

##### Window "chunks"

The window `1h` means that each individual query to OpenTSDB should only span 1 hour of time (e.g. `start=2h-ago&end=1h-ago`).

It is important to ensure this window somewhat matches the row size in HBase to help improve query times.

For example, if the query is hitting a rollup table with a 4-hour row size, we should set a chunk size of a multiple of 4 hours (e.g. `4h`, `8h`, etc.) to avoid requesting data across row boundaries. Landing on row boundaries allows for more consistent request times to HBase.

The default table created in HBase for OpenTSDB has a 1-hour row size, so if you aren't sure on a correct row size to use, `1h` is a reasonable choice.

##### Time range

The time range `30d` simply means we are asking for the last 30 days of data. This time range can be written using `h`, `d`, `w`, or `y`. (We can't use `m` for month because it already means `minute` in time parsing).

#### Results of retention string

The resultant queries that will be created, based on our example retention string of `sum-1m-avg:1h:30d` look like this:

```sh
http://opentsdb:4242/api/query?start=1h-ago&end=now&m=sum:1m-avg-none:<series>
http://opentsdb:4242/api/query?start=2h-ago&end=1h-ago&m=sum:1m-avg-none:<series>
http://opentsdb:4242/api/query?start=3h-ago&end=2h-ago&m=sum:1m-avg-none:<series>
...
http://opentsdb:4242/api/query?start=721h-ago&end=720h-ago&m=sum:1m-avg-none:<series>
```

Chunking the data like this means each individual query returns faster, so we can start populating data into VictoriaMetrics quicker.

### Configuration

Run the following command to get all configuration options:

```sh
./vmctl opentsdb --help
```

### Restarting OpenTSDB migrations

One important note for OpenTSDB migration: Queries/HBase scans can "get stuck" within OpenTSDB itself. This can cause instability and performance issues within an OpenTSDB cluster, so stopping the migrator to deal with it may be necessary. Because of this, we provide the timestamp we started collecting data from at the beginning of the run. You can stop and restart the importer using this "hard timestamp" to ensure you collect data from the same time range over multiple runs.

## Migrating data from InfluxDB (1.x)

`vmctl` supports the `influx` mode for [migrating data from InfluxDB to VictoriaMetrics](https://docs.victoriametrics.com/guides/migrate-from-influx.html)
time-series database.

See `./vmctl influx --help` for details and full list of flags.

To use migration tool please specify the InfluxDB address `--influx-addr`, the database `--influx-database` and VictoriaMetrics address `--vm-addr`.
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of vminsert component. Please note, that vmctl performs initial readiness check for the given address
by checking `/health` endpoint. For cluster version it is additionally required to specify the `--vm-account-id` flag.
See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the InfluxDB scheme exploration.
Basically, it just fetches all fields and timeseries from the provided database and builds up registry of all available timeseries.
Then `vmctl` sends fetch requests for each timeseries to InfluxDB one by one and pass results to VM importer.
VM importer then accumulates received samples in batches and sends import requests to VM.

The importing process example for local installation of InfluxDB(`http://localhost:8086`)
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl influx --influx-database benchmark
InfluxDB import mode
2020/01/18 20:47:11 Exploring scheme for database "benchmark"
2020/01/18 20:47:11 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/18 20:47:11 found 10 fields
2020/01/18 20:47:11 fetching series: command: "show series "; database: "benchmark"; retention: "autogen"
Found 40000 timeseries to import. Continue? [Y/n] y
40000 / 40000 [----------------------------------------------------------------------------------------] 100.00% 21 p/s
2020/01/18 21:19:00 Import finished!
2020/01/18 21:19:00 VictoriaMetrics importer stats:
  idle duration: 13m51.461434876s;
  time spent while importing: 17m56.923899847s;
  total samples: 345600000;
  samples/s: 320914.04;
  total bytes: 5.9 GB;
  bytes/s: 5.4 MB;
  import requests: 40001;
2020/01/18 21:19:00 Total time: 31m48.467044016s
```

### Data mapping

Vmctl maps InfluxDB data the same way as VictoriaMetrics does by using the following rules:

- `influx-database` arg is mapped into `db` label value unless `db` tag exists in the InfluxDB line.
If you want to skip this mapping just enable flag `influx-skip-database-label`.
- Field names are mapped to time series names prefixed with {measurement}{separator} value,
where {separator} equals to _ by default.
It can be changed with `--influx-measurement-field-separator` command-line flag.
- Field values are mapped to time series values.
- Tags are mapped to Prometheus labels format as-is.

For example, the following InfluxDB line:

```
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following Prometheus format data points:

```
foo_field1{tag1="value1", tag2="value2"} 12
foo_field2{tag1="value1", tag2="value2"} 40
```

### Configuration

Run the following command to get all configuration options:
```sh
./vmctl influx --help
```


### Filtering

The filtering consists of two parts: timeseries and time.
The first step of application is to select all available timeseries
for given database and retention. User may specify additional filtering
condition via `--influx-filter-series` flag. For example:

```sh
./vmctl influx --influx-database benchmark \
  --influx-filter-series "on benchmark from cpu where hostname='host_1703'"
InfluxDB import mode
2020/01/26 14:23:29 Exploring scheme for database "benchmark"
2020/01/26 14:23:29 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/26 14:23:29 found 12 fields
2020/01/26 14:23:29 fetching series: command: "show series on benchmark from cpu where hostname='host_1703'"; database: "benchmark"; retention: "autogen"
Found 10 timeseries to import. Continue? [Y/n]
```

The timeseries select query would be following:
 `fetching series: command: "show series on benchmark from cpu where hostname='host_1703'"; database: "benchmark"; retention: "autogen"`

The second step of filtering is a time filter and it applies when fetching the datapoints from Influx.
Time filtering may be configured with two flags:

- --influx-filter-time-start
- --influx-filter-time-end
Here's an example of importing timeseries for one day only:
`./vmctl influx --influx-database benchmark --influx-filter-series "where hostname='host_1703'" --influx-filter-time-start "2020-01-01T10:07:00Z" --influx-filter-time-end "2020-01-01T15:07:00Z"`

Please see more about time filtering [here](https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#filter-meta-queries-by-time).

## Migrating data from InfluxDB (2.x)

Migrating data from InfluxDB v2.x is not supported yet ([#32](https://github.com/VictoriaMetrics/vmctl/issues/32)).
You may find useful a 3rd party solution for this - <https://github.com/jonppe/influx_to_victoriametrics>.

## Migrating data from Promscale

[Promscale](https://github.com/timescale/promscale) supports [Prometheus Remote Read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).
To migrate historical data from Promscale to VictoriaMetrics we recommend using `vmctl`
in [remote-read](https://docs.victoriametrics.com/vmctl/#migrating-data-by-remote-read-protocol) mode.

See the example of migration command below:
```sh
./vmctl remote-read --remote-read-src-addr=http://<promscale>:9201/read \
                    --remote-read-step-interval=day \
                    --remote-read-use-stream=false \ # promscale doesn't support streaming
                    --vm-addr=http://<victoriametrics>:8428 \
                    --remote-read-filter-time-start=2023-08-21T00:00:00Z \
                    --remote-read-disable-path-append=true # promscale has custom remote read API HTTP path
Selected time range "2023-08-21 00:00:00 +0000 UTC" - "2023-08-21 14:11:41.561979 +0000 UTC" will be split into 1 ranges according to "day" step. Continue? [Y/n] y
VM worker 0:↙ 82831 samples/s                                                                                                                                                                        
VM worker 1:↙ 54378 samples/s                                                                                                                                                                        
VM worker 2:↙ 121616 samples/s                                                                                                                                                                       
VM worker 3:↙ 59164 samples/s                                                                                                                                                                        
VM worker 4:↙ 59220 samples/s                                                                                                                                                                        
VM worker 5:↙ 102072 samples/s                                                                                                                                                                       
Processing ranges: 1 / 1 [████████████████████████████████████████████████████████████████████████████████████] 100.00%
2023/08/21 16:11:55 Import finished!
2023/08/21 16:11:55 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 14.047045459s;
  total samples: 262111;
  samples/s: 18659.51;
  total bytes: 5.3 MB;
  bytes/s: 376.4 kB;
  import requests: 6;
  import requests retries: 0;
2023/08/21 16:11:55 Total time: 14.063458792s
```

Here we specify the full path to Promscale's Remote Read API via `--remote-read-src-addr`, and disable auto-path 
appending via `--remote-read-disable-path-append` cmd-line flags. This is necessary, as Promscale has a different to 
Prometheus API path. Promscale doesn't support stream mode for Remote Read API,
so we disable it via `--remote-read-use-stream=false`. 

## Migrating data from Prometheus

`vmctl` supports the `prometheus` mode for migrating data from Prometheus to VictoriaMetrics time-series database.
Migration is based on reading Prometheus snapshot, which is basically a hard-link to Prometheus data files.

See `./vmctl prometheus --help` for details and full list of flags. Also see Prometheus related articles [here](#articles).

To use migration tool please specify the file path to Prometheus snapshot `--prom-snapshot` (see how to make a snapshot [here](https://www.robustperception.io/taking-snapshots-of-prometheus-data)) and VictoriaMetrics address `--vm-addr`.
Please note, that `vmctl` *do not make a snapshot from Prometheus*, it uses an already prepared snapshot. More about Prometheus snapshots may be found [here](https://www.robustperception.io/taking-snapshots-of-prometheus-data) and [here](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-d44a6728f043).
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of vminsert component. Please note, that vmctl performs initial readiness check for the given address
by checking `/health` endpoint. For cluster version it is additionally required to specify the `--vm-account-id` flag.
See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the Prometheus snapshot exploration.
Basically, it just fetches all available blocks in provided snapshot and read the metadata. It also does initial filtering by time
if flags `--prom-filter-time-start` or `--prom-filter-time-end` were set. The exploration procedure prints some stats from read blocks.
Please note that stats are not taking into account timeseries or samples filtering. This will be done during importing process.

The importing process takes the snapshot blocks revealed from Explore procedure and processes them one by one
accumulating timeseries and samples. Please note, that `vmctl` relies on responses from InfluxDB on this stage,
so ensure that Explore queries are executed without errors or limits. Please see this
[issue](https://github.com/VictoriaMetrics/vmctl/issues/30) for details.
The data processed in chunks and then sent to VM.

The importing process example for local installation of Prometheus
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --vm-concurrency=1 \
  --vm-batch-size=200000 \
  --prom-concurrency=3
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 14;
  blocks skipped: 0;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1582409128139 (2020-02-22T22:05:28Z);
  samples: 32549106;
  series: 27289.
Found 14 blocks to import. Continue? [Y/n] y
14 / 14 [-------------------------------------------------------------------------------------------] 100.00% 0 p/s
2020/02/23 15:50:03 Import finished!
2020/02/23 15:50:03 VictoriaMetrics importer stats:
  idle duration: 6.152953029s;
  time spent while importing: 44.908522491s;
  total samples: 32549106;
  samples/s: 724786.84;
  total bytes: 669.1 MB;
  bytes/s: 14.9 MB;
  import requests: 323;
  import requests retries: 0;
2020/02/23 15:50:03 Total time: 51.077451066s
```

### Data mapping

VictoriaMetrics has very similar data model to Prometheus and supports [RemoteWrite integration](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage).
So no data changes will be applied.

### Configuration

Run the following command to get all configuration options:
```sh
./vmctl prometheus --help
```


### Filtering

The filtering consists of three parts: by timeseries and time.

Filtering by time may be configured via flags `--prom-filter-time-start` and `--prom-filter-time-end`
in RFC3339 format. This filter applied twice: to drop blocks out of range and to filter timeseries in blocks with
overlapping time range.

Example of applying time filter:

```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --prom-filter-time-start=2020-02-07T00:07:01Z \
  --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Found 2 blocks to import. Continue? [Y/n] y
```

Please notice, that total amount of blocks in provided snapshot is 14, but only 2 of them were in provided
time range. So other 12 blocks were marked as `skipped`. The amount of samples and series is not taken into account,
since this is heavy operation and will be done during import process.

Filtering by timeseries is configured with following flags:

- `--prom-filter-label` - the label name, e.g. `__name__` or `instance`;
- `--prom-filter-label-value` - the regular expression to filter the label value. By default, matches all `.*`

For example:

```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --prom-filter-label="__name__" \
  --prom-filter-label-value="promhttp.*" \
  --prom-filter-time-start=2020-02-07T00:07:01Z \
  --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Found 2 blocks to import. Continue? [Y/n] y
14 / 14 [-----------------------------------------------------------------------------------------------] 100.00% ? p/s
2020/02/23 15:51:07 Import finished!
2020/02/23 15:51:07 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 37.415461ms;
  total samples: 10128;
  samples/s: 270690.24;
  total bytes: 195.2 kB;
  bytes/s: 5.2 MB;
  import requests: 2;
  import requests retries: 0;
2020/02/23 15:51:07 Total time: 7.153158218s
```

## Migrating data by remote read protocol

`vmctl` provides the `remote-read` mode for migrating data from remote databases supporting 
[Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).
Remote read API has two implementations of remote read API: default (`SAMPLES`) and 
[streamed](https://prometheus.io/blog/2019/10/10/remote-read-meets-streaming/) (`STREAMED_XOR_CHUNKS`).
Streamed version is more efficient but has lower adoption (e.g. [Promscale](#migrating-data-from-promscale)
doesn't support it).

See `./vmctl remote-read --help` for details and the full list of flags.

To start the migration process configure the following flags:

1. `--remote-read-src-addr` - data source address to read from;
1. `--vm-addr` - VictoriaMetrics address to write to. For single-node VM is usually equal to `--httpListenAddr`, 
   and for cluster version is equal to `--httpListenAddr` flag of vminsert component (for example `http://<vminsert>:8480/insert/<accountID>/prometheus`);
1. `--remote-read-filter-time-start` - the time filter in RFC3339 format to select time series with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z';
1. `--remote-read-filter-time-end` - the time filter in RFC3339 format to select time series with timestamp equal or smaller than provided value. E.g. '2020-01-01T20:07:00Z'. Current time is used when omitted.;
1. `--remote-read-step-interval` - split export data into chunks. Valid values are `month, day, hour, minute`;
1. `--remote-read-use-stream` - defines whether to use `SAMPLES` or `STREAMED_XOR_CHUNKS` mode. By default, is uses `SAMPLES` mode.

The importing process example for local installation of Prometheus
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl remote-read \
--remote-read-src-addr=http://<prometheus>:9091 \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://<victoria-metrics>:8428 \
--vm-concurrency=6

Split defined times into 8798 ranges to import. Continue? [Y/n]
VM worker 0:↘ 127177 samples/s
VM worker 1:↘ 140137 samples/s
VM worker 2:↘ 151606 samples/s
VM worker 3:↘ 130765 samples/s
VM worker 4:↘ 131904 samples/s
VM worker 5:↘ 132693 samples/s
Processing ranges: 8798 / 8798 [██████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/19 16:45:37 Import finished!
2022/10/19 16:45:37 VictoriaMetrics importer stats:
  idle duration: 6m57.793987511s;
  time spent while importing: 1m18.463744801s;
  total samples: 25348208;
  samples/s: 323056.31;
  total bytes: 669.7 MB;
  bytes/s: 8.5 MB;
  import requests: 127;
  import requests retries: 0;
2022/10/19 16:45:37 Total time: 1m19.406283424s
```

Migrating big volumes of data may result in remote read client reaching the timeout.
Consider increasing the value of `--remote-read-http-timeout` (default `5m`) command-line flag when seeing
timeouts or `context canceled` errors.

### Filtering

The filtering consists of two parts: by labels and time.

Filtering by time can be configured via flags `--remote-read-filter-time-start` and `--remote-read-filter-time-end`
in RFC3339 format.

Filtering by labels can be configured via flags `--remote-read-filter-label` and `--remote-read-filter-label-value`.
For example, `--remote-read-filter-label=tenant` and `--remote-read-filter-label-value="team-eu"` will select only series
with `tenant="team-eu"` label-value pair.

## Migrating data from Thanos

Thanos uses the same storage engine as Prometheus and the data layout on-disk should be the same. That means
`vmctl` in mode `prometheus` may be used for Thanos historical data migration as well.
These instructions may vary based on the details of your Thanos configuration.
Please read carefully and verify as you go. We assume you're using Thanos Sidecar on your Prometheus pods,
and that you have a separate Thanos Store installation.

### Current data

1. For now, keep your Thanos Sidecar and Thanos-related Prometheus configuration, but add this to also stream
    metrics to VictoriaMetrics:

    ```yaml
    remote_write:
    - url: http://victoria-metrics:8428/api/v1/write
    ```

1. Make sure VM is running, of course. Now check the logs to make sure that Prometheus is sending and VM is receiving.
    In Prometheus, make sure there are no errors. On the VM side, you should see messages like this:

    ```sh
    2020-04-27T18:38:46.474Z info VictoriaMetrics/lib/storage/partition.go:207 creating a partition "2020_04" with smallPartsPath="/victoria-metrics-data/data/small/2020_04", bigPartsPath="/victoria-metrics-data/data/big/2020_04"
    2020-04-27T18:38:46.506Z info VictoriaMetrics/lib/storage/partition.go:222 partition "2020_04" has been created
    ```

1. Now just wait. Within two hours, Prometheus should finish its current data file and hand it off to Thanos Store for long term
    storage.

### Historical data

Let's assume your data is stored on S3 served by minio. You first need to copy that out to a local filesystem,
then import it into VM using `vmctl` in `prometheus` mode.

1. Copy data from minio.
    1. Run the `minio/mc` Docker container.
    1. `mc config host add minio http://minio:9000 accessKey secretKey`, substituting appropriate values for the last 3 items.
    1. `mc cp -r minio/prometheus thanos-data`

1. Import using `vmctl`.
    1. Follow the [instructions](#how-to-build) to compile `vmctl` on your machine.
    1. Use [prometheus](#migrating-data-from-prometheus) mode to import data:

    ```
    vmctl prometheus --prom-snapshot thanos-data --vm-addr http://victoria-metrics:8428
    ```

### Remote read protocol

Currently, Thanos doesn't support streaming remote read protocol. It is [recommended](https://thanos.io/tip/thanos/integrations.md/#storeapi-as-prometheus-remote-read)
to use [thanos-remote-read](https://github.com/G-Research/thanos-remote-read) a proxy, that allows exposing any Thanos 
service (or anything that exposes gRPC StoreAPI e.g. Querier) via Prometheus remote read protocol.

If you want to migrate data, you should run [thanos-remote-read](https://github.com/G-Research/thanos-remote-read) proxy
and define the Thanos store address `./thanos-remote-read -store 127.0.0.1:19194`. 
It is important to know that `store` flag is Thanos Store API gRPC endpoint. 
Also, it is important to know that thanos-remote-read proxy doesn't support stream mode.
When you run thanos-remote-read proxy, it exposes port to serve HTTP on `10080 by default`.

The importing process example for local installation of Thanos
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl remote-read \
--remote-read-src-addr=http://127.0.0.1:10080 \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://127.0.0.1:8428 \
--vm-concurrency=6
```

On the [thanos-remote-read](https://github.com/G-Research/thanos-remote-read) proxy side you will see logs like:
```sh
ts=2022-10-19T15:05:04.193916Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666180800000 max_time:1666184399999 matchers:<type:RE value:\".*\" > aggregates:RAW "
ts=2022-10-19T15:05:04.468852Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666184400000 max_time:1666187999999 matchers:<type:RE value:\".*\" > aggregates:RAW "
ts=2022-10-19T15:05:04.553914Z caller=main.go:278 level=info traceID=00000000000000000000000000000000 msg="thanos request" request="min_time:1666188000000 max_time:1666191364863 matchers:<type:RE value:\".*\" > aggregates:RAW "
```

And when process will finish you will see:
```sh
Split defined times into 8799 ranges to import. Continue? [Y/n]
VM worker 0:↓ 98183 samples/s
VM worker 1:↓ 114640 samples/s
VM worker 2:↓ 131710 samples/s
VM worker 3:↓ 114256 samples/s
VM worker 4:↓ 105671 samples/s
VM worker 5:↓ 124000 samples/s
Processing ranges: 8799 / 8799 [██████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/19 18:05:07 Import finished!
2022/10/19 18:05:07 VictoriaMetrics importer stats:
  idle duration: 52m13.987637229s;
  time spent while importing: 9m1.728983776s;
  total samples: 70836111;
  samples/s: 130759.32;
  total bytes: 2.2 GB;
  bytes/s: 4.0 MB;
  import requests: 356;
  import requests retries: 0;
2022/10/19 18:05:07 Total time: 9m2.607521618s
```

## Migrating data from Cortex

Cortex has an implementation of the Prometheus remote read protocol. That means
`vmctl` in mode `remote-read` may also be used for Cortex historical data migration.
These instructions may vary based on the details of your Cortex configuration.
Please read carefully and verify as you go.

### Remote read protocol

If you want to migrate data, you should check your cortex configuration in the section
```yaml
api:
  prometheus_http_prefix:
```

If you defined some prometheus prefix, you should use it when you define flag `--remote-read-src-addr=http://127.0.0.1:9009/{prometheus_http_prefix}`.
By default, Cortex uses the `prometheus` path prefix, so you should define the flag `--remote-read-src-addr=http://127.0.0.1:9009/prometheus`.

It is important to know that Cortex doesn't support the stream mode.
When you run Cortex, it exposes a port to serve HTTP on `9009 by default`.

The importing process example for the local installation of Cortex
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl remote-read \ 
--remote-read-src-addr=http://127.0.0.1:9009/prometheus \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://127.0.0.1:8428 \
--vm-concurrency=6 
```
And when the process finishes, you will see the following:

```sh
Split defined times into 8842 ranges to import. Continue? [Y/n]
VM worker 0:↗ 3863 samples/s
VM worker 1:↗ 2686 samples/s
VM worker 2:↗ 2620 samples/s
VM worker 3:↗ 2705 samples/s
VM worker 4:↗ 2643 samples/s
VM worker 5:↗ 2593 samples/s
Processing ranges: 8842 / 8842 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/21 12:09:49 Import finished!
2022/10/21 12:09:49 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 3.82640757s;
  total samples: 160232;
  samples/s: 41875.31;
  total bytes: 11.3 MB;
  bytes/s: 3.0 MB;
  import requests: 6;
  import requests retries: 0;
2022/10/21 12:09:49 Total time: 4.71824253s
```
It is important to know that if you run your Cortex installation in multi-tenant mode, remote read protocol
requires an Authentication header like `X-Scope-OrgID`. You can define it via the flag `--remote-read-headers=X-Scope-OrgID:demo`

## Migrating data from Mimir

Mimir has similar implementation as Cortex and supports Prometheus remote read API. That means historical data
from Mimir can be migrated via `vmctl` in mode `remote-read` mode.
The instructions for data migration via vmctl vary based on the details of your Mimir configuration.
Please read carefully and verify as you go.

### Remote read protocol

By default, Mimir uses the `prometheus` path prefix so specifying the source
should be as simple as `--remote-read-src-addr=http://<mimir>:9009/prometheus`.
But if prefix was overriden via `prometheus_http_prefix`, then source address should be updated
to `--remote-read-src-addr=http://<mimir>:9009/{prometheus_http_prefix}`.

Mimir supports [streamed remote read API](https://prometheus.io/blog/2019/10/10/remote-read-meets-streaming/),
so it is recommended setting `--remote-read-use-stream=true` flag for better performance and resource usage.

When you run Mimir, it exposes a port to serve HTTP on `8080 by default`.

Next example of the local installation was in multi-tenant mode (3 instances of Mimir) with nginx as load balancer.
Load balancer expose single port `:9090`.

As you can see in the example we call `:9009` instead of `:8080` because of proxy.

The importing process example for the local installation of Mimir
and single-node VictoriaMetrics(`http://localhost:8428`):

```
./vmctl remote-read 
--remote-read-src-addr=http://<mimir>:9009/prometheus \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--remote-read-headers=X-Scope-OrgID:demo \
--remote-read-use-stream=true \
--vm-addr=http://<victoria-metrics>:8428 \
```

And when the process finishes, you will see the following:

```sh
Split defined times into 8847 ranges to import. Continue? [Y/n]
VM worker 0:→ 12176 samples/s
VM worker 1:→ 11918 samples/s
VM worker 2:→ 11261 samples/s
VM worker 3:→ 12861 samples/s
VM worker 4:→ 11096 samples/s
VM worker 5:→ 11575 samples/s
Processing ranges: 8847 / 8847 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/21 17:22:23 Import finished!
2022/10/21 17:22:23 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 15.379614356s;
  total samples: 81243;
  samples/s: 5282.51;
  total bytes: 6.1 MB;
  bytes/s: 397.8 kB;
  import requests: 6;
  import requests retries: 0;
2022/10/21 17:22:23 Total time: 16.287405248s
```

It is important to know that if you run your Mimir installation in multi-tenant mode, remote read protocol
requires an Authentication header like `X-Scope-OrgID`. You can define it via the flag `--remote-read-headers=X-Scope-OrgID:demo`

## Migrating data from VictoriaMetrics

The simplest way to migrate data between VM instances is [to copy data between instances](https://docs.victoriametrics.com/single-server-victoriametrics/#data-migration).

vmctl uses [native binary protocol](https://docs.victoriametrics.com/#how-to-export-data-in-native-format)
(available since [1.42.0 release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0))
to migrate data between VM instances: single to single, cluster to cluster, single to cluster and vice versa.

See `./vmctl vm-native --help` for details and full list of flags.

Migration in `vm-native` mode takes two steps:
1. Explore the list of the metrics to migrate via `api/v1/label/__name__/values` API;
1. Migrate explored metrics one-by-one with specified `--vm-concurrency`.

```sh
./vmctl vm-native \
    --vm-native-src-addr=http://127.0.0.1:8481/select/0/prometheus \ # migrate from
    --vm-native-dst-addr=http://localhost:8428 \                     # migrate to
    --vm-native-filter-time-start='2022-11-20T00:00:00Z' \           # starting from
    --vm-native-filter-match='{__name__!~"vm_.*"}'                   # filter out metrics matching the selector
VictoriaMetrics Native import mode

2023/03/02 09:22:02 Initing import process from "http://127.0.0.1:8481/select/0/prometheus/api/v1/export/native" 
                    to "http://localhost:8428/api/v1/import/native" with filter 
        filter: match[]={__name__!~"vm_.*"}
        start: 2022-11-20T00:00:00Z
2023/03/02 09:22:02 Exploring metrics...
Found 9 metrics to import. Continue? [Y/n] 
2023/03/02 09:22:04 Requests to make: 9
Requests to make: 9 / 9 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2023/03/02 09:22:06 Import finished!
2023/03/02 09:22:06 VictoriaMetrics importer stats:
  time spent while importing: 3.632638875s;
  total bytes: 7.8 MB;
  bytes/s: 2.1 MB;
  requests: 9;
  requests retries: 0;
2023/03/02 09:22:06 Total time: 3.633127625s
```

_To disable explore phase and switch to the old way of data migration via single connection use 
`--vm-native-disable-per-metric-migration` cmd-line flag. Please note, in this mode vmctl won't be able to retry failed requests._

Importing tips:

1. vmctl acts as a proxy between `src` and `dst`. It doesn't use much of CPU or RAM, but network connection
   between `src`=>vmctl=>`dst` should be as fast as possible for improving the migration speed.
1. Migrating big volumes of data may result in reaching the safety limits on `src` side.
   Please verify that `-search.maxExportDuration` and `-search.maxExportSeries` were set with
   proper values for `src`. If hitting the limits, follow the recommendations 
   [here](https://docs.victoriametrics.com/#how-to-export-data-in-native-format).
   If hitting `the number of matching timeseries exceeds...` error, adjust filters to match less time series or 
   update `-search.maxSeries` command-line flag on vmselect/vmsingle;
1. Using smaller intervals via `--vm-native-step-interval` cmd-line flag can reduce the number of matched series per-request
   for sources with [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate).
   See more about [step interval here](#using-time-based-chunking-of-migration).
1. Migrating all the metrics from one VM to another may collide with existing application metrics
   (prefixed with `vm_`) at destination and lead to confusion when using
   [official Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).
   To avoid such situation try to filter out VM process metrics via `--vm-native-filter-match='{__name__!~"vm_.*"}'` flag.
1. Migrating data with overlapping time range or via unstable network can produce duplicates series at destination.
   To avoid duplicates set `-dedup.minScrapeInterval=1ms` for `vmselect`/`vmstorage` at the destination.
   This will instruct `vmselect`/`vmstorage` to ignore duplicates with identical timestamps. Ignore this recommendation
   if you already have `-dedup.minScrapeInterval` set to 1ms or higher values at destination.
1. When migrating data from one VM cluster to another, consider using [cluster-to-cluster mode](#cluster-to-cluster-migration-mode).
   Or manually specify addresses according to [URL format](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format):
   ```sh
   # Migrating from cluster specific tenantID to single
   --vm-native-src-addr=http://<src-vmselect>:8481/select/0/prometheus
   --vm-native-dst-addr=http://<dst-vmsingle>:8428
    
   # Migrating from single to cluster specific tenantID
   --vm-native-src-addr=http://<src-vmsingle>:8428
   --vm-native-dst-addr=http://<dst-vminsert>:8480/insert/0/prometheus
    
   # Migrating single to single
   --vm-native-src-addr=http://<src-vmsingle>:8428
   --vm-native-dst-addr=http://<dst-vmsingle>:8428
    
   # Migrating cluster to cluster for specific tenant ID
   --vm-native-src-addr=http://<src-vmselect>:8481/select/0/prometheus
   --vm-native-dst-addr=http://<dst-vminsert>:8480/insert/0/prometheus
   ```
1. Migrating data from VM cluster which had replication (`-replicationFactor` > 1) enabled won't produce the same amount
   of data copies for the destination database, and will result only in creating duplicates. To remove duplicates,
   destination database need to be configured with `-dedup.minScrapeInterval=1ms`. To restore the replication factor
   the destination `vminsert` component need to be configured with the according `-replicationFactor` value. 
   See more about replication [here](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety).
1. Migration speed can be adjusted via `--vm-concurrency` cmd-line flag, which controls the number of concurrent 
   workers busy with processing. Please note, that each worker can load up to a single vCPU core on VictoriaMetrics. 
   So try to set it according to allocated CPU resources of your VictoriaMetrics destination installation.
1. Migration is a backfilling process, so it is recommended to read
   [Backfilling tips](https://github.com/VictoriaMetrics/VictoriaMetrics#backfilling) section.
1. `vmctl` doesn't provide relabeling or other types of labels management.
   Instead, use [relabeling in VictoriaMetrics](https://github.com/VictoriaMetrics/vmctl/issues/4#issuecomment-683424375).
1. `vmctl` supports `--vm-native-src-headers` and `--vm-native-dst-headers` to define headers sent with each request
   to the corresponding source address.
1. `vmctl` supports `--vm-native-disable-http-keep-alive` to allow `vmctl` to use non-persistent HTTP connections to avoid
   error `use of closed network connection` when running a heavy export requests.


### Using time-based chunking of migration

It is possible to split the migration process into steps based on time via `--vm-native-step-interval` cmd-line flag.
Supported values are: `month`, `week`, `day`, `hour`, `minute`. For example, when migrating 1 year of data with
`--vm-native-step-interval=month` vmctl will execute it in 12 separate requests from the beginning of the interval
to its end. To reverse the order set `--vm-native-filter-time-reverse` and migration will start from the newest to the 
oldest data. `--vm-native-filter-time-start` is required to be set when using `--vm-native-step-interval`.

It is recommended using default `month` step when migrating the data over the long time intervals. If you hit complexity
limits on `--vm-native-src-addr` and can't or don't want to change them, try lowering the step interval to `week`, `day` or `hour`.

Usage example:
```sh
./vmctl vm-native \
    --vm-native-src-addr=http://127.0.0.1:8481/select/0/prometheus \ 
    --vm-native-dst-addr=http://localhost:8428 \
    --vm-native-filter-time-start='2022-11-20T00:00:00Z' \
    --vm-native-step-interval=month \
    --vm-native-filter-match='{__name__!~"vm_.*"}'    
VictoriaMetrics Native import mode

2023/03/02 09:18:05 Initing import process from "http://127.0.0.1:8481/select/0/prometheus/api/v1/export/native" to "http://localhost:8428/api/v1/import/native" with filter 
        filter: match[]={__name__!~"vm_.*"}
        start: 2022-11-20T00:00:00Z
2023/03/02 09:18:05 Exploring metrics...
Found 9 metrics to import. Continue? [Y/n] 
2023/03/02 09:18:07 Selected time range will be split into 5 ranges according to "month" step. Requests to make: 45.
Requests to make: 45 / 45 [██████████████████████████████████████████████████████████████████████████████████] 100.00%
2023/03/02 09:18:12 Import finished!
2023/03/02 09:18:12 VictoriaMetrics importer stats:
  time spent while importing: 7.111870667s;
  total bytes: 7.7 MB;
  bytes/s: 1.1 MB;
  requests: 45;
  requests retries: 0;
2023/03/02 09:18:12 Total time: 7.112405875s
```

### Cluster-to-cluster migration mode

Using cluster-to-cluster migration mode helps to migrate all tenants data in a single `vmctl` run.

Cluster-to-cluster uses `/admin/tenants` endpoint (available starting from [v1.84.0](https://docs.victoriametrics.com/changelog/#v1840)) to discover list of tenants from source cluster.

To use this mode you need to set `--vm-intercluster` flag to `true`, `--vm-native-src-addr` flag to 'http://vmselect:8481/' and `--vm-native-dst-addr` value to http://vminsert:8480/:

```sh
  ./vmctl vm-native --vm-native-src-addr=http://127.0.0.1:8481/ \
  --vm-native-dst-addr=http://127.0.0.1:8480/ \
  --vm-native-filter-match='{__name__="vm_app_uptime_seconds"}' \
  --vm-native-filter-time-start='2023-02-01T00:00:00Z' \
  --vm-native-step-interval=day \  
  --vm-intercluster
  
VictoriaMetrics Native import mode
2023/02/28 10:41:42 Discovering tenants...
2023/02/28 10:41:42 The following tenants were discovered: [0:0 1:0 2:0 3:0 4:0]
2023/02/28 10:41:42 Initing import process from "http://127.0.0.1:8481/select/0:0/prometheus/api/v1/export/native" to "http://127.0.0.1:8480/insert/0:0/prometheus/api/v1/import/native" with filter 
        filter: match[]={__name__="vm_app_uptime_seconds"}
        start: 2023-02-01T00:00:00Z for tenant 0:0 
2023/02/28 10:41:42 Exploring metrics...
2023/02/28 10:41:42 Found 1 metrics to import 
2023/02/28 10:41:42 Selected time range will be split into 28 ranges according to "day" step. 
Requests to make for tenant 0:0: 28 / 28 [████████████████████████████████████████████████████████████████████] 100.00%

2023/02/28 10:41:45 Initing import process from "http://127.0.0.1:8481/select/1:0/prometheus/api/v1/export/native" to "http://127.0.0.1:8480/insert/1:0/prometheus/api/v1/import/native" with filter 
        filter: match[]={__name__="vm_app_uptime_seconds"}
        start: 2023-02-01T00:00:00Z for tenant 1:0 
2023/02/28 10:41:45 Exploring metrics...
2023/02/28 10:41:45 Found 1 metrics to import 
2023/02/28 10:41:45 Selected time range will be split into 28 ranges according to "day" step. Requests to make: 28 
Requests to make for tenant 1:0: 28 / 28 [████████████████████████████████████████████████████████████████████] 100.00%

...

2023/02/28 10:42:49 Import finished!
2023/02/28 10:42:49 VictoriaMetrics importer stats:
  time spent while importing: 1m6.714210417s;
  total bytes: 39.7 MB;
  bytes/s: 594.4 kB;
  requests: 140;
  requests retries: 0;
2023/02/28 10:42:49 Total time: 1m7.147971417s
```

### Configuration
Run the following command to get all configuration options:
```sh
./vmctl vm-native --help
```

## Tuning

## Verifying exported blocks from VictoriaMetrics

In this mode, `vmctl` allows verifying correctness and integrity of data exported via 
[native format](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-export-data-in-native-format)
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

## Tuning

### InfluxDB mode

The flag `--influx-concurrency` controls how many concurrent requests may be sent to InfluxDB while fetching
timeseries. Please set it wisely to avoid InfluxDB overwhelming.

The flag `--influx-chunk-size` controls the max amount of datapoints to return in single chunk from fetch requests.
Please see more details [here](https://docs.influxdata.com/influxdb/v1.7/guides/querying_data/#chunking).
The chunk size is used to control InfluxDB memory usage, so it won't OOM on processing large timeseries with
billions of datapoints.

### Prometheus mode

The flag `--prom-concurrency` controls how many concurrent readers will be reading the blocks in snapshot.
Since snapshots are just files on disk it would be hard to overwhelm the system. Please go with value equal
to number of free CPU cores.

### VictoriaMetrics importer

The flag `--vm-concurrency` controls the number of concurrent workers that process the import requests to **destination**.
Please note that each import request can load up to a single vCPU core on VictoriaMetrics. So try to set it according
to allocated CPU resources of your VictoriaMetrics installation.

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

By default `vmctl` waits confirmation from user before starting the import. If this is unwanted
behavior and no user interaction required - pass `-s` flag to enable "silence" mode:

See below the example of `vm-native` migration process:
```
    -s Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
```

### Significant figures

`vmctl` allows to limit the number of [significant figures](https://en.wikipedia.org/wiki/Significant_figures)
before importing. For example, the average value for response size is `102.342305` bytes and it has 9 significant figures.
If you ask a human to pronounce this value then with high probability value will be rounded to first 4 or 5 figures
because the rest aren't really that important to mention. In most cases, such a high precision is too much.
Moreover, such values may be just a result of [floating point arithmetic](https://en.wikipedia.org/wiki/Floating-point_arithmetic),
create a [false precision](https://en.wikipedia.org/wiki/False_precision) and result into bad compression ratio
according to [information theory](https://en.wikipedia.org/wiki/Information_theory).

`vmctl` provides the following flags for improving data compression:

- `--vm-round-digits` flag for rounding processed values to the given number of decimal digits after the point.
  For example, `--vm-round-digits=2` would round `1.2345` to `1.23`. By default, the rounding is disabled.

- `--vm-significant-figures` flag for limiting the number of significant figures in processed values. It takes no effect if set
  to 0 (by default), but set `--vm-significant-figures=5` and `102.342305` will be rounded to `102.34`.

The most common case for using these flags is to improve data compression for time series storing aggregation
results such as `average`, `rate`, etc.

### Adding extra labels

 `vmctl` allows to add extra labels to all imported series. It can be achieved with flag `--vm-extra-label label=value`.
 If multiple labels needs to be added, set flag for each label, for example, `--vm-extra-label label1=value1 --vm-extra-label label2=value2`.
 If timeseries already have label, that must be added with `--vm-extra-label` flag, flag has priority and will override label value from timeseries.

### Rate limiting

Limiting the rate of data transfer could help to reduce pressure on disk or on destination database.
The rate limit may be set in bytes-per-second via `--vm-rate-limit` flag.

Please note, you can also use [vmagent](https://docs.victoriametrics.com/vmagent/)
as a proxy between `vmctl` and destination with `-remoteWrite.rateLimit` flag enabled.

## How to build

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - `vmctl` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.22.
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

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.22.
1. Run `make vmctl-linux-arm` or `make vmctl-linux-arm64` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-linux-arm` or `vmctl-linux-arm64` binary respectively and puts it into the `bin` folder.

#### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmctl-linux-arm-prod` or `make vmctl-linux-arm64-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-linux-arm-prod` or `vmctl-linux-arm64-prod` binary respectively and puts it into the `bin` folder.
