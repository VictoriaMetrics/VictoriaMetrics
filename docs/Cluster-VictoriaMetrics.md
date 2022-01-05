---
sort: 2
---

# Cluster version

<img alt="VictoriaMetrics" src="logo.png">

VictoriaMetrics is a fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.

It is recommended using [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics) instead of cluster version
for ingestion rates lower than a million of data points per second.
Single-node version [scales perfectly](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
with the number of CPU cores, RAM and available storage space.
Single-node version is easier to configure and operate comparing to cluster version, so think twice before sticking to cluster version.

Join [our Slack](https://slack.victoriametrics.com/) or [contact us](mailto:info@victoriametrics.com) with consulting and support questions.


## Prominent features

- Supports all the features of [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics).
- Performance and capacity scales horizontally. See [these docs for details](#cluster-resizing-and-scalability).
- Supports multiple independent namespaces for time series data (aka multi-tenancy). See [these docs for details](#multitenancy).
- Supports replication. See [these docs for details](#replication-and-data-safety).


## Architecture overview

VictoriaMetrics cluster consists of the following services:

- `vmstorage` - stores the raw data and returns the queried data on the given time range for the given label filters
- `vminsert` - accepts the ingested data and spreads it among `vmstorage` nodes according to consistent hashing over metric name and all its labels
- `vmselect` - performs incoming queries by fetching the needed data from all the configured `vmstorage` nodes

Each service may scale independently and may run on the most suitable hardware.
`vmstorage` nodes don't know about each other, don't communicate with each other and don't share any data.
This is [shared nothing architecture](https://en.wikipedia.org/wiki/Shared-nothing_architecture).
It increases cluster availability, simplifies cluster maintenance and cluster scaling.

<img src="https://docs.google.com/drawings/d/e/2PACX-1vTvk2raU9kFgZ84oF-OKolrGwHaePhHRsZEcfQ1I_EC5AB_XPWwB392XshxPramLJ8E4bqptTnFn5LL/pub?w=1104&amp;h=746">


## Multitenancy

VictoriaMetrics cluster supports multiple isolated tenants (aka namespaces).
Tenants are identified by `accountID` or `accountID:projectID`, which are put inside request urls.
See [these docs](#url-format) for details. Some facts about tenants in VictoriaMetrics:

* Each `accountID` and `projectID` is identified by an arbitrary 32-bit integer in the range `[0 .. 2^32)`.
If `projectID` is missing, then it is automatically assigned to `0`. It is expected that other information about tenants
such as auth tokens, tenant names, limits, accounting, etc. is stored in a separate relational database. This database must be managed
by a separate service sitting in front of VictoriaMetrics cluster such as [vmauth](https://docs.victoriametrics.com/vmauth.html) or [vmgateway](https://docs.victoriametrics.com/vmgateway.html). [Contact us](mailto:info@victoriametrics.com) if you need assistance with such service.

* Tenants are automatically created when the first data point is written into the given tenant.

* Data for all the tenants is evenly spread among available `vmstorage` nodes. This guarantees even load among `vmstorage` nodes
when different tenants have different amounts of data and different query load.

* The database performance and resource usage doesn't depend on the number of tenants. It depends mostly on the total number of active time series in all the tenants. A time series is considered active if it received at least a single sample during the last hour or it has been touched by queries during the last hour.

* VictoriaMetrics doesn't support querying multiple tenants in a single request.


## Binaries

Compiled binaries for cluster version are available in the `assets` section of [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
See archives containing `cluster` word.

Docker images for cluster version are available here:

- `vminsert` - https://hub.docker.com/r/victoriametrics/vminsert/tags
- `vmselect` - https://hub.docker.com/r/victoriametrics/vmselect/tags
- `vmstorage` - https://hub.docker.com/r/victoriametrics/vmstorage/tags


## Building from sources

Source code for cluster version is available at [cluster branch](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).


### Production builds

There is no need in installing Go on a host system since binaries are built
inside [the official docker container for Go](https://hub.docker.com/_/golang).
This makes reproducible builds.
So [install docker](https://docs.docker.com/install/) and run the following command:

```
make vminsert-prod vmselect-prod vmstorage-prod
```

Production binaries are built into statically linked binaries. They are put into `bin` folder with `-prod` suffixes:
```
$ make vminsert-prod vmselect-prod vmstorage-prod
$ ls -1 bin
vminsert-prod
vmselect-prod
vmstorage-prod
```

### Development Builds

1. [Install go](https://golang.org/doc/install). The minimum supported version is Go 1.17.
2. Run `make` from [the repository root](https://github.com/VictoriaMetrics/VictoriaMetrics). It should build `vmstorage`, `vmselect`
   and `vminsert` binaries and put them into the `bin` folder.


### Building docker images

Run `make package`. It will build the following docker images locally:

* `victoriametrics/vminsert:<PKG_TAG>`
* `victoriametrics/vmselect:<PKG_TAG>`
* `victoriametrics/vmstorage:<PKG_TAG>`

`<PKG_TAG>` is auto-generated image tag, which depends on source code in [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package`.

By default images are built on top of [alpine](https://hub.docker.com/_/scratch) image in order to improve debuggability.
It is possible to build an image on top of any other base image by setting it via `<ROOT_IMAGE>` environment variable.
For example, the following command builds images on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package
```

## Operation

## Cluster setup

A minimal cluster must contain the following nodes:

* a single `vmstorage` node with `-retentionPeriod` and `-storageDataPath` flags
* a single `vminsert` node with `-storageNode=<vmstorage_host>`
* a single `vmselect` node with `-storageNode=<vmstorage_host>`

It is recommended to run at least two nodes for each service
for high availability purposes.

An http load balancer such as [vmauth](https://docs.victoriametrics.com/vmauth.html) or `nginx` must be put in front of `vminsert` and `vmselect` nodes. It must contain the following routing configs according to [the url format](#url-format):
- requests starting with `/insert` must be routed to port `8480` on `vminsert` nodes.
- requests starting with `/select` must be routed to port `8481` on `vmselect` nodes.

Ports may be altered by setting `-httpListenAddr` on the corresponding nodes.

It is recommended setting up [monitoring](#monitoring) for the cluster.

The following tools can simplify cluster setup:
* [An example docker-compose config for VictoriaMetrics cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/deployment/docker/docker-compose.yml)
* [Helm charts for VictoriaMetrics](https://github.com/VictoriaMetrics/helm-charts)
* [Kubernetes operator for VictoriaMetrics](https://github.com/VictoriaMetrics/operator)


It is possible manualy setting up a toy cluster on a single host. In this case every cluster component - `vminsert`, `vmselect` and `vmstorage` - must have distinct values for `-httpListenAddr` command-line flag. This flag specifies http address for accepting http requests for [monitoring](#monitoring) and [profiling](#profiling). `vmstorage` node must have distinct values for the following additional command-line flags in order to prevent resource usage clash:
* `-storageDataPath` - every `vmstorage` node must have a dedicated data storage.
* `-vminsertAddr` - every `vmstorage` node must listen for a distinct tcp address for accepting data from `vminsert` nodes.
* `-vmselectAddr` - every `vmstorage` node must listen for a distinct tcp address for accepting requests from `vmselect` nodes.


### Environment variables

Each flag values can be set thru environment variables by following these rules:

- The `-envflag.enable` flag must be set
- Each `.` in flag names must be substituted by `_` (for example `-insert.maxQueueDuration <duration>` will translate to `insert_maxQueueDuration=<duration>`)
- For repeating flags, an alternative syntax can be used by joining the different values into one using `,` as separator (for example `-storageNode <nodeA> -storageNode <nodeB>` will translate to `storageNode=<nodeA>,<nodeB>`)
- It is possible setting prefix for environment vars with `-envflag.prefix`. For instance, if `-envflag.prefix=VM_`, then env vars must be prepended with `VM_`


## Monitoring

All the cluster components expose various metrics in Prometheus-compatible format at `/metrics` page on the TCP port set in `-httpListenAddr` command-line flag.
By default the following TCP ports are used:
- `vminsert` - 8480
- `vmselect` - 8481
- `vmstorage` - 8482

It is recommended setting up [vmagent](https://docs.victoriametrics.com/vmagent.html)
or Prometheus to scrape `/metrics` pages from all the cluster components, so they can be monitored and analyzed
with [the official Grafana dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11176)
or [an alternative dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11831). Graphs on these dashboards contain useful hints - hover the `i` icon at the top left corner of each graph in order to read it.

It is recommended setting up alerts in [vmalert](https://docs.victoriametrics.com/vmalert.html) or in Prometheus from [this config](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/deployment/docker/alerts.yml).

## Readonly mode

`vmstorage` nodes automatically switch to readonly mode when the directory pointed by `-storageDataPath` contains less than `-storage.minFreeDiskSpaceBytes` of free space. `vminsert` nodes stop sending data to such nodes and start re-routing the data to the remaining `vmstorage` nodes.



## URL format

* URLs for data ingestion: `http://<vminsert>:8480/insert/<accountID>/<suffix>`, where:
  - `<accountID>` is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). It is possible to set it as `accountID:projectID`,
    where `projectID` is also arbitrary 32-bit integer. If `projectID` isn't set, then it equals to `0`.
  - `<suffix>` may have the following values:
     - `prometheus` and `prometheus/api/v1/write` - for inserting data with [Prometheus remote write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write).
     - `datadog/api/v1/series` - for inserting data with [DataDog submit metrics API](https://docs.datadoghq.com/api/latest/metrics/#submit-metrics). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-datadog-agent) for details.
     - `influx/write` and `influx/api/v2/write` - for inserting data with [InfluxDB line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf) for details.
     - `opentsdb/api/put` - for accepting [OpenTSDB HTTP /api/put requests](http://opentsdb.net/docs/build/html/api_http/put.html). This handler is disabled by default. It is exposed on a distinct TCP address set via `-opentsdbHTTPListenAddr` command-line flag. See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#sending-opentsdb-data-via-http-apiput-requests) for details.
     - `prometheus/api/v1/import` - for importing data obtained via `api/v1/export` at `vmselect` (see below).
     - `prometheus/api/v1/import/native` - for importing data obtained via `api/v1/export/native` on `vmselect` (see below).
     - `prometheus/api/v1/import/csv` - for importing arbitrary CSV data. See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-csv-data) for details.
     - `prometheus/api/v1/import/prometheus` - for importing data in [Prometheus text exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format) and in [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md). See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-prometheus-exposition-format) for details.

* URLs for [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/): `http://<vmselect>:8481/select/<accountID>/prometheus/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying data namespace for the query (aka tenant)
  - `<suffix>` may have the following values:
    - `api/v1/query` - performs [PromQL instant query](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries).
    - `api/v1/query_range` - performs [PromQL range query](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).
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

* URLs for [Graphite Metrics API](https://graphite-api.readthedocs.io/en/latest/api.html#the-metrics-api): `http://<vmselect>:8481/select/<accountID>/graphite/<suffix>`, where:
    - `<accountID>` is an arbitrary number identifying data namespace for query (aka tenant)
    - `<suffix>` may have the following values:
      - `render` - implements Graphite Render API. See [these docs](https://graphite.readthedocs.io/en/stable/render_api.html). This functionality is available in [Enterprise package](https://victoriametrics.com/products/enterprise/).
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

* URL with basic Web UI: `http://<vmselect>:8481/select/<accountID>/vmui/`.

* URL for query stats across all tenants: `http://<vmselect>:8481/api/v1/status/top_queries`. It lists with the most frequently executed queries and queries taking the most duration.

* URL for time series deletion: `http://<vmselect>:8481/delete/<accountID>/prometheus/api/v1/admin/tsdb/delete_series?match[]=<timeseries_selector_for_delete>`.
  Note that the `delete_series` handler should be used only in exceptional cases such as deletion of accidentally ingested incorrect time series. It shouldn't
  be used on a regular basis, since it carries non-zero overhead.

* `vmstorage` nodes provide the following HTTP endpoints on `8482` port:
  - `/internal/force_merge` - initiate [forced compactions](https://docs.victoriametrics.com/#forced-merge) on the given `vmstorage` node.
  - `/snapshot/create` - create [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282),
    which can be used for backups in background. Snapshots are created in `<storageDataPath>/snapshots` folder, where `<storageDataPath>` is the corresponding
    command-line flag value.
  - `/snapshot/list` - list available snasphots.
  - `/snapshot/delete?snapshot=<id>` - delete the given snapshot.
  - `/snapshot/delete_all` - delete all the snapshots.

  Snapshots may be created independently on each `vmstorage` node. There is no need in synchronizing snapshots' creation
  across `vmstorage` nodes.


## Cluster resizing and scalability

Cluster performance and capacity scales with adding new nodes.

* `vminsert` and `vmselect` nodes are stateless and may be added / removed at any time.
  Do not forget updating the list of these nodes on http load balancer.
  Adding more `vminsert` nodes scales data ingestion rate. See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/175#issuecomment-536925841)
  about ingestion rate scalability.
  Adding more `vmselect` nodes scales select queries rate.
* `vmstorage` nodes own the ingested data, so they cannot be removed without data loss.
  Adding more `vmstorage` nodes scales cluster capacity.

Steps to add `vmstorage` node:

1. Start new `vmstorage` node with the same `-retentionPeriod` as existing nodes in the cluster.
2. Gradually restart all the `vmselect` nodes with new `-storageNode` arg containing `<new_vmstorage_host>`.
3. Gradually restart all the `vminsert` nodes with new `-storageNode` arg containing `<new_vmstorage_host>`.


## Updating / reconfiguring cluster nodes

All the node types - `vminsert`, `vmselect` and `vmstorage` - may be updated via graceful shutdown.
Send `SIGINT` signal to the corresponding process, wait until it finishes and then start new version
with new configs.

Cluster should remain in working state if at least a single node of each type remains available during
the update process. See [cluster availability](#cluster-availability) section for details.

See also more advanced [cardinality limiter in vmagent](https://docs.victoriametrics.com/vmagent.html#cardinality-limiter).


## Cluster availability

* HTTP load balancer must stop routing requests to unavailable `vminsert` and `vmselect` nodes.
* The cluster remains available if at least a single `vmstorage` node exists:

  - `vminsert` re-routes incoming data from unavailable `vmstorage` nodes to healthy `vmstorage` nodes
  - `vmselect` continues serving partial responses if at least a single `vmstorage` node is available. If consistency over availability is preferred, then either pass `-search.denyPartialResponse` command-line flag to `vmselect` or pass `deny_partial_response=1` query arg in requests to `vmselect`.

`vmselect` doesn't serve partial responses for API handlers returning raw datapoints - [`/api/v1/export*` endpoints](https://docs.victoriametrics.com/#how-to-export-time-series), since users usually expect this data is always complete.

Data replication can be used for increasing storage durability. See [these docs](#replication-and-data-safety) for details.


## Capacity planning

VictoriaMetrics uses lower amounts of CPU, RAM and storage space on production workloads compared to competing solutions (Prometheus, Thanos, Cortex, TimescaleDB, InfluxDB, QuestDB, M3DB) according to [our case studies](https://docs.victoriametrics.com/CaseStudies.html).

Each node type - `vminsert`, `vmselect` and `vmstorage` - can run on the most suitable hardware. Cluster capacity scales linearly with the available resources. The needed amounts of CPU and RAM per each node type highly depends on the workload - the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series), [series churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate), query types, query qps, etc. It is recommended setting up a test VictoriaMetrics cluster for your production workload and iteratively scaling per-node resources and the number of nodes per node type until the cluster becomes stable. It is recommended setting up [monitoring for the cluster](#monitoring). It helps determining bottlenecks in cluster setup. It is also recommended following [the troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting).

The needed storage space for the given retention (the retention is set via `-retentionPeriod` command-line flag at `vmstorage`) can be extrapolated from disk space usage in a test run. For example, if the storage space usage is 10GB after a day-long test run on a production workload, then it will need at least `10GB*100=1TB` of disk space for `-retentionPeriod=100d` (100-days retention period). Storage space usage can be monitored with [the official Grafana dashboard for VictoriaMetrics cluster](#monitoring).

It is recommended leaving the following amounts of spare resources:

* 50% of free RAM across all the node types for reducing the probability of OOM (out of memory) crashes and slowdowns during temporary spikes in workload.
* 50% of spare CPU across all the node types for reducing the probability of slowdowns during temporary spikes in workload.
* At least 30% of free storage space at the directory pointed by `-storageDataPath` command-line flag at `vmstorage` nodes. See also `-storage.minFreeDiskSpaceBytes` command-line flag [description for vmstorage](#list-of-command-line-flags-for-vmstorage).


Some capacity planning tips for VictoriaMetrics cluster:

* The [replication](#replication-and-data-safety) increases the amounts of needed resources for the cluster by up to `N` times where `N` is replication factor.
* Cluster capacity for [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-active-time-series) can be increased by adding more `vmstorage` nodes and/or by increasing RAM and CPU resources per each `vmstorage` node.
* Query latency can be reduced by increasing the number of `vmstorage` nodes and/or by increasing RAM and CPU resources per each `vmselect` node.
* The total number of CPU cores needed for all the `vminsert` nodes can be calculated from the ingestion rate: `CPUs = ingestion_rate / 100K`.
* The `-rpc.disableCompression` command-line flag at `vminsert` nodes can increase ingestion capacity at the cost of higher network bandwidth usage between `vminsert` and `vmstorage`.


## High availability

It is recommended to run all the components for a single cluster in the same subnetwork with high bandwidth, low latency and low error rates.
This improves cluster performance and availability.
It isn't recommended spreading components for a single cluster across multiple availability zones, since cross-AZ network usually has lower bandwidth, higher latency
and higher error rates comparing the network inside AZ.

If you need multi-AZ setup, then it is recommended running independed clusters in each AZ and setting up
[vmagent](https://docs.victoriametrics.com/vmagent.html) in front of these clusters, so it could replicate incoming data
into all the cluster. Then [promxy](https://github.com/jacksontj/promxy) could be used for querying the data from multiple clusters.

Another solution is to use [multi-level cluster setup](#multi-level-cluster-setup).


## Multi-level cluster setup

`vminsert` nodes can accept data from another `vminsert` nodes starting from [v1.60.0](https://docs.victoriametrics.com/CHANGELOG.html#v1600) if `-clusternativeListenAddr` command-line flag is set. For example, if `vminsert` is started with `-clusternativeListenAddr=:8400` command-line flag, then it can accept data from another `vminsert` nodes at TCP port 8400 in the same way as `vmstorage` nodes do. This allows chaining `vminsert` nodes and building multi-level cluster topologies with flexible configs. For example, the top level of `vminsert` nodes can replicate data among the second level of `vminsert` nodes located in distinct availability zones (AZ), while the second-level `vminsert` nodes can spread the data among `vmstorage` nodes located in the same AZ. Such setup guarantees cluster availability if some AZ becomes unavailable. The data from all the `vmstorage` nodes in all the AZs can be read via `vmselect` nodes, which are configured to query all the `vmstorage` nodes in all the availability zones (e.g. all the `vmstorage` addresses are passed via `-storageNode` command-line flag to `vmselect` nodes). Additionally, `-replicationFactor=k+1` must be passed to `vmselect` nodes, where `k` is the lowest number of `vmstorage` nodes in a single AZ. See [replication docs](#replication-and-data-safety) for more details.

Another option is to set up [vmagent](https://docs.victoriametrics.com/vmagent.html) for replicating the data among multiple VictoriaMetrics clusters. See [these docs](https://docs.victoriametrics.com/vmagent.html#multitenancy) for details.


## Helm

Helm chart simplifies managing cluster version of VictoriaMetrics in Kubernetes.
It is available in the [helm-charts](https://github.com/VictoriaMetrics/helm-charts) repository.


## Kubernetes operator

[K8s operator](https://github.com/VictoriaMetrics/operator) simplifies managing VictoriaMetrics components in Kubernetes.


## Replication and data safety

By default VictoriaMetrics offloads replication to the underlying storage pointed by `-storageDataPath`.

The replication can be enabled by passing `-replicationFactor=N` command-line flag to `vminsert`.
This guarantees that all the data remains available for querying if up to `N-1` `vmstorage` nodes are unavailable.
The cluster must contain at least `2*N-1` `vmstorage` nodes, where `N`
is replication factor, in order to maintain the given replication factor for newly ingested data when `N-1` of storage nodes are lost.
For example, when `-replicationFactor=3` is passed to `vminsert`, then it replicates all the ingested data to 3 distinct `vmstorage` nodes,
so up to 2 `vmstorage` nodes can be lost without data loss. The minimum number of `vmstorage` nodes should be equal to `2*3-1 = 5`, so when 2 `vmstorage` nodes are lost,
the remaining 3 `vmstorage` nodes could provide the `-replicationFactor=3` for newly ingested data.

When the replication is enabled, `-dedup.minScrapeInterval=1ms` command-line flag must be passed to `vmselect` nodes.
Optional `-replicationFactor=N` command-line flag can be passed to `vmselect` for improving query performance when up to `N-1` vmstorage nodes respond slowly and/or temporarily unavailable, since `vmselect` doesn't wait for responses from up to `N-1` `vmstorage` nodes. Sometimes `-replicationFactor` at `vmselect` nodes can result in partial responses. See [this issues](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1207) for details.
The `-dedup.minScrapeInterval=1ms` de-duplicates replicated data during queries. If duplicate data is pushed to VictoriaMetrics from identically configured [vmagent](https://docs.victoriametrics.com/vmagent.html) instances or Prometheus instances, then the `-dedup.minScrapeInterval` must be set to bigger values according to [deduplication docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#deduplication).

Note that [replication doesn't save from disaster](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883),
so it is recommended performing regular backups. See [these docs](#backups) for details.

Note that the replication increases resource usage - CPU, RAM, disk space, network bandwidth - by up to `-replicationFactor` times. So it may be worth
offloading the replication to underlying storage pointed by `-storageDataPath` such as [Google Compute Engine persistent disk](https://cloud.google.com/compute/docs/disks/#pdspecs),
which is protected from data loss and data corruption. It also provide consistently high performance
and [may be resized](https://cloud.google.com/compute/docs/disks/add-persistent-disk) without downtime.
HDD-based persistent disks should be enough for the majority of use cases.

It is recommended using durable replicated persistent volumes in Kubernetes.


## Backups

It is recommended performing periodical backups from [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for protecting from user errors such as accidental data deletion.

The following steps must be performed for each `vmstorage` node for creating a backup:

1. Create an instant snapshot by navigating to `/snapshot/create` HTTP handler. It will create snapshot and return its name.
2. Archive the created snapshot from `<-storageDataPath>/snapshots/<snapshot_name>` folder using [vmbackup](https://docs.victoriametrics.com/vmbackup.html).
   The archival process doesn't interfere with `vmstorage` work, so it may be performed at any suitable time.
3. Delete unused snapshots via `/snapshot/delete?snapshot=<snapshot_name>` or `/snapshot/delete_all` in order to free up occupied storage space.

There is no need in synchronizing backups among all the `vmstorage` nodes.

Restoring from backup:

1. Stop `vmstorage` node with `kill -INT`.
2. Restore data from backup using [vmrestore](https://docs.victoriametrics.com/vmrestore.html) into `-storageDataPath` directory.
3. Start `vmstorage` node.


## Downsampling

Downsampling is available in [enterprise version of VictoriaMetrics](https://victoriametrics.com/products/enterprise/). It is configured with `-downsampling.period` command-line flag. The same flag value must be passed to both `vmstorage` and `vmselect` nodes. See [these docs](https://docs.victoriametrics.com/#downsampling) for details.


## Profiling

All the cluster components provide the following handlers for [profiling](https://blog.golang.org/profiling-go-programs):

* `http://vminsert:8480/debug/pprof/heap` for memory profile and `http://vminsert:8480/debug/pprof/profile` for CPU profile
* `http://vmselect:8481/debug/pprof/heap` for memory profile and `http://vmselect:8481/debug/pprof/profile` for CPU profile
* `http://vmstorage:8482/debug/pprof/heap` for memory profile and `http://vmstorage:8482/debug/pprof/profile` for CPU profile

Example command for collecting cpu profile from `vmstorage`:

```bash
curl -s http://vmstorage:8482/debug/pprof/profile > cpu.pprof
```

Example command for collecting memory profile from `vminsert`:

```bash
curl -s http://vminsert:8480/debug/pprof/heap > mem.pprof
```


## Community and contributions

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

* [List of command-line flags for vminsert](#list-of-command-line-flags-for-vminsert)
* [List of command-line flags for vmselect](#list-of-command-line-flags-for-vmselect)
* [List of command-line flags for vmstorage](#list-of-command-line-flags-for-vmstorage)


### List of command-line flags for vminsert

Below is the output for `/path/to/vminsert -help`:

```
  -clusternativeListenAddr string
    	TCP address to listen for data from other vminsert nodes in multi-level cluster setup. See https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multi-level-cluster-setup . Usually :8400 must be set. Doesn't work if empty
  -csvTrimTimestamp duration
    	Trim timestamps when importing csv data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -datadog.maxInsertRequestSize size
    	The maximum size in bytes of a single DataDog POST request to /api/v1/series
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 67108864)
  -disableRerouting
    	Whether to disable re-routing when some of vmstorage nodes accept incoming data at slower speed compared to other storage nodes. Disabled re-routing limits the ingestion rate by the slowest vmstorage node. On the other side, disabled re-routing minimizes the number of active time series in the cluster during rolling restarts and during spikes in series churn rate (default true)
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -eula
    	By specifying this flag, you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -graphiteListenAddr string
    	TCP and UDP address to listen for Graphite plaintext data. Usually :2003 must be set. Doesn't work if empty
  -graphiteTrimTimestamp duration
    	Trim timestamps for Graphite data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
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
  -httpListenAddr string
    	Address to listen for http connections (default ":8480")
  -import.maxLineLen size
    	The maximum length in bytes of a single line accepted by /api/v1/import; the line length can be limited with 'max_rows_per_line' query arg passed to /api/v1/export
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 104857600)
  -influx.databaseNames array
    	Comma-separated list of database names to return from /query and /influx/query API. This can be needed for accepting data from Telegraf plugins such as https://github.com/fangli/fluent-plugin-influxdb
    	Supports an array of values separated by comma or specified via multiple flags.
  -influx.maxLineSize size
    	The maximum size in bytes for a single InfluxDB line during parsing
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 262144)
  -influxListenAddr string
    	TCP and UDP address to listen for InfluxDB line protocol data. Usually :8189 must be set. Doesn't work if empty. This flag isn't needed when ingesting data over HTTP - just send it to http://<victoriametrics>:8428/write
  -influxMeasurementFieldSeparator string
    	Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol (default "_")
  -influxSkipMeasurement
    	Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'
  -influxSkipSingleField
    	Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if InfluxDB line contains only a single field
  -influxTrimTimestamp duration
    	Trim timestamps for InfluxDB line protocol data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -insert.maxQueueDuration duration
    	The maximum duration for waiting in the queue for insert requests due to -maxConcurrentInserts (default 1m0s)
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
  -maxConcurrentInserts int
    	The maximum number of concurrent inserts. Default value should work for most cases, since it minimizes the overhead for concurrent inserts. This option is tigthly coupled with -insert.maxQueueDuration (default 16)
  -maxInsertRequestSize size
    	The maximum size in bytes of a single Prometheus remote_write API request
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 33554432)
  -maxLabelValueLen int
    	The maximum length of label values in the accepted time series. Longer label values are truncated. In this case the vm_too_long_label_values_total metric at /metrics page is incremented (default 16384)
  -maxLabelsPerTimeseries int
    	The maximum number of labels accepted per time series. Superfluous labels are dropped. In this case the vm_metrics_with_dropped_labels_total metric at /metrics page is incremented (default 30)
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -opentsdbHTTPListenAddr string
    	TCP address to listen for OpentTSDB HTTP put requests. Usually :4242 must be set. Doesn't work if empty
  -opentsdbListenAddr string
    	TCP and UDP address to listen for OpentTSDB metrics. Telnet put messages and HTTP /api/put messages are simultaneously served on TCP port. Usually :4242 must be set. Doesn't work if empty
  -opentsdbTrimTimestamp duration
    	Trim timestamps for OpenTSDB 'telnet put' data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
  -opentsdbhttp.maxInsertRequestSize size
    	The maximum size of OpenTSDB HTTP put request
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 33554432)
  -opentsdbhttpTrimTimestamp duration
    	Trim timestamps for OpenTSDB HTTP data to this duration. Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data (default 1ms)
  -relabelConfig string
    	Optional path to a file with relabeling rules, which are applied to all the ingested metrics. The path can point either to local file or to http url. See https://docs.victoriametrics.com/#relabeling for details. The config is reloaded on SIGHUP signal
  -relabelDebug
    	Whether to log metrics before and after relabeling with -relabelConfig. If the -relabelDebug is enabled, then the metrics aren't sent to storage. This is useful for debugging the relabeling configs
  -replicationFactor int
    	Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. Higher values for -dedup.minScrapeInterval at vmselect is OK (default 1)
  -rpc.disableCompression
    	Whether to disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage
  -sortLabels
    	Whether to sort labels for incoming samples before writing them to storage. This may be needed for reducing memory usage at storage when the order of labels in incoming samples is random. For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}. Enabled sorting for labels can slow down ingestion performance a bit
  -storageNode array
    	Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN
    	Supports an array of values separated by comma or specified via multiple flags.
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
```

### List of command-line flags for vmselect

Below is the output for `/path/to/vmselect -help`:

```
  -cacheDataPath string
    	Path to directory for cache files. Cache isn't saved if empty
  -dedup.minScrapeInterval duration
    	Leave only the first sample in every time series per each discrete interval equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details
  -downsampling.period array
    	Comma-separated downsampling periods in the format 'offset:period'. For example, '30d:10m' instructs to leave a single sample per 10 minutes for samples older than 30 days. See https://docs.victoriametrics.com/#downsampling for details
    	Supports an array of values separated by comma or specified via multiple flags.
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -eula
    	By specifying this flag, you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -graphiteTrimTimestamp duration
    	Trim timestamps for Graphite data to this duration. Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data (default 1s)
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
  -httpListenAddr string
    	Address to listen for http connections (default ":8481")
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
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -replicationFactor int
    	How many copies of every time series is available on vmstorage nodes. See -replicationFactor command-line flag for vminsert nodes (default 1)
  -search.cacheTimestampOffset duration
    	The maximum duration since the current time for response data, which is always queried from the original raw data, without using the response cache. Increase this value if you see gaps in responses due to time synchronization issues between VictoriaMetrics and data sources (default 5m0s)
  -search.denyPartialResponse
    	Whether to deny partial responses if a part of -storageNode instances fail to perform queries; this trades availability over consistency; see also -search.maxQueryDuration
  -search.disableCache
    	Whether to disable response caching. This may be useful during data backfilling
  -search.graphiteMaxPointsPerSeries int
    	The maximum number of points per series Graphite render API can return (default 1000000)
  -search.graphiteStorageStep duration
    	The interval between datapoints stored in the database. It is used at Graphite Render API handler for normalizing the interval between datapoints in case it isn't normalized. It can be overriden by sending 'storage_step' query arg to /render API or by sending the desired interval via 'Storage-Step' http header during querying /render API (default 10s)
  -search.latencyOffset duration
    	The time when data points become visible in query results after the collection. Too small value can result in incomplete last points for query results (default 30s)
  -search.logSlowQueryDuration duration
    	Log queries with execution time exceeding this value. Zero disables slow query logging (default 5s)
  -search.maxConcurrentRequests int
    	The maximum number of concurrent search requests. It shouldn't be high, since a single request can saturate all the CPU cores. See also -search.maxQueueDuration (default 8)
  -search.maxExportDuration duration
    	The maximum duration for /api/v1/export call (default 720h0m0s)
  -search.maxLookback duration
    	Synonym to -search.lookback-delta from Prometheus. The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. See also '-search.maxStalenessInterval' flag, which has the same meaining due to historical reasons
  -search.maxPointsPerTimeseries int
    	The maximum points per a single timeseries returned from /api/v1/query_range. This option doesn't limit the number of scanned raw samples in the database. The main purpose of this option is to limit the number of per-series points returned to graphing UI such as Grafana. There is no sense in setting this limit to values bigger than the horizontal resolution of the graph (default 30000)
  -search.maxQueryDuration duration
    	The maximum duration for query execution (default 30s)
  -search.maxQueryLen size
    	The maximum search query length in bytes
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 16384)
  -search.maxQueueDuration duration
    	The maximum time the request waits for execution when -search.maxConcurrentRequests limit is reached; see also -search.maxQueryDuration (default 10s)
  -search.maxSamplesPerQuery int
    	The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries (default 1000000000)
  -search.maxSamplesPerSeries int
    	The maximum number of raw samples a single query can scan per each time series. See also -search.maxSamplesPerQuery (default 30000000)
  -search.maxStalenessInterval duration
    	The maximum interval for staleness calculations. By default it is automatically calculated from the median interval between samples. This flag could be useful for tuning Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. See also '-search.maxLookback' flag, which has the same meaning due to historical reasons
  -search.maxStatusRequestDuration duration
    	The maximum duration for /api/v1/status/* requests (default 5m0s)
  -search.maxStepForPointsAdjustment duration
    	The maximum step when /api/v1/query_range handler adjusts points with timestamps closer than -search.latencyOffset to the current time. The adjustment is needed because such points may contain incomplete data (default 1m0s)
  -search.minStalenessInterval duration
    	The minimum interval for staleness calculations. This flag could be useful for removing gaps on graphs generated from time series with irregular intervals between samples. See also '-search.maxStalenessInterval'
  -search.noStaleMarkers
    	Set this flag to true if the database doesn't contain Prometheus stale markers, so there is no need in spending additional CPU time on its handling. Staleness markers may exist only in data obtained from Prometheus scrape targets
  -search.queryStats.lastQueriesCount int
    	Query stats for /api/v1/status/top_queries is tracked on this number of last queries. Zero value disables query stats tracking (default 20000)
  -search.queryStats.minQueryDuration duration
    	The minimum duration for queries to track in query stats at /api/v1/status/top_queries. Queries with lower duration are ignored in query stats (default 1ms)
  -search.resetCacheAuthKey string
    	Optional authKey for resetting rollup cache via /internal/resetRollupResultCache call
  -search.treatDotsAsIsInRegexps
    	Whether to treat dots as is in regexp label filters used in queries. For example, foo{bar=~"a.b.c"} will be automatically converted to foo{bar=~"a\\.b\\.c"}, i.e. all the dots in regexp filters will be automatically escaped in order to match only dot char instead of matching any char. Dots in ".+", ".*" and ".{n}" regexps aren't escaped. This option is DEPRECATED in favor of {__graphite__="a.*.c"} syntax for selecting metrics matching the given Graphite metrics filter
  -selectNode array
    	Comma-serparated addresses of vmselect nodes; usage: -selectNode=vmselect-host1,...,vmselect-hostN
    	Supports an array of values separated by comma or specified via multiple flags.
  -storageNode array
    	Comma-separated addresses of vmstorage nodes; usage: -storageNode=vmstorage-host1,...,vmstorage-hostN
    	Supports an array of values separated by comma or specified via multiple flags.
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
```

### List of command-line flags for vmstorage

Below is the output for `/path/to/vmstorage -help`:

```
  -bigMergeConcurrency int
    	The maximum number of CPU cores to use for big merges. Default value is used if set to 0
  -dedup.minScrapeInterval duration
    	Leave only the first sample in every time series per each discrete interval equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details
  -denyQueriesOutsideRetention
    	Whether to deny queries outside of the configured -retentionPeriod. When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. This may be useful when multiple data sources with distinct retentions are hidden behind query-tee
  -downsampling.period array
    	Comma-separated downsampling periods in the format 'offset:period'. For example, '30d:10m' instructs to leave a single sample per 10 minutes for samples older than 30 days. See https://docs.victoriametrics.com/#downsampling for details
    	Supports an array of values separated by comma or specified via multiple flags.
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -eula
    	By specifying this flag, you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -finalMergeDelay duration
    	The delay before starting final merge for per-month partition after no new data is ingested into it. Final merge may require additional disk IO and CPU resources. Final merge may increase query speed and reduce disk space usage in some cases. Zero value disables final merge
  -forceFlushAuthKey string
    	authKey, which must be passed in query string to /internal/force_flush pages
  -forceMergeAuthKey string
    	authKey, which must be passed in query string to /internal/force_merge pages
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
  -httpListenAddr string
    	Address to listen for http connections (default ":8482")
  -logNewSeries
    	Whether to log new series. This option is for debug purposes only. It can lead to performance issues when big number of new series are ingested into VictoriaMetrics
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
  -memory.allowedBytes size
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -precisionBits int
    	The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss (default 64)
  -retentionPeriod value
    	Data with timestamps outside the retentionPeriod is automatically deleted
    	The following optional suffixes are supported: h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 1)
  -rpc.disableCompression
    	Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage
  -search.maxTagKeys int
    	The maximum number of tag keys returned per search (default 100000)
  -search.maxTagValueSuffixesPerSearch int
    	The maximum number of tag value suffixes returned from /metrics/find (default 100000)
  -search.maxTagValues int
    	The maximum number of tag values returned per search (default 100000)
  -search.maxUniqueTimeseries int
    	The maximum number of unique time series a single query can process. This allows protecting against heavy queries, which select unexpectedly high number of series. See also -search.maxSamplesPerQuery and -search.maxSamplesPerSeries (default 300000)
  -smallMergeConcurrency int
    	The maximum number of CPU cores to use for small merges. Default value is used if set to 0
  -snapshotAuthKey string
    	authKey, which must be passed in query string to /snapshot* pages
  -storage.maxDailySeries int
    	The maximum number of unique series can be added to the storage during the last 24 hours. Excess series are logged and dropped. This can be useful for limiting series churn rate. See also -storage.maxHourlySeries
  -storage.maxHourlySeries int
    	The maximum number of unique series can be added to the storage during the last hour. Excess series are logged and dropped. This can be useful for limiting series cardinality. See also -storage.maxDailySeries
  -storage.minFreeDiskSpaceBytes size
    	The minimum free disk space at -storageDataPath after which the storage stops accepting new data
    	Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 10000000)
  -storageDataPath string
    	Path to storage data (default "vmstorage-data")
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
  -vminsertAddr string
    	TCP address to accept connections from vminsert services (default ":8400")
  -vmselectAddr string
    	TCP address to accept connections from vmselect services (default ":8401")
```


## VictoriaMetrics Logo

[Zip](VM_logo.zip) contains three folders with different image orientation (main color and inverted version).

Files included in each folder:

* 2 JPEG Preview files
* 2 PNG Preview files with transparent background
* 2 EPS Adobe Illustrator EPS10 files


### Logo Usage Guidelines

#### Font used:

* Lato Black
* Lato Regular

#### Color Palette:

* HEX [#110f0f](https://www.color-hex.com/color/110f0f)
* HEX [#ffffff](https://www.color-hex.com/color/ffffff)

### We kindly ask:

- Please don't use any other font instead of suggested.
- There should be sufficient clear space around the logo.
- Do not change spacing, alignment, or relative locations of the design elements.
- Do not change the proportions of any of the design elements or the design itself. You    may resize as needed but must retain all proportions.
