---
weight: 5
menu:
  docs:
    parent: victoriametrics
    weight: 5
title: vmauth
tags:
  - metrics
aliases:
  - /vmauth.html
  - /vmauth/index.html
  - /vmauth/
---
`vmauth` is an HTTP proxy, which can [authorize](https://docs.victoriametrics.com/victoriametrics/vmauth/#authorization), [route](https://docs.victoriametrics.com/victoriametrics/vmauth/#routing) and [load balance](https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing) requests across [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) components or any other HTTP backends.

## Quick start

Just download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest), unpack it
and pass the following flag to `vmauth` binary in order to start authorizing and proxying requests:

```sh
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

The `-auth.config` command-line flag must point to valid [config](#auth-config). See [use cases](#use-cases) with typical `-auth.config` examples.

`vmauth` accepts HTTP requests on port `8427` and proxies them according to the provided [-auth.config](#auth-config).
The port can be modified via `-httpListenAddr` command-line flag.

See [how to reload config without restart](#config-reload).

Docker images for `vmauth` are available at [Docker Hub](https://hub.docker.com/r/victoriametrics/vmauth/tags) and [Quay](https://quay.io/repository/victoriametrics/vmauth?tab=tags).
See how `vmauth` is used in [docker-compose env](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/README.md#victoriametrics-cluster).

Pass `-help` to `vmauth` in order to see all the supported command-line flags with their descriptions.

Feel free to [contact us](mailto:info@victoriametrics.com) if you need customized auth proxy for VictoriaMetrics with the support of LDAP, SSO, RBAC, SAML, accounting and rate limiting such as [vmgateway](https://docs.victoriametrics.com/victoriametrics/vmgateway/).

## Use cases

* [Simple HTTP proxy](#simple-http-proxy)
* [Generic HTTP proxy for different backends](#generic-http-proxy-for-different-backends)
* [Generic HTTP load balancer](#generic-http-load-balancer)
* [Load balancer for vmagent](#load-balancer-for-vmagent)
* [Load balancer for VictoriaMetrics cluster](#load-balancer-for-victoriametrics-cluster)
* [High availability](#high-availability)
* [TLS termination proxy](#tls-termination-proxy)
* [Basic Auth proxy](#basic-auth-proxy)
* [Bearer Token auth proxy](#bearer-token-auth-proxy)
* [Per-tenant authorization](#per-tenant-authorization)
* [mTLS-based request routing](#mtls-based-request-routing)
* [Enforcing query args](#enforcing-query-args)

### Simple HTTP proxy

The following [`-auth.config`](#auth-config) instructs `vmauth` to proxy all the incoming requests to the given backend.
For example, requests to `http://vmauth:8427/foo/bar` are proxied to `http://backend/foo/bar`:

```yaml
unauthorized_user:
  url_prefix: "http://backend/"
```

`vmauth` can balance load among multiple backends - see [these docs](#load-balancing) for details.

See also [authorization](#authorization) and [routing](#routing) docs.

### Generic HTTP proxy for different backends

`vmauth` can proxy requests to different backends depending on the requested path, [query args](https://en.wikipedia.org/wiki/Query_string) and any HTTP request header.

For example, the following [`-auth.config`](#auth-config) instructs `vmauth` to make the following:

* Requests starting with `/app1/` are proxied to `http://app1-backend/`, while the `/app1/` path prefix is dropped according to [`drop_src_path_prefix_parts`](#dropping-request-path-prefix).
  For example, the request to `http://vmauth:8427/app1/foo/bar?baz=qwe` is proxied to `http://app1-backend/foo/bar?baz=qwe`.
* Requests starting with `/app2/` are proxied to `http://app2-backend/`, while the `/app2/` path prefix is dropped according to [`drop_src_path_prefix_parts`](#dropping-request-path-prefix).
  For example, the request to `http://vmauth:8427/app2/index.html` is proxied to `http://app2-backend/index.html`.
* Other requests are proxied to `http://default-backed/`.

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/app1/.*"
    drop_src_path_prefix_parts: 1
    url_prefix: "http://app1-backend/"
  - src_paths:
    - "/app2/.*"
    drop_src_path_prefix_parts: 1
    url_prefix: "http://app2-backend/"
  url_prefix: "http://default-backed/"
```

Sometimes it is needed to proxy all the requests, which do not match `url_map`, to a special `404` page, which could count invalid requests.
Use `default_url` for this case. For example, the following [`-auth.config`](#auth-config) instructs `vmauth` to send all the requests,
which do not match `url_map`, to the `http://some-backend/404-page.html` page. The requested path is passed via `request_path` query arg.
For example, the request to `http://vmauth:8427/foo/bar?baz=qwe` is proxied to `http://some-backend/404-page.html?request_path=%2Ffoo%2Fbar%3Fbaz%3Dqwe`.

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/app1/.*"
    drop_src_path_prefix_parts: 1
    url_prefix: "http://app1-backend/"
  - src_paths:
    - "/app2/.*"
    drop_src_path_prefix_parts: 1
    url_prefix: "http://app2-backend/"
  default_url: "http://some-backend/404-page.html"
```

See [routing docs](#routing) for details.

See also [authorization](#authorization) and [load balancing](#load-balancing) docs.

### Generic HTTP load balancer

`vmauth` can balance load among multiple HTTP backends in least-loaded round-robin mode.
For example, the following [`-auth.config`](#auth-config) instructs `vmauth` to spread load among multiple application instances:

```yaml
unauthorized_user:
  url_prefix:
  - "http://app-instance-1/"
  - "http://app-instance-2/"
  - "http://app-instance-3/"
```

See [load balancing docs](#load-balancing) for more details.

See also [authorization](#authorization) and [routing](#routing) docs.

### Load balancer for vmagent

If [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) is used for processing [data push requests](https://docs.victoriametrics.com/victoriametrics/vmagent/#how-to-push-data-to-vmagent), then it is possible to scale the performance of data processing at `vmagent` by spreading the load among multiple identically configured `vmagent` instances.
This can be done with the following [config](#auth-config) for `vmauth`:

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/prometheus/api/v1/write"
    - "/influx/write"
    - "/api/v1/import"
    - "/api/v1/import/.*"
    url_prefix:
    - "http://vmagent-1:8429/"
    - "http://vmagent-2:8429/"
    - "http://vmagent-3:8429/"
```

See [load balancing docs](#load-balancing) for more details.

See also [authorization](#authorization) and [routing](#routing) docs.

### Load balancer for VictoriaMetrics cluster

[VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) accepts incoming data via `vminsert` nodes and processes incoming requests via `vmselect` nodes according to [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview).
`vmauth` can be used for balancing both `insert` and `select` requests among `vminsert` and `vmselect` nodes, when the following [`-auth.config`](#auth-config) is used:

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/insert/.*"
    url_prefix:
    - "http://vminsert-1:8480/"
    - "http://vminsert-2:8480/"
    - "http://vminsert-3:8480/"
  - src_paths:
    - "/select/.*"
    - "/admin/.*"
    url_prefix:
    - "http://vmselect-1:8481/"
    - "http://vmselect-2:8481/"
```

See [load balancing docs](#load-balancing) for more details.

See also [authorization](#authorization) and [routing](#routing) docs.

### High availability

`vmauth` automatically switches from temporarily unavailable backend to other hot standby backends listed in `url_prefix`
if it runs with `-loadBalancingPolicy=first_available` command-line flag. The load balancing policy can be overridden at `user` and `url_map` sections of [`-auth.config`](#auth-config) via `load_balancing_policy` option. For example, the following config instructs `vmauth` to proxy requests to `http://victoria-metrics-main:8428/` backend.
If this backend becomes unavailable, then `vmauth` starts proxying requests to `http://victoria-metrics-standby1:8428/`.
If this backend becomes also unavailable, then requests are proxied to the last specified backend - `http://victoria-metrics-standby2:8428/`:

```yaml
unauthorized_user:
  url_prefix:
  - "http://victoria-metrics-main:8428/"
  - "http://victoria-metrics-standby1:8428/"
  - "http://victoria-metrics-standby2:8428/"
  load_balancing_policy: first_available
```

See [load-balancing docs](#load-balancing) for more details.

See also [authorization](#authorization) and [routing](#routing) docs.

### TLS termination proxy

`vmauth` can terminate HTTPS requests to backend services when it runs with the following command-line flags:

```sh
/path/to/vmauth -tls -tlsKeyFile=/path/to/tls_key_file -tlsCertFile=/path/to/tls_cert_file -httpListenAddr=0.0.0.0:443
```

* `-httpListenAddr` sets the address to listen for incoming HTTPS requests
* `-tls` enables accepting TLS connections at `-httpListenAddr`
* `-tlsKeyFile` sets the path to TLS certificate key file
* `-tlsCertFile` sets the path to TLS certificate file

See also [automatic issuing of TLS certificates](#automatic-issuing-of-tls-certificates).

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

### Basic Auth proxy

`vmauth` can authorize access to backends depending on the provided [Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication) request headers.
For example, the following [config](#auth-config) proxies requests to [single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
if they contain Basic Auth header with the given `username` and `password`:

```yaml
users:
- username: foo
  password: bar
  url_prefix: "http://victoria-metrics:8428/"
```

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

### Bearer Token auth proxy

`vmauth` can authorize access to backends depending on the provided `Bearer Token` request headers.
For example, the following [config](#auth-config) proxies requests to [single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
if they contain the given `bearer_token`:

```yaml
users:
- bearer_token: ABCDEF
  url_prefix: "http://victoria-metrics:8428/"
```

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

### Per-tenant authorization

The following [`-auth.config`](#auth-config) instructs proxying `insert` and `select` requests from the [Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication) user `tenant1` to the [tenant](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy) `1`, while requests from the user `tenant2` are sent to tenant `2`:

```yaml
users:
- username: tenant1
  password: "***"
  url_map:
  - src_paths:
    - "/api/v1/write"
    url_prefix: "http://vminsert-backend:8480/insert/1/prometheus/"
  - src_paths:
    - "/api/v1/query"
    - "/api/v1/query_range"
    - "/api/v1/series"
    - "/api/v1/labels"
    - "/api/v1/label/.+/values"
    url_prefix: "http://vmselect-backend:8481/select/1/prometheus/"
- username: tenant2
  password: "***"
  url_map:
  - src_paths:
    - "/api/v1/write"
    url_prefix: "http://vminsert-backend:8480/insert/2/prometheus/"
  - src_paths:
    - "/api/v1/query"
    - "/api/v1/query_range"
    - "/api/v1/series"
    - "/api/v1/labels"
    - "/api/v1/label/.+/values"
    url_prefix: "http://vmselect-backend:8481/select/2/prometheus/"
```

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

### mTLS-based request routing

[Enterprise version of `vmauth`](https://docs.victoriametrics.com/victoriametrics/enterprise/) can be configured for routing requests
to different backends depending on the following [subject fields](https://en.wikipedia.org/wiki/Public_key_certificate#Common_fields) in the TLS certificate provided by client:

* `organizational_unit` aka `OU`
* `organization` aka `O`
* `common_name` aka `CN`

For example, the following [`-auth.config`](#auth-config) routes requests from clients with `organizational_unit: finance` TLS certificates to `http://victoriametrics-finance:8428` backend, while requests from clients with `organizational_unit: devops` TLS certificates are routed to `http://victoriametrics-devops:8428` backend:

```yaml
users:
- mtls:
    organizational_unit: finance
  url_prefix: "http://victoriametrics-finance:8428"
- mtls:
    organizational_unit: devops
  url_prefix: "http://victoriametrics-devops:8428"
```

[mTLS protection](#mtls-protection) must be enabled for mTLS-based routing.

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

### Enforcing query args

`vmauth` can be configured for adding some mandatory query args before proxying requests to backends.
For example, the following [config](#auth-config) adds [`extra_label`](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-enhancements) to all the requests, which are proxied to [single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/):

```yaml
unauthorized_user:
  url_prefix: "http://victoria-metrics:8428/?extra_label=foo=bar"
```

See also [authorization](#authorization), [routing](#routing) and [load balancing](#load-balancing) docs.

## Dropping request path prefix

By default, `vmauth` doesn't drop the path prefix from the original request when proxying the request to the matching backend.
Sometimes it is needed to drop path prefix before proxying the request to the backend. This can be done by specifying the number of `/`-delimited prefix parts to drop from the request path via `drop_src_path_prefix_parts` option at `url_map` level or at `user` level or [`-auth.config`](#auth-config).

For example, if you need to serve requests to [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/) at `/vmalert/` path prefix, while serving requests to [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) at `/vmagent/` path prefix,
then the following [-auth.config](#auth-config) can be used:

```yaml
unauthorized_user:
  url_map:

    # proxy all the requests, which start with `/vmagent/`, to vmagent backend
  - src_paths:
    - "/vmagent/.*"

    # drop /vmagent/ path prefix from the original request before proxying it to url_prefix.
    drop_src_path_prefix_parts: 1
    url_prefix: "http://vmagent-backend:8429/"

    # proxy all the requests, which start with `/vmalert`, to vmalert backend
  - src_paths:
    - "/vmalert/.*"

    # drop /vmalert/ path prefix from the original request before proxying it to url_prefix.
    drop_src_path_prefix_parts: 1
    url_prefix: "http://vmalert-backend:8880/"
```

## Authorization

`vmauth` supports the following authorization mechanisms:

* [No authorization](https://docs.victoriametrics.com/victoriametrics/vmauth/#simple-http-proxy)
* [Basic Auth](https://docs.victoriametrics.com/victoriametrics/vmauth/#basic-auth-proxy)
* [Bearer token](https://docs.victoriametrics.com/victoriametrics/vmauth/#bearer-token-auth-proxy)
* [Client TLS certificate verification aka mTLS](https://docs.victoriametrics.com/victoriametrics/vmauth/#mtls-based-request-routing)
* [Auth tokens via Arbitrary HTTP request headers](https://docs.victoriametrics.com/victoriametrics/vmauth/#reading-auth-tokens-from-other-http-headers)

See also [security docs](#security), [routing docs](#routing) and [load balancing docs](#load-balancing).

## Routing

`vmauth` can proxy requests to different backends depending on the following parts of HTTP request:

* [Request path](#routing-by-path)
* [Request host](#routing-by-host)
* [Request query arg](#routing-by-query-arg)
* [HTTP request header](#routing-by-header)
* [Multiple parts](#routing-by-multiple-parts)

See also [authorization](#authorization) and [load balancing](#load-balancing).
For debug purposes, extra logging for failed requests can be enabled by setting `dump_request_on_errors: true` {{% available_from "v1.107.0" %}} on user level. Please note, such logging may expose sensitive info and is recommended to use only for debugging.

### Routing by path

`src_paths` option can be specified inside `url_map` in order to route requests by path.

The following [`-auth.config`](#auth-config) routes requests to paths starting with `/app1/` to `http://app1-backend`,
while requests with paths starting with `/app2` are routed to `http://app2-backend`, and the rest of requests
are routed to `http://some-backend/404-page.html`:

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/app1/.*"
    url_prefix: "http://app1-backend/"
  - src_paths:
    - "/app2/.*"
    url_prefix: "http://app2-backend/"
  default_url: http://some-backend/404-page.html
```

`src_paths` accepts a list of [regular expressions](https://github.com/google/re2/wiki/Syntax). The incoming request is routed to the given `url_prefix` if **the whole** requested path matches at least one `src_paths` entry.

See also [how to drop request path prefix](#dropping-request-path-prefix).

### Routing by host

`src_hosts` option can be specified inside `url_map` in order to route requests by host header.

The following [`-auth.config`](#auth-config) routes requests to `app1.my-host.com` host to `http://app1-backend`, while routing requests to `app2.my-host.com` host to `http://app2-backend`, and the rest of requests are routed to `http://some-backend/404-page.html`:

```yaml
unauthorized_user:
  url_map:
  - src_hosts:
    - "app1\\.my-host\\.com"
    url_prefix: "http://app1-backend/"
  - src_hosts:
    - "app2\\.my-host\\.com"
    url_prefix: "http://app2-backend/"
  default_url: http://some-backend/404-page.html
```

`src_hosts` accepts a list of [regular expressions](https://github.com/google/re2/wiki/Syntax). The incoming request is routed to the given `url_prefix` if **the whole** request host matches at least one `src_hosts` entry.

### Routing by query arg

`src_query_args` option can be specified inside `url_map` in order to route requests by the given [query arg](https://en.wikipedia.org/wiki/Query_string).

For example, the following [`-auth.config`](#auth-config) routes requests to `http://app1-backend/` if `db=foo` query arg is present in the request, while routing requests with `db` query arg starting with `bar` to `http://app2-backend`, and the rest of requests are routed to `http://some-backend/404-page.html`:

```yaml
unauthorized_user:
  url_map:
  - src_query_args: ["db=foo"]
    url_prefix: "http://app1-backend/"
  - src_query_args: ["db=~bar.*"]
    url_prefix: "http://app2-backend/"
  default_url: http://some-backend/404-page.html
```

`src_query_args` accepts a list of strings in the format `arg=value` or `arg=~regex`.
The `arg=value` format means exact matching of **the whole** `arg` query arg value to the given `value`.
The `arg=~regex` format means regex matching of **the whole** `arg` query arg value to the given `regex`.
If at least a single query arg in the request matches at least one `src_query_args` entry, then the request is routed to the given `url_prefix`.

### Routing by header

`src_headers` option can be specified inside `url_map` in order to route requests by the given HTTP request header.

For example, the following [`-auth.config`](#auth-config) routes requests to `http://app1-backend` if `TenantID` request header equals to `42`, while routing requests to `http://app2-backend` if `TenantID` request header equals to `123:456`, and the rest of requests are routed to `http://some-backend/404-page.html`:

```yaml
unauthorized_user:
  url_map:
  - src_headers: ["TenantID: 42"]
    url_prefix: "http://app1-backend/"
  - src_headers: ["TenantID: 123:456"]
    url_prefix: "http://app2-backend/"
  default_url: http://some-backend/404-page.html
```

If `src_headers` contains multiple entries, then it is enough to match only a single entry in order to route the request to the given `url_prefix`.

### Routing by multiple parts

Any subset of [`src_paths`](#routing-by-path), [`src_hosts`](#routing-by-host), [`src_query_args`](#routing-by-query-arg) and [`src_headers`](#routing-by-header) options can be specified simultaneously in a single `url_map` entry. In this case the request is routed to the given `url_prefix` if the request matches all the provided configs **simultaneously**.

For example, the following [`-auth.config`](#auth-config) routes requests to `http://app1-backend` if all the conditions mentioned below are simultaneously met:

* the request path starts with `/app/`
* the requested hostname ends with `.bar.baz`
* the request contains `db=abc` query arg
* the `TenantID` request header equals to `42`

```yaml
unauthorized_user:
  url_map:
  - src_paths: ["/app/.*"]
    src_hosts: [".+\\.bar\\.baz"]
    src_query_args: ["db=abc"]
    src_headers: ["TenantID: 42"]
    url_prefix: "http://app1-backend/"
```

## Load balancing

Each `url_prefix` in the [-auth.config](#auth-config) can be specified in the following forms:

* A single url. For example:

  ```yaml
  unauthorized_user:
    url_prefix: 'http://vminsert:8480/insert/0/prometheus/`
  ```

  In this case `vmauth` proxies requests to the specified url.

* A list of urls. For example:

  ```yaml
  unauthorized_user:
    url_prefix:
    - 'http://vminsert-1:8480/insert/0/prometheus/'
    - 'http://vminsert-2:8480/insert/0/prometheus/'
    - 'http://vminsert-3:8480/insert/0/prometheus/'
  ```

  In this case `vmauth` spreads requests among the specified urls using least-loaded round-robin policy.
  This guarantees that incoming load is shared uniformly among the specified backends.
  See also [discovering backend IPs](#discovering-backend-ips).

  `vmauth` automatically detects temporarily unavailable backends and spreads incoming queries among the remaining available backends.
  This allows restarting the backends and performing maintenance tasks on the backends without the need to remove them from the `url_prefix` list.

  By default, `vmauth` returns backend responses with all the http status codes to the client. It is possible to configure automatic retry of requests at other backends if the backend responds with status code specified in the `-retryStatusCodes` command-line flag.
  It is possible to customize the list of http response status codes to retry via `retry_status_codes` list at `user` and `url_map` level of [`-auth.config`](#auth-config).
  For example, the following config re-tries requests on other backends if the current backend returns response with `500` or `502` HTTP status code:

  ```yaml
  unauthorized_user:
    url_prefix:
    - http://vmselect1:8481/
    - http://vmselect2:8481/
    - http://vmselect3:8481/
    retry_status_codes: [500, 502]
  ```

  By default, `vmauth` uses `least_loaded` policy to spread the incoming requests among available backends.
  The policy can be changed to `first_available` via `-loadBalancingPolicy` command-line flag. In this case `vmauth` sends all the requests to the first specified backend while it is available. `vmauth` starts sending requests to the next specified backend when the first backend is temporarily unavailable.
  It is possible to customize the load balancing policy at the `user` and `url_map` level.
  For example, the following config specifies `first_available` load balancing policy for unauthorized requests:

  ```yaml
  unauthorized_user:
    url_prefix:
    - http://victoria-metrics-main:8428/
    - http://victoria-metrics-standby:8428/
    load_balancing_policy: first_available
  ```

Load balancing feature can be used in the following cases:

* Balancing the load among multiple `vmselect` and/or `vminsert` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/).
  The following [`-auth.config`](#auth-config) can be used to spread incoming requests among 3 vmselect nodes and re-trying failed requests or requests with 500 and 502 response status codes:

  ```yaml
  unauthorized_user:
    url_prefix:
    - http://vmselect1:8481/
    - http://vmselect2:8481/
    - http://vmselect3:8481/
    retry_status_codes: [500, 502]
  ```

* Sending select queries to the closest availability zone (AZ), while falling back to other AZs with identical data if the closest AZ is unavailable.
  For example, the following [`-auth.config`](#auth-config) sends select queries to `https://vmselect-az1/` and uses the `https://vmselect-az2/` as a fallback when `https://vmselect-az1/` is temporarily unavailable or cannot return full responses.
  See [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-availability) for details about `deny_partial_response` query arg, which is added to requests before they are proxied to backends.

  ```yaml
  unauthorized_user:
    url_prefix:
    - https://vmselect-az1/?deny_partial_response=1
    - https://vmselect-az2/?deny_partial_response=1
    retry_status_codes: [500, 502, 503]
    load_balancing_policy: first_available
  ```

Load balancing can be configured independently per each `user` entry and per each `url_map` entry. See [auth config docs](#auth-config) for more details.

See also [discovering backend IPs](#discovering-backend-ips), [authorization](#authorization) and [routing](#routing).

## Discovering backend IPs

By default, `vmauth` spreads load among the listed backends at `url_prefix` as described in [load balancing docs](#load-balancing).
Sometimes multiple backend instances can be hidden behind a single hostname. For example, `vmselect-service` hostname
may point to a cluster of `vmselect` instances in [VictoriaMetrics cluster setup](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview).
So the following config may fail to spread load among available `vmselect` instances, since `vmauth` will send all the requests to the same url, which may end up to a single backend instance:

```yaml
unauthorized_user:
  url_prefix: http://vmselect-service/select/0/prometheus/
```

There are the following solutions for this issue:

* To enumerate every `vmselect` hostname or IP in the `url_prefix` list:

  ```yaml
  unauthorized_user:
    url_prefix:
    - http://vmselect-1:8481/select/0/prometheus/
    - http://vmselect-2:8481/select/0/prometheus/
    - http://vmselect-3:8481/select/0/prometheus/
  ```

  This scheme works great, but it needs manual updating of the [`-auth.config`](#auth-config) every time `vmselect` services are restarted, downscaled or upscaled.

* To set `discover_backend_ips: true` option, so `vmauth` automatically discovers IPs behind the given hostname and then spreads load among the discovered IPs:

  ```yaml
  unauthorized_user:
    url_prefix: http://vmselect-service:8481/select/0/prometheus/
    discover_backend_ips: true
  ```

  If the `url_prefix` contains hostname with `srv+` prefix, then the hostname without `srv+` prefix is automatically resolved via [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) to the list of hostnames with TCP ports, and `vmauth` balances load among the discovered TCP addresses:

  ```yaml
  unauthorized_user:
    url_prefix: "http://srv+vmselect/select/0/prometheus"
    discover_backend_ips: true
  ```

  This functionality is useful for balancing load among backend instances, which run on different TCP ports, since DNS SRV records contain TCP ports.

  The `discover_backend_ips` option can be specified at `user` and `url_map` level in the [`-auth.config`](#auth-config). It can also be enabled globally via `-discoverBackendIPs` command-line flag.

See also [load balancing docs](#load-balancing).

## SRV urls

If `url_prefix` contains url with the hostname starting with `srv+` prefix, then `vmauth` uses [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) lookup for the hostname without the `srv+` prefix and selects random TCP address (e.g. hostname plus TCP port) form the resolved results.

For example, if `some-addr` [DNS SRV](https://en.wikipedia.org/wiki/SRV_record) record contains `some-host:12345` TCP address,
then `url_prefix: http://srv+some-addr/some/path` is automatically resolved into `url_prefix: http://some-host:12345/some/path`.
The DNS SRV resolution is performed every time new connection to the `url_prefix` backend is established.

See also [discovering backend addresses](#discovering-backend-ips).

## Modifying HTTP headers

`vmauth` supports the ability to set and remove HTTP request headers before sending the requests to backends.
This is done via `headers` option. For example, the following [`-auth.config`](#auth-config) sets `TenantID: foobar` header to requests proxied to `http://backend:1234/`. It also overrides `X-Forwarded-For` request header with an empty value. This effectively removes the `X-Forwarded-For` header from requests proxied to `http://backend:1234/`:

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  headers:
  - "TenantID: foobar"
  - "X-Forwarded-For:"

users:
  - username: "foo"
    password: "bar"
    dump_request_on_errors: true
    url_map:
      - src_paths: ["/select/.*"]
        headers:
          - "AccountID: 1"
          - "ProjectID: 0"
        url_prefix:
          - "http://backend:9428/"

```

`vmauth` also supports the ability to set and remove HTTP response headers before returning the response from the backend to client.
This is done via `response_headers` option. For example, the following [`-auth.config`](#auth-config) sets `Foo: bar` response header
and removes `Server` response header before returning the response to client:

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  response_headers:
  - "Foo: bar"
  - "Server:"
```

See also [`Host` header docs](#host-http-header).

## Host HTTP header

By default, `vmauth` sets the `Host` HTTP header to the backend hostname when proxying requests to the corresponding backend.
Sometimes it is needed to keep the original `Host` header from the client request sent to `vmauth`. For example, if backends use host-based routing.
In this case set `keep_original_host: true`. For example, the following config instructs to use the original `Host` header from client requests when proxying requests to the `backend:1234`:

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  keep_original_host: true
```

It is also possible to set the `Host` header to arbitrary value when proxying the request to the configured backend, via [`headers` option](#modifying-http-headers):

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  headers:
  - "Host: foobar"
```

## Config reload

`vmauth` supports dynamic reload of [`-auth.config`](#auth-config) via the following ways:

* By sending `SIGHUP` signal to `vmauth` process:

  ```sh
  kill -HUP `pidof vmauth`
  ```

* By querying `/-/reload` endpoint. It is recommended to protect it with `-reloadAuthKey`. See [security docs](#security) for details.
* By passing the interval for config check to `-configCheckInterval` command-line flag.

## Concurrency limiting

`vmauth` may limit the number of concurrent requests according to the following command-line flags:

* `-maxConcurrentRequests` limits the global number of concurrent requests `vmauth` can serve across all the configured users.
* `-maxConcurrentPerUserRequests` limits the number of concurrent requests `vmauth` can serve per each configured user.

It is also possible to set individual limits on the number of concurrent requests per each user with the `max_concurrent_requests` option.
For example, the following [`-auth.config`](#auth-config) limits the number of concurrent requests from the user `foo` to 10:

```yaml
users:
- username: foo
  password: bar
  url_prefix: "http://some-backend/"
  max_concurrent_requests: 10
```

`vmauth` responds with `429 Too Many Requests` HTTP error when the number of concurrent requests exceeds the configured limits for the duration
exceeding the `-maxQueueDuration` command-line flag value.

The following [metrics](#monitoring) related to concurrency limits are exposed by `vmauth`:

* `vmauth_concurrent_requests_capacity` - the global limit on the number of concurrent requests `vmauth` can serve.
  It is set via `-maxConcurrentRequests` command-line flag.
* `vmauth_concurrent_requests_current` - the current number of concurrent requests `vmauth` processes.
* `vmauth_concurrent_requests_limit_reached_total` - the number of requests rejected with `429 Too Many Requests` error
  because of the global concurrency limit has been reached.
* `vmauth_user_concurrent_requests_capacity{username="..."}` - the limit on the number of concurrent requests for the given `username`.
* `vmauth_user_concurrent_requests_current{username="..."}` - the current number of concurrent requests for the given `username`.
* `vmauth_user_concurrent_requests_limit_reached_total{username="..."}` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for the given `username`.
* `vmauth_unauthorized_user_concurrent_requests_capacity` - the limit on the number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
* `vmauth_unauthorized_user_concurrent_requests_current` - the current number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
* `vmauth_unauthorized_user_concurrent_requests_limit_reached_total` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for unauthorized users (if `unauthorized_user` section is used).

## Backend TLS setup

By default, `vmauth` uses system settings when performing requests to HTTPS backends specified via `url_prefix` option in the [`-auth.config`](#auth-config). These settings can be overridden with the following command-line flags:

* `-backend.tlsInsecureSkipVerify` allows skipping TLS verification when connecting to HTTPS backends.
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config) via `tls_insecure_skip_verify` option. For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_insecure_skip_verify: true
  ```

* `-backend.tlsCAFile` allows specifying the path to TLS Root CA for verifying backend TLS certificates.
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config) via `tls_ca_file` option.
  For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_ca_file: "/path/to/tls/root/ca"
  ```

* `-backend.tlsCertFile` and `-backend.tlsKeyFile` allows specifying client TLS certificate for passing in requests to HTTPS backends,
  so these certificate could be verified at the backend side (aka [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication)).
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config) via `tls_cert_file` and `tls_key_file` options. For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_cert_file: "/path/to/tls/cert"
    tls_key_file: "/path/to/tls/key"
  ```

* `-backend.tlsServerName` allows specifying optional [TLS ServerName](https://en.wikipedia.org/wiki/Server_Name_Indication) for passing in requests to HTTPS backends.
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config) via `tls_server_name` option. For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_server_name: "foo.bar.com"
  ```

The `-backend.tlsCAFile`, `-backend.tlsCertFile`, `-backend.tlsKeyFile`, `tls_ca_file`, `tls_cert_file` and `tls_key_file` may point either to local file or to `http` / `https` url.
The file is checked for modifications every second and is automatically re-read when it is updated.

## IP filters

[Enterprise version](https://docs.victoriametrics.com/victoriametrics/enterprise/) of `vmauth` can be configured to allow / deny incoming requests via global and per-user IP filters.

For example, the following config allows requests to `vmauth` from `10.0.0.0/24` network and from `1.2.3.4` IP address, while denying requests from `10.0.0.42` IP address:

```yaml
users:
# User configs here

ip_filters:
  allow_list:
  - 10.0.0.0/24
  - 1.2.3.4
  deny_list: [10.0.0.42]
```

The following config allows requests for the user `foobar` only from the IP `127.0.0.1`:

```yaml
users:
- username: "foobar"
  password: "***"
  url_prefix: "http://localhost:8428"
  ip_filters:
    allow_list: [127.0.0.1]
```

By default, the client's TCP address is utilized for IP filtering. In scenarios where `vmauth` operates behind a reverse proxy, it is advisable to configure `vmauth` to retrieve the client IP address from an [HTTP header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For) (e.g., `X-Forwarded-For`) {{% available_from "v1.107.0" %}} or via the [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) for TCP load balancers. This can be achieved using the global configuration flags:

* `-httpRealIPHeader=X-Forwarded-For` {{% available_from "v1.107.0" %}}
* `-httpListenAddr.useProxyProtocol=true`

### Security Considerations

**HTTP headers are inherently untrustworthy.** It is strongly recommended to implement additional security measures, such as:

* Dropping `X-Forwarded-For` headers at the internet-facing reverse proxy (e.g., before traffic reaches `vmauth`).
* Do not use `-httpRealIPHeader` at internet-facing `vmauth`.
* Add `removeXFFHTTPHeaderValue` for the internet-facing `vmauth`. It instructs `vmauth` to replace value of `X-Forwarded-For` HTTP header with `remoteAddr` of the client.

See additional recommendations for [security and privacy concerns](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#security_and_privacy_concerns)

### Per-User Configuration

The values of `httpRealIPHeader` {{% available_from "v1.107.0" %}} can be changed on a per-user basis within the user-specific configuration.

```yaml
users:
- username: "foobar"
  password: "***"
  url_prefix: "http://localhost:8428"
  ip_filters:
    allow_list: [127.0.0.1]
    real_ip_header: X-Forwarded-For
- username: "foobar"
  password: "***"
  url_prefix: "http://localhost:8428"
  ip_filters:
    allow_list: [127.0.0.1]
    real_ip_header: CF-Connecting-IP
```

See config example of using [IP filters](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmauth/example_config_ent.yml).

## Reading auth tokens from other HTTP headers

`vmauth` reads `username`, `password` and `bearer_token` [config values](#auth-config) from `Authorization` request header.
It is possible to read these auth tokens from any other request header by specifying it via `-httpAuthHeader` command-line flag.
For example, the following command instructs `vmauth` to read auth token from `X-Amz-Firehose-Access-Key` header:

```sh
./vmauth -httpAuthHeader='X-Amz-Firehose-Access-Key'
```

It is possible to read auth tokens from multiple headers. For example, the following command instructs `vmauth` to read auth token
from both `Authorization` and `X-Amz-Firehose-Access-Key` headers:

```sh
./vmauth -httpAuthHeader='Authorization' -httpAuthHeader='X-Amz-Firehose-Access-Key'
```

See also [authorization docs](#authorization) and [security docs](#security).

## Query args handling

By default, `vmauth` sends all the query args specified in the `url_prefix` to the backend. It also proxies query args from client requests if they do not clash with the args specified in the `url_prefix`. This is needed for security, e.g. it disallows the client overriding security-sensitive query args specified at the `url_prefix` such as `tenant_id`, `password`, `auth_key`, `extra_filters`, etc.

`vmauth` provides the ability to specify a list of query args, which can be proxied from the client request to the backend if they clash with the args specified in the `url_prefix`. In this case the client query args are added to the args from the `url_prefix` before being proxied to the backend. This can be done via the following options:

* Via `-mergeQueryArgs` command-line flag. This flag may contain comma-separated list of client query arg names, which are allowed
  to merge with the `url_prefix` query args when sending the request to the backend. This option is applied globally to all the configured backends.

* Via `merge_query_args` option at the `user` and `url_map` level. These values override the `-mergeQueryArgs` command-line flag.

The example below sends the request to `http://victoria-logs:9429/select/logsql/query?extra_filters={env="prod"}&extra_filters={team="dev"}&query=error` when `vmauth` receives request to `http://vmauth/select/logsql/query?extra_filters={team="dev"}&query=error`:

```yaml
unauthorized_user:
  # Merge `extra_filter` query arg from the clients with the `extra_filter` query args specified in the `url_prefix` below
  merge_query_args: [extra_filters]
  url_map:
  - src_paths: ["/select/.+"]
    url_prefix: 'http://victoria-logs:9428/?extra_filters={env="prod"}'
```

## Auth config

`-auth.config` is represented in the following `yml` format:

```yaml
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

  # Requests with the 'Authorization: Foo XXXX' header are proxied to http://localhosT:8428 .
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query
- auth_token: "Foo XXXX"
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
  # For example, http://vmauth:8427/api/v1/query is proxied to http://localhost:8428/api/v1/query?extra_label=team=dev
- username: "local-single-node2"
  password: "***"
  url_prefix: "http://localhost:8428?extra_label=team=dev"

  # All the requests to http://vmauth:8427 with the given Basic Auth (username:password)
  # are proxied to https://localhost:8428.
  # For example, http://vmauth:8427/api/v1/query is proxied to https://localhost/api/v1/query
  # TLS verification is skipped for https://localhost.
- username: "local-single-node-with-tls"
  password: "***"
  url_prefix: "https://localhost"
  tls_insecure_skip_verify: true

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
  #
  # Regular expressions are allowed in `src_paths` and `src_hosts` entries.
- username: "foobar"
  # log requests that failed url_map rules, for debugging purposes
  dump_request_on_errors: true
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

# Requests without Authorization header are proxied according to `unauthorized_user` section.
# Requests are proxied in round-robin fashion between `url_prefix` backends.
# The deny_partial_response query arg is added to all the proxied requests.
# The requests are re-tried if url_prefix backends send 500 or 503 response status codes.
# Note that the unauthorized_user section takes precedence when processing a route without credentials,
# even if such a route also exists in the users section (see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5236).
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

## mTLS protection

By default, `vmauth` accepts http requests at `8427` port (this port can be changed via `-httpListenAddr` command-line flags).
[Enterprise version of vmauth](https://docs.victoriametrics.com/victoriametrics/enterprise/) supports the ability to accept [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication) requests at this port, by specifying `-tls` and `-mtls` command-line flags. For example, the following command runs `vmauth`, which accepts only mTLS requests at port `8427`:

```sh
./vmauth -tls -mtls -auth.config=...
```

By default, system-wide [TLS Root CA](https://en.wikipedia.org/wiki/Root_certificate) is used to verify client certificates, if `-mtls` command-line flag is specified.
It is possible to specify custom TLS Root CA via `-mtlsCAFile` command-line flag.

See also [automatic issuing of TLS certificates](#automatic-issuing-of-tls-certificates) and [mTLS-based request routing](#mtls-based-request-routing).

## Security

It is expected that all the backend services protected by `vmauth` are located in an isolated private network, so they can be accessed by external users only via `vmauth`.

Do not transfer auth headers in plaintext over untrusted networks. Enable https at `-httpListenAddr`. This can be done by passing the following `-tls*` command-line flags to `vmauth`:

```sh
  -tls
     Whether to enable TLS for incoming HTTP requests at -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
     Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
     Path to file with TLS key. Used only if -tls is set
```

See also [automatic issuing of TLS certificates](#automatic-issuing-of-tls-certificates).

See [these docs](#mtls-protection) on how to enable [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication) protection at `vmauth`.

Alternatively, [TLS termination proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy) may be put in front of `vmauth`.

It is recommended to protect the following endpoints with authKeys:

* `/-/reload` with `-reloadAuthKey` command-line flag, so external users couldn't trigger config reload.
* `/flags` with `-flagsAuthKey` command-line flag, so unauthorized users couldn't get command-line flag values.
* `/metrics` with `-metricsAuthKey` command-line flag, so unauthorized users couldn't access [vmauth metrics](#monitoring).
* `/debug/pprof` with `-pprofAuthKey` command-line flag, so unauthorized users couldn't access [profiling information](#profiling).

As an alternative, it's possible to serve internal API routes at the different listen address with command-line flag `-httpInternalListenAddr=127.0.0.1:8426`. {{% available_from "v1.111.0" %}}

`vmauth` also supports the ability to restrict access by IP - see [these docs](#ip-filters). See also [concurrency limiting docs](#concurrency-limiting).

## Automatic issuing of TLS certificates

`vmauth` [Enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/) supports automatic issuing of TLS certificates via [Let's Encrypt service](https://letsencrypt.org/).
The following command-line flags must be set in order to enable automatic issuing of TLS certificates:

* `-httpListenAddr` must be set to listen on TCP port `443`. For example, `-httpListenAddr=:443`. This port must be accessible by the [Let's Encrypt service](https://letsencrypt.org/).
* `-tls` must be set in order to accept HTTPS requests at `-httpListenAddr`. Note that `-tlcCertFile` and `-tlsKeyFile` aren't needed when automatic TLS certificate issuing is enabled.
* `-tlsAutocertHosts` must be set to comma-separated list of hosts, which can be reached via `-httpListenAddr`. TLS certificates are automatically issued for these hosts.
* `-tlsAutocertEmail` must be set to contact email for the issued TLS certificates.
* `-tlsAutocertCacheDir` may be set to the directory path to persist the issued TLS certificates between `vmauth` restarts. If this flag isn't set, then TLS certificates are re-issued on every restart.

This functionality can be evaluated for free according to [these docs](https://docs.victoriametrics.com/victoriametrics/enterprise/).

See also [security recommendations](#security).

## Monitoring

`vmauth` exports various metrics in Prometheus exposition format at `http://vmauth-host:8427/metrics` page. It is recommended to set up regular scraping of this page either via [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) or via Prometheus-compatible scraper, so the exported metrics could be analyzed later.
Use the official [Grafana dashboard](https://grafana.com/grafana/dashboards/21394) and [alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/alerts-vmauth.yml) for `vmauth` monitoring.

If you use Google Cloud Managed Prometheus for scraping metrics from VictoriaMetrics components, then pass `-metrics.exposeMetadata`
command-line to them, so they add `TYPE` and `HELP` comments per each exposed metric at `/metrics` page.
See [these docs](https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type) for details.

`vmauth` exports the following metrics per each defined user in [`-auth.config`](#auth-config):

* `vmauth_user_requests_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of requests served for the given `username`
* `vmauth_user_request_errors_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of request errors for the given `username`
* `vmauth_user_request_backend_requests_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of backend requests for the given `username`
* `vmauth_user_request_backend_errors_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of backend request errors for the given `username`
* `vmauth_user_request_duration_seconds` [summary](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#summary) - the duration of requests for the given `username`
* `vmauth_user_concurrent_requests_limit_reached_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of failed requests
  for the given `username` because of exceeded [concurrency limits](#concurrency-limiting)
* `vmauth_user_concurrent_requests_capacity` [gauge](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#gauge) - the maximum number of [concurrent requests](#concurrency-limiting)
  for the given `username`
* `vmauth_user_concurrent_requests_current` [gauge](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#gauge) - the current number of [concurrent requests](#concurrency-limiting)
  for the given `username`

By default, per-user metrics contain only `username` label. This label is set to `username` field value at the corresponding user section in the [`-auth.config`](#auth-config) file.
It is possible to override the `username` label value by specifying `name` field additionally to `username` field.
For example, the following config will result in `vmauth_user_requests_total{username="foobar"}` instead of `vmauth_user_requests_total{username="secret_user"}`:

```yaml
users:
- username: "secret_user"
  name: "foobar"
  # other config options here
```

Additional labels for per-user metrics can be specified via `metric_labels` section. For example, the following config defines `{dc="eu",team="dev"}` labels additionally to `username="foobar"` label:

```yaml
users:
- username: "foobar"
  metric_labels:
   dc: eu
   team: dev
  # other config options here
```

`vmauth` exports the following metrics if `unauthorized_user` section is defined in [`-auth.config`](#auth-config):

* `vmauth_unauthorized_user_requests_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of requests served for unauthorized user
* `vmauth_unauthorized_user_request_errors_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of request errors for unauthorized user
* `vmauth_unauthorized_user_request_backend_requests_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of backend requests for unauthorized user
* `vmauth_unauthorized_user_request_backend_errors_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of backend request errors for unauthorized user
* `vmauth_unauthorized_user_request_duration_seconds` [summary](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#summary) - the duration of requests for unauthorized user
* `vmauth_unauthorized_user_concurrent_requests_limit_reached_total` [counter](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#counter) - the number of failed requests because of exceeded [concurrency limits](#concurrency-limiting) for unauthorized user
* `vmauth_unauthorized_user_concurrent_requests_capacity` [gauge](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#gauge) - the maximum number of [concurrent requests](#concurrency-limiting) for unauthorized user
* `vmauth_unauthorized_user_concurrent_requests_current` [gauge](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#gauge) - the current number of [concurrent requests](#concurrency-limiting) for unauthorized user

## How to build from sources

It is recommended to use [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - `vmauth` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install).
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

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```sh
ROOT_IMAGE=scratch make package-vmauth
```

## Profiling

`vmauth` provides handlers for collecting the following [Go profiles](https://blog.golang.org/profiling-go-programs):

* Memory profile. It can be collected with the following command (replace `0.0.0.0` with hostname if needed):

```sh
curl http://0.0.0.0:8427/debug/pprof/heap > mem.pprof
```

* CPU profile. It can be collected with the following command (replace `0.0.0.0` with hostname if needed):

```sh
curl http://0.0.0.0:8427/debug/pprof/profile > cpu.pprof
```

The command for collecting CPU profile waits for 30 seconds before returning.

The collected profiles may be analyzed with [go tool pprof](https://github.com/google/pprof).
It is safe to share the collected profiles from security point of view, since they do not contain sensitive information.

## Advanced usage

Pass `-help` command-line arg to `vmauth` in order to see all the configuration options:

### Common flags
These flags are available in both VictoriaMetrics OSS and VictoriaMetrics Enterprise.
{{% content "vmauth_common_flags.md" %}}

### Enterprise flags
These flags are available only in [VictoriaMetrics enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/).
{{% content "vmauth_enterprise_flags.md" %}}
