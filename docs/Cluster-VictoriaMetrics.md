# Cluster version

<img alt="Victoria Metrics" src="logo.png">

VictoriaMetrics is fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.

It is recommended using [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics) instead of cluster version
for ingestion rates lower than a million of data points per second.
Single-node version [scales perfectly](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
with the number of CPU cores, RAM and available storage space.
Single-node version is easier to configure and operate comparing to cluster version, so think twice before sticking to cluster version.

Join [our Slack](http://slack.victoriametrics.com/) or [contact us](mailto:info@victoriametrics.com) with consulting and support questions.


## Prominent features

- Supports all the features of [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics).
- Performance and capacity scales horizontally. See [these docs for details](#cluster-resizing-and-scalability).
- Supports multiple independent namespaces for time series data (aka multi-tenancy). See [these docs for details](#multitenancy).
- Supports replication. See [these docs for details](#replication-and-data-safety).


## Architecture overview

VictoriaMetrics cluster consists of the following services:

- `vmstorage` - stores the data
- `vminsert` - proxies the ingested data to `vmstorage` shards using consistent hashing
- `vmselect` - performs incoming queries using the data from `vmstorage`

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
by a separate service sitting in front of VictoriaMetrics cluster such as [vmauth](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmauth/README.md).
[Contact us](mailto:info@victoriametrics.com) if you need help with creating such a service.

* Tenants are automatically created when the first data point is written into the given tenant.

* Data for all the tenants is evenly spread among available `vmstorage` nodes. This guarantees even load among `vmstorage` nodes
when different tenants have different amounts of data and different query load.

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

1. [Install go](https://golang.org/doc/install). The minimum supported version is Go 1.13.
2. Run `make` from the repository root. It should build `vmstorage`, `vmselect`
   and `vminsert` binaries and put them into the `bin` folder.


### Building docker images

Run `make package`. It will build the following docker images locally:

* `victoriametrics/vminsert:<PKG_TAG>`
* `victoriametrics/vmselect:<PKG_TAG>`
* `victoriametrics/vmstorage:<PKG_TAG>`

`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package`.

By default images are built on top of [alpine](https://hub.docker.com/_/scratch) image in order to improve debuggability.
It is possible to build an image on top of any other base image by setting it via `<ROOT_IMAGE>` environment variable.
For example, the following command builds images on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package
```

## Operation

### Cluster setup

A minimal cluster must contain the following nodes:

* a single `vmstorage` node with `-retentionPeriod` and `-storageDataPath` flags
* a single `vminsert` node with `-storageNode=<vmstorage_host>:8400`
* a single `vmselect` node with `-storageNode=<vmstorage_host>:8401`

It is recommended to run at least two nodes for each service
for high availability purposes.

An http load balancer such as `nginx` must be put in front of `vminsert` and `vmselect` nodes:
- requests starting with `/insert` must be routed to port `8480` on `vminsert` nodes.
- requests starting with `/select` must be routed to port `8481` on `vmselect` nodes.

Ports may be altered by setting `-httpListenAddr` on the corresponding nodes.

It is recommended setting up [monitoring](#monitoring) for the cluster.

#### Environment variables

Each flag values can be set thru environment variables by following these rules:

- The `-envflag.enable` flag must be set
- Each `.` in flag names must be substituted by `_` (for example `-insert.maxQueueDuration <duration>` will translate to `insert_maxQueueDuration=<duration>`)
- For repeating flags, an alternative syntax can be used by joining the different values into one using `,` as separator (for example `-storageNode <nodeA> -storageNode <nodeB>` will translate to `storageNode=<nodeA>,<nodeB>`)
- It is possible setting prefix for environment vars with `-envflag.prefix`. For instance, if `-envflag.prefix=VM_`, then env vars must be prepended with `VM_`


### Monitoring

All the cluster components expose various metrics in Prometheus-compatible format at `/metrics` page on the TCP port set in `-httpListenAddr` command-line flag.
By default the following TCP ports are used:
- `vminsert` - 8480
- `vmselect` - 8481
- `vmstorage` - 8482

It is recommended setting up [vmagent](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmagent/README.md)
or Prometheus to scrape `/metrics` pages from all the cluster components, so they can be monitored and analyzed
with [the official Grafana dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11176)
or [an alternative dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11831).


### URL format

* URLs for data ingestion: `http://<vminsert>:8480/insert/<accountID>/<suffix>`, where:
  - `<accountID>` is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). It is possible to set it as `accountID:projectID`,
    where `projectID` is also arbitrary 32-bit integer. If `projectID` isn't set, then it equals to `0`.
  - `<suffix>` may have the following values:
     - `prometheus` and `prometheus/api/v1/write` - for inserting data with [Prometheus remote write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
     - `influx/write` and `influx/api/v2/write` - for inserting data with [Influx line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/).
     - `opentsdb/api/put` - for accepting [OpenTSDB HTTP /api/put requests](http://opentsdb.net/docs/build/html/api_http/put.html).
       This handler is disabled by default. It is exposed on a distinct TCP address set via `-opentsdbHTTPListenAddr` command-line flag.
       See [these docs](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#sending-opentsdb-data-via-http-apiput-requests) for details.
     - `prometheus/api/v1/import` - for importing data obtained via `api/v1/export` on `vmselect` (see below).
     - `prometheus/api/v1/import/csv` - for importing arbitrary CSV data. See [these docs](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-import-csv-data) for details.

* URLs for querying: `http://<vmselect>:8481/select/<accountID>/prometheus/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying data namespace for the query (aka tenant)
  - `<suffix>` may have the following values:
    - `api/v1/query` - performs [PromQL instant query](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries).
    - `api/v1/query_range` - performs [PromQL range query](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).
    - `api/v1/series` - performs [series query](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers).
    - `api/v1/labels` - returns a [list of label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names).
    - `api/v1/label/<label_name>/values` - returns values for the given `<label_name>` according [to API](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values).
    - `federate` - returns [federated metrics](https://prometheus.io/docs/prometheus/latest/federation/).
    - `api/v1/export` - exports raw data. See [this article](https://medium.com/@valyala/analyzing-prometheus-data-with-external-tools-5f3e5e147639) for details.
    - `api/v1/status/tsdb` - for time series stats. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats) for details.

* URL for time series deletion: `http://<vmselect>:8481/delete/<accountID>/prometheus/api/v1/admin/tsdb/delete_series?match[]=<timeseries_selector_for_delete>`.
  Note that the `delete_series` handler should be used only in exceptional cases such as deletion of accidentally ingested incorrect time series. It shouldn't
  be used on a regular basis, since it carries non-zero overhead.

* `vmstorage` nodes provide the following HTTP endpoints on `8482` port:
  - `/snapshot/create` - create [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282),
    which can be used for backups in background. Snapshots are created in `<storageDataPath>/snapshots` folder, where `<storageDataPath>` is the corresponding
    command-line flag value.
  - `/snapshot/list` - list available snasphots.
  - `/snapshot/delete?snapshot=<id>` - delete the given snapshot.
  - `/snapshot/delete_all` - delete all the snapshots.

  Snapshots may be created independently on each `vmstorage` node. There is no need in synchronizing snapshots' creation
  across `vmstorage` nodes.


### Cluster resizing and scalability

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
2. Gradually restart all the `vmselect` nodes with new `-storageNode` arg containing `<new_vmstorage_host>:8401`.
3. Gradually restart all the `vminsert` nodes with new `-storageNode` arg containing `<new_vmstorage_host>:8400`.


### Updating / reconfiguring cluster nodes

All the node types - `vminsert`, `vmselect` and `vmstorage` - may be updated via graceful shutdown.
Send `SIGINT` signal to the corresponding process, wait until it finishes and then start new version
with new configs.

Cluster should remain in working state if at least a single node of each type remains available during
the update process. See [cluster availability](#cluster-availability) section for details.


### Cluster availability

* HTTP load balancer must stop routing requests to unavailable `vminsert` and `vmselect` nodes.
* The cluster remains available if at least a single `vmstorage` node exists:

  - `vminsert` re-routes incoming data from unavailable `vmstorage` nodes to healthy `vmstorage` nodes
  - `vmselect` continues serving partial responses if at least a single `vmstorage` node is available.

Data replication can be used for increasing storage durability. See [these docs](#replication-and-data-safety) for details.


### Capacity planning

Each instance type - `vminsert`, `vmselect` and `vmstorage` - can run on the most suitable hardware.

#### vminsert

* The recommended total number of vCPU cores for all the `vminsert` instances can be calculated from the ingestion rate: `vCPUs = ingestion_rate / 150K`.
* The recommended number of vCPU cores per each `vminsert` instance should equal to the number of `vmstorage` instances in the cluster.
* The amount of RAM per each `vminsert` instance should be 1GB or more. RAM is used as a buffer for spikes in ingestion rate.
  The maximum amount of used RAM per `vminsert` node can be tuned with `-memory.allowedPercent` command-line flag. For instance, `-memory.allowedPercent=20`
  limits the maximum amount of used RAM to 20% of the available RAM on the host system.
* Sometimes `-rpc.disableCompression` command-line flag on `vminsert` instances could increase ingestion capacity at the cost
  of higher network bandwidth usage between `vminsert` and `vmstorage`.

#### vmstorage

* The recommended total number of vCPU cores for all the `vmstorage` instances can be calculated from the ingestion rate: `vCPUs = ingestion_rate / 150K`.
* The recommended total amount of RAM for all the `vmstorage` instances can be calculated from the number of active time series: `RAM = active_time_series * 1KB`.
  Time series is active if it received at least a single data point during the last hour or if it has been queried during the last hour.
* The recommended total amount of storage space for all the `vmstorage` instances can be calculated
  from the ingestion rate and retention: `storage_space = ingestion_rate * retention_seconds`.

#### vmselect

The recommended hardware for `vmselect` instances highly depends on the type of queries. Lightweight queries over small number of time series usually require
small number of vCPU cores and small amount of RAM on `vmselect`, while heavy queries over big number of time series (>10K) usually require
bigger number of vCPU cores and bigger amounts of RAM.

In general it is recommended increasing the number of vCPU cores and RAM per `vmselect` node for higher query performance,
while adding new `vmselect` nodes only when old nodes are overloaded with incoming query stream.


### High availability

It is recommended to run all the components for a single cluster in the same subnetwork with high bandwidth, low latency and low error rates.
This improves cluster performance and availability.
It isn't recommended spreading components for a single cluster across multiple availability zones, since cross-AZ network usually has lower bandwidth, higher latency
and higher error rates comparing the network inside AZ.

If you need multi-AZ setup, then it is recommended running independed clusters in each AZ and setting up
[vmagent](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmagent/README.md) in front of these clusters, so it could replicate incoming data
into all the cluster. Then [promxy](https://github.com/jacksontj/promxy) could be used for querying the data from multiple clusters.


### Helm

Helm chart simplifies managing cluster version of VictoriaMetrics in Kubernetes.
It is available in the [helm-charts](https://github.com/VictoriaMetrics/helm-charts) repository.

Upgrade follows `Cluster resizing procedure` under the hood.


### Replication and data safety

In order to enable application-level replication, `-replicationFactor=N` command-line flag must be passed to `vminsert`.
This guarantees that all the data remains available for querying if up to `N-1` `vmstorage` nodes are unavailable.
For example, when `-replicationFactor=3` is passed to `vminsert`, then it replicates all the ingested data to 3 distinct `vmstorage` nodes.

When the replication is enabled, `-dedup.minScrapeInterval=1ms` command-line flag must be passed to `vmselect`
in order to de-duplicate replicated data during queries. It is OK if `-dedup.minScrapeInterval` exceeds 1ms
when [deduplication](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#deduplication) is used additionally to replication.

Note that [replication doesn't save from disaster](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883),
so it is recommended performing regular backups. See [these docs](#backups) for details.

By default VictoriaMetrics offloads replication to the underlying storage pointed by `-storageDataPath`.
It is recommended storing data on [Google Compute Engine persistent disks](https://cloud.google.com/compute/docs/disks/#pdspecs),
since they are protected from data loss and data corruption. They also provide consistently high performance
and [may be resized](https://cloud.google.com/compute/docs/disks/add-persistent-disk) without downtime.
HDD-based persistent disks should be enough for the majority of use cases.

It is recommended using durable replicated persistent volumes in Kubernetes.


### Backups

It is recommended performing periodical backups from [instant snapshots](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282)
for protecting from user errors such as accidental data deletion.

The following steps must be performed for each `vmstorage` node for creating a backup:

1. Create an instant snapshot by navigating to `/snapshot/create` HTTP handler. It will create snapshot and return its name.
2. Archive the created snapshot from `<-storageDataPath>/snapshots/<snapshot_name>` folder using [vmbackup](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/app/vmbackup/README.md).
   The archival process doesn't interfere with `vmstorage` work, so it may be performed at any suitable time.
3. Delete unused snapshots via `/snapshot/delete?snapshot=<snapshot_name>` or `/snapshot/delete_all` in order to free up occupied storage space.

There is no need in synchronizing backups among all the `vmstorage` nodes.

Restoring from backup:

1. Stop `vmstorage` node with `kill -INT`.
2. Restore data from backup using [vmrestore](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/app/vmrestore/README.md) into `-storageDataPath` directory.
3. Start `vmstorage` node.


## Community and contributions

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle):

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency or performance.

Adhering `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and verified by many people.

Due to `KISS` cluster version of VictoriaMetrics has no the following "features" popular in distributed computing world:

- Fragile gossip protocols. See [failed attempt in Thanos](https://github.com/improbable-eng/thanos/blob/030bc345c12c446962225221795f4973848caab5/docs/proposals/completed/201809_gossip-removal.md).
- Hard-to-understand-and-implement-properly [Paxos protocols](https://www.quora.com/In-distributed-systems-what-is-a-simple-explanation-of-the-Paxos-algorithm).
- Complex replication schemes, which may go nuts in unforesseen edge cases. See [replication docs](#replication-and-data-safety) for details.
- Automatic data reshuffling between storage nodes, which may hurt cluster performance and availability.
- Automatic cluster resizing, which may cost you a lot of money if improperly configured.
- Automatic discovering and addition of new nodes in the cluster, which may mix data between dev and prod clusters :)
- Automatic leader election, which may result in split brain disaster on network errors.


## Reporting bugs

Report bugs and propose new features [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).


## Victoria Metrics Logo

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
