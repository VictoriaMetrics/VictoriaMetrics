# Docker compose Vector integration with VictoriaLogs using given below protocols:

* [elasticsearch](./elasticsearch)
* [loki](./loki)
* [jsonline single node](./jsonline)
* [jsonline HA setup](./jsonline-ha)

The folder contains the example of integration of [vector](https://vector.dev/docs/) with Victorialogs

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

* vector - vector is configured to collect logs from the `docker`, you can find configuration in the `vector.yaml`. It writes data in VictoriaLogs. It pushes metrics to VictoriaMetrics.
* VictoriaLogs - the log database, it accepts the data from `vector` by DataDog protocol
* VictoriaMetrics - collects metrics from `VictoriaLogs` and `VictoriaMetrics`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Vector configuration example can be found below:
* [elasticsearch](./elasticsearch/vector.yaml)
* [loki](./loki/vector.yaml)
* [jsonline single node](./jsonline/vector.yaml)
* [jsonline HA setup](./jsonline-ha/vector.yaml)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
