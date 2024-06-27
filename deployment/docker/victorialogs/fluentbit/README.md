# Docker compose Fluentbit integration with VictoriaLogs for docker

The folder contains the example of integration of [fluentbit](https://docs.fluentbit.io/manual) with Victorialogs

To spin-up environment  run the following command:
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


the example of fluentbit configuration(`filebeat.yml`)

```shell
[INPUT]
    name              tail
    path              /var/lib/docker/containers/**/*.log
    multiline.parser  docker, cri
    Parser docker
    Docker_Mode  On

[INPUT]
    Name     syslog
    Listen   0.0.0.0
    Port     5140
    Parser   syslog-rfc3164
    Mode     tcp

[SERVICE]
    Flush        1
    Parsers_File parsers.conf

[Output]
    Name http
    Match *
    host victorialogs
    port 9428
    compress gzip
    uri /insert/jsonline?_stream_fields=stream&_msg_field=log&_time_field=date
    format json_lines
    json_date_format iso8601
    header AccountID 0
    header ProjectID 0
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.
