---
title: Remote Read
weight: 4
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-remote-read"
    weight: 4
---

`vmctl` supports `remote-read` mode for migrating data from remote databases that support
[Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).

`vmctl remote-read` allows migrating data from the following systems:
- [Cortex](https://docs.victoriametrics.com/victoriametrics/vmctl/cortex/)
- [Mimir](https://docs.victoriametrics.com/victoriametrics/vmctl/mimir/)
- [Promscale](https://docs.victoriametrics.com/victoriametrics/vmctl/promscale/)
- [Thanos](https://docs.victoriametrics.com/victoriametrics/vmctl/thanos/#remote-read-protocol)

Remote read API has two implementations of remote read API: default (`SAMPLES`) and
[streamed](https://prometheus.io/blog/2019/10/10/remote-read-meets-streaming/) (`STREAMED_XOR_CHUNKS`).
Streamed version is more efficient but has lower adoption.

See `./vmctl remote-read --help` for details and the full list of flags.

> Migration via remote read protocol allows to fetch data via API. This is usually a resource intensive operation
for Thanos and may be slow or expensive in terms of resources.

The importing process example for local installation of Prometheus:

```sh
./vmctl remote-read \
--remote-read-src-addr=http://<prometheus>:9091 \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://<victoria-metrics>:8428 \
```

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

## Filtering

Filtering by time can be configured via flags `--remote-read-filter-time-start` and `--remote-read-filter-time-end`
in RFC3339 format.

Filtering by labels can be configured via flags `--remote-read-filter-label` and `--remote-read-filter-label-value`.
For example, `--remote-read-filter-label=tenant` and `--remote-read-filter-label-value="team-eu"` will select only series
with `tenant="team-eu"` label-value pair.

## Configuration 

Migrating big volumes of data may result in remote read client reaching the timeout. Increase the value of 
`--remote-read-http-timeout` (default `5m`) command-line flag when seeing timeouts or `context canceled` errors.

Flag `--remote-read-step-interval` allows splitting export data into chunks to reduce pressure on the source `--remote-read-src-addr`.
Valid values are `month, day, hour, minute`.

Flag `--remote-read-use-stream` defines whether to use `SAMPLES` or `STREAMED_XOR_CHUNKS` mode. 
Mode `STREAMED_XOR_CHUNKS` is much less resource intensive for the source, but not many databases support it. 
By default, is uses `SAMPLES` mode.

See general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).

See `./vmctl remote-read --help` for details and full list of flags:
```shellhelp
   --remote-read-concurrency value         Number of concurrently running remote read readers (default: 1)
   --remote-read-filter-time-start value   The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --remote-read-filter-time-end value     The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --remote-read-filter-label value        Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name. (default: "__name__")
   --remote-read-filter-label-value value  Prometheus regular expression to filter label from "remote-read-filter-label-value" flag. (default: ".*")
   --remote-read                           Use Prometheus remote read protocol (default: false)
   --remote-read-use-stream                Defines whether to use SAMPLES or STREAMED_XOR_CHUNKS mode. By default, is uses SAMPLES mode. See https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#streamed-chunks (default: false)
   --remote-read-step-interval value       The time interval to split the migration into steps. For example, to migrate 1y of data with '--remote-read-step-interval=month' vmctl will execute it in 12 separate requests from the beginning of the time range to its end. To reverse the order use '--remote-read-filter-time-reverse'. Requires setting '--remote-read-filter-time-start'. Valid values are 'month','week','day','hour','minute'.
   --remote-read-filter-time-reverse       Whether to reverse the order of time intervals split by '--remote-read-step-interval' cmd-line flag. When set, the migration will start from the newest to the oldest data. (default: false)
   --remote-read-src-addr value            Remote read address to perform read from.
   --remote-read-user value                Remote read username for basic auth [$REMOTE_READ_USERNAME]
   --remote-read-password value            Remote read password for basic auth [$REMOTE_READ_PASSWORD]
   --remote-read-http-timeout value        Timeout defines timeout for HTTP requests made by remote read client (default: 0s)
   --remote-read-headers value             Optional HTTP headers to send with each request to the corresponding remote source storage 
      For example, --remote-read-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding remote source storage. 
      Multiple headers must be delimited by '^^': --remote-read-headers='header1:value1^^header2:value2'
   --remote-read-cert-file value       Optional path to client-side TLS certificate file to use when connecting to -remote-read-src-addr
   --remote-read-key-file value        Optional path to client-side TLS key to use when connecting to -remote-read-src-addr
   --remote-read-CA-file value         Optional path to TLS CA file to use for verifying connections to -remote-read-src-addr. By default, system CA is used
   --remote-read-server-name value     Optional TLS server name to use for connections to remoteReadSrcAddr. By default, the server name from -remote-read-src-addr is used
   --remote-read-insecure-skip-verify  Whether to skip TLS certificate verification when connecting to the remote read address (default: false)
   --remote-read-disable-path-append   Whether to disable automatic appending of the /api/v1/read suffix to --remote-read-src-addr (default: false)
```
