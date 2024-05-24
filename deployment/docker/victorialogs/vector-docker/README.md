# Docker compose Vector integration with VictoriaLogs for docker

The folder contains the example of integration of [vector](https://vector.dev/docs/) with Victorialogs

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

* vector - vector is configured to collect logs from the `docker`, you can find configuration in the `vector.toml`. It writes data in VictoriaLogs. It pushes metrics to VictoriaMetrics.
* VictoriaLogs - the log database, it accepts the data from `vector` by elastic protocol
* VictoriaMetrics - collects metrics from `VictoriaLogs` and `VictoriaMetrics`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)


the example of vector configuration(`vector.toml`)

```
[sources.docker]
  type = "docker_logs"

[transforms.msg_parser]
  type = "remap"
  inputs = ["docker"]
  source = '''
  .log = parse_json!(.message)
  del(.message)
  '''

[sinks.vlogs]
  type = "elasticsearch"
  inputs = [ "msg_parser" ]
  endpoints = [ "http://victorialogs:9428/insert/elasticsearch/" ]
  mode = "bulk"
  api_version = "v8"
  compression = "gzip"
  healthcheck.enabled = false

  [sinks.vlogs.query]
    _msg_field = "log.msg"
    _time_field = "timestamp"
    _stream_fields = "source_type,host,container_name"

  [sinks.vlogs.request.headers]
    AccountID = "0"
    ProjectID = "0"
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
