---
sort: 5
weight: 5
menu:
  docs:
    parent: 'victoriametrics'
    weight: 5
title: vmauth
aliases:
  - /vmauth.html
---
# vmauth

`vmauth` is a simple auth proxy, router and [load balancer](#load-balancing) for [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics).
It reads auth credentials from `Authorization` http header ([Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication), `Bearer token` and [InfluxDB authorization](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1897) is supported),
matches them against configs pointed by [-auth.config](#auth-config) command-line flag and proxies incoming HTTP requests to the configured per-user `url_prefix` on successful match.
The `-auth.config` can point to either local file or to http url.

## Quick start

Just download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases), unpack it
and pass the following flag to `vmauth` binary in order to start authorizing and routing requests:

```console
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

After that `vmauth` starts accepting HTTP requests on port `8427` and routing them according to the provided [-auth.config](#auth-config).
The port can be modified via `-httpListenAddr` command-line flag.

The auth config can be reloaded via the following ways:

- By passing `SIGHUP` signal to `vmauth`.
- By querying `/-/reload` http endpoint. This endpoint can be protected with `-reloadAuthKey` command-line flag. See [security docs](#security) for more details.
- By specifying `-configCheckInterval` command-line flag to the interval between config re-reads. For example, `-configCheckInterval=5s` will re-read the config
  and apply new changes every 5 seconds.

Docker images for `vmauth` are available [here](https://hub.docker.com/r/victoriametrics/vmauth/tags).
See how `vmauth` used in [docker-compose env](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/README.md#victoriametrics-cluster).

Pass `-help` to `vmauth` in order to see all the supported command-line flags with their descriptions.

Feel free [contacting us](mailto:info@victoriametrics.com) if you need customized auth proxy for VictoriaMetrics with the support of LDAP, SSO, RBAC, SAML,
accounting and rate limiting such as [vmgateway](https://docs.victoriametrics.com/vmgateway.html).

## Load balancing

Each `url_prefix` in the [-auth.config](#auth-config) may contain either a single url or a list of urls.
In the latter case `vmauth` balances load among the configured urls in least-loaded round-robin manner.

If the backend at the configured url isn't available, then `vmauth` tries sending the request to the remaining configured urls.

It is possible to configure automatic retry of requests if the backend responds with status code from optional `retry_status_codes` list.

Load balancing feature can be used in the following cases:

- Balancing the load among multiple `vmselect` and/or `vminsert` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html).
  The following `-auth.config` file can be used for spreading incoming requests among 3 vmselect nodes and re-trying failed requests
  or requests with 500 and 502 response status codes:

  ```yml
  unauthorized_user:
    url_prefix:
    - http://vmselect1:8481/
    - http://vmselect2:8481/
    - http://vmselect3:8481/
    retry_status_codes: [500, 502]
  ```

- Spreading select queries among multiple availability zones (AZs) with identical data. For example, the following config spreads select queries
  among 3 AZs. Requests are re-tried if some AZs are temporarily unavailable or if some `vmstorage` nodes in some AZs are temporarily unavailable.
  `vmauth` adds `deny_partial_response=1` query arg to all the queries in order to guarantee to get full response from every AZ.
  See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-availability) for details.

  ```yml
  unauthorized_user:
    url_prefix:
    - https://vmselect-az1/?deny_partial_response=1
    - https://vmselect-az2/?deny_partial_response=1
    - https://vmselect-az3/?deny_partial_response=1
    retry_status_codes: [500, 502, 503]
  ```

Load balancig can also be configured independently per each user and per each `url_map` entry.
See [auth config docs](#auth-config) for more details.

## Concurrency limiting

`vmauth` limits the number of concurrent requests it can proxy according to the following command-line flags:

- `-maxConcurrentRequests` limits the global number of concurrent requests `vmauth` can serve across all the configured users.
- `-maxConcurrentPerUserRequests` limits the number of concurrent requests `vmauth` can serve per each configured user.

It is also possible to set individual limits on the number of concurrent requests per each user
with the `max_concurrent_requests` option - see [auth config example](#auth-config).

`vmauth` responds with `429 Too Many Requests` HTTP error when the number of concurrent requests exceeds the configured limits.

The following [metrics](#monitoring) related to concurrency limits are exposed by `vmauth`:

- `vmauth_concurrent_requests_capacity` - the global limit on the number of concurrent requests `vmauth` can serve.
  It is set via `-maxConcurrentRequests` command-line flag.
- `vmauth_concurrent_requests_current` - the current number of concurrent requests `vmauth` processes.
- `vmauth_concurrent_requests_limit_reached_total` - the number of requests rejected with `429 Too Many Requests` error
  because of the global concurrency limit has been reached.
- `vmauth_user_concurrent_requests_capacity{username="..."}` - the limit on the number of concurrent requests for the given `username`.
- `vmauth_user_concurrent_requests_current{username="..."}` - the current number of concurrent requests for the given `username`.
- `vmauth_user_concurrent_requests_limit_reached_total{username="foo"}` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for the given `username`.
- `vmauth_unauthorized_user_concurrent_requests_capacity` - the limit on the number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
- `vmauth_unauthorized_user_concurrent_requests_current` - the current number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
- `vmauth_unauthorized_user_concurrent_requests_limit_reached_total` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for unauthorized users (if `unauthorized_user` section is used).


## IP filters

[Enterprise version](https://docs.victoriametrics.com/enterprise.html) of `vmauth` can be configured to allow / deny incoming requests via global and per-user IP filters.

For example, the following config allows requests to `vmauth` from `10.0.0.0/24` network and from `1.2.3.4` IP address, while denying requests from `10.0.0.42` IP address:

```yml
users:
# User configs here

ip_filters:
  allow_list:
  - 10.0.0.0/24
  - 1.2.3.4
  deny_list: [10.0.0.42]
```

The following config allows requests for the user 'foobar' only from the IP `127.0.0.1`:

```yml
users:
- username: "foobar"
  password: "***"
  url_prefix: "http://localhost:8428"
  ip_filters:
    allow_list: [127.0.0.1]
```

See config example of using IP filters [here](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmauth/example_config_ent.yml).

## Auth config

`-auth.config` is represented in the following simple `yml` format:

```yml
# Arbitrary number of usernames may be put here.
# It is possible to set multiple identical usernames with different passwords.
# Such usernames can be differentiated by `name` option.

users:
  # Requests with the 'Authorization: Bearer XXXX' and 'Authorization: Token XXXX'
  # header are proxied to http://localhost:8428 .
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
  # Requests with the Basic Auth username=XXXX are proxied to http://localhost:8428 as well.
- bearer_token: "XXXX"
  url_prefix: "http://localhost:8428"

  # Requests with the 'Authorization: Bearer YYY' header are proxied to http://localhost:8428 ,
  # The `X-Scope-OrgID: foobar` http header is added to every proxied request.
  # The `X-Server-Hostname` http header is removed from the proxied response.
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
- bearer_token: "YYY"
  url_prefix: "http://localhost:8428"
  # extra headers to add to the request or remove from the request (if header value is empty)
  headers:
    - "X-Scope-OrgID: foobar"
  # extra headers to add to the response or remove from the response (if header value is empty)
  response_headers:
    - "X-Server-Hostname:" # empty value means the header will be removed from the response

  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # are proxied to http://localhost:8428 .
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
  #
  # The given user can send maximum 10 concurrent requests according to the provided max_concurrent_requests.
  # Excess concurrent requests are rejected with 429 HTTP status code.
  # See also -maxConcurrentPerUserRequests and -maxConcurrentRequests command-line flags.
- username: "local-single-node"
  password: "***"
  url_prefix: "http://localhost:8428"
  max_concurrent_requests: 10

  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # are proxied to http://localhost:8428 with extra_label=team=dev query arg.
  # For example, http://vmauth:8427/api/v1/query is routed to http://localhost:8428/api/v1/query?extra_label=team=dev
- username: "local-single-node2"
  password: "***"
  url_prefix: "http://localhost:8428?extra_label=team=dev"

  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # are load-balanced among http://vmselect1:8481/select/123/prometheus and http://vmselect2:8481/select/123/prometheus
  # For example, http://vmauth:8427/api/v1/query is proxied to the following urls in a round-robin manner:
  #   - http://vmselect1:8481/select/123/prometheus/api/v1/select
  #   - http://vmselect2:8481/select/123/prometheus/api/v1/select
- username: "cluster-select-account-123"
  password: "***"
  url_prefix:
  - "http://vmselect1:8481/select/123/prometheus"
  - "http://vmselect2:8481/select/123/prometheus"

  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # are load-balanced between http://vminsert1:8480/insert/42/prometheus and http://vminsert2:8480/insert/42/prometheus
  # For example, http://vmauth:8427/api/v1/write is proxied to the following urls in a round-robin manner:
  #   - http://vminsert1:8480/insert/42/prometheus/api/v1/write
  #   - http://vminsert2:8480/insert/42/prometheus/api/v1/write
- username: "cluster-insert-account-42"
  password: "***"
  url_prefix:
  - "http://vminsert1:8480/insert/42/prometheus"
  - "http://vminsert2:8480/insert/42/prometheus"

  # A single user for querying and inserting data:
  #
  # - Requests to http://vmauth:8427/api/v1/query, http://vmauth:8427/api/v1/query_range
  #   and http://vmauth:8427/api/v1/label/<label_name>/values are proxied to the following urls in a round-robin manner:
  #     - http://vmselect1:8481/select/42/prometheus
  #     - http://vmselect2:8481/select/42/prometheus
  #   For example, http://vmauth:8427/api/v1/query is proxied to http://vmselect1:8480/select/42/prometheus/api/v1/query
  #   or to http://vmselect2:8480/select/42/prometheus/api/v1/query .
  #   Requests are re-tried at other url_prefix backends if response status codes match 500 or 502.
  #
  # - Requests to http://vmauth:8427/api/v1/write are proxied to http://vminsert:8480/insert/42/prometheus/api/v1/write .
  #   The "X-Scope-OrgID: abc" http header is added to these requests.
  #   The "X-Server-Hostname" http header is removed from the proxied response.
  #
  # Request which do not match `src_paths` from the `url_map` are proxied to the urls from `default_url`
  # in a round-robin manner. The original request path is passed in `request_path` query arg.
  # For example, request to http://vmauth:8427/non/existing/path are proxied:
  #  - to http://default1:8888/unsupported_url_handler?request_path=/non/existing/path
  #  - or http://default2:8888/unsupported_url_handler?request_path=/non/existing/path
- username: "foobar"
  url_map:
  - src_paths:
    - "/api/v1/query"
    - "/api/v1/query_range"
    - "/api/v1/label/[^/]+/values"
    url_prefix:
    - "http://vmselect1:8481/select/42/prometheus"
    - "http://vmselect2:8481/select/42/prometheus"
    retry_status_codes: [500, 502]
  - src_paths: ["/api/v1/write"]
    url_prefix: "http://vminsert:8480/insert/42/prometheus"
    headers:
    - "X-Scope-OrgID: abc"
    response_headers:
    - "X-Server-Hostname:" # empty value means the header will be removed from the response
    ip_filters:
      deny_list: [127.0.0.1]
  default_url:
  - "http://default1:8888/unsupported_url_handler"
  - "http://default2:8888/unsupported_url_handler"

# Requests without Authorization header are routed according to `unauthorized_user` section.
# Requests are routed in round-robin fashion between `url_prefix` backends.
# The deny_partial_response query arg is added to all the routed requests.
# The requests are re-tried if url_prefix backends send 500 or 503 response status codes.
unauthorized_user:
  url_prefix:
  - http://vmselect-az1/?deny_partial_response=1
  - http://vmselect-az2/?deny_partial_response=1
  retry_status_codes: [503, 500]

ip_filters:
  allow_list: ["1.2.3.0/24", "127.0.0.1"]
  deny_list:
  - 10.1.0.1
```

The config may contain `%{ENV_VAR}` placeholders, which are substituted by the corresponding `ENV_VAR` environment variable values.
This may be useful for passing secrets to the config.

Please note, vmauth doesn't follow redirects. If destination redirects request to a new location, make sure this 
location is supported in vmauth `url_map` config.

## Security

It is expected that all the backend services protected by `vmauth` are located in an isolated private network, so they can be accessed by external users only via `vmauth`.

Do not transfer Basic Auth headers in plaintext over untrusted networks. Enable https. This can be done by passing the following `-tls*` command-line flags to `vmauth`:

```console
  -tls
     Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
     Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
     Path to file with TLS key. Used only if -tls is set
```

Alternatively, [https termination proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy) may be put in front of `vmauth`.

It is recommended protecting the following endpoints with authKeys:
* `/-/reload` with `-reloadAuthKey` command-line flag, so external users couldn't trigger config reload.
* `/flags` with `-flagsAuthKey` command-line flag, so unauthorized users couldn't get application command-line flags.
* `/metrics` with `-metricsAuthKey` command-line flag, so unauthorized users couldn't get access to [vmauth metrics](#monitoring).
* `/debug/pprof` with `-pprofAuthKey` command-line flag, so unauthorized users couldn't get access to [profiling information](#profiling).

`vmauth` also supports the ability to restrict access by IP - see [these docs](#ip-filters). See also [concurrency limiting docs](#concurrency-limiting).

## Monitoring

`vmauth` exports various metrics in Prometheus exposition format at `http://vmauth-host:8427/metrics` page. It is recommended setting up regular scraping of this page
either via [vmagent](https://docs.victoriametrics.com/vmagent.html) or via Prometheus, so the exported metrics could be analyzed later.

`vmauth` exports `vmauth_user_requests_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) metric 
and `vmauth_user_request_duration_seconds_*` [summary](https://docs.victoriametrics.com/keyConcepts.html#summary) metric 
with `username` label. The `username` label value equals to `username` field value set in the `-auth.config` file.
It is possible to override or hide the value in the label by specifying `name` field. 
For example, the following config will result in `vmauth_user_requests_total{username="foobar"}` 
instead of `vmauth_user_requests_total{username="secret_user"}`:

```yml
users:
- username: "secret_user"
  name: "foobar"
  # other config options here
```

For unauthorized users `vmauth` exports `vmauth_unauthorized_user_requests_total` 
[counter](https://docs.victoriametrics.com/keyConcepts.html#counter) metric and 
`vmauth_unauthorized_user_request_duration_seconds_*` [summary](https://docs.victoriametrics.com/keyConcepts.html#summary)
metric without label (if `unauthorized_user` section of config is used).

## How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) - `vmauth` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.20.
1. Run `make vmauth` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmauth` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmauth-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmauth-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmauth`. It builds `victoriametrics/vmauth:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmauth`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```console
ROOT_IMAGE=scratch make package-vmauth
```

## Profiling

`vmauth` provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile. It can be collected with the following command (replace `0.0.0.0` with hostname if needed):

<div class="with-copy" markdown="1">

```console
curl http://0.0.0.0:8427/debug/pprof/heap > mem.pprof
```

</div>

* CPU profile. It can be collected with the following command (replace `0.0.0.0` with hostname if needed):

<div class="with-copy" markdown="1">

```console
curl http://0.0.0.0:8427/debug/pprof/profile > cpu.pprof
```

</div>

The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).
It is safe sharing the collected profiles from security point of view, since they do not contain sensitive information.

## Advanced usage

Pass `-help` command-line arg to `vmauth` in order to see all the configuration options:

```console
./vmauth -help

vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmauth.html .

  -auth.config string
     Path to auth config. It can point either to local file or to http url. See https://docs.victoriametrics.com/vmauth.html for details on the format of this auth config
  -configCheckInterval duration
     interval for config file re-read. Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -failTimeout duration
     Sets a delay period for load balancing to skip a malfunctioning backend. (defaults 3s)
  -flagsAuthKey string
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
     Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
     The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
     An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
     Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
     Password for HTTP server's Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
     Username for HTTP server's Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
     TCP address to listen for http connections. See also -httpListenAddr.useProxyProtocol (default ":8427")
  -httpListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
  -internStringCacheExpireDuration duration
     The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
     Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
     The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -license string
     See https://victoriametrics.com/products/enterprise/ for trial license. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -license.forceOffline
     See https://victoriametrics.com/products/enterprise/ for trial license. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -licenseFile string
     See https://victoriametrics.com/products/enterprise/ for trial license. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
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
  -loggerOutput string
     Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
     Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
     Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxConcurrentPerUserRequests int
     The maximum number of concurrent requests vmauth can process per each configured user. Other requests are rejected with '429 Too Many Requests' http status code. See also -maxConcurrentRequests command-line option and max_concurrent_requests option in per-user config (default 300)
  -maxConcurrentRequests int
     The maximum number of concurrent requests vmauth can process. Other requests are rejected with '429 Too Many Requests' http status code. See also -maxConcurrentPerUserRequests and -maxIdleConnsPerBackend command-line options (default 1000)
  -maxIdleConnsPerBackend int
     The maximum number of idle connections vmauth can open per each backend host. See also -maxConcurrentRequests (default 100)
  -maxRequestBodySizeToRetry size
     The maximum request body size, which can be cached and re-tried at other backends. Bigger values may require more memory
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 16384)
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pprofAuthKey string
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.interval duration
     Interval for pushing metrics to -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
  -reloadAuthKey string
     Auth key for /-/reload http endpoint. It must be passed as authKey=...
  -responseTimeout duration
     The timeout for receiving a response from backend (default 5m0s)
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
     Show VictoriaMetrics version
```
