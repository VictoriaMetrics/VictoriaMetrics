# VictoriaLogs

VictoriaLogs is log management and log analytics system from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/).

It provides the following key features:

- VictoriaLogs can accept logs from popular log collectors, which support
  [ElasticSearch data ingestion format](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html). See [these docs](#data-ingestion).
  [Grafana Loki data ingestion format](https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki) will be supported in the near future -
  see [the Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html).
- VictoriaLogs is much easier to setup and operate comparing to ElasticSearch and Grafana Loki. See [these docs](#operation).
- VictoriaLogs provides easy yet powerful query language with full-text search capabilities across
  all the [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) -
  see [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html).
- VictoriaLogs can be seamlessly combined with good old Unix tools for log analysis such as `grep`, `less`, `sort`, `jq`, etc.
  See [these docs](#querying-via-command-line) for details.
- VictoriaLogs capacity and performance scales lineraly with the available resources (CPU, RAM, disk IO, disk space).
  It runs smoothly on both Raspberry PI and a beefy server with hundreds of CPU cores and terabytes of RAM.
- VictoriaLogs can handle much bigger data volumes than ElasticSearch and Grafana Loki when running on comparable hardware.
  A single-node VictoriaLogs instance can substitute large ElasticSearch cluster.

## Operation

### How to run VictoriaLogs

Checkout VictoriaLogs source code. It is located in the VictoriaMetrics repository:

```bash
git clone https://github.com/VictoriaMetrics/VictoriaMetrics
cd VictoriaMetrics
```

Then build VictoriaLogs. The build command requires [Go 1.20](https://golang.org/doc/install).

```bash
make victoria-logs
```

Then run the built binary:

```bash
bin/victoria-logs
```

VictoriaLogs is ready to [receive logs](#data-ingestion) and [query logs](#querying) at the TCP port `9428` now!
It has no any external dependencies, so it may run in various environments without additional setup and configuration.
VictoriaLogs automatically adapts to the available CPU and RAM resources. It also automatically setups and creates
the needed indexes during [data ingestion](#data-ingestion).

It is possible to change the TCP port via `-httpListenAddr` command-line flag. For example, the following command
starts VictoriaLogs, which accepts incoming requests at port `9200` (aka ElasticSearch HTTP API port):

```bash
/path/to/victoria-logs -httpListenAddr=:9200
```

VictoriaLogs stores the ingested data to the `victoria-logs-data` directory by default. The directory can be changed
via `-storageDataPath` command-line flag. See [these docs](#storage) for details.

By default VictoriaLogs stores log entries with timestamps in the time range `[now-7d, now]`, while dropping logs outside the given time range.
E.g. it uses the retention of 7 days. Read [these docs](#retention) on how to control the retention for the [ingested](#data-ingestion) logs.

It is recommended setting up monitoring of VictoriaLogs according to [these docs](#monitoring).

### Data ingestion

VictoriaLogs supports the following data ingestion techniques:

- Via [Filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/filebeat-overview.html). See [these docs](#filebeat-setup).
- Via [Logstash](https://www.elastic.co/guide/en/logstash/current/introduction.html). See [these docs](#logstash-setup).

The ingested log entries can be queried according to [these docs](#querying).

#### Data ingestion troubleshooting

VictoriaLogs provides the following command-line flags, which can help debugging data ingestion issues:

- `-logNewStreams` - if this flag is passed to VictoriaLogs, then it logs all the newly
  registered [log streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
  This may help debugging [high cardinality issues](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#high-cardinality).
- `-logIngestedRows` - if this flag is passed to VictoriaLogs, then it logs all the ingested
  [log entries](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model).

VictoriaLogs exposes various [metrics](#monitoring), which may help debugging data ingestion issues:

- `vl_rows_ingested_total` - the number of ingested [log entries](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
  since the last VictoriaLogs restart. If this number icreases over time, then logs are successfully ingested into VictoriaLogs.
  The ingested logs can be inspected in logs by passing `-logIngestedRows` command-line flag to VictoriaLogs.
- `vl_streams_created_total` - the number of created [log streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields)
  since the last VictoriaLogs restart. If this metric grows rapidly during extended periods of time, then this may lead
  to [high cardinality issues](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#high-cardinality).
  The newly created log streams can be inspected in logs by passing `-logNewStreams` command-line flag to VictoriaLogs.

#### Filebeat setup

Specify [`output.elasicsearch`](https://www.elastic.co/guide/en/beats/filebeat/current/elasticsearch-output.html) section in the `filebeat.yml`
for sending the collected logs to VictoriaLogs:

```yml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.hostname,log.file.path"
```

Substitute the `localhost:9428` address inside `hosts` section with the real TCP address of VictoriaLogs.

The `_msg_field` parameter must contain the field name with the log message generated by Filebeat. This is usually `message` field.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) for details.

The `_time_field` parameter must contain the field name with the log timestamp generated by Filebeat. This is usually `@timestamp` field.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field) for details.

It is recommended specifying comma-separated list of field names, which uniquely identify every log stream collected by Filebeat, in the `_stream_fields` parameter.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) for details.

If some [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) aren't needed,
then VictoriaLogs can be instructed to ignore them during data ingestion - just pass `ignore_fields` parameter with comma-separated list of fields to ignore.
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```yml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
    ignore_fields: "log.offset,event.original"
```

When Filebeat ingests logs into VictoriaLogs at a high rate, then it may be needed to tune `worker` and `bulk_max_size` options.
For example, the following config is optimized for higher than usual ingestion rate:

```yml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
  worker: 8
  bulk_max_size: 1000
```

If the Filebeat sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `compression_level` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```yml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
  compression_level: 1
```

By default the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `headers` at `output.elasticsearch` section.
For example, the following `filebeat.yml` config instructs Filebeat to store the data to `(AccountID=12, ProjectID=34)` tenant:

```yml
output.elasticsearch:
  hosts: ["http://localhost:9428/insert/elasticsearch/"]
  headers:
    AccountID: 12
    ProjectID: 34
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "host.name,log.file.path"
```

The ingested log entries can be queried according to [these docs](#querying).

See also [data ingestion troubleshooting](#data-ingestion-trobuleshooting) docs.

#### Logstash setup

Specify [`output.elasticsearch`](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html) section in the `logstash.conf` file
for sending the collected logs to VictoriaLogs:

```conf
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.name,process.name"
    }
  }
}
```

Substitute `localhost:9428` address inside `hosts` with the real TCP address of VictoriaLogs.

The `_msg_field` parameter must contain the field name with the log message generated by Logstash. This is usually `message` field.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) for details.

The `_time_field` parameter must contain the field name with the log timestamp generated by Logstash. This is usually `@timestamp` field.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field) for details.

It is recommended specifying comma-separated list of field names, which uniquely identify every log stream collected by Logstash, in the `_stream_fields` parameter.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) for details.

If some [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) aren't needed,
then VictoriaLogs can be instructed to ignore them during data ingestion - just pass `ignore_fields` parameter with comma-separated list of fields to ignore.
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```conf
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
        "ignore_fields" => "log.offset,event.original"
    }
  }
}
```

If the Logstash sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `http_compression: true` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```conf
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
    }
    http_compression => true
  }
}
```

By default the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `custom_headers` at `output.elasticsearch` section.
For example, the following `logstash.conf` config instructs Logstash to store the data to `(AccountID=12, ProjectID=34)` tenant:

```conf
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    custom_headers => {
        "AccountID" => "1"
        "ProjectID" => "2"
    }
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
    }
  }
}
```

The ingested log entries can be queried according to [these docs](#querying).

See also [data ingestion troubleshooting](#data-ingestion-trobuleshooting) docs.

### Querying

VictoriaLogs can be queried at the `/select/logsql/query` endpoint. The [LogsQL](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html)
query must be passed via `query` argument. For example, the following query returns all the log entries with the `error` word:

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

The `query` argument can be passed either in the request url itself (aka HTTP GET request) or via request body
with the `x-www-form-urlencoded` encoding (aka HTTP POST request). The HTTP POST is useful for sending long queries
when they do not fit the maximum url length of the used clients and proxies.

See [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html) for details on what can be passed to the `query` arg.
The `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools.

The `/select/logsql/query` endpoint returns [a stream of JSON lines](https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON),
where each line contains JSON-encoded log entry in the form `{field1="value1",...,fieldN="valueN"}`.
Example response:

```
{"_msg":"error: disconnect from 19.54.37.22: Auth fail [preauth]","_stream":"{}","_time":"2023-01-01T13:32:13Z"}
{"_msg":"some other error","_stream":"{}","_time":"2023-01-01T13:32:15Z"}
```

The matching lines are sent to the response stream as soon as they are found in VictoriaLogs storage.
This means that the returned response may contain billions of lines for queries matching too many log entries.
The response can be interrupted at any time by closing the connection to VictoriaLogs server.
This allows post-processing the returned lines at the client side with the usual Unix commands such as `grep`, `jq`, `less`, `head`, etc.
See [these docs](#querying-via-command-line) for more details.

The returned lines aren't sorted by default, since sorting disables the ability to send matching log entries to response stream as soon as they are found.
Query results can be sorted either at VictoriaLogs side according [to these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#sorting)
or at client side with the usual `sort` command according to [these docs](#querying-via-command-line).

By default the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#multitenancy) is queried.
If you need querying other tenant, then specify the needed tenant via http request headers. For example, the following query searches
for log messages at `(AccountID=12, ProjectID=34)` tenant:

```bash
curl http://localhost:9428/select/logsql/query -H 'AccountID: 12' -H 'ProjectID: 34' -d 'query=error'
```

The number of requests to `/select/logsql/query` can be [monitored](#monitoring) with `vl_http_requests_total{path="/select/logsql/query"}` metric.

#### Querying via command-line

VictoriaLogs provides good integration with `curl` and other command-line tools because of the following features:

- VictoriaLogs sends the matching log entries to the response stream as soon as they are found.
  This allows forwarding the response stream to arbitrary [Unix pipes](https://en.wikipedia.org/wiki/Pipeline_(Unix)).
- VictoriaLogs automatically adjusts query execution speed to the speed of the client, which reads the response stream.
  For example, if the response stream is piped to `less` command, then the query is suspended
  until the `less` command reads the next block from the response stream.
- VictoriaLogs automatically cancels query execution when the client closes the response stream.
  For example, if the query response is piped to `head` command, then VictoriaLogs stops executing the query
  when the `head` command closes the response stream.

These features allow executing queries at command-line interface, which potentially select billions of rows,
without the risk of high resource usage (CPU, RAM, disk IO) at VictoriaLogs server.

For example, the following query can return very big number of matching log entries (e.g. billions) if VictoriaLogs contains
many log messages with the `error` [word](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#word):

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error'
```

If the command returns "never-ending" response, then just press `ctrl+C` at any time in order to cancel the query.
VictoriaLogs notices that the response stream is closed, so it cancels the query and instantly stops consuming CPU, RAM and disk IO for this query.

Then just use `head` command for investigating the returned log messages and narrowing down the query:

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error' | head -10
```

The `head -10` command reads only the first 10 log messages from the response and then closes the response stream.
This automatically cancels the query at VictoriaLogs side, so it stops consuming CPU, RAM and disk IO resources.

Sometimes it may be more convenient to use `less` command instead of `head` during the investigation of the returned response:

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error' | less
```

The `less` command reads the response stream on demand, when the user scrolls down the output.
VictoriaLogs suspends query execution when `less` stops reading the response stream.
It doesn't consume CPU and disk IO resources during this time. It resumes query execution
when the `less` continues reading the response stream.

Suppose that the initial investigation of the returned query results helped determining that the needed log messages contain
`cannot open file` [phrase](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#phrase-filter).
Then the query can be narrowed down to `error AND "cannot open file"`
(see [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#logical-filter) about `AND` operator).
Then run the updated command in order to continue the investigation:

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error AND "cannot open file"' | head
```

Note that the `query` arg must be properly encoded with [percent encoding](https://en.wikipedia.org/wiki/URL_encoding) when passing it to `curl`
or similar tools.

The `pipe the query to "head" or "less" -> investigate the results -> refine the query` iteration
can be repeated multiple times until the needed log messages are found.

The returned VictoriaLogs query response can be post-processed with any combination of Unix commands,
which are usually used for log analysis - `grep`, `jq`, `awk`, `sort`, `uniq`, `wc`, etc.

For example, the following command uses `wc -l` Unix command for counting the number of log messages
with the `error` [word](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#word)
received from [streams](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) with `app="nginx"` field
during the last 5 minutes:

```bash
curl http://localhost:9428/select/logsql/query -d 'query=_stream:{app="nginx"} AND _time:[now-5m,now] AND error' | wc -l
```

See [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stream-filter) about `_stream` filter,
[these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#time-filter) about `_time` filter
and [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#logical-filter) about `AND` operator.

The following example shows how to sort query results by the [`_time` field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field):

```bash
curl http://localhost:9428/select/logsql/query -d 'query=error' | jq -r '._time + " " + ._msg' | sort | less
```

This command uses `jq` for extracting [`_time`](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#time-field)
and [`_msg`](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#message-field) fields from the returned results,
and piping them to `sort` command.

Note that the `sort` command needs to read all the response stream before returning the sorted results. So the command above
can take non-trivial amounts of time if the `query` returns too many results. The solution is to narrow down the `query`
before sorting the results. See [these tips](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#performance-tips)
on how to narrow down query results.

The following example calculates stats on the number of log messages received during the last 5 minutes
grouped by `log.level` [field](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model):

```bash
curl http://localhost:9428/select/logsql/query -d 'query=_time:[now-5m,now] log.level:*' | jq -r '."log.level"' | sort | uniq -c 
```

The query selects all the log messages with non-empty `log.level` field via ["any value" filter](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#any-value-filter),
then pipes them to `jq` command, which extracts the `log.level` field value from the returned JSON stream, then the extracted `log.level` values
are sorted with `sort` command and, finally, they are passed to `uniq -c` command for calculating the needed stats.

See also:

- [Key concepts](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html).
- [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html).


### Monitoring

VictoriaLogs exposes internal metrics in Prometheus exposition format at `http://localhost:9428/metrics` page.
It is recommended to set up monitoring of these metrics via VictoriaMetrics
(see [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)),
vmagent (see [these docs](https://docs.victoriametrics.com/vmagent.html#how-to-collect-metrics-in-prometheus-format)) or via Prometheus.

VictoriaLogs emits own logs to stdout. It is recommended investigating these logs during troubleshooting.


### Retention

By default VictoriaLogs stores log entries with timestamps in the time range `[now-7d, now]`, while dropping logs outside the given time range.
E.g. it uses the retention of 7 days. The retention can be configured with `-retentionPeriod` command-line flag.
This flag accepts values starting from `1d` (one day) up to `100y` (100 years). See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs with the retention of 8 weeks:

```bash
/path/to/victoria-logs -retentionPeriod=8w
```

VictoriaLogs stores the [ingested](#data-ingestion) logs in per-day partition directories. It automatically drops partition directories
outside the configured retention.

VictoriaLogs automatically drops logs at [data ingestion](#data-ingestion) stage if they have timestamps outside the configured retention.
A sample of dropped logs is logged with `WARN` message in order to simplify troubleshooting.
The `vlinsert_rows_dropped_total` [metric](#monitoring) is incremented each time an ingested log entry is dropped because of timestamp outside the retention.
It is recommended setting up the following alerting rule at [vmalert](https://docs.victoriametrics.com/vmalert.html) in order to be notified
when logs with wrong timestamps are ingested into VictoriaLogs:

```metricsql
rate(vlinsert_rows_dropped_total[5m]) > 0
```

By default VictoriaLogs doesn't accept log entries with timestamps bigger than `now+2d`, e.g. 2 days in the future.
If you need accepting logs with bigger timestamps, then specify the desired "future retention" via `-futureRetention` command-line flag.
This flag accepts values starting from `1d`. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs, which accepts logs with timestamps up to a year in the future:

```bash
/path/to/victoria-logs -futureRetention=1y
```

### Storage

VictoriaLogs stores all its data in a single directory - `victoria-logs-data`. The path to the directory can be changed via `-storageDataPath` command-line flag.
For example, the following command starts VictoriaLogs, which stores the data at `/var/lib/victoria-logs`:

```bash
/path/to/victoria-logs -storageDataPath=/var/lib/victoria-logs
```

VictoriaLogs automatically creates the `-storageDataPath` directory on the first run if it is missing.
