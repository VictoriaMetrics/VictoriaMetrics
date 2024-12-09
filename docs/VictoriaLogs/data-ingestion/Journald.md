---
weight: 10
title: Journald setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 10
aliases:
  - /VictoriaLogs/data-ingestion/Journald.html
---
On a client site which should already have journald please install additionally [systemd-journal-upload](https://www.freedesktop.org/software/systemd/man/latest/systemd-journal-upload.service.html) and edit `/etc/systemd/journal-upload.conf` and set `URL` to VictoriaLogs endpoint:

```
[Upload]
URL=http://localhost:9428/insert/journald
```

Substitute the `localhost:9428` address inside `endpoints` section with the real TCP address of VictoriaLogs.

Since neither HTTP query arguments nor HTTP headers are configurable on systemd-journal-upload,
[stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) and other params can be configured on VictoriaLogs using command-line flags:
- `journald.streamFields` - configures [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) for ingested data.
Here's a [list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html)
- `journald.ignoreFields` - configures Journald fields, that should be ignored.
- `journald.tenantID` - configures TenantID for ingested data.
- `journald.timeField` - configures time field for ingested data.

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Docker-compose demo for Journald integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/journald).
