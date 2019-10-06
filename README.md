<img alt="Victoria Metrics" src="logo.png">

# Cluster version of VictoriaMetrics

VictoriaMetrics is fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.

It is recommended using [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics) instead of cluster version
for ingestion rates lower than 10 million of data points per second.
Single-node version [scales perfectly](https://medium.com/@valyala/measuring-vertical-scalability-for-time-series-databases-in-google-cloud-92550d78d8ae)
with the number of CPU cores, RAM and available storage space.
Single-node version is easier to configure and operate comparing to cluster version, so think twice before sticking to cluster version.


## Prominent features

- Supports all the features of [single-node version](https://github.com/VictoriaMetrics/VictoriaMetrics).
- Scales horizontally to multiple nodes.
- Supports multiple independent namespaces for time series data (aka multi-tenancy).


## Architecture overview

VictoriaMetrics cluster consists of the following services:

- `vmstorage` - stores the data
- `vminsert` - proxies the ingested data to `vmstorage` shards using consistent hashing
- `vmselect` - performs incoming queries using the data from `vmstorage`

Each service may scale independently and may run on the most suitable hardware.

<img src="https://docs.google.com/drawings/d/e/2PACX-1vTvk2raU9kFgZ84oF-OKolrGwHaePhHRsZEcfQ1I_EC5AB_XPWwB392XshxPramLJ8E4bqptTnFn5LL/pub?w=1104&amp;h=746">


## Binaries

Compiled binaries for cluster version are available in the `assets` section of [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
See archives containing `cluster` word.

Docker images for cluster version are available here:

- `vminsert` - https://hub.docker.com/r/victoriametrics/vminsert/tags
- `vmselect` - https://hub.docker.com/r/victoriametrics/vmselect/tags
- `vmstorage` - https://hub.docker.com/r/victoriametrics/vmstorage/tags


## Building from sources

Source code for cluster version is available at [cluster branch](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).


### Development Builds

1. [Install go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make` from the repository root. It should build `vmstorage`, `vmselect`
   and `vminsert` binaries and put them into the `bin` folder.


### Production builds

There is no need in installing Go on a host system since binaries are built
inside [the official docker container for Go](https://hub.docker.com/_/golang).
This makes reproducible builds.
So [install docker](https://docs.docker.com/install/) and run the following command:

```
make vminsert-prod vmselect-prod vmstorage-prod
```

Production binaries are built into statically linked binaries for `GOARCH=amd64`, `GOOS=linux`.
They are put into `bin` folder with `-prod` suffixes:
```
$ make vminsert-prod vmselect-prod vmstorage-prod
$ ls -1 bin
vminsert-prod
vmselect-prod
vmstorage-prod
```

### Building docker images

Run `make package`. It will build the following docker images locally:

* `victoriametrics/vminsert:<PKG_TAG>`
* `victoriametrics/vmselect:<PKG_TAG>`
* `victoriametrics/vmstorage:<PKG_TAG>`

`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package`.



## Operation

### Cluster setup

A minimal cluster must contain the following nodes:

* a single `vmstorage` node with `-retentionPeriod` and `-storageDataPath` flags
* a single `vminsert` node with `-storageNode=<vmstorage_host>:8400`
* a single `vmselect` node with `-storageNode=<vmstorage_host>:8401`

It is recommended to run at least two nodes for each service
for high availability purposes.

An http load balancer must be put in front of `vminsert` and `vmselect` nodes:
- requests starting with `/insert` must be routed to port `8480` on `vminsert` nodes.
- requests starting with `/select` must be routed to port `8481` on `vmselect` nodes.

Ports may be altered by setting `-httpListenAddr` on the corresponding nodes.


### URL format

* URLs for data ingestion: `/insert/<accountID>/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying namespace for data ingestion (aka tenant)
  - `<suffix>` may have the following values:
     - `prometheus` - for inserting data with [Prometheus remote write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
     - `influx/write` or `influx/api/v2/write` - for inserting data with [Influx line protocol](https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/)

* URLs for querying: `/select/<accountID>/prometheus/<suffix>`, where:
  - `<accountID>` is an arbitrary number identifying data namespace for the query (aka tenant)
  - `<suffix>` may have the following values:
    - `api/v1/query` - performs [PromQL instant query](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries)
    - `api/v1/query_range` - performs [PromQL range query](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries)
    - `api/v1/series` - performs [series query](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers)
    - `api/v1/labels` - returns a [list of label names](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names)
    - `api/v1/label/<label_name>/values` - returns values for the given `<label_name>` according [to API](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values)
    - `federate` - returns [federated metrics](https://prometheus.io/docs/prometheus/latest/federation/)
    - `api/v1/export` - exports raw data. See [this article](https://medium.com/@valyala/analyzing-prometheus-data-with-external-tools-5f3e5e147639) for details

* `vmstorage` nodes provide the following HTTP endpoints on `8482` port:
  - `/snapshot/create` - create [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282),
    which can be used for backups in background. Snapshots are created in `<storageDataPath>/snapshots` folder, where `<storageDataPath>` is the corresponding
    command-line flag value.
  - `/snapshot/list` - list available snasphots.
  - `/snapshot/delete?snapshot=<id>` - delete the given snapshot.
  - `/snapshot/delete_all` - delete all the snapshots.

  Snapshots may be created independently on each `vmstorage` node. There is no need in synchronizing snapshots' creation
  across `vmstorage` nodes.


### Cluster resizing

* `vminsert` and `vmselect` nodes are stateless and may be added / removed at any time.
  Do not forget updating the list of these nodes on http load balancer.
* `vmstorage` nodes own the ingested data, so they cannot be removed without data loss.

Steps to add `vmstorage` node:

1. Start new `vmstorage` node with the same `-retentionPeriod` as existing nodes in the cluster.
2. Gradually restart all the `vmselect` nodes with new `-storageNode` arg containing `<new_vmstorage_host>:8401`.
3. Gradually restart all the `vminsert` nodes with new `-storageNode` arg containing `<new_vmstorage_host>:8400`.


### Cluster availability

* HTTP load balancer must stop routing requests to unavailable `vminsert` and `vmselect` nodes.
* The cluster remains available if at least a single `vmstorage` node exists:

  - `vminsert` re-routes incoming data from unavailable `vmstorage` nodes to healthy `vmstorage` nodes
  - `vmselect` continues serving partial responses if at least a single `vmstorage` node is available.


### Updating / reconfiguring cluster nodes

All the node types - `vminsert`, `vmselect` and `vmstorage` - may be updated via graceful shutdown.
Send `SIGINT` signal to the corresponding process, wait until it finishes and then start new version
with new configs.

Cluster should remain in working state if at least a single node of each type remains available during
the update process. See [cluster availability](#cluster-availability) section for details.


### Helm

Helm chart simplifies managing cluster version of VictoriaMetrics in Kubernetes.
It is available in the `deployment/k8s/helm/victoria-metrics` folder.

1. Install Cluster: `helm install -n <NAME> deployment/k8s/helm/victoria-metrics` or `ENV=<NAME> make helm-install`.
2. Upgrade Cluster: `helm upgrade <NAME> deployment/k8s/helm/victoria-metrics` or `ENV=<NAME> make helm-upgrade`.
3. Delete Cluster: `helm del --purge <NAME>` or `ENV=<NAME> make helm-delete`.

Upgrade follows `Cluster resizing procedure` under the hood.


### Replication and data safety

VictoriaMetrics offloads replication to the underlying storage pointed by `-storageDataPath`.
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
2. Archive the created snapshot from `<-storageDataPath>/snapshots/<snapshot_name>` folder using any suitable tool that follows symlinks. For instance,
   `cp -L`, `rsync -L` or `scp -r`. The archival process doesn't interfere with `vmstorage` work, so it may be performed at any suitable time.
   Incremental backups are possible with `rsync --delete`, which should [remove extraneous files from backup dir](https://askubuntu.com/questions/476041/how-do-i-make-rsync-delete-files-that-have-been-deleted-from-the-source-folder).
3. Delete unused snapshots via `/snapshot/delete?snapshot=<snapshot_name>` or `/snapshot/delete_all` in order to free up occupied storage space.

There is no need in synchronizing backups among all the `vmstorage` nodes.

Restoring from backup:

1. Stop `vmstorage` node with `kill -INT`.
2. Delete all the contents of the directory pointed by `-storageDataPath` command-line flag.
3. Copy all the contents of the backup directory to `-storageDataPath` directory.
4. Start `vmstorage` node.


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
- Complex replication schemes, which may go nuts in unforesseen edge cases. The replication is offloaded to the underlying durable replicated storage
  such as [persistent disks in Google Compute Engine](https://cloud.google.com/compute/docs/disks/#pdspecs).
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
