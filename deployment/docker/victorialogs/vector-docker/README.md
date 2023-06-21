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
* VictoriaMetrics - collects metrics from `VictoriaLogs` and `VictoriaMetrics`(itself)

the example of vector configuration(`vector.toml`)

```
[api]
  enabled = true
  address = "0.0.0.0:8686"

  [sources.docker]
  type = "docker_logs"

  [sinks.vlogs]
  type = "elasticsearch"
  inputs = [ "docker" ]
  endpoints = [ "http://victorialogs:9428/insert/elasticsearch/" ]
  id_key = "id"
  mode = "bulk"
  healthcheck.enabled = false

  [sinks.vlogs.query]
  _msg_field = "message"
  _time_field = "timestamp"
  _stream_fields = "host,container_name"

  [sources.vector_metrics]
  type = "internal_metrics"

  [sinks.victoriametrics]
  type = "prometheus_remote_write"
  endpoint = "http://victoriametrics:8428/api/v1/write"
  inputs = ["vector_metrics"]
  healthcheck.enabled = false
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields) to achieve better performance.