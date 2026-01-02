---
title: InfluxDB
weight: 5
menu:
  docs:
    parent: "integrations-vm"
    weight: 5
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive inserts via InfluxDB line protocol.

Additional resources:
* [How to Migrate from InfluxDB to VictoriaMetrics](https://docs.victoriametrics.com/guides/migrate-from-influx/)
* [Data model differences](https://docs.victoriametrics.com/guides/migrate-from-influx/#data-model-differences)

See full list of InfluxDB-related configuration flags by running:
```sh
/path/to/victoria-metrics-prod --help | grep influx
```

## InfluxDB-compatible agents such as [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/)

Use `http://<victoriametrics-addr>:8428` URL instead of InfluxDB URL in agent configs:
```toml
[[outputs.influxdb]]
  urls = ["http://<victoriametrics-addr>:8428"]
```
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For cluster version use vminsert address:
```
http://<vminsert-addr>:8480/insert/<tenant>/influx
```
_Replace `<vminsert-addr>` with the hostname or IP address of vminsert service._

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

In case of [`http`](https://github.com/influxdata/telegraf/blob/master/plugins/outputs/http) output:
```toml
[[outputs.http]]
  url = "http://<victoriametrics-addr>:8428/influx/write"
  data_format = "influx"
  non_retryable_statuscodes = [400]
```

Some plugins for Telegraf such as [fluentd](https://github.com/fangli/fluent-plugin-influxdb), [Juniper/open-nti](https://github.com/Juniper/open-nti)
or [Juniper/jitmon](https://github.com/Juniper/jtimon) send `SHOW DATABASES` query to `/query` and expect a particular database name in the response.
Comma-separated list of expected databases can be passed to VictoriaMetrics via `-influx.databaseNames` command-line flag.

## InfluxDB v2 format

VictoriaMetrics exposes endpoint for InfluxDB v2 HTTP API at `/influx/api/v2/write` and `/api/v2/write`.

Here's an example writing data with `curl`:
```sh
curl --data-binary 'measurement1,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<victoriametrics-addr>:8428/api/v2/write'
```

And to write multiple lines of data at once, prepare a file (e.g., `influx.data`) with your data:
```text
measurement2,tag1=value1,tag2=value2 field1=456,field2=4.56
measurement3,tag1=value1,tag2=value2 field1=789,field2=7.89
```

And execute this command to import the data:
```sh
curl -X POST 'http://<victoriametrics-addr>:8428/api/v2/write' --data-binary @influx.data
```

The `/api/v1/export` endpoint should return the following response:
```json
{"metric":{"__name__":"measurement1_field1","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1766983684142]}
{"metric":{"__name__":"measurement1_field2","tag1":"value1","tag2":"value2"},"values":[1.23],"timestamps":[1766983684142]}
{"metric":{"__name__":"measurement2_field1","tag1":"value1","tag2":"value2"},"values":[456],"timestamps":[1767012583021]}
{"metric":{"__name__":"measurement2_field2","tag1":"value1","tag2":"value2"},"values":[4.56],"timestamps":[1767012583021]}
{"metric":{"__name__":"measurement3_field1","tag1":"value1","tag2":"value2"},"values":[789],"timestamps":[1767012583021]}
{"metric":{"__name__":"measurement3_field2","tag1":"value1","tag2":"value2"},"values":[7.89],"timestamps":[1767012583021]}
```

## Data transformations

VictoriaMetrics performs the following transformations to the ingested InfluxDB data:
* [db query arg](https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint) is mapped into `db`
  [label](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels) value unless `db` tag exists in the InfluxDB line.
  The `db` label name can be overridden via `-influxDBLabel` command-line flag. If more strict data isolation is required,
  read more about multi-tenancy [here](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#multi-tenancy).
* Field names are mapped to time series names prefixed with `{measurement}{separator}` value, where `{separator}` equals to `_` by default.
  It can be changed with `-influxMeasurementFieldSeparator` command-line flag. See also `-influxSkipSingleField` command-line flag.
  If `{measurement}` is empty or if `-influxSkipMeasurement` command-line flag is set, then time series names correspond to field names.
* Field values are mapped to time series values.
* Non-numeric field values are converted to 0.
* Tags are mapped to Prometheus labels as-is.
* If `-usePromCompatibleNaming` command-line flag is set, then all the metric names and label names
  are normalized to [Prometheus-compatible naming](https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels) by replacing unsupported chars with `_`.
  For example, `foo.bar-baz/1` metric name or label name is substituted with `foo_bar_baz_1`.

For example, the following InfluxDB line:
```influxtextmetric
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following Prometheus data points:
```promtextmetric
foo_field1{tag1="value1", tag2="value2"} 12
foo_field2{tag1="value1", tag2="value2"} 40
```

Example for writing data with [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/)
to local VictoriaMetrics using `curl`:
```sh
curl -d 'measurement,tag1=value1,tag2=value2 field1=123,field2=1.23' -X POST 'http://<victoriametrics-addr>:8428/write'
```

An arbitrary number of lines delimited by '\n' (aka newline char) can be sent in a single request.
After that the data may be read via [/api/v1/export](https://docs.victoriametrics.com/victoriametrics/#how-to-export-data-in-json-line-format) endpoint:
```sh
curl -G 'http://<victoriametrics-addr>:8428/api/v1/export' -d 'match={__name__=~"measurement_.*"}'
```

The `/api/v1/export` endpoint should return the following response:
```json
{"metric":{"__name__":"measurement_field1","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560272508147]}
{"metric":{"__name__":"measurement_field2","tag1":"value1","tag2":"value2"},"values":[1.23],"timestamps":[1560272508147]}
```

InfluxDB line protocol expects [timestamps in *nanoseconds* by default](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/#timestamp),
but VictoriaMetrics stores them with *milliseconds* precision. It is allowed to ingest timestamps with seconds,
microseconds or nanoseconds precision - VictoriaMetrics will automatically convert them to milliseconds.

Extra labels may be added to all the written time series by passing `extra_label=name=value` query args.
For example, `/write?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.

## Tuning

The maximum request size for Influx HTTP endpoints is limited by -influx.maxRequestSize (default: 64MB).

For better ingestion speed and lower memory use, enable stream processing.
You can do this in two ways:
* Add `Stream-Mode: 1` HTTP header in your request.
* Set the `-influx.forceStreamMode` flag to enable it for all requests.

In stream mode:
* Data is processed one line at a time (see `-influx.maxLineSize`).
* Invalid lines are skipped and logged.
* Valid lines are ingested immediately, even if the client disconnects partway through.

You can also enable InfluxDB line protocol over TCP or UDP with `-influxListenAddr`.
Just send plain Influx lines to the specified address.
_Note: TCP and UDP receivers always use streaming mode._
