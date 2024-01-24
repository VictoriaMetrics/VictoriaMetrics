---
sort: 2
weight: 2
title: VictoriaLogs key concepts
menu:
  docs:
    parent: "victorialogs"
    weight: 2
    title: Key concepts
aliases:
- /VictoriaLogs/keyConcepts.html
---

# VictoriaLogs key concepts

## Data model

[VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/) works with both structured and unstructured logs.
Every log entry must contain at least [log message field](#message-field) plus arbitrary number of additional `key=value` fields.
A single log entry can be expressed as a single-level [JSON](https://www.json.org/json-en.html) object with string keys and values.
For example:

```json
{
  "job": "my-app",
  "instance": "host123:4567",
  "level": "error",
  "client_ip": "1.2.3.4",
  "trace_id": "1234-56789-abcdef",
  "_msg": "failed to serve the client request"
}
```

VictoriaLogs automatically transforms multi-level JSON (aka nested JSON) into single-level JSON
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) according to the following rules:

- Nested dictionaries are flattened by concatenating dictionary keys with `.` char. For example, the following multi-level JSON
  is transformed into the following single-level JSON:

  ```json
  {
    "host": {
      "name": "foobar"
      "os": {
        "version": "1.2.3"
      }
    }
  }
  ```

  ```json
  {
    "host.name": "foobar",
    "host.os.version": "1.2.3"
  }
  ```

- Arrays, numbers and boolean values are converted into strings. This simplifies [full-text search](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html) over such values.
  For example, the following JSON with an array, a number and a boolean value is converted into the following JSON with string values:

  ```json
  {
    "tags": ["foo", "bar"],
    "offset": 12345,
    "is_error": false
  }
  ```

  ```json
  {
    "tags": "[\"foo\", \"bar\"]",
    "offset": "12345",
    "is_error": "false"
  }
  ```

Both label name and label value may contain arbitrary chars. Such chars must be encoded
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
according to [JSON string encoding](https://www.rfc-editor.org/rfc/rfc7159.html#section-7).
Unicode chars must be encoded with [UTF-8](https://en.wikipedia.org/wiki/UTF-8) encoding:

```json
{
  "label with whitepsace": "value\nwith\nnewlines",
  "Поле": "价值",
}
```

VictoriaLogs automatically indexes all the fields in all the [ingested](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) logs.
This enables [full-text search](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html) across all the fields.

VictoriaLogs supports the following field types:

* [`_msg` field](#message-field)
* [`_time` field](#time-field)
* [`_stream` fields](#stream-fields)
* [other fields](#other-fields)


### Message field

Every ingested [log entry](#data-model) must contain at least a `_msg` field with the actual log message. For example, this is the minimal
log entry, which can be ingested into VictoriaLogs:

```json
{
  "_msg": "some log message"
}
```

If the actual log message has other than `_msg` field name, then it is possible to specify the real log message field
via `_msg_field` query arg during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
For example, if log message is located in the `event.original` field, then specify `_msg_field=event.original` query arg
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).

### Time field

The ingested [log entries](#data-model) may contain `_time` field with the timestamp of the ingested log entry.
For example:

```json
{
  "_msg": "some log message",
  "_time": "2023-04-12T06:38:11.095Z"
}
```

If the actual timestamp has other than `_time` field name, then it is possible to specify the real timestamp
field via `_time_field` query arg during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
For example, if timestamp is located in the `event.created` field, then specify `_time_field=event.created` query arg
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).

If `_time` field is missing, then the data ingestion time is used as log entry timestamp.

The log entry timestamp allows quickly narrowing down the search to a particular time range.
See [these docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#time-filter) for details.

### Stream fields

Some [structured logging](#data-model) fields may uniquely identify the application instance, which generates log entries.
This may be either a single field such as `instance=host123:456` or a set of fields such as
`(datacenter=..., env=..., job=..., instance=...)` or
`(kubernetes.namespace=..., kubernetes.node.name=..., kubernetes.pod.name=..., kubernetes.container.name=...)`.

Log entries received from a single application instance form a log stream in VictoriaLogs.
VictoriaLogs optimizes storing and querying of individual log streams. This provides the following benefits:

- Reduced disk space usage, since a log stream from a single application instance is usually compressed better
  than a mixed log stream from multiple distinct applications.

- Increased query performance, since VictoriaLogs needs to scan lower amounts of data
  when [searching by stream labels](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stream-filter).

VictoriaLogs cannot determine automatically, which fields uniquely identify every log stream,
so it stores all the received log entries in a single default stream - `{}`.
This may lead to not-so-optimal resource usage and query performance.

Therefore it is recommended specifying stream-level fields via `_stream_fields` query arg
during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).
For example, if logs from Kubernetes containers have the following fields:

```json
{
  "kubernetes.namespace": "some-namespace",
  "kubernetes.node.name": "some-node",
  "kubernetes.pod.name": "some-pod",
  "kubernetes.container.name": "some-container",
  "_msg": "some log message"
}
```

then sepcify `_stream_fields=kubernetes.namespace,kubernetes.node.name,kubernetes.pod.name,kubernetes.container.name`
query arg during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) in order to properly store
per-container logs into distinct streams.

#### How to determine which fields must be associated with log streams?

[Log streams](#stream-fields) can be associated with fields, which simultaneously meet the following conditions:

- Fields, which remain constant across log entries received from a single application instance.
- Fields, which uniquely identify the application instance. For example, `instance`, `host`, `container`, etc.

Sometimes a single application instance may generate multiple log streams and store them into distinct log files.
In this case it is OK to associate the log stream with filepath fields such as `log.file.path` additionally to instance-specific fields.

Structured logs may contain big number of fields, which do not change across log entries received from a single application instance.
There is no need in associating all these fields with log stream - it is enough to associate only those fields, which uniquely identify
the application instance across all the ingested logs. Additionally, some fields such as `datacenter`, `environment`, `namespace`, `job` or `app`,
can be associated with log stream in order to optimize searching by these fields with [stream filtering](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html#stream-filter).

Never associate log streams with fields, which may change across log entries of the same application instance. See [these docs](#high-cardinality) for details.

#### High cardinality

Some fields in the [ingested logs](#data-model) may contain big number of unique values across log entries.
For example, fields with names such as `ip`, `user_id` or `trace_id` tend to contain big number of unique values.
VictoriaLogs works perfectly with such fields unless they are associated with [log streams](#stream-fields).

Never associate high-cardinality fields with [log streams](#stream-fields), since this may result
to the following issues:

- Performance degradation during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
  and [querying](https://docs.victoriametrics.com/VictoriaLogs/querying/)
- Increased memory usage
- Increased CPU usage
- Increased disk space usage
- Increased disk read / write IO

VictoriaLogs exposes `vl_streams_created_total` [metric](https://docs.victoriametrics.com/VictoriaLogs/#monitoring),
which shows the number of created streams since the last VictoriaLogs restart. If this metric grows at a rapid rate
during long period of time, then there are high chances of high cardinality issues mentioned above.
VictoriaLogs can log all the newly registered streams when `-logNewStreams` command-line flag is passed to it.
This can help narrowing down and eliminating high-cardinality fields from [log streams](#stream-fields).

### Other fields

The rest of [structured logging](#data-model) fields are optional. They can be used for simplifying and optimizing search queries.
For example, it is usually faster to search over a dedicated `trace_id` field instead of searching for the `trace_id` inside long log message.
E.g. the `trace_id:XXXX-YYYY-ZZZZ` query usually works faster than the `_msg:"trace_id=XXXX-YYYY-ZZZZ"` query.

See [LogsQL docs](https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html) for more details.

