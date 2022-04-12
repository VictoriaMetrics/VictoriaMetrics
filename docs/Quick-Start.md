---
sort: 13
---

# Quick start

## Installation

Single-server-VictoriaMetrics VictoriaMetrics is available as:

* [Managed VictoriaMetrics at AWS](https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc)
* [Docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/) 
* [Snap packages](https://snapcraft.io/victoriametrics)
* [Helm Charts](https://github.com/VictoriaMetrics/helm-charts#list-of-charts)
* [Binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
* [Source code](https://github.com/VictoriaMetrics/VictoriaMetrics). See [How to build from sources](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-build-from-sources)
* [VictoriaMetrics on Linode](https://www.linode.com/marketplace/apps/victoriametrics/victoriametrics/)
* [VictoriaMetrics on DigitalOcean](https://marketplace.digitalocean.com/apps/victoriametrics-single)

Just download VictoriaMetrics and follow [these instructions](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-start-victoriametrics).
Then read [Prometheus setup](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prometheus-setup) and [Grafana setup](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#grafana-setup) docs.

### Starting VM-Signle via Docker:

The following commands download the latest available [Docker image of VictoriaMetrics](https://hub.docker.com/r/victoriametrics/victoria-metrics) and start it at port 8428, while storing the ingested data at `victoria-metrics-data` subdirectory under the current directory:

```bash
docker pull victoriametrics/victoria-metrics:latest
docker run -it --rm -v `pwd`/victoria-metrics-data:/victoria-metrics-data -p 8428:8428 victoriametrics/victoria-metrics:latest
```

Open `http://localhost:8428` in web browser and read [these docs](https://docs.victoriametrics.com/#operation).

There are also the following versions of VictoriaMetrics available:

* [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) - horizontally scalable VictoriaMetrics, which scales to multiple nodes.

### Starting VM-Cluster via Docker:

The following commands clone the latest available [VictoriaMetrics cluster repository](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) and start the docker container via 'docker-compose'. Further customization is possible by editing the [docker-compose.yaml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/deployment/docker/docker-compose.yml) file.

```bash
git clone https://github.com/VictoriaMetrics/VictoriaMetrics --branch cluster && cd VictoriaMetrics/deployment/docker && docker-compose up
```

* [Cluster setup](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup)

## Writing data

Data can be written to VictoriaMetrics in the following ways:

* [DataDog agent](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-datadog-agent)
* [InfluxDB-compatible agents such as Telegraf](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf)
* [Graphite-compatible agents such as StatsD](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-graphite-compatible-agents-such-as-statsd)
* [OpenTSDB-compatible agents](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-opentsdb-compatible-agents)
* [Prometheus remote_write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
* [In JSON line format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-json-line-format)
* [Imported in CSV format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-csv-data)
* [Imported in Prometheus exposition format](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-prometheus-exposition-format)
* `/api/v1/import` for importing data obtained from [/api/v1/export](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-export-data-in-json-line-format).
 See [these docs](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-import-data-in-json-line-format) for details.

## Reading data

VictoriaMetrics various APIs for reading the data. [This document briefly describes these APIs](https://docs.victoriametrics.com/url-examples.html).

### Grafana setup:

Create [Prometheus datasource](http://docs.grafana.org/features/datasources/prometheus/) in Grafana with the following url:

```url
http://<victoriametrics-addr>:8428
```

Substitute `<victoriametrics-addr>` with the hostname or IP address of VictoriaMetrics.

Then build graphs and dashboards for the created datasource using [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) or [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html).
