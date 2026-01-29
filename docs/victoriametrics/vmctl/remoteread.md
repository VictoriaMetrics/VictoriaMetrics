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

Filtering flags can be provided multiple times {{% available_from "v1.129.0" %}} to narrow down the selection of timeseries to migrate.
For example:
```sh
./vmctl remote-read \
    --remote-read-filter-label=tenant --remote-read-filter-label-value="team-eu" \
    --remote-read-filter-label=__name__ --remote-read-filter-label-value="cpu_.*"
```
will select only timeseries with `tenant="team-eu"` label and metric names matching `cpu_.*` regex.

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

{{% content "vmctl_remote-read_flags.md" %}}