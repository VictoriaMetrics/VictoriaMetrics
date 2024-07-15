---
title: Prometheus
weight: 1
sort: 1
menu:
  docs:
    identifier: "prometheus"
    parent: "data-ingestion"
    weight: 1
aliases:
  - /data-ingestion/prometheus.html
  - /data-ingestion/Prometheus.html
---

VictoriaMetrics Supports ingesting data from Prometheus via the Prometheus remote write protocol.

## Remote Write (Push)
The following changes will need to be added to your Prometheus configuration and Prometheus will need to be restarted to enable forwarding data from Prometheus to VictoriaMetrics.
The Configuration location will vary based on your platform but we have listed the default locations below

Linux: `/etc/prometheus/prometheus.yml`
Docker and Podman: a volume mapped to `/etc/prometheus/prometheus.yml` ex. `sudo podman run -p 9090:9090 -v ./prometheus.yml:/etc/prometheus/prometheus.yml docker.io/prom/prometheus`
Helm: add snippets to the `values.yml` of the helm chart and upgrade helm
Windows and MacOS: Is determined by the `--config.file` flag in the Prometheus command

### No Authentication


```yaml
remote_write:
  - url: 'https://<victoriametrics_url>/prometheus/api/v1/write'
```


### Basic Authentication


```yaml
remote_write:
  - url: 'https://<victoriametrics_url>/prometheus/api/v1/write'
    basic_auth:
      username: '<username>'
      password: '<password>'
```

### Bearer (Token) Authentication


```yaml
remote_write:
  - url: 'https://<victoriametrics_url>/prometheus/api/v1/write'
    authorization:
      type: 'Bearer'
      credentials: '<token>'
```

### Signed TLS/SSL Certificate


```yaml
remote_write:
  - url: 'https://<victoriametrics_url>/prometheus/api/v1/write'
    authorization:
      type: 'Bearer'
      credentials: '<token>'
    tls_config:
      insecure_skip_verify: true
```

## References


- [Prometheus configuration documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
- [Prometheus Helm chart values](https://github.com/prometheus-community/helm-charts/blob/main/charts/prometheus/values.yaml)
