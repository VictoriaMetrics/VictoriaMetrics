---
sort: 8
weight: 8
title: Roadmap
disableToc: true
menu:
  docs:
    identifier: victorialogs-roadmap
    parent: "victorialogs"
    weight: 8
aliases:
- /VictoriaLogs/Roadmap.html
---
The following functionality is available in [VictoriaLogs](./README.md):

- [Data ingestion](./data-ingestion/README.md).
- [Querying](./querying/README.md).
- [Querying via command-line](./querying/#command-line).

See [these docs](./README.md) for details.

The following functionality is planned in the future versions of VictoriaLogs:

- Support for [data ingestion](./data-ingestion/README.md) from popular log collectors and formats:
  - [ ] [OpenTelemetry for logs](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4839)
  - [ ] Fluentd
  - [ ] [Journald](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4618) (systemd)
  - [ ] [Datadog protocol for logs](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6632)
  - [ ] [Telegraf http output](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5310)
- [ ] Integration with Grafana. Partially done, check the [documentation](./victorialogs-datasource.md) and [datasource repository](https://github.com/VictoriaMetrics/victorialogs-datasource).
- [ ] Ability to make instant snapshots and backups in the way [similar to VictoriaMetrics](../#how-to-work-with-snapshots).
- [ ] Cluster version of VictoriaLogs.
- [ ] Ability to store data to object storage (such as S3, GCS, Minio).
- [ ] Alerting on LogsQL queries.
- [ ] Data migration tool from Grafana Loki to VictoriaLogs (similar to [vmctl](../vmctl.md)).
