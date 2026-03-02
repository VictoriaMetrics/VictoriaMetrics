---
weight: 35
title: Troubleshooting
menu:
  docs:
    parent: 'victoriametrics'
    weight: 35
tags:
  - metrics
aliases:
- /Troubleshooting.html
- /troubleshooting/index.html
- /troubleshooting/
---

This document contains troubleshooting guides for the most common issues when working with VictoriaMetrics.

## General troubleshooting checklist

If you encounter an issue or have a question about VictoriaMetrics components, follow these steps to quickly find a solution:

1. Check the version of the VictoriaMetrics component you are troubleshooting and compare
   it with [the latest available version](https://docs.victoriametrics.com/victoriametrics/changelog/).

   If you are running an older version, the issue may already be fixed. Review the [changelog](https://docs.victoriametrics.com/victoriametrics/changelog/)
   for all releases between your version and the latest release to see whether the problem has been resolved.

   If the issue is fixed in a newer release, upgrade and verify that the problem no longer occurs:

   - [How to upgrade single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-upgrade-victoriametrics)
   - [How to upgrade VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#updating--reconfiguring-cluster-nodes)

   The upgrade procedure for other VictoriaMetrics components is as simple as gracefully stopping the component
   by sending it a `SIGINT` signal and starting the new version of the component.

   In rare cases, upgrades may include breaking changes. These cases are documented in the [changelog](https://docs.victoriametrics.com/victoriametrics/changelog/),
   especially check the **Update notes** near the top of the changelog, as they point out any special actions or considerations to take when upgrading.

1. Review command-line flags passed to VictoriaMetrics components and remove any flags whose impact on your workload is unclear.
   VictoriaMetrics components are optimized to work well with default settings (that is, when flags aren't explicitly set).
   Unnecessary or poorly understood flags can lead to unexpected behavior, so it's best to remove them unless you clearly understand why they are needed.

1. Check logs. They often contain useful details about the root cause and possible fixes.

   If the logs don't provide enough information, try searching the error message on Google. In many cases, the issue has
   already been discussed (in documentation, on Stack Overflow, or in GitHub issues), and a solution may already be available.

1. If VictoriaMetrics logs do not have relevant information, then try searching for the issue on Google
   using multiple keywords and phrases specific to the issue. In many cases, both the issue and its solution are already documented.

1. Try searching for the issue in [VictoriaMetrics GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
   The signal-to-noise ratio of search results here is much lower than on Google, but sometimes it can help
   find relevant information when Google fails.
   If you located the relevant GitHub issue, but it lacks details for diagnosis or troubleshooting,
   then please add them in the issue comments. This increases the chance that it will be resolved soon.

1. Try searching for information about the issue in the [VictoriaMetrics source code](https://github.com/search?q=repo%3AVictoriaMetrics%2FVictoriaMetrics&type=code).
   GitHub code search may not be very effective in some cases, so it is recommended [to check out the VictoriaMetrics source code](https://github.com/VictoriaMetrics/VictoriaMetrics/)
   and perform a local search in the code.
   Note that the source code for the VictoriaMetrics cluster is located in [the cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) branch.

1. If the steps above didn't help to find the solution to the issue, then please [file a new issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose)
   with as many details as possible on how to reproduce it.

   After that you can post the link to the issue in the [VictoriaMetrics Slack chat](https://victoriametrics.slack.com),
   so the VictoriaMetrics community can help find a solution. It is better to file the issue on VictoriaMetrics GitHub
   before posting your question to the VictoriaMetrics Slack chat, since GitHub issues are indexed by Google,
   while Slack messages are not. This simplifies finding a solution to the issue for future VictoriaMetrics users.

1. Pro tip 1: if you see that [VictoriaMetrics docs](https://docs.victoriametrics.com/victoriametrics/) contain incomplete or incorrect information,
   then please create a pull request with the relevant changes or a new issue explaining the problem. This will help the VictoriaMetrics community.

1. Pro tip 2: please provide links to existing docs / GitHub issues / StackOverflow questions
   instead of copying and pasting the information from these sources when asking or answering questions
   to the VictoriaMetrics community. If the linked resources do not have enough information,
   then it is better to add the missing information to the original web resource before linking it to Slack chat. This will simplify searching for this information in the future
   for VictoriaMetrics users via Google and [Perplexity](https://www.perplexity.ai/).

1. Pro tip 3: if you are answering somebody's question about VictoriaMetrics components
   in GitHub issues / Slack chat / StackOverflow, then the best answer is a direct link to the information
   with the answer or solution to the question.
   The better answer is a concise message with multiple links to the relevant information.
   The worst answer is a message with misleading or completely wrong information.

1. Pro tip 4: If you can fix the issue on your own, then please do it and provide the corresponding pull request!
   We are happy to get pull requests from the VictoriaMetrics community.

## Unexpected query results

If you see unexpected or unreliable query results from VictoriaMetrics, then try the following steps:

1. Check whether simplified queries return unexpected results. For example, if the query looks like
   `sum(rate(http_requests_total[5m])) by (job)`, then check whether the following queries return
   expected results:

   - Remove the outer `sum` and execute `rate(http_requests_total[5m])`.
     Aggregations could hide missing series, data gaps, or anomalies.
     
   - If the query returns too many series, try adding more specific label filters.
     For example, if you see that the original query returns unexpected results for the `job="foo"`,
     then use the `rate(http_requests_total{job="foo"}[5m])` query.
     Continue adding more specific label filters until the resulting query returns a manageable number of time series.

   - Remove the outer `rate` and execute `http_requests_total`. Add label filters to reduce the number of returned series
     if needed.

   Sometimes the query may be improperly constructed, leading to unexpected results.
   It is recommended to read and understand [MetricsQL docs](https://docs.victoriametrics.com/victoriametrics/metricsql/),
   especially [subqueries](https://docs.victoriametrics.com/victoriametrics/metricsql/#subqueries)
   and [rollup functions](https://docs.victoriametrics.com/victoriametrics/metricsql/#rollup-functions) sections.


1. If the simplest query continues returning unexpected / unreliable results, then try verifying correctness
   of raw unprocessed samples in [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) via the `Raw Query` tab.

   Responses returned from [/api/v1/query](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#instant-query)
   and [/api/v1/query_range](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#range-query) contain **evaluated** data
   instead of stored raw samples. In some cases, [staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness), 
   [deduplication](https://docs.victoriametrics.com/victoriametrics/#deduplication), or irregular scrapes can affect evaluations.
   See [this short video](https://www.youtube.com/watch?v=7AyVCC6uKfI) for details.

   Raw data can be downloaded via the `Export` button in vmui's  `Raw Query` tab or via [/api/v1/export](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-export-data-in-json-line-format)
   query on the given `[start..end]` time range and check whether they are expected:

   ```sh
   single-node: curl http://victoriametrics:8428/api/v1/export -d 'match[]=http_requests_total' -d 'start=...' -d 'end=...' -d 'reduce_mem_usage=1'

   cluster: curl http://<vmselect>:8481/select/<tenantID>/prometheus/api/v1/export -d 'match[]=http_requests_total' -d 'start=...' -d 'end=...' -d 'reduce_mem_usage=1'
   ```
   When raising a GitHub ticket about query issues, please also attach the raw data, so maintainers can reproduce your case locally.   

1. Try executing the query with [tracer](https://docs.victoriametrics.com/victoriametrics/#query-tracing) enabled. The trace
   contains a lot of additional information about query execution, series matching, caches, and internal modifications.
   When raising a GitHub ticket about query issues, please also attach the trace so maintainers can investigate.

1. If you observe gaps when plotting series, it is likely caused by irregular intervals for metrics collection (network delays
   or targets unavailability during scrapes, irregular pushes, irregular timestamps).
   VictoriaMetrics automatically [fills the gaps](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#range-query)
   based on the median interval between [data samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples).
   This may yield incorrect results for irregular data, as the median will be skewed. In this case, it is recommended to either fix the
   irregularities or switch to the static interval for gaps filling by setting `-search.minStalenessInterval=5m` command-line flag (`5m` is
   used by Prometheus by default).

1. Sometimes, response caching may lead to unexpected results when samples with older timestamps
   are ingested into VictoriaMetrics (aka [backfilling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#backfilling)).
   Try disabling response cache and see whether this helps:

   - By clicking on the toggle `Disable cache` in vmui.
   
   - By passing `-search.disableCache` command-line flag to a single-node VictoriaMetrics
     or to all the `vmselect` components if the cluster version of VictoriaMetrics is used.

   - By passing `nocache=1` query arg to every request to `/api/v1/query` and `/api/v1/query_range`.
     If you use Grafana, then this query arg can be specified in the `Custom Query Parameters` field
     in Prometheus datasource settings. See [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.

    If the problem was in the cache, try resetting it via the [resetRollupCache handler](https://docs.victoriametrics.com/victoriametrics/url-examples/#internalresetrollupresultcache).

1. Cluster version of VictoriaMetrics may return partial responses by default when some of the `vmstorage` nodes are temporarily
   unavailable. See [cluster availability docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-availability).
   If you want to prioritize query consistency over cluster availability, then pass `-search.denyPartialResponse` command-line flag to all the `vmselect` nodes.
   This causes VictoriaMetrics to return an error during query execution if at least one `vmstorage` node is unavailable.
   Another option is to pass `deny_partial_response=1` query argument to `/api/v1/query` and `/api/v1/query_range`.
   If you use Grafana, then this query argument can be specified in the `Custom Query Parameters` field
   in Prometheus/VictoriaMetrics datasource settings. See [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.

1. If you pass the `-replicationFactor` command-line flag to `vmselect`, then it is recommended to remove this flag from `vmselect`,
   since it may lead to incomplete responses when `vmstorage` nodes contain less than `-replicationFactor`
   copies of the requested data.

1. If you observe that recently written data is not immediately visible/queryable, then read more about
   [query latency](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-latency) behavior.

1. Try upgrading to the [latest available version of VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
   and verifying whether the issue is fixed there.

1. Inspect command-line flags passed to VictoriaMetrics components. If you don't clearly understand the purpose
   or the effect of some flags, then remove them from the list of flags. 
   VictoriaMetrics components are optimized to work well with default settings (that is, when flags aren't explicitly set).
   Unnecessary or poorly understood flags can lead to unexpected behavior, so it's best to remove them unless you clearly understand why they are needed.

1. If the steps above didn't help identify the root cause of unexpected query results,
   then [file a bug report](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new) with details on how to reproduce the issue.
   Instead of sharing screenshots in the issue, consider sharing the query, [raw samples](https://docs.victoriametrics.com/victoriametrics/#vmui) and [trace](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#query-tracing)
   results via [VMUI](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui).

## Slow data ingestion

These are the most common reasons for slow data ingestion in VictoriaMetrics:

1. Memory shortage for the given amounts of [active time series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series).

   VictoriaMetrics (or `vmstorage` in the cluster version of VictoriaMetrics) maintains an in-memory cache `storage/tsid`
   for a quick search for internal series IDs for each incoming metric. VictoriaMetrics automatically determines the maximum 
   size for this cache depending on the available memory on the host where VictoriaMetrics (or `vmstorage`) runs. 
   If the cache size isn't enough to hold all the entries for active time series, then VictoriaMetrics locates the required data on disk,
   unpacks it, reconstructs the missing entry, and adds it to the cache. This takes additional CPU time and disk read I/O.

   The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
   contain a `Slow inserts` graph that shows the cache miss percentage for the `storage/tsid` cache during data ingestion. 
   If the `slow inserts` graph shows values greater than 5% for more than 10 minutes,
   then it is likely that the current number of [active time series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series)
   cannot fit the `storage/tsid` cache.

   These are the solutions that exist for this issue:

   - Increase the available memory on the host where VictoriaMetrics runs until the `slow inserts` percentage
     drops to 5% or less. If you run a VictoriaMetrics cluster, then you need to increase the total available
     memory at all `vmstorage` nodes. This can be done in two ways: either to increase the available memory
     for each `vmstorage` node or to add more `vmstorage` nodes to the cluster to spread the load.

   - Reduce the number of active time series. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
     contain a graph showing the number of active time series. Use the [cardinality explorer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-explorer)
     to determine and fix the source of [high cardinality](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-cardinality).

   - Insert performance can degrade when the same time series arrives with labels in a different order.
     Ensure your ingestion client always sends labels in a consistent order for each series.
     Prometheus and `vmagent` already guarantee this, but custom or third-party clients might not.
     As a fallback, you can enable `-sortLabels=true` on VictoriaMetrics or on `vminsert` in cluster mode.
     This forces the server to normalize label order, though it increases CPU usage during ingestion.

1. [High churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).
   When VictoriaMetrics encounters a sample for a new time series, it needs to register the time series
   in the internal index (aka `indexdb`), so it can be quickly located during select queries.
   The process of registering new time series in the internal index is an order of magnitude slower
   than the process of adding a new sample to an already registered time series.
   So VictoriaMetrics may work slower than expected under [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).

   The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
   provide a `Churn rate` graph, which shows the average number of new time series registered
   during the last 24 hours. If this number exceeds the number of [active time series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series),
   then you need to identify and fix the source of [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).
   The most common source of high churn rate is a label that frequently changes its value (like timestamp, session_id). **Try avoiding such labels.**
   The [cardinality explorer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-explorer) can help identify
   such labels.

1. Resource shortage. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
   contain `Resource usage` graphs that show memory usage, CPU usage, disk I/O usage, etc.
   Make sure VictoriaMetrics has enough free resources for gracefully handling potential spikes in workload
   according to the following recommendations:

   - 50% of free CPU
   - 50% of free memory
   - 20% of free disk space

   If VictoriaMetrics components have lower amounts of free resources, then this may lead
   to **significant** performance degradation when workload increases slightly.
   For example:

   - If the percentage of free CPU is close to 0, then VictoriaMetrics
     may experience arbitrarily long delays during data ingestion, even with slight increases in ingestion rate.

   - If the percentage of free memory reaches 0, then the Operating System where VictoriaMetrics components run
     may not have enough memory for the [page cache](https://en.wikipedia.org/wiki/Page_cache).
     VictoriaMetrics relies on the page cache for quick queries over recently ingested data.
     If the operating system does not have enough free memory for the page cache, then it must
     re-read the requested data from disk. This may **significantly** increase disk read I/O
     and slow down both queries and data ingestion.

   - If free disk space is below 20%, then VictoriaMetrics may be unable to perform optimal
     background merge of the incoming data. This results in more data files on disk.
     That, in turn, slows down both data ingestion and querying. See [these docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#storage) for details.

1. If you run the cluster version of VictoriaMetrics, then make sure `vminsert` and `vmstorage` components
   are located in the same network with a low network latency between them.
   `vminsert` packs incoming data into batch packets and sends them to `vmstorage` one by one.
   It waits until `vmstorage` returns back an `ack` response before sending the next packet.
   If the network latency between `vminsert` and `vmstorage` is high (for example, if they run in different datacenters),
   then this may become a limiting factor for data ingestion speed.

   The [official Grafana dashboard for the cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring)
   contains a `connection saturation` panel for `vminsert` components. If this graph reaches 100% (1s),
   then it is likely you have issues with network latency between `vminsert` and `vmstorage`.
   Another possible issue for 100% connection saturation between `vminsert` and `vmstorage`
   is a resource shortage in the `vmstorage` nodes. In this case, you need to increase the amount
   of available resources (CPU, RAM, disk I/O) at `vmstorage` nodes or add more `vmstorage` nodes to the cluster.

1. Noisy neighbor. Make sure VictoriaMetrics components run in an environment without other resource-hungry apps.
   Such apps may steal RAM, CPU, disk I/O, and network bandwidth that are needed for VictoriaMetrics components.
   Issues like this are hard to catch via the [official Grafana dashboard for the cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring)
   and proper diagnosis would require checking resource usage on the instances where VictoriaMetrics runs.

1. If you see a `TooHighSlowInsertsRate` [alert](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring) when single-node VictoriaMetrics or `vmstorage` has enough
   free CPU and RAM, then increase the `-cacheExpireDuration` command-line flag at single-node VictoriaMetrics or at `vmstorage` to a value
   that exceeds the interval between ingested samples for the same time series (aka `scrape_interval`).
   See [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3976#issuecomment-1476883183) for more details.

1. If you see constant and abnormally high CPU usage for the VictoriaMetrics component, check the `CPU spent on GC` panel
   on the corresponding [Grafana dashboard](https://grafana.com/orgs/victoriametrics) in the `Resource usage` section. If the percentage of CPU time spent on garbage collection
   is high, then CPU usage of the component can be reduced at the cost of higher memory usage by increasing the [GOGC](https://tip.golang.org/doc/gc-guide#GOGC) environment variable.
   By default, VictoriaMetrics components use `GOGC=30`. Try running VictoriaMetrics components with `GOGC=100` and see whether this helps reduce CPU usage.
   Note that higher `GOGC` values may increase memory usage.

## Slow queries

Some queries may take more time and resources (CPU, RAM, network bandwidth) than others.
VictoriaMetrics logs slow queries if their execution time exceeds the duration passed
to `-search.logSlowQueryDuration` command-line flag (5s by default).

VictoriaMetrics provides a [`top queries` page in VMUI](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#top-queries) that shows
the longest-running queries. And [Query execution stats](https://docs.victoriametrics.com/victoriametrics/query-stats/) for dumping slow queries
to logs.

These are the solutions that exist for improving the performance of slow queries:

- Investigating the bottleneck in query execution using [query tracing](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#query-tracing).
  It will show the percentage of time spent on each execution step and help understand the volume of processed data.

- Adding more CPU and memory to VictoriaMetrics, so it may perform the slow query faster.
  If you use the cluster version of VictoriaMetrics, then migrating `vmselect` nodes to machines
  with more CPU and RAM should help improve speed for slow queries. Query performance
  is always limited by the resources of **one** `vmselect` that processes the query. For example, if 2 vCPU cores on `vmselect`
  can't process queries fast enough, then migrating `vmselect` to a machine with 4 vCPU cores should increase heavy query performance by up to 2x.
  If the line on the `concurrent select` graph from the [official Grafana dashboard for VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring)
  is close to the limit, then prefer adding more `vmselect` nodes to the cluster.
  Sometimes adding more `vmstorage` nodes can also help improve the speed for slow queries.

- Rewriting slow queries, so they become faster.

  The main source of slow queries in practice is [alerting and recording rules](https://docs.victoriametrics.com/victoriametrics/vmalert/#rules)
  with long lookbehind windows in square brackets. These queries are frequently used in SLI/SLO calculations such as [Sloth](https://github.com/slok/sloth).

  For example, `avg_over_time(up[30d]) > 0.99` needs to read and process
  all the [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples)
  for the `up` [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) over the last 30 days
  each time it executes. If this query is executed frequently, it can take a significant share of CPU, disk read I/O, network bandwidth, and RAM.
  Such queries can be optimized in the following ways:

  - To reduce the look-behind window in square brackets. For example, `avg_over_time(up[10d])` takes up to 3x less compute resources
    than `avg_over_time(up[30d])` at VictoriaMetrics.
  - To increase the evaluation interval for alerting and recording rules, so they are executed less frequently.
    For example, increasing the `-evaluationInterval` command-line flag value at [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/)
    from `1m` to `2m` should reduce compute resource usage by VictoriaMetrics 2x.

  Another source of slow queries is improper use of [subqueries](https://docs.victoriametrics.com/victoriametrics/metricsql/#subqueries).
  It is recommended to avoid subqueries if you don't clearly understand how they work.
  It is easy to create a subquery without knowing about it.
  For example, `rate(sum(some_metric))` is implicitly transformed into the following subquery
  according to [implicit conversion rules for MetricsQL queries](https://docs.victoriametrics.com/victoriametrics/metricsql/#implicit-query-conversions):

  ```metricsql
  rate(
    sum(
      default_rollup(some_metric[1i])
    )[1i:1i]
  )
  ```

  It is likely this query won't return the expected results. Instead, `sum(rate(some_metric))` must be used instead.
  See [this article](https://www.robustperception.io/rate-then-sum-never-sum-then-rate/) for more details.

  See also [this article](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986),
  which explains how to identify and optimize slow queries.

## Out of memory errors

The following are the most common sources of out-of-memory (aka OOM) crashes in VictoriaMetrics:

1. Improper command-line flag values. Inspect command-line flags passed to VictoriaMetrics components.
   If you don't clearly understand the purpose or the effect of some flags, remove them
   from the list of flags passed to VictoriaMetrics components. Improper command-line flag values
   may lead to increased memory and CPU usage. Increased memory usage increases the risk of OOM crashes.
   VictoriaMetrics is optimized to run with default flag values (e.g., when they aren't explicitly set).

   For example, it isn't recommended to change cache sizes in VictoriaMetrics, as this frequently leads to OOM exceptions.
   [These docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cache-tuning) refer to command-line flags that aren't
   recommended to tune. If you see that VictoriaMetrics needs to increase some cache sizes for the current workload,
   then it is better to migrate to a host with more memory instead of trying to tune cache sizes manually.

1. Unexpected heavy queries. The query is considered heavy if it needs to select and process millions of unique time series.
   Such a query may cause an OOM exception, as VictoriaMetrics needs to keep some per-series data in memory.
   VictoriaMetrics provides [various settings](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#resource-usage-limits)
   that can help limit resource usage.
   For more context, see [How to optimize PromQL and MetricsQL queries](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986).
   VictoriaMetrics also provides [query tracer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#query-tracing)
   to help identify the source of heavy queries. Slow queries can be logged with additional details via [Query execution stats](https://docs.victoriametrics.com/victoriametrics/query-stats/). 

1. Lack of free memory for processing workload spikes. If VictoriaMetrics components use almost all the available memory
   under the current workload, then it is recommended to migrate to a host with larger amounts of memory.
   This would protect from possible OOM crashes on workload spikes. It is recommended to have at least 50%
   of free memory to gracefully handle possible workload spikes.
   See [capacity planning for single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#capacity-planning)
   and [capacity planning for the cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning).

## Cluster instability

The VictoriaMetrics cluster may become unstable if there are not enough free resources (CPU, RAM, disk I/O, network bandwidth)
for processing the current workload.

The most common sources of cluster instability are:

- Workload spikes. For example, if the number of active time series increases by 2x while
  the cluster does not have enough free resources for processing the increased workload, then it may become unstable.
  VictoriaMetrics provides several configuration settings to limit unexpected workload spikes.
  See [these docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#resource-usage-limits) for details.

- Various maintenance tasks, such as rolling upgrades or rolling restarts, during configuration changes.
  For example, if a cluster contains `N=3` `vmstorage` nodes and they are restarted one-by-one (aka rolling restart),
  then the cluster will have only `N-1=2` healthy `vmstorage` nodes during the rolling restart.
  This means that the load on healthy `vmstorage` nodes increases by at least `100%/(N-1)=50%`
  compared to the load before rolling restart. E.g., they need to process 50% more incoming
  data and to return 50% more data during queries. In reality, the load on the remaining `vmstorage`
  nodes increases even more because they need to register new time series that were re-routed
  from a temporarily unavailable `vmstorage` node. If `vmstorage` nodes had less than 50%
  of free resources (CPU, RAM, disk I/O) before the rolling restart, then it
  can lead to cluster overload and instability for both data ingestion and querying.

  The workload increase during rolling restart can be reduced by increasing
  the number of `vmstorage` nodes in the cluster. For example, if the VictoriaMetrics cluster contains
  `N=11` `vmstorage` nodes, then the workload increase during rolling restart of `vmstorage` nodes
  would be `100%/(N-1)=10%`. It is recommended to have at least 8 `vmstorage` nodes in the cluster.
  The recommended number of `vmstorage` nodes should be multiplied by `-replicationFactor` if replication is enabled -
  see [replication and data safety docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety)
  for details.

- Time series sharding. Received time series [are consistently sharded](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview)
  by `vminsert` between configured `vmstorage` nodes. As a sharding key, `vminsert` is using time series name and labels,
  respecting their order. If the order of labels in a time series is constantly changing, this could cause wrong sharding
  calculation and result in uneven and suboptimal time series distribution across available vmstorages. It is expected
  that the client who is pushing metrics is responsible for consistent label order (like `Prometheus` or `vmagent` during scraping).
  If this can't be guaranteed, set `-sortLabels=true` command-line flag to `vminsert`. Please note that sorting may increase
  CPU usage for `vminsert`.

- Network instability between cluster components (`vminsert`, `vmselect`, `vmstorage`) may lead to increased error rates, timeouts, or degraded performance.
  Check resource usage graphs for all components on [the official Grafana dashboard for VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring).
  If the graphs show high CPU usage, then the cluster is likely overloaded and requires more resources.
  Note that short-lived 100% CPU spikes may not be visible in metrics with typical 10–30s scrape intervals,
  but can still cause transient network failures. In such cases, check CPU usage at the OS level with higher-resolution tools.
  Consider increasing `-vmstorageDialTimeout` and `-rpc.handshakeTimeout`{{% available_from "v1.124.0" %}} to mitigate the effects of CPU spikes.

  If resource usage appears normal but networking issues persist, the root cause is likely outside VictoriaMetrics.
  This may be caused by unreliable or congested network links, especially across availability zones or regions.
  In multi-AZ setups, consider [a multi-level cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multi-level-cluster-setup) with region-local load balancers to reduce cross-zone connections.
  If the network cannot be improved, increasing timeouts such as `-vmstorageDialTimeout`, `-rpc.handshakeTimeout`{{% available_from "v1.124.0" %}}, or `-search.maxQueueDuration` may help, but should be done cautiously, as higher timeouts can impact cluster stability in other ways.
  Keep in mind that VictoriaMetrics assumes reliable networking between components. If the network is unstable, the overall cluster stability may degrade regardless of resource availability.

The obvious solution to VictoriaMetrics cluster instability is to make sure cluster components
have sufficient free resources to handle the increased workload gracefully.
See [capacity planning docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning)
and [cluster resizing and scalability docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-resizing-and-scalability)
for details.

## Too much disk space used

If too much disk space is used by a [single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) or by `vmstorage` component
at [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/), then please check the following:

- Make sure that there are no old snapshots, since they can occupy disk space. See [how to work with snapshots](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots),
  [snapshot troubleshooting](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#snapshot-troubleshooting) and [vmbackup troubleshooting](https://docs.victoriametrics.com/victoriametrics/vmbackup/#troubleshooting).

- Under normal conditions, the size of `<-storageDataPath>/indexdb` folder must be smaller than the size of `<-storageDataPath>/data` folder, where `-storageDataPath`
  is the corresponding command-line flag value. This can be checked by the following query if [VictoriaMetrics monitoring](#monitoring) is properly set up:

  ```metricsql
  sum(vm_data_size_bytes{type=~"indexdb/.+"}) without(type)
    /
  sum(vm_data_size_bytes{type=~"(storage|indexdb)/.+"}) without(type)
  ```

  If this query returns values greater than 0.5, then it is likely there is a [high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate) issue,
  that results in excess disk space usage for both the `indexdb` and `data` folders under the `-storageDataPath` folder.
  The solution is to identify and fix the source of the high churn rate with the [cardinality explorer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-explorer).

## Monitoring

Having proper [monitoring](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#monitoring)
can help identify and prevent most of the issues listed above.

[Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards) contain panels reflecting the
health state, resource usage, and other specific metrics for VictoriaMetrics components.

Check the list of [recommended alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
for VictoriaMetrics components to receive notifications about issues and receive recommendations for resolving them.

Internally, we rely heavily on both dashboards and alerts, and we constantly improve them.
It is important to stay up to date with such changes.


## Filesystem read corruption on ZFS

On some ZFS filesystems, mixing reads from memory-mapped files (`mmap`) with usage of the `mincore()` syscall can trigger a bug in the ZFS in-memory cache (ARC), potentially resulting in **data read corruption** in VictoriaMetrics processes. This scenario has been observed when VictoriaMetrics instances access data directories on ZFS.

Symptoms:
   Note that the source code for the VictoriaMetrics cluster is located in [the cluster](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster) branch.
- Unexpected read errors when accessing data on ZFS.
- Corrupted or inconsistent query results.
- Crashes or panics in storage/query components when reading from ZFS.

It could be mitigated with the `--fs.disableMincore` flag:

```text
./bin/victoria-metrics --storageDataPath /path/to/zfs/data --fs.disableMincore
```
