# How to set up sending logs to VictoriaLogs from logstash

It is required to use [OpenSearch plugin](https://github.com/opensearch-project/logstash-output-opensearch) for output configuration.
Plugin can be installed by using the following command:
```
bin/logstash-plugin install logstash-output-opensearch
```
OpenSearch plugin is required because elasticsearch output plugin performs various checks for Elasticsearch version and license which are not applicable for VictoriaLogs.

Here is an example of logstash configuration:

```
  opensearch {
    hosts => ["http://vlogs:9428/insert/elasticsearch"]
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
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) to achieve better performance.
