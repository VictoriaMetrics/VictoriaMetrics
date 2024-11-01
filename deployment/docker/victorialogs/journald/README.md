# Docker compose Journald integration with VictoriaLogs

The folder contains examples of Journald integration with VictoriaLogs using protocols:

* [journald](./journald)

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

* journald - Journald logs collection agent, which is configured to collect and write data to `victorialogs`
* victorialogs - VictoriaLogs log database, which accepts the data from `journald`
* victoriametrics - VictoriaMetrics metrics database, which collects metrics from `victorialogs` and `journald`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
