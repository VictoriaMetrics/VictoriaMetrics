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
Enable logs and specify a custom URL instead of default one for sending collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```yaml
logs_enabled: true
logs_config:
  logs_dd_url: http://localhost:9428/
  use_http: true
```

While using [Serverless DataDog plugin](https://github.com/DataDog/serverless-plugin-datadog) please set VictoriaLogs endpoint using `LOGS_DD_URL` environment variable:

```yaml
custom:
  datadog:
    apiKey: fakekey                 # Set any key, otherwise plugin fails
provider:
  environment:
    LOGS_DD_URL: <<vm-url>>/   # VictoriaLogs endpoint for DataDog
```

Substitute the `localhost:9428` address with the real address of VictoriaLogs.
