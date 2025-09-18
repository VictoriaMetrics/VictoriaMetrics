---
title: Cortex
weight: 6
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-cortex"
    weight: 6
---

Cortex supports [Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).
`vmctl` in [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) can be used 
for historical data migration from Cortex.

Check Cortex configuration in the `api` section:
```yaml
api:
  prometheus_http_prefix:
```

If you defined some prometheus prefix, you should use it when you define flag `--remote-read-src-addr=http://127.0.0.1:9009/{prometheus_http_prefix}`.
By default, Cortex uses the `prometheus` path prefix, so you should define the flag `--remote-read-src-addr=http://127.0.0.1:9009/prometheus`.

By default, Cortex exposes HTTP port on `:9009 `. The importing process example for the local installation of Cortex
and single-node VictoriaMetrics(`http://localhost:8428`):

```sh
./vmctl remote-read \ 
--remote-read-src-addr=http://127.0.0.1:9009/prometheus \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--vm-addr=http://127.0.0.1:8428 \
```

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

When the process finishes, you will see the following:

```sh
Split defined times into 8842 ranges to import. Continue? [Y/n]
VM worker 0:↗ 3863 samples/s
VM worker 1:↗ 2686 samples/s
VM worker 2:↗ 2620 samples/s
VM worker 3:↗ 2705 samples/s
VM worker 4:↗ 2643 samples/s
VM worker 5:↗ 2593 samples/s
Processing ranges: 8842 / 8842 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/21 12:09:49 Import finished!
2022/10/21 12:09:49 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 3.82640757s;
  total samples: 160232;
  samples/s: 41875.31;
  total bytes: 11.3 MB;
  bytes/s: 3.0 MB;
  import requests: 6;
  import requests retries: 0;
2022/10/21 12:09:49 Total time: 4.71824253s
```

## Configuration

If you run Cortex installation in multi-tenant mode, remote read protocol requires an Authentication header like `X-Scope-OrgID`.
You can define it via the flag `--remote-read-headers=X-Scope-OrgID:demo`.

See [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) for more details.

See also general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).