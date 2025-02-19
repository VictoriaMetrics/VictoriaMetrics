package utils

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

// GetOrCreateGauge creates a new Gauge with the given name
func GetOrCreateGauge(set *metrics.Set, name string, f func() float64) *Gauge {
	return &Gauge{
		namedMetric: namedMetric{Name: name, set: set},
		Gauge:       set.GetOrCreateGauge(name, f),
	}
}

// Counter is a metrics.Counter with Name
type Counter struct {
	namedMetric
	*metrics.Counter
}

// GetOrCreateCounter creates a new Counter with the given name
func GetOrCreateCounter(set *metrics.Set, name string) *Counter {
	return &Counter{
		namedMetric: namedMetric{Name: name, set: set},
		Counter:     set.GetOrCreateCounter(name),
	}
}

// Summary is a metrics.Summary with Name
type Summary struct {
	namedMetric
	*metrics.Summary
}

// GetOrCreateSummary creates a new Summary with the given name
func GetOrCreateSummary(set *metrics.Set, name string) *Summary {
	return &Summary{
		namedMetric: namedMetric{Name: name, set: set},
		Summary:     set.GetOrCreateSummary(name),
	}
}
