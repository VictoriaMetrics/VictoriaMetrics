// Package metrics implements Prometheus-compatible metrics for applications.
//
// This package is lightweight alternative to https://github.com/prometheus/client_golang
// with simpler API and smaller dependencies.
//
// Usage:
//
//     1. Register the required metrics via New* functions.
//     2. Expose them to `/metrics` page via WritePrometheus.
//     3. Update the registered metrics during application lifetime.
//
// The package has been extracted from https://victoriametrics.com/
package metrics

import (
	"io"
)

type namedMetric struct {
	name   string
	metric metric
}

type metric interface {
	marshalTo(prefix string, w io.Writer)
}

var defaultSet = NewSet()

// WritePrometheus writes all the registered metrics in Prometheus format to w.
//
// If exposeProcessMetrics is true, then various `go_*` and `process_*` metrics
// are exposed for the current process.
//
// The WritePrometheus func is usually called inside "/metrics" handler:
//
//     http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//         metrics.WritePrometheus(w, true)
//     })
//
func WritePrometheus(w io.Writer, exposeProcessMetrics bool) {
	defaultSet.WritePrometheus(w)
	if exposeProcessMetrics {
		WriteProcessMetrics(w)
	}
}

// WriteProcessMetrics writes additional process metrics in Prometheus format to w.
//
// Various `go_*` and `process_*` metrics are exposed for the currently
// running process.
//
// The WriteProcessMetrics func is usually called in combination with writing Set metrics
// inside "/metrics" handler:
//
//     http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//         mySet.WritePrometheus(w)
//         metrics.WriteProcessMetrics(w)
//     })
//
func WriteProcessMetrics(w io.Writer) {
	writeGoMetrics(w)
	writeProcessMetrics(w)
}
