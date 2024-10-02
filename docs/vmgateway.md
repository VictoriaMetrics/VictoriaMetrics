---
weight: 9
menu:
  docs:
    parent: victoriametrics
    weight: 9
title: vmgateway
aliases:
  - /vmgateway.html
---
***vmgateway is a part of [enterprise package](https://docs.victoriametrics.com/enterprise/). 
It is available for download and evaluation at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).***

![vmgateway](vmgateway-overview.webp)

`vmgateway` is a proxy for the VictoriaMetrics Time Series Database (TSDB). It provides the following features:

* Rate Limiter
  * Based on cluster tenant's utilization, it supports multiple time interval limits for both the ingestion and retrieval of metrics
* Token Access Control
  * Supports additional per-label access control for both the Single and Cluster versions of the VictoriaMetrics TSDB
  * Provides access by tenantID in the Cluster version
  * Allows for separate write/read/admin access to data

`vmgateway` is included in our [enterprise packages](https://docs.victoriametrics.com/enterprise/).

## Access Control

![vmgateway-ac](vmgateway-access-control.webp)

`vmgateway` supports jwt based authentication. With jwt payload can be configured to give access to specific tenants and labels as well as to read/write.

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
      "extra_filters": ["{env=~\"prod|dev\",team!=\"test\"}"],
      "mode": 1
  }
}
```

Where:

* `exp` - required, expire time in unix_timestamp. If the token expires then `vmgateway` rejects the request.
* `vm_access` - required, dict with claim info, minimum form: `{"vm_access": {"tenant_id": {}}`
* `tenant_id` - optional, for cluster mode, routes requests to the corresponding tenant.
* `extra_labels` - optional, key-value pairs for label filters added to the ingested or selected metrics. Multiple filters are added with `and` operation. If defined, `extra_label` from original request removed.
* `extra_filters` - optional, [series selectors](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) added to the select query requests. Multiple selectors are added with `or` operation. If defined, `extra_filter` from original request removed.
* `mode` - optional, access mode for api - read, write, or full. Supported values: 0 - full (default value), 1 - read, 2 - write.

## QuickStart

Start the single version of VictoriaMetrics

```sh
# single
# start node
./bin/victoria-metrics --selfScrapeInterval=10s
```

Start vmgateway

```sh
./bin/vmgateway -eula -enable.auth -read.url http://localhost:8428 --write.url http://localhost:8428
```

Retrieve data from the database

```sh
curl 'http://localhost:8431/api/v1/series/count' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ2bV9hY2Nlc3MiOnsidGVuYW50X2lkIjp7fSwicm9sZSI6MX0sImV4cCI6MTkzOTM0NjIxMH0.5WUxEfdcV9hKo4CtQdtuZYOGpGXWwaqM9VuVivMMrVg'
```

A request with an incorrect token or without any token will be rejected:

```sh
curl 'http://localhost:8431/api/v1/series/count'

curl 'http://localhost:8431/api/v1/series/count' -H 'Authorization: Bearer incorrect-token'
```

## Rate Limiter

![vmgateway-rl](vmgateway-rate-limiting.webp)

Limits incoming requests by given, pre-configured limits. It supports read and write limiting by tenant.

`vmgateway` needs a datasource for rate limit queries. It can be either single-node or cluster version of `victoria-metrics`.
The metrics that you want to rate limit must be scraped from the cluster.

List of supported limit types:

* `queries` - count of api requests made at tenant to read the api, such as `/api/v1/query`, `/api/v1/series` and others.
* `active_series` - count of current active series at any given tenant.
* `new_series` - count of created series; aka churn rate
* `rows_inserted` - count of inserted rows per tenant.

List of supported time windows:

* `minute`
* `hour`

Limits can be specified per tenant or at a global level if you omit `project_id` and `account_id`.

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

cluster version of VictoriaMetrics is required for rate limiting.

```sh
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

# create base rate limiting config:
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

## JWT signature verification

`vmgateway` supports JWT signature verification.

Supported algorithms are `RS256`, `RS384`, `RS512`, `ES256`, `ES384`, `ES512`, `PS256`, `PS384`, `PS512`.
Tokens with unsupported algorithms will be rejected.

In order to enable JWT signature verification, you need to specify keys for signature verification.
The following flags are used to specify keys:
- `-auth.publicKeyFiles` - allows to pass file path to file with public key.
- `-auth.publicKeys` - allows to pass public key directly.

Note that both flags support passing multiple keys and also can be used together.

Example usage:
```sh
./bin/vmgateway -eula \
  -enable.auth \
  -write.url=http://localhost:8480 \
  -read.url=http://localhost:8481 \
  -auth.publicKeyFiles=public_key.pem \
  -auth.publicKeyFiles=public_key2.pem \
  -auth.publicKeys=`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu1SU1LfVLPHCozMxH2Mo
4lgOEePzNm0tRgeLezV6ffAt0gunVTLw7onLRnrq0/IzW7yWR7QkrmBL7jTKEn5u
+qKhbwKfBstIs+bMY2Zkp18gnTxKLxoS2tFczGkPLPgizskuemMghRniWaoLcyeh
kd3qqGElvW/VDL5AaWTg0nLVkjRo9z+40RQzuVaE8AkAFmxZzow3x+VJYKdjykkJ
0iT9wCS0DRTXu269V264Vf/3jvredZiKRkgwlL9xNAwxXFg0x/XFw005UWVRIkdg
cKWTjpBP2dPwVZ4WWC+9aGVd+Gyn1o0CLelf4rEjGoXbAAEgAqeGUxrcIlbjXfbc
mwIDAQAB
-----END PUBLIC KEY-----
`
```
This command will result in 3 keys loaded: 2 keys from files and 1 from command line.

### Using OpenID discovery endpoint for JWT signature verification

`vmgateway` supports using OpenID discovery endpoint for JWKS keys discovery.

In order to enable [OpenID discovery](https://openid.net/specs/openid-connect-discovery-1_0.html) endpoint for JWT signature verification, you need to specify OpenID discovery endpoint URLs by using `auth.oidcDiscoveryEndpoints` flag.
When `auth.oidcDiscoveryEndpoints` is specified `vmgateway` will fetch JWKS keys from the specified endpoint and use them for JWT signature verification.

Example usage for tokens issued by Azure Active Directory:
```sh
/bin/vmgateway -eula \
  -enable.auth \
  -write.url=http://localhost:8480 \
  -read.url=http://localhost:8481 \
  -auth.oidcDiscoveryEndpoints=https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration
```

Example usage for tokens issued by Google:
```sh
/bin/vmgateway -eula \
  -enable.auth \
  -write.url=http://localhost:8480 \
  -read.url=http://localhost:8481 \
  -auth.oidcDiscoveryEndpoints=https://accounts.google.com/.well-known/openid-configuration
```

### Using JWKS endpoint for JWT signature verification

`vmgateway` supports using JWKS endpoint for JWT signature verification.

In order to enable JWKS endpoint for JWT signature verification, you need to specify JWKS endpoint URL by using `auth.jwksEndpoints` flag.
When `auth.jwksEndpoints` is specified `vmgateway` will fetch public keys from the specified endpoint and use them for JWT signature verification.

Example usage for tokens issued by Azure Active Directory:
```sh
/bin/vmgateway -eula \
  -enable.auth \
  -write.url=http://localhost:8480 \
  -read.url=http://localhost:8481 \
  -auth.jwksEndpoints=https://login.microsoftonline.com/common/discovery/v2.0/keys
```

Example usage for tokens issued by Google:
```sh
/bin/vmgateway -eula \
  -enable.auth \
  -write.url=http://localhost:8480 \
  -read.url=http://localhost:8481 \
  -auth.jwksEndpoints=https://www.googleapis.com/oauth2/v3/certs
```

## Configuration

Below is the list of configuration flags (it can be viewed by running `./vmgateway -help`):

```sh
  -auth.httpHeader string
     HTTP header name to look for JWT authorization token (default "Authorization")
  -auth.jwksEndpoints array
     JWKS endpoints to fetch keys for JWT tokens signature verification
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -auth.oidcDiscoveryEndpoints array
     OpenID Connect discovery endpoints to fetch keys for JWT tokens signature verification
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -auth.publicKeyFiles array
     Path file with public key to verify JWT token signature
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -auth.publicKeys array
     Public keys to verify JWT token signature
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -clusterMode
     enable this for the cluster version
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
     Whether to disable adding 'step' param to the issued instant queries. This might be useful when using vmalert with datasources that do not support 'step' param for instant queries, like Google Managed Prometheus. It is not recommended to enable this flag if you use vmalert with VictoriaMetrics.
  -datasource.headers string
     Optional HTTP extraHeaders to send with each request to the corresponding -datasource.url. For example, -datasource.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -datasource.url. Multiple headers must be delimited by '^^': -datasource.headers='header1:value1^^header2:value2'
  -datasource.lookback duration
     Deprecated: please adjust "-search.latencyOffset" at datasource side or specify "latency_offset" in rule group's params. Lookback defines how far into the past to look when evaluating queries. For example, if the datasource.lookback=5m then param "time" with value now()-5m will be added to every query.
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
     How far a value can fallback to when evaluating queries. For example, if -datasource.queryStep=15s then param "step" with value "15s" will be added to every query. If set to 0, rule's evaluation interval will be used instead. (default 5m0s)
  -datasource.queryTimeAlignment
     Deprecated: please use "eval_alignment" in rule group instead. Whether to align "time" parameter with evaluation interval. Alignment supposed to produce deterministic results despite number of vmalert replicas or time they were started. See more details at https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1257 (default true)
  -datasource.roundDigits int
     Adds "round_digits" GET param to datasource requests. In VM "round_digits" limits the number of digits after the decimal point in response values.
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
     Datasource compatible with Prometheus HTTP API. It can be single node VictoriaMetrics or vmselect URL. Required parameter. E.g. http://127.0.0.1:8428 . See also -remoteRead.disablePathAppend and -datasource.showURL
  -enable.auth
     enables auth with jwt token
  -enable.rateLimit
     enables rate limiter
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming connections to -httpListenAddr are closed after the configured timeout. This may help evenly spreading load among a cluster of services behind TCP-level load balancer. Zero value disables closing of incoming connections (default 2m0s)
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
     TCP address to listen for incoming http requests. See also -httpListenAddr.useProxyProtocol
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -httpListenAddr.useProxyProtocol array
     Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
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
  -logInvalidAuthTokens
     Whether to log requests with invalid auth tokens. Such requests are always counted at vmgateway_auth_rejects_total metric, which is exposed at /metrics page. Note that this option is expected to be used only for debugging purposes, since it may leak sensitive information to logs.
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
     The maximum length of a single logged argument. Longer arguments are replaced with 'arg_start..arg_end', where 'arg_start' and 'arg_end' is prefix and suffix of the arg with the length not exceeding -loggerMaxArgLen / 2 (default 1000)
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
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
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
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -ratelimit.config string
     path for configuration file. Accepts url address
  -ratelimit.configCheckInterval duration
     interval for config file re-read. Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.
  -ratelimit.extraLabels array
     additional labels, that will be applied to fetchdata from datasource
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -ratelimit.refreshInterval duration
      (default 5s)
  -read.url string
     read access url address, example: http://vmselect:8481
  -remoteRead.disablePathAppend
     Whether to disable automatic appending of '/api/v1/query' path to the configured -datasource.url and -remoteRead.url
  -tls array
     Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir .This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
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
  -write.url string
     write access url address, example: http://vminsert:8480
```

## TroubleShooting

* Access control:
  * incorrect `jwt` format, try <https://jwt.io/#debugger-io> with our tokens
  * expired token, check `exp` field.
* Rate Limiting:
  * `scrape_interval` at datasource, reduce it to apply limits faster.

## Limitations

* Access Control:
  * `jwt` token signature verification for `HMAC` algorithms is not supported.
* RateLimiting:
  * limits applied based on queries to `datasource.url`
  * only cluster version can be rate-limited.
