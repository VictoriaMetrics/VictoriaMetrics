package vmalertutil

import "github.com/VictoriaMetrics/metrics"

type namedMetric struct {
	Name string

	set *metrics.Set
}

// Unregister removes the metric by name from default registry
func (nm namedMetric) Unregister() {
	nm.set.UnregisterMetric(nm.Name)
}

// Gauge is a metrics.Gauge with Name
type Gauge struct {
	namedMetric
	*metrics.Gauge
}

// NewGauge creates a new Gauge with the given name
func NewGauge(set *metrics.Set, name string, f func() float64) *Gauge {
	return &Gauge{
		namedMetric: namedMetric{Name: name, set: set},
		Gauge:       set.NewGauge(name, f),
	}
}

// Counter is a metrics.Counter with Name
type Counter struct {
	namedMetric
	*metrics.Counter
}

// NewCounter creates a new Counter with the given name
func NewCounter(set *metrics.Set, name string) *Counter {
	return &Counter{
		namedMetric: namedMetric{Name: name, set: set},
		Counter:     set.NewCounter(name),
	}
}

// Summary is a metrics.Summary with Name
type Summary struct {
	namedMetric
	*metrics.Summary
}

// NewSummary creates a new Summary with the given name
func NewSummary(set *metrics.Set, name string) *Summary {
	return &Summary{
		namedMetric: namedMetric{Name: name, set: set},
		Summary:     set.NewSummary(name),
	}
}
