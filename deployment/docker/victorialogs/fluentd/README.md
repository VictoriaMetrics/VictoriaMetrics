# Docker compose Fluentd integration with VictoriaLogs

The folder contains examples of [Fluentd](https://www.fluentd.org/) integration with VictoriaLogs using protocols:

* [loki](./loki)
* [jsonline](./jsonline)
* [datadog](./datadog)
* [elasticsearch](./elasticsearch)

All required plugins, that should be installed in order to support protocols listed above can be found in a [Dockerfile](./Dockerfile)

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

* fluentd - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `fluentd` agent
* victoriametrics - metrics database, collects metrics from `victorialogs` and `fluentd` for observability purposes

## Querying

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Fluentd configuration example can be found below:
* [loki](./loki/fluent.conf)
* [jsonline](./jsonline/fluent.conf)
* [elasticsearch](./elasticsearch/fluent.conf)

> Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
