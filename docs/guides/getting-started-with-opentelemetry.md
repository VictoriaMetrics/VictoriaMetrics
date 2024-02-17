---
weight: 5
title: How to use opentelemetry metrics with VictoriaMetrics
menu:
  docs:
    parent: "guides"
    weight: 5
---

# Application monitoring with opentelemetry

VictoriaMetrics supports metrics ingestion with [opentelemetry](https://opentelemetry.io/docs/specs/otel/metrics/) metrics format.
This guide covers data ingestion with [opentelemetry-colletor](https://opentelemetry.io/docs/collector/) and direct metrics push from application.

## Pre-Requirements  

 * kubernenetes cluster
 * kubectl
 * helm
### Install VictoriaMetrics single with helm chart 

Install single node version:
```sh
helm repo add vm https://victoriametrics.github.io/helm-charts/
helm repo update 
helm install victoria-metrics vm/victoria-metrics-single
```

 Check it's up and running:
```sh
kubectl get pods
# victoria-metrics-victoria-metrics-single-server-0   1/1     Running   0          3m1s
```

 Helm chart provides following urls for reading and writing data:

 ```text
   Write url inside the kubernetes cluster:
    http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local:8428

Rread Data:
  The following url can be used as the datasource url in Grafana::
    http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local:8428
```

## Using opentelemetry-colletor with VictoriaMetrics

### Deploy opentelemetry-colletor and configure metric forward

```sh
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update 

# add values
cat << EOF > values.yaml
presets:
  clusterMetrics:
    enabled: true
config:
  exporters:
   prometheusremotewrite:
     endpoint: "http://victoria-metrics-victoria-metrics-single-server.default.svc.cluster.local:8428/api/v1/write" 
  service:
      pipelines:
        metrics:
          receivers: [otlp]
          processors: []
          exporters: [prometheusremotewrite] 
EOF

# install helm chart
helm upgrade -i otl-colletor open-telemetry/opentelemetry-collector --set mode=deployment -f values.yaml

# check if pod is healthy
kubectl get pod
NAME                                                    READY   STATUS    RESTARTS   AGE
otl-colletor-opentelemetry-collector-7467bbb559-2pq2n   1/1     Running   0          23m

# forward port to local machine and check metrics ingestion flow
kubectl port-forward victoria-metrics-victoria-metrics-single-server-0 8428

# check metric k8s_container_ready
# at url with browser http://localhost:8428/vmui/#/?g0.expr=k8s_container_ready
```

  Full version of possible configuration options could be found at [docs](https://opentelemetry.io/docs/collector/configuration/)

## Direct metrics push
 Metrics could be ingested into VictoriaMetrics directly with http requests. You can use any compatible opentelemetry instrumentation [clients](https://opentelemetry.io/docs/languages/).
In our example, we'll create golang webserver and instrument metrics for it.

### create sample apple
At first we have to define go.mod with app dependecies:
```golang
module otlp-web-example

go 1.18

require (
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.30.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v0.30.0
	go.opentelemetry.io/otel/metric v0.30.0
	go.opentelemetry.io/otel/sdk v1.7.0
	go.opentelemetry.io/otel/sdk/metric v0.30.0
)
```
#### setup metrics controller 
```golang
func newMetricsController(ctx context.Context) (*controller.Controller, error) {
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(*collectorEndpoint),
		otlpmetrichttp.WithURLPath(*collectorURL),
	}
	if !*isSecure {
		options = append(options, otlpmetrichttp.WithInsecure())
	}

	metricExporter, err := otlpmetrichttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("cannot create otlphttp exporter: %w", err)
	}

	resourceConfig, err := resource.New(ctx, resource.WithAttributes(attribute.String("job", *jobName), attribute.String("instance", *instanceName)))
	if err != nil {
		return nil, fmt.Errorf("cannot create meter resource: %w", err)
	}
	meterController := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries([]float64{0.01, 0.05, 0.1, 0.5, 0.9, 1.0, 5.0, 10.0, 100.0}),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithExporter(metricExporter),
		controller.WithCollectPeriod(*pushInterval),
		controller.WithResource(resourceConfig),
	)
	if err := meterController.Start(ctx); err != nil {
		return nil, fmt.Errorf("cannot start meter controller: %w", err)
	}
	return meterController, nil
}
```
### add metric definitions
```golang
func newMetricsMiddleware(ctx context.Context, h http.Handler) (*metricMiddleWare, error) {
	mw := &metricMiddleWare{
		ctx: ctx,
		h:   h,
	}
	mc, err := newMetricsController(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot build metrics collector: %w", err)
	}
	global.SetMeterProvider(mc)

	prov := mc.Meter("")

	mw.requestsLatency, err = prov.SyncFloat64().Histogram("http_request_latency_seconds")
	if err != nil {
		return nil, fmt.Errorf("cannot create histogram: %w", err)
	}
	mw.requestsCount, err = prov.SyncInt64().Counter("http_requests_total")
	if err != nil {
		return nil, fmt.Errorf("cannot create syncInt64 counter: %w", err)
	}
	ar, err := prov.AsyncInt64().Gauge("http_active_requests")
	if err != nil {
		return nil, fmt.Errorf("cannot create AsyncInt64 gauge: %w", err)
	}
	if err := prov.RegisterCallback([]instrument.Asynchronous{ar}, func(ctx context.Context) {
		ar.Observe(ctx, atomic.LoadInt64(&mw.activeRequests))
	}); err != nil {
		return nil, fmt.Errorf("cannot Register int64 gauge: %w", err)
	}
	mw.onShutdown = mc.Stop

	return mw, nil
}
```
### full app example

```golang

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	collectorEndpoint = flag.String("vm.endpoint", "localhost:8428", "VictoriaMetrics endpoint - host:port.")
	collectorURL      = flag.String("vm.ingestPath", "/opentelemetry/api/v1/push", "url path for ingestion path.")
	isSecure          = flag.Bool("vm.isSecure", false, "enables https connection for metrics push.")
	pushInterval      = flag.Duration("vm.pushInterval", 10*time.Second, "how often push samples, aka scrapeInterval at pull model.")
	jobName           = flag.String("metrics.jobName", "otlp", "job name for web-application.")
	instanceName      = flag.String("metrics.instance", "localhost", "hostname of web-application instance.")
)

func newMetricsController(ctx context.Context) (*controller.Controller, error) {
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(*collectorEndpoint),
		otlpmetrichttp.WithURLPath(*collectorURL),
	}
	if !*isSecure {
		options = append(options, otlpmetrichttp.WithInsecure())
	}

	metricExporter, err := otlpmetrichttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("cannot create otlphttp exporter: %w", err)
	}

	resourceConfig, err := resource.New(ctx, resource.WithAttributes(attribute.String("job", *jobName), attribute.String("instance", *instanceName)))
	if err != nil {
		return nil, fmt.Errorf("cannot create meter resource: %w", err)
	}
	meterController := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries([]float64{0.01, 0.05, 0.1, 0.5, 0.9, 1.0, 5.0, 10.0, 100.0}),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithExporter(metricExporter),
		controller.WithCollectPeriod(*pushInterval),
		controller.WithResource(resourceConfig),
	)
	if err := meterController.Start(ctx); err != nil {
		return nil, fmt.Errorf("cannot start meter controller: %w", err)
	}
	return meterController, nil
}

func newMetricsMiddleware(ctx context.Context, h http.Handler) (*metricMiddleWare, error) {
	mw := &metricMiddleWare{
		ctx: ctx,
		h:   h,
	}
	mc, err := newMetricsController(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot build metrics collector: %w", err)
	}
	global.SetMeterProvider(mc)

	prov := mc.Meter("")

	mw.requestsLatency, err = prov.SyncFloat64().Histogram("http_request_latency_seconds")
	if err != nil {
		return nil, fmt.Errorf("cannot create histogram: %w", err)
	}
	mw.requestsCount, err = prov.SyncInt64().Counter("http_requests_total")
	if err != nil {
		return nil, fmt.Errorf("cannot create syncInt64 counter: %w", err)
	}
	ar, err := prov.AsyncInt64().Gauge("http_active_requests")
	if err != nil {
		return nil, fmt.Errorf("cannot create AsyncInt64 gauge: %w", err)
	}
	if err := prov.RegisterCallback([]instrument.Asynchronous{ar}, func(ctx context.Context) {
		ar.Observe(ctx, atomic.LoadInt64(&mw.activeRequests))
	}); err != nil {
		return nil, fmt.Errorf("cannot Register int64 gauge: %w", err)
	}
	mw.onShutdown = mc.Stop

	return mw, nil
}

type metricMiddleWare struct {
	ctx             context.Context
	h               http.Handler
	requestsCount   syncint64.Counter
	requestsLatency syncfloat64.Histogram
	activeRequests  int64

	onShutdown func(ctx context.Context) error
}

func (m *metricMiddleWare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	path := r.URL.Path
	m.requestsCount.Add(m.ctx, 1, attribute.String("path", path))
	atomic.AddInt64(&m.activeRequests, 1)
	defer func() {
		atomic.AddInt64(&m.activeRequests, -1)
		m.requestsLatency.Record(m.ctx, time.Since(t).Seconds(), attribute.String("path", path))
	}()

	m.h.ServeHTTP(w, r)
}

func main() {
	flag.Parse()
	log.Printf("Starting web server...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/fast", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(`fast ok`))
	})
	mux.HandleFunc("/api/slow", func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(time.Second * 2)
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(`slow ok`))
	})
	mw, err := newMetricsMiddleware(ctx, mux)
	if err != nil {
		panic(fmt.Sprintf("cannot build metricMiddleWare: %q", err))
	}
	mustStop := make(chan os.Signal, 1)
	signal.Notify(mustStop, os.Interrupt, syscall.SIGTERM)
	go func() {
		http.ListenAndServe("localhost:8081", mw)
	}()
	<-mustStop
	log.Println("receive shutdown signal, stopping webserver")

	if err := mw.onShutdown(ctx); err != nil {
		log.Println("cannot shutdown metric provider ", err)
	}

	cancel()
	log.Printf("Done!")
}

```

#### build and start app

```sh 
# build app
go build main.go 
# start metrics colllection
./main --vm.ingestPath=/opentelemetry/api/v1/push -vm.endpoint=localhost:8428
```

#### test metrics ingestion 

```sh
# port-forward victoriametrics to ingest metrics
kubectl port-forward victoria-metrics-victoria-metrics-single-server-0 8428

# open vmui and query `http_requests_total` and `http_request_latency_seconds_bucket`
```

## Known opentelemetry limitations.

 * VictoriaMetrics supports only `AggregationTemporalityCumulative` type for [histogram](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram) and [summary](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#summary-legacy)
 * VictoriaMetrics doesn't support experimental JSON encoding [format](https://github.com/open-telemetry/opentelemetry-proto/blob/main/examples/metrics.json).

