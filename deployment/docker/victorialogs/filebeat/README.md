# Docker compose Filebeat integration with VictoriaLogs

The folder contains examples of [Filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/filebeat-overview.html) integration with VictoriaLogs using protocols:

* [syslog](./syslog)
* [elasticsearch](./elasticsearch)

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

* filebeat - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `filebeat` agent
* victoriametrics - metrics database, which collects metrics from `victorialogs` and `filebeat` for observability purposes

Querying the data 

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Filebeat configuration example can be found below:
- [syslog](./syslog/filebeat.yml)
- [elasticsearch](./elasticsearch/filebeat.yml)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
