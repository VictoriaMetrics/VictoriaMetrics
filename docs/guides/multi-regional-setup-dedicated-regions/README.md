---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

## Overview {#scenario}

This guide shows how to run VictoriaMetrics across multiple regions with dedicated monitoring clusters. Each workload region runs vmagent and sends metrics to two monitoring regions, so both monitoring regions store the same data.

Use this architecture when you need region-level resilience and want monitoring to keep working even if one region becomes unavailable.

This setup gives you:

* High availability for data across regions
* A global query view
* Simpler disaster recovery

The trade-off is that you store and transmit the same data twice, so storage and compute requirements are doubled.

## Architecture

This architecture separates workload regions from monitoring regions, called Earth, Mars, and Venus in the example. Each workload region runs vmagent and sends the same metrics to two dedicated monitoring regions, called Ground Control 1 and 2, each running VictoriaMetrics. 

![Multi-regional setup with VictoriaMetrics: Dedicated regions for monitoring](setup-1.webp)

This setup works with VictoriaMetrics [single-node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) and [cluster mode](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/). As vmagent keeps a separate queue for each remote-write destination, a problem in one region does not block delivery to the other.

## High Availability

This setup provides high availability by storing a full copy of the data in each Ground Control region. Because every workload region sends the same metrics to both monitoring regions, you can lose one Ground Control region and still query the data from the other one.

You do not need to use VictoriaMetrics cluster `-replicationFactor` for this cross-region setup. Here, availability comes from vmagent replicating writes to independent monitoring regions.

vmagent also keeps a separate queue for each `-remoteWrite.url`, so a problem in one region does not block delivery to the other. The trade-off is that storage and cross-region traffic are duplicated.

## How to write the data to Ground Control regions

Run one vmagent in each workload region (Earth, Mars, Venus) and configure each with two `-remoteWrite.url` flags, one for each VictoriaMetrics instance. For example:

```sh
/path/to/vmagent-prod \
  -remoteWrite.url=<ground-control-1-remote-write>:8428 \
  -remoteWrite.url=<ground-control-2-remote-write>:8428
```

If you scrape Prometheus-compatible targets, also pass `-promscrape.config` so vmagent knows what to scrape before it forwards data.

```sh
/path/to/vmagent-prod \
  -promscrape.config=/path/to/scrape.yaml \
  -remoteWrite.url=https://ground-control-1:8428/api/v1/write \
  -remoteWrite.url=https://ground-control-2:8428/api/v1/write
```

For more details, read the [vmagent Quickstart guide](https://docs.victoriametrics.com/victoriametrics/vmagent/#quick-start)

## How to read the data from Ground Control regions

You can read data from Ground Control regions in a few different ways. The best option depends on your needs:

* Regional endpoints: use one region endpoint as default and manually switch to the other during an outage. This is the simplest option but needs manual failover.
* Load balancer: put one stable endpoint and route traffic to a preferred region, with automatic failover to another region if needed.
* Global vmselect: can keep serving queries during a regional outage, but it waits on all configured regions and needs [deduplication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#deduplication). Requires VictoriaMetrics cluster mode.
* Multi-level vmselect: Can also keep serving queries during a regional outage, with a top-level vmselect merging regional vmselect nodes and [deduplicating](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#deduplication) replicated data. Requires VictoriaMetrics cluster mode.

You can read more about choosing the right architecture in the [VictoriaMetrics topologies guide](https://docs.victoriametrics.com/guides/vm-architectures/).

### Regional endpoints

In this setup, Grafana, vmalert, or any other query client sends requests to one region by default, and you switch to another region only if the default region becomes unavailable. For instance, use Ground Control 1 as the primary datasource and keep Ground Control 2 as a standby endpoint.

TODO: diagram

This option works well when operational simplicity matters more than automatic failover or a single global query endpoint. It also works with both VictoriaMetrics single-node and cluster deployments, because it only relies on each region exposing a working query endpoint.

If you use VictoriaMetrics single-node, the endpoints point directly to the single-node HTTP API. For example:
- Primary endpoint: `https://ground-control-1:8428/prometheus/api/v1/query`
- Standby endpoint: `https://ground-control-2:8428/prometheus/api/v1/query`

On VictoriaMetrics cluster, the endpoints point to each cluster vmselect HTTP API. For example:
- Primary endpoint: `https://ground-control-1-vmselect:8428/select/0/prometheus`
- Standby endpoint: `https://ground-control-2-vmselect:8428/select/0/prometheus`

### Load balancer

Use a load balancer when you want one stable query endpoint in front of multiple Ground Control regions. In this setup, dashboards and tools send queries to a single URL, and the load balancer routes them to a preferred region. If that region becomes unavailable, the load balancer can fail over to another one.

This option keeps the client side simple because users only need one endpoint. It also works with both VictoriaMetrics single-node and cluster deployments, because the load balancer only routes requests to an existing query endpoint in each region.

You can use [vmauth] as a load balancer. For VictoriaMetrics single node, you can start with the following configuration:

```yaml
unauthorized_user:
  url_prefix:
    - "http://ground-control-1:8428/"
    - "http://ground-control-2:8428/"
  load_balancing_policy: first_available
```

On VictoriaMetrics cluster, the URLs must point to the vmselect nodes. For example:

```yaml
unauthorized_user:
  url_prefix:
    - "http://ground-control-1-vmselect:8481/select/0/prometheus/"
    - "http://ground-control-2-vmselect:8481/select/0/prometheus/"
  load_balancing_policy: first_available
```

The examples above show how to load balance requests without authentication. You can optionally implement authentication in several ways, for more details read the [vmauth authorization section](https://docs.victoriametrics.com/victoriametrics/vmauth/#authorization).

Start vmauth with your configuration file as follows:

```sh
/path/to/vmauth-prod -auth.config=/path/to/auth.yaml
```

### Global vmselect

> This options requires [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/)

Use a global vmselect when each Ground Control region runs VictoriaMetrics cluster and you want one VictoriaMetrics-native query endpoint across all regions. In this setup, you run an additional vmselect that knows about storage in every Ground Control region and queries all of them directly.

TODO: diagram

Because the same samples are stored in more than one region, you must enable [deduplication] on the global vmselect. VictoriaMetrics stores timestamps with millisecond precision, so you need to provide the `-dedup.minScrapeInterval=1ms` flag to handle duplicated samples.

For example:

```sh
/path/to/vmselect-prod \
  -storageNode=ground-control-1-vmstorage-1:8401,ground-control-1-vmstorage-2:8401,ground-contorl-2-vmstorage-1:8401,ground-control-2-vmstorage-2:8401 \
  -dedup.minScrapeInterval=1ms
```

This option supports MetricsQL and gives you a single query endpoint for all Ground Control regions. It can also continue serving data during a regional outage, because the global vmselect can still query the remaining region.

The main trade-off is query behavior under failure or latency, as vmselect waits for responses from all storages in all regions, so slow or unavailable backends can increase query latency or lead to [partial responses](https://docs.victoriametrics.com/guides/vm-architectures/#query-consistency-partial-vs-complete-responses).

### Multi-level vmselect

Use multi-level vmselect when each Ground Control region runs VictoriaMetrics cluster and you want a global VictoriaMetrics query layer on top of the regional clusters. In this setup, each Ground Control region runs its own local vmselect, and a top-level vmselect queries those regional vmselect nodes instead of querying vmstorage directly.

TODO: diagram

This option is useful when direct access to regional vmstorage is not practical or not desirable. For example when running on Kubernetes, since the vmstorage services don't provide an HTTP query endpoint.

To enable this setup, each regional vmselect must listen for requests from the global layer by setting `-clusternativeListenAddr`. The global vmselect must then use `-storageNode` to point to the regional vmselect nodes.

```sh
# Ground Control 1 cluster vmselect
/path/to/vmselect-prod \
  -storageNode=ground-control-1-vmstorage-1:8401,ground-control-1-vmstorage-2-vmstorage-2:8401 \
  -clusternativeListenAddr=:8401

# Ground Control 2 cluster vmselect
/path/to/vmselect-prod \
  -storageNode=ground-control-2-vmstorage-1:8401,ground-control-2-vmstorage-2-vmstorage-2:8401 \
  -clusternativeListenAddr=:8401

# top-level vmselect
/path/to/vmselect-prod \
  -storageNode=ground-control-1-vmselect:8401,ground-control-2-vmselect:8401 \
  -dedup.minScrapeInterval=1ms
```

This option supports MetricsQL and can keep serving queries during a regional outage, because the top-level vmselect can still query the remaining regional vmselect nodes. It also gives you a cleaner separation between regional and global query layers than a single global vmselect that talks to all vmstorage nodes directly.

The main trade-off is complexity. You add another query layer and more moving parts, so this setup is harder to deploy and operate than regional endpoints, a load balancer, or regional endpoints.

## Alerting

Run vmalert in each Ground Control region and point it to the local VictoriaMetrics endpoint. Because each region stores the same data, you can deploy the same alerting and recording rules in every region without cross-region rule synchronization. Send alerts to an [Alertmanager cluster](https://prometheus.io/docs/alerting/latest/alertmanager/#high-availability) so duplicate notifications are deduplicated automatically.

TODO: diagram

You can set up vmalert in each Ground Control region to evaluate and enforce recording and alerting rules. As every region contains a full copy of the data, you don't need to synchronize recording rules from one region to another.

A simple vmalert example for single node VictoriaMetrics looks like this:

```sh
/path/to/vmalert \
  -rule=/path/to/rules.yaml \
  -datasource.url=http://ground-control-1:8428 \
  -notifier.url=http://alertmanager-1:9093,http://alertmanager-2:9093
```

In VictoriaMetrics cluster mode, point `-datasource.url` to the regional vmselect endpoint. For example:

```sh
/path/to/vmalert \
  -rule=/path/to/rules.yaml \
  -datasource.url=http://ground-control-1-vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager-1:9093,http://alertmanager-2:9093
```

If you want vmalert to preserve alert state and recording rule results across restarts, configure `-remoteWrite.url` and `-remoteRead.url` to point to VictoriaMetrics as well. For example, for VictoriaMetrics cluster:

```sh
/path/to/vmalert \
  -rule=/path/to/rules.yaml \
  -datasource.url=http://ground-control-1-vmselect:8481/select/0/prometheus \
  -remoteWrite.url=http://ground-control-1-vminsert:8480/insert/0/prometheus \
  -remoteRead.url=http://ground-control-1-vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager-1:9093,http://alertmanager-2:9093
```

We recommend adopting the list of [alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
for VictoriaMetrics components.

## Monitoring

Monitor the monitoring system through a separate path. A practical approach is to run an additional VictoriaMetrics single-node instance in each Ground Control region and scrape metrics from the local VictoriaMetrics components into it.

Refer to the following pages to learn more:
* [How to monitor VictoriaMetrics single node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
* [How to monitor a VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring)

You also may evaluate the option to send these metrics to the neighbour region to achieve high-availability in the monitoring of monitoring system.

## What more can we do?

You can deploy additional vmagent instances in Ground Control regions and use them as regional ingestion proxies. This places the write endpoint closer to storage and adds another disk-backed buffer, which improves resilience when storage is temporarily unavailable.

TODO: diagram

This pattern is useful when you want more reliable delivery, local relabeling, or a cleaner separation between cross-region traffic and local storage ingestion.

For a Ground Control running VictoriaMetrics single node, you can run vmagent as follows:

```sh
# vmagent inside Ground Control 1
/path/to/vmagent-prod \
  -remoteWrite.url=http://ground-control-1:8428/api/v1/write
```

If running in cluster mode, use this instead:

```sh
# vmagent inside Ground Control 1 for cluster mode
/path/to/vmagent-prod \
  -remoteWrite.url=http://ground-control-1-vminsert:8480/insert/0/prometheus
```

