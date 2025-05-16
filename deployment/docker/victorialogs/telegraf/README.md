# Docker compose Telegraf integration with VictoriaLogs

The folder contains examples of [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/) integration with VictoriaLogs using protocols:

* [elasticsearch](./elasticsearch)
* [loki](./loki)
* [jsonline](./jsonline)
* [syslog](./syslog)

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

* telegraf - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `telegraf` agent
* victoriametrics - metrics database, collects metrics from `victorialogs` and `telegraf` for observability purposes

## Querying

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Telegraf configuration example can be found below:
* [elasticsearch](./elasticsearch/telegraf.conf)
* [loki](./loki/telegraf.conf)
* [jsonline](./jsonline/telegraf.conf)
* [syslog](./syslog/telegraf.conf)

> Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
