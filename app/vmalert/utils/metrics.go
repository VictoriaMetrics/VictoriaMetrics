package utils

import "github.com/VictoriaMetrics/metrics"

type namedMetric struct {
	Name string
}

// Unregister removes the metric by name from default registry
func (nm namedMetric) Unregister() {
	metrics.UnregisterMetric(nm.Name)
}

// Gauge is a metrics.Gauge with Name
type Gauge struct {
	namedMetric
	*metrics.Gauge
}

// GetOrCreateGauge creates a new Gauge with the given name
func GetOrCreateGauge(name string, f func() float64) *Gauge {
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
	return &Summary{
		namedMetric: namedMetric{Name: name},
		Summary:     metrics.GetOrCreateSummary(name),
	}
}
