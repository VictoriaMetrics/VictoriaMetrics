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
   vmctl opentsdb - Migrate time series from OpenTSDB

USAGE:
   vmctl opentsdb [command options]

OPTIONS:
   -s                                                                 Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
   --verbose                                                          Whether to enable verbosity in logs output. (default: false)
   --disable-progress-bar                                             Whether to disable progress bar during the import. (default: false)
   --pushmetrics.url value [ --pushmetrics.url value ]                Optional URL to push metrics. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics
   --pushmetrics.interval value                                       Interval for pushing metrics to every -pushmetrics.url (default: 10s)
   --pushmetrics.extraLabel value [ --pushmetrics.extraLabel value ]  Extra labels to add to pushed metrics. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels. For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
   --pushmetrics.header value [ --pushmetrics.header value ]          Optional HTTP headers to add to pushed metrics. Flag can be set multiple times, to add few additional headers.
   --pushmetrics.disableCompression                                   Whether to disable compression when pushing metrics. (default: false)
   --otsdb-addr value                                                 OpenTSDB server addr (default: "http://localhost:4242")
   --otsdb-concurrency value                                          Number of concurrently running fetch queries to OpenTSDB per metric (default: 1)
   --otsdb-retentions value [ --otsdb-retentions value ]              Retentions patterns to collect on. Each pattern should describe the aggregation performed for the query, the row size (in HBase) that will define how long each individual query is, and the time range to query for. e.g. sum-1m-avg:1h:3d. The first time range defined should be a multiple of the row size in HBase. e.g. if the row size is 2 hours, 4h is good, 5h less so. We want each query to land on unique rows.
   --otsdb-filters value [ --otsdb-filters value ]                    Filters to process for discovering metrics in OpenTSDB (default: "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z")
   --otsdb-offset-days value                                          Days to offset our 'starting' point for collecting data from OpenTSDB (default: 0)
   --otsdb-hard-ts-start value                                        A specific timestamp to start from, will override using an offset (default: 0)
   --otsdb-query-limit value                                          Result limit on meta queries to OpenTSDB (affects both metric name and tag value queries, recommended to use a value exceeding your largest series) (default: 100000000)
   --otsdb-msecstime                                                  Whether OpenTSDB is writing values in milliseconds or seconds (default: false)
   --otsdb-normalize                                                  Whether to normalize all data received to lower case before forwarding to VictoriaMetrics (default: false)
   --otsdb-cert-file value                                            Optional path to client-side TLS certificate file to use when connecting to -otsdb-addr
   --otsdb-key-file value                                             Optional path to client-side TLS key to use when connecting to -otsdb-addr
   --otsdb-CA-file value                                              Optional path to TLS CA file to use for verifying connections to -otsdb-addr. By default, system CA is used
   --otsdb-server-name value                                          Optional TLS server name to use for connections to -otsdb-addr. By default, the server name from -otsdb-addr is used
   --otsdb-insecure-skip-verify                                       Whether to skip tls verification when connecting to -otsdb-addr (default: false)
   --vm-addr value                                                    VictoriaMetrics address to perform import requests. 
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
