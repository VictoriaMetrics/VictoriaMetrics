---
weight: 120
title: SQL to LogsQL tutorial
menu:
  docs:
    parent: "victorialogs"
    weight: 120
---

This is a tutorial for the migration from SQL to [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/).
It is expected you are familiar with SQL and know [how to execute queries at VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).


## data model

SQL is usually used for querying relational tables. Every such table contains a pre-defined set of columns with pre-defined types.
LogsQL is used for querying logs. Logs are stored in [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
So log streams is an analogue of tables in relational databases. Log streams and relational tables have the following major differences:

- Log streams are created automatically when the first log entry (row) is ingested into them.
- There is no pre-defined scheme in log streams - logs with arbitrary set of fields can be ingested into every log stream.
  Both names and values in every log entry have string type. They may contain arbitrary string data.
- Every log entry (row) can be represented as a flat JSON object: `{"f1":"v1",...,"fN":"vN"}`. See [these docs](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model).
- By default VictoriaLogs selects log entries across all the log streams. The needed set of log streams can be specified
  via [stream filters](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter).
- By default VictoriaLogs returns all the fields across the selected logs. The set of returned fields
  can be limited with [`fields` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe).

## query structure

SQL query structure is quite convoluted:

```sql
SELECT
  <fields, aggregations, calculations, transformations>
FROM <table>
  <optional JOINs>
  <optional filters with optional subqueries>
  <optional GROUP BY>
  <optional HAVING>
  <optional ORDER BY>
  <optional LIMIT / OFFSET>
  <optional UNION>
```

[LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/) query structure is much simpler:

```logsql
<filters>
  | <optional_pipe1>
  | ...
  | <optional_pipeN>
```

The `<filters>` part selects the needed logs (rows) according to the provided [filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters).
Then the provided [pipes](https://docs.victoriametrics.com/victorialogs/logsql/#pipes) are executed sequentlially.
Every such pipe receives all the rows from the previous stage, performs some calculations and/or transformations,
and then pushes the resulting rows to the next stage. This simplifies reading and understanding the query - just read it from the beginning
to the end in order to understand what does it do at every stage.

LogsQL pipes cover all the functionality from SQL: aggregations, calculations, transformations, subqueries, joins, post-filters, sorting, etc.
See the [conversion rules](#conversion-rules) on how to convert SQL to LogsQL.

## conversion rules

The following rules must be used for converting SQL query into LogsQL query:

* If the SQL query contains `WHERE`, then convert it into [LogsQL filters](https://docs.victoriametrics.com/victorialogs/logsql/#filters).
  Otherwise just start LogsQL query with [`*`](https://docs.victoriametrics.com/victorialogs/logsql/#any-value-filter).
  For example, `SELECT * FROM table WHERE field1=value1 AND field2<>value2` is converted into `field1:=value1 field2:!=value2`,
  while `SELECT * FROM table` is converted into `*`.
* `IN` subqueries inside `WHERE` must be converted into [`in` filters](https://docs.victoriametrics.com/victorialogs/logsql/#multi-exact-filter).
  For example, `SELECT * FROM table WHERE id IN (SELECT id2 FROM table)` is converted into `id:in(* | fields id2)`.
* If the `SELECT` part isn't equal to `*` and there are no `GROUP BY` / aggregate functions in the SQL query, then enumerate
  the selected columns at [`fields` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#fields-pipe).
  For example, `SELECT field1, field2 FROM table` is converted into `* | fields field1, field2`.
* If the SQL query contains `JOIN`, then convert it into [`join` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#join-pipe).
* If the SQL query contains `GROUP BY` / aggregate functions, then convert them to [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe).
  For example, `SELECT count(*) FROM table` is converted into `* | count()`, while `SELECT user_id, count(*) FROM table GROUP BY user_id`
  is converted to `* | stats by (user_id) count()`. Note how the LogsQL query mentions the `GROUP BY` fields only once,
  while SQL forces mentioning these fields twice - at the `SELECT` and at the `GROUP BY`. How many times did you hit the discrepancy
  between `SELECT` and `GROUP BY` fields?
* If the SQL query contains additional calculations and/or transformations at the `SELECT`, which aren't covered yet by `GROUP BY`,
  then convert them into the corresponding [LogsQL pipes](https://docs.victoriametrics.com/victorialogs/logsql/#pipes).
  The most frequently used pipes are [`math`](https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe)
  and [`format`](https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe).
  For example, `SELECT field1 + 10 AS x, CONCAT("foo", field2) AS y FROM table` is converted into `* | math field1 + 10 as x | format "foo<field2>" as y | fields x, y`.
* If the SQL query contains `HAVING`, then convert it into [`filter` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe).
  For example, `SELECT user_id, count(*) AS c FROM table GROUP BY user_id HAVING c > 100` is converted into `* | stats by (user_id) count() c | filter c:>100`.
* If the SQL query contains `ORDER BY`, `LIMIT` and `OFFSET`, then convert them into [`sort` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe).
  For example, `SELECT * FROM table ORDER BY field1, field2 LIMIT 10 OFFSET 20` is converted into `* | sort by (field1, field2) limit 10 offset 20`.
* If the SQL query contains `UNION`, then convert it into [`union` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#union-pipe).
  For example `SELECT * FROM table WHERE filters1 UNION ALL SELECT * FROM table WHERE filters2` is converted into `filters1 | union (filters2)`.

SQL queries are frequently used for obtaining top N column values, which are the most frequently seen in the selected rows.
For example, the query below returns top 5 `user_id` values, which present in the biggest number of rows:

```sql
SELECT user_id, count(*) hits FROM table GROUP BY user_id ORDER BY hits DESC LIMIT 5
```

LogsQL provides a shortcut syntax with [`top` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#top-pipe) for this case:

```logsql
* | top 5 (user_id)
```

It is equivalent to the longer LogsQL query:

```logsql
* | by (user_id) count() hits | sort by (hits desc) limit 5
```

[LogsQL pipes](https://docs.victoriametrics.com/victorialogs/logsql/#pipes) support much wider functionality comparing to SQL,
so spend your spare time by reading [pipe docs](https://docs.victoriametrics.com/victorialogs/logsql/) and playing with them
at [VictoriaLogs playground](https://play-vmlogs.victoriametrics.com/).
