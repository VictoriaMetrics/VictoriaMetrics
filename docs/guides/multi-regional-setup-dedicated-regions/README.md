---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

## Overview {#scenario}

This guide shows how to run VictoriaMetrics across multiple regions in high-availability mode. Each workload runs a local vmagent and sends metrics to dedicated monitoring deployments, so metric data is duplicated and available even if one VictoriaMetrics instance is down.

Use this architecture when you need region-level resilience and want monitoring to keep working even if one region becomes unavailable.

This setup gives you:

* High availability for data across regions
* A global query view
* Simpler disaster recovery

The trade-off is that you store and transmit the same data twice, so storage and compute requirements are increased.

## Architecture

This architecture separates workload regions, called Earth, Mars, and Venus, from monitoring regions, called Ground Control 1 and 2. Each workload region runs a local vmagent and sends the same metrics to the two dedicated monitoring regions, each running a separate instance of VictoriaMetrics.

![Multi-regional setup with VictoriaMetrics: Dedicated regions for monitoring](setup-1.webp)

This setup works with VictoriaMetrics [single-node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) and [cluster mode](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/). As vmagent keeps a separate queue for each remote-write destination, an outage in one region does not block delivery to the other.

### High Availability

The architecture provides high availability by storing a full copy of the data in each of the Ground Control regions. Since both regions receive the same data, losing one keeps metric data available for queries, dashboards, and alerts.

You do not need to use VictoriaMetrics cluster `-replicationFactor` for this cross-region setup, as availability comes from vmagent replicating writes to independent monitoring regions.

## How to write the data to Ground Control regions

Run a local vmagent in each workload region (Earth, Mars, Venus) and point them to the Ground Control URLs using the `-remoteWrite.url` flag. For example:

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
* Load balancer: put a load balancer in front of both Ground Control regions. Route traffic to a preferred region, with automatic failover to the other region on failure.
* Global vmselect: wire vmselect directly to the vmstorage nodes for both Ground Control regions (which must run in cluster mode).
* Multi-level vmselect: run a dedicated vmselect on top of the Ground Control regional vmselect nodes. This setup also requires both Ground Control instances to run in cluster mode.

You can read more about choosing the right architecture in the [VictoriaMetrics topologies guide](https://docs.victoriametrics.com/guides/vm-architectures/).

### Regional endpoints

In this setup, Grafana, vmalert, or any other query client sends requests to one region by default, and you manually switch to another region only if the default region becomes unavailable. For instance, use Ground Control 1 as the primary datasource and keep Ground Control 2 as a standby endpoint.

TODO: diagram

This option works well when operational simplicity matters more than automatic failover or a single global query endpoint. It also works with both VictoriaMetrics single-node and cluster deployments, because it only relies on each region exposing a query endpoint.

If you use VictoriaMetrics single-node, the endpoints point directly to the single-node HTTP API. For example:
- Primary endpoint: `https://ground-control-1:8428/prometheus/api/v1/query`
- Standby endpoint: `https://ground-control-2:8428/prometheus/api/v1/query`

On the VictoriaMetrics cluster, the endpoints point to each cluster's vmselect HTTP API. For example:
- Primary endpoint: `https://ground-control-1-vmselect:8428/select/0/prometheus`
- Standby endpoint: `https://ground-control-2-vmselect:8428/select/0/prometheus`

### Load balancer

Use a load balancer when you want one stable query endpoint in front of multiple Ground Control regions. In this setup, dashboards and tools send queries to a single URL, and the load balancer routes them to a preferred region. If that region becomes unavailable, the load balancer can fail over to another one.

This option keeps the client side simple because users only need one endpoint. It also works with both VictoriaMetrics single-node and cluster deployments, because the load balancer only routes requests to an existing query endpoint in each region.

You can use [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/) as a load balancer. For VictoriaMetrics single node, you can run it with the following configuration:

```yaml
unauthorized_user:
  url_prefix:
    - "http://ground-control-1:8428/"
    - "http://ground-control-2:8428/"
  load_balancing_policy: first_available
```

On the VictoriaMetrics cluster, the URLs must point to the Ground Control vmselect nodes. For example:

```yaml
unauthorized_user:
  url_prefix:
    - "http://ground-control-1-vmselect:8481/select/0/prometheus/"
    - "http://ground-control-2-vmselect:8481/select/0/prometheus/"
  load_balancing_policy: first_available
```

The examples above show how to load balance requests without authentication. You can optionally implement authentication in several ways; for more details, read the [vmauth authorization section](https://docs.victoriametrics.com/victoriametrics/vmauth/#authorization).

To start vmauth with your configuration, use the `-auth.config` flag. For example:

```sh
/path/to/vmauth-prod -auth.config=/path/to/auth.yaml
```

### Global vmselect

> This option requires a [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) for Ground Control.

Use a global vmselect when each Ground Control region runs a VictoriaMetrics cluster, and you want one VictoriaMetrics-native query endpoint across all regions. In this setup, you run an extra vmselect node that knows about storage in every Ground Control region and queries all of them directly.

TODO: diagram

Since the same samples are duplicated across Ground Control regions, you must enable [deduplication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#deduplication) on the global vmselect. VictoriaMetrics stores timestamps with millisecond precision, so you need to provide the `-dedup.minScrapeInterval=1ms` flag to handle duplicated samples.

For example:

```sh
/path/to/vmselect-prod \
  -storageNode=ground-control-1-vmstorage-1:8401,ground-control-1-vmstorage-2:8401,ground-contorl-2-vmstorage-1:8401,ground-control-2-vmstorage-2:8401 \
  -dedup.minScrapeInterval=1ms
```

This option supports MetricsQL and gives you a single query endpoint for all Ground Control regions. It can also continue serving data during an outage on a Ground Control region.

The main trade-off is query behavior under region failure or latency, as the global vmselect waits for responses from all storages in all regions; a slow or unavailable backend can increase query latency or lead to [partial responses](https://docs.victoriametrics.com/guides/vm-architectures/#query-consistency-partial-vs-complete-responses).

### Multi-level vmselect

> This option requires a [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) for Ground Control.

Use multi-level vmselect when each Ground Control region runs a VictoriaMetrics cluster, and you want a top-level VictoriaMetrics query layer over the regional Ground Control clusters. In this setup, each Ground Control region runs its own local vmselect, and a top-level vmselect queries them instead of connecting to vmstorage nodes directly.

TODO: diagram

This option is useful when direct access to vmstorage nodes is not practical or not desirable. For example, when running on Kubernetes, the vmstorage services don't provide an HTTP query endpoint by default.

To enable this setup, each regional vmselect must listen for requests from the global layer by setting the `-clusternativeListenAddr` flag. The top-level vmselect must then use `-storageNode` to point to the Ground Control vmselect nodes and must set a [deduplication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#deduplication) interval to handle duplicated data across the Ground Control regions.

For example, here's how we can run the local and top-level vmselect nodes:

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

This option supports MetricsQL and can keep serving queries during a regional outage, because the top-level vmselect can still query the remaining regional vmselect nodes. It also gives you a cleaner separation between regional and top-level query layers than a single global vmselect that talks to all vmstorage nodes directly. 

The main trade-off is complexity. You add another query layer and more moving parts, so this setup is harder to deploy and operate than regional endpoints, a load balancer, or global vmselect.

## Alerting

Run a vmalert node in each Ground Control region and point it to the local VictoriaMetrics endpoint. Since each region stores the same data, you can deploy the same alerting and recording rules in every region without needing cross-region rule synchronization. Send alerts to an [Alertmanager cluster](https://prometheus.io/docs/alerting/latest/alertmanager/#high-availability) to deduplicate firing alerts automatically.

TODO: diagram

A simple vmalert example for a single-node VictoriaMetrics looks like this:

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

If you want vmalert to preserve alert state and recording rule results across restarts, configure `-remoteWrite.url` and `-remoteRead.url` to point to VictoriaMetrics as well. For example, for a VictoriaMetrics cluster:

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

You can monitor VictoriaMetrics itself with a separate, dedicated monitoring path. In this setup, each Ground Control region runs its own monitoring instance that scrapes metrics from the Ground Control components. 

TODO: diagram

You can optionally duplicate the monitored metrics to the neighboring region for extra resilience. That way, if a whole Ground Control region goes down, you still have access to the telemetry of the downed VictoriaMetrics instance, which can help you troubleshoot and restore service more easily.

TODO: diagram

Refer to the following pages on how to monitor your monitoring deployments:
* [How to monitor VictoriaMetrics single node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
* [How to monitor a VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring)

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

