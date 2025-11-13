---
title: Grafana
weight: 1
menu:
  docs:
    parent: "integrations-vm"
    identifier: "integrations-grafana-vm"
    weight: 1
---

VictoriaMetrics integrates with Grafana using either [Prometheus datasource](https://grafana.com/docs/grafana/latest/datasources/prometheus/)
or [VictoriaMetrics datasource](https://grafana.com/grafana/plugins/victoriametrics-metrics-datasource/) plugins.

Resources:
* [VictoriaMetrics Grafana demo playground](https://play-grafana.victoriametrics.com)
* [VictoriaMetrics and Grafana in docker-compose environment](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#docker-compose-environment-for-victoriametrics)

## VictoriaMetrics datasource

Create [VictoriaMetrics datasource](https://grafana.com/grafana/plugins/victoriametrics-metrics-datasource/)
in Grafana with the following URL for single-server:
```

http://<victoriametrics-addr>:8428
```
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For the cluster version, use `vmselect` address:
```
http://<vmselect-addr>:8481/select/<tenant>/prometheus
```
_Replace `<vmselect-addr>` with the hostname or IP address of vmselect service._ 

If you have more than 1 vmselect, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

Once connected, you can start building graphs and dashboards using [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/)
or [MetricsQL](https://docs.victoriametrics.com/victoriametrics/metricsql/).

VictoriaMetrics datasource is publicly available on [GitHub](https://github.com/VictoriaMetrics/victoriametrics-datasource).
See more in [plugin docs](https://docs.victoriametrics.com/victoriametrics/integrations/grafana/datasource/).

_Creating a datasource may require [specific permissions](https://grafana.com/docs/grafana/latest/administration/data-source-management/).
If you don't see an option to create a data source - try contacting system administrator._


## Prometheus datasource

Create [Prometheus datasource](https://grafana.com/docs/grafana/latest/datasources/prometheus/configure-prometheus-data-source/)
in Grafana. Follow the same connection instructions as for [VictoriaMetrics datasource](#VictoriaMetrics-datasource).

In the "Type and version" section set the type to "Prometheus" and the version to at least "2.24.x".
This allows Grafana to use a more efficient API to get label values:

![Datasource](datasource-prometheus.webp)

Once connected, you can build graphs and dashboards using [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/).

_Creating a datasource may require [specific permissions](https://grafana.com/docs/grafana/latest/administration/data-source-management/).
If you don't see an option to create a data source - try contacting system administrator._
