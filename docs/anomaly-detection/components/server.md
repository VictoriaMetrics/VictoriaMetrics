---
title: Server
weight: 7
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 7
    identifier: "vmanomaly-server"
tags:
  - metrics
  - enterprise
aliases:
  - ./server.html
---

Server component of VictoriaMetrics Anomaly Detection (`vmanomaly`) is responsible for serving the REST API (e.g. `/metrics` endpoint) and the [web UI](https://docs.victoriametrics.com/anomaly-detection/ui/) for anomaly detection. 

> If set, it also acts as metrics publishing endpoint for VictoriaMetrics Agent or other Prometheus-compatible scrapers to collect [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/self-monitoring/), so no `monitoring.pull` is needed to be set in such cases.

### Parameters

`addr`, `port`, `path_prefix`, `uvicorn_config`, `ui_default_state`, and `max_concurrent_tasks` parameters can be set in the `server` section of the vmanomaly configuration file. Below is the description of each parameter:

- `addr`: IP address of the query server to listen on. Default is `0.0.0.0`.
- `port`: Port of the query server to listen on. Default is `8490`.
- `path_prefix`: Optional URL path prefix for all HTTP routes. If set to `my-app` or `/my-app`, routes will be served under `<vmanomaly-host>:<port>/my-app/...`.
- `ui_default_state`: Optional [UI](https://docs.victoriametrics.com/anomaly-detection/ui/) state fragment to open on `/vmui/`. Must be URL-encoded and start with `#/?` (e.g. `#/?param=value`). See [Default State](https://docs.victoriametrics.com/anomaly-detection/ui/#default-state) section for details on constructing the value from UI state.
- `max_concurrent_tasks`: Maximum number of concurrent anomaly detection tasks processed by the backend. Positive integer. All tasks above the limit will be cancelled if the limit is exceeded. Defaults to `2`.
- `uvicorn_config`: Uvicorn configuration dictionary. Default is `{"log_level": "warning"}`. See [Uvicorn server settings](https://www.uvicorn.org/settings/) for details.

### Example Configuration

> If [hot-reloading](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#hot-reloading) is enabled in vmanomaly service, the server will automatically pick up changes made to the configuration file without requiring a restart.

```yaml
server:
  addr: '0.0.0.0'
  port: 8490
  path_prefix: '/vmanomaly'  # optional path prefix for all HTTP routes
  
  # see https://docs.victoriametrics.com/anomaly-detection/ui/#default-state section for details on constructing the value from UI state
  ui_default_state: '#/?anomaly_threshold=1.0&anomaly_consecutive=true&fit_window=3d'  # optional default UI state opened on /vmui/
  max_concurrent_tasks: 4  # maximum number of concurrent anomaly detection tasks processed by backend

  uvicorn_config:  # optional Uvicorn server configuration
    log_level: 'warning'

# other vmanomaly configuration sections, like reader, scheduler, models, etc.
```

### Accessing the server

After starting the `vmanomaly` server with the above configuration, UI can be accessed at `<vmanomaly-host>:8490/vmanomaly/vmui/` (e.g. `http://localhost:8490/vmanomaly/vmui/`).

Rest API endpoints (e.g. `/metrics`) can be accessed at `<vmanomaly-host>:8490/vmanomaly/metrics` (e.g. `http://localhost:8490/vmanomaly/metrics`).