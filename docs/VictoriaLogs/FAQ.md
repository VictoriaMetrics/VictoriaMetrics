---
sort: 6
weight: 6
title: VictoriaLogs FAQ
menu:
  docs:
    identifier: "victorialogs-faq"
    parent: "victorialogs"
    weight: 6
    title: FAQ
aliases:
- /VictoriaLogs/FAQ.html
- /VictoriaLogs/faq.html
---

# VictoriaLogs FAQ

## What is the difference between VictoriaLogs and Elasticsearch (OpenSearch)?

Both Elasticsearch and VictoriaLogs allow ingesting structured and unstructured logs
and performing fast full-text search over the ingested logs.

Elasticsearch and OpenSearch are designed as general-purpose databases for fast full-text search over large set of documents.
They aren't optimized specifically for logs. This results in the following issues, which are resolved by VictoriaLogs:

- High RAM usage
- High disk space usage
- Non-trivial index setup
- Inability to select more than 10K matching log lines in a single query

VictoriaLogs is optimized specifically for logs. So it provides the following features useful for logs, which are missing in Elasticsearch:

- Easy to setup and operate. There is no need in tuning configuration for optimal performance or in creating any indexes for various log types.
  Just run VictoriaLogs on the most suitable hardware - and it automatically provides the best performance.
- Up to 30x less RAM usage than Elasticsearch for the same workload.
- Up to 15x less disk space usage than Elasticsearch for the same amounts of stored logs.
- Ability to work with hundreds of terabytes of logs on a single node.
- Very easy to use query language optimized for typical log analysis tasks - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).
- Fast full-text search over all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) out of the box.
- Good integration with traditional command-line tools for log analysis. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line).


## What is the difference between VictoriaLogs and Grafana Loki?

Both Grafana Loki and VictoriaLogs are designed for log management and processing.
Both systems support [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) concept.

VictoriaLogs and Grafana Loki have the following differences:

- Grafana Loki doesn't support high-cardinality log fields (aka labels) such as `user_id`, `trace_id` or `ip`.
  It starts consuming huge amounts of RAM and working very slow when logs with high-cardinality fields are ingested into it.
  See [these docs](https://grafana.com/docs/loki/latest/best-practices/) for details.

  VictoriaMetrics supports high-cardinality [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
  It automatically indexes all the ingested log fields and allows performing fast full-text search over any field.

- Grafana Loki provides very inconvenient query language - [LogQL](https://grafana.com/docs/loki/latest/logql/).
  This query language is hard to use for typical log analysis tasks.

  VictoriaMetrics provides easy to use query language for typical log analysis tasks - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).

- VictoriaLogs performs typical full-text queries up to 1000x faster than Grafana Loki.

- VictoriaLogs needs less storage space than Grafana Loki for the same amounts of logs.

- VictoriaLogs is much easier to setup and operate than Grafana Loki.


## What is the difference between VictoriaLogs and ClickHouse?

ClickHouse is an extremely fast and efficient analytical database. It can be used for logs storage, analysis and processing.
VictoriaLogs is designed solely for logs. VictoriaLogs uses [similar design ideas as ClickHouse](#how-does-victorialogs-work) for achieving high performance.

- ClickHouse is good for logs if you know the set of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) beforehand.
  Then you can create a table with a column per each log field and achieve the maximum possible query performance.

  If the set of log fields isn't known beforehand, or if it can change at any time, then ClickHouse can still be used,
  but its' efficiency may suffer significantly depending on how you design the database schema for log storage.

  ClickHouse efficiency highly depends on the used database schema. It must be optimized for the particular workload
  for achieving high efficiency and query performance.

  VictoriaLogs works optimally with any log types out of the box - structured, unstructured and mixed.
  It works optimally with any sets of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
  which can change in any way across different log sources.

- ClickHouse provides SQL dialect with additional analytical functionality. It allows performing arbitrary complex analytical queries
  over the stored logs.

  VictoriaLogs provides easy to use query language with full-text search specifically optimized
  for log analysis - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).
  LogsQL is usually much easier to use than SQL for typical log analysis tasks, while some
  non-trivial analytics may require SQL power.

- VictoriaLogs accepts logs from popular log shippers out of the box - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/).

  ClickHouse needs an intermediate applications for converting the ingested logs into `INSERT` SQL statements for the particular database schema.
  This may increase the complexity of the system and, subsequently, increase its' maintenance costs.


## How does VictoriaLogs work?

VictoriaLogs accepts logs as [JSON entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
It then stores every field value into a distinct data block. E.g. values for the same field across multiple log entries
are stored in a single data block. This allow reading data blocks only for the needed fields during querying.

Data blocks are compressed before being saved to persistent storage. This allows saving disk space and improving query performance
when it is limited by disk read IO bandwidth.

Smaller data blocks are merged into bigger blocks in background. Data blocks are limited in size. If the size of data block exceeds the limit,
then it is split into multiple blocks of smaller sizes.

Every data block is processed in an atomic manner during querying. For example, if the data block contains at least a single value,
which needs to be processed, then the whole data block is unpacked and read at once. Data blocks are processed in parallel
on all the available CPU cores during querying. This allows scaling query performance with the number of available CPU cores.

This architecture is inspired by [ClickHouse architecture](https://clickhouse.com/docs/en/development/architecture).

On top of this, VictoriaLogs employs additional optimizations for achieving high query performance:

- It uses [bloom filters](https://en.wikipedia.org/wiki/Bloom_filter) for skipping blocks without the given
  [word](https://docs.victoriametrics.com/victorialogs/logsql/#word-filter) or [phrase](https://docs.victoriametrics.com/victorialogs/logsql/#phrase-filter).
- It uses custom encoding and compression for fields with different data types.
  For example, it encodes IP addresses as 4-byte tuples. Custom fields' encoding reduces data size on disk and improves query performance.
- It physically groups logs for the same [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
  close to each other. This improves compression ratio, which helps reducing disk space usage. This also improves query performance
  by skipping blocks for unneeded streams when [stream filter](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) is used.
- It maintains sparse index for [log timestamps](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field),
  which allow improving query performance when [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) is used.

## How to export logs from VictoriaLogs?

Just send the query with the needed [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to [`/select/logsql/query`](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs) - VictoriaLogs will return
the requested logs as a [stream of JSON lines](https://jsonlines.org/). It is recommended specifying [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the amounts of exported logs.
