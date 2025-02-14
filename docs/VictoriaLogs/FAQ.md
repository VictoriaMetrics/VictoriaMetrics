---
weight: 6
title: FAQ
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

## Is VictoriaLogs ready for production use?

Yes. VictoriaLogs is ready for production use starting from [v1.0.0](https://docs.victoriametrics.com/victorialogs/changelog/).

## What is the difference between VictoriaLogs and Elasticsearch (OpenSearch)?

Both Elasticsearch and VictoriaLogs allow ingesting structured and unstructured logs
and performing fast full-text search over the ingested logs.

Elasticsearch and OpenSearch are designed as general-purpose databases for fast full-text search over large set of documents.
They aren't optimized specifically for logs. This results in the following issues, which are resolved by VictoriaLogs:

- High RAM usage
- High disk space usage
- Non-trivial index setup
- Inability to select more than 10K matching log lines in a single query with default configs

VictoriaLogs is optimized specifically for logs. So it provides the following features useful for logs, which are missing in Elasticsearch:

- Easy to setup and operate. There is no need in tuning configuration for optimal performance or in creating any indexes for various log types.
  Just run VictoriaLogs on the most suitable hardware, ingest logs into it via [supported data ingestion protocols](https://docs.victoriametrics.com/victorialogs/data-ingestion/)
  and get the best available performance out of the box.
- Up to 30x less RAM usage than Elasticsearch for the same workload. See [this article](https://itnext.io/how-do-open-source-solutions-for-logs-work-elasticsearch-loki-and-victorialogs-9f7097ecbc2f) for details.
- Up to 15x less disk space usage than Elasticsearch for the same amounts of stored logs.
- Ability to work efficiently with hundreds of terabytes of logs on a single node.
- Easy to use query language optimized for typical log analysis tasks - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).
- Fast full-text search over all the [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) out of the box.
- Good integration with traditional command-line tools for log analysis. See [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line).


## What is the difference between VictoriaLogs and Grafana Loki?

Both Grafana Loki and VictoriaLogs are designed for log management and processing.
Both systems support [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) concept.

VictoriaLogs and Grafana Loki have the following differences:

- VictoriaLogs is much easier to setup and operate than Grafana Loki. There is no need in non-trivial tuning -
  it works great with default configuration.

- VictoriaLogs performs typical full-text search queries up to 1000x faster than Grafana Loki.

- Grafana Loki doesn't support log fields with many unique values (aka high cardinality labels) such as `user_id`, `trace_id` or `ip`.
  It consumes huge amounts of RAM and slows down significantly when logs with high-cardinality fields are ingested into it.
  See [these docs](https://grafana.com/docs/loki/latest/best-practices/) for details.

  VictoriaLogs supports high-cardinality [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  out of the box without any additional configuration. It automatically indexes all the ingested log fields,
  so fast full-text search over any log field works without issues.

- Grafana Loki provides very inconvenient query language - [LogQL](https://grafana.com/docs/loki/latest/logql/).
  This query language is hard to use for typical log analysis tasks.

  VictoriaLogs provides easy to use query language for typical log analysis tasks - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).

- VictoriaLogs usually needs less RAM and storage space than Grafana Loki for the same amounts of logs.


## What is the difference between VictoriaLogs and ClickHouse?

ClickHouse is an extremely fast and efficient analytical database. It can be used for logs storage, analysis and processing.
VictoriaLogs is designed solely for logs. VictoriaLogs uses [similar design ideas as ClickHouse](#how-does-victorialogs-work) for achieving high performance.

- ClickHouse is good for logs if you know the set of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
  and the expected query types beforehand. Then you can create a table with a column per each log field, and use the most optimal settings for the table -
  sort order, partitioning and indexing - for achieving the maximum possible storage efficiency and query performance.

  If the expected log fields or the expected query types aren't known beforehand, or if they may change over any time,
  then ClickHouse can still be used, but its' efficiency may suffer significantly depending on how you design the database schema for log storage.

  VictoriaLogs works optimally with any log types out of the box - structured, unstructured and mixed.
  It works optimally with any sets of [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
  which can change in any way across different log sources.

- ClickHouse provides SQL dialect with additional analytical functionality. It allows performing arbitrary complex analytical queries
  over the stored logs.

  VictoriaLogs provides easy to use query language with full-text search specifically optimized
  for log analysis - [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).
  LogsQL is usually easier to use than SQL for typical log analysis tasks, while some
  non-trivial analytics may require SQL power.

- VictoriaLogs accepts logs from popular log shippers out of the box - see [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/).

  ClickHouse needs an intermediate applications for converting the ingested logs into `INSERT` SQL statements for the particular database schema.
  This may increase the complexity of the system and, subsequently, increase its' maintenance costs.

- VictoriaLogs provides [built-in Web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui) for logs' exploration.


## How does VictoriaLogs work?

VictoriaLogs accepts logs as [JSON entries](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
Then it stores log fields into distinct data blocks. E.g. values for the same log field across multiple log entries
are stored in a single data block. This allows reading data blocks only for the needed fields during querying.

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
  For example, it encodes IP addresses into 4 bytes. Custom fields' encoding reduces data size on disk and improves query performance.
- It physically groups logs for the same [log stream](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
  close to each other in the storage. This improves compression ratio, which helps reducing disk space usage. This also improves query performance
  by skipping blocks for unneeded streams when [stream filter](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) is used.
- It maintains sparse index for [log timestamps](https://docs.victoriametrics.com/victorialogs/keyconcepts/#time-field),
  which allow improving query performance when [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) is used.

## How to export logs from VictoriaLogs?

Just send the query with the needed [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters)
to [`/select/logsql/query`](https://docs.victoriametrics.com/victorialogs/querying/#querying-logs) - VictoriaLogs will return
the requested logs as a [stream of JSON lines](https://jsonlines.org/). It is recommended specifying [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter)
for limiting the amounts of exported logs.

## I want to ingest logs without message field, is that possible?

Starting from version `v0.30.0`, VictoriaLogs started blocking the ingestion of logs **without a message field**, as it is a requirement of the [VictoriaLogs data model](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field). 

However, some logs do not have a message field and only contain other fields, such as logs in [this comment](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/7056#issuecomment-2434189718) and [this slack thread](https://victoriametrics.slack.com/archives/C05UNTPAEDN/p1730982146818249). Therefore, starting from version `v0.39.0`, logs without a message field are **allowed to be ingested**, 
and their message field will be recorded as: 
```json
{"_msg": "missing _msg field; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field"}
```

The default message field value can be changed using the `-defaultMsgValue` flag, for example, `-defaultMsgValue=foo`.

Please note that the message field is **crucial** for VictoriaLogs, so it is important to fill it with meaningful content.

## What if my logs have multiple message fields candidates?

When ingesting with VictoriaLogs, the message fields is specified through `_msg_field` param, which can accept **multiple fields**, and the **first non-empty field** will be used as the message field. 
Here is an example URL when pushing logs to VictoriaLogs with Promtail:
```yaml
clients:
  - url: http://localhost:9428/insert/loki/api/v1/push?_stream_fields=instance,job,host,app&_msg=message,body
```

For the following log, its `_msg` will be `foo bar in message`:
```json
{
  "message": "foo bar in message",
  "body": "foo bar in body"
}
```

And for the following log, its `_msg` will be `foo bar in body`:
```json
{
  "message": "",
  "body": "foo bar in body"
}
```

## What length a log record is expected to have?

VictoriaLogs works optimally with log records of up to `10KB`. It works OK with
log records of up to `100KB`. It works not so optimal with log records exceeding
`100KB`.

The max size of a log record VictoriaLogs can accept during [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/)
is `2MB`, because log records are stored in blocks of up to `2MB` size.
Blocks of this size fit the L2 cache of a typical CPU, which gives an
optimal performance during data ingestion and querying.

Note that log records with sizes close to `2MB` aren't handled efficiently by
VictoriaLogs because per-block overhead translates to a single log record, and
this overhead is big.

The `2MB` limit is hadrcoded and is unlikely to increase.

The limit can be set to the lower value during [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/)
via `-insert.maxLineSizeBytes` command-line flag.

## What is the maximum supported field name length

VictoriaLogs limits [log field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) name length to 128 bytes -
Log entries with longer field names are ignored during [date ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/).

The maximum length of a field name is hardcoded and is unikely to increase, since this may increase RAM and CPU usage.

## How many fields a single log entry may contain

A single log entry may contain up to 2000 fields. This fits well the majority of use cases for structured logs and
for [wide events](https://jeremymorrell.dev/blog/a-practitioners-guide-to-wide-events/).

The maximum number of fields per log entry is hardcoded and is unlikely to increase, since this may increase RAM and CPU usage.

The limit can be set to the lower value during [data ingestion](https://docs.victoriametrics.com/victorialogs/data-ingestion/)
via `-insert.maxFieldsPerLine` command-line flag.

## How to determine which log fields occupy the most of disk space?

[Run](https://docs.victoriametrics.com/victorialogs/querying/) the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query
based on [`block_stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#block_stats-pipe):

```logsql
_time:1d
  | block_stats
  | stats by (field)
      sum(values_bytes) as values_bytes,
      sum(bloom_bytes) as bloom_bytes,
      sum(rows) as rows
  | math
      (values_bytes+bloom_bytes) as total_bytes,
      round(total_bytes / rows, 0.01) as bytes_per_row
  | first 10 (total_bytes desc)
```

This query returns top 10 [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model),
which occupy the most of disk space across the logs ingested during the last day. The occupied disk space
is returned in the `total_bytes` field.

If you use [VictoriaLogs web UI](https://docs.victoriametrics.com/victorialogs/querying/#web-ui)
or [Grafana plugin for VictoriaLogs](https://docs.victoriametrics.com/victorialogs/victorialogs-datasource/),
then make sure the selected time range covers the last day. Otherwise the query above returns
results on the intersection of the last day and the selected time range.

See [why the log field occupies a lot of disk space](#why-the-log-field-occupies-a-lot-of-disk-space).

## Why the log field occupies a lot of disk space?

See [how to determine which log fields occupy the most of disk space](#how-to-determine-which-log-fields-occupy-the-most-of-disk-space).
Log field may occupy a lot of disk space if it contains values with many unique parts (aka "random" values).
Such values do not compress well, so they occupy a lot of disk space. If you want reducing the amounts of occupied disk space,
then either remove the given log field from the [ingested](https://docs.victoriametrics.com/victorialogs/data-ingestion/) logs
or remove the unique parts from the log field before ingesting it into VictoriaLogs.

## How to detect the most frequently seen logs?

Use [`collapse_nums` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#collapse_nums-pipe).
For example, the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query
returns top 10 the most freqently seen [log messages](https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field) over the last hour:

```logsql
_time:1h | collapse_nums prettify | top 10 (_msg)
```

Add [`_stream` field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to the `top (...)` list in order to get top 10 the most frequently seen logs with the `_stream` field:

```logsql
_time:1h | collapse_nums prettify | top 10 (_stream, _msg)
```

## How to get field names seen in the selected logs?

Use [`field_names` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#field_names-pipe).
For example, the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query
returns all the [field names](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) seen
across all the logs during the last hour:

```logsql
_time:1h | field_names | sort by (name)
```

The `hits` field in the returned results contains an estimated number of logs with the given log field.

## How to get unique field values seen in the selected logs?

Use [`field_values` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#field_values-pipe).
For example, the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query
returns all the values for the `level` field across all the logs seen during the last hour:

```logsql
_time:1h | field_values level
```

The `hits` field in the returned results contains an esitmated number of logs with the given value for the `level` field.

## How to get the number of unique log streams on the given time range?

Use [`count_uniq` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#count_uniq-pipe)
over [`_stream`](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) field.
For example, the following [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query
returns the number of unique [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
across all the logs over the last day:

```logsql
_time:1d | count_uniq(_stream)
```

## Does LogsQL support subqueries?

LogsQL supports subqieries via [`in(<subquery>)` filter](https://docs.victoriametrics.com/victorialogs/logsql/#multi-exact-filter).
For example, the following query returns the total number of unique values for the `user_id` [field](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
across top 3 [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) with the biggest number of logs during the last hour:

```logsql
_time:1h _stream_id:in(_time:1h | top 3 (_stream_id) | keep _stream_id) | count_uniq(user_id)
```

The query works in the following way:

- It selects top 3 log streams with the biggest number of logs during the last hour with the following subquery:
  ```logsql
  _time:1h | top 3 (_stream_id) | keep _stream_id
  ```
  This subquery uses [`top`](https://docs.victoriametrics.com/victorialogs/logsql/#top-pipe) and [`keep`](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe) pipes.

- Then it selects all the logs across the selected log streams over the last hour with the help of [`_stream_id:...` filter](https://docs.victoriametrics.com/victorialogs/logsql/#_stream_id-filter).

See also [`subquery filters`](https://docs.victoriametrics.com/victorialogs/logsql/#subquery-filter).
