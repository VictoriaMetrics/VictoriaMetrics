package streamaggr

import "github.com/VictoriaMetrics/metrics"

type namedMetric struct {
	name string
}

// unregister removes the metric by name from default registry
func (nm namedMetric) unregister() {
	metrics.UnregisterMetric(nm.name)
}

// gauge is a metrics.Gauge with name
type gauge struct {
	namedMetric
	*metrics.Gauge
}

// getOrCreateGauge creates a new Gauge with the given name
func getOrCreateGauge(name string, f func() float64) *gauge {
	return &gauge{
		namedMetric: namedMetric{name: name},
		Gauge:       metrics.GetOrCreateGauge(name, f),
	}
}

// summary is a metrics.Summary with name
type summary struct {
	namedMetric
	*metrics.Summary
}

// getOrCreateSummary creates a new summary with the given name
func getOrCreateSummary(name string) *summary {
	return &summary{
		namedMetric: namedMetric{name: name},
		Summary:     metrics.GetOrCreateSummary(name),
	}
}

// counter is a metrics.Counter with name
type counter struct {
	namedMetric
	*metrics.Counter
}

// getOrCreateCounter creates a new Counter with the given name
func getOrCreateCounter(name string) *counter {
	return &counter{
		namedMetric: namedMetric{name: name},
		Counter:     metrics.GetOrCreateCounter(name),
	}
}
