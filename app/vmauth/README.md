# vmauth

`vmauth` is a simple auth proxy, router and [load balancer](#load-balancing) for [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics).
It reads auth credentials from `Authorization` http header ([Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication) and `Bearer token` is supported),
matches them against configs pointed by [-auth.config](#auth-config) command-line flag and proxies incoming HTTP requests to the configured per-user `url_prefix` on successful match.
The `-auth.config` can point to either local file or to http url.

## Quick start

Just download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases), unpack it
and pass the following flag to `vmauth` binary in order to start authorizing and routing requests:

```
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

After that `vmauth` starts accepting HTTP requests on port `8427` and routing them according to the provided [-auth.config](#auth-config).
The port can be modified via `-httpListenAddr` command-line flag.

The auth config can be reloaded either by passing `SIGHUP` signal to `vmauth` or by querying `/-/reload` http endpoint.

Docker images for `vmauth` are available [here](https://hub.docker.com/r/victoriametrics/vmauth/tags).

Pass `-help` to `vmauth` in order to see all the supported command-line flags with their descriptions.

Feel free [contacting us](mailto:info@victoriametrics.com) if you need customized auth proxy for VictoriaMetrics with the support of LDAP, SSO, RBAC, SAML,
accounting and rate limiting such as [vmgateway](https://docs.victoriametrics.com/vmgateway.html).

## Load balancing

Each `url_prefix` in the [-auth.config](#auth-config) may contain either a single url or a list of urls. In the latter case `vmauth` balances load among the configured urls in a round-robin manner. This feature is useful for balancing the load among multiple `vmselect` and/or `vminsert` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html).

## Auth config

`-auth.config` is represented in the following simple `yml` format:

```yml
# Arbitrary number of usernames may be put here.
# Username and bearer_token values must be unique.

users:
  # Requests with the 'Authorization: Bearer XXXX' header are proxied to http://localhost:8428 .
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
  # Requests with the Basic Auth username=XXXX are proxied to http://localhost:8428 as well.
- bearer_token: "XXXX"
  url_prefix: "http://localhost:8428"

  # Requests with the 'Authorization: Bearer YYY' header are proxied to http://localhost:8428 ,
  # The `X-Scope-OrgID: foobar` http header is added to every proxied request.
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
- bearer_token: "YYY"
  url_prefix: "http://localhost:8428"
  headers:
  - "X-Scope-OrgID: foobar"

  # The user for querying local single-node VictoriaMetrics.
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be proxied to http://localhost:8428 .
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
- username: "local-single-node"
  password: "***"
  url_prefix: "http://localhost:8428"

  # The user for querying local single-node VictoriaMetrics with extra_label team=dev.
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be routed to http://localhost:8428 with extra_label=team=dev query arg.
  # For example, http://vmauth:8427/api/v1/query is routed to http://localhost:8428/api/v1/query?extra_label=team=dev
- username: "local-single-node"
  password: "***"
  url_prefix: "http://localhost:8428?extra_label=team=dev"

  # The user for querying account 123 in VictoriaMetrics cluster
  # See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be load-balanced among http://vmselect1:8481/select/123/prometheus and http://vmselect2:8481/select/123/prometheus
  # For example, http://vmauth:8427/api/v1/query is proxied to the following urls in a round-robin manner:
  #   - http://vmselect1:8481/select/123/prometheus/api/v1/select
  #   - http://vmselect2:8481/select/123/prometheus/api/v1/select
- username: "cluster-select-account-123"
  password: "***"
  url_prefix:
  - "http://vmselect1:8481/select/123/prometheus"
  - "http://vmselect2:8481/select/123/prometheus"

  # The user for inserting Prometheus data into VictoriaMetrics cluster under account 42
  # See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be load-balanced between http://vminsert1:8480/insert/42/prometheus and http://vminsert2:8480/insert/42/prometheus
  # For example, http://vmauth:8427/api/v1/write is proxied to the following urls in a round-robin manner:
  #   - http://vminsert1:8480/insert/42/prometheus/api/v1/write
  #   - http://vminsert2:8480/insert/42/prometheus/api/v1/write
- username: "cluster-insert-account-42"
  password: "***"
  url_prefix:
  - "http://vminsert1:8480/insert/42/prometheus"
  - "http://vminsert2:8480/insert/42/prometheus"

  # A single user for querying and inserting data:
  # - Requests to http://vmauth:8427/api/v1/query, http://vmauth:8427/api/v1/query_range
  #   and http://vmauth:8427/api/v1/label/<label_name>/values are proxied to the following urls in a round-robin manner:
  #     - http://vmselect1:8481/select/42/prometheus
  #     - http://vmselect2:8481/select/42/prometheus
  #   For example, http://vmauth:8427/api/v1/query is proxied to http://vmselect1:8480/select/42/prometheus/api/v1/query
  #   or to http://vmselect2:8480/select/42/prometheus/api/v1/query .
  # - Requests to http://vmauth:8427/api/v1/write are proxied to http://vminsert:8480/insert/42/prometheus/api/v1/write .
  #   The "X-Scope-OrgID: abc" http header is added to these requests.
- username: "foobar"
  url_map:
  - src_paths:
    - "/api/v1/query"
    - "/api/v1/query_range"
    - "/api/v1/label/[^/]+/values"
    url_prefix:
    - "http://vmselect1:8481/select/42/prometheus"
    - "http://vmselect2:8481/select/42/prometheus"
  - src_paths: ["/api/v1/write"]
    url_prefix: "http://vminsert:8480/insert/42/prometheus"
    headers:
    - "X-Scope-OrgID: abc"
```

The config may contain `%{ENV_VAR}` placeholders, which are substituted by the corresponding `ENV_VAR` environment variable values.
This may be useful for passing secrets to the config.

## Security

Do not transfer Basic Auth headers in plaintext over untrusted networks. Enable https. This can be done by passing the following `-tls*` command-line flags to `vmauth`:

```
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
```

Alternatively, [https termination proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy) may be put in front of `vmauth`.

It is recommended protecting `/-/reload` endpoint with `-reloadAuthKey` command-line flag, so external users couldn't trigger config reload.

## Monitoring

`vmauth` exports various metrics in Prometheus exposition format at `http://vmauth-host:8427/metrics` page. It is recommended setting up regular scraping of this page
either via [vmagent](https://docs.victoriametrics.com/vmagent.html) or via Prometheus, so the exported metrics could be analyzed later.

`vmauth` exports `vmauth_user_requests_total` metric with `username` label. The `username` label value equals to `username` field value set in the `-auth.config` file. It is possible to override or hide the value in the label by specifying `name` field. For example, the following config will result in `vmauth_user_requests_total{username="foobar"}` instead of `vmauth_user_requests_total{username="secret_user"}`:

```yml
users:
- username: "secret_user"
  name: "foobar"
  # other config options here
```

## How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) - `vmauth` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make vmauth` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmauth` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmauth-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmauth-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmauth`. It builds `victoriametrics/vmauth:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmauth`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package-vmauth
```

## Profiling

`vmauth` provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile. It can be collected with the following command:

```bash
curl -s http://<vmauth-host>:8427/debug/pprof/heap > mem.pprof
```

* CPU profile. It can be collected with the following command:

```bash
curl -s http://<vmauth-host>:8427/debug/pprof/profile > cpu.pprof
```

The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).

## Advanced usage

Pass `-help` command-line arg to `vmauth` in order to see all the configuration options:

```
./vmauth -help

vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmauth.html .

  -auth.config string
    	Path to auth config. It can point either to local file or to http url. See https://docs.victoriametrics.com/vmauth.html for details on the format of this auth config
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
    	Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
    	Disable compression of HTTP responses to save CPU resources. By default compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
    	Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
    	The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
    	An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
    	Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
    	Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
    	Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
    	TCP address to listen for http connections (default ":8427")
  -logInvalidAuthTokens
    	Whether to log requests with invalid auth tokens. Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page
  -loggerDisableTimestamps
    	Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
    	Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit
  -loggerFormat string
    	Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
    	Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
    	Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
    	Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxIdleConnsPerBackend int
    	The maximum number of idle connections vmauth can open per each backend host (default 100)
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
    	Auth key for /metrics. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pprofAuthKey string
    	Auth key for /debug/pprof. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -reloadAuthKey string
    	Auth key for /-/reload http endpoint. It must be passed as authKey=...
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
```
