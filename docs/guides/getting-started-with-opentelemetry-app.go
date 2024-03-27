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
	collectorEndpoint = flag.String("vm.endpoint", "localhost:8428", "VictoriaMetrics endpoint - host:port")
	collectorURL      = flag.String("vm.ingestPath", "/opentelemetry/api/v1/push", "url path for ingestion path")
	isSecure          = flag.Bool("vm.isSecure", false, "enables https connection for metrics push")
	pushInterval      = flag.Duration("vm.pushInterval", 10*time.Second, "how often push samples, aka scrapeInterval at pull model")
	jobName           = flag.String("metrics.jobName", "otlp", "job name for web-application")
	instanceName      = flag.String("metrics.instance", "localhost", "hostname of web-application instance")
)

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
	log.Printf("web server started at localhost:8081.")
	<-mustStop
	log.Println("receive shutdown signal, stopping webserver")

	if err := mw.onShutdown(ctx); err != nil {
		log.Println("cannot shutdown metric provider ", err)
	}

	cancel()
	log.Printf("Done!")
}

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
