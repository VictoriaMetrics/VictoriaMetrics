## VM Alert

`vmalert` executes a list of given MetricsQL expressions (rules) and
sends alerts to [Alert Manager](https://github.com/prometheus/alertmanager).   

### Features:
* Integration with [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) TSDB;
* VictoriaMetrics [MetricsQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL)
 expressions validation;
* Prometheus [alerting rules definition format](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules)
 support;
* Integration with [Alertmanager](https://github.com/prometheus/alertmanager);
* Lightweight without extra dependencies.

### TODO:
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
* `http://<vmalert-addr>/-/reload` - hot configuration reload.

`vmalert` may be configured with `-remotewrite` flag to write alerts state in form of timeseries
via remote write protocol. Alerts state will be written as `ALERTS` timeseries. These timeseries
may be used to recover alerts state on `vmalert` restarts if `-remoteread` is configured.


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
  -enableTCP6
        Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP is used
  -evaluationInterval duration
        How often to evaluate the rules. Default 1m (default 1m0s)
  -external.url string
        External URL is used as alert's source for sent alerts to the notifier
  -http.maxGracefulShutdownDuration duration
        The maximum duration for graceful shutdown of HTTP server. Highly loaded server may require increased value for graceful shutdown (default 7s)
  -httpAuth.password string
        Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
        Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
        Address to listen for http connections (default ":8880")
  -notifier.url string
        Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093
  -remoteread.basicAuth.password string
        Optional basic auth password for -remoteread.url
  -remoteread.basicAuth.username string
        Optional basic auth username for -remoteread.url
  -remoteread.lookback duration
        Lookback defines how far to look into past for alerts timeseries. For example, if lookback=1h then range from now() to now()-1h will be scanned. (default 1h0m0s)
  -remoteread.url vmalert
        Optional URL to Victoria Metrics or VMSelect that will be used to restore alerts state. This configuration makes sense only if vmalert was configured with `remotewrite.url` before and has been successfully persisted its state. E.g. http://127.0.0.1:8428
  -remotewrite.basicAuth.password string
        Optional basic auth password for -remotewrite.url
  -remotewrite.basicAuth.username string
        Optional basic auth username for -remotewrite.url
  -remotewrite.url string
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
