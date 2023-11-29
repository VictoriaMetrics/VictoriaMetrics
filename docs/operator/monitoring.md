---
sort: 6
weight: 6
title: Monitoring
menu:
  docs:
    parent: "operator"
    weight: 6
aliases:
  - /operator/monitoring.html
---

# Monitoring of VictoriaMetrics Operator

VictoriaMetrics operator exports internal metrics in Prometheus exposition format at `/metrics` page.

These metrics can be scraped via [vmagent](./resources/vmagent.md) or Prometheus.

## Dashboard

Official Grafana dashboard available for [vmoperator](https://grafana.com/grafana/dashboards/17869-victoriametrics-operator/).

<img src="monitoring_operator-dashboard.png" width=1200>

Graphs on the dashboards contain useful hints - hover the `i` icon in the top left corner of each graph to read it.

<!-- TODO: alerts for operator -->

## Configuration

### Helm-chart victoria-metrics-k8s-stack

In [victoria-metrics-k8s-stack](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/README.md) helm-chart operator self-scrapes metrics by default.

This helm-chart also includes [official grafana dashboard for operator](#dashboard).

### Helm-chart victoria-metrics-operator

With [victoria-metrics-operator](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-operator/README.md) you can use following parameter in `values.yaml`:

```yaml
# values.yaml
#...
# -- configures monitoring with serviceScrape. VMServiceScrape must be pre-installed
serviceMonitor:
  enabled: true
```

This parameter makes helm-chart to create a scrape-object for installed operator instance.

You will also need to deploy a (vmsingle)[./resources/vmsingle.md] where the metrics will be collected.

### Pure operator installation

With pure operator installation you can use config with separate vmsingle and scrape object for operator like that:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  name: vmoperator
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app.kubernetes.io/instance: vm-operator
      app.kubernetes.io/name: victoria-metrics-operator
  endpoints:
    - port: http
  namespaceSelector:
    matchNames:
      - monitoring
```

See more info about object [VMServiceScrape](./resources/vmservicescrape.md).

You will also need a [vmsingle](https://docs.victoriametrics.com/operator/resources/vmsingle.html) where the metrics will be collected.

