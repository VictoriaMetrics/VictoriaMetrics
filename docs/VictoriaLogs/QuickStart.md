---
sort: 1
weight: 1
title: VictoriaLogs Quick Start
menu:
  docs:
    parent: "victorialogs"
    weight: 1
aliases:
- /VictoriaLogs/QuickStart.html
---

# VictoriaLogs Quick Start

It is recommended to read [README](https://docs.victoriametrics.com/VictoriaLogs/)
and [Key Concepts](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html)
before you start working with VictoriaLogs.

## How to install and run VictoriaLogs

There are the following options exist:

- [To run pre-built binaries](#pre-built-binaries)
- [To run Docker image](#docker-image)
- [To run in Kubernetes with Helm charts](#helm-charts)
- [To build VictoriaLogs from source code](#building-from-source-code)

### Pre-built binaries

Pre-built binaries for VictoriaLogs are availble at the [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/) page.
Just download archive for the needed Operating system and architecture, unpack it and run `victoria-logs-prod` from it.

For example, the following commands download VictoriaLogs archive for Linux/amd64, unpack and run it:

```bash
curl -L -O https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v0.4.2-victorialogs/victoria-logs-linux-amd64-v0.4.2-victorialogs.tar.gz
tar xzf victoria-logs-linux-amd64-v0.4.2-victorialogs.tar.gz
./victoria-logs-prod
```

VictoriaLogs is ready for [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
and [querying](https://docs.victoriametrics.com/VictoriaLogs/querying/) at the TCP port `9428` now!
It has no any external dependencies, so it may run in various environments without additional setup and configuration.
VictoriaLogs automatically adapts to the available CPU and RAM resources. It also automatically setups and creates
the needed indexes during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).

See also:

- [How to configure VictoriaLogs](#how-to-configure-victorialogs)
- [How to ingest logs into VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/)


### Docker image

You can run VictoriaLogs in a Docker container. It is the easiest way to start using VictoriaLogs.
Here is the command to run VictoriaLogs in a Docker container:

```bash
docker run --rm -it -p 9428:9428 -v ./victoria-logs-data:/victoria-logs-data \
  docker.io/victoriametrics/victoria-logs:v0.4.2-victorialogs
```

See also:

- [How to configure VictoriaLogs](#how-to-configure-victorialogs)
- [How to ingest logs into VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/)

### Helm charts

You can run VictoriaLogs in Kubernetes environment
with [these Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-logs-single/README.md).

### Building from source code

Follow the following steps in order to build VictoriaLogs from source code:

- Checkout VictoriaLogs source code. It is located in the VictoriaMetrics repository:

  ```bash
  git clone https://github.com/VictoriaMetrics/VictoriaMetrics
  cd VictoriaMetrics
  ```

- Build VictoriaLogs. The build command requires [Go 1.20](https://golang.org/doc/install).

  ```bash
  make victoria-logs
  ```

- Run the built binary:

  ```bash
  bin/victoria-logs
  ```

VictoriaLogs is ready for [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
and [querying](https://docs.victoriametrics.com/VictoriaLogs/querying/) at the TCP port `9428` now!
It has no any external dependencies, so it may run in various environments without additional setup and configuration.
VictoriaLogs automatically adapts to the available CPU and RAM resources. It also automatically setups and creates
the needed indexes during [data ingestion](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/).

See also:

- [How to configure VictoriaLogs](#how-to-configure-victorialogs)
- [How to ingest logs into VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/)


## How to configure VictoriaLogs

VictoriaLogs is configured via command-line flags. All the command-line flags have sane defaults,
so there is no need in tuning them in general case. VictoriaLogs runs smoothly in most environments
without additional configuration.

Pass `-help` to VictoriaLogs in order to see the list of supported command-line flags with their description and default values:

```bash
/path/to/victoria-logs -help
```

VictoriaLogs stores the ingested data to the `victoria-logs-data` directory by default. The directory can be changed
via `-storageDataPath` command-line flag. See [these docs](https://docs.victoriametrics.com/VictoriaLogs/#storage) for details.

By default VictoriaLogs stores [log entries](https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html) with timestamps
in the time range `[now-7d, now]`, while dropping logs outside the given time range.
E.g. it uses the retention of 7 days. Read [these docs](https://docs.victoriametrics.com/VictoriaLogs/#retention) on how to control the retention
for the [ingested](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/) logs.

It is recommended setting up monitoring of VictoriaLogs according to [these docs](https://docs.victoriametrics.com/VictoriaLogs/#monitoring).

See also:

- [How to ingest logs into VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/)
- [How to query VictoriaLogs](https://docs.victoriametrics.com/VictoriaLogs/querying/)

## Docker demos

Here are a Docker-compose demos, which start VictoriaLogs and push logs to it via various log collectors:

- [Filebeat demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/filebeat-docker)
- [Fluentbit demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/fluentbit-docker)
- [Logstash demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/logstash)
- [Vector demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/vector-docker)
- [Promtail demo](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/victorialogs/promtail)

You can use [this Helm chart](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-logs-single/README.md)
as a demo for running Fluentbit in Kubernetes with VictoriaLogs.
