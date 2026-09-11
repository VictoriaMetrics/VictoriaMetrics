---
weight: 14
title: Security model
menu:
  docs:
    identifier: vm-security-model
    parent: victoriametrics
    weight: 14
tags:
  - guide
  - metrics
  - security
aliases:
- /Security-Model.html
- /security-model/index.html
- /security-model/
---

## Trust model

VictoriaMetrics components must run in a protected private network without direct access from untrusted networks such as the Internet.

By default, VictoriaMetrics components expose HTTP APIs without mandatory authentication.
All requests from untrusted networks to VictoriaMetrics components must go through an auth proxy.
Only components designed for public exposure with TLS termination and authorization should be reachable from untrusted networks.

VictoriaMetrics does not provide a built-in user database, role model, or default authentication layer for every component.
Authentication, authorization, TLS termination, tenant routing, and rate limiting must be configured by the operator.

For most deployments, [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/) is the recommended authorization and routing layer in front of VictoriaMetrics components. Basic auth and TLS can also be configured directly on most components with `-httpAuth.*` and `-tls*` flags.

Tenant separation is not a substitute for authentication and authorization at the network or proxy layer.
Operators are responsible for limiting resource exhaustion from expensive or high-volume workloads.

The following components may be exposed to untrusted networks when they are configured with proper authentication, authorization, and TLS:

- `vmauth` for authentication, authorization, tenant routing, and request proxying.
- `vmgateway` for Enterprise deployments that need additional gateway features such as rate limiting.
- `vmagent` only when it intentionally receives pushed metrics from external sources. If `vmagent` scrapes internal targets, keep it private.

All other VictoriaMetrics services should be treated as private operational components unless their documentation explicitly describes a public-facing deployment mode.

VictoriaMetrics does not encrypt data at rest by itself.
Snapshots and backups have the same at-rest exposure as the underlying storage.

Authentication credentials such as basic auth, bearer tokens, and JWTs must not be sent over plain HTTP. Use TLS or mTLS for traffic that crosses untrusted networks.

## Single-node VictoriaMetrics

Single-node VictoriaMetrics listens on port `8428` by default.
The same port serves the full read, write, administrative, UI, and service API surface.

Access to the single-node API or to the storage path configured via `-storageDataPath` allows access to data for the whole instance.

Restrict access to the whole `:8428` HTTP API surface.
See [general security recommendations](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#general-security-recommendations) and [protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#protecting-service-endpoints) for the list of security-related flags and endpoints that require protection.

Single-node VictoriaMetrics can scrape targets itself.
When single-node VictoriaMetrics scrapes untrusted targets, treat these targets as untrusted input sources and use relabeling to drop sensitive labels before storage when needed.

## Cluster components

VictoriaMetrics cluster separates ingestion, querying, and storage into `vminsert`, `vmselect`, and `vmstorage`.

Keep cluster components on a private network, and route external access through an auth proxy.

See [cluster security recommendations](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#security) for cluster-specific security guidance.

### vminsert

`vminsert` listens on port `8480` by default and accepts writes.
Direct access to `vminsert` allows arbitrary writes, including overwriting existing series, and can drive high-cardinality growth that increases load on `vmstorage`.

The `/insert/multitenant/` endpoint allows the tenant to be supplied by the request.
Direct access to this endpoint allows writing to arbitrary tenants.
Expose it only to trusted sources allowed to select tenants.

`vminsert` can apply relabeling via `-relabelConfig`. Use this as a final ingestion-side control for dropping sensitive labels or rejecting unwanted series before they reach `vmstorage`.

### vmselect

`vmselect` listens on port `8481` by default and serves query APIs.
Direct access to `vmselect` allows querying stored data and calling expensive or administrative APIs.

The `/admin/tenants` endpoint lists registered tenants and should be restricted to trusted clients.

The delete-series endpoint at `/delete/<accountID>/prometheus/api/v1/admin/tsdb/delete_series` permanently deletes data and should be protected with `-deleteAuthKey`.

When `vmselect` serves multitenant read endpoints, `vmauth` must explicitly set `extra_label`, `extra_filters`, and `extra_filters[]` in the backend `url_prefix` to prevent a client from bypassing tenant restrictions. See [vmauth security recommendations](https://docs.victoriametrics.com/victoriametrics/vmauth/#security) for a configuration example.

### vmstorage

`vmstorage` listens on the following ports by default:

- `8400` for `vminsert` RPC traffic.
- `8401` for `vmselect` RPC traffic.
- `8482` for HTTP administrative and service endpoints.

`vmstorage` is not intended to be reachable from external clients.
Direct access to `vmstorage` allows affecting all tenants stored on that node.
Filesystem, container, or host access to the path configured via `-storageDataPath` allows direct access to stored data for all tenants on that node.

Restrict access to the `vmstorage` HTTP API surface.
See [cluster security recommendations](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#security) and [API examples](https://docs.victoriametrics.com/victoriametrics/url-examples/) for endpoint-specific protection guidance.

## vmauth

[`vmauth`](https://docs.victoriametrics.com/victoriametrics/vmauth/) is the recommended trust boundary for VictoriaMetrics deployments.
It can authenticate, authorize, route, and load balance requests to VictoriaMetrics components or other HTTP backends.

`vmauth` is entirely configuration-driven.
If a route is configured without authentication, `vmauth` will proxy it without authentication.

See [vmauth security recommendations](https://docs.victoriametrics.com/victoriametrics/vmauth/#security) for additional configuration and endpoint protection guidance.

## vmgateway

[`vmgateway`](https://docs.victoriametrics.com/victoriametrics/vmgateway/) is an Enterprise HTTP gateway for VictoriaMetrics.
It can proxy read and write traffic and provide tenant-aware rate limiting for cluster deployments.

`vmgateway` access control is deprecated.
Use [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/#jwt-token-auth-proxy) for JWT authentication and request routing.
See [Access Control migration to vmauth](https://docs.victoriametrics.com/victoriametrics/vmgateway/#access-control-migration-to-vmauth) for migration guidance.

Direct access to a misconfigured `vmgateway` may allow sending read or write traffic to backend VictoriaMetrics components or bypassing intended rate limits.

## vmagent

`vmagent` listens on port `8429` by default.
It can scrape targets and accept pushed metrics through multiple ingestion protocols.
It converts received data to remote write before forwarding it.

If `vmagent` accepts pushed metrics from external sources, expose it only through an authenticated and TLS-protected endpoint.
If `vmagent` only scrapes internal targets, keep it on a private network.

`vmagent` can send data directly to `vmauth` or `vminsert`.
When external clients or untrusted tenants are involved, send traffic through `vmauth` so writes can be authenticated and routed to the intended tenant.

`vmagent` can preprocess metrics before forwarding them.
Use these controls to drop sensitive labels, reduce duplicate samples, aggregate high-volume streams, and limit accidental or malicious cardinality growth.

See [vmagent security recommendations](https://docs.victoriametrics.com/victoriametrics/vmagent/#security) for endpoint protection and mTLS guidance.

## vmalert

See [vmalert security recommendations](https://docs.victoriametrics.com/victoriametrics/vmalert/#security) for additional configuration and endpoint protection guidance.

## TLS

### mTLS protection

The Enterprise version of VictoriaMetrics can enable mTLS between `vminsert`, `vmselect`, and `vmstorage` with `-cluster.tls`, `-cluster.tlsCAFile`, and related TLS flags.

### Certificate verification

Several VictoriaMetrics components and tools support options that disable TLS certificate verification, such as `insecure_skip_verify`, `-cluster.tlsInsecureSkipVerify`, and `-s3TLSInsecureSkipVerify`.

Disabling certificate verification allows a network attacker to impersonate the remote endpoint.
Use these options only for local development or controlled testing.
Do not use them in production.

## API security

Anyone with direct network access to a VictoriaMetrics HTTP API can reach every endpoint that component serves.

Individual sensitive endpoints can be protected with `-*AuthKey` flags, such as `-metricsAuthKey`, `-reloadAuthKey`, `-pprofAuthKey`, `-snapshotAuthKey`, and `-deleteAuthKey`. These flags provide defense in depth but are not a substitute for `vmauth` or another authenticating proxy in front of the component.

See [protecting service endpoints](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#protecting-service-endpoints) for the full list of endpoints and their protection flags.

## Denial of Service

VictoriaMetrics provides configurable resource limits, but these limits do not remove the need for capacity planning.
A deployment can still be overloaded when normal or misconfigured workload exceeds the resources available to it.

In a cluster deployment, a single noisy tenant can degrade shared `vmstorage` capacity, because tenant separation is not a resource isolation boundary.

Configure query and ingestion limits appropriate for the exposed workload.
See [single-node resource usage limits](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#resource-usage-limits) and [cluster resource usage limits](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#resource-usage-limits) for the available flags, and the [vmagent cardinality limiter](https://docs.victoriametrics.com/victoriametrics/vmagent/#cardinality-limiter) for reducing cardinality upstream of storage.

Enforce request and concurrency limits at the trust boundary with `vmauth` or `vmgateway`, so untrusted or misbehaving clients cannot exhaust backend resources.
See [vmauth concurrency limiting](https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting).

Operators are responsible for provisioning the underlying resources and for monitoring each component so failures can be detected and restarted.

<!-- TODO(reviewer): should we state that pure resource-exhaustion / DoS issues are not treated as security vulnerabilities? -->

## Secrets

Metric samples are not credentials, but metric names, labels, and query results can leak information about the environment they describe.

VictoriaMetrics-managed secrets such as authentication credentials and `-*AuthKey` values should not be passed inline on the command line.
Password and auth-key flags accept `file://` references and re-read the file on access, so secrets can be rotated by updating the file without restarting the process.

Protect `/flags` and other configuration endpoints.
Even with secret values masked, these endpoints still expose deployment structure.

## VictoriaLogs and VictoriaTraces

VictoriaLogs and VictoriaTraces follow the same trust-boundary pattern as VictoriaMetrics.

See [VictoriaLogs security and load balancing](https://docs.victoriametrics.com/victorialogs/security-and-lb/) for VictoriaLogs-specific security guidance.

See [VictoriaTraces security docs](https://docs.victoriametrics.com/victoriatraces/#security) for VictoriaTraces-specific security guidance.

## Vulnerability reporting

Supported VictoriaMetrics versions receive security fixes for the latest release and for [LTS releases](https://docs.victoriametrics.com/victoriametrics/lts-releases/).

Report security issues privately to <security@victoriametrics.com>.
See the [VictoriaMetrics disclosure policy](https://victoriametrics.com/legal/disclosure-policy/) for coordinated disclosure details.
