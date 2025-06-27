> VictoriaTraces is currently under active development and not ready for production use. It is built on top of VictoriaLogs and therefore shares some flags and APIs. These will be fully separated once VictoriaTraces reaches a stable release. Until then, features may change or break without notice.

VictoriaTraces is an open-source, user-friendly database designed for storing and querying distributed [tracing data](https://en.wikipedia.org/wiki/Tracing_(software)), 
built by the [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) team.

VictoriaTraces provides the following features:
- It is resource-efficient and fast. It uses up to [**3.7x less RAM and up to 2.6x less CPU**](https://victoriametrics.com/blog/dev-note-distributed-tracing-with-victorialogs/) than other solutions such as Grafana Tempo.
- VictoriaTraces' capacity and performance scales linearly with the available resources (CPU, RAM, disk IO, disk space).
- It accepts trace spans in the popular [OpenTelemetry protocol](https://opentelemetry.io/docs/specs/otel/protocol/)(OTLP).
- It provides [Jaeger Query Service JSON APIs](https://www.jaegertracing.io/docs/2.6/apis/#internal-http-json) 
  to integrate with [Grafana](https://grafana.com/docs/grafana/latest/datasources/jaeger/) or [Jaeger Frontend](https://www.jaegertracing.io/docs/2.6/frontend-ui/).

## Quick Start

The easiest way to get started with VictoriaTraces is by using the pre-built Docker Compose file.
It launches VictoriaTraces, Grafana, and HotROD (a sample application that generates tracing data).
Everything is preconfigured and connected out of the box, so you can start exploring distributed tracing within minutes.

Clone the repository:
```bash 
git clone -b victoriatraces --single-branch https://github.com/VictoriaMetrics/VictoriaMetrics.git;
cd  VictoriaMetrics;
```

Run VictoriaTraces with Docker Compose:
```bash
make docker-vt-single-up;
```

Now you can open HotROD at [http://localhost:8080](http://localhost:8080) and click around to generate some traces.
Then, you can open Grafana at [http://localhost:3000/explore](http://localhost:3000/explore) and explore the traces using the Jaeger data source.

To stop the services, run:
```bash
make docker-vt-single-down;
```

You can read more about docker compose and what's available there in the [Docker compose environment for VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/deployment/victoriatraces/deployment/docker/README.md#victoriatraces-server).

### How to build from sources

Building from sources is reasonable when developing additional features specific to your needs or when testing bugfixes.

{{% collapse name="How to build from sources" %}}

Clone VictoriaMetrics repository: 
```bash 
git clone -b victoriatraces --single-branch https://github.com/VictoriaMetrics/VictoriaMetrics.git;
cd  VictoriaMetrics;
```

#### Build binary with go build

1. [Install Go](https://golang.org/doc/install).
1. Run `make victoria-traces` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaTraces).
   It builds `victoria-traces` binary and puts it into the `bin` folder.

#### Build binary with Docker

1. [Install docker](https://docs.docker.com/install/).
1. Run `make victoria-traces-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaTraces).
   It builds `victoria-traces-prod` binary and puts it into the `bin` folder.

#### Building docker images

Run `make package-victoria-traces`. It builds `victoriametrics/victoria-traces:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-victoria-traces`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable.
For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```sh
ROOT_IMAGE=scratch make package-victoria-traces
```

{{% /collapse %}}

### Configure and run VictoriaTraces

VictoriaTraces can be run with:
```shell
/path/to/victoria-traces -storageDataPath=victoria-traces-data -retentionPeriod=7d
```

or with Docker:
```shell
docker run --rm -it -p 9428:9428 -v ./victoria-traces-data:/victoria-traces-data \
  docker.io/victoriametrics/victoria-traces:latest -storageDataPath=victoria-traces-data
```

VictoriaTraces is configured via command-line flags. 
All the command-line flags have sane defaults, so there is no need in tuning them in general case. 
VictoriaTraces runs smoothly in most environments without additional configuration.

Pass `-help` to VictoriaTraces in order to see the list of supported command-line flags with their description and default values:

```bash
/path/to/victoria-traces -help
```

The following command-line flags are used the most:

* `-storageDataPath` - VictoriaTraces stores all the data in this directory. The default path is `victoria-traces-data` in the current working directory.
* `-retentionPeriod` - retention for stored data. Older data is automatically deleted. Default retention is 7 days.

Once it's running, it will listen to port `9428` (`-httpListenAddr`) and provide the following APIs:
1. for ingestion:
```
http://<victoria-traces>:9428/insert/opentelemetry/v1/traces
```
2. for querying:
```
http://<victoria-traces>:9428/select/jaeger/<endpoints>
```

See [data ingestion](https://docs.victoriametrics.com/victoriatraces/data-ingestion/) and [querying](https://docs.victoriametrics.com/VictoriaTraces/querying/) for more details.

## How does it work

VictoriaTraces was initially built on top of [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/), a log database.
It receives trace spans in OTLP format, transforms them into structured logs, and provides [Jaeger Query Service JSON APIs](https://www.jaegertracing.io/docs/2.6/apis/#internal-http-json) for querying.

For detailed data model and example, see: [Key Concepts](https://docs.victoriametrics.com/victoriatraces/keyConcepts).

![How does VictoriaTraces work](how-does-it-work.webp)

Building VictoriaTraces in this way enables it to scale easily and linearly with the available resources, like VictoriaLogs.

## List of command-line flags

```shell
  -search.traceMaxDurationWindow
    	The lookbehind/lookahead window of searching for the rest trace spans after finding one span.
        It allows extending the search start time and end time by `-search.traceMaxDurationWindow` to make sure all spans are included.
        It affects both Jaeger's `/api/traces` and `/api/traces/<trace_id>` APIs. (default: 10m)
  -search.traceMaxServiceNameList
        The maximum number of service name can return in a get service name request.
        This limit affects Jaeger's `/api/services` API. (default: 1000)
  -search.traceMaxSpanNameList
        The maximum number of span name can return in a get span name request.
        This limit affects Jaeger's `/api/services/*/operations` API. (default: 1000)
  -search.traceSearchStep
        Splits the [0, now] time range into many small time ranges by -search.traceSearchStep
        when searching for spans by `trace_id`. Once it finds spans in a time range, it performs an additional search according to `-search.traceMaxDurationWindow` and then stops.
        It affects Jaeger's `/api/traces/<trace_id>` API.  (default: 1d)
  -search.traceServiceAndSpanNameLookbehind
        The time range of searching for service names and span names. 
        It affects Jaeger's `/api/services` and `/api/services/*/operations` APIs. (default: 7d)
```

See also: [VictoriaLogs - List of Command-line flags](https://docs.victoriametrics.com/victorialogs/#list-of-command-line-flags)