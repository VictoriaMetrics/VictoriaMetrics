# Docker compose Serverless with DataDog extension integration with VictoriaLogs

The folder contains examples of [DataDog serverless](https://docs.datadoghq.com/serverless) integration with VictoriaLogs for:

* [AWS Lambda](./aws)
* [GCP Cloud Run](./gcp)

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

* dd-proxy - VMAuth proxy, with path-based routing to `victoriametrics` and `victorialogs`
* lambda - Serverless application with Datadog logs collection extension, which is configured to collect and write data to `victorialogs` and `victoriametrics` via `dd-proxy`
* victorialogs - VictoriaLogs log database, which accepts the data from `datadog`
* victoriametrics - VictoriaMetrics metrics database, which collects metrics from `victorialogs` and `datadog`

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:9428/select/vmui`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
