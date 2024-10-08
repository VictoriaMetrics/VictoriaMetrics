---
weight: 10
title: Alerting
menu:
  docs:
    parent: "victorialogs"
    weight: 10
aliases:
- /VictoriaLogs/Alerting.html
---
VictoriaLogs provides log stats APIs [`/select/logsql/stats_query`](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-stats)  and [`/select/logsql/stats_query_range`](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats), which return the log stats in the format compatible with [Prometheus querying API](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries).
This allows users to use VictoriaLogs as the datasource of [vmalert](https://docs.victoriametrics.com/vmalert/) and to write alerting and recording rules using [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).

## Configuration

Run vmalert with the `-defaultRuleType=vlogs` flag.
```
./bin/vmalert -rule=alert.rules \            # Path to the file with rules configuration.
    -datasource.url=http://localhost:9428 \  # VictoriaLogs address.
    -defaultRuleType=vlogs \
    -notifier.url=http://localhost:9093 \    # AlertManager URL (required if alerting rules are used)
    -remoteWrite.url=http://localhost:8428 \ # Remote write compatible storage to persist rules and alerts state info (required if recording rules are used)
    -remoteRead.url=http://localhost:8428 \  # Prometheus HTTP API compatible datasource to restore alerts state from

```

The complete list of supported command-line flags is available at https://docs.victoriametrics.com/vmalert/#configuration.
The flags listed here are specifically related to the VictoriaLogs datasource.

```
-defaultRuleType
  Default type for rule expressions, can be overridden by type parameter inside the rule group. Supported values: "graphite", "prometheus" and "vlogs". 
  Default is "prometheus", change it to "vlogs" if all of the rules are written with LogsQL.
-datasource.applyIntervalAsTimeFilter
  Only work for victoriaLogs rules. Whether to apply the evaluation interval as a time filter for the rules. (default true)
-rule.evalDelay time
   Adjustment of the time parameter for rule evaluation requests to compensate intentional data delay from the datasource. Normally, should be equal to `-search.latencyOffset` (cm d-line flag configured for VictoriaMetrics single-node or vmselect).
   In victoriaLogs, since there is no intentional search delay, `-rule.evalDelay` can be reduced to a few seconds to accommodate network and ingestion time.
```

### rule file

See https://docs.victoriametrics.com/vmalert/#groups. 

## Use cases

### Alerting rules

Examples:
```
groups:
  - name: ServiceLog
    interval: 5m
    rules:
      - alert: HasErrorLog
        expr: 'env: "prod" AND status:~"error|warn" | stats by (service) count(*) as errorLog | filter errorLog:>0'
        annotations:
          description: "Service {{$labels.service}} generated {{$labels.errorLog}} error logs in the last 5 minutes"

  - name: ServiceRequest
    interval: 5m
    rules:
      - alert: TooManyFailedRequest
        expr: '* | extract "ip=<ip> " | extract "status_code=<code>;" | stats by (ip, code) count() if (code:!~200) as failed, count() as total| math failed / total as failed_percentage| filter failed_percentage :> 0.01 | fields ip,failed_percentage'
        annotations:
          description: "Connection from address {{$labels.ip}} has {{$value}} failed requests ratio in last 5 minutes"
```

### Recording rules

Examples:
```
groups:
  - name: RequestCount
    interval: 5m
    rules:
      - record: nginxRequestCount
        expr: 'env: "test" AND service: "nginx" | stats count(*) as requests'
        annotations:
          description: "Service nginx on env test accepted {{$labels.requests}} requests in the last 5 minutes"
      - record: prodRequestCount
        expr: 'env: "prod" | stats by (service) count(*) as requests'
        annotations:
          description: "Service {{$labels.service}} on env prod accepted {{$labels.requests}} requests in the last 5 minutes"
```

### Time filter

By default, vmalert uses group evaluation interval as the log stats query time range. Like in above examples, all the groups have `interval: 5m`, so the time filter `_time:5m` is automatically added to all the rule expressions.
This behavior can be modified by setting command-line flag `-datasource.applyIntervalAsTimeFilter=false` when starting vmalert. In this case, users can add different [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) in the rule expressions. 
For instance:
```
groups:
    interval: 5m
    rules:
      - alert: TooManyFailedRequest
        expr: '_time:10m | extract "ip=<ip> " | extract "status_code=<code>;" | stats by (ip, code) count() if (code:!~200) as failed, count() as total| math failed / total as failed_percentage| filter failed_percentage :> 0.01 | fields ip,failed_percentage'
        annotations: "Connection from address {{$labels.ip}} has {{$$value}} failed requests ratio in last 10 minutes"
```
This rule will be evaluated every 5 minutes, while reading all the logs from the last 10 minutes.

## Replay mode

Alerting and recording rules for VictoriaLogs still work in vmalert [replay mode](https://docs.victoriametrics.com/vmalert/#rules-backfilling), but the time filter cannot differ from the group evaluation interval as mentioned above.


## Performance tip

See https://docs.victoriametrics.com/victorialogs/logsql/#performance-tips.

In LogsQL, users can obtain multiple stats results from a single expression.
For instance, the following query calculates 50th, 90th and 99th percentiles for the request_duration_seconds field over logs for the last 5 minutes:

```
_time:5m | stats
  quantile(0.5, request_duration_seconds) p50,
  quantile(0.9, request_duration_seconds) p90,
  quantile(0.99, request_duration_seconds) p99
```

This expression can also be used in recording rules as follows:
```
groups:
  - name: requestDuration
    interval: 5m
    rules:
      - record: requestDurationQuantile
        expr: '_time:5m | stats by (service) quantile(0.5, request_duration_seconds) p50, quantile(0.9, request_duration_seconds) p90, quantile(0.99, request_duration_seconds) p99'
```
This will generate three metrics for each service:
```
requestDurationQuantile{stats_function="p50", service="service-1"}
requestDurationQuantile{stats_function="p90", service="service-1"}
requestDurationQuantile{stats_function="p99", service="service-1"}

requestDurationQuantile{stats_function="p50", service="service-2"}
requestDurationQuantile{stats_function="p90", service="service-2"}
requestDurationQuantile{stats_function="p00", service="service-2"}
...
```


## Limitations


