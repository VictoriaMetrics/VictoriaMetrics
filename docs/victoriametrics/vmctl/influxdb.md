---
title: InfluxDB
weight: 2
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-influxdb"
    weight: 2
---
`vmctl` can migrate historical data from InfluxDB (v1) to VictoriaMetrics. See `./vmctl influx --help` for details and 
full list of flags. Also see [migrating data from InfluxDB to VictoriaMetrics](https://docs.victoriametrics.com/guides/migrate-from-influx/) article.

To start migration, specify the InfluxDB address `--influx-addr`, database `--influx-database` and VictoriaMetrics address `--vm-addr`:
```sh
./vmctl influx --influx-addr=http://<influx-addr>:8086 \
  --influx-database=benchmark \
  --vm-addr=http://<victoriametrics-addr>:8428
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

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

## Data mapping

vmctl modifies InfluxDB data by using the following rules:
- Field values are mapped to time series values.
- Tags are mapped to [labels](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels) format as-is.
- `influx-database` is mapped into `db` label value, unless `db` tag already exists in the InfluxDB line.
    To skip this mapping, enable flag `influx-skip-database-label`.
- Field names are mapped to time series names prefixed with `{measurement}{separator}` value,
  where `{separator}` equals to `_` by default. It can be changed with `--influx-measurement-field-separator` cmd-line flag.

For example, the following InfluxDB line:
```text
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following format in VictoriaMetrics:
```text
foo_field1{tag1="value1", tag2="value2"} 12
foo_field2{tag1="value1", tag2="value2"} 40
```

See more about [data model differences](https://docs.victoriametrics.com/guides/migrate-from-influx/#data-model-differences) 
between VictoriaMetrics and InfluxDB.

## Filtering

Additional filtering for exported data from InfluxDB can be applied via `--influx-filter-series` flag. For example:
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

To filter by time specify the following flags:
- `--influx-filter-time-start`
- `--influx-filter-time-end`

Here's an example of importing timeseries for one day only:
```sh
./vmctl influx --influx-database benchmark \
  --influx-filter-time-start "2020-01-01T10:07:00Z" \
  --influx-filter-time-end "2020-01-01T15:07:00Z"
 ```

See more about [time filtering in InfluxDB](https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#filter-meta-queries-by-time).

## InfluxDB v2

Migrating data from InfluxDB v2.x is not supported yet ([#32](https://github.com/VictoriaMetrics/vmctl/issues/32)).
You may find useful a 3rd party solution for this - <https://github.com/jonppe/influx_to_victoriametrics>.

## Configuration

In Influx mode, vmctl fetches data from the InfluxDB by executing read queries. The speed of migration is mostly limited 
by capabilities of InfluxDB to respond to these queries fast enough. vmctl executes one read request at a time by default.
Increase `--influx-concurrency` to execute more read requests concurrently. But make sure to not overwhelm InfluxDB
during migration.

The flag `--influx-chunk-size` controls the max amount of datapoints to return in single chunk from fetch requests.
Please see more details [here](https://docs.influxdata.com/influxdb/v1.7/guides/querying_data/#chunking).
The chunk size is used to control InfluxDB memory usage, so it won't OOM on processing large timeseries with
billions of datapoints.

See general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).

See `./vmctl influx --help` for details and full list of flags:

{{% content "vmctl_influx_flags.md" %}}