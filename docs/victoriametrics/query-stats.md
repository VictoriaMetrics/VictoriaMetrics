---
weight: 82
title: Query execution stats
menu:
  docs:
    parent: 'victoriametrics'
    weight: 82
aliases:
- /query-stats.html
---

## Query statistics

[Enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/enterprise/) supports statistics logging for
served read queries for [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query)
and [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) API. To enable statistics 
logging specify `-search.logSlowQueryStats=<duration>` command line flag on [vmselect](https://docs.victoriametrics.com/cluster-victoriametrics/)
or [Single-node VictoriaMetrics](https://docs.victoriametrics.com/).
Where `<duration>` is a threshold for query duration after which it must be logged:
* `-search.logSlowQueryStats=5s` will log stats for queries with execution duration exceeding `5s`;
* `-search.logSlowQueryStats=1us` will log stats all queries;
* `-search.logSlowQueryStats=0` disables stats logging.

The example of query statistics log is the following:
```bash
2025-03-25T11:23:29.520Z        info    VictoriaMetrics/app/vmselect/promql/query_stats.go:60       vm_slow_query_stats type=instant query="vm_promscrape_config_last_reload_successful != 1\nor\nvmagent_relabel_config_last_reload_successful != 1\n" query_hash=1585303298 start_ms=1742901750000 end_ms=1742901750000 step_ms=300000 range_ms=0 tenant="0" execution_duration_ms=0 series_fetched=2 samples_fetched=163 bytes=975 memory_estimated_bytes=2032
```

* `type` is either [instant](https://docs.victoriametrics.com/keyconcepts/#instant-query)
  or [range](https://docs.victoriametrics.com/keyconcepts/#range-query) query;
* `query` is the executed [MetricsQL](https://docs.victoriametrics.com/metricsql/) query;
* `query_hash` is a hashed `query` and is used to simplify filtering logs by a specific query;
* `start_ms`, `end_ms`, `step_ms` are query params described [here](https://docs.victoriametrics.com/keyconcepts/#range-query);
* `range_ms` is a difference between `start_ms` and `end_ms`. If `range_ms==0` it means this query is instant;
* `tenant` is a tenant ID. Is available only for cluster version of VictoriaMetrics;
* `execution_duration_ms` is execution duration of the query. It doesn't include time spent on transferring query results to the requester over network;
* `series_fetched` is the amount of unique [time series](https://docs.victoriametrics.com/keyconcepts/#time-series) fetched during query execution. The number could be bigger than
  the actual number of returned series, as it accounts for series before filtering by bool conditions (like `cpu_usage > 0`);
* `samples_fetched` is the amount of [data samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples) fetched
  during query execution;
* `bytes` is the amount of bytes transferred from storage to execute the query;
* `memory_estimated_bytes` is the estimated amount of memory that is needed to evaluate query. See `-search.maxMemoryPerQuery` cmd-line flag.

### Analysis

It is recommended to do collect query statistics logs into [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/)
to do the post-analysis of the query performance.

The generated statistics logs are prefixed with `vm_slow_query_stats` key word to simplify filtering. All the logged fields
are formatted in [logfmt](https://brandur.org/logfmt) format to simplify parsing.

For example, once these logs are available in VictoriaLogs for querying, the following query will find top 5 slowest
queries:
```logsql
vm_slow_query_stats | extract 'vm_slow_query_stats <query_stats>' | unpack_logfmt from query_stats 
| stats by(query) max(execution_duration_ms) execution_duration_max 
| sort by(execution_duration_max) desc | limit 5
```

Here, we begin query with `vm_slow_query_stats` to filter only logs that have the key word. 
Then, with `extract 'vm_slow_query_stats <query_stats>' | unpack_logfmt from query_stats ` we extract and parse log message
into separate fields. And now we can calculate various [stats](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe-functions):
```logsql
| stats by(query) max(execution_duration_ms) execution_duration_max 
```
or 
```logsql
| stats by(query,query_hash) sum(series_fetched) series_fetched_sum
``` 

With use of the [VictoriaLogs Grafana datasource](https://docs.victoriametrics.com/victorialogs/victorialogs-datasource/)
we can build a dashboard with various stats and query filtering options:

![query-stats_dashboard.webp](query-stats_dashboard.webp)