---
weight: 3
menu:
  docs:
    parent: victorialogs
    weight: 3
title: vlagent
tags:
  - logs
aliases:
  - /vlagent.html
  - /vlagent/index.html
  - /vlagent/
---

`vlagent` is a tiny agent which helps you collect logs from various sources
and store them in [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/).
See [Quick Start](#quick-start) for details.


## Motivation

While VictoriaLogs provides an efficient solution to store and observe logs, it lacks of replication out of box.
Previous solution was to configure clients to replicate log streams into multiple VictoriaLogs installations.
`vlagent` is a missing piece of log streams replication.

## Features

- It can accept logs from popular log collectors. See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/).
* Can replicate collected logs simultaneously to multiple VictoriaLogs instances - see [these docs](#replication-and-high-availability).
* Works smoothly in environments with unstable connections to remote storage. If the remote storage is unavailable, the collected logs
  are buffered at `-remoteWrite.tmpDataPath`. The buffered logs are sent to remote storage as soon as the connection
  to the remote storage is repaired. The maximum disk usage for the buffer can be limited with `-remoteWrite.maxDiskUsagePerURL`.

## Quick Start

Please download `vlagent` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) (
`vlagent` is also available in docker images [Docker Hub](https://hub.docker.com/r/victoriametrics/vlagent/tags) and [Quay](https://quay.io/repository/victoriametrics/vlagent?tab=tags)),
unpack it and pass the following flags to the `vlagent` binary in order to start sending the data to the VictoriaLogs remote storage:

* `-remoteWrite.url` with VictoriaLogs native protocol compatible remote storage endpoint, where to send the data to.
  The `-remoteWrite.url` may refer to [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) address. See [these docs](#srv-urls) for details.

Example command for writing the data received via [supported push-based protocols](#how-to-push-data-to-vlagent)
to [single-node VictoriaLogs](https://docs.victoriametrics.com/victorialogs) located at `victoria-logs-host:9428`:

```sh
/path/to/vlagent -remoteWrite.url=https://victoria-logs-host:9428/internal/insert
```

Pass `-help` to `vlagent` in order to see [the full list of supported command-line flags with their descriptions](#advanced-usage).

### Replication and high availability

`vlagent` replicates the collected logs among multiple remote storage instances configured via `-remoteWrite.url` args.
If a single remote storage instance temporarily is out of service, then the collected data remains available in another remote storage instance.
`vlagent` buffers the collected data in files at `-remoteWrite.tmpDataPath` until the remote storage becomes available again,
and then it sends the buffered data to the remote storage in order to prevent data gaps.

## Monitoring

`vlagent` exports various metrics in Prometheus exposition format at `http://vmalent-host:9429/metrics` page.
We recommend setting up regular scraping of this page either through `vmagent` or by Prometheus-compatible scraper,
so that the exported metrics may be analyzed later.

Use official [Grafana dashboard](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/dashboards/vlagent.json) for `vlagent` state overview.
Graphs on this dashboard contain useful hints - hover the `i` icon at the top left corner of each graph in order to read it.
If you have suggestions for improvements or have found a bug - please open an issue on github or add a review to the dashboard.

## Troubleshooting

* It is recommended [setting up the official Grafana dashboard](#monitoring) in order to monitor the state of `vlagent`.

* It is recommended increasing `-remoteWrite.queues` if `vlagent_remotewrite_pending_data_bytes` [metric](#monitoring)
  grows constantly. It is also recommended increasing `-remoteWrite.maxBlockSize` command-line flags in this case.
  This can improve data ingestion performance to the configured remote storage systems at the cost of higher memory usage.

* If you see gaps in the data pushed by `vlagent` to remote storage when `-remoteWrite.maxDiskUsagePerURL` is set,
  try increasing `-remoteWrite.queues`. Such gaps may appear because `vlagent` cannot keep up with sending the collected data to remote storage.
  Therefore, it starts dropping the buffered data if the on-disk buffer size exceeds `-remoteWrite.maxDiskUsagePerURL`.

* `vlagent` drops data blocks if remote storage replies with `400 Bad Request` and `404 Not Found` HTTP responses.
  The number of dropped blocks can be monitored via `vlagent_remotewrite_packets_dropped_total` metric exported at [/metrics page](#monitoring).

* `vlagent` buffers scraped data at the `-remoteWrite.tmpDataPath` directory until it is sent to `-remoteWrite.url`.
  The directory can grow large when remote storage is unavailable for extended periods of time and if the maximum directory size isn't limited
  with `-remoteWrite.maxDiskUsagePerURL` command-line flag.
  If you don't want to send all the buffered data from the directory to remote storage then simply stop `vlagent` and delete the directory.

* By default `vlagent` masks `-remoteWrite.url` with `secret-url` values in logs and at `/metrics` page because
  the url may contain sensitive information such as auth tokens or passwords.
  Pass `-remoteWrite.showURL` command-line flag when starting `vlagent` in order to see all the valid urls.

See also:

- [General Troubleshooting](https://docs.victoriametrics.com/victoriametrics/troubleshooting/)


## Profiling

`vlagent` provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile can be collected with the following command (replace `0.0.0.0` with hostname if needed):


```sh
curl http://0.0.0.0:9429/debug/pprof/heap > mem.pprof
```


* CPU profile can be collected with the following command (replace `0.0.0.0` with hostname if needed):


```sh
curl http://0.0.0.0:9429/debug/pprof/profile > cpu.pprof
```


The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).

It is safe sharing the collected profiles from security point of view, since they do not contain sensitive information.

## Advanced usage

`vlagent` can be fine-tuned with various command-line flags. Run `./vlagent -help` in order to see the full list of these flags with their descriptions and default values:

```bash
vlagent collects logs via popular data ingestion protocols and routes it to VictoriaLogs.

See the docs at https://docs.victoriametrics.com/victorialogs/vlagent/ .

  -blockcache.missesBeforeCaching int
    	The number of cache misses before putting the block into cache. Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage (default 2)
  -datadog.ignoreFields array
    	Comma-separated list of fields to ignore for logs ingested via DataDog protocol. See https://docs.victoriametrics.com/victorialogs/data-ingestion/datadog-agent/#dropping-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -datadog.maxRequestSize size
    	The maximum size in bytes of a single DataDog request
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -datadog.streamFields array
    	Comma-separated list of fields to use as log stream fields for logs ingested via DataDog protocol. See https://docs.victoriametrics.com/victorialogs/data-ingestion/datadog-agent/#stream-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -defaultMsgValue string
    	Default value for _msg field if the ingested log entry doesn't contain it; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field (default "missing _msg field; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field")
  -elasticsearch.version string
    	Elasticsearch version to report to client (default "8.9.0")
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
    	Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -filestream.disableFadvise
    	Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
    	Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
    	Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
    	Incoming connections to -httpListenAddr are closed after the configured timeout. This may help evenly spreading load among a cluster of services behind TCP-level load balancer. Zero value disables closing of incoming connections (default 2m0s)
  -http.disableCORS
    	Disable CORS for all origins (*)
  -http.disableKeepAlive
    	Whether to disable HTTP keep-alive for incoming connections at -httpListenAddr
  -http.disableResponseCompression
    	Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
    	Value for 'Content-Security-Policy' header, recommended: "default-src 'self'"
  -http.header.frameOptions string
    	Value for 'X-Frame-Options' header
  -http.header.hsts string
    	Value for 'Strict-Transport-Security' header, recommended: 'max-age=31536000; includeSubDomains'
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
  -httpListenAddr array
    	TCP address to listen for incoming http requests. Set this flag to empty value in order to disable listening on any port. This mode may be useful for running multiple vlagent instances on the same server. Note that /targets and /metrics pages aren't available if -httpListenAddr=''. See also -tls and -httpListenAddr.useProxyProtocol
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpListenAddr.useProxyProtocol array
    	Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -insert.disable
    	Whether to disable /insert/* HTTP endpoints
  -insert.maxFieldsPerLine int
    	The maximum number of log fields per line, which can be read by /insert/* handlers; see https://docs.victoriametrics.com/victorialogs/faq/#how-many-fields-a-single-log-entry-may-contain (default 1000)
  -insert.maxLineSizeBytes size
    	The maximum size of a single line, which can be read by /insert/* handlers; see https://docs.victoriametrics.com/victorialogs/faq/#what-length-a-log-record-is-expected-to-have
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 262144)
  -insert.maxQueueDuration duration
    	The maximum duration to wait in the queue when -maxConcurrentInserts concurrent insert requests are executed (default 1m0s)
  -internStringCacheExpireDuration duration
    	The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
    	Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
    	The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -internalinsert.disable
    	Whether to disable /internal/insert HTTP endpoint
  -internalinsert.maxRequestSize size
    	The maximum size in bytes of a single request, which can be accepted at /internal/insert HTTP endpoint
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -journald.ignoreFields array
    	Comma-separated list of fields to ignore for logs ingested over journald protocol. See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#dropping-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -journald.includeEntryMetadata
    	Include Journald fields with double underscore prefixes
  -journald.streamFields array
    	Comma-separated list of fields to use as log stream fields for logs ingested over journald protocol. See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#stream-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -journald.tenantID string
    	TenantID for logs ingested via the Journald endpoint. See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#multitenancy (default "0:0")
  -journald.timeField string
    	Field to use as a log timestamp for logs ingested via journald protocol. See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#time-field (default "__REALTIME_TIMESTAMP")
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
  -loggerMaxArgLen int
    	The maximum length of a single logged argument. Longer arguments are replaced with 'arg_start..arg_end', where 'arg_start' and 'arg_end' is prefix and suffix of the arg with the length not exceeding -loggerMaxArgLen / 2 (default 5000)
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
    	Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
    	Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -loki.disableMessageParsing
    	Whether to disable automatic parsing of JSON-encoded log fields inside Loki log message into distinct log fields
  -loki.maxRequestSize size
    	The maximum size in bytes of a single Loki request
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -maxConcurrentInserts int
    	The maximum number of concurrent insert requests. Set higher value when clients send data over slow networks. Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage. See also -insert.maxQueueDuration (default 20)
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
    	Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
    	Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
    	Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -opentelemetry.maxRequestSize size
    	The maximum size in bytes of a single OpenTelemetry request
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -pprofAuthKey value
    	Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides -httpAuth.*
    	Flag value can be read from the given file when using -pprofAuthKey=file:///abs/path/to/file or -pprofAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -pprofAuthKey=http://host/path or -pprofAuthKey=https://host/path
  -pushmetrics.disableCompression
    	Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
    	Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.header array
    	Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.interval duration
    	Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
    	Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.basicAuth.password array
    	Optional basic auth password to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.basicAuth.passwordFile array
    	Optional path to basic auth password to use for the corresponding -remoteWrite.url. The file is re-read every second
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.basicAuth.username array
    	Optional basic auth username to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.bearerToken array
    	Optional bearer auth token to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.bearerTokenFile array
    	Optional path to bearer token file to use for the corresponding -remoteWrite.url. The token is re-read from the file every second
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.flushInterval duration
    	Interval for flushing the data to remote storage. This option takes effect only when less than 2MB of data per second are pushed to -remoteWrite.url (default 1s)
  -remoteWrite.headers array
    	Optional HTTP headers to send with each request to the corresponding -remoteWrite.url. For example, -remoteWrite.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteWrite.url. Multiple headers must be delimited by '^^': -remoteWrite.headers='header1:value1^^header2:value2'
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.maxBlockSize size
    	The maximum block size to send to remote storage. Bigger blocks may improve performance at the cost of the increased memory usage.
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 8388608)
  -remoteWrite.maxDiskUsagePerURL array
    	The maximum file-based buffer size in bytes at -remoteWrite.tmpDataPath for each -remoteWrite.url. When buffer size reaches the configured maximum, then old data is dropped when adding new data to the buffer. Buffered data is stored in ~500MB chunks. It is recommended to set the value for this flag to a multiple of the block size 500MB. Disk usage is unlimited if the value is set to 0
    	Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB. (default 0)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.oauth2.clientID array
    	Optional OAuth2 clientID to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.oauth2.clientSecret array
    	Optional OAuth2 clientSecret to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.oauth2.clientSecretFile array
    	Optional OAuth2 clientSecretFile to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.oauth2.endpointParams array
    	Optional OAuth2 endpoint parameters to use for the corresponding -remoteWrite.url . The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.oauth2.scopes array
    	Optional OAuth2 scopes to use for the corresponding -remoteWrite.url. Scopes must be delimited by ';'
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.oauth2.tokenUrl array
    	Optional OAuth2 tokenURL to use for the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.proxyURL array
    	Optional proxy URL for writing data to the corresponding -remoteWrite.url. Supported proxies: http, https, socks5. Example: -remoteWrite.proxyURL=socks5://proxy:1234
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.queues int
    	The number of concurrent queues to each -remoteWrite.url. Set more queues if default number of queues isn't enough for sending high volume of collected data to remote storage. Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage (default 20)
  -remoteWrite.rateLimit array
    	Optional rate limit in bytes per second for data sent to the corresponding -remoteWrite.url. By default, the rate limit is disabled. It can be useful for limiting load on remote storage when big amounts of buffered data  (default 0)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.retryMaxTime array
    	The max time spent on retry attempts to send a block of data to the corresponding -remoteWrite.url. Change this value if it is expected for -remoteWrite.url to be unreachable for more than -remoteWrite.retryMaxTime. See also -remoteWrite.retryMinInterval (default 1m0s)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.retryMinInterval array
    	The minimum delay between retry attempts to send a block of data to the corresponding -remoteWrite.url. Every next retry attempt will double the delay to prevent hammering of remote database. See also -remoteWrite.retryMaxTime (default 1s)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.sendTimeout array
    	Timeout for sending a single block of data to the corresponding -remoteWrite.url (default 1m0s)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.showURL
    	Whether to show -remoteWrite.url in the exported metrics. It is hidden by default, since it can contain sensitive info such as auth key
  -remoteWrite.tlsCAFile array
    	Optional path to TLS CA file to use for verifying connections to the corresponding -remoteWrite.url. By default, system CA is used
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.tlsCertFile array
    	Optional path to client-side TLS certificate file to use when connecting to the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.tlsHandshakeTimeout array
    	The timeout for establishing tls connections to the corresponding -remoteWrite.url (default 20s)
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to default value.
  -remoteWrite.tlsInsecureSkipVerify array
    	Whether to skip tls verification when connecting to the corresponding -remoteWrite.url
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -remoteWrite.tlsKeyFile array
    	Optional path to client-side TLS certificate key to use when connecting to the corresponding -remoteWrite.url
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.tlsServerName array
    	Optional TLS server name to use for connections to the corresponding -remoteWrite.url. By default, the server name from -remoteWrite.url is used
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -remoteWrite.tmpDataPath string
    	Path to directory for storing pending data, which isn't sent to the configured -remoteWrite.url . See also -remoteWrite.maxDiskUsagePerURL (default "vlagent-remotewrite-data")
  -remoteWrite.url array
    	Remote storage URL to write data to. It must support VictoriaLogs native protocol. Example url: http://<victorialogs-host>:9428/internal/insert. Pass multiple -remoteWrite.url options in order to replicate the collected data to multiple remote storage systems.
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.compressMethod.tcp array
    	Compression method for syslog messages received at the corresponding -syslog.listenAddr.tcp. Supported values: none, gzip, deflate. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#compression
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.compressMethod.udp array
    	Compression method for syslog messages received at the corresponding -syslog.listenAddr.udp. Supported values: none, gzip, deflate. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#compression
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.decolorizeFields.tcp array
    	Fields to remove ANSI color codes across logs ingested via the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#decolorizing-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.decolorizeFields.udp array
    	Fields to remove ANSI color codes across logs ingested via the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#decolorizing-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.extraFields.tcp array
    	Fields to add to logs ingested via the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#adding-extra-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.extraFields.udp array
    	Fields to add to logs ingested via the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#adding-extra-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.ignoreFields.tcp array
    	Fields to ignore at logs ingested via the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#dropping-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.ignoreFields.udp array
    	Fields to ignore at logs ingested via the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#dropping-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.listenAddr.tcp array
    	Comma-separated list of TCP addresses to listen to for Syslog messages. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.listenAddr.udp array
    	Comma-separated list of UDP address to listen to for Syslog messages. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.streamFields.tcp array
    	Fields to use as log stream labels for logs ingested via the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#stream-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.streamFields.udp array
    	Fields to use as log stream labels for logs ingested via the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#stream-fields
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.tenantID.tcp array
    	TenantID for logs ingested via the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#multitenancy
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.tenantID.udp array
    	TenantID for logs ingested via the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#multitenancy
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.timezone string
    	Timezone to use when parsing timestamps in RFC3164 syslog messages. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 . See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/ (default "Local")
  -syslog.tls array
    	Whether to enable TLS for receiving syslog messages at the corresponding -syslog.listenAddr.tcp. The corresponding -syslog.tlsCertFile and -syslog.tlsKeyFile must be set if -syslog.tls is set. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -syslog.tlsCertFile array
    	Path to file with TLS certificate for the corresponding -syslog.listenAddr.tcp if the corresponding -syslog.tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.tlsCipherSuites array
    	Optional list of TLS cipher suites for -syslog.listenAddr.tcp if -syslog.tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants . See also https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.tlsKeyFile array
    	Path to file with TLS key for the corresponding -syslog.listenAddr.tcp if the corresponding -syslog.tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -syslog.tlsMinVersion string
    	The minimum TLS version to use for -syslog.listenAddr.tcp if -syslog.tls is set. Supported values: TLS10, TLS11, TLS12, TLS13. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#security (default "TLS13")
  -syslog.useLocalTimestamp.tcp array
    	Whether to use local timestamp instead of the original timestamp for the ingested syslog messages at the corresponding -syslog.listenAddr.tcp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#log-timestamps
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -syslog.useLocalTimestamp.udp array
    	Whether to use local timestamp instead of the original timestamp for the ingested syslog messages at the corresponding -syslog.listenAddr.udp. See https://docs.victoriametrics.com/victorialogs/data-ingestion/syslog/#log-timestamps
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -tls array
    	Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
    	Supports array of values separated by comma or specified via multiple flags.
    	Empty values are set to false.
  -tlsCertFile array
    	Path to file with TLS certificate for the corresponding -httpListenAddr if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsCipherSuites array
    	Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsKeyFile array
    	Path to file with TLS key for the corresponding -httpListenAddr if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsMinVersion array
    	Optional minimum TLS version to use for the corresponding -httpListenAddr if -tls is set. Supported values: TLS10, TLS11, TLS12, TLS13
    	Supports an array of values separated by comma or specified via multiple flags.
    	Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -version
    	Show VictoriaMetrics version
```
