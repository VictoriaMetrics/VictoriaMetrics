---
sort: 23
---

# Troubleshooting

This document contains troubleshooting guides for most common issues when working with VictoriaMetrics:

- [Unexpected query results](#unexpected-query-results)
- [Slow data ingestion](#slow-data-ingestion)
- [Slow queries](#slow-queries)
- [Out of memory errors](#out-of-memory-errors)


## Unexpected query results

If you see unexpected or unreliable query results from VictoriaMetrics, then try the following steps:

1. Check whether simplified queries return unexpected results. For example, if the query looks like
  `sum(rate(http_requests_total[5m])) by (job)`, then check whether the following queries return
   expected results:

   - Remove the outer `sum`: `rate(http_requests_total[5m])`. If this query returns too many time series,
     then try adding more specific label filters to it. For example, if you see that the original query
     returns unexpected results for the `job="foo"`, then use `rate(http_requests_total{job="foo"}[5m])` query.
     If this isn't enough, then continue adding more specific label filters, so the resulting query returns
     manageable number of time series.

   - Remove the outer `rate`: `http_requests_total`. Additional label filters may be added here in order
     to reduce the number of returned series.

2. If the simplest query continues returning unexpected / unreliable results, then export raw samples
   for this query via [/api/v1/export](https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format)
   on the given '[start..end]' time range and check whether they are expected:

   ```console
   curl http://victoriametrics:8428/api/v1/export -d 'match[]=http_requests_total' -d 'start=...' -d 'end=...'
   ```

   Note that responses returned from [/api/v1/query](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries)
   and from [/api/v1/query_range](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries) contain **evaluated** data
   instead of raw samples stored in VictoriaMetrics. See [these docs](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness)
   for details.

3. Sometimes response caching may lead to unexpected results when samples with older timestamps
   are ingested into VictoriaMetrics (aka [backfilling](https://docs.victoriametrics.com/#backfilling)).
   Try disabling response cache and see whether this helps. This can be done in the following ways:

   - By passing `-search.disableCache` command-line flag to a single-node VictoriaMetrics
     or to all the `vmselect` components if cluster version of VictoriaMetrics is used.

   - By passing `nocache=1` query arg to every request to `/api/v1/query` and `/api/v1/query_range`.
     If you use Grafana, then this query arg can be specified in `Custom Query Parameters` field
     at Prometheus datasource settings - see [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.

4. If you use cluster version of VictoriaMetrics, then it may return partial responses by default
   when some of `vmstorage` nodes are temporarily unavailable - see [cluster availability docs](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-availability)
   for details. If you want prioritizing query consistency over cluster availability,
   then you can pass `-search.denyPartialResponse` command-line flag to all the `vmselect` nodes.
   In this case VictoriaMetrics returns an error during querying if at least a single `vmstorage` node is unavailable.
   Another option is to pass `deny_partial_response=1` query arg to `/api/v1/query` and `/api/v1/query_range`.
   If you use Grafana, then this query arg can be specified in `Custom Query Parameters` field
   at Prometheus datasource settings - see [these docs](https://grafana.com/docs/grafana/latest/datasources/prometheus/) for details.

5. If you pass `-replicationFactor` command-line flag to `vmselect`, then it is recommended removing this flag from `vmselect`,
   since it may lead to incomplete responses when `vmstorage` nodes contain less than `-replicationFactor`
   copies of the requested data.

6. Try upgrading to the [latest available version of VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
   and verifying whether the issue is fixed there.

7. Try executing the query with `trace=1` query arg. This enables query tracing, which may contain
   useful information on why the query returns unexpected data. See [query tracing docs](https://docs.victoriametrics.com/#query-tracing) for details.

8. Inspect command-line flags passed to VictoriaMetrics components. If you don't understand clearly the purpose
   or the effect of some flags, then remove them from the list of flags passed to VictoriaMetrics components,
   because some command-line flags may change query results in unexpected ways when set to improper values.
   VictoriaMetrics is optimized for running with default flag values (e.g. when they aren't set explicitly).

9. If the steps above didn't help identifying the root cause of unexpected query results,
   then [file a bugreport](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new) with details on how to reproduce the issue.


## Slow data ingestion

There are the following most commons reasons for slow data ingestion in VictoriaMetrics:

1. Memory shortage for the given amounts of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series).

  VictoriaMetrics (or `vmstorage` in cluster version of VictoriaMetrics) maintains an in-memory cache
  for quick search for internal series ids per each incoming metric.
  This cache is named `storage/tsid`. VictoriaMetrics automatically determines the maximum size for this cache
  depending on the available memory on the host where VictoriaMetrics (or `vmstorage`) runs. If the cache size isn't enough
  for holding all the entries for active time series, then VictoriaMetrics locates the needed data on disk,
  unpacks it, re-constructs the missing entry and puts it into the cache. This takes additional CPU time and disk read IO.

  The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
  contain `Slow inserts` graph, which shows the cache miss percentage for `storage/tsid` cache
  during data ingestion. If `slow inserts` graph shows values greater than 5% for more than 10 minutes,
  then it is likely the current number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series)
  cannot fit the `storage/tsid` cache.

  There are the following solutions exist for this issue:

  - To increase the available memory on the host where VictoriaMetrics runs until `slow inserts` percentage
    will become lower than 5%. If you run VictoriaMetrics cluster, then you need increasing total available
    memory at `vmstorage` nodes. This can be done in two ways: either increasing the available memory
    per each existing `vmstorage` node or to add more `vmstorage` nodes to the cluster.

  - To reduce the number of active time series. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
    contain a graph showing the number of active time series. Recent versions of VictoriaMetrics
    provide [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer),
    which can help determining and fixing the source of [high cardinality](https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality).

2. [High churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate),
  e.g. when old time series are substituted with new time series at a high rate.
  When VitoriaMetrics encounters a sample for new time series, it needs to register the time series
  in the internal index (aka `indexdb`), so it can be quickly located on subsequent select queries.
  The process of registering new time series in the internal index is an order of magnitude slower
  than the process of adding new sample to already registered time series.
  So VictoriaMetrics may work slower than expected under [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).

  The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
  provides `Churn rate` graph, which shows the average number of new time series registered
  during the last 24 hours. If this number exceeds the number of [active time series](https://docs.victoriametrics.com/FAQ.html#what-is-an-active-time-series),
  then you need to identify and fix the source of [high churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).
  The most commons source of high churn rate is a label, which frequently change its value. Try avoiding such labels.
  The [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer) can help identifying
  such labels.

3. Resource shortage. The [official Grafana dashboards for VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
   contain `resource usage` graphs, which show memory usage, CPU usage, disk IO usage and free disk size.
   Make sure VictoriaMetrics has enough free resources for graceful handling of potential spikes in workload
   according to the following recommendations:

   - 50% of free CPU
   - 30% of free memory
   - 20% of free disk space

   If VictoriaMetrics components have lower amounts of free resources, then this may lead
   to **significant** performance degradation during data ingestion.
   For example:

   - If the percentage of free CPU is close to 0, then VictoriaMetrics
     may experience arbitrary long delays during data ingestion when it cannot keep up
     with the data ingestion rate.

   - If the percentage of free memory reaches 0, then the Operating System where VictoriaMetrics components run
     may have no enough memory for [page cache](https://en.wikipedia.org/wiki/Page_cache).
     VictoriaMetrics relies on page cache for quick queries over recently ingested data.
     If the operating system has no enough free memory for page cache, then it needs
     to re-read the requested data from disk. This may **significantly** increase disk read IO.

   - If free disk space is lower than 20%, then VictoriaMetrics is unable to perform optimal
     background merge of the incoming data. This leads to increased number of data files on disk,
     which, in turn, slows down both data ingestion and querying. See [these docs](https://docs.victoriametrics.com/#storage) for details.

4. If you run cluster version of VictoriaMetrics, then make sure `vminsert` and `vmstorage` components
   are located in the same network with short network latency between them.
   `vminsert` packs incoming data into in-memory packets and sends them to `vmstorage` on-by-one.
   It waits until `vmstorage` returns back `ack` response before sending the next packet.
   If the network latency between `vminsert` and `vmstorage` is big (for example, if they run in different datacenters),
   then this may become limiting factor for data ingestion speed.

   The [official Grafana dashboard for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#monitoring)
   contain `connection saturation` graph for `vminsert` components. If these graphs reach 100%,
   then it is likely you have issues with network latency between `vminsert` and `vmstorage`.
   Another possible issue for 100% connection saturation between `vminsert` and `vmstorage`
   is resource shortage at `vmstorage` nodes. In this case you need to increase amounts
   of available resources (CPU, RAM, disk IO) at `vmstorage` nodes or to add more `vmstorage` nodes to the cluster.

5. Noisy neighboor. Make sure VictoriaMetrics components run in envirnoments without other resource-hungry apps.
   Such apps may steal RAM, CPU, disk IO and network bandwidth, which is needed for VictoriaMetrics components.

## Slow queries

Some queries may take more time and resources (CPU, RAM, network bandwidth) than others.
VictoriaMetrics logs slow queries if their execution time exceeds the duration passed
to `-search.logSlowQueryDuration` command-line flag.
VictoriaMetrics also provides `/api/v1/status/top_queries` endpoint, which returns
queries took the most time to execute.
See [these docs](https://docs.victoriametrics.com/#prometheus-querying-api-enhancements) for details.

There are the following solutions exist for slow queries:

- Adding more CPU and memory to VictoriaMetrics, so it may perform the slow query faster.
  If you use cluster version of VictoriaMetrics, then migration of `vmselect` nodes to machines
  with more CPU and RAM should help improving speed for slow queries.
  Sometimes adding more `vmstorage` nodes also can help improving the speed for slow queries.

- Rewriting slow queries, so they become faster. Unfortunately it is hard determining
  whether the given query will be slow by just looking at it.
  VictoriaMetrics provides [query tracing](https://docs.victoriametrics.com/#query-tracing) functionality,
  which can help determine the source of slow query.
  See also [this article](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986),
  which explains how to determine and optimize slow queries.


## Out of memory errors

There are the following most common sources of out of memory (aka OOM) crashes in VictoriaMetrics:

1. Improper command-line flag values. Inspect command-line flags passed to VictoriaMetrics components.
   If you don't understand clearly the purpose or the effect of some flags, then remove them
   from the list of flags passed to VictoriaMetrics components, because some command-line flags
   may lead to increased memory usage and increased CPU usage. The increased memory usage increases chances for OOM crashes.
   VictoriaMetrics is optimized for running with default flag values (e.g. when they aren't set explicitly).

   For example, it isn't recommended tuning cache sizes in VictoriaMetrics, since it frequently leads to OOM.
   [These docs](https://docs.victoriametrics.com/#cache-tuning) refer command-line flags, which aren't
   recommended to tune. If you see that VictoriaMetrics needs increasing some cache sizes for the current workload,
   then it is better migrating to a host with more memory instead of trying to tune cache sizes.

2. Unexpected heavy queries. The query is considered heavy if it needs to select and process millions of unique time series.
   Such query may lead to OOM, since VictoriaMetrics needs to keep some per-series data in memory.
   VictoriaMetrics provides various settings, which can help limiting resource usage in this case -
   see [these docs](https://docs.victoriametrics.com/#resource-usage-limits).
   See also [this article](https://valyala.medium.com/how-to-optimize-promql-and-metricsql-queries-85a1b75bf986),
   which explains how to detect and optimize heavy queries.
   VictoriaMetrics also provides [query tracer](https://docs.victoriametrics.com/#query-tracing),
   which may help identifying the source of heavy query.

3. Lack of free memory for processing workload spikes. If VictoriaMetrics components use almost all the available memory
   under the current workload, then it is recommended migrating to a host with bigger amounts of memory
   in order to protect from possible OOM crashes on workload spikes. It is recommended to have at least 30%
   of free memory for graceful handling of possible workload spikes.
