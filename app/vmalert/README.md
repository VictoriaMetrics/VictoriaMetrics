## VM Alert

`vmalert` executes a list of given MetricsQL expressions (rules) and
sends alerts to [Alert Manager](https://github.com/prometheus/alertmanager).   

NOTE: `vmalert` is in early alpha and wasn't tested in production systems yet.

### Features:
* Integration with [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) TSDB;
* VictoriaMetrics [MetricsQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL)
 expressions validation;
* Prometheus [alerting rules definition format](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules)
 support;
* Integration with [Alertmanager](https://github.com/prometheus/alertmanager);
* Lightweight without extra dependencies.

### TODO:
* Persist alerts state as timeseries in TSDB. Currently, alerts state is stored
in process memory only and will be lost on restart;
* Configuration hot reload.

### QuickStart

To build `vmalert` from sources:
```
git clone https://github.com/VictoriaMetrics/VictoriaMetrics
cd VictoriaMetrics
make vmalert
```
The build binary will be placed to `VictoriaMetrics/bin` folder.

To start using `vmalert` you will need the following things:
* list of alert rules - PromQL/MetricsQL expressions to execute;
* datasource address - reachable VictoriaMetrics instance for rules execution;
* notifier address - reachable Alertmanager instance for processing, 
aggregating alerts and sending notifications.

Then configure `vmalert` accordingly:
```
./bin/vmalert -rule=alert.rules \
		-datasource.url=http://localhost:8428 \
        -notifier.url=http://localhost:9093
```

Example for `.rules` file may be found [here](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmalert/testdata/rules0-good.rules)

`vmalert` runs evaluation for every group in a separate goroutine.
Rules in group evaluated one-by-one sequentially. 

`vmalert` also runs a web-server (`-httpListenAddr`) for serving metrics and alerts endpoints:
* `http://<vmalert-addr>/api/v1/alerts` - list of all active alerts;
* `http://<vmalert-addr>/api/v1/<groupName>/<alertID>/status" ` - get alert status by ID.
Used as alert source in AlertManager.
* `http://<vmalert-addr>/metrics` - application metrics.

### Configuration

The shortlist of configuration flags is the following:
```
Usage of vmalert:
  -datasource.url string
        Victoria Metrics or VMSelect url. Required parameter. e.g. http://127.0.0.1:8428
  -datasource.basicAuth.password string
        Optional basic auth password to use for -datasource.url
  -datasource.basicAuth.username string
        Optional basic auth username to use for -datasource.url
  -evaluationInterval duration
        How often to evaluate the rules. Default 1m (default 1m0s)
  -external.url string
        External URL is used as alert's source for sent alerts to the notifier
  -httpListenAddr string
        Address to listen for http connections (default ":8880")
  -notifier.url string
        Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093
  -remotewrite.url string
        Optional URL to remote-write compatible storage where to write timeseriesbased on active alerts. E.g. http://127.0.0.1:8428
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

### Contributing

`vmalert` is mostly designed and built by VictoriaMetrics community.
Feel free to share your experience and ideas for improving this 
software. Please keep simplicity as the main priority.