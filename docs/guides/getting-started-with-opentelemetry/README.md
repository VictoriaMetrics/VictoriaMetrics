VictoriaMetrics and VictoriaLogs support ingestion of OpenTelemetry [metrics](https://docs.victoriametrics.com/single-server-victoriametrics/#sending-data-via-opentelemetry) and [logs](https://docs.victoriametrics.com/victorialogs/data-ingestion/opentelemetry/) respectively.
This guide covers data ingestion via [opentelemetry-collector](https://opentelemetry.io/docs/collector/) and direct metrics and logs push from application.

## Pre-Requirements  

* [kubernetes cluster](https://kubernetes.io/docs/tasks/tools/#kind)
* [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [helm](https://helm.sh/docs/intro/install/)

### Install VictoriaMetrics and VictoriaLogs

Install VictoriaMetrics helm repo:
```sh
helm repo add vm https://victoriametrics.github.io/helm-charts/
helm repo update
```

Add VictoriaMetrics chart values to sanitize OTEL metrics:
```sh
cat << EOF > vm-values.yaml
server:
  extraArgs:
    opentelemetry.usePrometheusNaming: true
EOF
```

Install VictoriaMetrics single-server version
```sh
helm install victoria-metrics vm/victoria-metrics-single -f vm-values.yaml
```

Verify it's up and running:

```sh
kubectl get pods
# NAME                                                READY   STATUS    RESTARTS   AGE
# victoria-metrics-victoria-metrics-single-server-0   1/1     Running   0          3m1s
```

VictoriaMetrics helm chart provides the following URL for writing data:

```text
Write URL inside the kubernetes cluster:
  http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local.:8428/<protocol-specific-write-endpoint>

All supported write endpoints can be found at https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-time-series-data.
```

For OpenTelemetry VictoriaMetrics write endpoint is:
```text
http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local.:8428/opentelemetry/v1/metrics
```

Install VictoriaLogs single-server version
```sh
helm install victoria-logs vm/victoria-logs-single
```

Verify it's up and running:

```sh
kubectl get pods
# NAME                                            READY   STATUS    RESTARTS   AGE
# victoria-logs-victoria-logs-single-server-0     1/1     Running   0          1m10s
```

VictoriaLogs helm chart provides the following URL for writing data:

```text
Write URL inside the kubernetes cluster:
  http://victoria-logs-victoria-logs-single-server.default.svc.cluster.local.:9428/<protocol-specific-write-endpoint>

All supported write endpoints can be found at https://docs.victoriametrics.com/victorialogs/data-ingestion/
```

For OpenTelemetry VictoriaLogs write endpoint is:
```text
http://victoria-logs-victoria-logs-single-server.default.svc.cluster.local.:9428/insert/opentelemetry/v1/logs
```

## Using opentelemetry-collector with VictoriaMetrics and VictoriaLogs

![OTEL Collector](collector.webp)

### Deploy opentelemetry-collector and configure metrics and logs forwarding

Add OpenTelemetry helm repo
```sh
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update
```

Add OTEL Collector values
```sh
cat << EOF > otel-values.yaml
mode: deployment
image:
  repository: "otel/opentelemetry-collector-contrib"
presets:
  clusterMetrics:
    enabled: true
  logsCollection:
    enabled: true
config:
  # deltatocumulative processor is needed only to support metrics with delta temporality, which is not supported by VictoriaMetrics
  processors:
    deltatocumulative:
      max_stale: 5m
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318
  exporters:
    otlphttp/victoriametrics:
      compression: gzip
      encoding: proto
      # Setting below will work for sending data to VictoriaMetrics single-node version.
      # Cluster version of VictoriaMetrics will require a different URL - https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
      metrics_endpoint: http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local:8428/opentelemetry/v1/metrics
      logs_endpoint: http://victoria-logs-victoria-logs-single-server.default.svc.cluster.local:9428/insert/opentelemetry/v1/logs
      tls:
        insecure: true
  service:
    pipelines:
      logs:
        processors: []
        exporters: [otlphttp/victoriametrics]
      metrics:
        receivers: [otlp]
        processors: [deltatocumulative]
        exporters: [otlphttp/victoriametrics]
EOF
```

Install OTEL Collector helm chart
```sh
helm upgrade -i otel open-telemetry/opentelemetry-collector -f otel-values.yaml
```

Check if OTEL Collector pod is healthy
```
kubectl get pod
# NAME                                            READY   STATUS    RESTARTS   AGE
# otel-opentelemetry-collector-7467bbb559-2pq2n   1/1     Running   0          23m
```

Forward VictoriaMetrics port to local machine to verify metrics are ingested
```sh
kubectl port-forward svc/victoria-metrics-victoria-metrics-single-server 8428
```

Check metric `k8s_container_ready` via browser `http://localhost:8428/vmui/#/?g0.expr=k8s_container_ready`

Forward VictoriaMetrics port to local machine to verify metrics are ingested
```sh
kubectl port-forward svc/victoria-logs-victoria-logs-single-server 9428
```

Check ingested logs in browser at `http://localhost:9428/select/vmui`

The full version of possible configuration options can be found in [OpenTelemetry docs](https://opentelemetry.io/docs/collector/configuration/).

## Sending to VictoriaMetrics and VictoriaLogs via OpenTelemetry
Metrics and logs can be sent to VictoriaMetrics and VictoriaLogs via OpenTelemetry instrumentation libraries. You can use any compatible OpenTelemetry instrumentation [clients](https://opentelemetry.io/docs/languages/).
In our example, we'll create a WEB server in [Golang](https://go.dev/) and instrument it with metrics and logs.

### Building the Go application instrumented with metrics and logs
Copy the go file from [here](app.go-collector.example). This will give you a basic implementation of a dice roll WEB server with the urls for opentelemetry-collector pointing to localhost:4318. 
In the same directory run the following command to create the `go.mod` file:
```sh
go mod init vm/otel
```

For demo purposes, we'll add the following dependencies to `go.mod` file:
```go
require (
        go.opentelemetry.io/contrib/bridges/otelslog v0.7.0
        go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.57.0
        go.opentelemetry.io/otel v1.32.0
        go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.8.0
        go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.32.0
        go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.32.0
        go.opentelemetry.io/otel/log v0.8.0
        go.opentelemetry.io/otel/metric v1.32.0
        go.opentelemetry.io/otel/sdk v1.32.0
        go.opentelemetry.io/otel/sdk/log v0.8.0
        go.opentelemetry.io/otel/sdk/metric v1.32.0
)

require (
        github.com/cenkalti/backoff/v4 v4.3.0 // indirect
        github.com/felixge/httpsnoop v1.0.4 // indirect
        github.com/go-logr/logr v1.4.2 // indirect
        github.com/go-logr/stdr v1.2.2 // indirect
        github.com/google/uuid v1.6.0 // indirect
        github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0 // indirect
        go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.32.0 // indirect
        go.opentelemetry.io/otel/trace v1.32.0 // indirect
        go.opentelemetry.io/proto/otlp v1.4.0 // indirect
        golang.org/x/net v0.32.0 // indirect
        golang.org/x/sys v0.28.0 // indirect
        golang.org/x/text v0.21.0 // indirect
        google.golang.org/genproto/googleapis/api v0.0.0-20241209162323-e6fa225c2576 // indirect
        google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
        google.golang.org/grpc v1.68.1 // indirect
        google.golang.org/protobuf v1.35.2 // indirect
)
```

Once you have these in your `go.mod` file, you can run the following command to download the dependencies:
```sh
go mod tidy
```

Now you can run the application:
```sh
go run .
```

### Test ingestion
By default, the application will be available at `localhost:8080`. You can start sending requests to /rolldice endpoint to generate metrics. The following command will send 20 requests to the /rolldice endpoint:
```sh
for i in `seq 1 20`; do curl http://localhost:8080/rolldice; done
```

After a few seconds you should start to see metrics sent to VictoriaMetrics by visiting `http://localhost:8428/vmui/#/?g0.expr=dice_rolls_total` in your browser or by querying the metric `dice_rolls_total` in the UI interface.
![Dice roll metrics](vmui-dice-roll-metrics.webp)

Logs should be available by visiting `http://localhost:9428/select/vmui` using query `service.name: unknown_service:otel`.
![Dice roll logs](vmui-dice-roll-logs.webp)

## Direct metrics and logs push

Metrics and logs can be ingested into VictoriaMetrics directly with HTTP requests. You can use any compatible OpenTelemetry 
instrumentation [clients](https://opentelemetry.io/docs/languages/).
In our example, we'll create a WEB server in [Golang](https://go.dev/) and instrument it with metrics and logs.

![OTEL direct](direct.webp)


### Building the Go application instrumented with metrics and logs

See the full source code of the example [here](app.go.example).

The list of OpenTelemetry dependencies for `go.mod` is the following:

```go
go 1.23.4

require (
        go.opentelemetry.io/contrib/bridges/otelslog v0.7.0
        go.opentelemetry.io/otel v1.32.0
        go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.8.0
        go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.32.0
        go.opentelemetry.io/otel/log v0.8.0
        go.opentelemetry.io/otel/metric v1.32.0
        go.opentelemetry.io/otel/sdk v1.32.0
        go.opentelemetry.io/otel/sdk/log v0.8.0
        go.opentelemetry.io/otel/sdk/metric v1.32.0
)

require (
        github.com/cenkalti/backoff/v4 v4.3.0 // indirect
        github.com/go-logr/logr v1.4.2 // indirect
        github.com/go-logr/stdr v1.2.2 // indirect
        github.com/google/uuid v1.6.0 // indirect
        github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0 // indirect
        go.opentelemetry.io/otel/trace v1.32.0 // indirect
        go.opentelemetry.io/proto/otlp v1.4.0 // indirect
        golang.org/x/net v0.32.0 // indirect
        golang.org/x/sys v0.28.0 // indirect
        golang.org/x/text v0.21.0 // indirect
        google.golang.org/genproto/googleapis/api v0.0.0-20241209162323-e6fa225c2576 // indirect
        google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
        google.golang.org/grpc v1.68.1 // indirect
        google.golang.org/protobuf v1.35.2 // indirect
)
```

Let's create a new file `main.go` with basic implementation of the WEB server:
```go
package main

func main() {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        mux := http.NewServeMux()
        mux.HandleFunc("/api/fast", func(writer http.ResponseWriter, request *http.Request) {
                logger.InfoContext(ctx, "Anonymous access to fast endpoint")
                writer.WriteHeader(http.StatusOK)
                writer.Write([]byte(`fast ok`))
        })
        mux.HandleFunc("/api/slow", func(writer http.ResponseWriter, request *http.Request) {
                time.Sleep(time.Second * 2)
                logger.InfoContext(ctx, "Anonymous access to slow endpoint")
                writer.WriteHeader(http.StatusOK)
                writer.Write([]byte(`slow ok`))
        })
        mw, err := newMiddleware(ctx, mux)
        if err != nil {
                panic(fmt.Sprintf("cannot build middleware: %q", err))
        }
        mustStop := make(chan os.Signal, 1)
        signal.Notify(mustStop, os.Interrupt, syscall.SIGTERM)
        go func() {
                http.ListenAndServe("localhost:8081", mw)
        }()
        log.Printf("web server started at localhost:8081.")
        <-mustStop
        log.Println("receive shutdown signal, stopping webserver")

        for _, shutdown := range mw.onShutdown {
                if err := shutdown(ctx); err != nil {
                        log.Println("cannot shutdown metric provider ", err)
                }
        }
        log.Printf("Done!")
}
```

In the code above, we used `newMiddleware` function to create a `handler` for our server.
Let's define it below:
```go
type middleware struct {
        ctx             context.Context
        h               http.Handler
        requestsCount   metric.Int64Counter
        requestsLatency metric.Float64Histogram
        activeRequests  int64
        onShutdown      []func(ctx context.Context) error
}

func newMiddleware(ctx context.Context, h http.Handler) (*middleware, error) {
        mw := &middleware{
                ctx: ctx,
                h:   h,
        }

        lp, err := newLoggerProvider(ctx)
        if err != nil {
                return nil, fmt.Errorf("cannot build logs provider: %w", err)
        }
        global.SetLoggerProvider(lp)

        mp, err := newMeterProvider(ctx)
        if err != nil {
                return nil, fmt.Errorf("cannot build metrics provider: %w", err)
        }
        otel.SetMeterProvider(mp)
        meter := mp.Meter("")

        mw.requestsLatency, err = meter.Float64Histogram("http.requests.latency")
        if err != nil {
                return nil, fmt.Errorf("cannot create histogram: %w", err)
        }
        mw.requestsCount, err = meter.Int64Counter("http.requests")
        if err != nil {
                return nil, fmt.Errorf("cannot create int64 counter: %w", err)
        }
        cb := func(c context.Context, o metric.Int64Observer) error {
                o.Observe(atomic.LoadInt64(&mw.activeRequests))
                return nil
        }
        _, err = meter.Int64ObservableGauge("http.requests.active", metric.WithInt64Callback(cb))
        if err != nil {
                return nil, fmt.Errorf("cannot create Int64ObservableGauge: %w", err)
        }
        mw.onShutdown = append(mw.onShutdown, mp.Shutdown, lp.Shutdown)

        return mw, nil
}
```

Also you can find there `logger`, which is `otel` logger:

```go
var (
        logger = otelslog.NewLogger("rolldice")
)
```

and initialized in a `newLoggerProvider`
```go
func newLoggerProvider(ctx context.Context) (*sdklog.LoggerProvider, error) {
        exporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(*logsEndpoint))
        if err != nil {
                return nil, err
        }
        provider := sdklog.NewLoggerProvider(
                sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
        )
        return provider, nil
}
```

The new type `middleware` is instrumented with 3 [metrics](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#timeseries-model)
initialized in `newMiddleware` method:
* counter `http.requests`
* histogram `http.requests.latency`
* gauge `http.requests.active`

Let's implement http.Handler interface for `middleWare` by adding `ServeHTTP` method:
```go
func (m *middleWare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        t := time.Now()
        path := r.URL.Path
        m.requestsCount.Add(m.ctx, 1, metric.WithAttributes(attribute.String("path", path)))
        atomic.AddInt64(&m.activeRequests, 1)
        defer func() {
                atomic.AddInt64(&m.activeRequests, -1)
                m.requestsLatency.Record(m.ctx, time.Since(t).Seconds(), metric.WithAttributes(attribute.String("path", path)))
        }()

        m.h.ServeHTTP(w, r)
}
```

In method above, our middleware processes received HTTP requests and updates metrics with each new request. 
But for these metrics to be shipped we need to add a new method `newMeterProvider` to organize metrics collection:
```go
func newMeterProvider(ctx context.Context) (*sdkmetric.MeterProvider, error) {
        exporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(*metricsEndpoint))
        if err != nil {
                return nil, fmt.Errorf("cannot create otlphttp exporter: %w", err)
        }

        res, err := resource.New(ctx,
                resource.WithAttributes(
                        attribute.String("job", *jobName),
                        attribute.String("instance", *instanceName),
                ),
        )
        if err != nil {
                return nil, fmt.Errorf("cannot create meter resource: %w", err)
        }
        expView := sdkmetric.NewView(
                sdkmetric.Instrument{
                        Name: "http.requests.latency",
                        Kind: sdkmetric.InstrumentKindHistogram,
                },
                sdkmetric.Stream{
                        Name: "http.requests.latency.exp",
                        Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
                                MaxSize:  160,
                                MaxScale: 20,
                        },
                },
        )
        return sdkmetric.NewMeterProvider(
                sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(*pushInterval))),
                sdkmetric.WithResource(res),
                sdkmetric.WithView(expView),
        ), nil
}
```

This controller will collect and push collected metrics to VictoriaMetrics address with interval of `10s`.

See the full source code of the example [here](app.go.example).

### Test ingestion

In order to push metrics and logs of our WEB server to VictoriaMetrics and VictoriaLogs it is necessary to ensure that both services are available locally.
In previous steps we already deployed a single-server VictoriaMetrics and VictoriaLogs, so let's make them available locally:

```sh
# port-forward victoriametrics to ingest metrics
kubectl port-forward victoria-metrics-victoria-metrics-single-server-0 8428
# port-forward victorialogs to ingest logs
kubectl port-forward victoria-logs-victoria-logs-single-server-0 9428
```

Now let's run our WEB server and call its APIs:
```sh
# build and run the app
go run main.go 
2024/03/25 19:27:41 Starting web server...
2024/03/25 19:27:41 web server started at localhost:8081.

# execute few queries with curl
curl http://localhost:8081/api/fast
curl http://localhost:8081/api/slow
```

Open VictoriaMetrics at `http://localhost:8428/vmui` and query `http_requests_total` or `http_requests_active`

![OTEL Metrics VMUI](vmui-direct-metrics.webp)

Open VictoriaLogs UI at `http://localhost:9428/select/vmui` and query `service.name: unknown_service:otel`

![OTEL Logs VMUI](vmui-direct-logs.webp)

## Limitations

* VictoriaMetrics and VictoriaLogs do not support experimental JSON encoding [format](https://github.com/open-telemetry/opentelemetry-proto/blob/main/examples/metrics.json).
* VictoriaMetrics supports only `AggregationTemporalityCumulative` type for [histogram](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram) and [summary](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#summary-legacy). Either consider using cumulative temporality temporality or try [`delta-to-cumulative processor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/deltatocumulativeprocessor) to make conversion to cumulative temporality in OTEL Collector.
