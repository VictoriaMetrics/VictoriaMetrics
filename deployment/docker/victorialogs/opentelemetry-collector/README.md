# Docker compose OpenTelemetry integration with VictoriaLogs using protocols:

* [loki](./loki)
* [otlp](./otlp)
* [syslog](./syslog)
* [elasticsearch single node](./elasticsearch)
* [elasticsearch HA mode](./elasticsearch-ha/)

The folder contains the example of integration of [OpenTelemetry collector](https://opentelemetry.io/docs/collector/) with Victorialogs

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

* collector - vector is configured to collect logs from the `docker`, you can find configuration in the `config.yaml`. It writes data in VictoriaLogs. It pushes metrics to VictoriaMetrics.
* VictoriaLogs - the log database, it accepts the data from `collector` by elastic protocol
* VictoriaMetrics - collects metrics from `VictoriaLogs` and `VictoriaMetrics`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

OpenTelemetry collector configuration example can be found below:
* [loki](./loki/config.yaml)
* [otlp](./otlp/config.yaml)
* [syslog](./syslog/config.yaml)
* [elasticsearch single node](./elasticsearch/config.yaml)
* [elasticsearch HA mode](./elasticsearch-ha/config.yaml)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
