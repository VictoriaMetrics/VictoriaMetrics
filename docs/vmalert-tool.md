---
weight: 12
menu:
  docs:
    parent: victoriametrics
    weight: 12
title: vmalert-tool
aliases:
  - /vmalert-tool.html
---
VMAlert command-line tool

## Unit testing for rules

You can use `vmalert-tool` to run unit tests for alerting and recording rules.
It will perform the following actions:
* sets up an isolated VictoriaMetrics instance;
* simulates the periodic ingestion of time series;
* queries the ingested data for recording and alerting rules evaluation like [vmalert](https://docs.victoriametrics.com/vmalert/);
* checks whether the firing alerts or resulting recording rules match the expected results.

See how to run vmalert-tool for unit test below:

```
# Run vmalert-tool with one or multiple test files via `--files` cmd-line flag
# Supports file path with hierarchical patterns and regexpes, and http url.
./vmalert-tool unittest --files /path/to/file --files http://<some-server-addr>/path/to/test.yaml
```

vmalert-tool unittest is compatible with [Prometheus config format for tests](https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/#test-file-format)
except `promql_expr_test` field. Use `metricsql_expr_test` field name instead. The name is different because vmalert-tool
validates and executes [MetricsQL](https://docs.victoriametrics.com/metricsql/) expressions,
which aren't always backward compatible with [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/).

### Limitations

* vmalert-tool evaluates all the groups defined in `rule_files` using `evaluation_interval`(default `1m`) instead of `interval` under each rule group.
* vmalert-tool shares the same limitation with [vmalert](https://docs.victoriametrics.com/vmalert/#limitations) on chaining rules under one group:

>by default, rules execution is sequential within one group, but persistence of execution results to remote storage is asynchronous. Hence, user shouldn’t rely on chaining of recording rules when result of previous recording rule is reused in the next one;

For example, you have recording rule A and alerting rule B in the same group, and rule B's expression is based on A's results.
Rule B won't get the latest data of A, since data didn't persist to remote storage yet.
The workaround is to divide them in two groups and put groupA in front of groupB (or use `group_eval_order` to define the evaluation order).
In this way, vmalert-tool makes sure that the results of groupA must be written to storage before evaluating groupB:

```yaml
groups:
- name: groupA
  rules:
  - record: A
    expr: sum(xxx)
- name: groupB
  rules:
  - alert: B
    expr: A >= 0.75
    for: 1m
```

### Test file format

The configuration format for files specified in `--files` cmd-line flag is the following:

```yaml
# Path to the files or http url containing [rule groups](https://docs.victoriametrics.com/vmalert/#groups) configuration.
# Enterprise version of vmalert-tool supports S3 and GCS paths to rules.
rule_files:
  [ - <string> ]

# The evaluation interval for rules specified in `rule_files`
[ evaluation_interval: <duration> | default = 1m ]

# Groups listed below will be evaluated by order.
# Not All the groups need not be mentioned, if not, they will be evaluated by define order in rule_files.
group_eval_order:
  [ - <string> ]

# The list of unit test files to be checked during evaluation.
tests:
  [ - <test_group> ]
```

#### `<test_group>`

```yaml
# Interval between samples for input series
[ interval: <duration> | default = evaluation_interval ]
# Time series to persist into the database according to configured <interval> before running tests.
input_series:
  [ - <series> ]

# Name of the test group, optional
[ name: <string> ]

# Unit tests for alerting rules
alert_rule_test:
  [ - <alert_test_case> ]

# Unit tests for Metricsql expressions.
metricsql_expr_test:
  [ - <metricsql_expr_test> ]

# external_labels is not accessible for [templating](https://docs.victoriametrics.com/vmalert/#templating), use "-external.label" cmd-line flag instead.
# Will be deprecated soon, check https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6735 for details.
external_labels:
  [ <labelname>: <string> ... ]

```

#### `<series>`

```yaml
# series in the following format '<metric name>{<label name>=<label value>, ...}'
# Examples:
#      series_name{label1="value1", label2="value2"}
#      go_goroutines{job="prometheus", instance="localhost:9090"}
series: <string>

# values support several special equations:
#    'a+bxc' becomes 'a a+b a+(2*b) a+(3*b) … a+(c*b)'
#     Read this as series starts at a, then c further samples incrementing by b.
#    'a-bxc' becomes 'a a-b a-(2*b) a-(3*b) … a-(c*b)'
#     Read this as series starts at a, then c further samples decrementing by b (or incrementing by negative b).
#    '_' represents a missing sample from scrape
#    'stale' indicates a stale sample
# Examples:
#     1. '-2+4x3' becomes '-2 2 6 10' - series starts at -2, then 3 further samples incrementing by 4.
#     2. ' 1-2x4' becomes '1 -1 -3 -5 -7' - series starts at 1, then 4 further samples decrementing by 2.
#     3. ' 1x4' becomes '1 1 1 1 1' - shorthand for '1+0x4', series starts at 1, then 4 further samples incrementing by 0.
#     4. ' 1 _x3 stale' becomes '1 _ _ _ stale' - the missing sample cannot increment, so 3 missing samples are produced by the '_x3' expression.
values: <string>
```

#### `<alert_test_case>`

vmalert by default adds `alertgroup` and `alertname` to the generated alerts and time series.
So you will need to specify both `groupname` and `alertname` under a single `<alert_test_case>`,
but no need to add them under `exp_alerts`.
You can also pass `--disableAlertgroupLabel` to skip `alertgroup` check.

```yaml
# The time elapsed from time=0s when this alerting rule should be checked.
# Means this rule should be firing at this point, or shouldn't be firing if 'exp_alerts' is empty.
eval_time: <duration>

# Name of the group name to be tested.
groupname: <string>

# Name of the alert to be tested.
alertname: <string>

# List of the expected alerts that are firing under the given alertname at
# the given evaluation time. If you want to test if an alerting rule should
# not be firing, then you can mention only the fields above and leave 'exp_alerts' empty.
exp_alerts:
  [ - <alert> ]
```

#### `<alert>`

```yaml
# These are the expanded labels and annotations of the expected alert.
# Note: labels also include the labels of the sample associated with the alert
exp_labels:
  [ <labelname>: <string> ]
exp_annotations:
  [ <labelname>: <string> ]
```

#### `<metricsql_expr_test>`

```yaml
# Expression to evaluate
expr: <string>

# The time elapsed from time=0s when this expression be evaluated.
eval_time: <duration>

# Expected samples at the given evaluation time.
exp_samples:
  [ - <sample> ]
```

#### `<sample>`

```yaml
# Labels of the sample in usual series notation '<metric name>{<label name>=<label value>, ...}'
# Examples:
#      series_name{label1="value1", label2="value2"}
#      go_goroutines{job="prometheus", instance="localhost:9090"}
labels: <string>

# The expected value of the Metricsql expression.
value: <number>
```

### Example

This is an example input file for unit testing which will pass.
`test.yaml` is the test file which follows the syntax above and `rules.yaml` contains the alerting rules.

With `rules.yaml` in the same directory with `test.yaml`, run `./vmalert-tool unittest --files=./unittest/testdata/test.yaml -external.label=cluster=prod`.

#### `test.yaml`

```yaml
rule_files:
  - rules.yaml

evaluation_interval: 1m

tests:
  - interval: 1m
    input_series:
      - series: 'up{job="prometheus", instance="localhost:9090"}'
        values: "0+0x1440"

    metricsql_expr_test:
      - expr: subquery_interval_test
        eval_time: 4m
        exp_samples:
          - labels: '{__name__="subquery_interval_test", cluster="prod", instance="localhost:9090", job="prometheus"}'
            value: 1

    alert_rule_test:
      - eval_time: 2h
        groupname: group1
        alertname: InstanceDown
        exp_alerts:
          - exp_labels:
              job: prometheus
              severity: page
              instance: localhost:9090
              cluster: prod
            exp_annotations:
              summary: "Instance localhost:9090 down"
              description: "localhost:9090 of job prometheus in cluster prod has been down for more than 5 minutes."

      - eval_time: 0
        groupname: group1
        alertname: AlwaysFiring
        exp_alerts:
          - exp_labels:
              cluster: prod

      - eval_time: 0
        groupname: group1
        alertname: InstanceDown
        exp_alerts: []
```

#### `rules.yaml`

```yaml
# This is the rules file.

groups:
  - name: group1
    rules:
      - alert: InstanceDown
        expr: up == 0
        for: 5m
        labels:
          severity: page
        annotations:
          summary: "Instance {{ $labels.instance }} down"
          description: "{{ $labels.instance }} of job {{ $labels.job }} in cluster {{ $externalLabels.cluster }} has been down for more than 5 minutes."
      - alert: AlwaysFiring
        expr: 1

  - name: group2
    rules:
      - record: job:test:count_over_time1m
        expr: sum without(instance) (count_over_time(test[1m]))
      - record: subquery_interval_test
        expr: count_over_time(up[5m:])
```

### Debug mode

vmalert-tool can print additional log messages for specific alerting rules, similar to [vmalert](https://docs.victoriametrics.com/vmalert/#debug-mode), by following these steps:
1. Set `debug: true` in rule’s configuration;
2. Run vmalert-tool with the flag `-loggerLevel=INFO`.

The additional log messages include tips for alert state transformations, timestamp and result of each evaluation:
```shell-session
2024-12-10T12:10:26.339Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:00:00Z: query returned 0 samples (elapsed: 2.148792ms)
2024-12-10T12:10:26.339Z	info	VictoriaMetrics/app/vmalert/datasource/client.go:254	DEBUG datasource request: executing POST request with params "http://127.0.0.1:8880/prometheus/api/v1/query?query=test_metric+%3E+0&step=300s&time=1970-01-01T00%3A01%3A00Z"
2024-12-10T12:10:26.339Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:01:00Z: query returned 0 samples (elapsed: 277µs)
2024-12-10T12:10:26.339Z	info	VictoriaMetrics/app/vmalert/datasource/client.go:254	DEBUG datasource request: executing POST request with params "http://127.0.0.1:8880/prometheus/api/v1/query?query=test_metric+%3E+0&step=300s&time=1970-01-01T00%3A02%3A00Z"
2024-12-10T12:10:26.340Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:02:00Z: query returned 1 samples (elapsed: 566.083µs)
2024-12-10T12:10:26.340Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:02:00Z: alert 11669695145351808068 {alertgroup="TestGroup",alertname="TestRule"} created in state PENDING
2024-12-10T12:10:26.343Z	info	VictoriaMetrics/app/vmalert/datasource/client.go:254	DEBUG datasource request: executing POST request with params "http://127.0.0.1:8880/prometheus/api/v1/query?query=test_metric+%3E+0&step=300s&time=1970-01-01T00%3A03%3A00Z"
2024-12-10T12:10:26.344Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:03:00Z: query returned 1 samples (elapsed: 822.958µs)
2024-12-10T12:10:26.344Z	info	VictoriaMetrics/app/vmalert/rule/alerting.go:212	DEBUG rule "TestGroup":"TestRule" (14686524233356632740) at 1970-01-01T00:03:00Z: alert 11669695145351808068 {alertgroup="TestGroup",alertname="TestRule"} PENDING => FIRING: 1m0s since becoming active at 1970-01-01 00:02:00 +0000 UTC
```

### Configuration

Run `vmalert-tool unittest --help` to get all configuration options:

```sh
  -files
    File path or http url with test files. Supports an array of values separated by comma or specified via multiple flags. Supports hierarchical patterns and regexpes.
      Examples:
       -files="/path/to/file". Path to a single test file.
       -files="http://<some-server-addr>/path/to/test.yaml". HTTP URL to a test file.
       -files="dir/**/*.yaml". Includes all the .yaml files in "dir" subfolders recursively.
  -disableAlertgroupLabel
    disable adding group's Name as label to generated alerts and time series. (default: false)
  -external.label
    Optional label in the form 'name=value' to add to all generated recording rules and alerts. Supports an array of values separated by comma or specified via multiple flags.
  -external.url
    Optional external URL to template in rule's labels or annotations.
  -loggerLevel
    Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "ERROR").
```
