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

# Logstash setup

Specify [`output.opensearch`](https://github.com/opensearch-project/logstash-output-opensearch) section in the `logstash.conf` file
for sending the collected logs to [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/):

```conf
output {
  opensearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.name,process.name"
    }
    manage_template => false
  }
}
```

Substitute `localhost:9428` address inside `hosts` with the real TCP address of VictoriaLogs.

See [these docs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters) for details on the `parameters` section.

It is recommended verifying whether the initial setup generates the needed [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model)
and uses the correct [stream fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields).
This can be done by specifying `debug` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters)
and inspecting VictoriaLogs logs then:

```conf
output {
  opensearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.name,process.name"
        "debug" => "1"
    }
    manage_template => false
  }
}
```

If some [log fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model) must be skipped
during data ingestion, then they can be put into `ignore_fields` [parameter](https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters).
For example, the following config instructs VictoriaLogs to ignore `log.offset` and `event.original` fields in the ingested logs:

```conf
output {
  opensearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
        "ignore_fields" => "log.offset,event.original"
    }
    manage_template => false
  }
}
```

If the Logstash sends logs to VictoriaLogs in another datacenter, then it may be useful enabling data compression via `http_compression: true` option.
This usually allows saving network bandwidth and costs by up to 5 times:

```conf
output {
  opensearch {
    hosts => ["http://localhost:9428/insert/elasticsearch/"]
    parameters => {
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
        "_stream_fields" => "host.hostname,process.name"
    }
    http_compression => true
    manage_template => false
  }
}
```

By default, the ingested logs are stored in the `(AccountID=0, ProjectID=0)` [tenant](https://docs.victoriametrics.com/victorialogs/#multitenancy).
If you need storing logs in other tenant, then specify the needed tenant via `custom_headers` at `output.opensearch` section.
For example, the following `logstash.conf` config instructs Logstash to store the data to `(AccountID=12, ProjectID=34)` tenant:

```conf
output {
  opensearch {
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
    manage_template => false
  }
}
```

See also:

- [Data ingestion troubleshooting](https://docs.victoriametrics.com/victorialogs/data-ingestion/#troubleshooting).
- [How to query VictoriaLogs](https://docs.victoriametrics.com/victorialogs/querying/).
- [Logstash `output.opensearch` docs](https://github.com/opensearch-project/logstash-output-opensearch).
- [Docker-compose demo for Logstash integration with VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/logstash).
