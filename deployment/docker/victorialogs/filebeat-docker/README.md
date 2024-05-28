# Docker compose Filebeat integration with VictoriaLogs for docker

The folder contains the example of integration of [filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/filebeat-overview.html) with Victorialogs

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

* filebeat - fileabeat is configured to collect logs from the `docker`, you can find configuration in the `filebeat.yml`. It writes data in VictoriaLogs
* filebeat-exporter - it export metrics about the filebeat
* VictoriaLogs - the log database, it accepts the data from `filebeat` by elastic protocol
* VictoriaMetrics - collects metrics from `filebeat` via `filebeat-exporter`, `VictoriaLogs` and `VictoriaMetrics`
* grafana - it comes with two predefined dashboards for `VictoriaLogs` and `VictoriaMetrics`

Querying the data 

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

the example of filebeat configuration(`filebeat.yml`)

```yaml
filebeat.autodiscover:
  providers:
    - type: docker
      hints.enabled: true

processors:
  - add_docker_metadata: ~

output.elasticsearch:
  hosts: [ "http://victorialogs:9428/insert/elasticsearch/" ]
  worker: 5
  parameters:
    _msg_field: "message"
    _time_field: "@timestamp"
    _stream_fields: "container.name"

http:
  enabled: true
  host: 0.0.0.0
  port: 5066
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
