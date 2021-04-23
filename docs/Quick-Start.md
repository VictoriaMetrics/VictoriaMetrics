---
sort: 12
---

# Quick Start

1. If you run Ubuntu please run the `snap install victoriametrics` command to install and start VictoriaMetrics. Then read [these docs](https://snapcraft.io/victoriametrics).
   Otherwise you can download the latest VictoriaMetrics release from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases),
   or [Docker hub](https://hub.docker.com/r/victoriametrics/victoria-metrics/)
   or [build it from sources](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-build-from-sources). 

2. This step isn't needed if you run VictoriaMetrics via `snap install victoriametrics` as described above.
   Otherwise, please run the binary or Docker image with your desired command-line flags. You can look at `-help` to see descriptions of all available flags
   and their default values. The default flag values should fit the majority of cases. The minimum required flags that must be configured are:

   * `-storageDataPath` - the path to directory where VictoriaMetrics stores your data.
   * `-retentionPeriod` - data retention.

   For example:

   `./victoria-metrics-prod -storageDataPath=/var/lib/victoria-metrics-data -retentionPeriod=3`

   Check [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/43) to configure VictoriaMetrics as an OS service.
   We recommended setting up [VictoriaMetrics monitoring](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#monitoring).

3. Configure either [vmagent](https://docs.victoriametrics.com/vmagent.html) or Prometheus to write data to VictoriaMetrics.
   We recommended using `vmagent` instead of Prometheus because it is more resource efficient. If you still prefer Prometheus
   see [these instructions](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prometheus-setup)
   for details on how it may be properly configured.

4. To configure Grafana to query VictoriaMetrics instead of Prometheus
   please see [these instructions](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#grafana-setup).


There is also [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) and [SaaS playground](https://play.victoriametrics.com/signIn).
