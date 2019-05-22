package metrics

import (
	"fmt"
	"io"
)

// NewGauge registers and returns gauge with the given name, which calls f
// to obtain gauge value.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// f must be safe for concurrent calls.
//
// The returned gauge is safe to use from concurrent goroutines.
func NewGauge(name string, f func() float64) *Gauge {
	if f == nil {
		panic(fmt.Errorf("BUG: f cannot be nil"))
	}
	g := &Gauge{
		f: f,
	}
	registerMetric(name, g)
	return g
}

// Gauge is a float64 gauge.
//
// See also Counter, which could be used as a gauge with Set and Dec calls.
type Gauge struct {
	f func() float64
}

// Get returns the current value for g.
func (g *Gauge) Get() float64 {
	return g.f()
}

func (g *Gauge) marshalTo(prefix string, w io.Writer) {
	v := g.f()
	if float64(int64(v)) == v {
		// Marshal integer values without scientific notations
		fmt.Fprintf(w, "%s %d\n", prefix, int64(v))
	} else {
		fmt.Fprintf(w, "%s %g\n", prefix, v)
	}
}

// GetOrCreateGauge returns registered gauge with the given name
// or creates new gauge if the registry doesn't contain gauge with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned gauge is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewGauge instead of GetOrCreateGauge.
func GetOrCreateGauge(name string, f func() float64) *Gauge {
	metricsMapLock.Lock()
	nm := metricsMap[name]
	metricsMapLock.Unlock()
	if nm == nil {
		// Slow path - create and register missing gauge.
		if f == nil {
			panic(fmt.Errorf("BUG: f cannot be nil"))
		}
		if err := validateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name: name,
			metric: &Gauge{
				f: f,
			},
		}
		metricsMapLock.Lock()
		nm = metricsMap[name]
		if nm == nil {
			nm = nmNew
			metricsMap[name] = nm
			metricsList = append(metricsList, nm)
		}
		metricsMapLock.Unlock()
	}
	g, ok := nm.metric.(*Gauge)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Gauge. It is %T", name, nm.metric))
	}
	return g
}
