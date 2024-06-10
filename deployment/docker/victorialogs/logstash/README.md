# Docker compose Logstash integration with VictoriaLogs for syslog

It is required to use [OpenSearch plugin](https://github.com/opensearch-project/logstash-output-opensearch) for output configuration.
Plugin can be installed by using the following command:
```
bin/logstash-plugin install logstash-output-opensearch
```
OpenSearch plugin is required because elasticsearch output plugin performs various checks for Elasticsearch version and license which are not applicable for VictoriaLogs.

To spin-up environment  run the following command:
```
docker compose up -d 
```

To shut down the docker-compose environment run the following command:
```
docker compose down
docker compose rm -f
```

The docker compose file contains the following components:

* logstash - logstash is configured to accept `syslog` on `5140` port, you can find configuration in the `pipeline.conf`. It writes data in VictoriaLogs
* VictoriaLogs - the log database, it accepts the data from `logstash` by elastic protocol

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)


Here is an example of logstash configuration(`pipeline.conf`):

```
input {
  syslog {
    port => 5140
  }
}
output {
  opensearch {
    hosts => ["http://victorialogs:9428/insert/elasticsearch"]
    custom_headers => {
        "AccountID" => "0"
        "ProjectID" => "0"
    }
    parameters => {
        "_stream_fields" => "host.ip,process.name"
        "_msg_field" => "message"
        "_time_field" => "@timestamp"
    }
  }
}
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
