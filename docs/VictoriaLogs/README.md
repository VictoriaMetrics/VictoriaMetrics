---
title: VictoriaLogs
weight: 0
aliases:
- /VictoriaLogs/
- /VictoriaLogs/index.html 
---
# VictoriaLogs

VictoriaLogs is [open source](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/victoria-logs) user-friendly database for logs
from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/).

VictoriaLogs provides the following key features:

- VictoriaLogs can accept logs from popular log collectors. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
- VictoriaLogs is much easier to set up and operate compared to Elasticsearch and Grafana Loki.
  See [these docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html).
- VictoriaLogs provides easy yet powerful query language with full-text search capabilities across
  all the [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) -
  see [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html).
- VictoriaLogs can be seamlessly combined with good old Unix tools for log analysis such as `grep`, `less`, `sort`, `jq`, etc.
  See [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line) for details.
- VictoriaLogs capacity and performance scales linearly with the available resources (CPU, RAM, disk IO, disk space).
  It runs smoothly on both Raspberry PI and a server with hundreds of CPU cores and terabytes of RAM.
- VictoriaLogs can handle up to 30x bigger data volumes than Elasticsearch and Grafana Loki when running on the same hardware.
  See [these docs](#benchmarks).
- VictoriaLogs supports fast full-text search over high-cardinality [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
  such as `trace_id`, `user_id` and `ip`.
- VictoriaLogs supports multitenancy - see [these docs](#multitenancy).
- VictoriaLogs supports out-of-order logs' ingestion aka backfilling.
- VictoriaLogs provides a simple web UI for querying logs - see [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/#web-ui).

VictoriaLogs is at the Preview stage now. It is ready for evaluation in production and verifying the claims given above.
It isn't recommended to migrate from existing logging solutions to VictoriaLogs Preview in general cases yet.
See the [Roadmap](https://docs.victoriametrics.com/VictoriaLogs/Roadmap.html) for details.

If you have questions about VictoriaLogs, then read [this FAQ](https://docs.victoriametrics.com/VictoriaLogs/FAQ.html).
Also feel free asking any questions at [VictoriaMetrics community Slack chat](https://victoriametrics.slack.com/), 
you can join it via [Slack Inviter](https://slack.victoriametrics.com/).

See [Quick start docs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html) for start working with VictoriaLogs.

## Monitoring

VictoriaLogs exposes internal metrics in Prometheus exposition format at `http://localhost:9428/metrics` page.
It is recommended to set up monitoring of these metrics via VictoriaMetrics
(see [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)),
vmagent (see [these docs](https://docs.victoriametrics.com/vmagent.html#how-to-collect-metrics-in-prometheus-format)) or via Prometheus.

VictoriaLogs emits its own logs to stdout. It is recommended to investigate these logs during troubleshooting.

## Upgrading

It is safe upgrading VictoriaLogs to new versions unless [release notes](https://docs.victoriametrics.com/VictoriaLogs/CHANGELOG.html) say otherwise.
It is safe to skip multiple versions during the upgrade unless [release notes](https://docs.victoriametrics.com/VictoriaLogs/CHANGELOG.html) say otherwise.
It is recommended to perform regular upgrades to the latest version, since it may contain important bug fixes, performance optimizations or new features.

It is also safe to downgrade to older versions unless [release notes](https://docs.victoriametrics.com/VictoriaLogs/CHANGELOG.html) say otherwise.

The following steps must be performed during the upgrade / downgrade procedure:

* Send `SIGINT` signal to VictoriaLogs process in order to gracefully stop it.
  See [how to send signals to processes](https://stackoverflow.com/questions/33239959/send-signal-to-process-from-command-line).
* Wait until the process stops. This can take a few seconds.
* Start the upgraded VictoriaLogs.

## Retention

By default VictoriaLogs stores log entries with timestamps in the time range `[now-7d, now]`, while dropping logs outside the given time range.
E.g. it uses the retention of 7 days. The retention can be configured with `-retentionPeriod` command-line flag.
This flag accepts values starting from `1d` (one day) up to `100y` (100 years). See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs with the retention of 8 weeks:

```sh
/path/to/victoria-logs -retentionPeriod=8w
```

VictoriaLogs stores the [ingested](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) logs in per-day partition directories.
It automatically drops partition directories outside the configured retention.

VictoriaLogs automatically drops logs at [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) stage
if they have timestamps outside the configured retention. A sample of dropped logs is logged with `WARN` message in order to simplify troubleshooting.
The `vl_rows_dropped_total` [metric](#monitoring) is incremented each time an ingested log entry is dropped because of timestamp outside the retention.
It is recommended to set up the following alerting rule at [vmalert](https://docs.victoriametrics.com/vmalert.html) in order to be notified
when logs with wrong timestamps are ingested into VictoriaLogs:

```metricsql
rate(vl_rows_dropped_total[5m]) > 0
```

By default, VictoriaLogs doesn't accept log entries with timestamps bigger than `now+2d`, e.g. 2 days in the future.
If you need accepting logs with bigger timestamps, then specify the desired "future retention" via `-futureRetention` command-line flag.
This flag accepts values starting from `1d`. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations)
for the supported duration formats.

For example, the following command starts VictoriaLogs, which accepts logs with timestamps up to a year in the future:

```sh
/path/to/victoria-logs -futureRetention=1y
```

## Storage

VictoriaLogs stores all its data in a single directory - `victoria-logs-data`. The path to the directory can be changed via `-storageDataPath` command-line flag.
For example, the following command starts VictoriaLogs, which stores the data at `/var/lib/victoria-logs`:

```sh
/path/to/victoria-logs -storageDataPath=/var/lib/victoria-logs
```

VictoriaLogs automatically creates the `-storageDataPath` directory on the first run if it is missing.

## Multitenancy

VictoriaLogs supports multitenancy. A tenant is identified by `(AccountID, ProjectID)` pair, where `AccountID` and `ProjectID` are arbitrary 32-bit unsigned integers.
The `AccountID` and `ProjectID` fields can be set during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
and [querying](https://docs.victoriametrics.com/VictoriaLogs/querying/) via `AccountID` and `ProjectID` request headers.

If `AccountID` and/or `ProjectID` request headers aren't set, then the default `0` value is used.

VictoriaLogs has very low overhead for per-tenant management, so it is OK to have thousands of tenants in a single VictoriaLogs instance.

VictoriaLogs doesn't perform per-tenant authorization. Use [vmauth](https://docs.victoriametrics.com/vmauth.html) or similar tools for per-tenant authorization.

## Benchmarks

Here is a [benchmark suite](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/logs-benchmark) for comparing data ingestion performance
and resource usage between VictoriaLogs and Elasticsearch.

It is recommended [setting up VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/QuickStart.html) in production alongside the existing
log management systems and comparing resource usage + query performance between VictoriaLogs and your system such as Elasticsearch or Grafana Loki.

Please share benchmark results and ideas on how to improve benchmarks / VictoriaLogs
via [VictoriaMetrics community channels](https://docs.victoriametrics.com/#community-and-contributions).

## List of command-line flags

Pass `-help` to VictoriaLogs in order to see the list of supported command-line flags with their description:

```
  -cacheExpireDuration duration
    	Items are removed from in-memory caches after they aren't accessed for this duration. Lower values may reduce memory usage at the cost of higher CPU usage. See also -prevCacheRemovalPercent (default 30m0s)
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
    	Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -flagsAuthKey value
    	Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
        Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -futureRetention value
    	Log entries with timestamps bigger than now+futureRetention are rejected during data ingestion; see https://docs.victoriametrics.com/VictoriaLogs/#retention
    	The following optional suffixes are supported: h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 2d)
  -gogc int
    	GOGC to use. See https://tip.golang.org/doc/gc-guide (default 100)
  -http.connTimeout duration
        Incoming connections to -httpListenAddr are closed after the configured timeout. This may help evenly spreading load among a cluster of services behind TCP-level load balancer. Zero value disables closing of incoming connections (default 2m0s)
  -http.disableResponseCompression
    	Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
     Value for 'Content-Security-Policy' header
  -http.header.frameOptions string
     Value for 'X-Frame-Options' header
  -http.header.hsts string
     Value for 'Strict-Transport-Security' header
  -http.idleConnTimeout duration
    	Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
    	The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
    	An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
    	Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password value
    	Password for HTTP server's Basic Auth. The authentication is disabled if -httpAuth.username is empty
        Flag value can be read from the given file when using -httpAuth.password=file:///abs/path/to/file or -httpAuth.password=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -httpAuth.password=http://host/path or -httpAuth.password=https://host/path
  -httpAuth.username string
    	Username for HTTP server's Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
    	TCP address to listen for http connections. See also -httpListenAddr.useProxyProtocol (default ":9428")
  -httpListenAddr.useProxyProtocol
    	Whether to use proxy protocol for connections accepted at -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
  -inmemoryDataFlushInterval duration
    	The interval for guaranteed saving of in-memory data to disk. The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). Smaller intervals increase disk IO load. Minimum supported value is 1s (default 5s)
  -insert.maxFieldsPerLine int
    	The maximum number of log fields per line, which can be read by /insert/* handlers (default 1000)
  -insert.maxLineSizeBytes size
    	The maximum size of a single line, which can be read by /insert/* handlers
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 262144)
  -insert.maxQueueDuration duration
    	The maximum duration to wait in the queue when -maxConcurrentInserts concurrent insert requests are executed (default 1m0s)
  -internStringCacheExpireDuration duration
    	The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
    	Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
    	The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -logIngestedRows
    	Whether to log all the ingested log entries; this can be useful for debugging of data ingestion; see https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/ ; see also -logNewStreams
  -logNewStreams
    	Whether to log creation of new streams; this can be useful for debugging of high cardinality issues with log streams; see https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields ; see also -logIngestedRows
  -loggerDisableTimestamps
    	Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
    	Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit
  -loggerFormat string
    	Format for logs. Possible values: default, json (default "default")
  -loggerJSONFields string
    	Allows renaming fields in JSON formatted logs. Example: "ts:timestamp,msg:message" renames "ts" to "timestamp" and "msg" to "message". Supported fields: ts, level, caller, msg
  -loggerLevel string
    	Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
    	Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
    	Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxConcurrentInserts int
    	The maximum number of concurrent insert requests. The default value should work for most cases, since it minimizes memory usage. The default value can be increased when clients send data over slow networks. See also -insert.maxQueueDuration.
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metricsAuthKey value
    	Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
        Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -pprofAuthKey value
    	Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
        Flag value can be read from the given file when using -pprofAuthKey=file:///abs/path/to/file or -pprofAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -pprofAuthKey=http://host/path or -pprofAuthKey=https://host/path
  -prevCacheRemovalPercent float
    	Items in the previous caches are removed when the percent of requests it serves becomes lower than this value. Higher values reduce memory usage at the cost of higher CPU usage. See also -cacheExpireDuration (default 0.1)
  -pushmetrics.extraLabel array
    	Optional labels to add to metrics pushed to -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to -pushmetrics.url
    	Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.interval duration
    	Interval for pushing metrics to -pushmetrics.url (default 10s)
  -pushmetrics.url array
    	Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
    	Supports an array of values separated by comma or specified via multiple flags.
  -retentionPeriod value
    	Log entries with timestamps older than now-retentionPeriod are automatically deleted; log entries with timestamps outside the retention are also rejected during data ingestion; the minimum supported retention is 1d (one day); see https://docs.victoriametrics.com/VictoriaLogs/#retention
    	The following optional suffixes are supported: h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 7d)
  -search.maxConcurrentRequests int
    	The maximum number of concurrent search requests. It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. See also -search.maxQueueDuration (default 6)
  -search.maxQueryDuration duration
    	The maximum duration for query execution (default 30s)
  -search.maxQueueDuration duration
    	The maximum time the search request waits for execution when -search.maxConcurrentRequests limit is reached; see also -search.maxQueryDuration (default 10s)
  -select.maxSortBufferSize size
    	Query results from /select/logsql/query are automatically sorted by _time if their summary size doesn't exceed this value; otherwise, query results are streamed in the response without sorting; too big value for this flag may result in high memory usage since the sorting is performed in memory
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 1048576)
  -storageDataPath string
    	Path to directory with the VictoriaLogs data; see https://docs.victoriametrics.com/VictoriaLogs/#storage (default "victoria-logs-data")
  -storage.minFreeDiskSpaceBytes size
    	The minimum free disk space at -storageDataPath after which the storage stops accepting new data
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 10000000)
  -tls
    	Whether to enable TLS for incoming HTTP requests at -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated
  -tlsCipherSuites array
    	Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
    	Supports an array of values separated by comma or specified via multiple flags.
  -tlsKeyFile string
    	Path to file with TLS key if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated
  -tlsMinVersion string
    	Optional minimum TLS version to use for incoming requests over HTTPS if -tls is set. Supported values: TLS10, TLS11, TLS12, TLS13
  -version
```
