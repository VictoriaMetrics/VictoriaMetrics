# Docker compose FluentBit integration with VictoriaLogs

The folder contains examples of [FluentBit](https://docs.fluentbit.io/manual) integration with VictoriaLogs using protocols:

* [datadog](./datadog)
* [loki](./loki)
* [jsonline single node](./jsonline)
* [jsonline HA setup](./jsonline-ha)
* [otlp](./otlp)

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

* fluentbit - logs collection agent configured to collect and write data to `victorialogs`
* victorialogs - logs database, receives data from `fluentbit` agent
* victoriametrics - metrics database, collects metrics from `victorialogs` and `fluentbit` for observability purposes

## Querying

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [vlogscli](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

FluentBit configuration example can be found below:
* [datadog](./datadog/fluent-bit.conf)
* [loki](./loki/fluent-bit.conf)
* [jsonline single node](./jsonline/fluent-bit.conf)
* [jsonline HA setup](./jsonline-ha/fluent-bit.conf)
* [otlp](./otlp/fluent-bit.conf)

> Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
