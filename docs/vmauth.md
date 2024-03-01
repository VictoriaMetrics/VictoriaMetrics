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
It reads auth credentials from `Authorization` http header ([Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication),
`Bearer token` and [InfluxDB authorization](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1897) is supported),
matches them against configs pointed by [-auth.config](#auth-config) command-line flag and proxies incoming HTTP requests to the configured per-user `url_prefix` on successful match.
The `-auth.config` can point to either local file or to http url.

## Quick start

Just download `vmutils-*` archive from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest), unpack it
and pass the following flag to `vmauth` binary in order to start authorizing and proxying requests:

```sh
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

The `-auth.config` command-line flag must point to valid [config](#auth-config). See [use cases](#use-cases) with typical `-auth.config` examples.

`vmauth` accepts HTTP requests on port `8427` and proxies them according to the provided [-auth.config](#auth-config).
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

### Generic HTTP proxy for different backends

`vmauth` can proxy requests to different backends depending on the requested host and/or path.
For example, the following [`-auth.config`](#auth-config) instructs `vmauth` to make the following:

- Requests starting with `/app1/` are proxied to `http://app1-backend/`, while the `/app1/` path prefix is dropped according to [`drop_src_path_prefix_parts`](#dropping-request-path-prefix).
  For example, the request to `http://vmauth:8427/app1/foo/bar?baz=qwe` is proxied to `http://app1-backend/foo/bar?baz=qwe`.
- Requests starting with `/app2/` are proxied to `http://app2-backend/`, while the `/app2/` path prefix is dropped according to [`drop_src_path_prefix_parts`](#dropping-request-path-prefix).
  For example, the request to `http://vmauth:8427/app2/index.html` is proxied to `http://app2-backend/index.html`.
- Other requests are proxied to `http://some-backend/404-page.html`, while the requested path is passed via `request_path` query arg.
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
  default_url: http://some-backend/404-page.html
```

The following config routes requests to host `app1.my-host.com` to `http://app1-backend`, while routing requests to `app2.my-host.com` to `http://app2-backend`:

```yaml
unauthorized_user:
  url_map:
  - src_hosts:
    - "app1\\.my-host\\.com"
    url_prefix: "http://app1-backend/"
  - src_hosts:
    - "app2\\.my-host\\.com"
    url_prefix: "http://app2-backend/"
```

`src_paths` and `src_hosts` accept a list of [regular expressions](https://github.com/google/re2/wiki/Syntax). The incoming request is routed to the given `url_prefix`
if the whole request path matches at least one `src_paths` entry. The incoming request is routed to the given `url_prefix` if the whole request host matches at least one `src_hosts` entry.
If both `src_paths` and `src_hosts` lists are specified, then the request is routed to the given `url_prefix` when both request path and request host match at least one entry
in the corresponding lists.

### Generic HTTP load balancer

`vmauth` can balance load among multiple HTTP backends in least-loaded round-robin mode.
For example, the following [`-auth.config`](#auth-config) instructs `vmauth` to spread load load among multiple application instances:

```yaml
unauthorized_user:
  url_prefix:
  - "http://app-instance-1/"
  - "http://app-instance-2/"
  - "http://app-instance-3/"
```

See [load balancing docs](#load-balancing) for more details.

### Load balancer for vmagent

If [vmagent](https://docs.victoriametrics.com/vmagent.html) is used for processing [data push requests](https://docs.victoriametrics.com/vmagent.html#how-to-push-data-to-vmagent),
then it is possible to scale the performance of data processing at `vmagent` by spreading load among multiple identically configured `vmagent` instances.
This can be done with the following [config](#auth-config) for `vmagent`:

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/prometheus/api/v1/write"
    - "/influx/write"
    - "/api/v1/import"
    - "/api/v1/import/.+"
    url_prefix:
    - "http://vmagent-1:8429/"
    - "http://vmagent-2:8429/"
    - "http://vmagent-3:8429/"
```

See [load balancing docs](#load-balancing) for more details.

### Load balancer for VictoriaMetrics cluster

[VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) accepts incoming data via `vminsert` nodes
and processes incoming requests via `vmselect` nodes according to [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#architecture-overview).
`vmauth` can be used for balancing both `insert` and `select` requests among `vminsert` and `vmselect` nodes, when the following [`-auth.config`](#auth-config) is used:

```yaml
unauthorized_user:
  url_map:
  - src_paths:
    - "/insert/.+"
    url_prefix:
    - "http://vminsert-1:8480/"
    - "http://vminsert-2:8480/"
    - "http://vminsert-3:8480/"
  - src_paths:
    - "/select/.+"
    url_prefix:
    - "http://vmselect-1:8481/"
    - "http://vmselect-2:8481/"
```

See [load balancing docs](#load-balancing) for more details.

### High availability

`vmauth` automatically switches from temporarily unavailable backend to other hot standby backends listed in `url_prefix`
if it runs with `-loadBalancingPolicy=first_available` command-line flag. The load balancing policy can be overridden at `user` and `url_map` sections
of [`-auth.config`](#auth-config) via `load_balancing_policy` option. For example, the following config instructs `vmauth` to proxy requests to `http://victoria-metrics-main:8428/` backend.
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

### TLS termination proxy

`vmauth` can terminate HTTPS requests to backend services when it runs with the following command-line flags:

```
/path/to/vmauth -tls -tlsKeyFile=/path/to/tls_key_file -tlsCertFile=/path/to/tls_cert_file -httpListenAddr=0.0.0.0:443
```

* `-httpListenAddr` sets the address to listen for incoming HTTPS requests
* `-tls` enables accepting TLS connections at `-httpListenAddr`
* `-tlsKeyFile` sets the path to TLS certificate key file
* `-tlsCertFile` sets the path to TLS certificate file

### Basic Auth proxy

`vmauth` can authorize access to backends depending on the provided [Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication) request headers.
For example, the following [config](#auth-config) proxies requests to [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
if they contain Basic Auth header with the given `username` and `password`:

```yaml
users:
- username: foo
  password: bar
  url_prefix: "http://victoria-metrics:8428/"
```

See also [security docs](#security).

### Bearer Token auth proxy

`vmauth` can authorize access to backends depending on the provided `Bearer Token` request headers.
For example, the following [config](#auth-config) proxies requests to [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
if they contain the given `bearer_token`:

```yaml
users:
- bearer_token: ABCDEF
  url_prefix: "http://victoria-metrics:8428/"
```

See also [security docs](#security).

### Per-tenant authorization

The following [`-auth.config`](#auth-config) instructs proxying `insert` and `select` requests from the [Basic Auth](https://en.wikipedia.org/wiki/Basic_access_authentication)
user `tenant1` to the [tenant](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy) `1`,
while requests from the user `tenant2` are sent to tenant `2`:

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

### mTLS-based request routing

[Enterprise version of `vmauth`](https://docs.victoriametrics.com/enterprise.html) can be configured for routing requests
to different backends depending on the following [subject fields](https://en.wikipedia.org/wiki/Public_key_certificate#Common_fields) in the TLS certificate provided by client:

* `organizational_unit` aka `OU`
* `organization` aka `O`
* `common_name` aka `CN`

For example, the following [`-auth.config`](#auth-config) routes requests from clients with `organizational_unit: finance` TLS certificates
to `http://victoriametrics-finance:8428` backend:

```yaml
users:
- mtls:
    organizational_unit: finance
  url_prefix: "http://victoriametrics-finance:8428"
```

[mTLS protection](#mtls-protection) must be enabled for mTLS-based routing.


### Enforcing query args

`vmauth` can be configured for adding some mandatory query args before proxying requests to backends.
For example, the following [config](#auth-config) adds [`extra_label`](https://docs.victoriametrics.com/#prometheus-querying-api-enhancements)
to all the requests, which are proxied to [single-node VictoriaMetrics](https://docs.victoriametrics.com/):

```yaml
unauthorized_user:
  url_prefix: "http://victoria-metrics:8428/?extra_label=foo=bar"
```

## Dropping request path prefix

By default `vmauth` doesn't drop the path prefix from the original request when proxying the request to the matching backend.
Sometimes it is needed to drop path prefix before proxying the request to the backend. This can be done by specifying the number of `/`-delimited
prefix parts to drop from the request path via `drop_src_path_prefix_parts` option at `url_map` level or at `user` level.

For example, if you need to serve requests to [vmalert](https://docs.victoriametrics.com/vmalert.html) at `/vmalert/` path prefix,
while serving requests to [vmagent](https://docs.victoriametrics.com/vmagent.html) at `/vmagent/` path prefix for a particular user,
then the following [-auth.config](#auth-config) can be used:

```yaml
users:
- username: foo
  url_map:

    # proxy all the requests, which start with `/vmagent/`, to vmagent backend
  - src_paths:
    - "/vmagent/.+"

    # drop /vmagent/ path prefix from the original request before proxying it to url_prefix.
    drop_src_path_prefix_parts: 1
    url_prefix: "http://vmagent-backend:8429/"

    # proxy all the requests, which start with `/vmalert`, to vmalert backend
  - src_paths:
    - "/vmalert/.+"

    # drop /vmalert/ path prefix from the original request before proxying it to url_prefix.
    drop_src_path_prefix_parts: 1
    url_prefix: "http://vmalert-backend:8880/"
```

## Load balancing

Each `url_prefix` in the [-auth.config](#auth-config) can be specified in the following forms:

- A single url. For example:

  ```yaml
  unauthorized_user:
    url_prefix: 'http://vminsert:8480/insert/0/prometheus/`
  ```

  In this case `vmauth` proxies requests to the specified url.

- A list of urls. For example:

  ```yaml
  unauthorized_user:
    url_prefix:
    - 'http://vminsert-1:8480/insert/0/prometheus/'
    - 'http://vminsert-2:8480/insert/0/prometheus/'
    - 'http://vminsert-3:8480/insert/0/prometheus/'
  ```

  In this case `vmauth` spreads requests among the specified urls using least-loaded round-robin policy.
  This guarantees that incoming load is shared uniformly among the specified backends.

  `vmauth` automatically detects temporarily unavailable backends and spreads incoming queries among the remaining available backends.
  This allows restarting the backends and peforming mantenance tasks on the backends without the need to remove them from the `url_prefix` list.

  By default `vmauth` returns backend responses with all the http status codes to the client. It is possible to configure automatic retry of requests
  at other backends if the backend responds with status code specified in the `-retryStatusCodes` command-line flag.
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

  By default `vmauth` uses `least_loaded` policy for spreading incoming requests among available backends.
  The policy can be changed to `first_available` via `-loadBalancingPolicy` command-line flag. In this case `vmauth`
  sends all the requests to the first specified backend while it is available. `vmauth` starts sending requests to the next
  specified backend when the first backend is temporarily unavailable.
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

- Balancing the load among multiple `vmselect` and/or `vminsert` nodes in [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html).
  The following [`-auth.config`](#auth-config) can be used for spreading incoming requests among 3 vmselect nodes and re-trying failed requests
  or requests with 500 and 502 response status codes:

  ```yaml
  unauthorized_user:
    url_prefix:
    - http://vmselect1:8481/
    - http://vmselect2:8481/
    - http://vmselect3:8481/
    retry_status_codes: [500, 502]
  ```

- Sending select queries to the closest availability zone (AZ), while falling back to other AZs with identical data if the closest AZ is unavaialable.
  For example, the following [`-auth.config`](#auth-config) sends select queries to `https://vmselect-az1/` and uses the `https://vmselect-az2/` as a fallback
  when `https://vmselect-az1/` is temporarily unavailable or cannot return full responses.
  See [these docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-availability) for details about `deny_partial_response` query arg,
  which is added to requests before they are proxied to backends.

  ```yaml
  unauthorized_user:
    url_prefix:
    - https://vmselect-az1/?deny_partial_response=1
    - https://vmselect-az2/?deny_partial_response=1
    retry_status_codes: [500, 502, 503]
    load_balancing_policy: first_available
  ```

Load balancig can be configured independently per each `user` entry and per each `url_map` entry. See [auth config docs](#auth-config) for more details.

## Modifying HTTP headers

`vmauth` supports the ability to set and remove HTTP request headers before sending the requests to backends.
This is done via `headers` option. For example, the following [`-auth.config`](#auth-config) adds `TenantID: foobar` header
to requests proxied to `http://backend:1234/`. It also overrides `X-Forwarded-For` request header with an empty value. This effectively
removes the `X-Forwarded-For` header from requests proxied to `http://backend:1234/`:

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  headers:
  - "TenantID: foobar"
  - "X-Forwarded-For:"
```

`vmauth` also supports the ability to set and remove HTTP response headers before returning the response from the backend to client.
This is done via `response_headers` option. For example, the following [`-auth.config`](#auth-config) adds `Foo: bar` response header
and removes `Server` response header before returning the response to client:

```yaml
unauthorized_user:
  url_prefix: "http://backend:1234/"
  response_headers:
  - "Foo: bar"
  - "Server:"
```

## Config reload

`vmauth` supports dynamic reload of [`-auth.config`](#auth-config) via the following ways:

- By sending `SIGHUP` signal to `vmauth` process:
  ```
  kill -HUP `pidof vmauth`
  ```
- By querying `/-/reload` endpoint. It is recommended protecting it with `-reloadAuthKey`. See [security docs](#security) for details.
- By passing the interval for config check to `-configCheckInterval` command-line flag.

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
- `vmauth_user_concurrent_requests_limit_reached_total{username="..."}` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for the given `username`.
- `vmauth_unauthorized_user_concurrent_requests_capacity` - the limit on the number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
- `vmauth_unauthorized_user_concurrent_requests_current` - the current number of concurrent requests for unauthorized users (if `unauthorized_user` section is used).
- `vmauth_unauthorized_user_concurrent_requests_limit_reached_total` - the number of requests rejected with `429 Too Many Requests` error
  because of the concurrency limit has been reached for unauthorized users (if `unauthorized_user` section is used).

## Backend TLS setup

By default `vmauth` uses system settings when performing requests to HTTPS backends specified via `url_prefix` option
in the [`-auth.config`](#auth-config). These settings can be overridden with the following command-line flags:

- `-backend.tlsInsecureSkipVerify` allows skipping TLS verification when connecting to HTTPS backends.
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config)
  via `tls_insecure_skip_verify` option. For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_insecure_skip_verify: true
  ```

- `-backend.tlsCAFile` allows specifying the path to TLS Root CA, which will be used for TLS verification when connecting to HTTPS backends.
  The `-backend.tlsCAFile` may point either to local file or to `http` / `https` url.
  This global setting can be overridden at per-user level inside [`-auth.config`](#auth-config)
  via `tls_ca_file` option. For example:

  ```yaml
  - username: "foo"
    url_prefix: "https://localhost"
    tls_ca_file: "/path/to/tls/root/ca"
  ```

## IP filters

[Enterprise version](https://docs.victoriametrics.com/enterprise.html) of `vmauth` can be configured to allow / deny incoming requests via global and per-user IP filters.

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

The following config allows requests for the user 'foobar' only from the IP `127.0.0.1`:

```yaml
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

Please note, vmauth doesn't follow redirects. If destination redirects request to a new location, make sure this 
location is supported in vmauth `url_map` config.

## mTLS protection

By default `vmauth` accepts http requests at `8427` port (this port can be changed via `-httpListenAddr` command-line flags).
[Enterprise version of vmauth](https://docs.victoriametrics.com/enterprise.html) supports the ability to accept [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication)
requests at this port, by specifying `-tls` and `-mtls` command-line flags. For example, the following command runs `vmauth`, which accepts only mTLS requests at port `8427`:

```
./vmauth -tls -mtls -auth.config=...
```

By default system-wide [TLS Root CA](https://en.wikipedia.org/wiki/Root_certificate) is used for verifying client certificates if `-mtls` command-line flag is specified.
It is possible to specify custom TLS Root CA via `-mtlsCAFile` command-line flag.

See also [mTLS-based request routing](#mtls-based-request-routing).

## Security

It is expected that all the backend services protected by `vmauth` are located in an isolated private network, so they can be accessed by external users only via `vmauth`.

Do not transfer Basic Auth headers in plaintext over untrusted networks. Enable https at `-httpListenAddr`. This can be done by passing the following `-tls*` command-line flags to `vmauth`:

```sh
  -tls
     Whether to enable TLS for incoming HTTP requests at -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
     Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
     Path to file with TLS key. Used only if -tls is set
```

See [these docs](#mtls-protection) on how to enable [mTLS](https://en.wikipedia.org/wiki/Mutual_authentication) protection at `vmauth`.

Alternatively, [TLS termination proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy) may be put in front of `vmauth`.

It is recommended protecting the following endpoints with authKeys:
* `/-/reload` with `-reloadAuthKey` command-line flag, so external users couldn't trigger config reload.
* `/flags` with `-flagsAuthKey` command-line flag, so unauthorized users couldn't get application command-line flags.
* `/metrics` with `-metricsAuthKey` command-line flag, so unauthorized users couldn't get access to [vmauth metrics](#monitoring).
* `/debug/pprof` with `-pprofAuthKey` command-line flag, so unauthorized users couldn't get access to [profiling information](#profiling).

`vmauth` also supports the ability to restrict access by IP - see [these docs](#ip-filters). See also [concurrency limiting docs](#concurrency-limiting).

## Monitoring

`vmauth` exports various metrics in Prometheus exposition format at `http://vmauth-host:8427/metrics` page. It is recommended setting up regular scraping of this page
either via [vmagent](https://docs.victoriametrics.com/vmagent.html) or via Prometheus-compatible scraper, so the exported metrics could be analyzed later.

If you use Google Cloud Managed Prometheus for scraping metrics from VictoriaMetrics components, then pass `-metrics.exposeMetadata`
command-line to them, so they add `TYPE` and `HELP` comments per each exposed metric at `/metrics` page.
See [these docs](https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type) for details.

`vmauth` exports the following metrics per each defined user in [`-auth.config`](#auth-config):

* `vmauth_user_requests_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of requests served for the given `username`
* `vmauth_user_request_backend_errors_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of request errors for the given `username`
* `vmauth_user_request_duration_seconds` [summary](https://docs.victoriametrics.com/keyConcepts.html#summary) - the duration of requests for the given `username`
* `vmauth_user_concurrent_requests_limit_reached_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of failed requests
  for the given `username` because of exceeded [concurrency limits](#concurrency-limiting)
* `vmauth_user_concurrent_requests_capacity` [gauge](https://docs.victoriametrics.com/keyConcepts.html#gauge) - the maximum number of [concurrent requests](#concurrency-limiting)
  for the given `username`
* `vmauth_user_concurrent_requests_current` [gauge](https://docs.victoriametrics.com/keyConcepts.html#gauge) - the current number of [concurrent requests](#concurrency-limiting)
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

Additional labels for per-user metrics can be specified via `metric_labels` section. For example, the following config
defines `{dc="eu",team="dev"}` labels additionally to `username="foobar"` label:

```yaml
users:
- username: "foobar"
  metric_labels:
   dc: eu
   team: dev
  # other config options here
```

`vmauth` exports the following metrics if `unauthorized_user` section is defined in [`-auth.config`](#auth-config):

* `vmauth_unauthorized_user_requests_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of unauthorized requests served
* `vmauth_unauthorized_user_request_backend_errors_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of unauthorized request errors
* `vmauth_unauthorized_user_request_duration_seconds` [summary](https://docs.victoriametrics.com/keyConcepts.html#summary) - the duration of unauthorized requests
* `vmauth_unauthorized_user_concurrent_requests_limit_reached_total` [counter](https://docs.victoriametrics.com/keyConcepts.html#counter) - the number of failed unauthorized requests
  because of exceeded [concurrency limits](#concurrency-limiting)
* `vmauth_unauthorized_user_concurrent_requests_capacity` [gauge](https://docs.victoriametrics.com/keyConcepts.html#gauge) - the maximum number
  of [concurrent unauthorized requests](#concurrency-limiting)
* `vmauth_user_concurrent_requests_current` [gauge](https://docs.victoriametrics.com/keyConcepts.html#gauge) - the current number of [concurrent unauthorized requests](#concurrency-limiting)

## How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - `vmauth` is located in `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.22.
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
It is safe sharing the collected profiles from security point of view, since they do not contain sensitive information.

## Advanced usage

Pass `-help` command-line arg to `vmauth` in order to see all the configuration options:

```sh
./vmauth -help

vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmauth.html .

  -auth.config string
     Path to auth config. It can point either to local file or to http url. See https://docs.victoriametrics.com/vmauth.html for details on the format of this auth config
  -backend.TLSCAFile string
     Optional path to TLS root CA file, which is used for TLS verification when connecting to backends over HTTPS. See https://docs.victoriametrics.com/vmauth.html#backend-tls-setup
  -backend.tlsInsecureSkipVerify
     Whether to skip TLS verification when connecting to backends over HTTPS. See https://docs.victoriametrics.com/vmauth.html#backend-tls-setup
  -configCheckInterval duration
     interval for config file re-read. Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
  -failTimeout duration
     Sets a delay period for load balancing to skip a malfunctioning backend (default 3s)
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
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
     TCP address to listen for incoming http requests. See also -tls and -httpListenAddr.useProxyProtocol
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
     Lisense key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed via file specified by -licenseFile command-line flag
  -license.forceOffline
     Whether to enable offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification. This flag is avilable only in Enterprise binaries
  -licenseFile string
     Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed inline via -license command-line flag
  -loadBalancingPolicy string
     The default load balancing policy to use for backend urls specified inside url_prefix section. Supported policies: least_loaded, first_available. See https://docs.victoriametrics.com/vmauth.html#load-balancing for more details (default "least_loaded")
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
     The maximum length of a single logged argument. Longer arguments are replaced with 'arg_start..arg_end', where 'arg_start' and 'arg_end' is prefix and suffix of the arg with the length not exceeding -loggerMaxArgLen / 2 (default 1000)
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
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pprofAuthKey value
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
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
  -reloadAuthKey value
     Auth key for /-/reload http endpoint. It must be passed as authKey=...
     Flag value can be read from the given file when using -reloadAuthKey=file:///abs/path/to/file or -reloadAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -reloadAuthKey=http://host/path or -reloadAuthKey=https://host/path
  -responseTimeout duration
     The timeout for receiving a response from backend (default 5m0s)
  -retryStatusCodes array
     Comma-separated list of default HTTP response status codes when vmauth re-tries the request on other backends. See https://docs.victoriametrics.com/vmauth.html#load-balancing for details (default 0)
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to default value.
  -tls array
     Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -tlsCertFile array
     Path to file with TLS certificate for the corresponding -httpListenAddr if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsCipherSuites array
     Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsKeyFile array
     Path to file with TLS key for the corresponding -httpListenAddr if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsMinVersion array
     Optional minimum TLS version to use for the corresponding -httpListenAddr if -tls is set. Supported values: TLS10, TLS11, TLS12, TLS13
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -version
     Show VictoriaMetrics version
```
