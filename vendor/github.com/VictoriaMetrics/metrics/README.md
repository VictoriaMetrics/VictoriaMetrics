[![Build Status](https://github.com/VictoriaMetrics/metrics/workflows/main/badge.svg)](https://github.com/VictoriaMetrics/metrics/actions)
[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/metrics?status.svg)](http://godoc.org/github.com/VictoriaMetrics/metrics)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/metrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/metrics)
[![codecov](https://codecov.io/gh/VictoriaMetrics/metrics/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/metrics)


# metrics - lightweight package for exporting metrics in Prometheus format


### Features

* Lightweight. Has minimal number of third-party dependencies and all these deps are small.
  See [this article](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d) for details.
* Easy to use. See the [API docs](http://godoc.org/github.com/VictoriaMetrics/metrics).
* Fast.
* Allows exporting distinct metric sets via distinct endpoints. See [Set](http://godoc.org/github.com/VictoriaMetrics/metrics#Set).
* Supports [easy-to-use histograms](http://godoc.org/github.com/VictoriaMetrics/metrics#Histogram), which just work without any tuning.
  Read more about VictoriaMetrics histograms at [this article](https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).
* Can push metrics to VictoriaMetrics or to any other remote storage, which accepts metrics
  in [Prometheus text exposition format](https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format).
  See [these docs](http://godoc.org/github.com/VictoriaMetrics/metrics#InitPush).


### Limitations

* It doesn't implement advanced functionality from [github.com/prometheus/client_golang](https://godoc.org/github.com/prometheus/client_golang).


### Usage

```go
import "github.com/VictoriaMetrics/metrics"

// Register various metrics.
// Metric name may contain labels in Prometheus format - see below.
var (
	// Register counter without labels.
	requestsTotal = metrics.NewCounter("requests_total")

	// Register summary with a single label.
	requestDuration = metrics.NewSummary(`requests_duration_seconds{path="/foobar/baz"}`)

	// Register gauge with two labels.
	queueSize = metrics.NewGauge(`queue_size{queue="foobar",topic="baz"}`, func() float64 {
		return float64(foobarQueue.Len())
	})

	// Register histogram with a single label.
	responseSize = metrics.NewHistogram(`response_size{path="/foo/bar"}`)
)

// ...
func requestHandler() {
	// Increment requestTotal counter.
	requestsTotal.Inc()

	startTime := time.Now()
	processRequest()
	// Update requestDuration summary.
	requestDuration.UpdateDuration(startTime)

	// Update responseSize histogram.
	responseSize.Update(responseSize)
}

// Expose the registered metrics at `/metrics` path.
http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
	metrics.WritePrometheus(w, true)
})

// ... or push registered metrics every 10 seconds to http://victoria-metrics:8428/api/v1/import/prometheus
// with the added `instance="foobar"` label to all the pushed metrics.
metrics.InitPush("http://victoria-metrics:8428/api/v1/import/prometheus", 10*time.Second, `instance="foobar"`, true)
```

By default, exposed metrics [do not have](https://github.com/VictoriaMetrics/metrics/issues/48#issuecomment-1620765811)
`TYPE` or `HELP` meta information. Call [`ExposeMetadata(true)`](https://pkg.go.dev/github.com/VictoriaMetrics/metrics#ExposeMetadata)
in order to generate `TYPE` and `HELP` meta information per each metric.

See [docs](https://pkg.go.dev/github.com/VictoriaMetrics/metrics) for more info.

### Users

* `Metrics` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.


### FAQ

#### Why the `metrics` API isn't compatible with `github.com/prometheus/client_golang`?

Because the `github.com/prometheus/client_golang` is too complex and is hard to use.


#### Why the `metrics.WritePrometheus` doesn't expose documentation for each metric?

Because this documentation is ignored by Prometheus. The documentation is for users.
Just give [meaningful names to the exported metrics](https://prometheus.io/docs/practices/naming/#metric-names)
or add comments in the source code or in other suitable place explaining each metric exposed from your application.


#### How to implement [CounterVec](https://godoc.org/github.com/prometheus/client_golang/prometheus#CounterVec) in `metrics`?

Just use [GetOrCreateCounter](http://godoc.org/github.com/VictoriaMetrics/metrics#GetOrCreateCounter)
instead of `CounterVec.With`. See [this example](https://pkg.go.dev/github.com/VictoriaMetrics/metrics#example-Counter-Vec) for details.


#### Why [Histogram](http://godoc.org/github.com/VictoriaMetrics/metrics#Histogram) buckets contain `vmrange` labels instead of `le` labels like in Prometheus histograms?

Buckets with `vmrange` labels occupy less disk space compared to Prometheus-style buckets with `le` labels,
because `vmrange` buckets don't include counters for the previous ranges. [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) provides `prometheus_buckets`
function, which converts `vmrange` buckets to Prometheus-style buckets with `le` labels. This is useful for building heatmaps in Grafana.
Additionally, its `histogram_quantile` function transparently handles histogram buckets with `vmrange` labels.

However, for compatibility purposes package provides classic [Prometheus Histograms](http://godoc.org/github.com/VictoriaMetrics/metrics#PrometheusHistogram) with `le` labels.
