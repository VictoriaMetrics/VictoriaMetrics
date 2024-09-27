# Docker compose Vector integration with VictoriaLogs

The folder contains examples of [Vector](https://vector.dev/docs/) integration with VictoriaLogs using protocols:

* [elasticsearch](./elasticsearch)
* [loki](./loki)
* [jsonline single node](./jsonline)
* [jsonline HA setup](./jsonline-ha)

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

* vector - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `vector` agent
* victoriametrics - metrics database, which collects metrics from `victorialogs` and `vector` for observability purposes

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Vector configuration example can be found below:
* [elasticsearch](./elasticsearch/vector.yaml)
* [loki](./loki/vector.yaml)
* [jsonline single node](./jsonline/vector.yaml)
* [jsonline HA setup](./jsonline-ha/vector.yaml)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
