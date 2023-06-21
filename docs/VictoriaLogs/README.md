# VictoriaLogs

VictoriaLogs is log management and log analytics system from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/).

It provides the following key features:

- VictoriaLogs can accept logs from popular log collectors. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
- VictoriaLogs is much easier to setup and operate comparing to ElasticSearch and Grafana Loki.
  See [these docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html).
- VictoriaLogs provides easy yet powerful query language with full-text search capabilities across
  all the [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) -
  see [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html).
- VictoriaLogs can be seamlessly combined with good old Unix tools for log analysis such as `grep`, `less`, `sort`, `jq`, etc.
  See [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line) for details.
- VictoriaLogs capacity and performance scales lineraly with the available resources (CPU, RAM, disk IO, disk space).
  It runs smoothly on both Raspberry PI and a server with hundreds of CPU cores and terabytes of RAM.
- VictoriaLogs can handle much bigger data volumes than ElasticSearch and Grafana Loki when running on comparable hardware.
- VictoriaLogs supports multitenancy - see [these docs](#multitenancy).
- VictoriaLogs supports out of order logs' ingestion aka backfilling.

VictoriaLogs is at Preview stage now. It is ready for evaluation in production and verifying claims given above.
It isn't recommended migrating from existing logging solutions to VictoriaLogs Preview in general case yet.
See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

If you have questions about VictoriaLogs, then feel free asking them at [VictoriaMetrics community Slack chat](https://slack.victoriametrics.com/).

See [Quick start docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html) for start working with VictoriaLogs.

## Monitoring

VictoriaLogs exposes internal metrics in Prometheus exposition format at `http://localhost:9428/metrics` page.
It is recommended to set up monitoring of these metrics via VictoriaMetrics
(see [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)),
vmagent (see [these docs](https://docs.victoriametrics.com/vmagent.html#how-to-collect-metrics-in-prometheus-format)) or via Prometheus.

VictoriaLogs emits own logs to stdout. It is recommended investigating these logs during troubleshooting.

## Retention

By default VictoriaLogs stores log entries with timestamps in the time range `[now-7d, now]`, while dropping logs outside the given time range.
E.g. it uses the retention of 7 days. The retention can be configured with `-retentionPeriod` command-line flag.
This flag accepts values starting from `1d` (one day) up to `100y` (100 years). See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs with the retention of 8 weeks:

```bash
/path/to/victoria-logs -retentionPeriod=8w
```

VictoriaLogs stores the [ingested](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) logs in per-day partition directories.
It automatically drops partition directories outside the configured retention.

VictoriaLogs automatically drops logs at [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) stage
if they have timestamps outside the configured retention. A sample of dropped logs is logged with `WARN` message in order to simplify troubleshooting.
The `vl_rows_dropped_total` [metric](#monitoring) is incremented each time an ingested log entry is dropped because of timestamp outside the retention.
It is recommended setting up the following alerting rule at [vmalert](https://docs.victoriametrics.com/vmalert.html) in order to be notified
when logs with wrong timestamps are ingested into VictoriaLogs:

```metricsql
rate(vl_rows_dropped_total[5m]) > 0
```

By default VictoriaLogs doesn't accept log entries with timestamps bigger than `now+2d`, e.g. 2 days in the future.
If you need accepting logs with bigger timestamps, then specify the desired "future retention" via `-futureRetention` command-line flag.
This flag accepts values starting from `1d`. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs, which accepts logs with timestamps up to a year in the future:

```bash
/path/to/victoria-logs -futureRetention=1y
```

## Storage

VictoriaLogs stores all its data in a single directory - `victoria-logs-data`. The path to the directory can be changed via `-storageDataPath` command-line flag.
For example, the following command starts VictoriaLogs, which stores the data at `/var/lib/victoria-logs`:

```bash
/path/to/victoria-logs -storageDataPath=/var/lib/victoria-logs
```

VictoriaLogs automatically creates the `-storageDataPath` directory on the first run if it is missing.

## Multitenancy

VictoriaLogs supports multitenancy. A tenant is identified by `(AccountID, ProjectID)` pair, where `AccountID` and `ProjectID` are arbitrary 32-bit unsigned integeres.
The `AccountID` and `ProjectID` fields can be set during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
and [querying](https://docs.victoriametrics.com/VictoriaLogs/querying/) via `AccountID` and `ProjectID` request headers.

If `AccountID` and/or `ProjectID` request headers aren't set, then the default `0` value is used.

VictoriaLogs has very low overhead for per-tenant management, so it is OK to have thousands of tenants in a single VictoriaLogs instance.

VictoriaLogs doesn't perform per-tenant authorization. Use [vmauth](https://docs.victoriametrics.com/vmauth.html) or similar tools for per-tenant authorization.

## Benchmarks

Here is a [benchmark suite](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/logs-benchmark) that covers ElasticSearch and VictoriaLogs.

However, we encourage you to run benchmarks on your own. Please share the results or feedback with us by just dropping the line on any of our [Community channels](https://docs.victoriametrics.com/#community-and-contributions).
