## vmauth

`vmauth` is a simple auth proxy and router for [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics).
It reads username and password from [Basic Auth headers](https://en.wikipedia.org/wiki/Basic_access_authentication),
matches them against configs pointed by `-auth.config` command-line flag and proxies incoming HTTP requests to the configured per-user `url_prefix` on successful match.


### Quick start

Just download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases), unpack it
and pass the following flag to `vmauth` binary in order to start authorizing and routing requests:

```
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

After that `vmauth` starts accepting HTTP requests on port `8427` and routing them according to the provided [-auth.config](#auth-config).
The port can be modified via `-httpListenAddr` command-line flag.

The auth config can be reloaded by passing `SIGHUP` signal to `vmauth`.

Docker images for `vmauth` are available [here](https://hub.docker.com/r/victoriametrics/vmauth/tags).

Pass `-help` to `vmauth` in order to see all the supported command-line flags with their descriptions.

Feel free [contacting us](mailto:info@victoriametrics.com) if you need customized auth proxy for VictoriaMetrics with the support of LDAP, SSO, RBAC, SAML, accounting, limits, etc.


### Auth config

Auth config is represented in the following simple `yml` format:

```yml

# Arbitrary number of usernames may be put here.
# Usernames must be unique.

users:

  # The user for querying local single-node VictoriaMetrics.
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be routed to http://localhost:8428 .
  # For example, http://vmauth:8427/api/v1/query is routed to http://localhost:8428/api/v1/query
- username: "local-single-node"
  password: "***"
  url_prefix: "http://localhost:8428"

  # The user for querying account 123 in VictoriaMetrics cluster
  # See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md#url-format
  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be routed to http://vmselect:8481/select/123/prometheus .
  # For example, http://vmauth:8427/api/v1/query is routed to http://vmselect:8481/select/123/prometheus/api/v1/select
- username: "cluster-select-account-123"
  password: "***"
  url_prefix: "http://vmselect:8481/select/123/prometheus"

  # The user for inserting Prometheus data into VictoriaMetrics cluster under account 42
  # See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md#url-format
  # All the reuqests to http://vmauth:8427 with the given Basic Auth (username:password)
  # will be routed to http://vminsert:8480/insert/42/prometheus .
  # For example, http://vmauth:8427/api/v1/write is routed to http://vminsert:8480/insert/42/prometheus/api/v1/write
- username: "cluster-insert-account-42"
  password: "***"
  url_prefix: "http://vminsert:8480/insert/42/prometheus"
```


### Security

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


### Monitoring

`vmauth` exports various metrics in Prometheus exposition format at `http://vmauth-host:8427/metrics` page. It is recommended setting up regular scraping of this page
either via [vmagent](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmagent/README.md) or via Prometheus, so the exported metrics could be analyzed later.


### How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) - `vmauth` is located in `vmutils-*` archives there.


#### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.13.
2. Run `make vmauth` from the root folder of the repository.
   It builds `vmauth` binary and puts it into the `bin` folder.

#### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmauth-prod` from the root folder of the repository.
   It builds `vmauth-prod` binary and puts it into the `bin` folder.

#### Building docker images

Run `make package-vmauth`. It builds `victoriametrics/vmauth:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmauth`.

By default the image is built on top of [alpine](https://hub.docker.com/_/alpine) image. It is possible to build the package on top of any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package-vmauth
```


### Profiling

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
