---
title: vmagent
description: "Using vmagent as a data ingestion agent."
weight: 2
menu:
  docs:
    identifier: data-ingestion-vmagent
    parent: data-ingestion
    weight: 2
tags:
  - metrics
aliases:
  - /data-ingestion/vmagent.html
  - /data-ingestion/vmagent/index.html
  - /data-ingestion/vmagent/
  - /data-ingestion/VMAgent.html
---

vmagent can receive data via the same protocols as VictoriaMetrics Single or Cluster versions,
as well as scrape Prometheus endpoints. In other words, 
it supports both [Push](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model) and [Pull](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#pull-model) models.

This section of the documentation only covers forwarding data from vmagent to another destination.
For extra information about vmagent as well as quickstart guide please refer to the [vmagent documentation](https://docs.victoriametrics.com/victoriametrics/vmagent/).

To configure vmagent to push metrics to VictoriaMetrics via Prometheus remote write protocol,
configure the `-remoteWrite.url` cmd-line flag:

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>/api/v1/write
```

For pushing data to VictoriaMetrics cluster the `-remoteWrite.url` should point to vminsert and include
the [tenantID](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format):
```sh
/path/to/vmagent -remoteWrite.url=https://<vminsert-addr>/insert/<tenant_id>/prometheus/api/v1/write
```

> Note: read more about [multitenancy](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
> or [multitenancy via labels](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels).

Please note, `-remoteWrite.url` cmd-line flag can be specified multiple times with different values. In this case,
vmagent will [replicate](https://docs.victoriametrics.com/victoriametrics/vmagent/#replication-and-high-availability) data to each
specified destination. In addition, it is possible to configure [metrics sharding](https://docs.victoriametrics.com/victoriametrics/vmagent/#sharding-among-remote-storages)
across `-remoteWrite.url` destinations.

## Remote write directly to vmstorage

`vmagent` can write data directly to `vmstorage` nodes via Prometheus Remote Write v1 protocol.
This requires starting every target `vmstorage` node with `-enableIngestionAPI` and using the
[cluster URL format](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format):

```sh
/path/to/vmagent \
  -remoteWrite.url=http://<vmstorage-1>:8482/insert/<tenant_id>/prometheus/api/v1/write \
  -remoteWrite.url=http://<vmstorage-2>:8482/insert/<tenant_id>/prometheus/api/v1/write \
  -remoteWrite.url=http://<vmstorage-3>:8482/insert/<tenant_id>/prometheus/api/v1/write \
  -remoteWrite.shardByURL \
  -remoteWrite.shardByURLReplicas=2
```

In this setup, `vmagent` behaves like `vminsert` running with `-replicationFactor=2` and `-disableReroutingOnUnavailable`.
`vmagent` maintains a separate [persistent queue](https://docs.victoriametrics.com/victoriametrics/vmagent/#on-disk-persistence)
for each `vmstorage` node and doesn't reroute data to other nodes. This helps mitigate [re-routing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#slowness-based-re-routing) issues when some `vmstorage` nodes respond slowly or become unavailable.

## Remote write with basic authentication

This requires setting the `-remoteWrite.basicAuth.username` and `-remoteWrite.basicAuth.password` command line flags:
```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>/api/v1/write \
 -remoteWrite.basicAuth.username=<username> \
 -remoteWrite.basicAuth.password=<password>
```

## Remote write with bearer Authentication

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>/api/v1/write \
 -remoteWrite.bearerToken=<token>
```

The token can be placed in a file and accessed via the `-remoteWrite.bearerTokenFile` command line argument.
The file needs to be readable by the user vmagent is running as. The token file should only contain the token as seen below:
```
<token>
```

The command to run vmagent with a token file will be the following:
```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>/api/v1/write \
 -remoteWrite.bearerTokenFile=/path/to/tokenfile
```

## Ignore TLS/SSL errors

If you are using self-signed certificates you can either ignore certificates issues using 
the `-remoteWrite.tlsInsecureSkipVerify`, which is a security risk, or use `-remoteWrite.tlsCAFile` to point
to a file containing the self-signed CA certificate:

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>/api/v1/write \
 -remoteWrite.tlsInsecureSkipVerify
```

## References

- [vmagent docs](https://docs.victoriametrics.com/victoriametrics/vmagent/)
- [vmagent commandline flags](https://docs.victoriametrics.com/victoriametrics/vmagent/#advanced-usage)
