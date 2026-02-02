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
   vmctl remote-read - Migrate time series via Prometheus remote-read protocol

USAGE:
   vmctl remote-read [command options]

OPTIONS:
   -s                                                                                 Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
   --verbose                                                                          Whether to enable verbosity in logs output. (default: false)
   --disable-progress-bar                                                             Whether to disable progress bar during the import. (default: false)
   --pushmetrics.url value [ --pushmetrics.url value ]                                Optional URL to push metrics. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics
   --pushmetrics.interval value                                                       Interval for pushing metrics to every -pushmetrics.url (default: 10s)
   --pushmetrics.extraLabel value [ --pushmetrics.extraLabel value ]                  Extra labels to add to pushed metrics. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels. For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
   --pushmetrics.header value [ --pushmetrics.header value ]                          Optional HTTP headers to add to pushed metrics. Flag can be set multiple times, to add few additional headers.
   --pushmetrics.disableCompression                                                   Whether to disable compression when pushing metrics. (default: false)
   --remote-read-concurrency value                                                    Number of concurrently running remote read readers (default: 1)
   --remote-read-filter-time-start value                                              The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --remote-read-filter-time-end value                                                The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --remote-read-filter-label value [ --remote-read-filter-label value ]              Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name. (default: __name__)
   --remote-read-filter-label-value value [ --remote-read-filter-label-value value ]  Prometheus regular expression to filter label from "remote-read-filter-label-value" flag. (default: .*)
   --remote-read                                                                      Use Prometheus remote read protocol (default: false)
   --remote-read-use-stream                                                           Defines whether to use SAMPLES or STREAMED_XOR_CHUNKS mode. By default, is uses SAMPLES mode. See https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#streamed-chunks (default: false)
   --remote-read-step-interval value                                                  The time interval to split the migration into steps. For example, to migrate 1y of data with '--remote-read-step-interval=month' vmctl will execute it in 12 separate requests from the beginning of the time range to its end. To reverse the order use '--remote-read-filter-time-reverse'. Requires setting '--remote-read-filter-time-start'. Valid values are 'month','week','day','hour','minute'.
   --remote-read-filter-time-reverse                                                  Whether to reverse the order of time intervals split by '--remote-read-step-interval' cmd-line flag. When set, the migration will start from the newest to the oldest data. (default: false)
   --remote-read-src-addr value                                                       Remote read address to perform read from.
   --remote-read-user value                                                           Remote read username for basic auth [$REMOTE_READ_USERNAME]
   --remote-read-password value                                                       Remote read password for basic auth [$REMOTE_READ_PASSWORD]
   --remote-read-http-timeout value                                                   Timeout defines timeout for HTTP requests made by remote read client (default: 0s)
   --remote-read-headers value                                                        Optional HTTP headers to send with each request to the corresponding remote source storage 
      For example, --remote-read-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding remote source storage. 
      Multiple headers must be delimited by '^^': --remote-read-headers='header1:value1^^header2:value2'
   --remote-read-cert-file value       Optional path to client-side TLS certificate file to use when connecting to -remote-read-src-addr
   --remote-read-key-file value        Optional path to client-side TLS key to use when connecting to -remote-read-src-addr
   --remote-read-CA-file value         Optional path to TLS CA file to use for verifying connections to -remote-read-src-addr. By default, system CA is used
   --remote-read-server-name value     Optional TLS server name to use for connections to remoteReadSrcAddr. By default, the server name from -remote-read-src-addr is used
   --remote-read-insecure-skip-verify  Whether to skip TLS certificate verification when connecting to the remote read address (default: false)
   --remote-read-disable-path-append   Whether to disable automatic appending of the /api/v1/read suffix to --remote-read-src-addr (default: false)
   --vm-addr value                     VictoriaMetrics address to perform import requests. 
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
