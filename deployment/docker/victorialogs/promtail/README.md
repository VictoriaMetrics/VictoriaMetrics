# Docker compose Promtail integration with VictoriaLogs

The folder contains the example of integration of [Promtail agent](https://grafana.com/docs/loki/latest/send-data/promtail/) with VictoriaLogs using protocols:

* [loki](./loki)

## Quick start

To spin-up environment `cd` to any of listed above directories run the following command:
```sh
docker compose up -d 
```

To shut down the docker-compose environment run the following command:
```sh
docker compose down -v
```

The docker compose file contains the following components:

* promtail - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `promtail` agent
* victoriametrics - metrics database, which collects metrics from `victorialogs` and `promtail` for observability purposes

## Querying

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Promtail agent configuration example can be found below:
* [loki](./loki/config.yml)

> Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
