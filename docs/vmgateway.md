---
sort: 8
---

# vmgateway


<img alt="vmgateway" src="vmgateway-overview.jpeg">

`vmgateway` is a proxy for Victoria Metrics TSDB. It provides the following features:

* Rate Limiter
  * Based on cluster tenants' utilization supports multiple time interval limits for ingestion/retrieving metrics
* Token Access Control
  * Supports additional per-label access control for Single and Cluster versions of Victoria Metrics TSDB
  * Provides access by tenantID at Cluster version
  * Allows to separate write/read/admin access to data

`vmgateway` is included in an [enterprise package](https://victoriametrics.com/enterprise.html).


## Access Control

<img alt="vmgateway-ac" src="vmgateway-access-control.jpg">

`vmgateway` supports jwt based authentication. With jwt payload can be configured access to specific tenant, labels, read/write.

jwt token must be in following format:
```json
{
  "exp": 1617304574,
  "vm_access": {
      "tenant_id": {
        "account_id": 1,
        "project_id": 5
      },
      "extra_labels": {
         "team": "dev",
         "project": "mobile"
      },
      "mode": 1
  }
}
```
Where:
- `exp` - required, expire time in unix_timestamp. If token expires, `vmgateway` rejects request.
- `vm_access` - required, dict with claim info, minimum form: `{"vm_access": {"tenand_id": {}}`
- `tenant_id` - optional, make sense only for cluster mode, routes request to corresponding tenant.
- `extra_labels` - optional, key-value pairs for label filters - added to ingested or selected metrics.
- `mode` - optional, access mode for api - read, write, full. supported values: 0 - full (default value), 1 - read, 2 - write.

## QuickStart

Start single version of Victoria Metrics

```bash
# single
# start node
./bin/victoria-metrics --selfScrapeInterval=10s
```

Start vmgateway

```bash
./bin/vmgateway -eula -enable.auth -read.url http://localhost:8428 --write.url http://localhost:8428
```

Retrieve data from database
```bash
curl 'http://localhost:8431/api/v1/series/count' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ2bV9hY2Nlc3MiOnsidGVuYW50X2lkIjp7fSwicm9sZSI6MX0sImV4cCI6MTkzOTM0NjIxMH0.5WUxEfdcV9hKo4CtQdtuZYOGpGXWwaqM9VuVivMMrVg'
```

 Request with incorrect token or with out token will be rejected:
```bash
curl 'http://localhost:8431/api/v1/series/count'

curl 'http://localhost:8431/api/v1/series/count' -H 'Authorization: Bearer incorrect-token'
```


## Rate Limiter

<img alt="vmgateway-rl" src="vmgateway-rate-limiting.jpg">

 Limits incoming requests by given pre-configured limits. It supports read and write limiting by a tenant. 

 `vmgateway` needs datasource for rate limits queries. It can be single-node or cluster version of `victoria-metrics`. 
It must have metrics scrapped from cluster, that you want to rate limit.

List of supported limit types:
- `queries` - count of api requests made at tenant to read api, such as `/api/v1/query`, `/api/v1/series` and others.
- `active_series` - count of current active series at given tenant.
- `new_series` - count of created series aka churn rate
- `rows_inserted` - count of inserted rows per tenant.

List of supported time windows:
- `minute`
- `hour`

Limits can be specified per tenant or at global level, if you omit `project_id` and `account_id`.

Example of configuration file:

```yaml
limits:
  - type: queries
    value: 1000
    resolution: minute
  - type: queries
    value: 10000
    resolution: hour
  - type: queries
    value: 10
    resolution: minute
    project_id: 5
    account_id: 1
```

## QuickStart

 cluster version required for rate limiting.
```bash
# start datasource for cluster metrics

cat << EOF > cluster.yaml
scrape_configs:
  - job_name: cluster
    scrape_interval: 5s
    static_configs:
    - targets: ['127.0.0.1:8481','127.0.0.1:8482','127.0.0.1:8480']
EOF

./bin/victoria-metrics --promscrape.config cluster.yaml

# start cluster

# start vmstorage, vmselect and vminsert
./bin/vmstorage -eula
./bin/vmselect -eula -storageNode 127.0.0.1:8401
./bin/vminsert -eula -storageNode 127.0.0.1:8400

# create base rate limitng config:
cat << EOF > limit.yaml
limits:
  - type: queries
    value: 100
  - type: rows_inserted 
    value: 100000
  - type: new_series
    value: 1000
  - type: active_series
    value: 100000
  - type: queries
    value: 1
    account_id: 15
EOF

# start gateway with clusterMoe
./bin/vmgateway -eula -enable.rateLimit -ratelimit.config limit.yaml -datasource.url http://localhost:8428 -enable.auth -clusterMode -write.url=http://localhost:8480 --read.url=http://localhost:8481

# ingest simple metric to tenant 1:5
curl 'http://localhost:8431/api/v1/import/prometheus' -X POST  -d 'foo{bar="baz1"} 123' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MjAxNjIwMDAwMDAsInZtX2FjY2VzcyI6eyJ0ZW5hbnRfaWQiOnsiYWNjb3VudF9pZCI6MTV9fX0.PB1_KXDKPUp-40pxOGk6lt_jt9Yq80PIMpWVJqSForQ'
# read metric from tenant 1:5
curl 'http://localhost:8431/api/v1/labels' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MjAxNjIwMDAwMDAsInZtX2FjY2VzcyI6eyJ0ZW5hbnRfaWQiOnsiYWNjb3VudF9pZCI6MTV9fX0.PB1_KXDKPUp-40pxOGk6lt_jt9Yq80PIMpWVJqSForQ'

# check rate limit 
```

## Configuration

The shortlist of configuration flags is the following:
```bash
  -clusterMode
        enable it for cluster version
  -datasource.appendTypePrefix
        Whether to add type prefix to -datasource.url based on the query type. Set to true if sending different query types to VMSelect URL.
  -datasource.basicAuth.password string
        Optional basic auth password for -datasource.url
  -datasource.basicAuth.username string
        Optional basic auth username for -datasource.url
  -datasource.lookback duration
        Lookback defines how far to look into past when evaluating queries. For example, if datasource.lookback=5m then param "time" with value now()-5m will be added to every query.
  -datasource.maxIdleConnections int
        Defines the number of idle (keep-alive connections) to configured datasource.Consider to set this value equal to the value: groups_total * group.concurrency. Too low value may result into high number of sockets in TIME_WAIT state. (default 100)
  -datasource.queryStep duration
        queryStep defines how far a value can fallback to when evaluating queries. For example, if datasource.queryStep=15s then param "step" with value "15s" will be added to every query.
  -datasource.tlsCAFile string
        Optional path to TLS CA file to use for verifying connections to -datasource.url. By default system CA is used
  -datasource.tlsCertFile string
        Optional path to client-side TLS certificate file to use when connecting to -datasource.url
  -datasource.tlsInsecureSkipVerify
        Whether to skip tls verification when connecting to -datasource.url
  -datasource.tlsKeyFile string
        Optional path to client-side TLS certificate key to use when connecting to -datasource.url
  -datasource.tlsServerName string
        Optional TLS server name to use for connections to -datasource.url. By default the server name from -datasource.url is used
  -datasource.url string
        Victoria Metrics or VMSelect url. Required parameter. E.g. http://127.0.0.1:8428
  -enable.auth
        enables auth with jwt token
  -enable.rateLimit
        enables rate limiter
  -enableTCP6
        Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
        Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isnt set
  -envflag.prefix string
        Prefix for environment variables if -envflag.enable is set
  -eula
        By specifying this flag you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -fs.disableMmap
        Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
        Incoming http connections are closed after the configured timeout. This may help spreading incoming load among a cluster of services behind load balancer. Note that the real timeout may be bigger by up to 10% as a protection from Thundering herd problem (default 2m0s)
  -http.disableResponseCompression
        Disable compression of HTTP responses for saving CPU resources. By default compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
        Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
        The maximum duration for graceful shutdown of HTTP server. Highly loaded server may require increased value for graceful shutdown (default 7s)
  -http.pathPrefix string
        An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
        Optional delay before http server shutdown. During this dealy the servier returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
        Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
        Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
        TCP address to listen for http connections (default ":8431")
  -loggerDisableTimestamps
        Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
        Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, then the remaining errors are suppressed. Zero value disables the rate limit
  -loggerFormat string
        Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
        Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
        Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
        Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
        Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero value disables the rate limit
  -memory.allowedBytes size
        Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to non-zero value. Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. Too high value may evict too much data from OS page cache, which will result in higher disk IO usage
        Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
        Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. Too high value may evict too much data from OS page cache, which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
        Auth key for /metrics. It overrides httpAuth settings
  -pprofAuthKey string
        Auth key for /debug/pprof. It overrides httpAuth settings
  -ratelimit.config string
        path for configuration file
  -ratelimit.extraLabels array
        additional labels, that will be applied to fetchdata from datasource
        Supports array of values separated by comma or specified via multiple flags.
  -ratelimit.refreshInterval duration
         (default 5s)
  -read.url string
        read access url address, example: http://vmselect:8481
  -tls
        Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
        Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
        Path to file with TLS key. Used only if -tls is set
  -version
        Show VictoriaMetrics version
  -write.url string
        write access url address, example: http://vminsert:8480

```

## TroubleShooting

* Access control:
  * incorrect `jwt` format, try https://jwt.io/#debugger-io with our tokens
  * expired token, check `exp` field.
* Rate Limiting:
  * `scrape_interval` at datasource, reduce it to apply limits faster.


## Limitations

* Access Control:
  * `jwt` token must be validated by external system, currently `vmgateway` can't validate the signature.
* RateLimiting:
  * limits applied based on queries to `datasource.url`
  * only cluster version can be rate-limited.
