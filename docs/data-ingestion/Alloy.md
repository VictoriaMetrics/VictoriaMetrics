---
title: Grafana Alloy
weight: 3
menu:
  docs:
    identifier: "alloy"
    parent: "data-ingestion"
    weight: 3
aliases:
  - /data-ingestion/Grafana-Alloy.html
  - /data-ingestion/grafana-alloy.html
  - /data-ingestion/Grafana-Agent.html
  - /data-ingestion/grafana-agent.html

---

[Grafana Alloy](https://grafana.com/docs/alloy/latest/) supports sending data via the Prometheus remote write protocol and OpenTelemetry Protocol (OTLP).
Collecting metrics and forwarding them to VictoriaMetrics using Prometheus scraping and remote writing is more straightforward, but using OpenTelemetry enables more complex processing operations to occur before sending data to VictoriaMetrics.

The Alloy configuration file can be found in the following location depending on your platform:
- Linux: `/etc/alloy/config.alloy`
- Windows: `%ProgramFiles%\GrafanaLabs\Alloy\config.alloy`
- MacOS: `$(brew --prefix)/etc/alloy/config.alloy`

To configure Grafana Alloy to push collected metrics to VictoriaMetrics via Prometheus remote write protocol,
update the `prometheus.remote_write` endpoint:
```Alloy
prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoriametrics-addr>/prometheus/api/v1/write"
  }
}
```

For pushing data to VictoriaMetrics cluster the `url` should point to vminsert and include 
the [tenantID](https://docs.victoriametrics.com/cluster-victoriametrics/#url-format):
```sh
https://<vminsert-addr>/insert/<tenant_id>/prometheus/api/v1/write
```

> Note: read more about [multitenancy](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy)
> or [multitenancy via labels](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy-via-labels).

After the configuration has been updated, Alloy must be reloaded or restarted for the change to be applied:
- Linux: `sudo systemctl reload alloy.service`
- Windows: `Restart-Service Alloy`, this can also be done from the GUI using task manager
- MacOS: `brew services restart alloy`
- Helm chart: changing the `alloy.configMap` in the Alloy [helm values](https://raw.githubusercontent.com/grafana/alloy/main/operations/helm/charts/alloy/values.yaml)

## remote write

In the example below we will be using the node exporter component built into Alloy to generate metrics,
but any Prometheus scrape target can forward data to VictoriaMetrics.
Metrics are forwarded from the scrape target to VictoriaMetrics by creating a `prometheus.remote_write` component
and configuring the `promethues.scrape` component to forward metrics to the `prometheus.remote_write` component.

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

## remote write with basic authentication

This is the same as the previous configuration but adds the `basic_auth` parameters:

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

## remote write with bearer authentication

This is the same as the first config but adds the `bearer_token` parameter:

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

## OpenTelemetry

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

## OpenTelemetry with Basic Authentication

This is the same configuration without authentication but contains the `otelcol.auth.basic` block 
and references it in `otelcol.exporter.otlphttp`:

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

## OpenTelemetry with Bearer Authentication

This is the same as the basic authentication configuration but swaps the `otelcol.auth.basic` for `otelcol.auth.bearer`:

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
- [Grafana Alloy Documentation](https://grafana.com/docs/alloy/latest)
