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

VictoriaLogs can accept logs from the following syslog collectors:

- [Rsyslog](https://www.rsyslog.com/). See [these docs](#rsyslog).
- [Syslog-ng](https://www.syslog-ng.com/). See [these docs](#syslog-ng).

Multiple logs in Syslog format can be ingested via a single TCP connection or via a single UDP packet - just put every log on a separate line
and delimit them with `\n` char.

VictoriaLogs automatically extracts the following [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
from the received Syslog lines:

- [`_time`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field) - log timestamp. See also [log timestamps](#log-timestamps)
- [`_msg`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) - the `MESSAGE` field from the supported syslog formats above
- `hostname`, `app_name` and `proc_id` - [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) for unique identification
  over every log stream
- `priority`, `factility` and `severity` - these fields are extracted from `<PRI>` field
- `format` - this field is set to either `rfc3164` or `rfc5424` depending on the format of the parsed syslog line
- `msg_id` - `MSGID` field from log line in `RFC5424` format.

The `[STRUCTURED-DATA]` is parsed into fields with the `SD-ID.param1`, `SD-ID.param2`, ..., `SD-ID.paramN` names and the corresponding values
according to [the specification](https://datatracker.ietf.org/doc/html/rfc5424#section-6.3).

By default local timezone is used when parsing timestamps in `rfc3164` lines. This can be changed to any desired timezone via `-syslog.timezone` command-line flag.
See [the list of supported timezone identifiers](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones). For example, the following command starts VictoriaLogs,
which parses syslog timestamps in `rfc3164` using `Europe/Berlin` timezone:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.timezone='Europe/Berlin'
```

The ingested logs can be queried via [logs querying API](https://docs.victoriametrics.com/victorialogs/querying/#http-api). For example, the following command
returns ingested logs for the last 5 minutes by using [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter):

```sh
curl http://localhost:9428/select/logsql/query -d 'query=_time:5m'
```

See also:

- [Log timestamps](#log-timestamps)
- [Security](#security)
- [Compression](#compression)
- [Multitenancy](#multitenancy)
- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).

## Log timestamps

By default VictoriaLogs uses the timestamp from the parsed Syslog message as [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field).
Sometimes the ingested Syslog messages may contain incorrect timestamps (for example, timestamps with incorrect timezone). In this case VictoriaLogs can be configured
for using the log ingestion timestamp as [`_time` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field). This can be done by specifying
`-syslog.useLocalTimestamp.tcp` command-line flag for the corresponding `-syslog.listenAddr.tcp` address:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.useLocalTimestamp.tcp
```

In this case the original timestamp from the Syslog message is stored in `timestamp` [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).

The `-syslog.useLocalTimestamp.udp` command-line flag can be used for instructing VictoriaLogs to use local timestamps for the ingested logs
via the corresponding `-syslog.listenAddr.udp` address:

```sh
./victoria-logs -syslog.listenAddr.udp=:514 -syslog.useLocalTimestamp.udp
```

## Security

By default VictoriaLogs accepts plaintext data at `-syslog.listenAddr.tcp` address. Run VictoriaLogs with `-syslog.tls` command-line flag
in order to accept TLS-encrypted logs at `-syslog.listenAddr.tcp` address. The `-syslog.tlsCertFile` and `-syslog.tlsKeyFile` command-line flags
must be set to paths to TLS certificate file and TLS key file if `-syslog.tls` is set. For example, the following command
starts VictoriaLogs, which accepts TLS-encrypted syslog messages at TCP port 6514:

```sh
./victoria-logs -syslog.listenAddr.tcp=:6514 -syslog.tls -syslog.tlsCertFile=/path/to/tls/cert -syslog.tlsKeyFile=/path/to/tls/key
```

## Compression

By default VictoriaLogs accepts uncompressed log messages in Syslog format at `-syslog.listenAddr.tcp` and `-syslog.listenAddr.udp` addresses.
It is possible configuring VictoriaLogs to accept compressed log messages via `-syslog.compressMethod.tcp` and `-syslog.compressMethod.udp` command-line flags.
The following compression methods are supported:

- `none` - no compression
- `gzip` - [gzip compression](https://en.wikipedia.org/wiki/Gzip)
- `deflate` - [deflate compression](https://en.wikipedia.org/wiki/Deflate)

For example, the following command starts VictoriaLogs, which accepts gzip-compressed syslog messages at TCP port 514:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.compressMethod.tcp=gzip
```

## Multitenancy

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `-syslog.tenantID.tcp` or `-syslog.tenantID.udp` command-line flags
depending on whether TCP or UDP ports are listened for syslog messages.
For example, the following command starts VictoriaLogs, which writes syslog messages received at TCP port 514, to `(AccountID=12, ProjectID=34)` tenant:

```sh
./victoria-logs -syslog.listenAddr.tcp=:514 -syslog.tenantID.tcp=12:34
```

## Multiple configs

VictoriaLogs can accept syslog messages via multiple TCP and UDP ports with individual configurations for [log timestamps](#log-timestamps), [compression](#compression), [security](#security)
and [multitenancy](#multitenancy). Specify multiple command-line flags for this. For example, the following command starts VictoriaLogs,
which accepts gzip-compressed syslog messages via TCP port 514 at localhost interface and stores them to [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) `123:0`,
plus it accepts TLS-encrypted syslog messages via TCP port 6514 and stores them to [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy) `567:0`:

```sh
./victoria-logs \
  -syslog.listenAddr.tcp=localhost:514 -syslog.tenantID.tcp=123:0 -syslog.compressMethod.tcp=gzip -syslog.tls=false -syslog.tlsKeyFile='' -syslog.tlsCertFile='' \
  -syslog.listenAddr.tcp=:6514 -syslog.tenantID.tcp=567:0 -syslog.compressMethod.tcp=none -syslog.tls=true -syslog.tlsKeyFile=/path/to/tls/key -syslog.tlsCertFile=/path/to/tls/cert
```

## Rsyslog

1. Run VictoriaLogs with `-syslog.listenAddr.tcp=:29514` command-line flag.
1. Put the following line to [rsyslog](https://www.rsyslog.com/) config (this config is usually located at `/etc/rsyslog.conf`):
   ```
   *.* @@victoria-logs-server:29514
   ```
   Where `victoria-logs-server` is the hostname where VictoriaLogs runs. See [these docs](https://www.rsyslog.com/sending-messages-to-a-remote-syslog-server/)
   for more details.

## Syslog-ng

1. Run VictoriaLogs with `-syslog.listenAddr.tcp=:29514` command-line flag.
1. Put the following line to [syslog-ng](https://www.syslog-ng.com/) config:
   ```
   destination d_remote {
    tcp("victoria-logs-server" port(29514));
   };
   ```
   Where `victoria-logs-server` is the hostname where VictoriaLogs runs.
   See [these docs](https://www.syslog-ng.com/technical-documents/doc/syslog-ng-open-source-edition/3.19/administration-guide/29#TOPIC-1094570) for details.
