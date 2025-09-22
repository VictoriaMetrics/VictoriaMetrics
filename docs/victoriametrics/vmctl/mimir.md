---
title: Mimir
weight: 8
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-mimir" 
    weight: 8
---

GrafanaLabs Mimir supports [Prometheus remote read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).
`vmctl` in [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) can be used
for historical data migration from Mimir.

By default, Mimir uses the `prometheus` path prefix so specifying the source
should be as simple as `--remote-read-src-addr=http://<mimir>:9009/prometheus`.
But if prefix was overridden via `prometheus_http_prefix`, then source address should be updated
to `--remote-read-src-addr=http://<mimir>:9009/{prometheus_http_prefix}`.

When you run Mimir, it exposes a port to serve HTTP on `8080 by default`.

Next example of the local installation was in multi-tenant mode (3 instances of Mimir) with nginx as load balancer.
Load balancer expose single port `:9090`. As you can see in the example we call `:9009` instead of `:8080` because of proxy.

The importing process example for the local installation of Mimir and single-node VictoriaMetrics(`http://localhost:8428`):

```
./vmctl remote-read 
--remote-read-src-addr=http://<mimir>:9009/prometheus \
--remote-read-filter-time-start=2021-10-18T00:00:00Z \
--remote-read-step-interval=hour \
--remote-read-headers=X-Scope-OrgID:demo \
--remote-read-use-stream=true \
--vm-addr=http://<victoria-metrics>:8428 \
```

> Mimir supports [streamed remote read API](https://prometheus.io/blog/2019/10/10/remote-read-meets-streaming/),
so it is recommended setting `--remote-read-use-stream=true` flag for better performance and resource usage.

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

And when the process finishes, you will see the following:
```sh
Split defined times into 8847 ranges to import. Continue? [Y/n]
VM worker 0:→ 12176 samples/s
VM worker 1:→ 11918 samples/s
VM worker 2:→ 11261 samples/s
VM worker 3:→ 12861 samples/s
VM worker 4:→ 11096 samples/s
VM worker 5:→ 11575 samples/s
Processing ranges: 8847 / 8847 [█████████████████████████████████████████████████████████████████████████████] 100.00%
2022/10/21 17:22:23 Import finished!
2022/10/21 17:22:23 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 15.379614356s;
  total samples: 81243;
  samples/s: 5282.51;
  total bytes: 6.1 MB;
  bytes/s: 397.8 kB;
  import requests: 6;
  import requests retries: 0;
2022/10/21 17:22:23 Total time: 16.287405248s
```

## Configuration

If you run Mimir installation in multi-tenant mode, remote read protocol requires an Authentication header like `X-Scope-OrgID`. Y
ou can define it via the flag `--remote-read-headers=X-Scope-OrgID:demo`.

See [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) for more details.

See also general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).