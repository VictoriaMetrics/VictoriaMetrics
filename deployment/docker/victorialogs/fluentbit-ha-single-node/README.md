# Docker compose Fluentbit integration with VictoriaLogs. High-Availability example 

The folder contains the example of integration of [fluentbit](https://docs.fluentbit.io/manual) with VictoriaLogs Single-Nodes(s) and [vmauth](https://docs.victoriametrics.com/vmauth/) for achieving High Availability. 

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

* fluentbit - fluentbit is configured to collect logs from the `docker`, you can find configuration in the `fluent-bit.conf`. It writes data in VictoriaLogs
* VictoriaLogs - the two instances of log database, they accept the data from `fluentbit` by json line protocol
* vmauth - load balancer for proxying requests to one of VictoriaLogs

Querying the data

* [vmui](https://docs.victoriametrics.com/victorialogs/querying/#vmui) - a web UI is accessible by `http://localhost:8427/select/vmui/`
* for querying the data via command-line please check [these docs](https://docs.victoriametrics.com/victorialogs/querying/#command-line)


the example of fluentbit configuration(`filebeat.yml`)

```text
[INPUT]
    name             tail
    path             /var/lib/docker/containers/**/*.log
    path_key         path
    multiline.parser docker, cri
    Parser           docker
    Docker_Mode      On

[INPUT]
    Name     syslog
    Listen   0.0.0.0
    Port     5140
    Parser   syslog-rfc3164
    Mode     tcp

[SERVICE]
    Flush        1
    Parsers_File parsers.conf

[OUTPUT]
    Name http
    Match *
    host victorialogs-2
    port 9428
    compress gzip
    uri /insert/jsonline?_stream_fields=stream,path&_msg_field=log&_time_field=date
    format json_lines
    json_date_format iso8601
    header AccountID 0
    header ProjectID 0

[OUTPUT]
    Name http
    Match *
    host victorialogs-1
    port 9428
    compress gzip
    uri /insert/jsonline?_stream_fields=stream,path&_msg_field=log&_time_field=date
    format json_lines
    json_date_format iso8601
    header AccountID 0
    header ProjectID 0
```

Please, note that `_stream_fields` parameter must follow recommended [best practices](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to achieve better performance.

The example of vmauth configuration (`auth.yml`)

```yaml
unauthorized_user:
  url_prefix:
    - http://victorialogs-1:9428
    - http://victorialogs-2:9428
```