# Docker compose Filebeat integration with VictoriaLogs

The folder contains the example of integration of [filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/filebeat-overview.html) with Victorialogs


To spin-up environment  run the following command:
```
docker compose up -d 
```

To shut down the docker-compose environment run the following command:
```
docker compose down -v
```

The docker compose file contains the following component

* filebeat - fileabeat is configured to collect logs from the docker, you can find configuration in the `filebeat.yml`. It writes data in VictoriaLogs
* filebeat-exporter - it export metrics about the filebeat
* VictoriaLogs - the log database, it accept data from `filebeat` by elastic protocol
* VictoriaMetrics - collect metrics from `filebeat` via `filebeat-exporter`, `VictoriaLogs` and `VictoriaMetrics`(itself)
* grafana - it comes with two predefined dashboards for `VictoriaLogs` and `VictoriaMetrics` 