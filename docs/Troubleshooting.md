---
sort: 35
weight: 35
title: Troubleshooting
menu:
  docs:
    parent: 'victoriametrics'
    weight: 35
aliases:
- /Troubleshooting.html
---

# Troubleshooting

This document contains troubleshooting guides for most common issues when working with VictoriaMetrics:

- [General troubleshooting checklist](#general-troubleshooting-checklist)
- [Unexpected query results](#unexpected-query-results)
- [Slow data ingestion](#slow-data-ingestion)
- [Slow queries](#slow-queries)
- [Out of memory errors](#out-of-memory-errors)
- [Cluster instability](#cluster-instability)
- [Too much disk space used](#too-much-disk-space-used)
- [Monitoring](#monitoring)

## General troubleshooting checklist

If you hit some issue or have some question about VictoriaMetrics components,
then please follow the following steps in order to quickly find the solution:

1. Check the version of VictoriaMetrics component, which needs to be troubleshot and compare
   it to [the latest available version](https://docs.victoriametrics.com/changelog/).
   If the used version is lower than the latest available version, then there are high chances
   that the issue is already resolved in newer versions. Carefully read [the changelog](https://docs.victoriametrics.com/changelog/)
   between your version and the latest version and check whether the issue is already fixed there.

   If the issue is already fixed in newer versions, then upgrade to the newer version and verify whether the issue is fixed:

   - [How to upgrade single-node VictoriaMetrics](https://docs.victoriametrics.com/#how-to-upgrade-victoriametrics)
   - [How to upgrade VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#updating--reconfiguring-cluster-nodes)

   Upgrade procedure for other VictoriaMetrics components is as simple as gracefully stopping the component
   by sending `SIGINT` signal to it and starting the new version of the component.

   There may be breaking changes between different versions of VictoriaMetrics components in rare cases.
   These cases are documented in [the changelog](https://docs.victoriametrics.com/changelog/).
   So please read the changelog before the upgrade.

1. Inspect command-line flags passed to VictoriaMetrics components and remove flags which unclear outcomes for your workload.
   VictoriaMetrics components are designed to work optimally with the default command-line flag values (e.g. when these flags aren't set explicitly).
   It is recommended removing flags with unclear outcomes, since they may result in unexpected issues.

1. Check for logs in VictoriaMetrics components. They may contain useful information about cause of the issue
   and how to fix the issue. If the log message doesn't have enough useful information for troubleshooting,
   then search the log message in Google. There are high chances that the issue is already reported
   somewhere (docs, StackOverflow, Github issues, etc.) and the solution is already documented there.

1. If VictoriaMetrics logs have no relevant information, then try searching for the issue in Google
   via multiple keywords and phrases specific to the issue. There are high chances that the issue
   and the solution is already documented somewhere.

1. Try searching for the issue at [VictoriaMetrics GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
   The signal/noise quality of search results here is much lower than in Google, but sometimes it may help
   finding the relevant information about the issue when Google fails to find the needed information.
   If you located the relevant GitHub issue, but it misses some information on how to diagnose or troubleshoot it,
   then please provide this information in comments to the issue. This increases chances that it will be resolved soon.

1. Try searching for information about the issue in [VictoriaMetrics source code](https://github.com/search?q=repo%3AVictoriaMetrics%2FVictoriaMetrics&type=code).
   GitHub code search may be not very good in some cases, so it is recommended [checking out VictoriaMetrics source code](https://github.com/VictoriaMetrics/VictoriaMetrics/)
   and perform local search in the checked out code.
   Note that the source code for VictoriaMetrics cluster is located in [the cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) branch.

1. Try searching for information about the issue in the history of [VictoriaMetrics Slack chat](https://victoriametrics.slack.com).
   There are non-zero chances that somebody already stuck with the same issue and documented the solution at Slack.

1. If steps above didn't help finding the solution to the issue, then please [file a new issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose)
   by providing the maximum details on how to reproduce the issue.

   After that you can post the link to the issue to [VictoriaMetrics Slack chat](https://victoriametrics.slack.com),
   so VictoriaMetrics community could help finding the solution to the issue. It is better filing the issue at VictoriaMetrics GitHub
   before posting your question to VictoriaMetrics Slack chat, since GitHub issues are indexed by Google,
   while Slack messages aren't indexed by Google. This simplifies searching for the solution to the issue for future VictoriaMetrics users.

1. Pro tip 1: if you see that [VictoriaMetrics docs](https://docs.victoriametrics.com/) contain incomplete or incorrect information,
   then please create a pull request with the relevant changes. This will help VictoriaMetrics community.

   All the docs published at `https://docs.victoriametrics.com` are located in the [docs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs)
   folder inside VictoriaMetrics repository.

1. Pro tip 2: please provide links to existing docs / GitHub issues / StackOverflow questions
   instead of copy-n-pasting the information from these sources when asking or answering questions
   from VictoriaMetrics community. If the linked resources have no enough information,
   then it is better posting the missing information in the web resource before providing links
   to this information in Slack chat. This will simplify searching for this information in the future
   for VictoriaMetrics users via Google and [Perplexity](https://www.perplexity.ai/).

1. Pro tip 3: if you are answering somebody's question about VictoriaMetrics components
   at GitHub issues / Slack chat / StackOverflow, then the best answer is a direct link to the information
   regarding the question.
   The better answer is a concise message with multiple links to the relevant information.
   The worst answer is a message with misleading or completely wrong information.

1. Pro tip 4: if you can fix the issue on yourself, then please do it and provide the corresponding pull request!
   We are glad to get pull requests from VictoriaMetrics community.


## Unexpected query results

If you see unexpected or unreliable query results from VictoriaMetrics, then try the following steps:

1. Check whether simplified queries return unexpected results. For example, if the query looks like
   `sum(rate(http_requests_total[5m])) by (job)`, then check whether the following queries return
   expected results:

   - Remove the outer `sum` and execute `rate(http_requests_total[5m])`,
     since aggregations could hide some missing series, gaps in data or anomalies in existing series.
     If this query returns too many time series, then try adding more specific label filters to it. 
     For example, if you see that the original query returns unexpected results for the `job="foo"`, 
     then use `rate(http_requests_total{job="foo"}[5m])` query.
     If this isn't enough, then continue adding more specific label filters, so the resulting query returns
     manageable number of time series.

   - Remove the outer `rate` and execute `http_requests_total`. Additional label filters may be added here in order
     to reduce the number of returned series.

   Sometimes the query may be improperly constructed, so it returns unexpected results.
   It is recommended reading and understanding [MetricsQL docs](https://docs.victoriametrics.com/metricsql/),
   especially [subqueries](https://docs.victoriametrics.com/metricsql/#subqueries)
   and [rollup functions](https://docs.victoriametrics.com/metricsql/#rollup-functions) sections.

1. If the simplest query continues returning unexpected / unreliable results, then try verifying correctness
   of raw unprocessed samples for this query via [/api/v1/export](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format)
   on the given `[start..end]` time range and check whether they are expected:

   ```sh
   single-node: curl http://victoriametrics:8428/api/v1/export -d 'match[]=http_requests_total' -d 'start=...' -d 'end=...'
   
   cluster: curl http://<vmselect>:8481/select/<tenantID>/prometheus/api/v1/export -d 'match[]=http_requests_total' -d 'start=...' -d 'end=...'
   ```
   Note that responses returned from [/api/v1/query](https://docs.victoriametrics.com/keyconcepts/#instant-query)
   and from [/api/v1/query_range](https://docs.victoriametrics.com/keyconcepts/#range-query) contain **evaluated** data
   instead of raw samples stored in VictoriaMetrics. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness)
   for details.

   If you migrate from InfluxDB, then pass `-search.setLookbackToStep` command-line flag to single-node VictoriaMetrics
   or to `vmselect` in VictoriaMetrics cluster. See also [how to migrate from InfluxDB to VictoriaMetrics](https://docs.victoriametrics.com/guides/migrate-from-influx.html).

1. Sometimes response caching may lead to unexpected results when samples with older timestamps
   are ingested into VictoriaMetrics (aka [backfilling](https://docs.victoriametrics.com/#backfilling)).
   Try disabling response cache and see whether this helps. This can be done in the following ways:

   - By passing `-search.disableCache` command-line flag to a single-node VictoriaMetrics
     or to all the `vmselect` components if cluster version of VictoriaMetrics is used.

   - By passing `nocache=1` query arg to every request to `/api/v1/query` and `/api/v1/query_range`.
     If you use Grafana, then this query arg can be specified in `Custom Query Parameters` field
     at Prometheus datasource settings - see [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.
    
    If the problem was in the cache, try resetting it via [resetRollupCache handler](https://docs.victoriametrics.com/url-examples/#internalresetrollupresultcache).

1. If you use cluster version of VictoriaMetrics, then it may return partial responses by default
   when some of `vmstorage` nodes are temporarily unavailable - see [cluster availability docs](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-availability)
   for details. If you want to prioritize query consistency over cluster availability,
   then you can pass `-search.denyPartialResponse` command-line flag to all the `vmselect` nodes.
   In this case VictoriaMetrics returns an error during querying if at least a single `vmstorage` node is unavailable.
   Another option is to pass `deny_partial_response=1` query arg to `/api/v1/query` and `/api/v1/query_range`.
   If you use Grafana, then this query arg can be specified in `Custom Query Parameters` field
   at Prometheus datasource settings - see [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.

1. If you pass `-replicationFactor` command-line flag to `vmselect`, then it is recommended removing this flag from `vmselect`,
   since it may lead to incomplete responses when `vmstorage` nodes contain less than `-replicationFactor`
   copies of the requested data.

1. If you observe gaps when plotting time series try simplifying your query according to p2 and follow the list.
   If problem still remains, then it is likely caused by irregular intervals for metrics collection (network delays
   or targets unavailability on scrapes, irregular pushes, irregular timestamps).
   VictoriaMetrics automatically [fills the gaps](https://docs.victoriametrics.com/keyconcepts/#range-query)
   based on median interval between [data samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples).
   This might work incorrect for irregular data as median will be skewed. In this case it is recommended to switch
   to the static interval for gaps filling by setting `-search.minStalenessInterval=5m` cmd-line flag (`5m` is
   the static interval used by Prometheus).

1. If you observe recently written data is not immediately visible/queryable, then read more about 
   [query latency](https://docs.victoriametrics.com/keyconcepts/#query-latency) behavior.

1. Try upgrading to the [latest available version of VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
   and verifying whether the issue is fixed there.

1. Try executing the query with `trace=1` query arg. This enables query tracing, which may contain
   useful information on why the query returns unexpected data. See [query tracing docs](https://docs.victoriametrics.com/#query-tracing) for details.

1. Inspect command-line flags passed to VictoriaMetrics components. If you don't understand clearly the purpose
   or the effect of some flags, then remove them from the list of flags passed to VictoriaMetrics components,
   because some command-line flags may change query results in unexpected ways when set to improper values.
   VictoriaMetrics is optimized for running with default flag values (e.g. when they aren't set explicitly).

1. If the steps above didn't help identifying the root cause of unexpected query results,
   then [file a bugreport](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new) with details on how to reproduce the issue.
   Instead of sharing screenshots in the issue, consider sharing query and [trace](https://docs.victoriametrics.com/#query-tracing) 
   results in [VMUI](https://docs.victoriametrics.com/#vmui) by clicking on `Export query` button in top right corner of the graph area.


## Slow data ingestion

There are the following most commons reasons for slow data ingestion in VictoriaMetrics:

1. Memory shortage for the given amounts of [active time series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series).

   VictoriaMetrics (or `vmstorage` in cluster version of VictoriaMetrics) maintains an in-memory cache
   for quick search for internal series ids per each incoming metric.
   This cache is named `storage/tsid`. VictoriaMetrics automatically determines the maximum size for this cache
   depending on the available memory on the host where VictoriaMetrics (or `vmstorage`) runs. If the cache size isn't enough
   for holding all the entries for active time series, then VictoriaMetrics locates the needed data on disk,
   unpacks it, re-constructs the missing entry and puts it into the cache. This takes additional CPU time and disk read IO.

   The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
   contain `Slow inserts` graph, which shows the cache miss percentage for `storage/tsid` cache
   during data ingestion. If `slow inserts` graph shows values greater than 5% for more than 10 minutes,
   then it is likely the current number of [active time series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series)
   cannot fit the `storage/tsid` cache.

   There are the following solutions exist for this issue:

   - To increase the available memory on the host where VictoriaMetrics runs until `slow inserts` percentage
     will become lower than 5%. If you run VictoriaMetrics cluster, then you need increasing total available
     memory at `vmstorage` nodes. This can be done in two ways: either to increase the available memory
     per each existing `vmstorage` node or to add more `vmstorage` nodes to the cluster.

   - To reduce the number of active time series. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
     contain a graph showing the number of active time series. Recent versions of VictoriaMetrics
     provide [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer),
     which can help determining and fixing the source of [high cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality).

1. [High churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate),
   e.g. when old time series are substituted with new time series at a high rate.
   When VictoriaMetrics encounters a sample for new time series, it needs to register the time series
   in the internal index (aka `indexdb`), so it can be quickly located on subsequent select queries.
   The process of registering new time series in the internal index is an order of magnitude slower
   than the process of adding new sample to already registered time series.
   So VictoriaMetrics may work slower than expected under [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate).

   The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
   provides `Churn rate` graph, which shows the average number of new time series registered
   during the last 24 hours. If this number exceeds the number of [active time series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series),
   then you need to identify and fix the source of [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate).
   The most commons source of high churn rate is a label, which frequently changes its value. Try avoiding such labels.
   The [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer) can help identifying
   such labels.

1. Resource shortage. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
   contain `resource usage` graphs, which show memory usage, CPU usage, disk IO usage and free disk size.
   Make sure VictoriaMetrics has enough free resources for graceful handling of potential spikes in workload
   according to the following recommendations:

   - 50% of free CPU
   - 50% of free memory
   - 20% of free disk space

   If VictoriaMetrics components have lower amounts of free resources, then this may lead
   to **significant** performance degradation after workload increases slightly.
   For example:

   - If the percentage of free CPU is close to 0, then VictoriaMetrics
     may experience arbitrary long delays during data ingestion when it cannot keep up
     with slightly increased data ingestion rate.

   - If the percentage of free memory reaches 0, then the Operating System where VictoriaMetrics components run,
     may have no enough memory for [page cache](https://en.wikipedia.org/wiki/Page_cache).
     VictoriaMetrics relies on page cache for quick queries over recently ingested data.
     If the operating system has no enough free memory for page cache, then it needs
     to re-read the requested data from disk. This may **significantly** increase disk read IO
     and slow down both queries and data ingestion.

   - If free disk space is lower than 20%, then VictoriaMetrics is unable to perform optimal
     background merge of the incoming data. This leads to increased number of data files on disk,
     which, in turn, slows down both data ingestion and querying. See [these docs](https://docs.victoriametrics.com/#storage) for details.

1. If you run cluster version of VictoriaMetrics, then make sure `vminsert` and `vmstorage` components
   are located in the same network with small network latency between them.
   `vminsert` packs incoming data into batch packets and sends them to `vmstorage` on-by-one.
   It waits until `vmstorage` returns back `ack` response before sending the next packet.
   If the network latency between `vminsert` and `vmstorage` is high (for example, if they run in different datacenters),
   then this may become limiting factor for data ingestion speed.

   The [official Grafana dashboard for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#monitoring)
   contain `connection saturation` graph for `vminsert` components. If this graph reaches 100% (1s),
   then it is likely you have issues with network latency between `vminsert` and `vmstorage`.
   Another possible issue for 100% connection saturation between `vminsert` and `vmstorage`
   is resource shortage at `vmstorage` nodes. In this case you need to increase amounts
   of available resources (CPU, RAM, disk IO) at `vmstorage` nodes or to add more `vmstorage` nodes to the cluster.

1. Noisy neighbor. Make sure VictoriaMetrics components run in an environments without other resource-hungry apps.
   Such apps may steal RAM, CPU, disk IO and network bandwidth, which is needed for VictoriaMetrics components.
   Issues like this are very hard to catch via [official Grafana dashboard for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#monitoring)
   and proper diagnosis would require checking resource usage on the instances where VictoriaMetrics runs.

1. If you see `TooHighSlowInsertsRate` [alert](https://docs.victoriametrics.com/#monitoring) when single-node VictoriaMetrics or `vmstorage` has enough
   free CPU and RAM, then increase `-cacheExpireDuration` command-line flag at single-node VictoriaMetrics or at `vmstorage` to the value,
   which exceeds the interval between ingested samples for the same time series (aka `scrape_interval`).
   See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3976#issuecomment-1476883183) for more details.

1. If you see constant and abnormally high CPU usage of VictoriaMetrics component, check `CPU spent on GC` panel
   on the corresponding [Grafana dashboard](https://grafana.com/orgs/victoriametrics) in `Resource usage` section. If percentage of CPU time spent on garbage collection
   is high, then CPU usage of the component can be reduced at the cost of higher memory usage by changing [GOGC](https://tip.golang.org/doc/gc-guide#GOGC) environment variable
   to higher values. By default VictoriaMetrics components use `GOGC=30`. Try running VictoriaMetrics components with `GOGC=100` and see whether this helps reducing CPU usage.
   Note that higher `GOGC` values may increase memory usage.

## Slow queries

Some queries may take more time and resources (CPU, RAM, network bandwidth) than others.
VictoriaMetrics logs slow queries if their execution time exceeds the duration passed
to `-search.logSlowQueryDuration` command-line flag (5s by default).

VictoriaMetrics provides [`top queries` page at VMUI](https://docs.victoriametrics.com/#top-queries), which shows
queries that took the most time to execute.

There are the following solutions exist for improving performance of slow queries:

- Adding more CPU and memory to VictoriaMetrics, so it may perform the slow query faster.
  If you use cluster version of VictoriaMetrics, then migrating `vmselect` nodes to machines
  with more CPU and RAM should help improving speed for slow queries. Query performance
  is always limited by resources of one `vmselect` which processes the query. For example, if 2vCPU cores on `vmselect`
  isn't enough to process query fast enough, then migrating `vmselect` to a machine with 4vCPU cores should increase heavy query performance by up to 2x.
  If the line on `concurrent select` graph form the [official Grafana dashboard for VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#monitoring)
  is close to the limit, then prefer adding more `vmselect` nodes to the cluster.
  Sometimes adding more `vmstorage` nodes also can help improving the speed for slow queries.

- Rewriting slow queries, so they become faster. Unfortunately it is hard determining
  whether the given query is slow by just looking at it.

  The main source of slow queries in practice is [alerting and recording rules](https://docs.victoriametrics.com/vmalert/#rules)
  with long lookbehind windows in square brackets. These queries are frequently used in SLI/SLO calculations such as [Sloth](https://github.com/slok/sloth).

  For example, `avg_over_time(up[30d]) > 0.99` needs to read and process
  all the [raw samples](https://docs.victoriametrics.com/keyconcepts/#raw-samples)
  for `up` [time series](https://docs.victoriametrics.com/keyconcepts/#time-series) over the last 30 days
  each time it executes. If this query is executed frequently, then it can take significant share of CPU, disk read IO, network bandwidth and RAM.
  Such queries can be optimized in the following ways:

  - To reduce the lookbehind window in square brackets. For example, `avg_over_time(up[10d])` takes up to 3x less compute resources
    than `avg_over_time(up[30d])` at VictoriaMetrics.
  - To increase evaluation interval for alerting and recording rules, so they are executed less frequently.
    For example, increasing `-evaluationInterval` command-line flag value at [vmalert](https://docs.victoriametrics.com/vmalert/)
    from `1m` to `2m` should reduce compute resource usage at VictoriaMetrics by 2x.

  Another source of slow queries is improper use of [subqueries](https://docs.victoriametrics.com/metricsql/#subqueries).
  It is recommended avoiding subqueries if you don't understand clearly how they work.
  It is easy to create a subquery without knowing about it.
  For example, `rate(sum(some_metric))` is implicitly transformed into the following subquery
  according to [implicit conversion rules for MetricsQL queries](https://docs.victoriametrics.com/metricsql/#implicit-query-conversions):

  ```metricsql
  rate(
    sum(
      default_rollup(some_metric[1i])
    )[1i:1i]
  )
  ```

  It is likely this query won't return the expected results. Instead, `sum(rate(some_metric))` must be used instead.
  See [this article](https://www.robustperception.io/rate-then-sum-never-sum-then-rate/) for more details.

  VictoriaMetrics provides [query tracing](https://docs.victoriametrics.com/#query-tracing) feature,
  which can help determining the source of slow query.
  See also [this article](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986),
  which explains how to determine and optimize slow queries.


## Out of memory errors

There are the following most common sources of out of memory (aka OOM) crashes in VictoriaMetrics:

1. Improper command-line flag values. Inspect command-line flags passed to VictoriaMetrics components.
   If you don't understand clearly the purpose or the effect of some flags - remove them
   from the list of flags passed to VictoriaMetrics components. Improper command-line flags values
   may lead to increased memory and CPU usage. The increased memory usage increases chances for OOM crashes.
   VictoriaMetrics is optimized for running with default flag values (e.g. when they aren't set explicitly).

   For example, it isn't recommended tuning cache sizes in VictoriaMetrics, since it frequently leads to OOM exceptions.
   [These docs](https://docs.victoriametrics.com/#cache-tuning) refer command-line flags, which aren't
   recommended to tune. If you see that VictoriaMetrics needs increasing some cache sizes for the current workload,
   then it is better migrating to a host with more memory instead of trying to tune cache sizes manually.

1. Unexpected heavy queries. The query is considered as heavy if it needs to select and process millions of unique time series.
   Such query may lead to OOM exception, since VictoriaMetrics needs to keep some of per-series data in memory.
   VictoriaMetrics provides [various settings](https://docs.victoriametrics.com/#resource-usage-limits), 
   which can help limit resource usage.
   For more context, see [How to optimize PromQL and MetricsQL queries](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986).
   VictoriaMetrics also provides [query tracer](https://docs.victoriametrics.com/#query-tracing) 
   to help identify the source of heavy query.

1. Lack of free memory for processing workload spikes. If VictoriaMetrics components use almost all the available memory
   under the current workload, then it is recommended migrating to a host with bigger amounts of memory.
   This would protect from possible OOM crashes on workload spikes. It is recommended to have at least 50%
   of free memory for graceful handling of possible workload spikes.
   See [capacity planning for single-node VictoriaMetrics](https://docs.victoriametrics.com/#capacity-planning)
   and [capacity planning for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning).


## Cluster instability

VictoriaMetrics cluster may become unstable if there is no enough free resources (CPU, RAM, disk IO, network bandwidth)
for processing the current workload.

The most common sources of cluster instability are:

- Workload spikes. For example, if the number of active time series increases by 2x while
  the cluster has no enough free resources for processing the increased workload,
  then it may become unstable.
  VictoriaMetrics provides various configuration settings, which can be used for limiting unexpected workload spikes.
  See [these docs](https://docs.victoriametrics.com/cluster-victoriametrics/#resource-usage-limits) for details.

- Various maintenance tasks such as rolling upgrades or rolling restarts during configuration changes.
  For example, if a cluster contains `N=3` `vmstorage` nodes and they are restarted one-by-one (aka rolling restart),
  then the cluster will have only `N-1=2` healthy `vmstorage` nodes during the rolling restart.
  This means that the load on healthy `vmstorage` nodes increases by at least `100%/(N-1)=50%`
  comparing to the load before rolling restart. E.g. they need to process 50% more incoming
  data and to return 50% more data during queries. In reality, the load on the remaining `vmstorage`
  nodes increases even more because they need to register new time series, which were re-routed
  from temporarily unavailable `vmstorage` node. If `vmstorage` nodes had less than 50%
  of free resources (CPU, RAM, disk IO) before the rolling restart, then it
  can lead to cluster overload and instability for both data ingestion and querying.

  The workload increase during rolling restart can be reduced by increasing
  the number of `vmstorage` nodes in the cluster. For example, if VictoriaMetrics cluster contains
  `N=11` `vmstorage` nodes, then the workload increase during rolling restart of `vmstorage` nodes
  would be `100%/(N-1)=10%`. It is recommended to have at least 8 `vmstorage` nodes in the cluster.
  The recommended number of `vmstorage` nodes should be multiplied by `-replicationFactor` if replication is enabled -
  see [replication and data safety docs](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety)
  for details.

- Time series sharding. Received time series [are consistently sharded](https://docs.victoriametrics.com/cluster-victoriametrics/#architecture-overview)
  by `vminsert` between configured `vmstorage` nodes. As a sharding key `vminsert` is using time series name and labels,
  respecting their order. If the order of labels in time series is constantly changing, this could cause wrong sharding
  calculation and result in un-even and sub-optimal time series distribution across available vmstorages. It is expected
  that metrics pushing client is responsible for consistent labels order (like `Prometheus` or `vmagent` during scraping).
  If this can't be guaranteed, set `-sortLabels=true` cmd-line flag to `vminsert`. Please note, sorting may increase
  CPU usage for `vminsert`.

The obvious solution against VictoriaMetrics cluster instability is to make sure cluster components
have enough free resources for graceful processing of the increased workload.
See [capacity planning docs](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning)
and [cluster resizing and scalability docs](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-resizing-and-scalability)
for details.


## Too much disk space used

If too much disk space is used by a [single-node VictoriaMetrics](https://docs.victoriametrics.com/) or by `vmstorage` component
at [VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/), then please check the following:

- Make sure that there are no old snapsots, since they can occupy disk space. See [how to work with snapshots](https://docs.victoriametrics.com/#how-to-work-with-snapshots)
  and [snapshot troubleshooting](https://docs.victoriametrics.com/#snapshot-troubleshooting).

- Under normal conditions the size of `<-storageDataPath>/indexdb` folder must be smaller than the size of `<-storageDataPath>/data` folder, where `-storageDataPath`
  is the corresponding command-line flag value. This can be checked by the following query if [VictoriaMetrics monitoring](#monitoring) is properly set up:
  ```metricsql
  sum(vm_data_size_bytes{type=~"indexdb/.+"}) without(type)
    /
  sum(vm_data_size_bytes{type=~"(storage|indexdb)/.+"}) without(type)
  ```
  If this query returns values bigger than 0.5, then it is likely there is a [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) issue,
  which results in excess disk space usage for both `indexdb` and `data` folders under `-storageDataPath` folder.
  The solution is to identify and fix the source of high churn rate with [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer).


## Monitoring

Having proper [monitoring](https://docs.victoriametrics.com/single-server-victoriametrics/#monitoring)
would help identify and prevent most of the issues listed above.

[Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards) contain panels reflecting the
health state, resource usage and other specific metrics for VictoriaMetrics components.

The list of [recommended alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
for VictoriaMetrics components will notify about issues and provide recommendations for how to solve them.

Internally, we heavily rely both on dashboards and alerts, and constantly improve them.
It is important to stay up to date with such changes.
