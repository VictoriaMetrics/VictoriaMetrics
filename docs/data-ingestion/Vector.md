---
title: Vector
weight: 1
sort: 1
menu:
  docs:
    identifier: "Vector"
    parent: "data-ingestion"
    weight: 1
    # sort: 1
aliases:
  - /data-ingestion/Vector.html
  - /data-ingestion/vector.html
---
To Send data to Vector you need to configure with a Prometheus remote write sink and forward metrics to that sink from at least 1 source.
You will need to replace the values in `<>` with your to match your setup.

## Minimum Config
```yaml
sources:
  host_metrics_source:
    type: host_metrics
sinks:
  victoriametrics_sink:
    type: prometheus_remote_write
    inputs:
      - host_metrics_source
    endpoint: "https://<victoriametrics_url>/api/v1/write"
    healthcheck:
      enabled: false
```

## Basic Authentication

This adds support for basic authentication by defining the auth strategy, user, and password fields:


```yaml
sources:
  host_metrics_source:
    type: host_metrics
sinks:
  victoriametrics_sink:
    type: prometheus_remote_write
    inputs:
      - host_metrics_source
    endpoint: "https://<victoriametrics_url>/api/v1/write"
    auth:
      strategy: "basic"
      user: "<victoriametrics_user"
      password: "<victoriametrics_password>"
    healthcheck:
      enabled: false

```

## Bearer / Token Authentication

This adds support for bearer/token authentication by defining the auth strategy and token fields:


```yaml
sources:
  host_metrics_source:
    type: host_metrics
sinks:
  victoriametrics_sink:
    type: prometheus_remote_write
    inputs:
      - host_metrics_source
    endpoint: "https://<victoriametrics_url>/api/v1/write"
    auth:
      strategy: "bearer"
      token: "<victoriametrics_token>"
    healthcheck:
      enabled: false
```

## VictoriaMetrics and VictoriaLogs

This combines the Bearer Authentication section with the [VictoriaLogs docs for Vector](https://docs.victoriametrics.com/victorialogs/data-ingestion/vector/),
so you can send metrics and logs with 1 agent to multiple sources:


```yaml
sources:
  host_metrics_source:
    type: host_metrics
  journald_source:
    type: journald
sinks:
  victoriametrics_sink:
    type: prometheus_remote_write
    inputs:
      - host_metrics_source
    endpoint: "https://<victoriametrics_url>/api/v1/write"
    auth:
      strategy: "bearer"
      token: "<token>"
    healthcheck:
      enabled: false
  victorialogs_sink:
    inputs:
      - journald_source
    type: elasticsearch
    endpoints:
      - "https://<victorialogs_url>/insert/elasticsearch/"
    mode: bulk
    api_version: "v8"
    healthcheck:
      enabled: false
    query:
      _msg_field: "message"
      _time_field: "timestamp"
      _stream_fields: "host,container_name"
```

# References
- [Vector documentation](https://vector.dev/docs/)
- [VictoriaLogs documentation for using vector](../VictoriaLogs/data-ingestion/Vector.md)
