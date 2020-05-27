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

Then configure `vmalert` accordingly:
```
./bin/vmalert -rule=alert.rules \
		-datasource.url=http://localhost:8428 \
        -notifier.url=http://localhost:9093
```

Example for `.rules` file may be found [here](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmalert/testdata)

`vmalert` may be configured with `-remoteWrite` flag to write recording rules and 
alerts state in form of timeseries via remote write protocol. Alerts state will be written 
as `ALERTS` timeseries. These timeseries may be used to recover alerts state on `vmalert` 
restarts if `-remoteRead` is configured.

`vmalert` runs evaluation for every group in a separate goroutine.
Rules in group evaluated one-by-one sequentially. 

**Important:** while recording rules execution is sequential, writing of timeseries results to remote
storage is asynchronous. Hence, user shouldn't rely on recording chaining when result of pervious
recording rule is reused in next one.

`vmalert` also runs a web-server (`-httpListenAddr`) for serving metrics and alerts endpoints:
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
  -notifier.url string
        Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093
  -remoteRead.basicAuth.password string
        Optional basic auth password for -remoteRead.url
  -remoteRead.basicAuth.username string
        Optional basic auth username for -remoteRead.url
  -remoteRead.lookback duration
        Lookback defines how far to look into past for alerts timeseries. For example, if lookback=1h then range from now() to now()-1h will be scanned. (default 1h0m0s)
  -remoteRead.url vmalert
        Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts state. This configuration makes sense only if vmalert was configured with `remoteWrite.url` before and has been successfully persisted its state. E.g. http://127.0.0.1:8428
  -remoteWrite.basicAuth.password string
        Optional basic auth password for -remoteWrite.url
  -remoteWrite.basicAuth.username string
        Optional basic auth username for -remoteWrite.url
  -remoteWrite.concurrency int
        Defines number of readers that concurrently write into remote storage (default 1)
  -remoteWrite.maxBatchSize int
        Defines defines max number of timeseries to be flushed at once (default 1000)
  -remoteWrite.maxQueueSize int
        Defines the max number of pending datapoints to remote write endpoint (default 100000)
  -remoteWrite.url string
        Optional URL to Victoria Metrics or VMInsert where to persist alerts state in form of timeseries. E.g. http://127.0.0.1:8428
  -rule value
        Path to the file with alert rules. 
        Supports patterns. Flag can be specified multiple times. 
        Examples:
         -rule /path/to/file. Path to a single file with alerting rules
         -rule dir/*.yaml -rule /*.yaml. Relative path to all .yaml files in "dir" folder, 
        absolute path to all .yaml files in root.
  -rule.validateTemplates
        Indicates to validate annotation and label templates (default true)
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
