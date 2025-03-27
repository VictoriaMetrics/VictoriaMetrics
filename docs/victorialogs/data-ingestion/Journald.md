---
weight: 10
title: Journald setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 10
aliases:
  - /victorialogs/data-ingestion/Journald.html
---
On a client site which should already have journald please install additionally [systemd-journal-upload](https://www.freedesktop.org/software/systemd/man/latest/systemd-journal-upload.service.html) and edit `/etc/systemd/journal-upload.conf` and set `URL` to VictoriaLogs endpoint:

```
[Upload]
URL=http://localhost:9428/insert/journald
```

Substitute the `localhost:9428` address inside `endpoints` section with the real TCP address of VictoriaLogs.

## Time field

By default VictoriaLogs use the `__REALTIME_TIMESTAMP` field as [timestamp](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field)
for the logs ingested via journald protocol. This can be modified by setting the `-journald.timeField` command-line flag to the log field name,
which contains the needed timestamp.

See [the list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

## Dropping fields

VictoriaLogs can be configured for skipping the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
for logs ingested via journald protocol, via `-journald.ignoreFields` command-line flag, which accepts comma-separated list of log fields to ignore.
This list can contain log field prefixes ending with `*` such as `some-prefix*`. In this case all the fields starting from `some-prefix` are ignored.

See [the list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

## Stream fields

VictoriaLogs can be configured to use the particular fields as [log stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
for logs ingested via jorunald protocol, via `-journald.streamFields` command-line flag, which accepts comma-separated list of fields to use as log stream fields.

See [the list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

## Multitenancy

By default VictoriaLogs stores logs ingested via journald protocol into `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
This can be changed by passing the needed tenant in the format `AccountID:ProjectID` at the `-journlad.tenantID` command-line flag.
For example, `-journald.tenantID=123:456` would store logs ingested via journald protocol into `(AccountID=123, ProjectID=456)` tenant.

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Docker-compose demo for Journald integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/journald).
