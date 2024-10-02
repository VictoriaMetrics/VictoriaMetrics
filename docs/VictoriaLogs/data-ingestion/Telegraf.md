---
weight: 5
title: Telegraf setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 5
aliases:
  - /VictoriaLogs/data-ingestion/Telegraf.html
---
VictoriaLogs supports given below Telegraf outputs:
- [Elasticsearch](#elasticsearch)
- [Loki](#loki)
- [HTTP JSON](#http)

## Elasticsearch

Specify [Elasticsearch output](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/elasticsearch) in the `telegraf.toml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```toml
[[outputs.elasticsearch]]
  urls = ["http://localhost:9428/insert/elasticsearch"]
  timeout = "1m"
  flush_interval = "30s"
  enable_sniffer = false
  health_check_interval = "0s"
  index_name = "device_log-%Y.%m.%d"
  manage_template = false
  template_name = "telegraf"
  overwrite_template = false
  namepass = ["tail"]
  [outputs.elasticsearch.headers]
    "VL-Msg-Field" = "tail.value"
    "VL-Time-Field" = "@timestamp"
    "VL-Stream-Fields" = "tag.log_source,tag.metric_type"

[[inputs.tail]]
  files = ["/tmp/telegraf.log"]
  from_beginning = false
  interval = "10s"
  pipe = false
  watch_method = "inotify"
  data_format = "value"
  data_type = "string"
  character_encoding = "utf-8"
  [inputs.tail.tags]
     metric_type = "logs"
     log_source = "telegraf"
```


## Loki

Specify [Loki output](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/loki) in the `telegraf.toml`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```toml
[[outputs.loki]]
  domain = "http://localhost:9428"
  endpoint = "/insert/loki/api/v1/push&_msg_field=tail.value&_time_field=@timefield&_stream_fields=log_source,metric_type"
  namepass = ["tail"]
  gzip_request = true
  sanitize_label_names = true

[[inputs.tail]]
  files = ["/tmp/telegraf.log"]
  from_beginning = false
  interval = "10s"
  pipe = false
  watch_method = "inotify"
  data_format = "value"
  data_type = "string"
  character_encoding = "utf-8"
  [inputs.tail.tags]
     metric_type = "logs"
     log_source = "telegraf"
```


## HTTP

Specify [HTTP output](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/http) in the `telegraf.toml with batch mode disabled`
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```toml
[[inputs.tail]]
  files = ["/tmp/telegraf.log"]
  from_beginning = false
  interval = "10s"
  pipe = false
  watch_method = "inotify"
  data_format = "value"
  data_type = "string"
  character_encoding = "utf-8"
  [inputs.tail.tags]
     metric_type = "logs"
     log_source = "telegraf"

[[outputs.http]]
  url = "http://localhost:9428/insert/jsonline?_msg_field=fields.message&_time_field=timestamp,_stream_fields=tags.log_source,tags.metric_type"
  data_format = "json"
  namepass = ["docker_log"]
  use_batch_format = false
```

Substitute the `localhost:9428` address inside `endpoints` section with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-headers) for details on headers specified
in the `[[output.elasticsearch]]` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Elasticsearch output docs for Telegraf](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/elasticsearch).
- [Docker-compose demo for Telegraf integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/telegraf).
