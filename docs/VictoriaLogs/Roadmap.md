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

The [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/) Preview is ready for evaluation in production.
It is recommended running it alongside the existing solutions such as Elasticsearch and Grafana Loki
and comparing their resource usage and usability.
It isn't recommended migrating from existing solutions to VictoriaLogs Preview yet.

The following functionality is available in VictoriaLogs Preview:

- [Data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
- [Querying](https://docs.victoriametrics.com/VictoriaLogs/querying/).
- [Querying via command-line](https://docs.victoriametrics.com/VictoriaLogs/querying/#command-line).

See [these docs](https://docs.victoriametrics.com/VictoriaLogs/) for details.

The following functionality is planned in the future versions of VictoriaLogs:

- Support for [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) from popular log collectors and formats:
  - Fluentd
  - Syslog
  - Journald (systemd)
- Add missing functionality to [LogsQL](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html):
  - [Stream context](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stream-context).
  - [Transformation functions](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#transformations).
  - [Post-filtering](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#post-filters).
  - [Stats calculations](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stats).
  - [Sorting](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#sorting).
  - [Limiters](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#limiters).
  - The ability to use subqueries inside [in()](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#multi-exact-filter) function.
- Live tailing for [LogsQL filters](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#filters) aka `tail -f`.
- Web UI with the following abilities:
  - Explore the ingested logs ([partially done](https://docs.victoriametrics.com/VictoriaLogs/querying/#web-ui)).
  - Build graphs over time for the ingested logs.
- Integration with Grafana.
- Ability to make instant snapshots and backups in the way [similar to VictoriaMetrics](https://docs.victoriametrics.com/#how-to-work-with-snapshots).
- Cluster version of VictoriaLogs.
- Ability to store data to object storage (such as S3, GCS, Minio).
- Alerting on LogsQL queries.
