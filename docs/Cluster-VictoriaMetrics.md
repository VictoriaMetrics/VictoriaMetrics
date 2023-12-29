---
sort: 2
weight: 2
menu:
  docs:
    parent: 'victoriametrics'
    weight: 2
title: Cluster version
aliases:
  - /Cluster-VictoriaMetrics.html
---
# Cluster version

<img src="logo.webp" width="300">

VictoriaMetrics is a fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.

It is recommended to use the [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics) instead of the cluster version
for ingestion rates lower than a million data points per second.
The single-node version [scales perfectly](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
with the number of CPU cores, RAM and available storage space.
The single-node version is easier to configure and operate compared to the cluster version, so think twice before choosing the cluster version.
See [this question](https://docs.victoriametrics.com/FAQ.html#which-victoriametrics-type-is-recommended-for-use-in-production---single-node-or-cluster) for more details.

There is also user-friendly database for logs - [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/).

If you have questions about VictoriaMetrics, then feel free asking them at [VictoriaMetrics community Slack chat](https://slack.victoriametrics.com/).

[Contact us](mailto:info@victoriametrics.com) if you need enterprise support for VictoriaMetrics. 
See [features available in enterprise package](https://docs.victoriametrics.com/enterprise.html).
Enterprise binaries can be downloaded and evaluated for free 
from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).

VictoriaMetrics is developed at a fast pace, so it is recommended periodically checking the [CHANGELOG](https://docs.victoriametrics.com/CHANGELOG.html) and performing [regular upgrades](#how-to-upgrade-victoriametrics).

VictoriaMetrics has achieved security certifications for Database Software Development and Software-Based Monitoring Services. We apply strict security measures in everything we do. See our [Security page](https://victoriametrics.com/security/) for more details.

## Prominent features

- Supports all the features of the [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics).
- Performance and capacity scale horizontally. See [these docs for details](#cluster-resizing-and-scalability).
- Supports multiple independent namespaces for time series data (aka multi-tenancy). See [these docs for details](#multitenancy).
- Supports replication. See [these docs for details](#replication-and-data-safety).

## Architecture overview

VictoriaMetrics cluster consists of the following services:

- `vmstorage` - stores the raw data and returns the queried data on the given time range for the given label filters
- `vminsert` - accepts the ingested data and spreads it among `vmstorage` nodes according to consistent hashing over metric name and all its labels
- `vmselect` - performs incoming queries by fetching the needed data from all the configured `vmstorage` nodes

Each service may scale independently and may run on the most suitable hardware.
`vmstorage` nodes don't know about each other, don't communicate with each other and don't share any data.
This is a [shared nothing architecture](https://en.wikipedia.org/wiki/Shared-nothing_architecture).
It increases cluster availability, and simplifies cluster maintenance as well as cluster scaling.

<p align="center">
  <img src="Cluster-VictoriaMetrics_cluster-scheme.webp" width="800">
</p>

## Multitenancy

VictoriaMetrics cluster supports multiple isolated tenants (aka namespaces).
Tenants are identified by `accountID` or `accountID:projectID`, which are put inside request urls.
See [these docs](#url-format) for details.

Some facts about tenants in VictoriaMetrics:

- Each `accountID` and `projectID` is identified by an arbitrary 32-bit integer in the range `[0 .. 2^32)`.
If `projectID` is missing, then it is automatically assigned to `0`. It is expected that other information about tenants
such as auth tokens, tenant names, limits, accounting, etc. is stored in a separate relational database. This database must be managed
by a separate service sitting in front of VictoriaMetrics cluster such as [vmauth](https://docs.victoriametrics.com/vmauth.html)
or [vmgateway](https://docs.victoriametrics.com/vmgateway.html). [Contact us](mailto:info@victoriametrics.com) if you need assistance with such service.

- Tenants are automatically created when the first data point is written into the given tenant.

- Data for all the tenants is evenly spread among available `vmstorage` nodes. This guarantees even load among `vmstorage` nodes
when different tenants have different amounts of data and different query load.

- The database performance and resource usage doesn't depend on the number of tenants. It depends mostly on the total number of active time series in all the tenants. A time series is considered active if it received at least a single sample during the last hour or it has been touched by queries during the last hour.

- VictoriaMetrics doesn't support querying multiple tenants in a single request.

- The list of registered tenants can be obtained via `http://<vmselect>:8481/admin/tenants` url. See [these docs](#url-format).

- VictoriaMetrics exposes various per-tenant statistics via metrics - see [these docs](https://docs.victoriametrics.com/PerTenantStatistic.html).

See also [multitenancy via labels](#multitenancy-via-labels).


## Multitenancy via labels

`vminsert` can accept data from multiple [tenants](#multitenancy) via a special `multitenant` endpoints `http://vminsert:8480/insert/multitenant/<suffix>`,
where `<suffix>` can be replaced with any supported suffix for data ingestion from [this list](#url-format).
In this case the account id and project id are obtained from optional `vm_account_id` and `vm_project_id` labels of the incoming samples.
If `vm_account_id` or `vm_project_id` labels are missing or invalid, then the corresponding `accountID` or `projectID` is set to 0.
These labels are automatically removed from samples before forwarding them to `vmstorage`.
For example, if the following samples are written into `http://vminsert:8480/insert/multitenant/prometheus/api/v1/write`:

```
http_requests_total{path="/foo",vm_account_id="42"} 12
http_requests_total{path="/bar",vm_account_id="7",vm_project_id="9"} 34
```

Then the `http_requests_total{path="/foo"} 12` would be stored in the tenant `accountID=42, projectID=0`,
while the `http_requests_total{path="/bar"} 34` would be stored in the tenant `accountID=7, projectID=9`.

The `vm_account_id` and `vm_project_id` labels are extracted after applying the [relabeling](https://docs.victoriametrics.com/relabeling.html)
set via `-relabelConfig` command-line flag, so these labels can be set at this stage.

The `vm_account_id` and `vm_project_id` labels are also taken into account when ingesting data via non-http-based protocols
such as [Graphite](https://docs.victoriametrics.com/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd),
[InfluxDB line protocol via TCP and UDP](https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf) and
[OpenTSDB telnet put protocol](https://docs.victoriametrics.com/#sending-data-via-telnet-put-protocol).

**Security considerations:** it is recommended restricting access to `multitenant` endpoints only to trusted sources,
since untrusted source may break per-tenant data by writing unwanted samples to arbitrary tenants.


## Binaries

Compiled binaries for the cluster version are available in the `assets` section of the [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
Also see archives containing the word `cluster`.

Docker images for the cluster version are available here:

- `vminsert` - <https://hub.docker.com/r/victoriametrics/vminsert/tags>
- `vmselect` - <https://hub.docker.com/r/victoriametrics/vmselect/tags>
- `vmstorage` - <https://hub.docker.com/r/victoriametrics/vmstorage/tags>

## Building from sources

The source code for the cluster version is available in the [cluster branch](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

### Production builds

There is no need to install Go on a host system since binaries are built
inside [the official docker container for Go](https://hub.docker.com/_/golang).
This allows reproducible builds.
So [install docker](https://docs.docker.com/install/) and run the following command:

```
make vminsert-prod vmselect-prod vmstorage-prod
```

Production binaries are built into statically linked binaries. They are put into the `bin` folder with `-prod` suffixes:

```
$ make vminsert-prod vmselect-prod vmstorage-prod
$ ls -1 bin
vminsert-prod
vmselect-prod
vmstorage-prod
```

### Development Builds

1. [Install go](https://golang.org/doc/install). The minimum supported version is Go 1.20.
1. Run `make` from [the repository root](https://github.com/VictoriaMetrics/VictoriaMetrics). It should build `vmstorage`, `vmselect`
   and `vminsert` binaries and put them into the `bin` folder.

### Building docker images

Run `make package`. It will build the following docker images locally:

- `victoriametrics/vminsert:<PKG_TAG>`
- `victoriametrics/vmselect:<PKG_TAG>`
- `victoriametrics/vmstorage:<PKG_TAG>`

`<PKG_TAG>` is auto-generated image tag, which depends on source code in [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package`.

By default, images are built on top of [alpine](https://hub.docker.com/_/scratch) image in order to improve debuggability.
It is possible to build an image on top of any other base image by setting it via `<ROOT_IMAGE>` environment variable.
For example, the following command builds images on top of [scratch](https://hub.docker.com/_/scratch) image:

```console
ROOT_IMAGE=scratch make package
```

## Operation

## Cluster setup

A minimal cluster must contain the following nodes:

- a single `vmstorage` node with `-retentionPeriod` and `-storageDataPath` flags
- a single `vminsert` node with `-storageNode=<vmstorage_host>`
- a single `vmselect` node with `-storageNode=<vmstorage_host>`

[Enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise.html) supports automatic discovering and updating of `vmstorage` nodes.
See [these docs](#automatic-vmstorage-discovery) for details.

It is recommended to run at least two nodes for each service for high availability purposes. In this case the cluster continues working when a single node is temporarily unavailable and the remaining nodes can handle the increased workload. The node may be temporarily unavailable when the underlying hardware breaks, during software upgrades, migration or other maintenance tasks.

It is preferred to run many small `vmstorage` nodes over a few big `vmstorage` nodes, since this reduces the workload increase on the remaining `vmstorage` nodes when some of `vmstorage` nodes become temporarily unavailable.

An http load balancer such as [vmauth](https://docs.victoriametrics.com/vmauth.html) or `nginx` must be put in front of `vminsert` and `vmselect` nodes.
It must contain the following routing configs according to [the url format](#url-format):

- requests starting with `/insert` must be routed to port `8480` on `vminsert` nodes.
- requests starting with `/select` must be routed to port `8481` on `vmselect` nodes.

Ports may be altered by setting `-httpListenAddr` on the corresponding nodes.

It is recommended setting up [monitoring](#monitoring) for the cluster.

The following tools can simplify cluster setup:
- [An example docker-compose config for VictoriaMetrics cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/docker-compose-cluster.yml)
- [Helm charts for VictoriaMetrics](https://github.com/VictoriaMetrics/helm-charts)
- [Kubernetes operator for VictoriaMetrics](https://github.com/VictoriaMetrics/operator)

It is possible manually setting up a toy cluster on a single host. In this case every cluster component - `vminsert`, `vmselect` and `vmstorage` - must have distinct values for `-httpListenAddr` command-line flag. This flag specifies http address for accepting http requests for [monitoring](#monitoring) and [profiling](#profiling). `vmstorage` node must have distinct values for the following additional command-line flags in order to prevent resource usage clash:
- `-storageDataPath` - every `vmstorage` node must have a dedicated data storage.
- `-vminsertAddr` - every `vmstorage` node must listen for a distinct tcp address for accepting data from `vminsert` nodes.
- `-vmselectAddr` - every `vmstorage` node must listen for a distinct tcp address for accepting requests from `vmselect` nodes.

### Environment variables

All the VictoriaMetrics components allow referring environment variables in command-line flags via `%{ENV_VAR}` syntax.
For example, `-metricsAuthKey=%{METRICS_AUTH_KEY}` is automatically expanded to `-metricsAuthKey=top-secret`
if `METRICS_AUTH_KEY=top-secret` environment variable exists at VictoriaMetrics startup.
This expansion is performed by VictoriaMetrics itself.

VictoriaMetrics recursively expands `%{ENV_VAR}` references in environment variables on startup.
For example, `FOO=%{BAR}` environment variable is expanded to `FOO=abc` if `BAR=a%{BAZ}` and `BAZ=bc`.

Additionally, all the VictoriaMetrics components allow setting flag values via environment variables according to these rules:

- The `-envflag.enable` flag must be set
- Each `.` in flag names must be substituted by `_` (for example `-insert.maxQueueDuration <duration>` will translate to `insert_maxQueueDuration=<duration>`)
- For repeating flags, an alternative syntax can be used by joining the different values into one using `,` as separator (for example `-storageNode <nodeA> -storageNode <nodeB>` will translate to `storageNode=<nodeA>,<nodeB>`)
- It is possible setting prefix for environment vars with `-envflag.prefix`. For instance, if `-envflag.prefix=VM_`, then env vars must be prepended with `VM_`

## Automatic vmstorage discovery

`vminsert` and `vmselect` components in [enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise.html) support
the following approaches for automatic discovery of `vmstorage` nodes:

- file-based discovery - put the list of `vmstorage` nodes into a file - one node address per each line - and then pass `-storageNode=file:/path/to/file-with-vmstorage-list`
  to `vminsert` and `vmselect`. It is possible to read the list of vmstorage nodes from http or https urls.
  For example, `-storageNode=file:http://some-host/vmstorage-list` would read the list of storage nodes
  from `http://some-host/vmstorage-list`.
  The list of discovered `vmstorage` nodes is automatically updated when the file contents changes.
  The update frequency can be controlled with `-storageNode.discoveryInterval` command-line flag.

- [dns+srv](https://en.wikipedia.org/wiki/SRV_record) - pass `dns+srv:some-name` value to `-storageNode` command-line flag.
  In this case the provided `dns+srv` names are resolved into tcp addresses of `vmstorage` nodes.
  The list of discovered `vmstorage` nodes is automatically updated at `vminsert` and `vmselect`
  when it changes behind the corresponding `dns+srv` names.
  The update frequency can be controlled with `-storageNode.discoveryInterval` command-line flag.

It is possible passing multiple `file` and `dns+srv` names to `-storageNode` command-line flag. In this case all these names
are resolved to tcp addresses of `vmstorage` nodes to connect to.
For example, `-storageNode=file:/path/to/local-vmstorage-list -storageNode='dns+srv:vmstorage-hot' -storageNode='dns+srv:vmstorage-cold'`.

It is OK to pass regular static `vmstorage` addresses together with `file` and `dns+srv` addresses at `-storageNode`. For example,
`-storageNode=vmstorage1,vmstorage2 -storageNode='dns+srv:vmstorage-autodiscovery'`.

The discovered addresses can be filtered with optional `-storageNode.filter` command-line flag, which can contain arbitrary regular expression filter.
For example, `-storageNode.filter='^[^:]+:8400$'` would leave discovered addresses ending with `8400` port only, e.g. the default port used
for sending data from `vminsert` to `vmstorage` node according to `-vminsertAddr` command-line flag.

The currently discovered `vmstorage` nodes can be [monitored](#monitoring) with `vm_rpc_vmstorage_is_reachable` and `vm_rpc_vmstorage_is_read_only` metrics.

## Security

General security recommendations:

- All the VictoriaMetrics cluster components must run in protected private network without direct access from untrusted networks such as Internet.
- External clients must access `vminsert` and `vmselect` via auth proxy such as [vmauth](https://docs.victoriametrics.com/vmauth.html)
  or [vmgateway](https://docs.victoriametrics.com/vmgateway.html).
- The auth proxy must accept auth tokens from untrusted networks only via https in order to protect the auth tokens from MitM attacks.
- It is recommended using distinct auth tokens for distinct [tenants](#multitenancy) in order to reduce potential damage in case of compromised auth token for some tenants.
- Prefer using lists of allowed [API endpoints](#url-format), while disallowing access to other endpoints when configuring auth proxy in front of `vminsert` and `vmselect`.
  This minimizes attack surface.

See also [security recommendation for single-node VictoriaMetrics](https://docs.victoriametrics.com/#security)
and [the general security page at VictoriaMetrics website](https://victoriametrics.com/security/).


## mTLS protection

By default `vminsert` and `vmselect` nodes use unencrypted connections to `vmstorage` nodes, since it is assumed that all the cluster components [run in a protected environment](#security). [Enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise.html) provides optional support for [mTLS connections](https://en.wikipedia.org/wiki/Mutual_authentication#mTLS) between cluster components. Pass `-cluster.tls=true` command-line flag to `vminsert`, `vmselect` and `vmstorage` nodes in order to enable mTLS protection. Additionally, `vminsert`, `vmselect` and `vmstorage` must be configured with mTLS certificates via `-cluster.tlsCertFile`, `-cluster.tlsKeyFile` command-line options. These certificates are mutually verified when `vminsert` and `vmselect` dial `vmstorage`.

The following optional command-line flags related to mTLS are supported:

- `-cluster.tlsInsecureSkipVerify` can be set at `vminsert`, `vmselect` and `vmstorage` in order to disable peer certificate verification. Note that this breaks security.
- `-cluster.tlsCAFile` can be set at `vminsert`, `vmselect` and `vmstorage` for verifying peer certificates issued with custom [certificate authority](https://en.wikipedia.org/wiki/Certificate_authority). By default, system-wide certificate authority is used for peer certificate verification.
- `-cluster.tlsCipherSuites` can be set to the list of supported TLS cipher suites at `vmstorage`. See [the list of supported TLS cipher suites](https://pkg.go.dev/crypto/tls#pkg-constants).

When `vmselect` runs with `-clusternativeListenAddr` command-line option, then it can be configured with `-clusternative.tls*` options similar to `-cluster.tls*` for accepting `mTLS` connections from top-level `vmselect` nodes in [multi-level cluster setup](#multi-level-cluster-setup).

See [these docs](https://gist.github.com/f41gh7/76ed8e5fb1ebb9737fe746bae9175ee6) on how to set up mTLS in VictoriaMetrics cluster.

[Enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise.html) can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).

## Monitoring

All the cluster components expose various metrics in Prometheus-compatible format at `/metrics` page on the TCP port set in `-httpListenAddr` command-line flag.
By default, the following TCP ports are used:

- `vminsert` - 8480
- `vmselect` - 8481
- `vmstorage` - 8482

It is recommended setting up [vmagent](https://docs.victoriametrics.com/vmagent.html)
or Prometheus to scrape `/metrics` pages from all the cluster components, so they can be monitored and analyzed
with [the official Grafana dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11176-victoriametrics-cluster/)
or [an alternative dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11831). Graphs on these dashboards contain useful hints - hover the `i` icon at the top left corner of each graph in order to read it.

It is recommended setting up alerts in [vmalert](https://docs.victoriametrics.com/vmalert.html) or in Prometheus from [this list](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts).
See more details in the article [VictoriaMetrics Monitoring](https://victoriametrics.com/blog/victoriametrics-monitoring/).

## Cardinality limiter

`vmstorage` nodes can be configured with limits on the number of unique time series across all the tenants with the following command-line flags:

- `-storage.maxHourlySeries` is the limit on the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series) during the last hour.
- `-storage.maxDailySeries` is the limit on the number of unique time series during the day. This limit can be used for limiting daily [time series churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).

Note that these limits are set and applied individually per each `vmstorage` node in the cluster. So, if the cluster has `N` `vmstorage` nodes, then the cluster-level limits will be `N` times bigger than the per-`vmstorage` limits.

See more details about cardinality limiter in [these docs](https://docs.victoriametrics.com/#cardinality-limiter).

## Troubleshooting

- If your VictoriaMetrics cluster experiences data ingestion delays during
  [rolling restarts and configuration updates](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#updating--reconfiguring-cluster-nodes),
  then see [these docs](#improving-re-routing-performance-during-restart).

[Troubleshooting docs for single-node VictoriaMetrics](https://docs.victoriametrics.com/Troubleshooting.html) apply to VictoriaMetrics cluster as well.

## Readonly mode

`vmstorage` nodes automatically switch to readonly mode when the directory pointed by `-storageDataPath`
contains less than `-storage.minFreeDiskSpaceBytes` of free space. `vminsert` nodes stop sending data to such nodes
and start re-routing the data to the remaining `vmstorage` nodes.

`vmstorage` sets `vm_storage_is_read_only` metric at `http://vmstorage:8482/metrics` to `1` when it enters read-only mode.
The metric is set to `0` when the `vmstorage` isn't in read-only mode.


## URL format

The main differences between URL formats of cluster and [Single server](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html)
versions are that cluster has separate components for read and ingestion path, and because of multi-tenancy support.
Also in the cluster version the `/prometheus/api/v1` endpoint ingests  `jsonl`, `csv`, `native` and `prometheus` data formats **not** only `prometheus` data.
Check practical examples of VictoriaMetrics API [here](https://docs.victoriametrics.com/url-examples.html).

- URLs for data ingestion: `http://<vminsert>:8480/insert/<accountID>/<suffix>`, where:
  - `<accountID>` is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). It is possible to set it as `accountID:projectID`,
    where `projectID` is also arbitrary 32-bit integer. If `projectID` isn't set, then it equals to `0`. See [multitenancy docs](#multitenancy) for more details.
    The `<accountID>` can be set to `multitenant` string, e.g. `http://<vminsert>:8480/insert/multitenant/<suffix>`. Such urls accept data from multiple tenants
    specified via `vm_account_id` and `vm_project_id` labels. See [multitenancy via labels](#multitenancy-via-labels) for more details.
  - `<suffix>` may have the following values:
    - `prometheus` and `prometheus/api/v1/write` - for ingesting data with [Prometheus remote write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write).
    - `prometheus/api/v1/import` - for importing data obtained via `api/v1/export` at `vmselect` (see below), JSON line format.
    - `prometheus/api/v1/import/native` - for importing data obtained via `api/v1/export/native` on `vmselect` (see below).
    - `prometheus/api/v1/import/csv` - for importing arbitrary CSV data. See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-csv-data) for details.
    - `prometheus/api/v1/import/prometheus` - for importing data in [Prometheus text exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format) and in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md). This endpoint also supports [Pushgateway protocol](https://github.com/prometheus/pushgateway#url). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-prometheus-exposition-format) for details.
    - `opentelemetry/api/v1/push` - for ingesting data via [OpenTelemetry protocol for metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/ffddc289462dfe0c2041e3ca42a7b1df805706de/specification/metrics/data-model.md). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#sending-data-via-opentelemetry).
    - `datadog/api/v1/series` - for ingesting data with DataDog submit metrics API v1. See [these docs](https://docs.victoriametrics.com/url-examples.html#datadogapiv1series) for details.
    - `datadog/api/v2/series` - for ingesting data with [DataDog submit metrics API](https://docs.datadoghq.com/api/latest/metrics/#submit-metrics). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-datadog-agent) for details.
    - `influx/write` and `influx/api/v2/write` - for ingesting data with [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf) for details.
    - `newrelic/infra/v2/metrics/events/bulk` - for accepting data from [NewRelic infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent). See [these docs](https://docs.victoriametrics.com/#how-to-send-data-from-newrelic-agent) for details.
    - `opentsdb/api/put` - for accepting [OpenTSDB HTTP /api/put requests](http://opentsdb.net/docs/build/html/api_http/put.html). This handler is disabled by default. It is exposed on a distinct TCP address set via `-opentsdbHTTPListenAddr` command-line flag. See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#sending-opentsdb-data-via-http-apiput-requests) for details.

- URLs for [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/): `http://<vmselect>:8481/select/<accountID>/prometheus/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying data namespace for the query (aka tenant)
  - `<suffix>` may have the following values:
    - `api/v1/query` - performs [PromQL instant query](https://docs.victoriametrics.com/keyConcepts.html#instant-query).
    - `api/v1/query_range` - performs [PromQL range query](https://docs.victoriametrics.com/keyConcepts.html#range-query).
    - `api/v1/series` - performs [series query](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers).
    - `api/v1/labels` - returns a [list of label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names).
    - `api/v1/label/<label_name>/values` - returns values for the given `<label_name>` according [to API](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values).
    - `federate` - returns [federated metrics](https://prometheus.io/docs/prometheus/latest/federation/).
    - `api/v1/export` - exports raw data in JSON line format. See [this article](https://medium.com/@valyala/analyzing-prometheus-data-with-external-tools-5f3e5e147639) for details.
    - `api/v1/export/native` - exports raw data in native binary format. It may be imported into another VictoriaMetrics via `api/v1/import/native` (see above).
    - `api/v1/export/csv` - exports data in CSV. It may be imported into another VictoriaMetrics via `api/v1/import/csv` (see above).
    - `api/v1/series/count` - returns the total number of series.
    - `api/v1/status/tsdb` - for time series stats. See [these docs](https://docs.victoriametrics.com/#tsdb-stats) for details.
    - `api/v1/status/active_queries` - for currently executed active queries. Note that every `vmselect` maintains an independent list of active queries,
      which is returned in the response.
    - `api/v1/status/top_queries` - for listing the most frequently executed queries and queries taking the most duration.
    - `metric-relabel-debug` - for debugging [relabeling rules](https://docs.victoriametrics.com/relabeling.html).

- URLs for [Graphite Metrics API](https://graphite-api.readthedocs.io/en/latest/api.html#the-metrics-api): `http://<vmselect>:8481/select/<accountID>/graphite/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying data namespace for query (aka tenant)
  - `<suffix>` may have the following values:
    - `render` - implements Graphite Render API. See [these docs](https://graphite.readthedocs.io/en/stable/render_api.html).
    - `metrics/find` - searches Graphite metrics. See [these docs](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find).
    - `metrics/expand` - expands Graphite metrics. See [these docs](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-expand).
    - `metrics/index.json` - returns all the metric names. See [these docs](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-index-json).
    - `tags/tagSeries` - registers time series. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb).
    - `tags/tagMultiSeries` - register multiple time series. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb).
    - `tags` - returns tag names. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags).
    - `tags/<tag_name>` - returns tag values for the given `<tag_name>`. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags).
    - `tags/findSeries` - returns series matching the given `expr`. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags).
    - `tags/autoComplete/tags` - returns tags matching the given `tagPrefix` and/or `expr`. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support).
    - `tags/autoComplete/values` - returns tag values matching the given `valuePrefix` and/or `expr`. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support).
    - `tags/delSeries` - deletes series matching the given `path`. See [these docs](https://graphite.readthedocs.io/en/stable/tags.html#removing-series-from-the-tagdb).

- URL with basic Web UI: `http://<vmselect>:8481/select/<accountID>/vmui/`.

- URL for query stats across all tenants: `http://<vmselect>:8481/api/v1/status/top_queries`. It lists with the most frequently executed queries and queries taking the most duration.

- URL for time series deletion: `http://<vmselect>:8481/delete/<accountID>/prometheus/api/v1/admin/tsdb/delete_series?match[]=<timeseries_selector_for_delete>`.
  Note that the `delete_series` handler should be used only in exceptional cases such as deletion of accidentally ingested incorrect time series. It shouldn't
  be used on a regular basis, since it carries non-zero overhead.

- URL for listing [tenants](#multitenancy) with the ingested data on the given time range: `http://<vmselect>:8481/admin/tenants?start=...&end=...` .
The `start` and `end` query args are optional. If they are missing, then all the tenants with at least one sample stored in VictoriaMetrics are returned.

- URL for accessing [vmalerts](https://docs.victoriametrics.com/vmalert.html) UI: `http://<vmselect>:8481/select/<accountID>/prometheus/vmalert/`.
  This URL works only when `-vmalert.proxyURL` flag is set. See more about vmalert [here](#vmalert). 

- `vmstorage` nodes provide the following HTTP endpoints on `8482` port:
  - `/internal/force_merge` - initiate [forced compactions](https://docs.victoriametrics.com/#forced-merge) on the given `vmstorage` node.
  - `/snapshot/create` - create [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282),
    which can be used for backups in background. Snapshots are created in `<storageDataPath>/snapshots` folder, where `<storageDataPath>` is the corresponding
    command-line flag value.
  - `/snapshot/list` - list available snapshots.
  - `/snapshot/delete?snapshot=<id>` - delete the given snapshot.
  - `/snapshot/delete_all` - delete all the snapshots.

  Snapshots may be created independently on each `vmstorage` node. There is no need in synchronizing snapshots' creation
  across `vmstorage` nodes.

## Cluster resizing and scalability

Cluster performance and capacity can be scaled up in two ways:

- By adding more resources (CPU, RAM, disk IO, disk space, network bandwidth) to existing nodes in the cluster (aka vertical scalability).
- By adding more nodes to the cluster (aka horizontal scalability).

General recommendations for cluster scalability:

- Adding more CPU and RAM to existing `vmselect` nodes improves the performance for heavy queries, which process big number of time series with big number of raw samples. See [this article on how to detect and optimize heavy queries](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986).
- Adding more `vmstorage` nodes increases the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series) the cluster can handle. This also increases query performance over time series with [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate). The cluster stability is also improved with the number of `vmstorage` nodes, since active `vmstorage` nodes need to handle lower additional workload when some of `vmstorage` nodes become unavailable.
- Adding more CPU and RAM to existing `vmstorage` nodes increases the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series) the cluster can handle. It is preferred to add more `vmstorage` nodes over adding more CPU and RAM to existing `vmstorage` nodes, since higher number of `vmstorage` nodes increases cluster stability and improves query performance over time series with [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
- Adding more `vminsert` nodes increases the maximum possible data ingestion speed, since the ingested data may be split among bigger number of `vminsert` nodes.
- Adding more `vmselect` nodes increases the maximum possible queries rate, since the incoming concurrent requests may be split among bigger number of `vmselect` nodes.

Steps to add `vmstorage` node:

1. Start new `vmstorage` node with the same `-retentionPeriod` as existing nodes in the cluster.
1. Gradually restart all the `vmselect` nodes with new `-storageNode` arg containing `<new_vmstorage_host>`.
1. Gradually restart all the `vminsert` nodes with new `-storageNode` arg containing `<new_vmstorage_host>`.

## Updating / reconfiguring cluster nodes

All the node types - `vminsert`, `vmselect` and `vmstorage` - may be updated via graceful shutdown.
Send `SIGINT` signal to the corresponding process, wait until it finishes and then start new version
with new configs.

There are the following cluster update / upgrade approaches exist:

### No downtime strategy

Gracefully restart every node in the cluster one-by-one with the updated config / upgraded binary.

It is recommended restarting the nodes in the following order:

1. Restart `vmstorage` nodes.
1. Restart `vminsert` nodes.
1. Restart `vmselect` nodes.

This strategy allows upgrading the cluster without downtime if the following conditions are met:

- The cluster has at least a pair of nodes of each type - `vminsert`, `vmselect` and `vmstorage`,
  so it can continue to accept new data and serve incoming requests when a single node is temporary unavailable
  during its restart. See [cluster availability docs](#cluster-availability) for details.
- The cluster has enough compute resources (CPU, RAM, network bandwidth, disk IO) for processing
  the current workload when a single node of any type (`vminsert`, `vmselect` or `vmstorage`)
  is temporarily unavailable during its restart.
- The updated config / upgraded binary is compatible with the remaining components in the cluster.
  See the [CHANGELOG](https://docs.victoriametrics.com/CHANGELOG.html) for compatibility notes between different releases.

If at least a single condition isn't met, then the rolling restart may result in cluster unavailability
during the config update / version upgrade. In this case the following strategy is recommended.

#### Improving re-routing performance during restart

`vmstorage` nodes may experience increased usage for CPU, RAM and disk IO during
[rolling restarts](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#no-downtime-strategy),
since they need to process higher load when some of `vmstorage` nodes are temporarily unavailable in the cluster.
It is possible to reduce resource usage spikes by running more `vminsert` nodes and by passing bigger values
to `-storage.vminsertConnsShutdownDuration` (available from [v1.95.0](https://docs.victoriametrics.com/CHANGELOG.html#v1950))
command-line flag at `vmstorage` nodes.
In this case `vmstorage` increases the interval between gradual closing of `vminsert` connections during graceful shutdown.
This reduces data ingestion slowdown during rollout restarts.

Make sure that the `-storage.vminsertConnsShutdownDuration` is smaller than the graceful shutdown timeout configured at the system which manages `vmstorage`
(e.g. Docker, Kubernetes, systemd, etc.). Otherwise the system may kill `vmstorage` node before it finishes gradual closing of `vminsert` connections.

### Minimum downtime strategy

1. Gracefully stop all the `vminsert` and `vmselect` nodes in parallel.
1. Gracefully restart all the `vmstorage` nodes in parallel.
1. Start all the `vminsert` and `vmselect` nodes in parallel.

The cluster is unavailable for data ingestion and querying when performing the steps above.
The downtime is minimized by restarting cluster nodes in parallel at every step above.
The `minimum downtime` strategy has the following benefits comparing to `no downtime` strategy:

- It allows performing config update / version upgrade with minimum disruption
  when the previous config / version is incompatible with the new config / version.
- It allows performing config update / version upgrade with minimum disruption
  when the cluster has no enough compute resources (CPU, RAM, disk IO, network bandwidth)
  for rolling upgrade.
- It allows minimizing the duration of config update / version upgrade for clusters with big number of nodes
  of for clusters with big `vmstorage` nodes, which may take long time for graceful restart.

## Cluster availability

VictoriaMetrics cluster architecture prioritizes availability over data consistency.
This means that the cluster remains available for data ingestion and data querying
if some of its components are temporarily unavailable.

VictoriaMetrics cluster remains available if the following conditions are met:

- HTTP load balancer must stop routing requests to unavailable `vminsert` and `vmselect` nodes
  ([vmauth](https://docs.victoriametrics.com/vmauth.html) stops routing requests to unavailable nodes).

- At least a single `vminsert` node must remain available in the cluster for processing data ingestion workload.
  The remaining active `vminsert` nodes must have enough compute capacity (CPU, RAM, network bandwidth)
  for handling the current data ingestion workload.
  If the remaining active `vminsert` nodes have no enough resources for processing the data ingestion workload,
  then arbitrary delays may occur during data ingestion.
  See [capacity planning](#capacity-planning) and [cluster resizing](#cluster-resizing-and-scalability) docs for more details.

- At least a single `vmselect` node must remain available in the cluster for processing query workload.
  The remaining active `vmselect` nodes must have enough compute capacity (CPU, RAM, network bandwidth, disk IO)
  for handling the current query workload.
  If the remaining active `vmselect` nodes have no enough resources for processing query workload,
  then arbitrary failures and delays may occur during query processing.
  See [capacity planning](#capacity-planning) and [cluster resizing](#cluster-resizing-and-scalability) docs for more details.

- At least a single `vmstorage` node must remain available in the cluster for accepting newly ingested data
  and for processing incoming queries. The remaining active `vmstorage` nodes must have enough compute capacity
  (CPU, RAM, network bandwidth, disk IO, free disk space) for  handling the current workload.
  If the remaining active `vmstorage` nodes have no enough resources for processing query workload,
  then arbitrary failures and delay may occur during data ingestion and query processing.
  See [capacity planning](#capacity-planning) and [cluster resizing](#cluster-resizing-and-scalability) docs for more details.

The cluster works in the following way when some of `vmstorage` nodes are unavailable:

- `vminsert` re-routes newly ingested data from unavailable `vmstorage` nodes to remaining healthy `vmstorage` nodes.
  This guarantees that the newly ingested data is properly saved if the healthy `vmstorage` nodes have enough CPU, RAM, disk IO and network bandwidth
  for processing the increased data ingestion workload.
  `vminsert` spreads evenly the additional data among the healthy `vmstorage` nodes in order to spread evenly
  the increased load on these nodes. During re-routing, healthy `vmstorage` nodes will experience higher resource usage
  and increase in number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series).

- `vmselect` continues serving queries if at least a single `vmstorage` nodes is available.
  It marks responses as partial for queries served from the remaining healthy `vmstorage` nodes,
  since such responses may miss historical data stored on the temporarily unavailable `vmstorage` nodes.
  Every partial JSON response contains `"isPartial": true` option.
  If you prefer consistency over availability, then run `vmselect` nodes with `-search.denyPartialResponse` command-line flag.
  In this case `vmselect` returns an error if at least a single `vmstorage` node is unavailable.
  Another option is to pass `deny_partial_response=1` query arg to requests to `vmselect` nodes.

  `vmselect` also accepts `-replicationFactor=N` command-line flag. This flag instructs `vmselect` to return full response
  if less than `-replicationFactor` vmstorage nodes are unavailable during querying, since it assumes that the remaining
  `vmstorage` nodes contain the full data. See [these docs](#replication-and-data-safety) for details.

  It is also possible to configure independent replication factor per distinct `vmstorage` groups - see [these docs](#vmstorage-groups-at-vmselect).

`vmselect` doesn't serve partial responses for API handlers returning raw datapoints - [`/api/v1/export*` endpoints](https://docs.victoriametrics.com/#how-to-export-time-series), since users usually expect this data is always complete.

Data replication can be used for increasing storage durability. See [these docs](#replication-and-data-safety) for details.

## Capacity planning

VictoriaMetrics uses lower amounts of CPU, RAM and storage space on production workloads compared to competing solutions (Prometheus, Thanos, Cortex, TimescaleDB, InfluxDB, QuestDB, M3DB) according to [our case studies](https://docs.victoriametrics.com/CaseStudies.html).

Each node type - `vminsert`, `vmselect` and `vmstorage` - can run on the most suitable hardware. Cluster capacity scales linearly with the available resources. The needed amounts of CPU and RAM per each node type highly depends on the workload - the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series), [series churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate), query types, query qps, etc. It is recommended setting up a test VictoriaMetrics cluster for your production workload and iteratively scaling per-node resources and the number of nodes per node type until the cluster becomes stable. It is recommended setting up [monitoring for the cluster](#monitoring). It helps to determine bottlenecks in cluster setup. It is also recommended following [the troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting).

The needed storage space for the given retention (the retention is set via `-retentionPeriod` command-line flag at `vmstorage`) can be extrapolated from disk space usage in a test run. For example, if the storage space usage is 10GB after a day-long test run on a production workload, then it will need at least `10GB*100=1TB` of disk space for `-retentionPeriod=100d` (100-days retention period). Storage space usage can be monitored with [the official Grafana dashboard for VictoriaMetrics cluster](#monitoring).

It is recommended leaving the following amounts of spare resources:

- 50% of free RAM across all the node types for reducing the probability of OOM (out of memory) crashes and slowdowns during temporary spikes in workload.
- 50% of spare CPU across all the node types for reducing the probability of slowdowns during temporary spikes in workload.
- At least 20% of free storage space at the directory pointed by `-storageDataPath` command-line flag at `vmstorage` nodes. See also `-storage.minFreeDiskSpaceBytes` command-line flag [description for vmstorage](#list-of-command-line-flags-for-vmstorage).

Some capacity planning tips for VictoriaMetrics cluster:

- The [replication](#replication-and-data-safety) increases the amounts of needed resources for the cluster by up to `N` times where `N` is replication factor. This is because `vminsert` stores `N` copies of every ingested sample on distinct `vmstorage` nodes. These copies are de-duplicated by `vmselect` during querying. The most cost-efficient and performant solution for data durability is to rely on replicated durable persistent disks such as [Google Compute persistent disks](https://cloud.google.com/compute/docs/disks#pdspecs) instead of using the [replication at VictoriaMetrics level](#replication-and-data-safety).
- It is recommended to run a cluster with big number of small `vmstorage` nodes instead of a cluster with small number of big `vmstorage` nodes. This increases chances that the cluster remains available and stable when some of `vmstorage` nodes are temporarily unavailable during maintenance events such as upgrades, configuration changes or migrations. For example, when a cluster contains 10 `vmstorage` nodes and a single node becomes temporarily unavailable, then the workload on the remaining 9 nodes increases by `1/9=11%`. When a cluster contains 3 `vmstorage` nodes and a single node becomes temporarily unavailable, then the workload on the remaining 2 nodes increases by `1/2=50%`. The remaining `vmstorage` nodes may have no enough free capacity for handling the increased workload. In this case the cluster may become overloaded, which may result to decreased availability and stability.
- Cluster capacity for [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series) can be increased by increasing RAM and CPU resources per each `vmstorage` node or by adding new `vmstorage` nodes.
- Query latency can be reduced by increasing CPU resources per each `vmselect` node, since each incoming query is processed by a single `vmselect` node. Performance for heavy queries scales with the number of available CPU cores at `vmselect` node, since `vmselect` processes time series referred by the query on all the available CPU cores.
- If the cluster needs to process incoming queries at a high rate, then its capacity can be increased by adding more `vmselect` nodes, so incoming queries could be spread among bigger number of `vmselect` nodes.
- By default `vminsert` compresses the data it sends to `vmstorage` in order to reduce network bandwidth usage. The compression takes additional CPU resources at `vminsert`. If `vminsert` nodes have limited CPU, then the compression can be disabled by passing `-rpc.disableCompression` command-line flag at `vminsert` nodes.
- By default `vmstorage` compresses the data it sends to `vmselect` during queries in order to reduce network bandwidth usage. The compression takes additional CPU resources at `vmstorage`. If `vmstorage` nodes have limited CPU, then the compression can be disabled by passing `-rpc.disableCompression` command-line flag at `vmstorage` nodes.

See also [resource usage limits docs](#resource-usage-limits).

## Resource usage limits

By default, cluster components of VictoriaMetrics are tuned for an optimal resource usage under typical workloads.
Some workloads may need fine-grained resource usage limits. In these cases the following command-line flags may be useful:

- `-memory.allowedPercent` and `-memory.allowedBytes` limit the amounts of memory, which may be used for various internal caches
  at all the cluster components of VictoriaMetrics - `vminsert`, `vmselect` and `vmstorage`. Note that VictoriaMetrics components may use more memory,
  since these flags don't limit additional memory, which may be needed on a per-query basis.
- `-search.maxMemoryPerQuery` limits the amounts of memory, which can be used for processing a single query at `vmselect` node.
  Queries, which need more memory, are rejected. Heavy queries, which select big number of time series,
  may exceed the per-query memory limit by a small percent. The total memory limit for concurrently executed queries can be estimated
  as `-search.maxMemoryPerQuery` multiplied by `-search.maxConcurrentRequests`.
- `-search.maxUniqueTimeseries` at `vmselect` component limits the number of unique time series a single query can find and process.
  `vmselect` passes the limit to `vmstorage` component, which keeps in memory some metainformation about the time series located
  by each query and spends some CPU time for processing the found time series. This means that the maximum memory usage and CPU usage
  a single query can use at `vmstorage` is proportional to `-search.maxUniqueTimeseries`.
- `-search.maxQueryDuration` at `vmselect` limits the duration of a single query. If the query takes longer than the given duration, then it is canceled.
  This allows saving CPU and RAM at `vmselect` and `vmstorage` when executing unexpectedly heavy queries.
- `-search.maxConcurrentRequests` at `vmselect` and `vmstorage` limits the number of concurrent requests a single `vmselect` / `vmstorage` node can process.
  Bigger number of concurrent requests usually require bigger amounts of memory at both `vmselect` and `vmstorage`.
  For example, if a single query needs 100 MiB of additional memory during its execution, then 100 concurrent queries
  may need `100 * 100 MiB = 10 GiB` of additional memory. So it is better to limit the number of concurrent queries,
  while suspending additional incoming queries if the concurrency limit is reached.
  `vmselect` and `vmstorage` provides `-search.maxQueueDuration` command-line flag for limiting the maximum wait time for suspended queries.
  See also `-search.maxMemoryPerQuery` command-line flag at `vmselect`.
- `-search.maxQueueDuration` at `vmselect` and `vmstorage` limits the maximum duration queries may wait for execution when `-search.maxConcurrentRequests`
  concurrent queries are executed.
- `-search.maxSamplesPerSeries` at `vmselect` limits the number of raw samples the query can process per each time series.
  `vmselect` processes raw samples sequentially per each found time series during the query. It unpacks raw samples on the selected time range
  per each time series into memory and then applies the given [rollup function](https://docs.victoriametrics.com/MetricsQL.html#rollup-functions).
  The `-search.maxSamplesPerSeries` command-line flag allows limiting memory usage at `vmselect` in the case when the query is executed on a time range,
  which contains hundreds of millions of raw samples per each located time series.
- `-search.maxSamplesPerQuery` at `vmselect` limits the number of raw samples a single query can process. This allows limiting CPU usage at `vmselect` for heavy queries.
- `-search.maxResponseSeries` at `vmselect` limits the number of time series a single query can return
  from [`/api/v1/query`](https://docs.victoriametrics.com/keyConcepts.html#instant-query) and [`/api/v1/query_range`](https://docs.victoriametrics.com/keyConcepts.html#range-query).
- `-search.maxPointsPerTimeseries` limits the number of calculated points, which can be returned per each
  matching time series from [range query](https://docs.victoriametrics.com/keyConcepts.html#range-query).
- `-search.maxPointsSubqueryPerTimeseries` limits the number of calculated points, which can be generated
  per each matching time series during [subquery](https://docs.victoriametrics.com/MetricsQL.html#subqueries) evaluation.
- `-search.maxSeriesPerAggrFunc` limits the number of time series, which can be generated by [MetricsQL aggregate functions](https://docs.victoriametrics.com/MetricsQL.html#aggregate-functions) in a single query.
- `-search.maxSeries` at `vmselect` limits the number of time series, which may be returned from
  [/api/v1/series](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers).
  This endpoint is used mostly by Grafana for auto-completion of metric names, label names and label values.
  Queries to this endpoint may take big amounts of CPU time and memory at `vmstorage` and `vmselect` when the database contains
  big number of unique time series because of [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
  In this case it might be useful to set the `-search.maxSeries` to quite low value in order limit CPU and memory usage.
- `-search.maxTagKeys` at `vmstorage` limits the number of items, which may be returned from
  [/api/v1/labels](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names). This endpoint is used mostly by Grafana
  for auto-completion of label names. Queries to this endpoint may take big amounts of CPU time and memory at `vmstorage` and `vmselect`
  when the database contains big number of unique time series because of [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
  In this case it might be useful to set the `-search.maxTagKeys` to quite low value in order to limit CPU and memory usage.
- `-search.maxTagValues` at `vmstorage` limits the number of items, which may be returned from
  [/api/v1/label/.../values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values). This endpoint is used mostly by Grafana
  for auto-completion of label values. Queries to this endpoint may take big amounts of CPU time and memory at `vmstorage` and `vmselect`
  when the database contains big number of unique time series because of [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
  In this case it might be useful to set the `-search.maxTagValues` to quite low value in order to limit CPU and memory usage.
- `-storage.maxDailySeries` at `vmstorage` can be used for limiting the number of time series seen per day aka
  [time series churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate). See [cardinality limiter docs](#cardinality-limiter).
- `-storage.maxHourlySeries` at `vmstorage` can be used for limiting the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series).
  See [cardinality limiter docs](#cardinality-limiter).

See also [capacity planning docs](#capacity-planning) and [cardinality limiter in vmagent](https://docs.victoriametrics.com/vmagent.html#cardinality-limiter).

## High availability

The database is considered highly available if it continues accepting new data and processing incoming queries when some of its components are temporarily unavailable.
VictoriaMetrics cluster is highly available according to this definition - see [cluster availability docs](#cluster-availability).

It is recommended to run all the components for a single cluster in the same subnetwork with high bandwidth, low latency and low error rates.
This improves cluster performance and availability. It isn't recommended spreading components for a single cluster
across multiple availability zones, since cross-AZ network usually has lower bandwidth, higher latency and higher
error rates comparing the network inside a single AZ.

If you need multi-AZ setup, then it is recommended running independent clusters in each AZ and setting up
[vmagent](https://docs.victoriametrics.com/vmagent.html) in front of these clusters, so it could replicate incoming data
into all the cluster - see [these docs](https://docs.victoriametrics.com/vmagent.html#multitenancy) for details.
Then an additional `vmselect` nodes can be configured for reading the data from multiple clusters according to [these docs](#multi-level-cluster-setup).


## Multi-level cluster setup

`vmselect` nodes can be queried by other `vmselect` nodes if they run with `-clusternativeListenAddr` command-line flag.
For example, if `vmselect` is started with `-clusternativeListenAddr=:8401`, then it can accept queries from another `vmselect` nodes at TCP port 8401
in the same way as `vmstorage` nodes do. This allows chaining `vmselect` nodes and building multi-level cluster topologies.
For example, the top-level `vmselect` node can query second-level `vmselect` nodes in different availability zones (AZ),
while the second-level `vmselect` nodes can query `vmstorage` nodes in local AZ. See also [vmstorage groups at vmselect](#vmstorage-groups-at-vmselect).

`vminsert` nodes can accept data from another `vminsert` nodes if they run with `-clusternativeListenAddr` command-line flag.
For example, if `vminsert` is started with `-clusternativeListenAddr=:8400`, then it can accept data from another `vminsert` nodes at TCP port 8400
in the same way as `vmstorage` nodes do. This allows chaining `vminsert` nodes and building multi-level cluster topologies.
For example, the top-level `vminsert` node can replicate data among the second level of `vminsert` nodes located in distinct availability zones (AZ),
while the second-level `vminsert` nodes can spread the data among `vmstorage` nodes in local AZ.

The multi-level cluster setup for `vminsert` nodes has the following shortcomings because of synchronous replication and data sharding:

* Data ingestion speed is limited by the slowest link to AZ.
* `vminsert` nodes at top level re-route incoming data to the remaining AZs when some AZs are temporarily unavailable. This results in data gaps at AZs which were temporarily unavailable.

These issues are addressed by [vmagent](https://docs.victoriametrics.com/vmagent.html) when it runs in [multitenancy mode](https://docs.victoriametrics.com/vmagent.html#multitenancy).
`vmagent` buffers data, which must be sent to a particular AZ, when this AZ is temporarily unavailable. The buffer is stored on disk. The buffered data is sent to AZ as soon as it becomes available.

## vmstorage groups at vmselect

`vmselect` can be configured to query multiple distinct groups of `vmstorage` nodes with individual `-replicationFactor` per each group.
The following format for `-storageNode` command-line flag value should be used for assigning a particular `addr` of `vmstorage` to a particular `groupName` -
`-storageNode=groupName/addr`. For example, the following command runs `vmselect`, which continues returning full responses if up to one node per each group is temporarily unavailable
because the given `-replicationFactor=2` is applied individually per each group:

```
/path/to/vmselect \
	-replicationFactor=2 \
	-storageNode=group1/host1 \
	-storageNode=group1/host2 \
	-storageNode=group1/host3 \
	-storageNode=group2/host4 \
	-storageNode=group2/host5 \
	-storageNode=group2/host6 \
	-storageNode=group3/host7 \
	-storageNode=group3/host8 \
	-storageNode=group3/host9
```

It is possible to specify distinct `-replicationFactor` per each group via the following format - `-replicationFactor=groupName:rf`.
For example, the following command runs `vmselect`, which uses `-replicationFactor=3` for the `group1`, while it uses `-replicationFactor=1` for the `group2`:

```
/path/to/vmselect \
	-replicationFactor=group1:3 \
	-storageNode=group1/host1 \
	-storageNode=group1/host2 \
	-storageNode=group1/host3 \
	-replicationFactor=group2:1 \
	-storageNode=group2/host4 \
	-storageNode=group2/host5 \
	-storageNode=group2/host6
```

## Helm

Helm chart simplifies managing cluster version of VictoriaMetrics in Kubernetes.
It is available in the [helm-charts](https://github.com/VictoriaMetrics/helm-charts) repository.

## Kubernetes operator

[K8s operator](https://github.com/VictoriaMetrics/operator) simplifies managing VictoriaMetrics components in Kubernetes.

## Replication and data safety

By default, VictoriaMetrics offloads replication to the underlying storage pointed by `-storageDataPath` such as [Google compute persistent disk](https://cloud.google.com/compute/docs/disks#pdspecs),
which guarantees data durability. VictoriaMetrics supports application-level replication if replicated durable persistent disks cannot be used for some reason.

The replication can be enabled by passing `-replicationFactor=N` command-line flag to `vminsert`. This instructs `vminsert` to store `N` copies for every ingested sample
on `N` distinct `vmstorage` nodes. This guarantees that all the stored data remains available for querying if up to `N-1` `vmstorage` nodes are unavailable.

Passing `-replicationFactor=N` command-line flag to `vmselect` instructs it to not mark responses as `partial` if less than `-replicationFactor` vmstorage nodes are unavailable during the query.
See [cluster availability docs](#cluster-availability) for details.

The cluster must contain at least `2*N-1` `vmstorage` nodes, where `N` is replication factor, in order to maintain the given replication factor
for newly ingested data when `N-1` of storage nodes are unavailable.

VictoriaMetrics stores timestamps with millisecond precision, so `-dedup.minScrapeInterval=1ms` command-line flag must be passed to `vmselect` nodes when the replication is enabled,
so they could de-duplicate replicated samples obtained from distinct `vmstorage` nodes during querying. If duplicate data is pushed to VictoriaMetrics
from identically configured [vmagent](https://docs.victoriametrics.com/vmagent.html) instances or Prometheus instances, then the `-dedup.minScrapeInterval` must be set
to `scrape_interval` from scrape configs according to [deduplication docs](#deduplication).

Note that [replication doesn't save from disaster](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883),
so it is recommended performing regular backups. See [these docs](#backups) for details.

Note that the replication increases resource usage - CPU, RAM, disk space, network bandwidth - by up to `-replicationFactor=N` times, because `vminsert` stores `N` copies
of incoming data to distinct `vmstorage` nodes and `vmselect` needs to de-duplicate the replicated data obtained from `vmstorage` nodes during querying.
So it is more cost-effective to offload the replication to underlying replicated durable storage pointed by `-storageDataPath`
such as [Google Compute Engine persistent disk](https://cloud.google.com/compute/docs/disks/#pdspecs), which is protected from data loss and data corruption.
It also provides consistently high performance and [may be resized](https://cloud.google.com/compute/docs/disks/add-persistent-disk) without downtime.
HDD-based persistent disks should be enough for the majority of use cases. It is recommended using durable replicated persistent volumes in Kubernetes.

## Deduplication

Cluster version of VictoriaMetrics supports data deduplication in the same way as single-node version do. See [these docs](https://docs.victoriametrics.com/#deduplication) for details. The only difference is that the same `-dedup.minScrapeInterval` command-line flag value must be passed to both `vmselect` and `vmstorage` nodes because of the following aspects:

By default, `vminsert` tries to route all the samples for a single time series to a single `vmstorage` node. But samples for a single time series can be spread among multiple `vmstorage` nodes under certain conditions:
* when adding/removing `vmstorage` nodes. Then new samples for a part of time series will be routed to another `vmstorage` nodes;
* when `vmstorage` nodes are temporarily unavailable (for instance, during their restart). Then new samples are re-routed to the remaining available `vmstorage` nodes;
* when `vmstorage` node has no enough capacity for processing incoming data stream. Then `vminsert` re-routes new samples to other `vmstorage` nodes.


## Backups

It is recommended performing periodical backups from [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for protecting from user errors such as accidental data deletion.

The following steps must be performed for each `vmstorage` node for creating a backup:

1. Create an instant snapshot by navigating to `/snapshot/create` HTTP handler. It will create snapshot and return its name.
1. Archive the created snapshot from `<-storageDataPath>/snapshots/<snapshot_name>` folder using [vmbackup](https://docs.victoriametrics.com/vmbackup.html).
   The archival process doesn't interfere with `vmstorage` work, so it may be performed at any suitable time.
1. Delete unused snapshots via `/snapshot/delete?snapshot=<snapshot_name>` or `/snapshot/delete_all` in order to free up occupied storage space.

There is no need in synchronizing backups among all the `vmstorage` nodes.

Restoring from backup:

1. Stop `vmstorage` node with `kill -INT`.
1. Restore data from backup using [vmrestore](https://docs.victoriametrics.com/vmrestore.html) into `-storageDataPath` directory.
1. Start `vmstorage` node.

## Retention filters

[VictoriaMetrics enterprise](https://docs.victoriametrics.com/enterprise.html) supports configuring multiple retentions for distinct sets of time series
by passing `-retentionFilter` command-line flag to `vmstorage` nodes. See [these docs](https://docs.victoriametrics.com/#retention-filters) for details on this feature.

Additionally, enterprise version of VictoriaMetrics cluster supports multiple retentions for distinct sets of [tenants](#multitenancy)
by specifying filters on `vm_account_id` and/or `vm_project_id` pseudo-labels in `-retentionFilter` command-line flag.
If the tenant doesn't match specified `-retentionFilter` options, then the global `-retentionPeriod` is used for it.

For example, the following config sets retention to 1 day for [tenants](#multitenancy) with `accountID` starting from `42`,
then sets retention to 3 days for time series with label `env="dev"` or `env="prod"` from any tenant,
while the rest of tenants will have 4 weeks retention:

```
-retentionFilter='{vm_account_id=~"42.*"}:1d' -retentionFilter='{env=~"dev|staging"}:3d' -retentionPeriod=4w
```

It is OK to mix filters on real labels with filters on `vm_account_id` and `vm_project_id` pseudo-labels.
For example, the following config sets retention to 5 days for time series with `env="dev"` label from [tenant](#multitenancy) `accountID=5`:

```
-retentionFilter='{vm_account_id="5",env="dev"}:5d'
```

See also [these docs](https://docs.victoriametrics.com/#retention-filters) for additional details on retention filters.

Enterprise binaries can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).

## Downsampling

Downsampling is available in [enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise.html).
It is configured with `-downsampling.period` command-line flag according to [these docs](https://docs.victoriametrics.com/#downsampling).

The same flag value must be passed to both `vmstorage` and `vmselect` nodes. Configuring `vmselect` node with `-downsampling.period`
command-line flag makes query results more consistent, because `vmselect` uses the maximum configured downsampling interval
on the requested time range if this time range covers multiple downsampling levels.
For example, if `-downsampling.period=30d:5m` and the query requests the last 60 days of data, then `vmselect`
downsamples all the [raw samples](https://docs.victoriametrics.com/keyConcepts.html#raw-samples) on the requested time range
using 5 minute interval. If `-downsampling.period` command-line flag isn't set at `vmselect`,
then query results can be less consistent because of mixing raw and downsampled data.

Enterprise binaries can be downloaded and evaluated for free from [the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).

## Profiling

All the cluster components provide the following handlers for [profiling](https://blog.golang.org/profiling-go-programs):

- `http://vminsert:8480/debug/pprof/heap` for memory profile and `http://vminsert:8480/debug/pprof/profile` for CPU profile
- `http://vmselect:8481/debug/pprof/heap` for memory profile and `http://vmselect:8481/debug/pprof/profile` for CPU profile
- `http://vmstorage:8482/debug/pprof/heap` for memory profile and `http://vmstorage:8482/debug/pprof/profile` for CPU profile

Example command for collecting cpu profile from `vmstorage` (replace `0.0.0.0` with `vmstorage` hostname if needed):

<div class="with-copy" markdown="1">

```console
curl http://0.0.0.0:8482/debug/pprof/profile > cpu.pprof
```

</div>

Example command for collecting memory profile from `vminsert` (replace `0.0.0.0` with `vminsert` hostname if needed):

<div class="with-copy" markdown="1">

```console
curl http://0.0.0.0:8480/debug/pprof/heap > mem.pprof
```

It is safe sharing the collected profiles from security point of view, since they do not contain sensitive information.

</div>

## vmalert

vmselect is capable of proxying requests to [vmalert](https://docs.victoriametrics.com/vmalert.html)
when `-vmalert.proxyURL` flag is set. Use this feature for the following cases:
* for proxying requests from [Grafana Alerting UI](https://grafana.com/docs/grafana/latest/alerting/);
* for accessing vmalerts UI through vmselects Web interface.

For accessing vmalerts UI through vmselect configure `-vmalert.proxyURL` flag and visit
`http://<vmselect>:8481/select/<accountID>/prometheus/vmalert/` link.


## Community and contributions

Feel free asking any questions regarding VictoriaMetrics:

* [Slack](https://slack.victoriametrics.com/)
* [Twitter](https://twitter.com/VictoriaMetrics/)
* [Linkedin](https://www.linkedin.com/company/victoriametrics/)
* [Reddit](https://www.reddit.com/r/VictoriaMetrics/)
* [Telegram-en](https://t.me/VictoriaMetrics_en)
* [Telegram-ru](https://t.me/VictoriaMetrics_ru1)
* [Google groups](https://groups.google.com/forum/#!forum/victorametrics-users)
* [Mastodon](https://mastodon.social/@victoriametrics/)

We are open to third-party pull requests provided they follow the [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle):

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency or performance.

Adhering to the `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and verified by many people.

Due to `KISS`, cluster version of VictoriaMetrics has no the following "features" popular in distributed computing world:

- Fragile gossip protocols. See [failed attempt in Thanos](https://github.com/improbable-eng/thanos/blob/030bc345c12c446962225221795f4973848caab5/docs/proposals/completed/201809_gossip-removal.md).
- Hard-to-understand-and-implement-properly [Paxos protocols](https://www.quora.com/In-distributed-systems-what-is-a-simple-explanation-of-the-Paxos-algorithm).
- Complex replication schemes, which may go nuts in unforeseen edge cases. See [replication docs](#replication-and-data-safety) for details.
- Automatic data reshuffling between storage nodes, which may hurt cluster performance and availability.
- Automatic cluster resizing, which may cost you a lot of money if improperly configured.
- Automatic discovering and addition of new nodes in the cluster, which may mix data between dev and prod clusters :)
- Automatic leader election, which may result in split brain disaster on network errors.

## Reporting bugs

Report bugs and propose new features [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).

## List of command-line flags

- [List of command-line flags for vminsert](#list-of-command-line-flags-for-vminsert)
- [List of command-line flags for vmselect](#list-of-command-line-flags-for-vmselect)
- [List of command-line flags for vmstorage](#list-of-command-line-flags-for-vmstorage)

### List of command-line flags for vminsert

Below is the output for `/path/to/vminsert -help`:

```
  -blockcache.missesBeforeCaching int
     The number of cache misses before putting the block into cache. Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage (default 2)
  -cacheExpireDuration duration
     Items are removed from in-memory caches after they aren't accessed for this duration. Lower values may reduce memory usage at the cost of higher CPU usage. See also -prevCacheRemovalPercent (default 30m0s)
  -cluster.tls
     Whether to use TLS for connections to -storageNode. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCAFile string
     Path to TLS CA file to use for verifying certificates provided by -storageNode if -cluster.tls flag is set. By default system CA is used. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCertFile string
     Path to client-side TLS certificate file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsInsecureSkipVerify
     Whether to skip verification of TLS certificates provided by -storageNode nodes if -cluster.tls flag is set. Note that disabled TLS certificate verification breaks security. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsKeyFile string
     Path to client-side TLS key file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.vminsertConnsShutdownDuration duration
     The time needed for gradual closing of upstream vminsert connections during graceful shutdown. Bigger duration reduces spikes in CPU, RAM and disk IO load on the remaining lower-level clusters during rolling restart. Smaller duration reduces the time needed to close all the upstream vminsert connections, thus reducing the time for graceful shutdown. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#improving-re-routing-performance-during-restart (default 25s)
  -clusternativeListenAddr string
     TCP address to listen for data from other vminsert nodes in multi-level cluster setup. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multi-level-cluster-setup . Usually :8400 should be set to match default vmstorage port for vminsert. Disabled work if empty
  -csvTrimTimestamp duration
     Trim timestamps when importing csv data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -datadog.maxInsertRequestSize size
     The maximum size in bytes of a single DataDog POST request to /datadog/api/v1/series
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -datadog.sanitizeMetricName
     Sanitize metric names for the ingested DataDog data to comply with DataDog behaviour described at https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics (default true)
  -denyQueryTracing
     Whether to disable the ability to trace queries. See https://docs.victoriametrics.com/#query-tracing
  -disableRerouting
     Whether to disable re-routing when some of vmstorage nodes accept incoming data at slower speed compared to other storage nodes. Disabled re-routing limits the ingestion rate by the slowest vmstorage node. On the other side, disabled re-routing minimizes the number of active time series in the cluster during rolling restarts and during spikes in series churn rate. See also -dropSamplesOnOverload (default true)
  -dropSamplesOnOverload
     Whether to drop incoming samples if the destination vmstorage node is overloaded and/or unavailable. This prioritizes cluster availability over consistency, e.g. the cluster continues accepting all the ingested samples, but some of them may be dropped if vmstorage nodes are temporarily unavailable and/or overloaded. The drop of samples happens before the replication, so it's not recommended to use this flag with -replicationFactor enabled.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey string
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -graphiteListenAddr string
     TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty. See also -graphiteListenAddr.useProxyProtocol
  -graphiteListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -graphiteListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
  -graphiteTrimTimestamp duration
     Trim timestamps for Graphite data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
  -http.connTimeout duration
     Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
     Value for 'Content-Security-Policy' header
  -http.header.frameOptions string
     Value for 'X-Frame-Options' header
  -http.header.hsts string
     Value for 'Strict-Transport-Security' header
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
     Address to listen for http connections. See also -httpListenAddr.useProxyProtocol (default ":8480")
  -httpListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
  -import.maxLineLen size
     The maximum length in bytes of a single line accepted by /api/v1/import; the line length can be limited with 'max_rows_per_line' query arg passed to /api/v1/export
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 10485760)
  -influx.databaseNames array
     Comma-separated list of database names to return from /query and /influx/query API. This can be needed for accepting data from Telegraf plugins such as https://github.com/fangli/fluent-plugin-influxdb
     Supports an array of values separated by comma or specified via multiple flags.
  -influx.maxLineSize size
     The maximum size in bytes for a single InfluxDB line during parsing
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 262144)
  -influxDBLabel string
     Default label for the DB name sent over '?db={db_name}' query parameter (default "db")
  -influxListenAddr string
     TCP and UDP address to listen for InfluxDB line protocol data. Usually :8089 must be set. Doesn't work if empty. This flag isn't needed when ingesting data over HTTP - just send it to http://<victoriametrics>:8428/write . See also -influxListenAddr.useProxyProtocol
  -influxListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -influxListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
  -influxMeasurementFieldSeparator string
     Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol (default "_")
  -influxSkipMeasurement
     Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'
  -influxSkipSingleField
     Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metric name if InfluxDB line contains only a single field
  -influxTrimTimestamp duration
     Trim timestamps for InfluxDB line protocol data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -insert.maxQueueDuration duration
     The maximum duration to wait in the queue when -maxConcurrentInserts concurrent insert requests are executed (default 1m0s)
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
  -maxConcurrentInserts int
     The maximum number of concurrent insert requests. Default value should work for most cases, since it minimizes the memory usage. The default value can be increased when clients send data over slow networks. See also -insert.maxQueueDuration.
  -maxInsertRequestSize size
     The maximum size in bytes of a single Prometheus remote_write API request
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 33554432)
  -maxLabelValueLen int
     The maximum length of label values in the accepted time series. Longer label values are truncated. In this case the vm_too_long_label_values_total metric at /metrics page is incremented (default 16384)
  -maxLabelsPerTimeseries int
     The maximum number of labels accepted per time series. Superfluous labels are dropped. In this case the vm_metrics_with_dropped_labels_total metric at /metrics page is incremented (default 30)
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey string
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -newrelic.maxInsertRequestSize size
     The maximum size in bytes of a single NewRelic request to /newrelic/infra/v2/metrics/events/bulk
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 67108864)
  -opentsdbHTTPListenAddr string
     TCP address to listen for OpenTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty. See also -opentsdbHTTPListenAddr.useProxyProtocol
  -opentsdbHTTPListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -opentsdbHTTPListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
  -opentsdbListenAddr string
     TCP and UDP address to listen for OpenTSDB metrics. Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. Usually :4242 must be set. Doesn't work if empty. See also -opentsdbListenAddr.useProxyProtocol
  -opentsdbListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -opentsdbListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
  -opentsdbTrimTimestamp duration
     Trim timestamps for OpenTSDB 'telnet put' data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
  -opentsdbhttp.maxInsertRequestSize size
     The maximum size of OpenTSDB HTTP put request
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 33554432)
  -opentsdbhttpTrimTimestamp duration
     Trim timestamps for OpenTSDB HTTP data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -pprofAuthKey string
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -prevCacheRemovalPercent float
     Items in the previous caches are removed when the percent of requests it serves becomes lower than this value. Higher values reduce memory usage at the cost of higher CPU usage. See also -cacheExpireDuration (default 0.1)
  -pushmetrics.disableCompression
     Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.header array
     Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.interval duration
     Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
  -relabelConfig string
     Optional path to a file with relabeling rules, which are applied to all the ingested metrics. The path can point either to local file or to http url. See https://docs.victoriametrics.com/#relabeling for details. The config is reloaded on SIGHUP signal
  -replicationFactor int
     Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. Higher values for -dedup.minScrapeInterval at vmselect is OK (default 1)
  -rpc.disableCompression
     Whether to disable compression for the data sent from vminsert to vmstorage. This reduces CPU usage at the cost of higher network bandwidth usage
  -sortLabels
     Whether to sort labels for incoming samples before writing them to storage. This may be needed for reducing memory usage at storage when the order of labels in incoming samples is random. For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}. Enabled sorting for labels can slow down ingestion performance a bit
  -storageNode array
     Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN . Enterprise version of VictoriaMetrics supports automatic discovery of vmstorage addresses via dns+srv records. For example, -storageNode=dns+srv:vmstorage.addrs . See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery
     Supports an array of values separated by comma or specified via multiple flags.
  -storageNode.discoveryInterval duration
     Interval for refreshing -storageNode list behind dns+srv records. The minimum supported interval is 1s. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html (default 2s)
  -storageNode.filter string
     An optional regexp filter for discovered -storageNode addresses according to https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery. Discovered addresses matching the filter are retained, while other addresses are ignored. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
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
  -usePromCompatibleNaming
     Whether to replace characters unsupported by Prometheus with underscores in the ingested metric names and label names. For example, foo.bar{a.b='c'} is transformed into foo_bar{a_b='c'} during data ingestion if this flag is set. See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
  -version
     Show VictoriaMetrics version
  -vmstorageDialTimeout duration
     Timeout for establishing RPC connections from vminsert to vmstorage. See also -vmstorageUserTimeout (default 3s)
  -vmstorageUserTimeout duration
     Network timeout for RPC connections from vminsert to vmstorage (Linux only). Lower values speed up re-rerouting recovery when some of vmstorage nodes become unavailable because of networking issues. Read more about TCP_USER_TIMEOUT at https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/ . See also -vmstorageDialTimeout (default 3s)
```

### List of command-line flags for vmselect

Below is the output for `/path/to/vmselect -help`:

```
  -blockcache.missesBeforeCaching int
     The number of cache misses before putting the block into cache. Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage (default 2)
  -cacheDataPath string
     Path to directory for cache files. Cache isn't saved if empty
  -cacheExpireDuration duration
     Items are removed from in-memory caches after they aren't accessed for this duration. Lower values may reduce memory usage at the cost of higher CPU usage. See also -prevCacheRemovalPercent (default 30m0s)
  -cluster.tls
     Whether to use TLS for connections to -storageNode. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCAFile string
     Path to TLS CA file to use for verifying certificates provided by -storageNode if -cluster.tls flag is set. By default system CA is used. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCertFile string
     Path to client-side TLS certificate file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsInsecureSkipVerify
     Whether to skip verification of TLS certificates provided by -storageNode nodes if -cluster.tls flag is set. Note that disabled TLS certificate verification breaks security. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsKeyFile string
     Path to client-side TLS key file to use when connecting to -storageNode if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.disableCompression
     Whether to disable compression of the data sent to vmselect via -clusternativeListenAddr. This reduces CPU usage at the cost of higher network bandwidth usage
  -clusternative.maxConcurrentRequests int
     The maximum number of concurrent vmselect requests the server can process at -clusternativeListenAddr. It shouldn't be high, since a single request usually saturates a CPU core at the underlying vmstorage nodes, and many concurrently executed requests may require high amounts of memory. See also -clusternative.maxQueueDuration (default 32)
  -clusternative.maxQueueDuration duration
     The maximum time the incoming query to -clusternativeListenAddr waits for execution when -clusternative.maxConcurrentRequests limit is reached (default 10s)
  -clusternative.maxTagKeys int
     The maximum number of tag keys returned per search at -clusternativeListenAddr (default 100000)
  -clusternative.maxTagValueSuffixesPerSearch int
     The maximum number of tag value suffixes returned from /metrics/find at -clusternativeListenAddr (default 100000)
  -clusternative.maxTagValues int
     The maximum number of tag values returned per search at -clusternativeListenAddr (default 100000)
  -clusternative.tls
     Whether to use TLS when accepting connections at -clusternativeListenAddr. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.tlsCAFile string
     Path to TLS CA file to use for verifying certificates provided by vmselect, which connects at -clusternativeListenAddr if -clusternative.tls flag is set. By default system CA is used. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.tlsCertFile string
     Path to server-side TLS certificate file to use when accepting connections at -clusternativeListenAddr if -clusternative.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.tlsCipherSuites array
     Optional list of TLS cipher suites used for connections at -clusternativeListenAddr if -clusternative.tls flag is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
  -clusternative.tlsInsecureSkipVerify
     Whether to skip verification of TLS certificates provided by vmselect, which connects to -clusternativeListenAddr if -clusternative.tls flag is set. Note that disabled TLS certificate verification breaks security. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternative.tlsKeyFile string
     Path to server-side TLS key file to use when accepting connections at -clusternativeListenAddr if -clusternative.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -clusternativeListenAddr string
     TCP address to listen for requests from other vmselect nodes in multi-level cluster setup. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multi-level-cluster-setup . Usually :8401 should be set to match default vmstorage port for vmselect. Disabled work if empty
  -dedup.minScrapeInterval duration
     Leave only the last sample in every time series per each discrete interval equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details
  -denyQueryTracing
     Whether to disable the ability to trace queries. See https://docs.victoriametrics.com/#query-tracing
  -downsampling.period array
     Comma-separated downsampling periods in the format 'offset:period'. For example, '30d:10m' instructs to leave a single sample per 10 minutes for samples older than 30 days. When setting multiple downsampling periods, it is necessary for the periods to be multiples of each other. See https://docs.victoriametrics.com/#downsampling for details. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey string
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
     Value for 'Content-Security-Policy' header
  -http.header.frameOptions string
     Value for 'X-Frame-Options' header
  -http.header.hsts string
     Value for 'Strict-Transport-Security' header
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
     Address to listen for http connections. See also -httpListenAddr.useProxyProtocol (default ":8481")
  -httpListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
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
  -metricsAuthKey string
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pprofAuthKey string
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -prevCacheRemovalPercent float
     Items in the previous caches are removed when the percent of requests it serves becomes lower than this value. Higher values reduce memory usage at the cost of higher CPU usage. See also -cacheExpireDuration (default 0.1)
  -pushmetrics.disableCompression
     Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.header array
     Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.interval duration
     Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
  -replicationFactor int
     How many copies of every time series is available on the provided -storageNode nodes. vmselect continues returning full responses when up to replicationFactor-1 vmstorage nodes are temporarily unavailable during querying. See also -search.skipSlowReplicas (default 1)
  -search.cacheTimestampOffset duration
     The maximum duration since the current time for response data, which is always queried from the original raw data, without using the response cache. Increase this value if you see gaps in responses due to time synchronization issues between VictoriaMetrics and data sources (default 5m0s)
  -search.denyPartialResponse
     Whether to deny partial responses if a part of -storageNode instances fail to perform queries; this trades availability over consistency; see also -search.maxQueryDuration
  -search.disableCache
     Whether to disable response caching. This may be useful during data backfilling
  -search.graphiteMaxPointsPerSeries int
     The maximum number of points per series Graphite render API can return (default 1000000)
  -search.graphiteStorageStep duration
     The interval between datapoints stored in the database. It is used at Graphite Render API handler for normalizing the interval between datapoints in case it isn't normalized. It can be overridden by sending 'storage_step' query arg to /render API or by sending the desired interval via 'Storage-Step' http header during querying /render API (default 10s)
  -search.latencyOffset duration
     The time when data points become visible in query results after the collection. It can be overridden on per-query basis via latency_offset arg. Too small value can result in incomplete last points for query results (default 30s)
  -search.logQueryMemoryUsage size
     Log query and increment vm_memory_intensive_queries_total metric each time the query requires more memory than specified by this flag. This may help detecting and optimizing heavy queries. Query logging is disabled by default. See also -search.logSlowQueryDuration and -search.maxMemoryPerQuery
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -search.logSlowQueryDuration duration
     Log queries with execution time exceeding this value. Zero disables slow query logging. See also -search.logQueryMemoryUsage (default 5s)
  -search.maxConcurrentRequests int
     The maximum number of concurrent search requests. It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. See also -search.maxQueueDuration and -search.maxMemoryPerQuery (default 16)
  -search.maxExportDuration duration
     The maximum duration for /api/v1/export call (default 720h0m0s)
  -search.maxExportSeries int
     The maximum number of time series, which can be returned from /api/v1/export* APIs. This option allows limiting memory usage (default 10000000)
  -search.maxFederateSeries int
     The maximum number of time series, which can be returned from /federate. This option allows limiting memory usage (default 1000000)
  -search.maxGraphiteSeries int
     The maximum number of time series, which can be scanned during queries to Graphite Render API. See https://docs.victoriametrics.com/#graphite-render-api-usage (default 300000)
  -search.maxGraphiteTagKeys int
     The maximum number of tag keys returned from Graphite API, which returns tags. See https://docs.victoriametrics.com/#graphite-tags-api-usage (default 100000)
  -search.maxGraphiteTagValues int
     The maximum number of tag values returned from Graphite API, which returns tag values. See https://docs.victoriametrics.com/#graphite-tags-api-usage (default 100000)
  -search.maxLookback duration
     Synonym to -search.lookback-delta from Prometheus. The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. See also '-search.maxStalenessInterval' flag, which has the same meaning due to historical reasons
  -search.maxMemoryPerQuery size
     The maximum amounts of memory a single query may consume. Queries requiring more memory are rejected. The total memory limit for concurrently executed queries can be estimated as -search.maxMemoryPerQuery multiplied by -search.maxConcurrentRequests . See also -search.logQueryMemoryUsage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -search.maxPointsPerTimeseries int
     The maximum points per a single timeseries returned from /api/v1/query_range. This option doesn't limit the number of scanned raw samples in the database. The main purpose of this option is to limit the number of per-series points returned to graphing UI such as VMUI or Grafana. There is no sense in setting this limit to values bigger than the horizontal resolution of the graph. See also -search.maxResponseSeries (default 30000)
  -search.maxPointsSubqueryPerTimeseries int
     The maximum number of points per series, which can be generated by subquery. See https://valyala.medium.com/prometheus-subqueries-in-victoriametrics-9b1492b720b3 (default 100000)
  -search.maxQueryDuration duration
     The maximum duration for query execution (default 30s)
  -search.maxQueryLen size
     The maximum search query length in bytes
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 16384)
  -search.maxQueueDuration duration
     The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached; see also -search.maxQueryDuration (default 10s)
  -search.maxResponseSeries int
     The maximum number of time series which can be returned from /api/v1/query and /api/v1/query_range . The limit is disabled if it equals to 0. See also -search.maxPointsPerTimeseries and -search.maxUniqueTimeseries
  -search.maxSamplesPerQuery int
     The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries (default 1000000000)
  -search.maxSamplesPerSeries int
     The maximum number of raw samples a single query can scan per each time series. See also -search.maxSamplesPerQuery (default 30000000)
  -search.maxSeries int
     The maximum number of time series, which can be returned from /api/v1/series. This option allows limiting memory usage (default 30000)
  -search.maxSeriesPerAggrFunc int
     The maximum number of time series an aggregate MetricsQL function can generate (default 1000000)
  -search.maxStalenessInterval duration
     The maximum interval for staleness calculations. By default, it is automatically calculated from the median interval between samples. This flag could be useful for tuning Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. See also '-search.setLookbackToStep' flag
  -search.maxStatusRequestDuration duration
     The maximum duration for /api/v1/status/* requests (default 5m0s)
  -search.maxStepForPointsAdjustment duration
     The maximum step when /api/v1/query_range handler adjusts points with timestamps closer than -search.latencyOffset to the current time. The adjustment is needed because such points may contain incomplete data (default 1m0s)
  -search.maxTSDBStatusSeries int
     The maximum number of time series, which can be processed during the call to /api/v1/status/tsdb. This option allows limiting memory usage (default 10000000)
  -search.maxTagValueSuffixesPerSearch int
     The maximum number of tag value suffixes returned from /metrics/find (default 100000)
  -search.maxUniqueTimeseries int
     The maximum number of unique time series, which can be selected during /api/v1/query and /api/v1/query_range queries. This option allows limiting memory usage (default 300000)
  -search.maxWorkersPerQuery int
     The maximum number of CPU cores a single query can use. The default value should work good for most cases. The flag can be set to lower values for improving performance of big number of concurrently executed queries. The flag can be set to bigger values for improving performance of heavy queries, which scan big number of time series (>10K) and/or big number of samples (>100M). There is no sense in setting this flag to values bigger than the number of CPU cores available on the system (default 16)
  -search.minStalenessInterval duration
     The minimum interval for staleness calculations. This flag could be useful for removing gaps on graphs generated from time series with irregular intervals between samples. See also '-search.maxStalenessInterval'
  -search.minWindowForInstantRollupOptimization value
     Enable cache-based optimization for repeated queries to /api/v1/query (aka instant queries), which contain rollup functions with lookbehind window exceeding the given value
     The following optional suffixes are supported: s (second), m (minute), h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 3h)
  -search.noStaleMarkers
     Set this flag to true if the database doesn't contain Prometheus stale markers, so there is no need in spending additional CPU time on its handling. Staleness markers may exist only in data obtained from Prometheus scrape targets
  -search.queryStats.lastQueriesCount int
     Query stats for /api/v1/status/top_queries is tracked on this number of last queries. Zero value disables query stats tracking (default 20000)
  -search.queryStats.minQueryDuration duration
     The minimum duration for queries to track in query stats at /api/v1/status/top_queries. Queries with lower duration are ignored in query stats (default 1ms)
  -search.resetCacheAuthKey string
     Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call
  -search.setLookbackToStep
     Whether to fix lookback interval to 'step' query arg value. If set to true, the query model becomes closer to InfluxDB data model. If set to true, then -search.maxLookback and -search.maxStalenessInterval are ignored
  -search.skipSlowReplicas
     Whether to skip -replicationFactor - 1 slowest vmstorage nodes during querying. Enabling this setting may improve query speed, but it could also lead to incomplete results if some queried data has less than -replicationFactor copies at vmstorage nodes. Consider enabling this setting only if all the queried data contains -replicationFactor copies in the cluster
  -search.treatDotsAsIsInRegexps
     Whether to treat dots as is in regexp label filters used in queries. For example, foo{bar=~"a.b.c"} will be automatically converted to foo{bar=~"a\\.b\\.c"}, i.e. all the dots in regexp filters will be automatically escaped in order to match only dot char instead of matching any char. Dots in ".+", ".*" and ".{n}" regexps aren't escaped. This option is DEPRECATED in favor of {__graphite__="a.*.c"} syntax for selecting metrics matching the given Graphite metrics filter
  -selectNode array
     Comma-separated addresses of vmselect nodes; usage: -selectNode=vmselect-host1,...,vmselect-hostN
     Supports an array of values separated by comma or specified via multiple flags.
  -storageNode array
     Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN . Enterprise version of VictoriaMetrics supports automatic discovery of vmstorage addresses via dns+srv records. For example, -storageNode=dns+srv:vmstorage.addrs . See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery
     Supports an array of values separated by comma or specified via multiple flags.
  -storageNode.discoveryInterval duration
     Interval for refreshing -storageNode list behind dns+srv records. The minimum supported interval is 1s. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html (default 2s)
  -storageNode.filter string
     An optional regexp filter for discovered -storageNode addresses according to https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery. Discovered addresses matching the filter are retained, while other addresses are ignored. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
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
  -vmalert.proxyURL string
     Optional URL for proxying requests to vmalert. For example, if -vmalert.proxyURL=http://vmalert:8880 , then alerting API requests such as /api/v1/rules from Grafana will be proxied to http://vmalert:8880/api/v1/rules
  -vmstorageDialTimeout duration
     Timeout for establishing RPC connections from vmselect to vmstorage. See also -vmstorageUserTimeout (default 3s)
  -vmstorageUserTimeout duration
     Network timeout for RPC connections from vmselect to vmstorage (Linux only). Lower values reduce the maximum query durations when some vmstorage nodes become unavailable because of networking issues. Read more about TCP_USER_TIMEOUT at https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/ . See also -vmstorageDialTimeout (default 3s)
  -vmui.customDashboardsPath string
     Optional path to vmui dashboards. See https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
```

### List of command-line flags for vmstorage

Below is the output for `/path/to/vmstorage -help`:

```
  -bigMergeConcurrency int
     Deprecated: this flag does nothing. Please use -smallMergeConcurrency for controlling the concurrency of background merges. See https://docs.victoriametrics.com/#storage
  -blockcache.missesBeforeCaching int
     The number of cache misses before putting the block into cache. Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage (default 2)
  -cacheExpireDuration duration
     Items are removed from in-memory caches after they aren't accessed for this duration. Lower values may reduce memory usage at the cost of higher CPU usage. See also -prevCacheRemovalPercent (default 30m0s)
  -cluster.tls
     Whether to use TLS when accepting connections from vminsert and vmselect. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCAFile string
     Path to TLS CA file to use for verifying certificates provided by vminsert and vmselect if -cluster.tls flag is set. By default system CA is used. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCertFile string
     Path to server-side TLS certificate file to use when accepting connections from vminsert and vmselect if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsCipherSuites array
     Optional list of TLS cipher suites used for connections from vminsert and vmselect if -cluster.tls flag is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
  -cluster.tlsInsecureSkipVerify
     Whether to skip verification of TLS certificates provided by vminsert and vmselect if -cluster.tls flag is set. Note that disabled TLS certificate verification breaks security. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -cluster.tlsKeyFile string
     Path to server-side TLS key file to use when accepting connections from vminsert and vmselect if -cluster.tls flag is set. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection . This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
  -dedup.minScrapeInterval duration
     Leave only the last sample in every time series per each discrete interval equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details
  -denyQueriesOutsideRetention
     Whether to deny queries outside of the configured -retentionPeriod. When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. This may be useful when multiple data sources with distinct retentions are hidden behind query-tee
  -denyQueryTracing
     Whether to disable the ability to trace queries. See https://docs.victoriametrics.com/#query-tracing
  -downsampling.period array
     Comma-separated downsampling periods in the format 'offset:period'. For example, '30d:10m' instructs to leave a single sample per 10 minutes for samples older than 30 days. When setting multiple downsampling periods, it is necessary for the periods to be multiples of each other. See https://docs.victoriametrics.com/#downsampling for details. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise.html
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -finalMergeDelay duration
     The delay before starting final merge for per-month partition after no new data is ingested into it. Final merge may require additional disk IO and CPU resources. Final merge may increase query speed and reduce disk space usage in some cases. Zero value disables final merge
  -flagsAuthKey string
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -forceFlushAuthKey string
     authKey, which must be passed in query string to /internal/force_flush pages
  -forceMergeAuthKey string
     authKey, which must be passed in query string to /internal/force_merge pages
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
     Value for 'Content-Security-Policy' header
  -http.header.frameOptions string
     Value for 'X-Frame-Options' header
  -http.header.hsts string
     Value for 'Strict-Transport-Security' header
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
     Address to listen for http connections. See also -httpListenAddr.useProxyProtocol (default ":8482")
  -httpListenAddr.useProxyProtocol
     Whether to use proxy protocol for connections accepted at -httpListenAddr . See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing
  -inmemoryDataFlushInterval duration
     The interval for guaranteed saving of in-memory data to disk. The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). Smaller intervals increase disk IO load. Minimum supported value is 1s (default 5s)
  -insert.maxQueueDuration duration
     The maximum duration to wait in the queue when -maxConcurrentInserts concurrent insert requests are executed (default 1m0s)
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
  -logNewSeries
     Whether to log new series. This option is for debug purposes only. It can lead to performance issues when big number of new series are ingested into VictoriaMetrics
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
  -maxConcurrentInserts int
     The maximum number of concurrent insert requests. Default value should work for most cases, since it minimizes the memory usage. The default value can be increased when clients send data over slow networks. See also -insert.maxQueueDuration.
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey string
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pprofAuthKey string
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -precisionBits int
     The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss (default 64)
  -prevCacheRemovalPercent float
     Items in the previous caches are removed when the percent of requests it serves becomes lower than this value. Higher values reduce memory usage at the cost of higher CPU usage. See also -cacheExpireDuration (default 0.1)
  -pushmetrics.disableCompression
     Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.header array
     Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
  -pushmetrics.interval duration
     Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
  -retentionFilter array
     Retention filter in the format 'filter:retention'. For example, '{env="dev"}:3d' configures the retention for time series with env="dev" label to 3 days. See https://docs.victoriametrics.com/#retention-filters for details. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise.html
     Supports an array of values separated by comma or specified via multiple flags.
  -retentionPeriod value
     Data with timestamps outside the retentionPeriod is automatically deleted. The minimum retentionPeriod is 24h or 1d. See also -retentionFilter
     The following optional suffixes are supported: s (second), m (minute), h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 1)
  -retentionTimezoneOffset duration
     The offset for performing indexdb rotation. If set to 0, then the indexdb rotation is performed at 4am UTC time per each -retentionPeriod. If set to 2h, then the indexdb rotation is performed at 4am EET time (the timezone with +2h offset)
  -rpc.disableCompression
     Whether to disable compression of the data sent from vmstorage to vmselect. This reduces CPU usage at the cost of higher network bandwidth usage
  -search.maxConcurrentRequests int
     The maximum number of concurrent vmselect requests the vmstorage can process at -vmselectAddr. It shouldn't be high, since a single request usually saturates a CPU core, and many concurrently executed requests may require high amounts of memory. See also -search.maxQueueDuration (default 32)
  -search.maxQueueDuration duration
     The maximum time the incoming vmselect request waits for execution when -search.maxConcurrentRequests limit is reached (default 10s)
  -search.maxTagKeys int
     The maximum number of tag keys returned per search (default 100000)
  -search.maxTagValueSuffixesPerSearch int
     The maximum number of tag value suffixes returned from /metrics/find (default 100000)
  -search.maxTagValues int
     The maximum number of tag values returned per search (default 100000)
  -search.maxUniqueTimeseries int
     The maximum number of unique time series, which can be scanned during every query. This allows protecting against heavy queries, which select unexpectedly high number of series. Zero means 'no limit'. See also -search.max* command-line flags at vmselect
  -smallMergeConcurrency int
     The maximum number of workers for background merges. See https://docs.victoriametrics.com/#storage . It isn't recommended tuning this flag in general case, since this may lead to uncontrolled increase in the number of parts and increased CPU usage during queries
  -snapshotAuthKey string
     authKey, which must be passed in query string to /snapshot* pages
  -snapshotCreateTimeout duration
     The timeout for creating new snapshot. If set, make sure that timeout is lower than backup period
  -snapshotsMaxAge value
     Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted
     The following optional suffixes are supported: s (second), m (minute), h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 0)
  -storage.cacheSizeIndexDBDataBlocks size
     Overrides max size for indexdb/dataBlocks cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -storage.cacheSizeIndexDBIndexBlocks size
     Overrides max size for indexdb/indexBlocks cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -storage.cacheSizeIndexDBTagFilters size
     Overrides max size for indexdb/tagFiltersToMetricIDs cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -storage.cacheSizeStorageTSID size
     Overrides max size for storage/tsid cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -storage.maxDailySeries int
     The maximum number of unique series can be added to the storage during the last 24 hours. Excess series are logged and dropped. This can be useful for limiting series churn rate. See https://docs.victoriametrics.com/#cardinality-limiter . See also -storage.maxHourlySeries
  -storage.maxHourlySeries int
     The maximum number of unique series can be added to the storage during the last hour. Excess series are logged and dropped. This can be useful for limiting series cardinality. See https://docs.victoriametrics.com/#cardinality-limiter . See also -storage.maxDailySeries
  -storage.minFreeDiskSpaceBytes size
     The minimum free disk space at -storageDataPath after which the storage stops accepting new data
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 10000000)
  -storage.vminsertConnsShutdownDuration duration
     The time needed for gradual closing of vminsert connections during graceful shutdown. Bigger duration reduces spikes in CPU, RAM and disk IO load on the remaining vmstorage nodes during rolling restart. Smaller duration reduces the time needed to close all the vminsert connections, thus reducing the time for graceful shutdown. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#improving-re-routing-performance-during-restart (default 25s)
  -storageDataPath string
     Path to storage data (default "vmstorage-data")
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
  -vminsertAddr string
     TCP address to accept connections from vminsert services (default ":8400")
  -vmselectAddr string
     TCP address to accept connections from vmselect services (default ":8401")
```

## VictoriaMetrics Logo

[Zip](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/VM_logo.zip) contains three folders with different image orientation (main color and inverted version).

Files included in each folder:

- 2 JPEG Preview files
- 2 PNG Preview files with transparent background
- 2 EPS Adobe Illustrator EPS10 files

### Logo Usage Guidelines

#### Font used

- Lato Black
- Lato Regular

#### Color Palette

- HEX [#110f0f](https://www.color-hex.com/color/110f0f)
- HEX [#ffffff](https://www.color-hex.com/color/ffffff)

### We kindly ask

- Please don't use any other font instead of suggested.
- There should be sufficient clear space around the logo.
- Do not change spacing, alignment, or relative locations of the design elements.
- Do not change the proportions of any of the design elements or the design itself. You    may resize as needed but must retain all proportions.
