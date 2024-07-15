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

Grafana Alloy supports sending data via the Prometheus Remote Write Protocol and Open Telemetry.
Current this guide only explains 
Collecting metrics and forwarding them to VictoriaMetrics using Prometheus scraping and remote write is more straight forward, but using Open Telemetry enables more complex processing operations to occur before sending data to VictoriaMetrics.
The Alloy configuration can found in the following location depending on your platform

- Linux: `/etc/alloy/config.alloy`
- Windows: `%ProgramFiles%\GrafanaLabs\Alloy\config.alloy`
- MacOS: `$(brew --prefix)/etc/alloy/config.alloy`

After the configuration has been updated Alloy will need to be reloaded or restarted for the configuration change to be applied.

- Linux: `sudo systemctl reload alloy.service`
- Windows: `Restart-Service Alloy` This can also be done from the gui using task manager
- MacOS: `brew services restart alloy`


## Collect Node Exporter data locally and send it to VictoriaMetrics without authentication


```
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoria_metrics_url>/prometheus/api/v1/write"
  }
}
```


## Collect Node Exporter data locally and send it to VictoriaMetrics with basic authentication

This is same as the previous configuration but adds the `basic_auth` parameters

```
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoriametrics_url>/prometheus/api/v1/write/api/v1/write"
    basic_auth {
      username = "<victoriametrics_user>"
      password = "<victoriametrics_password>"
    }
  }
}
```

## Collect Node Exporter data locally and send it to VictoriaMetrics with bearer authentication (token)

This is same as the first config but adds the `authorization` parameter

```
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoria_metrics_url>/prometheus/api/v1/write"
    bearer_token  = "<token>"
  }
}
```

## Scrape Prometheus local Prometheus endpoints and send it to VictoriaMetrics

This configuration will work for remote endpoints as well if localhost is changed to the IP address or hostname of the target you scraping.
The authorization line can be removed or changed to basic authentication as seen in the previous examples.

```
prometheus.exporter.unix "nodeexporter" {}

prometheus.scrape "nodeexporter" {
  targets = prometheus.exporter.unix.nodeexporter.targets
  forward_to = [prometheus.remote_write.victoriametrics.receiver]
}


prometheus.scrape "remote_exporter" {
  targets = [
    { "__address__" = "localhost:9200"},
    { "__address__" = "localhost:9100"},
  ]
  forward_to      = [prometheus.remote_write.victoriametrics.receiver]
}

prometheus.remote_write "victoriametrics" {
  endpoint {
    url = "https://<victoria_metrics_url>/prometheus/api/v1/write"
    bearer_token  = "<token>"
  }
}
```


## Send Metrics to VictoriaMetrics via OpenTelemetry without Authentication 

```
otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "https://<victoriametrics_url>:<victoriametrics_port>/opentelemetry"
  }
}


otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}
```



## Send Metrics to VictoriaMetrics via OpenTelemetry with Basic Authentication

```
otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "https://<victoriametrics_url>:<victoriametrics_port>/opentelemetry"
    auth = otelcol.auth.basic.otel_auth.handler
  }
}

otelcol.auth.basic "otel_auth" {
  username = "<user>"
  password = "<password>"
}

otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}
```

## Send Metrics to VictoriaMetrics via OpenTelemetry with Bearer Authentication

```
otelcol.exporter.otlphttp "victoriametrics" {
  client {
    endpoint = "https://<victoriametrics_url>:<victoriametrics_port>/opentelemetry"
    auth = otelcol.auth.basic.otel_auth.handler
  }
}

otelcol.auth.bearer "otel_auth" {
  token = "<token>"
}

otelcol.processor.batch "batch" {
  output {
    metrics = [otelcol.exporter.otlphttp.victoriametrics.input]
  }
}

```

## References


- [Grafana Alloy Documenation](https://grafana.com/docs/alloy/latest)
