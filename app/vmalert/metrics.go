package main

import "github.com/VictoriaMetrics/metrics"

type gauge struct {
	name string
	*metrics.Gauge
}

func getOrCreateGauge(name string, f func() float64) *gauge {
	return &gauge{
		name:  name,
		Gauge: metrics.GetOrCreateGauge(name, f),
	}
}

type counter struct {
	name string
	*metrics.Counter
}

func getOrCreateCounter(name string) *counter {
	return &counter{
		name:    name,
		Counter: metrics.GetOrCreateCounter(name),
	}
}

type summary struct {
	name string
	*metrics.Summary
}

func getOrCreateSummary(name string) *summary {
	return &summary{
		name:    name,
		Summary: metrics.GetOrCreateSummary(name),
	}
}
