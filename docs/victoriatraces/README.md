VictoriaTraces is open source user-friendly database for distributed tracing data 
from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/).

VictoriaTraces provides the following features:
- It is resource-efficient and fast. It uses up to 3.7x less RAM and up to 2.6x less CPU than other solutions such as Grafana Tempo.
- VictoriaTraces' capacity and performance scales linearly with the available resources (CPU, RAM, disk IO, disk space).
- It accepts trace spans in the popular [OpenTelemetry protocol](https://opentelemetry.io/docs/specs/otel/protocol/)(OTLP), 
  which can be exported from applications, OpenTelemetry and various other collectors.
- It provides [Jaeger Query Service JSON APIs](https://www.jaegertracing.io/docs/2.6/apis/#internal-http-json) 
  which allows you to visualize trace with [Grafana](https://grafana.com/docs/grafana/latest/datasources/jaeger/) or [Jaeger Frontend](https://www.jaegertracing.io/docs/2.6/frontend-ui/).

![Visualization with Grafana](grafana-ui.webp)

## How does it work

VictoriaTraces is built on top of [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/), which is a log database. 
VictoriaTraces transforms trace spans into structured logs, ingests them, and uses LogsQL for querying to construct the data structure 
required by the trace query APIs.

![How does VictoriaTraces work](how-does-it-work.webp)

## Quick Start

Currently, VictoriaTraces is under actively developing. It can be built from VictoriaMetrics repository with:
```shell
make victoria-logs-prod
```

The `make` command generates a binary in `/bin` folder. It can be run with:
```shell
./victoria-logs-prod -storageDataPath=victoria-traces-data
```

Once it's running, it will listen to port `9428` (`-httpListenAddr`) and provide the following API for ingestion:
```
http://<victoria-traces>:<port>/insert/opentelemetry/v1/traces
```

Now, config your applications or trace collectors to export data to VictoriaTraces. Here's an example config for the OpenTelemetry Collector:
```yaml
exporters:
  otlphttp/victoriatraces:
    traces_endpoint: http://<victoria-traces>:<port>/insert/opentelemetry/v1/traces

service:
  pipelines:
    traces:
      exporters: [otlphttp/victoriatraces]
```

You can browse `http://<victoria-traces>:<port>/select/vmui` to verify the data ingestion, as trace spans should be displayed as logs.

And finally, to search and visualize traces with Grafana, add a new jaeger data source the following URL:
```
http://<victoria-traces>:<port>/select/jaeger`.
```

Now everything should be ready!

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