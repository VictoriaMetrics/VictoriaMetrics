# Docker compose Fluentbit integration with VictoriaLogs using given below protocols:

* [loki](./loki)
* [jsonline single node](./jsonline)
* [jsonline HA setup](./jsonline-ha)

The folder contains the example of integration of [fluentbit](https://docs.fluentbit.io/manual) with Victorialogs

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

* fluentbit - fluentbit is configured to collect logs from the `docker`, you can find configuration in the `fluent-bit.conf`. It writes data in VictoriaLogs
* VictoriaLogs - the log database, it accepts the data from `fluentbit` by json line protocol

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

FluentBit configuration example can be found below:
* [loki](./loki/fluent-bit.conf)
* [jsonline single node](./jsonline/fluent-bit.conf)
* [jsonline HA setup](./jsonline-ha/fluent-bit.conf)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
