# Docker compose Fluentd integration with VictoriaLogs using given below protocols:

* [loki](./loki)
* [jsonline](./jsonline)
* [elasticsearch](./elasticsearch)

The folder contains the example of integration of [fluentd](https://www.fluentd.org/) with Victorialogs

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

* fluentd - fluentd is configured to collect logs from the `docker`, you can find configuration in the `fluent-bit.conf`. It writes data in VictoriaLogs
* VictoriaLogs - the log database, it accepts the data from `fluentd` by json line protocol

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Fluentd configuration example can be found below:
* [loki](./loki/fluent.conf)
* [jsonline](./jsonline/fluent.conf)
* [elasticsearch](./elasticsearch/fluent.conf)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
