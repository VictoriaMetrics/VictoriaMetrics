---
title: Grafana Alloy
weight: 1
sort: 1
menu:
  docs:
    identifier: "alloy"
    parent: "data-ingestion"
    weight: 1
aliases:
  - /data-ingestion/Grafana-Alloy.html
  - /data-ingestion/grafana-alloy.html
  - /data-ingestion/Grafana-Agent.html
  - /data-ingestion/grafana-agent.html

---

Grafana Alloy supports sending data via the Prometheus Remote Write Protocol and OpenTelemetry Protocol (OTLP).
Collecting metrics and forwarding them to VictoriaMetrics using Prometheus scraping and remote writing is more straightforward, but using OpenTelemetry enables more complex processing operations to occur before sending data to VictoriaMetrics.
The Alloy configuration file can be found in the following location depending on your platform

- Linux: `/etc/alloy/config.alloy`
- Windows: `%ProgramFiles%\GrafanaLabs\Alloy\config.alloy`
- MacOS: `$(brew --prefix)/etc/alloy/config.alloy`

After the configuration has been updated, Alloy must be reloaded or restarted for the change to be applied:

- Linux: `sudo systemctl reload alloy.service`
- Windows: `Restart-Service Alloy`, this can also be done from the GUI using task manager
- MacOS: `brew services restart alloy`
- Helm chart: changing the `alloy.configMap` in the Alloy [helm values](https://raw.githubusercontent.com/grafana/alloy/main/operations/helm/charts/alloy/values.yaml)

In any of the examples below, you can add `insert/<tenant_id>/` to the URL path if you are sending metrics to vminsert.
For Prometheus remote write this would change from

```sh
https://<victoriametrics-addr>:<victoriametrics_port>/prometheus/api/v1/write
```

to

```sh
https://<vminsert-addr>:<victoriametrics_port>/insert/<tenant_id>/prometheus/api/v1/write
```

For OpenTelemetry the endpoint would change from 

```sh
https://<victoriametrics-addr>:<victoriametrics_port>/opentelemetry
```

to

```sh
https://<vminsert-addr>:<victoriametrics_port>/insert/<tenant_id>/opentelemetry
```


In the examples below we will be using node exporter component built into Alloy to generate metrics, but any Prometheus scrape target will work can forward data to VictoriaMetrics.
Metrics are forwarded from the scrape target to VictoriaMetrics by creating a `prometheus.remote_write` component and configuring the `promethues.scrape` component to forward metrics to the `prometheus.remote_write` component.

## Collect Node Exporter data locally and send it to VictoriaMetrics

```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoriametrics-addr>/prometheus/api/v1/write"
  }
}
```


## Collect Node Exporter data locally and send it to VictoriaMetrics with basic authentication

This is same as the previous configuration but adds the `basic_auth` parameters

```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoriametrics-addr>/prometheus/api/v1/write"
    basic_auth {
      username = "<victoriametrics_user>"
      password = "<victoriametrics_password>"
    }
  }
}
```

## Collect Node Exporter data locally and send it to VictoriaMetrics with bearer authentication (token)

This is same as the first config but adds the `bearer_token` parameter

```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoriametrics-addr>/prometheus/api/v1/write"
    bearer_token  = "<token>"
  }
}
```


## Send Metrics to VictoriaMetrics via OpenTelemetry without Authentication 

```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [otelcol.receiver.prometheus.victoriametrics.receiver]
}


otelcol.receiver.prometheus "victoriametrics" {
  output {
    metrics = [otelcol.processor.batch.batch.input]
  }
}

otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}


otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "http://<victoriametrics-addr>:<victoriametrics_port>/opentelemetry"
  }
}
```



## Send Metrics to VictoriaMetrics via OpenTelemetry with Basic Authentication

This is the same configuration without authentication but contains the `otelcol.auth.basic` block and references it in `otelcol.expoerter.otlphttp`


```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [otelcol.receiver.prometheus.victoriametrics.receiver]
}

otelcol.auth.basic "otel_auth" {
  username = "<user>"
  password = "<password>"
}

otelcol.receiver.prometheus "victoriametrics" {
  output {
    metrics = [otelcol.processor.batch.batch.input]
  }
}

otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}


otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "https://<victoriametrics-addr:<victoriametrics_port>/opentelemetry"
    auth = otelcol.auth.basic.otel_auth.handler
  }
}
```

## Send Metrics to VictoriaMetrics via OpenTelemetry with Bearer Authentication

This is the same as the basic authentication configuration but swaps the `otelcol.auth.basic` for `otelcol.auth.bearer`

```Alloy
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [otelcol.receiver.prometheus.victoriametrics.receiver]
}

otelcol.auth.bearer "otel_auth" {
  token = "<token>"  
}

otelcol.receiver.prometheus "victoriametrics" {
  output {
    metrics = [otelcol.processor.batch.batch.input]
  }
}

otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}


otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "https://<victoriametrics-addr>:<victoriametrics_port>/opentelemetry"
    auth = otelcol.auth.bearer.otel_auth.handler
  }
}
```

## References

- [Grafana Alloy Helm Chart](https://github.com/grafana/alloy/tree/main/operations/helm)
- [Grafana Alloy Documenation](https://grafana.com/docs/alloy/latest)
