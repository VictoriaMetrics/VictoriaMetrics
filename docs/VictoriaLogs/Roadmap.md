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

The [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) Preview is ready for evaluation in production.
It is recommended running it alongside the existing solutions such as Elasticsearch and Grafana Loki
and comparing their resource usage and usability.
It isn't recommended migrating from existing solutions to VictoriaLogs Preview yet.

The following functionality is available in VictoriaLogs Preview:

- [Data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/).
- [Querying](https://docs.victoriametrics.com/victorialogs/querying/).
- [Querying via command-line](https://docs.victoriametrics.com/victorialogs/querying/#command-line).

See [these docs](https://docs.victoriametrics.com/victorialogs/) for details.

The following functionality is planned in the future versions of VictoriaLogs:

- Support for [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/) from popular log collectors and formats:
  - OpenTelemetry for logs
  - Fluentd
  - Journald (systemd)
- Add missing functionality to [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/):
  - [Stream context](https://docs.victoriametrics.com/victorialogs/logsql/#stream-context).
- Live tailing for [LogsQL filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters) aka `tail -f`.
- Web UI with the following abilities:
  - Explore the ingested logs ([partially done](https://docs.victoriametrics.com/victorialogs/querying/#web-ui)).
  - Build graphs over time for the ingested logs via [hits HTTP API](https://docs.victoriametrics.com/victorialogs/querying/#querying-hits-stats).
- Integration with Grafana ([partially done](https://github.com/VictoriaMetrics/victorialogs-datasource)).
- Ability to make instant snapshots and backups in the way [similar to VictoriaMetrics](https://docs.victoriametrics.com/#how-to-work-with-snapshots).
- Cluster version of VictoriaLogs.
- Ability to store data to object storage (such as S3, GCS, Minio).
- Alerting on LogsQL queries.
