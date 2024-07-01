---
sort: 8
weight: 8
title: VictoriaLogs roadmap
disableToc: true
menu:
  docs:
    parent: "victorialogs"
    weight: 8
    title: Roadmap
aliases:
- /VictoriaLogs/Roadmap.html
---

# VictoriaLogs roadmap

The following functionality is available in [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

- [Data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/).
- [Querying](https://docs.victoriametrics.com/victorialogs/querying/).
- [Querying via command-line](https://docs.victoriametrics.com/victorialogs/querying/#command-line).

See [these docs](https://docs.victoriametrics.com/victorialogs/) for details.

The following functionality is planned in the future versions of VictoriaLogs:

- Support for [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/) from popular log collectors and formats:
  - OpenTelemetry for logs
  - Fluentd
  - Journald (systemd)
  - Datadog protocol for logs
- Integration with Grafana ([partially done](https://github.com/VictoriaMetrics/victorialogs-datasource)).
- Ability to make instant snapshots and backups in the way [similar to VictoriaMetrics](https://docs.victoriametrics.com/#how-to-work-with-snapshots).
- Cluster version of VictoriaLogs.
- Ability to store data to object storage (such as S3, GCS, Minio).
- Alerting on LogsQL queries.
- Data migration tool from Grafana Loki to VictoriaLogs (similar to [vmctl](https://docs.victoriametrics.com/vmctl/)).
