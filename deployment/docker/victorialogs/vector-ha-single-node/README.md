# Docker compose Vector integration with VictoriaLogs for docker. High-Availability example

The folder contains the example of integration of [vector](https://vector.dev/docs/) with VictoriaLogs Single-Node(s) and [vmauth](https://docs.victoriametrics.com/vmauth/) for achieving High Availability.

Check [this documentation](https://docs.victoriametrics.com/victorialogs/#high-availability) with a description of the architecture and components.


To spin-up environment  run the following command:

```shell
docker compose up -d 
```

To shut down the docker-compose environment run the following command:

```shell
docker compose down
docker compose rm -f
```

The docker compose file contains the following components:

* vector - vector is configured to collect logs from the `docker`, you can find configuration in the `vector.yaml`. It writes data in two instances of VictoriaLogs
* VictoriaLogs - the two instances of log database, they accept the data from `vector` by json line protocol
* vmauth - load balancer for proxying requests to one of VictoriaLogs

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:8427/select/vmui/`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)


the example of vector configuration(`vector.yaml`)

```yaml
api:
  enabled: true
  address: 0.0.0.0:8686
sources:
  docker:
    type: docker_logs
transforms:
  msg_parser:
    type: remap
    inputs:
      - docker
    source: |
      if exists(.message) {
        .log, err = parse_json(.message)
        if err == null {
          del(.message)
        }
      }
sinks:
  console_out:
    type: console
    inputs:
      - msg_parser
    encoding:
      codec: json
  vlogs_http_1:
    type: http
    inputs:
      - msg_parser
    uri: http://victorialogs-1:9428/insert/jsonline?_stream_fields=source_type,host,container_name&_msg_field=log.msg&_time_field=timestamp
    encoding:
      codec: json
    framing:
      method: newline_delimited
    compression: gzip
    healthcheck:
      enabled: false
    request:
      headers:
        AccountID: '0'
        ProjectID: '0'
  vlogs_http_2:
    type: http
    inputs:
      - msg_parser
    uri: http://victorialogs-2:9428/insert/jsonline?_stream_fields=source_type,host,container_name&_msg_field=log.msg&_time_field=timestamp
    encoding:
      codec: json
    framing:
      method: newline_delimited
    compression: gzip
    healthcheck:
      enabled: false
    request:
      headers:
        AccountID: '0'
        ProjectID: '0'
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.

The example of vmauth configuration (`auth.yml`)

```yaml
unauthorized_user:
  url_prefix:
    - http://victorialogs-1:9428
    - http://victorialogs-2:9428
```