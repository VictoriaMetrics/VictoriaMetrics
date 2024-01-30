---
sort: 22
weight: 22
title: Quick start
menu:
  docs:
    parent: 'victoriametrics'
    weight: 22
aliases:
- /Quick-Start.html
---

# Quick start

## How to install

VictoriaMetrics is distributed in two forms:
* [Single-server-VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html) - all-in-one
  binary, which is very easy to use and maintain.
  Single-server-VictoriaMetrics perfectly scales vertically and easily handles millions of metrics/s;
* [VictoriaMetrics Cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) - set of components
  for building horizontally scalable clusters.

Single-server-VictoriaMetrics VictoriaMetrics is available as:

* [Managed VictoriaMetrics at AWS](https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc)
* [Docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/)
* [Snap packages](https://snapcraft.io/victoriametrics)
* [Helm Charts](https://github.com/VictoriaMetrics/helm-charts#list-of-charts)
* [Binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
* [Source code](https://github.com/VictoriaMetrics/VictoriaMetrics).
  See [How to build from sources](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-build-from-sources)
* [VictoriaMetrics on Linode](https://www.linode.com/marketplace/apps/victoriametrics/victoriametrics/)
* [VictoriaMetrics on DigitalOcean](https://marketplace.digitalocean.com/apps/victoriametrics-single)

Just download VictoriaMetrics and follow
[these instructions](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-start-victoriametrics).
Then read [Prometheus setup](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prometheus-setup)
and [Grafana setup](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#grafana-setup) docs.

VictoriaMetrics is developed at a fast pace, so it is recommended periodically checking the [CHANGELOG](https://docs.victoriametrics.com/CHANGELOG.html) and performing [regular upgrades](https://docs.victoriametrics.com/#how-to-upgrade-victoriametrics).


### Starting VM-Single via Docker

The following commands download the latest available
[Docker image of VictoriaMetrics](https://hub.docker.com/r/victoriametrics/victoria-metrics)
and start it at port 8428, while storing the ingested data at `victoria-metrics-data` subdirectory
under the current directory:


```sh
docker pull victoriametrics/victoria-metrics:latest
docker run -it --rm -v `pwd`/victoria-metrics-data:/victoria-metrics-data -p 8428:8428 victoriametrics/victoria-metrics:latest
```


Open <a href="http://localhost:8428">http://localhost:8428</a> in web browser
and read [these docs](https://docs.victoriametrics.com/#operation).

There is also [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html)
- horizontally scalable installation, which scales to multiple nodes.

### Starting VM-Cluster via Docker

The following commands clone the latest available
[VictoriaMetrics repository](https://github.com/VictoriaMetrics/VictoriaMetrics)
and start the docker container via 'make docker-cluster-up'. Further customization is possible by editing
the [docker-compose-cluster.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/docker-compose-cluster.yml)
file.


```sh
git clone https://github.com/VictoriaMetrics/VictoriaMetrics && cd VictoriaMetrics
make docker-cluster-up
```


See more details [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#readme).

* [Cluster setup](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup)

## Write data

There are two main models in monitoring for data collection: 
[push](https://docs.victoriametrics.com/keyConcepts.html#push-model) 
and [pull](https://docs.victoriametrics.com/keyConcepts.html#pull-model). 
Both are used in modern monitoring and both are supported by VictoriaMetrics.

See more details on [writing data here](https://docs.victoriametrics.com/keyConcepts.html#write-data).


## Query data

VictoriaMetrics provides an 
[HTTP API](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prometheus-querying-api-usage)
for serving read queries. The API is used in various integrations such as
[Grafana](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#grafana-setup).
The same API is also used by
[VMUI](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#vmui) - graphical User Interface
for querying and visualizing metrics.

[MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) - is the query language for executing read queries
in VictoriaMetrics. MetricsQL is a [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics) 
-like query language with a powerful set of functions and features for working specifically with time series data.

See more details on [querying data here](https://docs.victoriametrics.com/keyConcepts.html#query-data)


## Alerting

It is not possible to physically trace all changes on graphs all the time, that is why alerting exists.
In [vmalert](https://docs.victoriametrics.com/vmalert.html) it is possible to create a set of conditions
based on PromQL and MetricsQL queries that will send a notification when such conditions are met.

## Data migration

Migrating data from other TSDBs to VictoriaMetrics is as simple as importing data via any of
[supported formats](https://docs.victoriametrics.com/keyConcepts.html#push-model).

The migration might get easier when using [vmctl](https://docs.victoriametrics.com/vmctl.html) - VictoriaMetrics
command line tool. It supports the following databases for migration to VictoriaMetrics:
* [Prometheus using snapshot API](https://docs.victoriametrics.com/vmctl.html#migrating-data-from-prometheus);
* [Thanos](https://docs.victoriametrics.com/vmctl.html#migrating-data-from-thanos);
* [InfluxDB](https://docs.victoriametrics.com/vmctl.html#migrating-data-from-influxdb-1x);
* [OpenTSDB](https://docs.victoriametrics.com/vmctl.html#migrating-data-from-opentsdb);
* [Migrate data between VictoriaMetrics single and cluster versions](https://docs.victoriametrics.com/vmctl.html#migrating-data-from-victoriametrics).

## Productionisation

When going to production with VictoriaMetrics we recommend following the recommendations.

### Monitoring

Each VictoriaMetrics component emits its own metrics with various details regarding performance
and health state. Docs for the components also contain a `Monitoring` section with an explanation
of what and how should be monitored. For example,
[Single-server-VictoriaMetrics Monitoring](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#monitoring).

VictoriaMetric team prepared a list of [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards)
for the main components. Each dashboard contains a lot of useful information and tips. It is recommended
to have these dashboards installed and up to date.

Using the [recommended alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
versions would also help to identify and notify about issues with the system.

The rule of thumb is to have a separate installation of VictoriaMetrics or any other monitoring system
to monitor the production installation of VictoriaMetrics. This would make monitoring independent and
will help identify problems with the main monitoring installation.

See more details in the article [VictoriaMetrics Monitoring](https://victoriametrics.com/blog/victoriametrics-monitoring/).

### Capacity planning

See capacity planning sections in [docs](https://docs.victoriametrics.com) for
[Single-server-VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#capacity-planning)
and [VictoriaMetrics Cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#capacity-planning).

Capacity planning isn't possible without [monitoring](#monitoring), so consider configuring it first.
Understanding resource usage and performance of VictoriaMetrics also requires knowing the tech terms
[active series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series),
[churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate),
[cardinality](https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality),
[slow inserts](https://docs.victoriametrics.com/FAQ.html#what-is-a-slow-insert).
All of them are present in [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).


### Data safety

It is recommended to read [Replication and data safety](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#replication-and-data-safety),
[Why replication doesn’t save from disaster?](https://valyala.medium.com/speeding-up-backups-for-big-time-series-databases-533c1a927883)
and [backups](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#backups).


### Configuring limits

To avoid excessive resource usage or performance degradation limits must be in place:
* [Resource usage limits](https://docs.victoriametrics.com/FAQ.html#how-to-set-a-memory-limit-for-victoriametrics-components);
* [Cardinality limiter](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cardinality-limiter).

### Security recommendations

* [Security recommendations for single-node VictoriaMetrics](https://docs.victoriametrics.com/#security)
* [Security recommendations for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#security)
