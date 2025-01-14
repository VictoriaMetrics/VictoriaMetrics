---
weight: 5
title: DataDog Agent setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 5
url: /victorialogs/data-ingestion/datadog-agent/
aliases:
  - /VictoriaLogs/data-ingestion/DataDogAgent.html
---
Datadog Agent doesn't support custom path prefix, so for this reason it's required to use [VMAuth](https://docs.victoriametrics.com/vmauth/) or any other
reverse proxy to append `/insert/datadog` path prefix to all Datadog API logs requests.

In case of [VMAuth](https://docs.victoriametrics.com/vmauth/) your config should look like:

```yaml
unauthorized_user:
  url_map:
    - src_paths:
        - "/api/v2/logs"
        - "/api/v1/validate"
      url_prefix: `<victoria-logs-base-url>`/insert/datadog/
    - src_paths:
        - "/api/v1/series"
        - "/api/v2/series"
        - "/api/beta/sketches"
        - "/api/v1/validate"
        - "/api/v1/check_run"
        - "/intake"
        - "/api/v1/metadata"
      url_prefix: `<victoria-metrics-base-url>`/datadog/
```

To start ingesting logs from DataDog agent please specify a custom URL instead of default one for sending collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```yaml
logs_enabled: true
logs_config:
  logs_dd_url: `<vmauth-base-url>`
  use_http: true
```

While using [Serverless DataDog plugin](https://github.com/DataDog/serverless-plugin-datadog) please set VictoriaLogs endpoint using `LOGS_DD_URL` environment variable:

```yaml
custom:
  datadog:
    apiKey: fakekey                 # Set any key, otherwise plugin fails
provider:
  environment:
    DD_DD_URL: `<vmauth-base-url>`/   # VMAuth endpoint for DataDog
```

Substitute the `<vmauth-base-url>` address with the real address of VMAuth proxy.

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Docker-compose demo for Datadog integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/datadog-agent).
- [Docker-compose demo for Datadog Serverless integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/datadog-serverless).
