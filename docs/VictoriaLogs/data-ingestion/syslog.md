---
weight: 10
title: Syslog setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 10
---

# Syslog setup

[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) can accept logs in [Syslog formats](https://en.wikipedia.org/wiki/Syslog) at the specified TCP and UDP addresses
via `-syslog.listenAddr.tcp` and `-syslog.listenAddr.udp` command-line flags. The following syslog formats are supported:

- [RFC3164](https://datatracker.ietf.org/doc/html/rfc3164) aka `<PRI>MMM DD hh:mm:ss HOSTNAME APP-NAME[PROCID]: MESSAGE`
- [RFC5424](https://datatracker.ietf.org/doc/html/rfc5424) aka `<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MESSAGE`

For example, the following command starts VictoriaLogs, which accepts logs in Syslog format at TCP port 514 on all the network interfaces:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514
```

It may be needed to run VictoriaLogs under `root` user or to set [`CAP_NET_BIND_SERVICE`](https://superuser.com/questions/710253/allow-non-root-process-to-bind-to-port-80-and-443)
option if syslog messages must be accepted at TCP port below 1024.

The following command starts VictoriaLogs, which accepts logs in Syslog format at TCP and UDP ports 514:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.listenAddr.udp=:514
```

Multiple logs in Syslog format can be ingested via a single TCP connection or via a single UDP packet - just put every log on a separate line
and delimit them with `\n` char.

VictoriaLogs automatically extracts the following [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
from the received Syslog lines:

- [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) - log timestamp
- [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) - the `MESSAGE` field from the supported syslog formats above
- `hostname`, `app_name` and `proc_id` - [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) for unique identification
  over every log stream
- `priority`, `factility` and `severity` - these fields are extracted from `<PRI>` field
- `format` - this field is set to either `rfc3164` or `rfc5424` depending on the format of the parsed syslog line
- `msg_id` - `MSGID` field from log line in `RFC5424` format.

By default local timezone is used when parsing timestamps in `rfc3164` lines. This can be changed to any desired timezone via `-syslog.timezone` command-line flag.
See [the list of supported timezone identifiers](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones). For example, the following command starts VictoriaLogs,
which parses syslog timestamps in `rfc3164` using `Europe/Berlin` timezone:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.timezone='Europe/Berlin'
```

See also:

- [Security](#security)
- [Compression](#compression)
- [Multitenancy](#multitenancy)
- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

## Security

By default VictoriaLogs accepts plaintext data at `-syslog.listenAddr.tcp` address. Run VictoriaLogs with `-syslog.tls` command-line flag
in order to accept TLS-encrypted logs at `-syslog.listenAddr.tcp` address. The `-syslog.tlsCertFile` and `-syslog.tlsKeyFile` command-line flags
must be set to paths to TLS certificate file and TLS key file if `-syslog.tls` is set. For example, the following command
starts VictoriaLogs, which accepts TLS-encrypted syslog messages at TCP port 514:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.tls -syslog.tlsCertFile=/path/to/tls/cert -syslog.tlsKeyFile=/path/to/tls/key
```

## Compression

By default VictoriaLogs accepts uncompressed log messages in Syslog format at `-syslog.listenAddr.tcp` and `-syslog.listenAddr.udp` addresses.
It is possible configuring VictoriaLogs to accept compressed log messages via `-syslog.compressMethod` command-line flag. The following
compression methods are supported:

- `none` - no compression
- `gzip` - [gzip compression](https://en.wikipedia.org/wiki/Gzip)
- `deflate` - [deflate compression](https://en.wikipedia.org/wiki/Deflate)

For example, the following command starts VictoriaLogs, which accepts gzip-compressed syslog messages at TCP port 514:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.compressMethod=gzip
```

## Multitenancy

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `-syslog.tenantID` command-line flag.
For example, the following command starts VictoriaLogs, which writes syslog messages received at TCP port 514, to `(AccountID=12, ProjectID=34)` tenant:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.tenantID=12:34
```
