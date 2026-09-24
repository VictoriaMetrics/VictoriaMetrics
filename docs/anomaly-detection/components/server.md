---
title: Server
description: "HTTP server. REST API, /metrics endpoint, and web UI."
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
- {{% available_from "v1.29.2" anomaly %}} `use_reader_connection_settings`: If set to `true`, UI will use connection settings (e.g. credentials, TLS, etc.) from the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters) configuration when connecting to data sources. This allows UI to connect to data sources with the same settings without requiring having `vmauth` in front of both UI and data sources.

### Example Configuration

> [!TIP]
> If [hot-reloading](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) is enabled in vmanomaly service, the server will automatically pick up changes made to the configuration file without requiring a restart.

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

  use_reader_connection_settings: true  # if set to true, UI will use connection settings from reader configuration below when connecting to data sources, allowing it to connect with the same credentials, TLS settings, etc. without requiring having vmauth in front of both UI and data sources.

# other vmanomaly configuration sections, like reader, scheduler, models, etc.
reader:
  datasource_url: %{DS_URL}
  user: %{DS_USER}
  password: %{DS_PASSWORD}
  # or
  # bearer_token: %{DS_BEARER_TOKEN}
  verify_tls: false
```

### Accessing the server

After starting the `vmanomaly` server with the above configuration, UI can be accessed at `<vmanomaly-host>:8490/vmanomaly/vmui/` (e.g. `http://localhost:8490/vmanomaly/vmui/`).

Rest API endpoints (e.g. `/metrics`) can be accessed at `<vmanomaly-host>:8490/vmanomaly/metrics` (e.g. `http://localhost:8490/vmanomaly/metrics`).

### Interactive API reference

Open `/docs` on your running vmanomaly server to inspect generated request and response schemas and try API calls. For a local instance without a path prefix, use [http://localhost:8490/docs](http://localhost:8490/docs). With the example configuration above, use `http://localhost:8490/vmanomaly/docs`. Replace the host, port and optional prefix to match your deployment. ReDoc is available at `/redoc`, and the OpenAPI schema at `/openapi.json`, under the same prefix.

The reference describes the API exposed in that runtime’s OpenAPI schema; separately mounted applications may expose their own routes. **Try it out** sends real requests and can start or cancel tasks. Restrict access to these documentation routes together with the API through your deployment’s authentication proxy or network controls. Hiding the documentation alone does not secure the API.

### Time-series analysis and autotune API

{{% available_from "v1.30.0" anomaly %}} The server exposes bounded endpoints for UI, MCP, and automation workflows:

- `GET /api/v1/timeseries/characteristics` samples the supplied query and summarizes trend, calendar seasonality, changepoints, gaps, and intermittent or spiky behavior. Use `limit` (default 100) to cap sampled series and pass the production `step` and timezone.
- `POST /api/v1/autotune/tasks` starts asynchronous shared-model tuning. The request contains the input query, candidate `tuned_class_name`, expected `anomaly_percentage`, data-source settings, and optimization parameters.
- `GET /api/v1/autotune/tasks/{task_id}` returns progress and the concrete suggested `modelConfig` when complete.
- `DELETE /api/v1/autotune/tasks/{task_id}` cancels pending work cooperatively.

{{% available_from "v1.30.1" anomaly %}} Seasonality analysis preserves the original timestamp grid when samples are offset from whole step boundaries. This avoids missing daily or weekly patterns solely because timestamps are shifted within the configured sampling interval.

{{% available_from "v1.30.5" anomaly %}} Shared autotune accepts exactly one of a legacy `query` expression or a `queries` object keyed by alias. Each named entry contains `expr` and optional `data_range`, `detection_direction`, `min_dev_from_expected`, and `min_rel_dev_from_expected`. Named input supports 1–50 queries; `limit` caps sampled series separately for each expression. Queries share the same resolved time window.

For multivariate tuning, request the multivariate class directly and supply grouping labels in `frozen_params.groupby`. The server aligns channels before evaluating each candidate across groups in one study; the result is one shared `modelConfig`, not a different hyperparameter configuration per group. Per-query policies remain attached to their channels. An assembled group missing a requested alias fails rather than silently tuning a partial input set. Inspect `inputQueryCount` and `tunedInputCount` in result data for input-query and assembled-input counts.

> [!TIP]
> For a complete request and recommended workflow, see [Shared asynchronous autotune workflow](https://docs.victoriametrics.com/anomaly-detection/components/models/#shared-asynchronous-autotune-workflow). See the [interactive API reference](#interactive-api-reference) for schemas from your running version.
