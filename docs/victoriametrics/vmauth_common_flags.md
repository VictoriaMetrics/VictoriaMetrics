---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file should not be updated manually. Run make docs-update-flags while preparing a new release to sync flags in docs from actual binaries. -->
```shellhelp

vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/victoriametrics/vmauth/ .

  -auth.config string
     Path to auth config. It can point either to local file or to http url. See https://docs.victoriametrics.com/victoriametrics/vmauth/ for details on the format of this auth config
  -backend.TLSCAFile string
     Optional path to TLS root CA file, which is used for TLS verification when connecting to backends over HTTPS. See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup
  -backend.TLSCertFile string
     Optional path to TLS client certificate file, which must be sent to HTTPS backend. See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup
  -backend.TLSKeyFile string
     Optional path to TLS client key file, which must be sent to HTTPS backend. See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup
  -backend.TLSServerName string
     Optional TLS ServerName, which must be sent to HTTPS backend. See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup
  -backend.tlsInsecureSkipVerify
     Whether to skip TLS verification when connecting to backends over HTTPS. See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup
  -configCheckInterval duration
     interval for config file re-read. Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.
  -discoverBackendIPs
     Whether to discover backend IPs via periodic DNS queries to hostnames specified in url_prefix. This may be useful when url_prefix points to a hostname with dynamically scaled instances behind it. See https://docs.victoriametrics.com/victoriametrics/vmauth/#discovering-backend-ips
  -discoverBackendIPsInterval duration
     The interval for re-discovering backend IPs if -discoverBackendIPs command-line flag is set. Too low value may lead to DNS errors (default 10s)
  -dryRun
     Whether to check only config files without running vmauth. The auth configuration file is validated. The -auth.config flag must be specified.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -failTimeout duration
     Sets a delay period for load balancing to skip a malfunctioning backend (default 3s)
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
  -fs.maxConcurrency int
     The maximum number of concurrent goroutines to work with files; smaller values may help reducing Go scheduling latency on systems with small number of CPU cores; higher values may help reducing data ingestion latency on systems with high-latency storage such as NFS or Ceph (default fsutil.getDefaultConcurrency())
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
  -httpAuthHeader array
     HTTP request header to use for obtaining authorization tokens. By default auth tokens are read from Authorization request header
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpInternalListenAddr array
     TCP address to listen for incoming internal API http requests. Such as /health, /-/reload, /debug/pprof, etc. If flag is set, vmauth no longer serves internal API at -httpListenAddr.
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpListenAddr array
     TCP address to listen for incoming http requests. By default, serves internal API and proxy requests.  See also -tls, -httpListenAddr.useProxyProtocol and -httpInternalListenAddr.
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpListenAddr.useProxyProtocol array
     Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -idleConnTimeout duration
     The timeout for HTTP keep-alive connections to backend services. It is recommended setting this value to values smaller than -http.idleConnTimeout set at backend services (default 50s)
  -internStringCacheExpireDuration duration
     The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
     Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
     The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -loadBalancingPolicy string
     The default load balancing policy to use for backend urls specified inside url_prefix section. Supported policies: least_loaded, first_available. See https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing (default "least_loaded")
  -logInvalidAuthTokens
     Whether to log requests with invalid auth tokens. Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page
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
  -maxConcurrentPerUserRequests int
     The maximum number of concurrent requests vmauth can process per each configured user. Requests exceeding this limit are queued for up to -maxQueueDuration and then rejected with '429 Too Many Requests' http status code if the limit is still reached. This provides fairness and isolation between users, preventing a single user from consuming all the available resources. It works in conjunction with -maxConcurrentRequests, which sets the global limit across all users. This default can be overridden for individual users via max_concurrent_requests option in per-user config. See https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting (default 100)
  -maxConcurrentRequests int
     The maximum number of concurrent requests vmauth can process simultaneously. Requests exceeding this limit are queued for up to -maxQueueDuration and then rejected with '429 Too Many Requests' http status code if the limit is still reached. This protects vmauth itself from overloading and out-of-memory (OOM) failures. See also -maxConcurrentPerUserRequests and https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting (default 1000)
  -maxIdleConnsPerBackend int
     The maximum number of idle connections vmauth can open per each backend host (default 100)
  -maxQueueDuration duration
     The maximum duration to wait before rejecting incoming requests if concurrency limit specified via -maxConcurrentRequests or -maxConcurrentPerUserRequests command-line flags is reached. Requests are rejected with '429 Too Many Requests' http status code if the limit is still reached after the -maxQueueDuration duration. This allows graceful handling of short spikes in concurrent requests. See https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting (default 10s)
  -maxRequestBodySizeToRetry size
     The maximum request body size to buffer in memory for potential retries at other backends. Request bodies larger than this size cannot be retried if the backend fails. Zero or negative value disables request body buffering and retries. See also -requestBufferSize
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 16384)
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -mergeQueryArgs array
     An optional list of client query arg names, which must be merged with args at backend urls. The rest of client query args are replaced by the corresponding query args from backend urls for security reasons; see https://docs.victoriametrics.com/victoriametrics/vmauth/#query-args-handling
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -pprofAuthKey value
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -pprofAuthKey=file:///abs/path/to/file or -pprofAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -pprofAuthKey=http://host/path or -pprofAuthKey=https://host/path
  -pushmetrics.disableCompression
     Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.header array
     Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.interval duration
     Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -reloadAuthKey value
     Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -reloadAuthKey=file:///abs/path/to/file or -reloadAuthKey=file://./relative/path/to/file.
     Flag value can be read from the given http/https url when using -reloadAuthKey=http://host/path or -reloadAuthKey=https://host/path
  -removeXFFHTTPHeaderValue
     Whether to remove the X-Forwarded-For HTTP header value from client requests before forwarding them to the backend. Recommended when vmauth is exposed to the internet.
  -requestBufferSize size
     The size of the buffer for reading the request body before proxying the request to backends. This allows reducing the comsumption of backend resources when processing requests from clients connected via slow networks. Set to 0 to disable request buffering. See https://docs.victoriametrics.com/victoriametrics/vmauth/#request-body-buffering
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 32768)
  -responseTimeout duration
     The timeout for receiving a response from backend (default 5m0s)
  -retryStatusCodes array
     Comma-separated list of default HTTP response status codes when vmauth re-tries the request on other backends. See https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing for details (default 0)
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to default value.
  -secret.flags array
     Comma-separated list of flag names with secret values. Values for these flags are hidden in logs and on /metrics page
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tls array
     Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -tlsCertFile array
     Path to file with TLS certificate for the corresponding -httpListenAddr if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsCipherSuites array
     Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsKeyFile array
     Path to file with TLS key for the corresponding -httpListenAddr if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsMinVersion array
     Optional minimum TLS version to use for the corresponding -httpListenAddr if -tls is set. Supported values: TLS10, TLS11, TLS12, TLS13
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -version
     Show VictoriaMetrics version
```
