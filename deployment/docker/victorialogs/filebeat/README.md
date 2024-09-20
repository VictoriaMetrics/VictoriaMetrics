# Docker compose Filebeat integration with VictoriaLogs using listed below protocols:

* [syslog](./syslog)
* [elasticsearch](./elasticsearch)

The folder contains the example of integration of [filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/filebeat-overview.html) with Victorialogs

To spin-up environment `cd` to any of listed above directories run the following command:
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
* VictoriaLogs - the log database, it accepts the data from `filebeat` by elastic protocol
* VictoriaMetrics - collects metrics from `filebeat` via `filebeat-exporter`, `VictoriaLogs` and `VictoriaMetrics`

Querying the data 

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

the example of filebeat configuration

https://github.com/VictoriaMetrics/VictoriaMetrics/blob/da6889f89bd298683cd25b71a3f851930c8fe39f/deployment/docker/victorialogs/filebeat/syslog/filebeat.yml#L1-L14

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
