---
title: Promscale
weight: 7
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-promscale" 
    weight: 7
---

[Promscale](https://github.com/timescale/promscale) supports [Prometheus Remote Read API](https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/).
To migrate historical data from Promscale to VictoriaMetrics use `vmctl` in [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/).

See the example of migration command below:
```sh
./vmctl remote-read \
  --remote-read-src-addr=http://<promscale>:9201/read \
  --remote-read-step-interval=day \
  --vm-addr=http://<victoriametrics>:8428 \
  --remote-read-filter-time-start=2023-08-21T00:00:00Z \
  --remote-read-disable-path-append=true # promscale has custom remote read API HTTP path
Selected time range "2023-08-21 00:00:00 +0000 UTC" - "2023-08-21 14:11:41.561979 +0000 UTC" will be split into 1 ranges according to "day" step. Continue? [Y/n] y
VM worker 0:↙ 82831 samples/s                                                                                                                                                                        
VM worker 1:↙ 54378 samples/s                                                                                                                                                                        
VM worker 2:↙ 121616 samples/s                                                                                                                                                                       
VM worker 3:↙ 59164 samples/s                                                                                                                                                                        
VM worker 4:↙ 59220 samples/s                                                                                                                                                                        
VM worker 5:↙ 102072 samples/s                                                                                                                                                                       
Processing ranges: 1 / 1 [████████████████████████████████████████████████████████████████████████████████████] 100.00%
2023/08/21 16:11:55 Import finished!
2023/08/21 16:11:55 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 14.047045459s;
  total samples: 262111;
  samples/s: 18659.51;
  total bytes: 5.3 MB;
  bytes/s: 376.4 kB;
  import requests: 6;
  import requests retries: 0;
2023/08/21 16:11:55 Total time: 14.063458792s
```

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

Here we specify the full path to Promscale's Remote Read API via `--remote-read-src-addr`, and disable auto-path
appending via `--remote-read-disable-path-append` cmd-line flags. This is necessary, as Promscale has a different to
Prometheus API path. Promscale doesn't support stream mode for Remote Read API,
so we disable it via `--remote-read-use-stream=false`.

## Configuration

See [remote-read mode](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/) for more details.

See also general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).