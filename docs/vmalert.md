## vmalert

`vmalert` executes a list of given [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
or [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
rules against configured address.

### Features:
* Integration with [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) TSDB;
* VictoriaMetrics [MetricsQL](https://victoriametrics.github.io/MetricsQL.html)
 support and expressions validation;
* Prometheus [alerting rules definition format](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules)
 support;
* Integration with [Alertmanager](https://github.com/prometheus/alertmanager);
* Keeps the alerts [state on restarts](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmalert#alerts-state-on-restarts);
* Graphite datasource can be used for alerting and recording rules. See [these docs](#graphite) for details.
* Lightweight without extra dependencies.

### Limitations:
* `vmalert` execute queries against remote datasource which has reliability risks because of network.
It is recommended to configure alerts thresholds and rules expressions with understanding that network request
may fail;
* by default, rules execution is sequential within one group, but persisting of execution results to remote
storage is asynchronous. Hence, user shouldn't rely on recording rules chaining when result of previous
recording rule is reused in next one;
* `vmalert` has no UI, just an API for getting groups and rules statuses.

### QuickStart

To build `vmalert` from sources:
```
git clone https://github.com/VictoriaMetrics/VictoriaMetrics
cd VictoriaMetrics
make vmalert
```
The build binary will be placed to `VictoriaMetrics/bin` folder.

To start using `vmalert` you will need the following things:
* list of rules - PromQL/MetricsQL expressions to execute;
* datasource address - reachable VictoriaMetrics instance for rules execution;
* notifier address - reachable [Alert Manager](https://github.com/prometheus/alertmanager) instance for processing,
aggregating alerts and sending notifications.
* remote write address - [remote write](https://prometheus.io/docs/prometheus/latest/storage/#remote-storage-integrations)
compatible storage address for storing recording rules results and alerts state in for of timeseries. This is optional.

Then configure `vmalert` accordingly:
```
./bin/vmalert -rule=alert.rules \
    -datasource.url=http://localhost:8428 \  # PromQL compatible datasource
    -notifier.url=http://localhost:9093 \    # AlertManager URL
    -notifier.url=http://127.0.0.1:9093 \    # AlertManager replica URL
    -remoteWrite.url=http://localhost:8428 \ # remote write compatible storage to persist rules
    -remoteRead.url=http://localhost:8428 \  # PromQL compatible datasource to restore alerts state from
    -external.label=cluster=east-1 \         # External label to be applied for each rule
    -external.label=replica=a \              # Multiple external labels may be set
    -evaluationInterval=3s                   # Default evaluation interval if not specified in rules group
```

If you run multiple `vmalert` services for the same datastore or AlertManager - do not forget
to specify different `external.label` flags in order to define which `vmalert` generated rules or alerts.

Configuration for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules is very
similar to Prometheus rules and configured using YAML. Configuration examples may be found
in [testdata](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmalert/config/testdata) folder.
Every `rule` belongs to `group` and every configuration file may contain arbitrary number of groups:
```yaml
groups:
  [ - <rule_group> ]
```

#### Groups

Each group has following attributes:
```yaml
# The name of the group. Must be unique within a file.
name: <string>

# How often rules in the group are evaluated.
[ interval: <duration> | default = global.evaluation_interval ]

# How many rules execute at once. Increasing concurrency may speed
# up round execution speed.
[ concurrency: <integer> | default = 1 ]

# Optional type for expressions inside the rules. Supported values: "graphite" and "prometheus".
# By default "prometheus" rule type is used.
[ type: <string> ]

rules:
  [ - <rule> ... ]
```

#### Rules

There are two types of Rules:
* [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) -
Alerting rules allows to define alert conditions via [MetricsQL](https://victoriametrics.github.io/MetricsQL.html)
and to send notifications about firing alerts to [Alertmanager](https://github.com/prometheus/alertmanager).
* [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) -
Recording rules allow you to precompute frequently needed or computationally expensive expressions
and save their result as a new set of time series.

`vmalert` forbids to define duplicates - rules with the same combination of name, expression and labels
within one group.

##### Alerting rules

The syntax for alerting rule is following:
```yaml
# The name of the alert. Must be a valid metric name.
alert: <string>

# Optional type for the rule. Supported values: "graphite", "prometheus".
# By default "prometheus" rule type is used.
[ type: <string> ]

# The expression to evaluate. The expression language depends on the type value.
# By default MetricsQL expression is used. If type="graphite", then the expression
# must contain valid Graphite expression.
expr: <string>

# Alerts are considered firing once they have been returned for this long.
# Alerts which have not yet fired for long enough are considered pending.
[ for: <duration> | default = 0s ]

# Labels to add or overwrite for each alert.
labels:
  [ <labelname>: <tmpl_string> ]

# Annotations to add to each alert.
annotations:
  [ <labelname>: <tmpl_string> ]
```

##### Recording rules

The syntax for recording rules is following:
```yaml
# The name of the time series to output to. Must be a valid metric name.
record: <string>

# Optional type for the rule. Supported values: "graphite", "prometheus".
# By default "prometheus" rule type is used.
[ type: <string> ]

# The expression to evaluate. The expression language depends on the type value.
# By default MetricsQL expression is used. If type="graphite", then the expression
# must contain valid Graphite expression.
expr: <string>

# Labels to add or overwrite before storing the result.
labels:
  [ <labelname>: <labelvalue> ]
```

For recording rules to work `-remoteWrite.url` must specified.


#### Alerts state on restarts

`vmalert` has no local storage, so alerts state is stored in the process memory. Hence, after reloading of `vmalert`
the process alerts state will be lost. To avoid this situation, `vmalert` should be configured via the following flags:
* `-remoteWrite.url` - URL to VictoriaMetrics (Single) or VMInsert (Cluster). `vmalert` will persist alerts state
into the configured address in the form of time series named `ALERTS` and `ALERTS_FOR_STATE` via remote-write protocol.
These are regular time series and may be queried from VM just as any other time series.
The state stored to the configured address on every rule evaluation.
* `-remoteRead.url` - URL to VictoriaMetrics (Single) or VMSelect (Cluster). `vmalert` will try to restore alerts state
from configured address by querying time series with name `ALERTS_FOR_STATE`.

Both flags are required for the proper state restoring. Restore process may fail if time series are missing
in configured `-remoteRead.url`, weren't updated in the last `1h` or received state doesn't match current `vmalert`
rules configuration.


#### WEB

`vmalert` runs a web-server (`-httpListenAddr`) for serving metrics and alerts endpoints:
* `http://<vmalert-addr>/api/v1/groups` - list of all loaded groups and rules;
* `http://<vmalert-addr>/api/v1/alerts` - list of all active alerts;
* `http://<vmalert-addr>/api/v1/<groupID>/<alertID>/status" ` - get alert status by ID.
Used as alert source in AlertManager.
* `http://<vmalert-addr>/metrics` - application metrics.
* `http://<vmalert-addr>/-/reload` - hot configuration reload.


### Graphite

vmalert sends requests to `<-datasource.url>/render?format=json` during evaluation of alerting and recording rules
if the corresponding group or rule contains `type: "graphite"` config option. It is expected that the `<-datasource.url>/render`
implements [Graphite Render API](https://graphite.readthedocs.io/en/stable/render_api.html) for `format=json`.
When using vmalert with both `graphite` and `prometheus` rules configured against cluster version of VM do not forget
to set `-datasource.appendTypePrefix` flag to `true`, so vmalert can adjust URL prefix automatically based on query type.


### Configuration

The shortlist of configuration flags is the following:
```
  -datasource.appendTypePrefix
    	Whether to add type prefix to -datasource.url based on the query type. Set to true if sending different query types to VMSelect URL.
  -datasource.basicAuth.password string
    	Optional basic auth password for -datasource.url
  -datasource.basicAuth.username string
    	Optional basic auth username for -datasource.url
  -datasource.lookback duration
    	Lookback defines how far to look into past when evaluating queries. For example, if datasource.lookback=5m then param "time" with value now()-5m will be added to every query.
  -datasource.maxIdleConnections int
    	Defines the number of idle (keep-alive connections) to configured datasource.Consider to set this value equal to the value: groups_total * group.concurrency. Too low value may result into high number of sockets in TIME_WAIT state. (default 100)
  -datasource.queryStep duration
    	queryStep defines how far a value can fallback to when evaluating queries. For example, if datasource.queryStep=15s then param "step" with value "15s" will be added to every query.
  -datasource.tlsCAFile string
    	Optional path to TLS CA file to use for verifying connections to -datasource.url. By default system CA is used
  -datasource.tlsCertFile string
    	Optional path to client-side TLS certificate file to use when connecting to -datasource.url
  -datasource.tlsInsecureSkipVerify
    	Whether to skip tls verification when connecting to -datasource.url
  -datasource.tlsKeyFile string
    	Optional path to client-side TLS certificate key to use when connecting to -datasource.url
  -datasource.tlsServerName string
    	Optional TLS server name to use for connections to -datasource.url. By default the server name from -datasource.url is used
  -datasource.url string
    	Victoria Metrics or VMSelect url. Required parameter. E.g. http://127.0.0.1:8428
  -dryRun -rule
    	Whether to check only config files without running vmalert. The rules file are validated. The -rule flag must be specified.
  -enableTCP6
    	Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP is used
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -evaluationInterval duration
    	How often to evaluate the rules (default 1m0s)
  -external.alert.source string
    	External Alert Source allows to override the Source link for alerts sent to AlertManager for cases where you want to build a custom link to Grafana, Prometheus or any other service.
    	eg. 'explore?orgId=1&left=[\"now-1h\",\"now\",\"VictoriaMetrics\",{\"expr\": \"{{$expr|quotesEscape|crlfEscape|queryEscape}}\"},{\"mode\":\"Metrics\"},{\"ui\":[true,true,true,\"none\"]}]'.If empty '/api/v1/:groupID/alertID/status' is used
  -external.label array
    	Optional label in the form 'name=value' to add to all generated recording rules and alerts. Pass multiple -label flags in order to add multiple label sets.
    	Supports array of values separated by comma or specified via multiple flags.
  -external.url string
    	External URL is used as alert's source for sent alerts to the notifier
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
    	Incoming http connections are closed after the configured timeout. This may help spreading incoming load among a cluster of services behind load balancer. Note that the real timeout may be bigger by up to 10% as a protection from Thundering herd problem (default 2m0s)
  -http.disableResponseCompression
    	Disable compression of HTTP responses for saving CPU resources. By default compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
    	Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
    	The maximum duration for graceful shutdown of HTTP server. Highly loaded server may require increased value for graceful shutdown (default 7s)
  -http.pathPrefix string
    	An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
    	Optional delay before http server shutdown. During this dealy the servier returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
    	Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
    	Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
    	Address to listen for http connections (default ":8880")
  -loggerDisableTimestamps
    	Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
    	Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, then the remaining errors are suppressed. Zero value disables the rate limit
  -loggerFormat string
    	Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
    	Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
    	Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
    	Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero value disables the rate limit
  -memory.allowedBytes value
    	Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to non-zero value. Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. Too high value may evict too much data from OS page cache, which will result in higher disk IO usage
    	Supports the following optional suffixes for values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. Too high value may evict too much data from OS page cache, which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
    	Auth key for /metrics. It overrides httpAuth settings
  -notifier.basicAuth.password array
    	Optional basic auth password for -notifier.url
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.basicAuth.username array
    	Optional basic auth username for -notifier.url
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.tlsCAFile array
    	Optional path to TLS CA file to use for verifying connections to -notifier.url. By default system CA is used
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.tlsCertFile array
    	Optional path to client-side TLS certificate file to use when connecting to -notifier.url
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.tlsInsecureSkipVerify array
    	Whether to skip tls verification when connecting to -notifier.url
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.tlsKeyFile array
    	Optional path to client-side TLS certificate key to use when connecting to -notifier.url
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.tlsServerName array
    	Optional TLS server name to use for connections to -notifier.url. By default the server name from -notifier.url is used
    	Supports array of values separated by comma or specified via multiple flags.
  -notifier.url array
    	Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093
    	Supports array of values separated by comma or specified via multiple flags.
  -pprofAuthKey string
    	Auth key for /debug/pprof. It overrides httpAuth settings
  -remoteRead.basicAuth.password string
    	Optional basic auth password for -remoteRead.url
  -remoteRead.basicAuth.username string
    	Optional basic auth username for -remoteRead.url
  -remoteRead.lookback duration
    	Lookback defines how far to look into past for alerts timeseries. For example, if lookback=1h then range from now() to now()-1h will be scanned. (default 1h0m0s)
  -remoteRead.tlsCAFile string
    	Optional path to TLS CA file to use for verifying connections to -remoteRead.url. By default system CA is used
  -remoteRead.tlsCertFile string
    	Optional path to client-side TLS certificate file to use when connecting to -remoteRead.url
  -remoteRead.tlsInsecureSkipVerify
    	Whether to skip tls verification when connecting to -remoteRead.url
  -remoteRead.tlsKeyFile string
    	Optional path to client-side TLS certificate key to use when connecting to -remoteRead.url
  -remoteRead.tlsServerName string
    	Optional TLS server name to use for connections to -remoteRead.url. By default the server name from -remoteRead.url is used
  -remoteRead.url vmalert
    	Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts state. This configuration makes sense only if vmalert was configured with `remoteWrite.url` before and has been successfully persisted its state. E.g. http://127.0.0.1:8428
  -remoteWrite.basicAuth.password string
    	Optional basic auth password for -remoteWrite.url
  -remoteWrite.basicAuth.username string
    	Optional basic auth username for -remoteWrite.url
  -remoteWrite.concurrency int
    	Defines number of writers for concurrent writing into remote querier (default 1)
  -remoteWrite.flushInterval duration
    	Defines interval of flushes to remote write endpoint (default 5s)
  -remoteWrite.maxBatchSize int
    	Defines defines max number of timeseries to be flushed at once (default 1000)
  -remoteWrite.maxQueueSize int
    	Defines the max number of pending datapoints to remote write endpoint (default 100000)
  -remoteWrite.tlsCAFile string
    	Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. By default system CA is used
  -remoteWrite.tlsCertFile string
    	Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url
  -remoteWrite.tlsInsecureSkipVerify
    	Whether to skip tls verification when connecting to -remoteWrite.url
  -remoteWrite.tlsKeyFile string
    	Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url
  -remoteWrite.tlsServerName string
    	Optional TLS server name to use for connections to -remoteWrite.url. By default the server name from -remoteWrite.url is used
  -remoteWrite.url string
    	Optional URL to Victoria Metrics or VMInsert where to persist alerts state and recording rules results in form of timeseries. E.g. http://127.0.0.1:8428
  -rule array
    	Path to the file with alert rules.
    	Supports patterns. Flag can be specified multiple times.
    	Examples:
    	 -rule="/path/to/file". Path to a single file with alerting rules
    	 -rule="dir/*.yaml" -rule="/*.yaml". Relative path to all .yaml files in "dir" folder,
    	absolute path to all .yaml files in root.
    	Rule files may contain %{ENV_VAR} placeholders, which are substituted by the corresponding env vars.
    	Supports array of values separated by comma or specified via multiple flags.
  -rule.validateExpressions
    	Whether to validate rules expressions via MetricsQL engine (default true)
  -rule.validateTemplates
    	Whether to validate annotation and label templates (default true)
  -tls
    	Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
    	Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs, since RSA certs are slow
  -tlsKeyFile string
    	Path to file with TLS key. Used only if -tls is set
  -version
    	Show VictoriaMetrics version
```

Pass `-help` to `vmalert` in order to see the full list of supported
command-line flags with their descriptions.

To reload configuration without `vmalert` restart send SIGHUP signal
or send GET request to `/-/reload` endpoint.

### Contributing

`vmalert` is mostly designed and built by VictoriaMetrics community.
Feel free to share your experience and ideas for improving this
software. Please keep simplicity as the main priority.

### How to build from sources

It is recommended using
[binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
- `vmalert` is located in `vmutils-*` archives there.


#### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.14.
2. Run `make vmalert` from the root folder of the repository.
   It builds `vmalert` binary and puts it into the `bin` folder.

#### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmalert-prod` from the root folder of the repository.
   It builds `vmalert-prod` binary and puts it into the `bin` folder.


#### ARM build

ARM build may run on Raspberry Pi or on [energy-efficient ARM servers](https://blog.cloudflare.com/arm-takes-wing/).

#### Development ARM build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.14.
2. Run `make vmalert-arm` or `make vmalert-arm64` from the root folder of the repository.
   It builds `vmalert-arm` or `vmalert-arm64` binary respectively and puts it into the `bin` folder.

#### Production ARM build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmalert-arm-prod` or `make vmalert-arm64-prod` from the root folder of the repository.
   It builds `vmalert-arm-prod` or `vmalert-arm64-prod` binary respectively and puts it into the `bin` folder.
