# Scenario

If you require high-level fault tolerance (e.g., across availability zones or regions) in VictoriaMetrics, we recommend the following cluster topologies based on your performance and availability needs::
- Multi-AZ cluster
- The Hyperscale (Cell-based)

For the detailed comparison of the trade-offs between these topologies, please refer to [VictoriaMetrics topologies](https://docs.victoriametrics.com/guides/vm-architectures/). This document will focus on how to correctly set them up.
# Multi-AZ cluster
See [Multi-Cluster and Multi-AZ topology](https://docs.victoriametrics.com/guides/vm-architectures/) for the detailed analysis of its trade-offs.

This topology involves deploying the self-contained VictoriaMetrics cluster in each Availability Zone (AZ). Read and write traffic will then be distributed across these AZ clusters. The topology supports both Active-Active and Active-Passive approach, the schema will be the same.

The setup process will be outlined in three stages:
- Set up a self-contained cluster in a single AZ.
- Expand the setup to multi AZ.
- Set up vmagent and vmauth.

## Self-contained cluster in single AZ

A minimal VictoriaMetrics cluster in the AZ must contain the following nodes:
- a single `vmstorage` node
- a single `vminsert` node with `-storageNode=<vmstorage-host>`
- a single `vmselect` node with `-storageNode=<vmstorage-host>`

For high availability purposes, it is recommended to run at least two nodes for each service.

### vmstorage
Execute the following command to start vmstorage process. (please refer to [List of command-line flags for vmstorage](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vmstorage) for more command-line flags)
```sh
/path/to/vmstorage-prod
```
You can see those in the output:
```text
lib/vminsertapi/server.go:60        started TCP vminsert server at "0.0.0.0:8400"
lib/vminsertapi/server.go:83        accepting vminsert conns at 0.0.0.0:8400
lib/vmselectapi/server.go:156        accepting vmselect conns at 0.0.0.0:8401
lib/httpserver/httpserver.go:145        started server at http://0.0.0.0:8482/
```
The default ports `:8400` and `:8401` are designated for data ingestion (from `vminsert`) and query (from `vmselect`), respectively.
You can start several `vmstorage` processes on different hosts to obtain `<az1-vmstorage-host[1-N]>`.

### vmselect
Execute the following command to start `vmselect` process. (please refer to [List of command-line flags for vmselect](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vmselect) for more command-line flags).

```sh
/path/to/vmselect-prod -storageNode=<az1-vmstorage-host1>:8401
```
If you have started several `vmstorage` instances before, you can separate them with commas in `-storageNode`:
```sh
/path/to/vmselect-prod \
    -storageNode=<az1-vmstorage-host1>:8401,<az1-vmstorage-host2>:8401
```
You can start several `vmselect` processes on different hosts to obtain `<az1-vmselect-host[1-N]>`.

### vminsert

Execute the following command to start `vminsert` process. (please refer to [List of command-line flags for vminsert](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vminsert) for more command-line flags)
```sh
/path/to/vminsert-prod -storageNode=<az1-vmstorage-host1>:8400
```
In the multi-AZ cluster topology, we don’t need to set `-replicationFactor` for `vminsert`,  data shoulD be replicated between these AZ clusters.

If you have started several `vmstorage` instances, you can separate them with commas in `-storageNode`:
```sh
/path/to/vminsert-prod \
    -storageNode=<az1-vmstorage-host1>:8400,<az1-vmstorage-host2>:8400
```
You can start several such `vminset` processes on different hosts to obtain `<az1-vminsert-host[1-N]>`.

At this stage, a self-contained VictoriaMetrics cluster has been successfully deployed within a single AZ.

## Expand the setup to multi AZ
You can repeat the above process to setup another self-contained clusters in one or more AZs. These AZ clusters can then be configured to achieve the desired fault tolerance:
- Primary AZ Failure (Active-Passive)
- Single AZ/cluster Failure (Active-Active)

Regardless of which approach you choose, the schema will be the same. The key difference lies in how the traffic is routed to these AZ clusters.

## vmagent
We recommend deploying the `vmagent` service, as it enhances fault tolerance by temporarily buffering data on disk in the event of a `vminsert` node failure. See [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/#).

Execute the following command to start `vmagent` process. (please refer to [List of command-line flags for vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/#advanced-usage) for more command-line flags)
```SH
/path/to/vmagent \
    -remoteWrite.url="http://<az1-vminsert-host1>:8480/insert/0/prometheus" \
    -remoteWrite.url="http://<az1-vminsert-host2>:8480/insert/0/prometheus"
```
`vmagent` runs on port `:8429` by default. You can deploy `vmagent` in other AZ clusters.

## vmauth
We also recommend deploying the `vmauth` service. It can work as a gateway, distributing requests to multiple `vminsert` and `vmselect` instances behind a single endpoint. Key capabilities include load balancing and failover. See [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/).

To set it up, we should prepare `config.yaml`. (Suppose we previously deployed two `vminsert` and `vmselect` instances within the AZ)
```yaml
unauthorized_user:
url_map:
- src_paths:
  - "/select/.*"
  url_prefix:
  - "http://<az1-vmselect-host1>:8481/"
  - "http://<az1-vmselect-host2>:8481/"
- src_paths:
  - "/insert/.*"
  url_prefix:
  - "http://<az1-vminsert-host1>:8480/"
  - "http://<az1-vminsert-host2>:8480/"
```
Execute the following command to start `vmauth` process. (please refer to [List of command-line flags for vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/#advanced-usage) for more command-line flags)
```sh
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```

`vmauth` runs on `:8427` by default. Similarly, you can deploy `vmauth` in other AZs.

# The Hyperscale (Cell-based)

The Hyperscale architecture is designed for systems that demand higher levels of reliability and scalability across multiple regions and zones. For a detailed analysis of its trade-offs, please refer to [The Hyperscale (Cell-based) topology](https://docs.victoriametrics.com/guides/vm-architectures/#the-hyperscale-cell-based).

This architecture incorporates self-contained clusters deployed across multiple AZs, similar to the Multi-AZ setup. Therefore, the procedure for setting up individual AZ cluster can be followed as described in the Multi-AZ setup guide.

The key distinction of the Hyperscale topology lies in its `global route layer`, which sits above the multi-AZ clusters. This layer can be configured to intelligently distribute read and write requests based on the desired balance between read speed and data completeness. We can deploy the global route layer for each Region.

## Global Route Layer
### PathA：Prioritize Data Completeness
#### Write Path: setup vmagent to shard data

In this path, `vmagent` will be configured to shard data across multi-AZ clusters in the region. To achieve AZ-level fault tolerance, use the `-remoteWrite.shardByURLReplicas` flag to enable data replication between multi-AZ clusters, for example (Suppose we have 4 AZ clusters):

```sh
/path/to/vmagent \
    -remoteWrite.url="http://<az1-vmauth-host1>:8480/insert/0/prometheus" \
    -remoteWrite.url="http://<az2-vmauth-host1>:8480/insert/0/prometheus" \
    -remoteWrite.url="http://<az3-vmauth-host1>:8480/insert/0/prometheus" \
    -remoteWrite.url="http://<az4-vmauth-host1>:8480/insert/0/prometheus" \
    -remoteWrite.shardByURLReplicas = 3
```
The data will be replicated to three of these AZ clusters, as long as two AZs remain available, the complete data can be returned.

#### Read Path: setup global vmselect

The global `vmselect` is a core component of the multi-level cluster architecture. For a comprehensive overview, please refer to [Multi level cluster setup](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multi-level-cluster-setup).

In this architecture, each second-level `vmselect` instance in AZ must be configured with `-clusternativeListenAddr` flag to expose its cluster-native API:

```sh
/path/to/vmselect-prod -storageNodes=<az1-vmstorage-host1>:8400,<az1-vmstorage-host2>:8400  \
    -clusternativeListenAddr=":8401"
```

This enables the second-level `vmselect` to accept queries from the `global vmselect` on TCP port `:8401` in the same way as `vmstorage` nodes do.

Then we can start the `global vmselect` process with `storageGroup` setting. Execute the following command to start `global vmselect` process.

```sh
/path/to/vmselect \
    -globalReplicationFactor=3 \
    -storageNode="az1/<az1-vmselect-host1>:8401,az1/<az1-vmselect-host2>:8401" \
    -storageNode="az2/<az2-vmselect-host1>:8401,az1/<az2-vmselect-host2>:8401" \
    -storageNode="az3/<az3-vmselect-host1>:8401,az1/<az3-vmselect-host2>:8401" \
    -storageNode="az4/<az4-vmselect-host1>:8401,az1/<az4-vmselect-host2>:8401"
```

Please notice `-globalReplicationFactor=3` should align with `-remoteWrite.shardByURLReplicas=3` in `vmagent`.

See more detail about `storageGroup` in [vmstorage groups at vmselect](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#vmstorage-groups-at-vmselect).

### PathB：Focus on Read Speed
#### Write path: setup `vmagent` to replicate data

In this path, `vmagent` should be configured to replicate data to every AZ cluster. Crucially, you should not specify the `-remoteWrite.shardByURLReplicas` or `-remoteWrite.shardByURL`, as the objective is to replicate data to all AZ clusters, not to shard them.

Execute the following command to start `vmagent` process.

```sh
/path/to/vmagent \
    -remoteWrite.url="http://<az1-vmauth-host1>:8427/insert/0/prometheus" \
    -remoteWrite.url="http://<az2-vmauth-host1>:8427/insert/0/prometheus" \
    -remoteWrite.url="http://<az3-vmauth-host1>:8427/insert/0/prometheus" \
    -remoteWrite.url="http://<az4-vmauth-host1>:8427/insert/0/prometheus"
```

You can start several such `vmagent` process.

#### Read path: setup global vmauth with first_available
Because `vmagent` will replicate data to all AZ clusters, and each cluster should contain a full copy of all data, so we can use `vmatuh` with `first_available` mode to read data from the first available AZ cluster.

prepare `config.yaml`:

```yaml
unauthorized_user:
  url_prefix:
    - "http://<az1-vmauth-host1>:8481/"
    - "http://<az2-vmauth-host1>:8481/"
    - "http://<az3-vmauth-host1>:8481/"
    - "http://<az4-vmauth-host1>:8481/"
  load_balancing_policy: first_available
```
Execute the following command to start global `vmauth` process.

```sh
/path/to/vmauth -auth.config=/path/to/auth/config.yml
```