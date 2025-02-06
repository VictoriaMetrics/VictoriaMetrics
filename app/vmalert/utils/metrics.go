package utils

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type namedMetric struct {
	Name string
}

var usedMetrics map[string]*atomic.Int64
var usedMetricMu sync.Mutex

func trackUsedMetric(name string) {
	usedMetricMu.Lock()
	defer usedMetricMu.Unlock()

	if usedMetrics == nil {
		usedMetrics = make(map[string]*atomic.Int64)
	}
	if _, ok := usedMetrics[name]; !ok {
		usedMetrics[name] = &atomic.Int64{}
	}
	usedMetrics[name].Add(1)
}

// Unregister removes the metric by name from default registry
func (nm namedMetric) Unregister() {
	if usedMetrics == nil {
		logger.Fatalf("BUG: unregistered metric %q before registering", nm.Name)
	}

	usedMetricMu.Lock()
	counter, ok := usedMetrics[nm.Name]
	if !ok {
		logger.Fatalf("BUG: unregistered metric %q before registering", nm.Name)
	}
	current := counter.Add(-1)
	usedMetricMu.Unlock()

	if current < 0 {
		logger.Fatalf("BUG: negative metric counter for %q", nm.Name)
	}

	if current == 0 {
		metrics.UnregisterMetric(nm.Name)
	}

}

// Gauge is a metrics.Gauge with Name
type Gauge struct {
	namedMetric
	*metrics.Gauge
}

// GetOrCreateGauge creates a new Gauge with the given name
func GetOrCreateGauge(name string, f func() float64) *Gauge {
	trackUsedMetric(name)
	return &Gauge{
		namedMetric: namedMetric{Name: name},
		Gauge:       metrics.GetOrCreateGauge(name, f),
	}
}

// Counter is a metrics.Counter with Name
type Counter struct {
	namedMetric
	*metrics.Counter
}

// GetOrCreateCounter creates a new Counter with the given name
func GetOrCreateCounter(name string) *Counter {
	trackUsedMetric(name)
	return &Counter{
		namedMetric: namedMetric{Name: name},
		Counter:     metrics.GetOrCreateCounter(name),
	}
}

// Summary is a metrics.Summary with Name
type Summary struct {
	namedMetric
	*metrics.Summary
}

// GetOrCreateSummary creates a new Summary with the given name
func GetOrCreateSummary(name string) *Summary {
	trackUsedMetric(name)
	return &Summary{
		namedMetric: namedMetric{Name: name},
		Summary:     metrics.GetOrCreateSummary(name),
	}
}
