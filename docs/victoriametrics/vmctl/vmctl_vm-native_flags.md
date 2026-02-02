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
   vmctl vm-native - Migrate time series between VictoriaMetrics installations

USAGE:
   vmctl vm-native [command options]

OPTIONS:
   -s                                                                 Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
   --verbose                                                          Whether to enable verbosity in logs output. (default: false)
   --disable-progress-bar                                             Whether to disable progress bar during the import. (default: false)
   --pushmetrics.url value [ --pushmetrics.url value ]                Optional URL to push metrics. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics
   --pushmetrics.interval value                                       Interval for pushing metrics to every -pushmetrics.url (default: 10s)
   --pushmetrics.extraLabel value [ --pushmetrics.extraLabel value ]  Extra labels to add to pushed metrics. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels. For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
   --pushmetrics.header value [ --pushmetrics.header value ]          Optional HTTP headers to add to pushed metrics. Flag can be set multiple times, to add few additional headers.
   --pushmetrics.disableCompression                                   Whether to disable compression when pushing metrics. (default: false)
   --vm-native-filter-match value                                     Time series selector to match series for export. For example, select {instance!="localhost"} will match all series with "instance" label different to "localhost".
       See more details here https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format (default: "{__name__!=\"\"}")
   --vm-native-filter-time-start value  The time filter may contain different timestamp formats. See more details here https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats
   --vm-native-filter-time-end value    The time filter may contain different timestamp formats. See more details here https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats
   --vm-native-step-interval value      The time interval to split the migration into steps. For example, to migrate 1y of data with '--vm-native-step-interval=month' vmctl will execute it in 12 separate requests from the beginning of the time range to its end. To reverse the order use '--vm-native-filter-time-reverse'. Requires setting '--vm-native-filter-time-start'. Valid values are 'month','week','day','hour','minute'. (default: "month")
   --vm-native-filter-time-reverse      Whether to reverse the order of time intervals split by '--vm-native-step-interval' cmd-line flag. When set, the migration will start from the newest to the oldest data. (default: false)
   --vm-native-disable-http-keep-alive  Disable HTTP persistent connections for requests made to VictoriaMetrics components during export (default: false)
   --vm-native-src-addr value           VictoriaMetrics address to perform export from. 
       Should be the same as --httpListenAddr value for single-node version or vmselect component. If exporting from cluster version see https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format
   --vm-native-src-user value      VictoriaMetrics username for basic auth [$VM_NATIVE_SRC_USERNAME]
   --vm-native-src-password value  VictoriaMetrics password for basic auth [$VM_NATIVE_SRC_PASSWORD]
   --vm-native-src-headers value   Optional HTTP headers to send with each request to the corresponding source address. 
      For example, --vm-native-src-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding source address. 
      Multiple headers must be delimited by '^^': --vm-native-src-headers='header1:value1^^header2:value2'
   --vm-native-src-bearer-token value    Optional bearer auth token to use for the corresponding --vm-native-src-addr
   --vm-native-src-cert-file value       Optional path to client-side TLS certificate file to use when connecting to --vm-native-src-addr
   --vm-native-src-key-file value        Optional path to client-side TLS key to use when connecting to --vm-native-src-addr
   --vm-native-src-ca-file value         Optional path to TLS CA file to use for verifying connections to --vm-native-src-addr. By default, system CA is used
   --vm-native-src-server-name value     Optional TLS server name to use for connections to --vm-native-src-addr. By default, the server name from --vm-native-src-addr is used
   --vm-native-src-insecure-skip-verify  Whether to skip TLS certificate verification when connecting to --vm-native-src-addr (default: false)
   --vm-native-dst-addr value            VictoriaMetrics address to perform import to. 
       Should be the same as --httpListenAddr value for single-node version or vminsert component. If importing into cluster version see https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format
   --vm-native-dst-user value      VictoriaMetrics username for basic auth [$VM_NATIVE_DST_USERNAME]
   --vm-native-dst-password value  VictoriaMetrics password for basic auth [$VM_NATIVE_DST_PASSWORD]
   --vm-native-dst-headers value   Optional HTTP headers to send with each request to the corresponding destination address. 
      For example, --vm-native-dst-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding destination address. 
      Multiple headers must be delimited by '^^': --vm-native-dst-headers='header1:value1^^header2:value2'
   --vm-native-dst-bearer-token value                 Optional bearer auth token to use for the corresponding --vm-native-dst-addr
   --vm-native-dst-cert-file value                    Optional path to client-side TLS certificate file to use when connecting to --vm-native-dst-addr
   --vm-native-dst-key-file value                     Optional path to client-side TLS key to use when connecting to --vm-native-dst-addr
   --vm-native-dst-ca-file value                      Optional path to TLS CA file to use for verifying connections to --vm-native-dst-addr. By default, system CA is used
   --vm-native-dst-server-name value                  Optional TLS server name to use for connections to --vm-native-dst-addr. By default, the server name from --vm-native-dst-addr is used
   --vm-native-dst-insecure-skip-verify               Whether to skip TLS certificate verification when connecting to --vm-native-dst-addr (default: false)
   --vm-extra-label value [ --vm-extra-label value ]  Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flag will have priority. Flag can be set multiple times, to add few additional labels.
   --vm-rate-limit value                              Optional data transfer rate limit in bytes per second.
      By default, the rate limit is disabled. It can be useful for limiting load on source or destination databases. 
      Rate limit is applied per worker, see --vm-concurrency. (default: 0)
   --vm-intercluster  Enables cluster-to-cluster migration mode with automatic tenants data migration.
       In this mode --vm-native-src-addr flag format is: 'http://vmselect:8481/'. --vm-native-dst-addr flag format is: http://vminsert:8480/. 
       TenantID will be appended automatically after discovering tenants from src. (default: false)
   --vm-concurrency value                    Number of workers concurrently performing import requests to VM (default: 2)
   --vm-native-disable-per-metric-migration  Defines whether to disable per-metric migration and migrate all data via one connection. In this mode, vmctl makes less export/import requests, but can't provide a progress bar or retry failed requests. (default: false)
   --vm-native-disable-binary-protocol       Whether to use https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-json-line-format instead of https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-native-format API. Binary export/import API protocol implies less network and resource usage, as it transfers compressed binary data blocks. Non-binary export/import API is less efficient, but supports deduplication if it is configured on vm-native-src-addr side. (default: false)
   --vm-native-backoff-retries value         How many export/import retries to perform before giving up. (default: 10)
   --vm-native-backoff-factor value          Factor to multiply the base duration after each failed export/import retry. Must be greater than 1.0 (default: 1.8)
   --vm-native-backoff-min-duration value    Minimum duration to wait before the first export/import retry. Each subsequent export/import retry will be multiplied by the '--vm-native-backoff-factor'. (default: 2s)
   --help, -h                                show help
```
