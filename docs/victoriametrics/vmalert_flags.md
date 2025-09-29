---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file has to be manually updated during feature work in PR, make docs-update-flags command could be used periodically to ensure the flags in sync. -->
```shellhelp

vmalert processes alerts and recording rules.

See the docs at https://docs.victoriametrics.com/victoriametrics/vmalert/ .

  -blockcache.missesBeforeCaching int
     The number of cache misses before putting the block into cache. Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage (default 2)
  -clusterMode
     If clusterMode is enabled, then vmalert automatically adds the tenant specified in config groups to -datasource.url, -remoteWrite.url and -remoteRead.url. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -configCheckInterval duration
     Interval for checking for changes in '-rule' or '-notifier.config' files. By default, the checking is disabled. Send SIGHUP signal in order to force config check for changes.
  -datasource.appendTypePrefix
     Whether to add type prefix to -datasource.url based on the query type. Set to true if sending different query types to the vmselect URL.
  -datasource.basicAuth.password string
     Optional basic auth password for -datasource.url
  -datasource.basicAuth.passwordFile string
     Optional path to basic auth password to use for -datasource.url
  -datasource.basicAuth.username string
     Optional basic auth username for -datasource.url
  -datasource.bearerToken string
     Optional bearer auth token to use for -datasource.url.
  -datasource.bearerTokenFile string
     Optional path to bearer token file to use for -datasource.url.
  -datasource.disableKeepAlive
     Whether to disable long-lived connections to the datasource. If true, disables HTTP keep-alive and will only use the connection to the server for a single HTTP request.
  -datasource.disableStepParam
     Whether to disable adding 'step' param in instant queries to the configured -datasource.url and -remoteRead.url. Only valid for prometheus datasource. This might be useful when using vmalert with datasources that do not support 'step' param for instant queries, like Google Managed Prometheus. It is not recommended to enable this flag if you use vmalert with VictoriaMetrics.
  -datasource.headers string
     Optional HTTP extraHeaders to send with each request to the corresponding -datasource.url. For example, -datasource.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -datasource.url. Multiple headers must be delimited by '^^': -datasource.headers='header1:value1^^header2:value2'
  -datasource.idleConnTimeout duration
     Defines a duration for idle (keep-alive connections) to exist. Consider setting this value less than "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors. (default 50s)
  -datasource.maxIdleConnections int
     Defines the number of idle (keep-alive connections) to each configured datasource. Consider setting this value equal to the value: groups_total * group.concurrency. Too low a value may result in a high number of sockets in TIME_WAIT state. (default 100)
  -datasource.oauth2.clientID string
     Optional OAuth2 clientID to use for -datasource.url
  -datasource.oauth2.clientSecret string
     Optional OAuth2 clientSecret to use for -datasource.url
  -datasource.oauth2.clientSecretFile string
     Optional OAuth2 clientSecretFile to use for -datasource.url
  -datasource.oauth2.endpointParams string
     Optional OAuth2 endpoint parameters to use for -datasource.url . The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}
  -datasource.oauth2.scopes string
     Optional OAuth2 scopes to use for -datasource.url. Scopes must be delimited by ';'
  -datasource.oauth2.tokenUrl string
     Optional OAuth2 tokenURL to use for -datasource.url
  -datasource.queryStep duration
     How far a value can fallback to when evaluating queries to the configured -datasource.url and -remoteRead.url. Only valid for prometheus datasource. For example, if -datasource.queryStep=15s then param "step" with value "15s" will be added to every query. If set to 0, rule's evaluation interval will be used instead. (default 5m0s)
  -datasource.roundDigits int
     Adds "round_digits" GET param to datasource requests which limits the number of digits after the decimal point in response values. Only valid for VictoriaMetrics as the datasource.
  -datasource.showURL
     Whether to avoid stripping sensitive information such as auth headers or passwords from URLs in log messages or UI and exported metrics. It is hidden by default, since it can contain sensitive info such as auth key
  -datasource.tlsCAFile string
     Optional path to TLS CA file to use for verifying connections to -datasource.url. By default, system CA is used
  -datasource.tlsCertFile string
     Optional path to client-side TLS certificate file to use when connecting to -datasource.url
  -datasource.tlsInsecureSkipVerify
     Whether to skip tls verification when connecting to -datasource.url
  -datasource.tlsKeyFile string
     Optional path to client-side TLS certificate key to use when connecting to -datasource.url
  -datasource.tlsServerName string
     Optional TLS server name to use for connections to -datasource.url. By default, the server name from -datasource.url is used
  -datasource.url string
     Datasource compatible with Prometheus HTTP API. It can be single node VictoriaMetrics or vmselect endpoint. Required parameter. Supports address in the form of IP address with a port (e.g., http://127.0.0.1:8428) or DNS SRV record. See also -remoteRead.disablePathAppend and -datasource.showURL
  -defaultTenant.graphite string
     Default tenant for Graphite alerting groups. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy .This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -defaultTenant.prometheus string
     Default tenant for Prometheus alerting groups. See https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -disableAlertgroupLabel
     Whether to disable adding group's Name as label to generated alerts and time series.
  -dryRun
     Whether to check only config files without running vmalert. The rules file are validated. The -rule flag must be specified.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -evaluationInterval duration
     How often to evaluate the rules (default 1m0s)
  -external.alert.source string
     External Alert Source allows to override the Source link for alerts sent to AlertManager for cases where you want to build a custom link to Grafana, Prometheus or any other service. Supports templating - see https://docs.victoriametrics.com/victoriametrics/vmalert/#templating . For example, link to Grafana: -external.alert.source='explore?orgId=1&left={"datasource":"VictoriaMetrics","queries":[{"expr":{{.Expr|jsonEscape|queryEscape}},"refId":"A"}],"range":{"from":"now-1h","to":"now"}}'. Link to VMUI: -external.alert.source='vmui/#/?g0.expr={{.Expr|queryEscape}}'. If empty 'vmalert/alert?group_id={{.GroupID}}&alert_id={{.AlertID}}' is used.
  -external.label exported_
     Optional label in the form 'Name=value' to add to all generated recording rules and alerts. In case of conflicts, original labels are kept with prefix exported_.
     Supports an `array` of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -external.url string
     External URL is used as alert's source for sent alerts to the notifier. By default, hostname is used as address.
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
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
     Flag value can be read from the given file when using -httpAuth.password=file:///abs/path/to/file or -httpAuth.password=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -httpAuth.password=http://host/path or -httpAuth.password=https://host/path
  -httpAuth.username string
     Username for HTTP server's Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr array
     Address to listen for incoming http requests. See also -tls and -httpListenAddr.useProxyProtocol
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpListenAddr.useProxyProtocol array
     Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -internStringCacheExpireDuration duration
     The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
     Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
     The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -license string
     License key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed via file specified by -licenseFile command-line flag
  -license.forceOffline
     Whether to enable offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification. This flag is available only in Enterprise binaries
  -licenseFile string
     Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed inline via -license command-line flag
  -licenseFile.reloadInterval duration
     Interval for reloading the license file specified via -licenseFile. See https://victoriametrics.com/products/enterprise/ . This flag is available only in Enterprise binaries (default 1h0m0s)
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
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.basicAuth.password array
     Optional basic auth password for -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.basicAuth.passwordFile array
     Optional path to basic auth password file for -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.basicAuth.username array
     Optional basic auth username for -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.bearerToken array
     Optional bearer token for -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.bearerTokenFile array
     Optional path to bearer token file for -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.blackhole
     Whether to blackhole alerting notifications. Enable this flag if you want vmalert to evaluate alerting rules without sending any notifications to external receivers (eg. alertmanager). -notifier.url, -notifier.config and -notifier.blackhole are mutually exclusive.
  -notifier.config string
     Path to configuration file for notifiers
  -notifier.headers array
     Optional HTTP headers to send with each request to the corresponding -notifier.url. For example, -remoteWrite.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -notifier.url. Multiple headers must be delimited by '^^': -notifier.headers='header1:value1^^header2:value2,header3:value3'
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.clientID array
     Optional OAuth2 clientID to use for -notifier.url. If multiple args are set, then they are applied independently for the corresponding -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.clientSecret array
     Optional OAuth2 clientSecret to use for -notifier.url. If multiple args are set, then they are applied independently for the corresponding -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.clientSecretFile array
     Optional OAuth2 clientSecretFile to use for -notifier.url. If multiple args are set, then they are applied independently for the corresponding -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.endpointParams array
     Optional OAuth2 endpoint parameters to use for the corresponding -notifier.url . The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.scopes array
     Optional OAuth2 scopes to use for -notifier.url. Scopes must be delimited by ';'. If multiple args are set, then they are applied independently for the corresponding -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.oauth2.tokenUrl array
     Optional OAuth2 tokenURL to use for -notifier.url. If multiple args are set, then they are applied independently for the corresponding -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.sendTimeout array
     Timeout when sending alerts to the corresponding -notifier.url (default 10s)
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to default value.
  -notifier.showURL
     Whether to avoid stripping sensitive information such as passwords from URL in log messages or UI for -notifier.url. It is hidden by default, since it can contain sensitive info such as auth key
  -notifier.suppressDuplicateTargetErrors
     Whether to suppress 'duplicate target' errors during discovery
  -notifier.tlsCAFile array
     Optional path to TLS CA file to use for verifying connections to -notifier.url. By default, system CA is used
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.tlsCertFile array
     Optional path to client-side TLS certificate file to use when connecting to -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.tlsInsecureSkipVerify array
     Whether to skip tls verification when connecting to -notifier.url
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -notifier.tlsKeyFile array
     Optional path to client-side TLS certificate key to use when connecting to -notifier.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.tlsServerName array
     Optional TLS server name to use for connections to -notifier.url. By default, the server name from -notifier.url is used
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -notifier.url array
     Prometheus Alertmanager URL, e.g. http://127.0.0.1:9093. List all Alertmanager URLs if it runs in the cluster mode to ensure high availability.
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pprofAuthKey value
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -pprofAuthKey=file:///abs/path/to/file or -pprofAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -pprofAuthKey=http://host/path or -pprofAuthKey=https://host/path
  -promscrape.consul.waitTime duration
     Wait time used by Consul service discovery. Default value is used if not set
  -promscrape.consulSDCheckInterval duration
     Interval for checking for changes in Consul. This works only if consul_sd_configs is configured in '-promscrape.config' file. See https://docs.victoriametrics.com/victoriametrics/sd_configs/#consul_sd_configs for details (default 30s)
  -promscrape.discovery.concurrency int
     The maximum number of concurrent requests to Prometheus autodiscovery API (Consul, Kubernetes, etc.) (default 100)
  -promscrape.discovery.concurrentWaitTime duration
     The maximum duration for waiting to perform API requests if more than -promscrape.discovery.concurrency requests are simultaneously performed (default 1m0s)
  -promscrape.dnsSDCheckInterval duration
     Interval for checking for changes in dns. This works only if dns_sd_configs is configured in '-promscrape.config' file. See https://docs.victoriametrics.com/victoriametrics/sd_configs/#dns_sd_configs for details (default 30s)
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
  -reloadAuthKey value
     Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -reloadAuthKey=file:///abs/path/to/file or -reloadAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -reloadAuthKey=http://host/path or -reloadAuthKey=https://host/path
  -remoteRead.basicAuth.password string
     Optional basic auth password for -remoteRead.url
  -remoteRead.basicAuth.passwordFile string
     Optional path to basic auth password to use for -remoteRead.url
  -remoteRead.basicAuth.username string
     Optional basic auth username for -remoteRead.url
  -remoteRead.bearerToken string
     Optional bearer auth token to use for -remoteRead.url.
  -remoteRead.bearerTokenFile string
     Optional path to bearer token file to use for -remoteRead.url.
  -remoteRead.disablePathAppend
     Whether to disable automatic appending of '/api/v1/query' or '/select/logsql/stats_query' path to the configured -datasource.url and -remoteRead.url
  -remoteRead.headers string
     Optional HTTP headers to send with each request to the corresponding -remoteRead.url. For example, -remoteRead.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteRead.url. Multiple headers must be delimited by '^^': -remoteRead.headers='header1:value1^^header2:value2'
  -remoteRead.idleConnTimeout duration
     Defines a duration for idle (keep-alive connections) to exist. Consider settings this value less to the value of "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors. (default 50s)
  -remoteRead.lookback duration
     Lookback defines how far to look into past for alerts timeseries. For example, if lookback=1h then range from now() to now()-1h will be scanned. (default 1h0m0s)
  -remoteRead.oauth2.clientID string
     Optional OAuth2 clientID to use for -remoteRead.url.
  -remoteRead.oauth2.clientSecret string
     Optional OAuth2 clientSecret to use for -remoteRead.url.
  -remoteRead.oauth2.clientSecretFile string
     Optional OAuth2 clientSecretFile to use for -remoteRead.url.
  -remoteRead.oauth2.endpointParams string
     Optional OAuth2 endpoint parameters to use for -remoteRead.url . The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}
  -remoteRead.oauth2.scopes string
     Optional OAuth2 scopes to use for -remoteRead.url. Scopes must be delimited by ';'.
  -remoteRead.oauth2.tokenUrl string
     Optional OAuth2 tokenURL to use for -remoteRead.url. 
  -remoteRead.showURL
     Whether to show -remoteRead.url in the exported metrics. It is hidden by default, since it can contain sensitive info such as auth key
  -remoteRead.tlsCAFile string
     Optional path to TLS CA file to use for verifying connections to -remoteRead.url. By default, system CA is used
  -remoteRead.tlsCertFile string
     Optional path to client-side TLS certificate file to use when connecting to -remoteRead.url
  -remoteRead.tlsInsecureSkipVerify
     Whether to skip tls verification when connecting to -remoteRead.url
  -remoteRead.tlsKeyFile string
     Optional path to client-side TLS certificate key to use when connecting to -remoteRead.url
  -remoteRead.tlsServerName string
     Optional TLS server name to use for connections to -remoteRead.url. By default, the server name from -remoteRead.url is used
  -remoteRead.url vmalert
     Optional URL to datasource compatible with MetricsQL. It can be single node VictoriaMetrics or vmselect.Remote read is used to restore alerts state.This configuration makes sense only if vmalert was configured with `remoteWrite.url` before and has been successfully persisted its state. Supports address in the form of IP address with a port (e.g., http://127.0.0.1:8428) or DNS SRV record. See also '-remoteRead.disablePathAppend', '-remoteRead.showURL'.
  -remoteWrite.basicAuth.password string
     Optional basic auth password for -remoteWrite.url
  -remoteWrite.basicAuth.passwordFile string
     Optional path to basic auth password to use for -remoteWrite.url
  -remoteWrite.basicAuth.username string
     Optional basic auth username for -remoteWrite.url
  -remoteWrite.bearerToken string
     Optional bearer auth token to use for -remoteWrite.url.
  -remoteWrite.bearerTokenFile string
     Optional path to bearer token file to use for -remoteWrite.url.
  -remoteWrite.concurrency int
     Defines number of writers for concurrent writing into remote write endpoint. Default value depends on the number of available CPU cores. (default 2*cgroup.AvailableCPUs())
  -remoteWrite.disablePathAppend
     Whether to disable automatic appending of '/api/v1/write' path to the configured -remoteWrite.url.
  -remoteWrite.flushInterval duration
     Defines interval of flushes to remote write endpoint (default 2s)
  -remoteWrite.headers string
     Optional HTTP headers to send with each request to the corresponding -remoteWrite.url. For example, -remoteWrite.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteWrite.url. Multiple headers must be delimited by '^^': -remoteWrite.headers='header1:value1^^header2:value2'
  -remoteWrite.idleConnTimeout duration
     Defines a duration for idle (keep-alive connections) to exist. Consider settings this value less to the value of "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors. (default 50s)
  -remoteWrite.maxBatchSize int
     Defines max number of timeseries to be flushed at once (default 10000)
  -remoteWrite.maxQueueSize int
     Defines the max number of pending datapoints to remote write endpoint (default 100000)
  -remoteWrite.oauth2.clientID string
     Optional OAuth2 clientID to use for -remoteWrite.url
  -remoteWrite.oauth2.clientSecret string
     Optional OAuth2 clientSecret to use for -remoteWrite.url
  -remoteWrite.oauth2.clientSecretFile string
     Optional OAuth2 clientSecretFile to use for -remoteWrite.url
  -remoteWrite.oauth2.endpointParams string
     Optional OAuth2 endpoint parameters to use for -remoteWrite.url . The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}
  -remoteWrite.oauth2.scopes string
     Optional OAuth2 scopes to use for -notifier.url. Scopes must be delimited by ';'.
  -remoteWrite.oauth2.tokenUrl string
     Optional OAuth2 tokenURL to use for -notifier.url.
  -remoteWrite.retryMaxTime duration
     The max time spent on retry attempts for the failed remote-write request. Change this value if it is expected for remoteWrite.url to be unreachable for more than -remoteWrite.retryMaxTime. See also -remoteWrite.retryMinInterval (default 30s)
  -remoteWrite.retryMinInterval duration
     The minimum delay between retry attempts. Every next retry attempt will double the delay to prevent hammering of remote database. See also -remoteWrite.retryMaxTime (default 1s)
  -remoteWrite.sendTimeout duration
     Timeout for sending data to the configured -remoteWrite.url. (default 30s)
  -remoteWrite.showURL
     Whether to show -remoteWrite.url in the exported metrics. It is hidden by default, since it can contain sensitive info such as auth key
  -remoteWrite.tlsCAFile string
     Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. By default, system CA is used
  -remoteWrite.tlsCertFile string
     Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url
  -remoteWrite.tlsInsecureSkipVerify
     Whether to skip tls verification when connecting to -remoteWrite.url
  -remoteWrite.tlsKeyFile string
     Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url
  -remoteWrite.tlsServerName string
     Optional TLS server name to use for connections to -remoteWrite.url. By default, the server name from -remoteWrite.url is used
  -remoteWrite.url string
     Optional URL to VictoriaMetrics or vminsert where to persist alerts state and recording rules results in form of timeseries. Supports address in the form of IP address with a port (e.g., http://127.0.0.1:8428) or DNS SRV record. For example, if -remoteWrite.url=http://127.0.0.1:8428 is specified, then the alerts state will be written to http://127.0.0.1:8428/api/v1/write . See also -remoteWrite.disablePathAppend, '-remoteWrite.showURL'.
  -replay.disableProgressBar
     Whether to disable rendering progress bars during the replay. Progress bar rendering might be verbose or break the logs parsing, so it is recommended to be disabled when not used in interactive mode.
  -replay.maxDatapointsPerQuery int
     Max number of data points expected in one request. It affects the max time range for every '/query_range' request during the replay. The higher the value, the less requests will be made during replay. (default 1000)
  -replay.ruleEvaluationConcurrency int
     The maximum number of concurrent '/query_range' requests when replay recording rule or alerting rule with for=0. Increasing this value when replaying for a long time, since each request is limited by -replay.maxDatapointsPerQuery. (default 1)
  -replay.ruleRetryAttempts int
     Defines how many retries to make before giving up on rule if request for it returns an error. (default 5)
  -replay.rulesDelay duration
     Delay before evaluating the next rule within the group. Is important for chained rules. Keep it equal or bigger than -remoteWrite.flushInterval. When set to >0, replay ignores group's concurrency setting. (default 1s)
  -replay.timeFrom string
     The time filter in RFC3339 format to start the replay from. E.g. '2020-01-01T20:07:00Z'
  -replay.timeTo string
     The time filter in RFC3339 format to finish the replay by. E.g. '2020-01-01T20:07:00Z'. By default, is set to the current time.
  -rule array
     Path to the files or http url with alerting and/or recording rules in YAML format.
     Supports hierarchical patterns and regexpes.
     Examples:
      -rule="/path/to/file". Path to a single file with alerting rules.
      -rule="http://<some-server-addr>/path/to/rules". HTTP URL to a page with alerting rules.
      -rule="dir/*.yaml" -rule="/*.yaml" -rule="gcs://vmalert-rules/tenant_%{TENANT_ID}/prod". 
      -rule="dir/**/*.yaml". Includes all the .yaml files in "dir" subfolders recursively.
     Rule files support YAML multi-document. Files may contain %{ENV_VAR} placeholders, which are substituted by the corresponding env vars.
     
     Enterprise version of vmalert supports S3 and GCS paths to rules.
     For example: gs://bucket/path/to/rules, s3://bucket/path/to/rules
     S3 and GCS paths support only matching by prefix, e.g. s3://bucket/dir/rule_ matches
     all files with prefix rule_ in folder dir.
     See https://docs.victoriametrics.com/victoriametrics/vmalert/#reading-rules-from-object-storage
     
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -rule.defaultRuleType string
     Default type for rule expressions, can be overridden via "type" parameter on the group level, see https://docs.victoriametrics.com/victoriametrics/vmalert/#groups. Supported values: "graphite", "prometheus" and "vlogs". (default "prometheus")
  -rule.evalDelay duration
     Adjustment of the 'time' parameter for rule evaluation requests to compensate intentional data delay from the datasource. Normally, should be equal to '-search.latencyOffset' (cmd-line flag configured for VictoriaMetrics single-node or vmselect). This doesn't apply to groups with eval_offset specified. (default 30s)
  -rule.maxResolveDuration duration
     Limits the maxiMum duration for automatic alert expiration, which by default is 4 times evaluationInterval of the parent group
  -rule.resendDelay duration
     MiniMum amount of time to wait before resending an alert to notifier.
  -rule.resultsLimit int
     Limits the number of alerts or recording results a single rule can produce. Can be overridden by the limit option under group if specified. If exceeded, the rule will be marked with an error and all its results will be discarded. 0 means no limit. (default 0)
  -rule.stripFilePath
     Whether to strip file path in responses from the api/v1/rules API for files configured via -rule cmd-line flag. For example, the file path '/path/to/tenant_id/rules.yml' will be stripped to just 'rules.yml'. This flag might be useful to hide sensitive information in file path such as tenant ID. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -rule.templates array
     Path or glob pattern to location with go template definitions for rules annotations templating. Flag can be specified multiple times.
     Examples:
      -rule.templates="/path/to/file". Path to a single file with go templates
      -rule.templates="dir/*.tpl" -rule.templates="/*.tpl". Relative path to all .tpl files in "dir" folder,
     absolute path to all .tpl files in root.
      -rule.templates="dir/**/*.tpl". Includes all the .tpl files in "dir" subfolders recursively.
     
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -rule.updateEntriesLimit int
     Defines the max number of rule's state updates stored in-memory. Rule's updates are available on rule's Details page and are used for debugging purposes. The number of stored updates can be overridden per rule via update_entries_limit param. (default 20)
  -rule.validateExpressions
     Whether to validate rules expressions for different types. (default true)
  -rule.validateTemplates
     Whether to validate annotation and label templates (default true)
  -s3.configFilePath string
     Path to file with S3 configs. Configs are loaded from default location if not set.
     See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.configProfile string
     Profile name for S3 configs. If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), or if both not set, DefaultSharedConfigProfile is used. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.credsFilePath string
     Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.
     See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.customEndpoint string
     Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -s3.forcePathStyle
     Prefixing endpoint with bucket name when set false, true by default. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default true)
  -tls array
     Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
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
