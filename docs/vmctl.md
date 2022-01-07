---
sort: 8
---

# vmctl

VictoriaMetrics command-line tool

Features:
- [x] Prometheus: migrate data from Prometheus to VictoriaMetrics using snapshot API
- [x] Thanos: migrate data from Thanos to VictoriaMetrics
- [ ] ~~Prometheus: migrate data from Prometheus to VictoriaMetrics by query~~(discarded)
- [x] InfluxDB: migrate data from InfluxDB to VictoriaMetrics
- [x] OpenTSDB: migrate data from OpenTSDB to VictoriaMetrics
- [ ] Storage Management: data re-balancing between nodes 

vmctl acts as a proxy between data source ([Prometheus](#migrating-data-from-prometheus), 
[InfluxDB](#migrating-data-from-influxdb-1x), [VictoriaMetrics](##migrating-data-from-victoriametrics), etc.)
and destination - VictoriaMetrics single or cluster version. To see the full list of supported modes 
run the following command:
```
./vmctl --help                                        
NAME:
   vmctl - VictoriaMetrics command-line tool

USAGE:
   vmctl [global options] command [command options] [arguments...]

COMMANDS:
   opentsdb    Migrate timeseries from OpenTSDB
   influx      Migrate timeseries from InfluxDB
   prometheus  Migrate timeseries from Prometheus
   vm-native   Migrate time series between VictoriaMetrics installations via native binary format
```

Each mode has its own unique set of flags specific (e.g. prefixed with `influx` for influx mode)
to the data source and common list of flags for destination (prefixed with `vm` for VictoriaMetrics):
```
./vmctl influx --help
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

When doing a migration user needs to specify flags for source (where and how to fetch data) and for
destination (where to migrate data). Every mode has additional details and nuances, please see 
them below in corresponding sections.

For the destination flags see the full description by running the following command:
```
./vmctl influx --help | grep vm-
```

Some flags like [--vm-extra-label](#adding-extra-labels) or [--vm-significant-figures](#significant-figures)
has additional sections with description below. Details about tweaking and adjusting settings
are explained in [Tuning](#tuning) section.

Please note, that if you're going to import data into VictoriaMetrics cluster do not
forget to specify the `--vm-account-id` flag. See more details for cluster version 
[here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

## Articles

* [How to migrate data from Prometheus](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-d44a6728f043)
* [How to migrate data from Prometheus. Filtering and modifying time series](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-filtering-and-modifying-time-series-6d40cea4bf21)


## Migrating data from OpenTSDB

`vmctl` supports the `opentsdb` mode to migrate data from OpenTSDB to VictoriaMetrics time-series database.

See `./vmctl opentsdb --help` for details and full list of flags.

*OpenTSDB migration is not possible without a functioning [meta](http://opentsdb.net/docs/build/html/user_guide/metadata.html) table to search for metrics/series.*

OpenTSDB migration works like so:

1. Find metrics based on selected filters (or the default filter set ['a','b','c','d','e','f','g','h','i','j','k','l','m','n','o','p','q','r','s','t','u','v','w','x','y','z'])
  * e.g. `curl -Ss "http://opentsdb:4242/api/suggest?type=metrics&q=sys"`
2. Find series associated with each returned metric
  * e.g. `curl -Ss "http://opentsdb:4242/api/search/lookup?m=system.load5&limit=1000000"`
3. Download data for each series in chunks defined in the CLI switches
  * e.g. `-retention=sum-1m-avg:1h:90d` ==
    * `curl -Ss "http://opentsdb:4242/api/query?start=1h-ago&end=now&m=sum:1m-avg-none:system.load5\{host=host1\}"`
    * `curl -Ss "http://opentsdb:4242/api/query?start=2h-ago&end=1h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`
    * `curl -Ss "http://opentsdb:4242/api/query?start=3h-ago&end=2h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`
    * ...
    * `curl -Ss "http://opentsdb:4242/api/query?start=2160h-ago&end=2159h-ago&m=sum:1m-avg-none:system.load5\{host=host1\}"`

This means that we must stream data from OpenTSDB to VictoriaMetrics in chunks. This is where concurrency for OpenTSDB comes in. We can query multiple chunks at once, but we shouldn't perform too many chunks at a time to avoid overloading the OpenTSDB cluster.

```
$ bin/vmctl opentsdb --otsdb-addr http://opentsdb:4242/ --otsdb-retentions sum-1m-avg:1h:1d --otsdb-filters system --otsdb-normalize --vm-addr http://victoria/
OpenTSDB import mode
2021/04/09 11:52:50 Will collect data starting at TS 1617990770
2021/04/09 11:52:50 Loading all metrics from OpenTSDB for filters:  [system]
Found 9 metrics to import. Continue? [Y/n] 
2021/04/09 11:52:51 Starting work on system.load1
23 / 402200 [>____________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________] 0.01% 2 p/s
```

### Retention strings

Starting with a relatively simple retention string (`sum-1m-avg:1h:30d`), let's describe how this is converted into actual queries.

There are two essential parts of a retention string:
1. [aggregation](#aggregation)
2. [windows/time ranges](#windows)

#### Aggregation

Retention strings essentially define the two levels of aggregation for our collected series.

`sum-1m-avg` would become:
* First order: `sum`
* Second order: `1m-avg-none`

##### First Order Aggregations

First-order aggregation addresses how to aggregate any un-mentioned tags.

This is, conceptually, directly opposite to how PromQL deals with tags. In OpenTSDB, if a tag isn't explicitly mentioned, all values assocaited with that tag will be aggregated.

It is recommended to use `sum` for the first aggregation because it is relatively quick and should not cause any changes to the incoming data (because we collect each individual series).

##### Second Order Aggregations

Second-order aggregation (`1m-avg` in our example) defines any windowing that should occur before returning the data

It is recommended to match the stat collection interval so we again avoid transforming incoming data.

We do not allow for defining the "null value" portion of the rollup window (e.g. in the aggreagtion, `1m-avg-none`, the user cannot change `none`), as the goal of this tool is to avoid modifying incoming data.

#### Windows

There are two important windows we define in a retention string:
1. the "chunk" range of each query
2. The time range we will be querying on with that "chunk"

From our example, our windows are `1h:30d`.

##### Window "chunks"

The window `1h` means that each individual query to OpenTSDB should only span 1 hour of time (e.g. `start=2h-ago&end=1h-ago`).

It is important to ensure this window somewhat matches the row size in HBase to help improve query times.

For example, if the query is hitting a rollup table with a 4 hour row size, we should set a chunk size of a multiple of 4 hours (e.g. `4h`, `8h`, etc.) to avoid requesting data across row boundaries. Landing on row boundaries allows for more consistent request times to HBase.

The default table created in HBase for OpenTSDB has a 1 hour row size, so if you aren't sure on a correct row size to use, `1h` is a reasonable choice.

##### Time range

The time range `30d` simply means we are asking for the last 30 days of data. This time range can be written using `h`, `d`, `w`, or `y`. (We can't use `m` for month because it already means `minute` in time parsing).

#### Results of retention string

The resultant queries that will be created, based on our example retention string of `sum-1m-avg:1h:30d` look like this:

```
http://opentsdb:4242/api/query?start=1h-ago&end=now&m=sum:1m-avg-none:<series>
http://opentsdb:4242/api/query?start=2h-ago&end=1h-ago&m=sum:1m-avg-none:<series>
http://opentsdb:4242/api/query?start=3h-ago&end=2h-ago&m=sum:1m-avg-none:<series>
...
http://opentsdb:4242/api/query?start=721h-ago&end=720h-ago&m=sum:1m-avg-none:<series>
```

Chunking the data like this means each individual query returns faster, so we can start populating data into VictoriaMetrics quicker.

### Restarting OpenTSDB migrations

One important note for OpenTSDB migration: Queries/HBase scans can "get stuck" within OpenTSDB itself. This can cause instability and performance issues within an OpenTSDB cluster, so stopping the migrator to deal with it may be necessary. Because of this, we provide the timstamp we started collecting data from at thebeginning of the run. You can stop and restart the importer using this "hard timestamp" to ensure you collect data from the same time range over multiple runs.

## Migrating data from InfluxDB (1.x)

`vmctl` supports the `influx` mode to migrate data from InfluxDB to VictoriaMetrics time-series database.

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
```
./vmctl influx --influx-database benchmark
InfluxDB import mode
2020/01/18 20:47:11 Exploring scheme for database "benchmark"
2020/01/18 20:47:11 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/18 20:47:11 found 10 fields
2020/01/18 20:47:11 fetching series: command: "show series "; database: "benchmark"; retention: "autogen"
Found 40000 timeseries to import. Continue? [Y/n] y
40000 / 40000 [-----------------------------------------------------------------------------------------------------------------------------------------------] 100.00% 21 p/s
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

* `influx-database` arg is mapped into `db` label value unless `db` tag exists in the InfluxDB line.
* Field names are mapped to time series names prefixed with {measurement}{separator} value, 
where {separator} equals to _ by default. 
It can be changed with `--influx-measurement-field-separator` command-line flag.
* Field values are mapped to time series values.
* Tags are mapped to Prometheus labels format as-is.

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

The configuration flags should contain self-explanatory descriptions. 

### Filtering

The filtering consists of two parts: timeseries and time.
The first step of application is to select all available timeseries
for given database and retention. User may specify additional filtering
condition via `--influx-filter-series` flag. For example:
```
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
* --influx-filter-time-start 
* --influx-filter-time-end 
Here's an example of importing timeseries for one day only:
`./vmctl influx --influx-database benchmark --influx-filter-series "where hostname='host_1703'" --influx-filter-time-start "2020-01-01T10:07:00Z" --influx-filter-time-end "2020-01-01T15:07:00Z"`

Please see more about time filtering [here](https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#filter-meta-queries-by-time).

## Migrating data from InfluxDB (2.x)

Migrating data from InfluxDB v2.x is not supported yet ([#32](https://github.com/VictoriaMetrics/vmctl/issues/32)).
You may find useful a 3rd party solution for this - https://github.com/jonppe/influx_to_victoriametrics.


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
```
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

The configuration flags should contain self-explanatory descriptions. 

### Filtering

The filtering consists of three parts: by timeseries and time.

Filtering by time may be configured via flags `--prom-filter-time-start` and `--prom-filter-time-end`
in in RFC3339 format. This filter applied twice: to drop blocks out of range and to filter timeseries in blocks with
overlapping time range.

Example of applying time filter:
```
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
* `--prom-filter-label` - the label name, e.g. `__name__` or `instance`;
* `--prom-filter-label-value` - the regular expression to filter the label value. By default matches all `.*`

For example:
```
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
14 / 14 [------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% ? p/s
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

## Migrating data from Thanos

Thanos uses the same storage engine as Prometheus and the data layout on-disk should be the same. That means
`vmctl` in mode `prometheus` may be used for Thanos historical data migration as well.
These instructions may vary based on the details of your Thanos configuration. 
Please read carefully and verify as you go. We assume you're using Thanos Sidecar on your Prometheus pods, 
and that you have a separate Thanos Store installation.

### Current data

1. For now, keep your Thanos Sidecar and Thanos-related Prometheus configuration, but add this to also stream 
    metrics to VictoriaMetrics:
    ```
    remote_write:
    - url: http://victoria-metrics:8428/api/v1/write
    ```
2. Make sure VM is running, of course. Now check the logs to make sure that Prometheus is sending and VM is receiving. 
    In Prometheus, make sure there are no errors. On the VM side, you should see messages like this:
    ```
    2020-04-27T18:38:46.474Z	info	VictoriaMetrics/lib/storage/partition.go:207	creating a partition "2020_04" with smallPartsPath="/victoria-metrics-data/data/small/2020_04", bigPartsPath="/victoria-metrics-data/data/big/2020_04"
    2020-04-27T18:38:46.506Z	info	VictoriaMetrics/lib/storage/partition.go:222	partition "2020_04" has been created
    ```
3. Now just wait. Within two hours, Prometheus should finish its current data file and hand it off to Thanos Store for long term
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

## Migrating data from VictoriaMetrics

### Native protocol

The [native binary protocol](https://docs.victoriametrics.com/#how-to-export-data-in-native-format)
was introduced in [1.42.0 release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0)
and provides the most efficient way to migrate data between VM instances: single to single, cluster to cluster,
single to cluster and vice versa. Please note that both instances (source and destination) should be of v1.42.0
or higher.

See `./vmctl vm-native --help` for details and full list of flags.

In this mode `vmctl` acts as a proxy between two VM instances, where time series filtering is done by "source" (`src`) 
and processing is done by "destination" (`dst`). Because of that, `vmctl` doesn't actually know how much data will be 
processed and can't show the progress bar. It will show the current processing speed and total number of processed bytes:

```
./vmctl vm-native --vm-native-src-addr=http://localhost:8528  \
  --vm-native-dst-addr=http://localhost:8428 \
  --vm-native-filter-match='{job="vmagent"}' \
  --vm-native-filter-time-start='2020-01-01T20:07:00Z'
VictoriaMetrics Native import mode
Initing export pipe from "http://localhost:8528" with filters: 
        filter: match[]={job="vmagent"}
Initing import process to "http://localhost:8428":
Total: 336.75 KiB ↖ Speed: 454.46 KiB p/s                                                                                                               
2020/10/13 17:04:59 Total time: 952.143376ms
``` 

Importing tips:
1. Migrating all the metrics from one VM to another may collide with existing application metrics 
(prefixed with `vm_`) at destination and lead to confusion when using 
[official Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards). 
To avoid such situation try to filter out VM process metrics via `--vm-native-filter-match` flag.
2. Migration is a backfilling process, so it is recommended to read 
[Backfilling tips](https://github.com/VictoriaMetrics/VictoriaMetrics#backfilling) section.
3. `vmctl` doesn't provide relabeling or other types of labels management in this mode.
Instead, use [relabeling in VictoriaMetrics](https://github.com/VictoriaMetrics/vmctl/issues/4#issuecomment-683424375).
4. When importing in or from cluster version remember to use correct [URL format](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format)
and specify `accountID` param.

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

The flag `--vm-concurrency` controls the number of concurrent workers that process the input from InfluxDB query results.
Please note that each import request can load up to a single vCPU core on VictoriaMetrics. So try to set it according
to allocated CPU resources of your VictoriMetrics installation.

The flag `--vm-batch-size` controls max amount of samples collected before sending the import request.
For example, if  `--influx-chunk-size=500` and `--vm-batch-size=2000` then importer will process not more 
than 4 chunks before sending the request. 

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

* `--vm-round-digits` flag for rounding processed values to the given number of decimal digits after the point.
  For example, `--vm-round-digits=2` would round `1.2345` to `1.23`. By default the rounding is disabled.

* `--vm-significant-figures` flag for limiting the number of significant figures in processed values. It takes no effect if set
  to 0 (by default), but set `--vm-significant-figures=5` and `102.342305` will be rounded to `102.34`.

The most common case for using these flags is to improve data compression for time series storing aggregation
results such as `average`, `rate`, etc.

### Adding extra labels

 `vmctl` allows to add extra labels to all imported series. It can be achived with flag `--vm-extra-label label=value`. 
 If multiple labels needs to be added, set flag for each label, for example, `--vm-extra-label label1=value1 --vm-extra-label label2=value2`.
 If timeseries already have label, that must be added with `--vm-extra-label` flag, flag has priority and will override label value from timeseries.

### Rate limiting

Limiting the rate of data transfer could help to reduce pressure on disk or on destination database.
The rate limit may be set in bytes-per-second via `--vm-rate-limit` flag.

Please note, you can also use [vmagent](https://docs.victoriametrics.com/vmagent.html) 
as a proxy between `vmctl` and destination with `-remoteWrite.rateLimit` flag enabled.


## How to build

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) - `vmctl` is located in `vmutils-*` archives there.


### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make vmctl` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmctl-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmctl`. It builds `victoriametrics/vmctl:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmctl`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package-vmctl
```

### ARM build

ARM build may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

#### Development ARM build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make vmctl-arm` or `make vmctl-arm64` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-arm` or `vmctl-arm64` binary respectively and puts it into the `bin` folder.

#### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmctl-arm-prod` or `make vmctl-arm64-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmctl-arm-prod` or `vmctl-arm64-prod` binary respectively and puts it into the `bin` folder.
