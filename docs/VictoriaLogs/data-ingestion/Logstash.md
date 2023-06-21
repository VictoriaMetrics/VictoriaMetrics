# Logstash setup

[Logstash](https://www.elastic.co/guide/en/logstash/8.8/introduction.html) log collector supports
[Elasticsearch output](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html) compatible with
[Elasticsearch bulk API](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#elasticsearch-bulk-api)
in VictoriaMetrics.

Specify [`output.elasticsearch`](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html) section in the `logstash.conf` file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/):

```conf
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

See [these docs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters) for details on the `parameters` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters)
and inspecting VictoriaLogs logs then:

```conf
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

If some [log fields](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```conf
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

```conf
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

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/VictoriaLogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `custom_headers` at `output.elasticsearch` section.
For example, the following `logstash.conf` config instructs Logstash to store the data to `(AccountID=12, ProjectID=34)` tenant:

```conf
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

More info about output tuning you can find in [these docs](https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html).

[Here is a demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/logstash)
for running Logstash with VictoriaLogs with docker-compose and collecting logs to VictoriaLogs
(via [Elasticsearch bulk API](https://docs.victoriametrics.com/VictoriaLogs/daat-ingestion/#elasticsearch-bulk-api)).

The ingested log entries can be queried according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/querying/).

See also [data ingestion troubleshooting](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#troubleshooting) docs.
