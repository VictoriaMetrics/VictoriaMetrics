---
weight: 20
title: VictoriaLogs cluster
menu:
  docs:
    parent: victorialogs
    identifier: vl-cluster
    weight: 20
    title: VictoriaLogs cluster
aliases:
- /victorialogs/cluster/
---

Cluster mode in VictoriaLogs provides horizontal scaling to many nodes when [single-node VictoriaLogs](https://docs.victoriametrics.com/victorialogs/)
reaches vertical scalability limits of a single host. If you have an ability to run a single-node VictoriaLogs on a host with more CPU / RAM / storage space / storage IO,
then it is preferred to do this instead of switching to cluster mode, since a single-node VictoriaLogs instance has the following advantages over cluster mode:

- It is easier to configure, manage and troubleshoot, since it consists of a single self-contained component.
- It provides better performance and capacity on the same hardware, since it doesn't need
  to transfer data over the network between cluster components.

The migration path from a single-node VictoriaLogs to cluster mode is very easy - just [upgrade](https://docs.victoriametrics.com/victorialogs/#upgrading)
a single-node VictoriaLogs executable to the [latest available release](https://docs.victoriametrics.com/victorialogs/changelog/) and add it to the list of `vlstorage` nodes
passed via `-storageNode` command-line flag to `vlinsert` and `vlselect` components of the cluster mode. See [cluster architecture](#architecture)
for more details about VictoriaLogs cluster components.

See [quick start guide](#quick-start) on how to start working with VictoriaLogs cluster.

## Architecture

VictoriaLogs in cluster mode consists of `vlinsert`, `vlselect` and `vlstorage` components:

- `vlinsert` accepts the ingested logs via [all the supported data ingestion protocols](https://docs.victoriametrics.com/victorialogs/data-ingestion/)
  and spreads them among the `vlstorage` nodes listed via the `-storageNode` command-line flag.

- `vlselect` accepts incoming queries via [all the supported HTTP querying endpoints](https://docs.victoriametrics.com/victorialogs/querying/),
  requests the needed data from `vlstorage` nodes listed via the `-storageNode` command-line flag, processes the queries and returns the corresponding responses.

- `vlstorage` is responsible for two tasks:

  - It accepts logs from `vlinsert` nodes and stores them at the directory specified via `-storageDataPath` command-line flag.
    See [these docs](https://docs.victoriametrics.com/victorialogs/#storage) for details about this flag.

  - It processes requests from `vlselect` nodes. It selects the requested logs and performs all data transformations and calculations,
    which can be executed locally, before sending the results to `vlselect`.

`vlstorage` is basically a single-node version of VictoriaLogs. See [these docs](#single-node-and-cluster-mode-duality) for details.
This means that the existing single-node VictoriaLogs instances can be added to the list of `vlstorage` nodes via `-storageNode` command-line flag at `vlselect`
in order to get global querying view over all the logs across all the single-node VictoriaLogs instances.

Every component of the VictoriaLogs cluster can scale from a single node to arbitrary number of nodes and can run on the most suitable hardware for the given workload.
`vlinsert` nodes can be used as `vlselect` nodes, so the minimum VictoriaLogs cluster must contain a `vlstorage` node plus a node, which plays both `vlinsert` and `vlselect` roles.
It isn't recommended sharing `vlinsert` and `vlselect` responsibilities in a single node, since this increases chances that heavy queries can negatively affect data ingestion
and vice versa.

`vlselect` and `vlinsert` communicate with `vlstorage` via HTTP at the TCP port specified via `-httpListenAddr` command-line flag:

- `vlinsert` sends requests to `/internal/insert` HTTP endpoint at `vlstorage`.
- `vlselect` sends requests to HTTP endpoints at `vlstorage` starting with `/internal/select/`.

This allows using various http proxies for authorization, routing and encryption of requests between these components.
It is recommended to use [vmauth](https://docs.victoriametrics.com/vmauth/).

See also [multi-level cluster setup](#multi-level-cluster-setup).

## Single-node and cluster mode duality

Every `vlstorage` node can be used as a single-node VictoriaLogs instance:

- It can accept logs via [all the supported data ingestion protocols](https://docs.victoriametrics.com/victorialogs/data-ingestion/).
- It can accept `select` queries via [all the supported HTTP querying endpoints](https://docs.victoriametrics.com/victorialogs/querying/).

A single-node VictoriaLogs instance can be used as `vlstorage` node in VictoriaLogs cluster:

- It accepts data ingestion requests from `vlinsert` via `/internal/insert` HTTP endpoint at the TCP port specified via `-httpListenAddr` command-line flag.
- It accepts queries from `vlselect` via `/internal/select/*` HTTP endpoints at the TCP port specified via `-httpListenAddr` command-line flags.

It is possible to disallow access to `/internal/insert` and `/internal/select/*` endpoints at single-node VictoriaLogs instance
by running it with `-internalinsert.disable` and `-internalselect.disable` command-line flags.

## Multi-level cluster setup

- `vlinsert` can send the ingested logs to other `vlinsert` nodes if they are specified via `-storageNode` command-line flag.
  This allows building multi-level data ingestion schemes when top-level `vlinsert` spreads the incoming logs among multiple lower-level clusters of VictoriaLogs.

- `vlselect` can send queries to other `vlselect` nodes if they are specified via `-storageNode` command-line flag.
  This allows building multi-level cluster schemes when top-level `vlselect` queries multiple lower-level clusters of VictoriaLogs.

See [security docs](#security) on how to protect communications between multiple levels of `vlinsert` and `vlselect` nodes.

## Security

All the VictoriaLogs cluster components must run in protected internal network without direct access from the Internet.
`vlstorage` must have no access from the Internet. HTTP authorization proxies such as [vmauth](https://docs.victoriametrics.com/vmauth/)
must be used in front of `vlinsert` and `vlselect` for authorizing access to these components from the Internet.

By default `vlinsert` and `vlselect` communicate with `vlstorage` via unencrypted HTTP. This is OK if all these components are located
in the same protected internal network. This isn't OK if these components communicate over the Internet, since a third party can intercept / modify
the transferred data. It is recommended switching to HTTPS in this case:

- Specify `-tls`, `-tlsCertFile` and `-tlsKeyFile` command-line flags at `vlstorage`, so it accepts incoming requests over HTTPS instead of HTTP at the corresponding `-httpListenAddr`:

  ```sh
  ./victoria-logs-prod -httpListenAddr=... -storageDataPath=... -tls -tlsCertFile=/path/to/certfile -tlsKeyFile=/path/to/keyfile
  ```

- Specify `-storageNode.tls` command-line flag at `vlinsert` and `vlselect`, which communicate with the `vlstorage` over untrusted networks such as Internet:

  ```sh
  ./victoria-logs-prod -storageNode=... -storageNode.tls
  ```

It is also recommended authorizing HTTPS requests to `vlstorage` via Basic Auth:

- Specify `-httpAuth.username` and `-httpAuth.password` command-line flags at `vlstorage`, so it verifies the Basic Auth username + password in HTTPS requests received via `-httpListenAddr`:

  ```sh
  ./victoria-logs-prod -httpListenAddr=... -storageDataPath=... -tls -tlsCertFile=... -tlsKeyFile=... -httpAuth.username=... -httpAuth.password=...
  ```

- Specify `-storageNode.username` and `-storageNode.password` command-line flags at `vlinsert` and `vlselect`, which communicate with the `vlostorage` over untrusted networks:

  ```sh
  ./victoria-logs-prod -storageNode=... -storageNode.tls -storageNode.username=... -storageNode.password=...
  ```

Another option is to use third-party HTTP proxies such as [vmauth](https://docs.victoriametrics.com/vmauth/), `nginx`, etc. for authorizing and encrypting communications
between VictoriaLogs cluster components over untrusted networks.


## Quick start

The following guide covers the following topics for Linux host:

- How to download VictoriaLogs executable.
- How to start VictoriaLogs cluster, which consists of two `vlstorage` nodes, a single `vlinsert` node and a single `vlselect` node
  running on a localhost according to [cluster architecture](#architecture).
- How to ingest logs into the cluster.
- How to query the ingested logs.

Download and unpack the latest VictoriaLogs release:

```sh
curl -L -O https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.18.0-victorialogs/victoria-logs-linux-amd64-v1.18.0-victorialogs.tar.gz
tar xzf victoria-logs-linux-amd64-v1.18.0-victorialogs.tar.gz
```

Start the first [`vlstorage` node](#architecture), which accepts incoming requests at the port `9491` and stores the ingested logs at `victoria-logs-data-1` directory:

```sh
./victoria-logs-prod -httpListenAddr=:9491 -storageDataPath=victoria-logs-data-1 &
```

This command and all the following commands start cluster components as background processes.
Use `jobs`, `fg`, `bg` commands for manipulating the running background processes. Use `kill` command and/or `Ctrl+C` for stopping the running processes when they no longer needed.
See [these docs](https://tldp.org/LDP/abs/html/x9644.html) for details.

Start the second `vlstorage` node, which accepts incoming requests at the port `9492` and stores the ingested logs at `victoria-logs-data-2` directory:

```sh
./victoria-logs-prod -httpListenAddr=:9492 -storageDataPath=victoria-logs-data-2 &
```

Start `vlinsert` node, which [accepts logs](https://docs.victoriametrics.com/victorialogs/data-ingestion/) at the port `9481` and spreads them among the two `vlstorage` nodes started above:

```sh
./victoria-logs-prod -httpListenAddr=:9481 -storageNode=localhost:9491,localhost:9492 &
```

Start `vlselect` node, which [accepts incoming queries](https://docs.victoriametrics.com/victorialogs/querying/) at the port `9471` and requests the needed data from `vlstorage` nodes started above:

```sh
./victoria-logs-prod -httpListenAddr=:9471 -storageNode=localhost:9491,localhost:9492 &
```

Note that all the VictoriaLogs cluster components - `vlstorage`, `vlinsert` and `vlselect` - share the same executable - `victoria-logs-prod`.
Their roles depend on whether the `-storageNode` command-line flag is set - if this flag is set, then the executable runs in `vlinsert` and `vlselect` modes.
Otherwise it runs in `vlstorage` mode, which is identical to a [single-node VictoriaLogs mode](https://docs.victoriametrics.com/victorialogs/).

Let's ingest some logs (aka [wide events](https://jeremymorrell.dev/blog/a-practitioners-guide-to-wide-events/))
from [GitHub archive](https://www.gharchive.org/) into the VictoriaLogs cluster with the following command:

```sh
curl -s https://data.gharchive.org/$(date -d '2 days ago' '+%Y-%m-%d')-10.json.gz \
        | curl -T - -X POST -H 'Content-Encoding: gzip' 'http://localhost:9481/insert/jsonline?_time_field=created_at&_stream_fields=type'
```

Let's query the ingested logs via [`/select/logsql/query` HTTP endpoint](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs).
For example, the following command returns the number of stored logs in the cluster:

```sh
curl http://localhost:9471/select/logsql/query -d 'query=* | count()'
```

See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line) for details on how to query logs from command line.

Logs also can be explored and queried via [built-in Web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui).
Open `http://localhost:9471/select/vmui/` in the web browser, select `last 7 days` time range in the top right corner and explore the ingested logs.
See [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/) to familiarize yourself with the query language.

Every `vmstorage` node can be queried individually because [it is equivalent to a single-node VictoriaLogs](#single-node-and-cluster-mode-duality).
For example, the following command returns the number of stored logs at the first `vmstorage` node started above:

```sh
curl http://localhost:9491/select/logsql/query -d 'query=* | count()'
```

It is recommended reading [key concepts](https://docs.victoriametrics.com/victorialogs/keyconcepts/) before you start working with VictoriaLogs.

See also [security docs](#security).

## Performance tuning

Cluster components of VictoriaLogs automatically adjust their settings for the best performance and the lowest resource usage on the given hardware.
So there is no need in any tuning of these components in general. The following options can be used for achieving higher performance / lower resource
usage on systems with constrained resources:

- `vlinsert` limits the number of concurrent requests to every `vlstorage` node. The default concurrency works great in most cases.
  Sometimes it can be increased via `-insert.concurrency` command-line flag at `vlinsert` in order to achieve higher data ingestion rate
  at the cost of higher RAM usage at `vlinsert` and `vlstorage` nodes.

- `vlinsert` compresses the data sent to `vlstorage` nodes in order to reduce network bandwidth usage at the cost of slightly higher CPU usage
  at `vlinsert` ant `vlstorage` nodes. The compression can be disabled by passing `-insert.disableCompression` command-line flag to `vlinsert`.
  This reduces CPU usage at `vlinsert` and `vlstorage` nodes at the cost of significantly higher network bandwidth usage.

- `vlselect` requests compressed data from `vlstorage` nodes in order to reduce network bandwidth usage at the cost of slightly higher CPU usage
  at `vlselect` and `vlstorage` nodes. The compression can be disabled by passing `-select.disableCompression` command-line flag to `vlselect`.
  This reduces CPU usage at `vlselect` and `vlstorage` nodes at the cost of significanlty higher network bandwidth usage.

## Advanced usage

Cluster components of VictoriaLogs provide various settings, which can be configured via command-line flags if needed.
Default values for all the command-line flags work great in most cases, so it isn't recommended
tuning them without the real need. See [the list of supported command-line flags at VictoriaLogs](https://docs.victoriametrics.com/victorialogs/#list-of-command-line-flags).
