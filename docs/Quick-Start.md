# Quick Start

1. Download the latest VictoriaMetrics release from [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases),
   from [Docker hub](https://hub.docker.com/r/victoriametrics/victoria-metrics/)
   or [build it from sources](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/Single-server-VictoriaMetrics#how-to-build-from-sources).

2. Run the binary or Docker image with the desired command-line flags. Pass `-help` in order to see description for all the available flags
   and their default values. Default flag values should fit the majoirty of cases. The minimum required flags to configure are:

   * `-storageDataPath` - path to directory where VictoriaMetrics stores all the data.
   * `-retentionPeriod` - data retention in months.

   For instance:

   `./victoria-metrics-prod -storageDataPath=/var/lib/victoria-metrics-data -retentionPeriod=3`

   See [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/43) in order to configure VictoriaMetrics as OS service.
   It is recommended setting up [VictoriaMetrics monitoring](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#monitoring).

3. Configure all the Prometheus instances to write data to VictoriaMetrics.
   See [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/Single-server-VictoriaMetrics#prometheus-setup).

4. Configure Grafana to query VictoriaMetrics instead of Prometheus.
   See [these instructions](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/Single-server-VictoriaMetrics#grafana-setup).


There is also [cluster version](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) and [SaaS playground](https://play.victoriametrics.com/signIn).
