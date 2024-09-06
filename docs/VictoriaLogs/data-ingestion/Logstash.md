---
weight: 3
title: Logstash setup
disableToc: true
menu:
  docs:
    parent: "victorialogs-data-ingestion"
    weight: 3
aliases:
  - /VictoriaLogs/data-ingestion/Logstash.html
  - /victorialogs/data-ingestion/logstash.html
  - /victorialogs/data-ingestion/Logstash.html
---
VictoriaLogs supports given below Logstash outputs:
- [Elasticsearch](#elasticsearch)
- [Loki](#loki)
- [HTTP JSON](#http)

## Elasticsearch

Specify [`output.elasticsearch`](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html) section in the `logstash.conf` file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```logstash
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.name,process.name"
    }
  }
}
```

Substitute `localhost:9428` address inside `hosts` with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on the `parameters` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters)
and inspecting VictoriaLogs logs then:

```logstash
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.name,process.name"
        "debug" => "1"
    }
  }
}
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```logstash
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
        "ignore_fields" => "log.offset,event.original"
    }
  }
}
```

If the Logstash sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `http_compression: true` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```logstash
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
    }
    http_compression => true
  }
}
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `custom_headers` at `output.elasticsearch` section.
For example, the following `logstash.conf` config instructs Logstash to store the data to `(AccountID=12, ProjectID=34)` tenant:

```logstash
output {
  elasticsearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    custom_headers => {
        "AccountID" => "1"
        "ProjectID" => "2"
    }
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
    }
  }
}
```

## Loki

Specify [`output.loki`](https://grafana.com/docs/loki/latest/send-data/logstash/) section in the `logstash.conf` file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```conf
output {
  loki {
     url => "http://victorialogs:9428/insert/loki/api/v1/push?_stream_fields=host.ip,process.name&_msg_field=message&_time_field=@timestamp"
  }
}
```

## HTTP

Specify [`output.http`](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-http.html) section in the `logstash.conf` file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```conf
output {
  url => "http://victorialogs:9428/insert/jsonline?_stream_fields=host.ip,process.name&_msg_field=message&_time_field=@timestamp"
  format => "json"
  http_method => "post"
}
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Logstash `output.elasticsearch` docs](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html).
- [Docker-compose demo for Logstash integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/logstash).
