package main

import (
	"bytes"
	"flag"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

var (
	cardinalityMetricsWrites        = metrics.NewCounter(`vmestimator_write_cardinality_metrics_total`)
	cardinalityMetricsWriteDuration = metrics.NewFloatCounter(`vmestimator_write_cardinality_metrics_duration_seconds_total`)
	cardinalityMetricsWriteBytes    = metrics.NewCounter(`vmestimator_write_cardinality_metrics_size_bytes_total`)

	cardinalityCacheMu         sync.Mutex
	cardinalityMetricsCacheAt  time.Time
	cardinalityMetricsCache    []byte
	cardinalityMetricsCacheTTL = flag.Duration("cardinalityMetrics.cacheTTL", time.Second*30, "Duration for caching cardinality metrics response")
	cardinalityMetricsExposeAt = flag.String(`cardinalityMetrics.exposeAt`, `/metrics`, "HTTP path for exposing cardinality metrics. "+
		"If set to the default /metrics, cardinality metrics are merged with regular metrics and exposed together. "+
		"If set to a different path, only cardinality metrics are exposed at that endpoint. "+
		"If set to an empty value, cardinality metrics are not exposed via HTTP at all.")
)

func writeCardinalityMetrics(w io.Writer, es []*estimator) {
	startTime := time.Now()

	cardinalityCacheMu.Lock()
	if time.Since(cardinalityMetricsCacheAt) >= *cardinalityMetricsCacheTTL || *cardinalityMetricsCacheTTL == 0 {
		plain := bytes.NewBuffer(cardinalityMetricsCache[:0])
		for _, e := range es {
			e.writeMetrics(plain)
		}
		cardinalityMetricsCache = plain.Bytes()
		cardinalityMetricsCacheAt = time.Now()
	}
	cm := cardinalityMetricsCache
	cardinalityCacheMu.Unlock()

	_, _ = w.Write(cm)

	cardinalityMetricsWrites.Inc()
	cardinalityMetricsWriteDuration.Add(time.Since(startTime).Seconds())
	cardinalityMetricsWriteBytes.Add(len(cm))
}
