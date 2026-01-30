---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file should not be updated manually. Run make docs-update-flags while preparing a new release to sync flags in docs from actual binaries. -->
```shellhelp
NAME:
   vmctl influx - Migrate time series from InfluxDB

USAGE:
   vmctl influx [command options]

OPTIONS:
   -s                                                                 Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
   --verbose                                                          Whether to enable verbosity in logs output. (default: false)
   --disable-progress-bar                                             Whether to disable progress bar during the import. (default: false)
   --pushmetrics.url value [ --pushmetrics.url value ]                Optional URL to push metrics. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics
   --pushmetrics.interval value                                       Interval for pushing metrics to every -pushmetrics.url (default: 10s)
   --pushmetrics.extraLabel value [ --pushmetrics.extraLabel value ]  Extra labels to add to pushed metrics. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels. For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
   --pushmetrics.header value [ --pushmetrics.header value ]          Optional HTTP headers to add to pushed metrics. Flag can be set multiple times, to add few additional headers.
   --pushmetrics.disableCompression                                   Whether to disable compression when pushing metrics. (default: false)
   --influx-addr value                                                InfluxDB server addr (default: "http://localhost:8086")
   --influx-user value                                                InfluxDB user [$INFLUX_USERNAME]
   --influx-password value                                            InfluxDB user password [$INFLUX_PASSWORD]
   --influx-database value                                            InfluxDB database
   --influx-retention-policy value                                    InfluxDB retention policy (default: "autogen")
   --influx-chunk-size value                                          The chunkSize defines max amount of series to be returned in one chunk (default: 10000)
   --influx-concurrency value                                         Number of concurrently running fetch queries to InfluxDB (default: 1)
   --influx-filter-series value                                       InfluxDB filter expression to select series. E.g. "from cpu where arch='x86' AND hostname='host_2753'".
      See for details https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#show-series
   --influx-filter-time-start value            The time filter to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --influx-filter-time-end value              The time filter to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --influx-measurement-field-separator value  The {separator} symbol used to concatenate {measurement} and {field} names into series name {measurement}{separator}{field}. (default: "_")
   --influx-skip-database-label                Whether to skip adding the label 'db' to timeseries. (default: false)
   --influx-prometheus-mode                    Whether to restore the original timeseries name previously written from Prometheus to InfluxDB v1 via remote_write. (default: false)
   --influx-cert-file value                    Optional path to client-side TLS certificate file to use when connecting to -influx-addr
   --influx-key-file value                     Optional path to client-side TLS key to use when connecting to -influx-addr
   --influx-CA-file value                      Optional path to TLS CA file to use for verifying connections to -influx-addr. By default, system CA is used
   --influx-server-name value                  Optional TLS server name to use for connections to -influx-addr. By default, the server name from -influx-addr is used
   --influx-insecure-skip-verify               Whether to skip tls verification when connecting to -influx-addr (default: false)
   --vm-addr value                             VictoriaMetrics address to perform import requests. 
      Should be the same as --httpListenAddr value for single-node version or vminsert component. 
      When importing into the clustered version do not forget to set additionally --vm-account-id flag. 
      Please note, that vmctl performs initial readiness check for the given address by checking /health endpoint. (default: "http://localhost:8428")
   --vm-user value        VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value    VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value  AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). 
      AccountID is required when importing into the clustered version of VictoriaMetrics. 
      It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. 
      If projectID isn't set, then it equals to 0
   --vm-concurrency value                             Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                                      Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value                              How many samples importer collects before sending the import request to VM (default: 200000)
   --vm-significant-figures value                     The number of significant figures to leave in metric values before importing. See https://en.wikipedia.org/wiki/Significant_figures. Zero value saves all the significant figures. This option may be used for increasing on-disk compression level for the stored metrics. See also --vm-round-digits option (default: 0)
   --vm-round-digits value                            Round metric values to the given number of decimal digits after the point. This option may be used for increasing on-disk compression level for the stored metrics (default: 100)
   --vm-extra-label value [ --vm-extra-label value ]  Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels.
   --vm-rate-limit value                              Optional data transfer rate limit in bytes per second.
      By default, the rate limit is disabled. It can be useful for limiting load on configured via '--vm-addr' destination. (default: 0)
   --vm-cert-file value             Optional path to client-side TLS certificate file to use when connecting to '--vm-addr'
   --vm-key-file value              Optional path to client-side TLS key to use when connecting to '--vm-addr'
   --vm-CA-file value               Optional path to TLS CA file to use for verifying connections to '--vm-addr'. By default, system CA is used
   --vm-server-name value           Optional TLS server name to use for connections to '--vm-addr'. By default, the server name from '--vm-addr' is used
   --vm-insecure-skip-verify        Whether to skip tls verification when connecting to '--vm-addr' (default: false)
   --vm-backoff-retries value       How many import retries to perform before giving up. (default: 10)
   --vm-backoff-factor value        Factor to multiply the base duration after each failed import retry. Must be greater than 1.0 (default: 1.8)
   --vm-backoff-min-duration value  Minimum duration to wait before the first import retry. Each subsequent import retry will be multiplied by the '--vm-backoff-factor'. (default: 2s)
   --help, -h                       show help
```
