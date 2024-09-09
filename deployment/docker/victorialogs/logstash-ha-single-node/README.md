# Docker compose Logstash integration with VictoriaLogs. High-Availability example

The folder contains the example of integration of [logstash](https://www.elastic.co/logstash) with VictoriaLogs Single-Node(s) and [vmauth](https://docs.victoriametrics.com/vmauth/) for achieving High Availability.

Check [this documentation](https://docs.victoriametrics.com/victorialogs/#high-availability) with a description of the architecture and components.

To spin-up environment  run the following command:

```shell
docker compose up -d 
```

To shut down the docker-compose environment run the following command:

```shell
docker compose down
docker compose rm -f
```

The docker compose file contains the following components:

* logstash - logstash is configured to read docker log files, you can find configuration in the `pipeline.conf`. It writes data in two instances of VictoriaLogs
* VictoriaLogs - the two instances of log database, they accept the data from `fluentbit` by json line protocol
* vmauth - load balancer for proxying requests to one of VictoriaLogs

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:8427/select/vmui/`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)


Here is an example of logstash configuration(`pipeline.conf`):

```text
input {
  file {
    path => "/var/lib/docker/containers/*/*.log"
    start_position => "beginning"
    type => "docker"
    sincedb_path => "/dev/null"
    codec => "json"
    add_field => {
      "path" => "%{[@metadata][path]}"
    }
  }
}

output {
  http {
    url => "http://victorialogs-1:9428/insert/jsonline?_stream_fields=host.name,stream&_msg_field=log&_time_field=time"
    format => "json"
    http_method => "post"
  }
  http {
    url => "http://victorialogs-2:9428/insert/jsonline?_stream_fields=host.name,stream&_msg_field=log&_time_field=time"
    format => "json"
    http_method => "post"
  }
}
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.

The example of vmauth configuration (`auth.yml`)

```yaml
unauthorized_user:
  url_prefix:
    - http://victorialogs-1:9428
    - http://victorialogs-2:9428
```