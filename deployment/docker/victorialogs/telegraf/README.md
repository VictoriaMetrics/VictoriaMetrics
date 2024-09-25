# Docker compose Telegraf integration with VictoriaLogs for docker

The folder contains the examples of integration of [telegraf](https://www.influxdata.com/time-series-platform/telegraf/) with VictoriaLogs using:

* [elasticsearch](./elasticsearch)
* [loki](./loki)
* [jsonline](./jsonline)
* [syslog](./syslog)

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

* telegraf - telegraf is configured to collect logs from the `docker`, you can find configuration in the `telegraf.conf`. It writes data in VictoriaLogs. It pushes metrics to VictoriaMetrics.
* VictoriaLogs - the log database, it accepts the data from `telegraf` by elastic protocol
* VictoriaMetrics - collects metrics from `VictoriaLogs` and `VictoriaMetrics`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Telegraf configuration example can be found below:
* [elasticsearch](./elasticsearch/telegraf.conf)
* [loki](./loki/telegraf.conf)
* [jsonline](./jsonline/telegraf.conf)
* [syslog](./syslog/telegraf.conf)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
