---
weight: 10
title: Journald setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 10
tags:
  - logs
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

VictoriaLogs uses the `__REALTIME_TIMESTAMP` field as [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field)
for the logs ingested via journald protocol. Other field can be used instead of `__REALTIME_TIMESTAMP` by specifying it via `-journald.timeField` command-line flag.
See [the list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

## Level field

VictoriaLogs automatically sets the `level` log field according to the [`PRIORITY` field value](https://wiki.archlinux.org/title/Systemd/Journal).

## Stream fields

VictoriaLogs uses `(_MACHINE_ID, _HOSTNAME, _SYSTEMD_UNIT)` as [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
for logs ingested via journald protocol. The list of log stream fields can be changed via `-journald.streamFields` command-line flag if needed,
by providing comma-separated list of journald fields form [this list](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

Please make sure that the log stream fields passed to `-journlad.streamFields` do not contain fields with high number or unbound number of unique values,
since this may lead to [high cardinality issues](https://docs.victoriametrics.com/victorialogs/keyconcepts/#high-cardinality).

The following Journald fields are also good candidates for stream fields:

- `_TRANSPORT`
- `_SYSTEMD_USER_UNIT`


## Dropping fields

VictoriaLogs can be configured for skipping the given [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
for logs ingested via journald protocol, via `-journald.ignoreFields` command-line flag, which accepts comma-separated list of log fields to ignore.
This list can contain log field prefixes ending with `*` such as `some-prefix*`. In this case all the fields starting from `some-prefix` are ignored.

See [the list of supported Journald fields](https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html).

## Multitenancy

By default VictoriaLogs stores logs ingested via journald protocol into `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
This can be changed by passing the needed tenant in the format `AccountID:ProjectID` at the `-journald.tenantID` command-line flag.
For example, `-journald.tenantID=123:456` would store logs ingested via journald protocol into `(AccountID=123, ProjectID=456)` tenant.

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Docker-compose demo for Journald integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/journald).
