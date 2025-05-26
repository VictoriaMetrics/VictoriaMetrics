# Docker compose DataDog Agent integration with VictoriaLogs

The folder contains examples of [DataDog agent](https://docs.datadoghq.com/agent) integration with VictoriaLogs using protocols:

* [datadog](./datadog)

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

* datadog - Datadog logs collection agent, which is configured to collect and write data to `victorialogs`
* victorialogs - VictoriaLogs log database, which accepts the data from `datadog`
* victoriametrics - VictoriaMetrics metrics database, collects metrics from `victorialogs` and `datadog`

## Querying

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

> Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
