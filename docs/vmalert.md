## vmalert

`vmalert` executes a list of given [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
or [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
rules against configured address.

### Features:
* Integration with [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) TSDB;
* VictoriaMetrics [MetricsQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL)
 support and expressions validation;
* Prometheus [alerting rules definition format](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules)
 support;
* Integration with [Alertmanager](https://github.com/prometheus/alertmanager);
* Lightweight without extra dependencies.

### Limitations:
* `vmalert` execute queries against remote datasource which has reliability risks because of network. 
It is recommended to configure alerts thresholds and rules expressions with understanding that network request
may fail;
* by default, rules execution is sequential within one group, but persisting of execution results to remote
storage is asynchronous. Hence, user shouldn't rely on recording rules chaining when result of previous
recording rule is reused in next one;
* there is no `query` function support in templates yet;
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
		-datasource.url=http://localhost:8428 \
        -notifier.url=http://localhost:9093
```

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

rules:
  [ - <rule> ... ]
```

#### Rules

There are two types of Rules:
* [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) - 
Alerting rules allows to define alert conditions via [MetricsQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL)
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

# The MetricsQL expression to evaluate.
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

`vmalert` has no local storage and alerts state is stored in process memory. Hence, after reloading of `vmalert` process
alerts state will be lost. To avoid this situation, `vmalert` may be configured via following flags:
* `-remoteWrite.url` - URL to Victoria Metrics or VMInsert. `vmalert` will persist alerts state into the configured
address in form of timeseries with name `ALERTS` via remote-write protocol.
* `-remoteRead.url` - URL to Victoria Metrics or VMSelect. `vmalert` will try to restore alerts state from configured
address by querying `ALERTS` timeseries.


##### Recording rules

The syntax for recording rules is following:
```yaml
# The name of the time series to output to. Must be a valid metric name.
record: <string>

# The MetricsQL expression to evaluate.
expr: <string>

# Labels to add or overwrite before storing the result.
labels:
  [ <labelname>: <labelvalue> ]
```

For recording rules to work `-remoteWrite.url` must specified.


#### WEB

`vmalert` runs a web-server (`-httpListenAddr`) for serving metrics and alerts endpoints:
* `http://<vmalert-addr>/api/v1/groups` - list of all loaded groups and rules;
* `http://<vmalert-addr>/api/v1/alerts` - list of all active alerts;
* `http://<vmalert-addr>/api/v1/<groupName>/<alertID>/status" ` - get alert status by ID.
Used as alert source in AlertManager.
* `http://<vmalert-addr>/metrics` - application metrics.
* `http://<vmalert-addr>/-/reload` - hot configuration reload.


### Configuration

The shortlist of configuration flags is the following:
```
Usage of vmalert:
  -datasource.basicAuth.password string
        Optional basic auth password for -datasource.url
  -datasource.basicAuth.username string
        Optional basic auth username for -datasource.url
  -datasource.tlsCAFile value
        Optional path to TLS CA file to use for verifying connections to -datasource.url. By default system CA is used.
  -datasource.tlsCertFile value
        Optional path to client-side TLS certificate file to use when connecting to -datasource.url.
  -datasource.tlsInsecureSkipVerify
        Whether to skip tls verification when connecting to -datasource.url
  -datasource.tlsKeyFile value
        Optional path to client-side TLS certificate key to use when connecting to -datasource.url.
  -datasource.tlsServerName value
        Optional TLS server name to use for connections to -datasource.url. By default the server name from -datasource.url is used.
  -datasource.url string
        Victoria Metrics or VMSelect url. Required parameter. E.g. http://127.0.0.1:8428
  -evaluationInterval duration
        How often to evaluate the rules (default 1m0s)
  -external.url string
        External URL is used as alert's source for sent alerts to the notifier
  -httpListenAddr string
        Address to listen for http connections (default ":8880")
  -metricsAuthKey string
        Auth key for /metrics. It overrides httpAuth settings
  -notifier.tlsCAFile value
        Optional path to TLS CA file to use for verifying connections to -notifier.url. By default system CA is used.
  -notifier.tlsCertFile value
        Optional path to client-side TLS certificate file to use when connecting to -notifier.url.
  -notifier.tlsInsecureSkipVerify
        Whether to skip tls verification when connecting to -notifier.url
  -notifier.tlsKeyFile value
        Optional path to client-side TLS certificate key to use when connecting to -notifier.url.
  -notifier.tlsServerName value
        Optional TLS server name to use for connections to -notifier.url. By default the server name from -notifier.url is used.
  -notifier.url string
        Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093
  -remoteRead.basicAuth.password string
        Optional basic auth password for -remoteRead.url
  -remoteRead.basicAuth.username string
        Optional basic auth username for -remoteRead.url
  -remoteRead.lookback duration
        Lookback defines how far to look into past for alerts timeseries. For example, if lookback=1h then range from now() to now()-1h will be scanned. (default 1h0m0s)
  -remoteRead.tlsCAFile value
        Optional path to TLS CA file to use for verifying connections to -remoteRead.url. By default system CA is used.
  -remoteRead.tlsCertFile value
        Optional path to client-side TLS certificate file to use when connecting to -remoteRead.url.
  -remoteRead.tlsInsecureSkipVerify
        Whether to skip tls verification when connecting to -remoteRead.url
  -remoteRead.tlsKeyFile value
        Optional path to client-side TLS certificate key to use when connecting to -remoteRead.url.
  -remoteRead.tlsServerName value
        Optional TLS server name to use for connections to -remoteRead.url. By default the server name from -remoteRead.url is used.
  -remoteRead.url vmalert
        Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts state. This configuration makes sense only if vmalert was configured with `remoteWrite.url` before and has been successfully persisted its state. E.g. http://127.0.0.1:8428
  -remoteWrite.basicAuth.password string
        Optional basic auth password for -remoteWrite.url
  -remoteWrite.basicAuth.username string
        Optional basic auth username for -remoteWrite.url
  -remoteWrite.concurrency int
        Defines number of readers that concurrently write into remote storage (default 1)
  -remoteWrite.flushInterval duration
        Defines interval of flushes to remote write endpoint (default 5s)
  -remoteWrite.maxBatchSize int
        Defines defines max number of timeseries to be flushed at once (default 1000)
  -remoteWrite.maxQueueSize int
        Defines the max number of pending datapoints to remote write endpoint (default 100000)
  -remoteWrite.tlsCAFile value
        Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. By default system CA is used.
  -remoteWrite.tlsCertFile value
        Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url.
  -remoteWrite.tlsInsecureSkipVerify
        Whether to skip tls verification when connecting to -remoteWrite.url
  -remoteWrite.tlsKeyFile value
        Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url.
  -remoteWrite.tlsServerName value
        Optional TLS server name to use for connections to -remoteWrite.url. By default the server name from -remoteWrite.url is used.
  -remoteWrite.url string
        Optional URL to Victoria Metrics or VMInsert where to persist alerts state in form of timeseries. E.g. http://127.0.0.1:8428
  -rule value
        Path to the file with alert rules. 
        Supports patterns. Flag can be specified multiple times. 
        Examples:
         -rule /path/to/file. Path to a single file with alerting rules
         -rule dir/*.yaml -rule /*.yaml. Relative path to all .yaml files in "dir" folder, 
        absolute path to all .yaml files in root.
  -rule.validateExpressions
        Whether to validate rules expressions via MetricsQL engine (default true)
  -rule.validateTemplates
        Whether to validate annotation and label templates (default true)
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

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.13.
2. Run `make vmalert` from the root folder of the repository.
   It builds `vmalert` binary and puts it into the `bin` folder.

#### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmalert-prod` from the root folder of the repository.
   It builds `vmalert-prod` binary and puts it into the `bin` folder.
