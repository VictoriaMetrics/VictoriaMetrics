---
weight: 2
title: Key concepts
menu:
  docs:
    identifier: vt-key-concepts
    parent: victoriatraces
    weight: 2
    title: Key concepts
tags:
  - traces
aliases:
- /victoriatraces/keyConcepts.html
---

> VictoriaTraces is currently under active development and not ready for production use. It is built on top of VictoriaLogs and therefore shares some flags and APIs. These will be fully separated once VictoriaTraces reaches a stable release. Until then, features may change or break without notice.

## Data model

VictoriaTraces is built on VictoriaLogs. It's recommended to go through [the data model of VictoriaLogs](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) first.

**Every [trace span](https://github.com/open-telemetry/opentelemetry-proto/blob/v1.7.0/opentelemetry/proto/trace/v1/trace.proto) must contain `service.name` in its resource attributes and a span `name`**. They will be used as the [stream fields](#stream-fields). All other data of the span will be mapped as the ordinary fields.

For example, here's how a trace span looks like in an OTLP export request:

{{% collapse name="OTLP trace span" %}}

```json
{
  "ResourceSpans": [{
    "Resource": {
      "Attributes": {
        "service.name": "payment",
        "telemetry.sdk.language": "nodejs",
        "telemetry.sdk.name": "opentelemetry",
        "telemetry.sdk.version": "1.30.1",
        "container.id": "18ee03279d38ed0e0eedad037c260df78dfc3323aa662ca14a2d38fcc8bf3762",
        "service.namespace": "opentelemetry-demo",
        "service.version": "2.0.2",
        "host.name": "18ee03279d38",
        "host.arch": "arm64",
        "os.type": "linux",
        "os.version": "6.10.14-linuxkit",
        "process.pid": 17,
        "process.executable.name": "node",
        "process.executable.path": "/usr/local/bin/node",
        "process.command_args": [
          "/usr/local/bin/node",
          "--require",
          "./opentelemetry.js",
          "/usr/src/app/index.js"
        ],
        "process.runtime.version": "22.16.0",
        "process.runtime.name": "nodejs",
        "process.runtime.description": "Node.js",
        "process.command": "/usr/src/app/index.js",
        "process.owner": "node"
      }
    },
    "ScopeSpans": [{
      "Scope": {
        "Name": "@opentelemetry/instrumentation-net",
        "Version": "0.43.1",
        "Attributes": null,
        "DroppedAttributesCount": 0
      },
      "Spans": [{
        "TraceID": "769d28c4b8633dc9de2cc421d1a1616f",
        "SpanID": "2a1f3c4bda1d0e43",
        "TraceState": "",
        "ParentSpanID": "3ea61f2d2a5a003d",
        "Flags": 0,
        "Name": "tcp.connect",
        "Kind": 1,
        "StartTimeUnixNano": 1750044408780000000,
        "EndTimeUnixNano": 1750044408796828584,
        "Attributes": {
          "net.transport": "ip_tcp",
          "net.peer.name": "169.254.169.254",
          "net.peer.port": 80,
          "net.peer.ip": "169.254.169.254",
          "net.host.ip": "172.18.0.14",
          "net.host.port": 50722
        },
        "DroppedAttributesCount": 0,
        "Events": null,
        "DroppedEventsCount": 0,
        "Links": null,
        "DroppedLinksCount": 0,
        "Status": {
          "Message": "",
          "Code": 0
        }
      }],
      "SchemaURL": ""
    }],
    "SchemaURL": ""
  }]
}
```

{{% /collapse %}}

And here's how this trace span looks like in VictoriaTraces:

{{% collapse name="span in VictoriaTraces" %}}

```json
  {
  "_time": "2025-06-16T03:26:48.796828584Z",
  "_stream_id": "00000000000000006fc12c07dedb6f693e73fc7b11b943e6",
  "_stream": "{name=\"tcp.connect\",resource_attr:service.name=\"payment\"}",
  "_msg": "-",
  "dropped_attributes_count": "0",
  "dropped_events_count": "0",
  "dropped_links_count": "0",
  "flags": "0",
  "kind": "1",
  "name": "tcp.connect",
  "resource_attr:container.id": "18ee03279d38ed0e0eedad037c260df78dfc3323aa662ca14a2d38fcc8bf3762",
  "resource_attr:host.arch": "arm64",
  "resource_attr:host.name": "18ee03279d38",
  "resource_attr:os.type": "linux",
  "resource_attr:os.version": "6.10.14-linuxkit",
  "resource_attr:process.command": "/usr/src/app/index.js",
  "resource_attr:process.command_args": "[\"/usr/local/bin/node\",\"--require\",\"./opentelemetry.js\",\"/usr/src/app/index.js\"]",
  "resource_attr:process.executable.name": "node",
  "resource_attr:process.executable.path": "/usr/local/bin/node",
  "resource_attr:process.owner": "node",
  "resource_attr:process.pid": "17",
  "resource_attr:process.runtime.description": "Node.js",
  "resource_attr:process.runtime.name": "nodejs",
  "resource_attr:process.runtime.version": "22.16.0",
  "resource_attr:service.name": "payment",
  "resource_attr:service.namespace": "opentelemetry-demo",
  "resource_attr:service.version": "2.0.2",
  "resource_attr:telemetry.sdk.language": "nodejs",
  "resource_attr:telemetry.sdk.name": "opentelemetry",
  "resource_attr:telemetry.sdk.version": "1.30.1",
  "scope_name": "@opentelemetry/instrumentation-net",
  "scope_version": "0.43.1",
  "span_attr:net.transport": "ip_tcp",
  "duration": "16828584",
  "end_time_unix_nano": "1750044408796828584",
  "parent_span_id": "3ea61f2d2a5a003d",
  "span_attr:net.host.ip": "172.18.0.14",
  "span_attr:net.host.port": "50722",
  "span_attr:net.peer.ip": "169.254.169.254",
  "span_attr:net.peer.name": "169.254.169.254",
  "span_attr:net.peer.port": "80",
  "span_id": "2a1f3c4bda1d0e43",
  "start_time_unix_nano": "1750044408780000000",
  "status_code": "0",
  "trace_id": "769d28c4b8633dc9de2cc421d1a1616f"
}
```

{{% /collapse %}}

### Special mappings

There are some special mappings when transforming a trace span into VictoriaTraces data model:
1. Empty attribute values in trace spans are replaced with `-`.
2. Resource, scope and span attributes are stored with corresponding prefixes `resource_attr`, `scope_attr` and `span_attr:` accordingly. 
3. For some attributes within a list (event list, link list in span), a corresponding prefix and index (such as `event:0:` and `event:0:event_attr:`) is added.
4. The `duration` field does not exist in the OTLP request, but for query efficiency, it's calculated during ingestion and stored as a separated field.

VictoriaTraces automatically indexes all the fields for ingested trace spans.
This enables [full-text search](https://docs.victoriametrics.com/victorialogs/logsql/) across all the fields.

VictoriaTraces stores data in different fields. There are some special fields in addition to [arbitrary fields](#other-fields):

* [`_time` field](#time-field)
* [`_stream` and `_stream_id` fields](#stream-fields)

### Time field

The ingested [trace spans](#data-model) may contain `_time` field with the timestamp of the ingested trace span.

By default, VictoriaTraces use the `EndTimeUnixNano` in trace span as the `_time` field.

The `_time` field is used by [time filter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) for quickly narrowing down the search to the selected time range.

### Stream fields

As service name and span name usually identify the application instance, VictoriaTraces uses `service.name` in resource attributes and `name` in span
as the stream fields.

VictoriaTraces optimizes storing and [querying](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter) of individual trace span streams.
This provides the following benefits:
- Reduced disk space usage, since a trace span stream from a single application instance is usually compressed better
  than a mixed trace span stream from multiple distinct applications.

- Increased query performance, since VictoriaTraces needs to scan lower amounts of data
  when [searching by stream fields](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter).

Every ingested trace span is associated with a trace span stream. Every trace span stream consists of the following special fields:

- `_stream_id` - this is an unique identifier for the trace span stream. All the trace spans for the particular stream can be selected
  via [`_stream_id:...` filter](https://docs.victoriametrics.com/victorialogs/logsql/#_stream_id-filter).

- `_stream` - this field contains stream labels in the format similar to [labels in Prometheus metrics](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels):
  ```
  {resource_attr:service_name="svc name", name="span name"}
  ```
  The `_stream` field can be searched with [stream filters](https://docs.victoriametrics.com/victorialogs/logsql/#stream-filter).

#### High cardinality

Some fields in the [trace spans](#data-model) may contain big number of unique values across log entries.
For example, fields with names such as `trace_id`, `span_id` or `ip` tend to contain big number of unique values.
VictoriaTraces works perfectly with such fields unless they are associated with [trace span streams](#stream-fields).

**Never** associate high-cardinality fields with [trace span streams](#stream-fields), since this may lead to the following issues:

- Performance degradation during [data ingestion](https://docs.victoriametrics.com/victoriatraces/data-ingestion/)
  and [querying](https://docs.victoriametrics.com/victoriatraces/querying/)
- Increased memory usage
- Increased CPU usage
- Increased disk space usage
- Increased disk read / write IO

VictoriaTraces exposes `vl_streams_created_total` [metric](https://docs.victoriametrics.com/victorialogs/#monitoring),
which shows the number of created streams since the last VictoriaTraces restart. If this metric grows at a rapid rate
during long period of time, then there are high chances of high cardinality issues mentioned above.
VictoriaTraces can log all the newly registered streams when `-logNewStreams` command-line flag is passed to it.
This can help narrowing down and eliminating high-cardinality fields from [trace span streams](#stream-fields).

### Other fields

Every ingested log entry may contain arbitrary number of [fields](#data-model) additionally to [`_time`](#time-field).
For example, `name`, `span_attr:ip`, `span_attr:ip`, etc. Such fields can be used for simplifying and optimizing [search queries](https://docs.victoriametrics.com/victorialogs/logsql/).

See [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/) for more details.
