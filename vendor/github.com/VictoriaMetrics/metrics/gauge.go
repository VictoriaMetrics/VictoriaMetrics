package metrics

import (
	"fmt"
	"io"
	"math"
	"sync/atomic"
)

// NewGauge registers and returns gauge with the given name, which calls f to obtain gauge value.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// f must be safe for concurrent calls.
// if f is nil, then it is expected that the gauge value is changed via Set(), Inc(), Dec() and Add() calls.
//
// The returned gauge is safe to use from concurrent goroutines.
//
// See also FloatCounter for working with floating-point values.
func NewGauge(name string, f func() float64) *Gauge {
	return defaultSet.NewGauge(name, f)
}

// Gauge is a float64 gauge.
type Gauge struct {
	// valueBits contains uint64 representation of float64 passed to Gauge.Set.
	valueBits uint64

	// f is a callback, which is called for returning the gauge value.
	f func() float64
}

// Get returns the current value for g.
func (g *Gauge) Get() float64 {
	if f := g.f; f != nil {
		return f()
	}
	n := atomic.LoadUint64(&g.valueBits)
	return math.Float64frombits(n)
}

// Set sets g value to v.
//
// The g must be created with nil callback in order to be able to call this function.
func (g *Gauge) Set(v float64) {
	if g.f != nil {
		panic(fmt.Errorf("cannot call Set on gauge created with non-nil callback"))
	}
	n := math.Float64bits(v)
	atomic.StoreUint64(&g.valueBits, n)
}

// Inc increments g by 1.
//
// The g must be created with nil callback in order to be able to call this function.
func (g *Gauge) Inc() {
	g.Add(1)
}

// Dec decrements g by 1.
//
// The g must be created with nil callback in order to be able to call this function.
func (g *Gauge) Dec() {
	g.Add(-1)
}

// Add adds fAdd to g. fAdd may be positive and negative.
//
// The g must be created with nil callback in order to be able to call this function.
func (g *Gauge) Add(fAdd float64) {
	if g.f != nil {
		panic(fmt.Errorf("cannot call Set on gauge created with non-nil callback"))
	}
	for {
		n := atomic.LoadUint64(&g.valueBits)
		f := math.Float64frombits(n)
		fNew := f + fAdd
		nNew := math.Float64bits(fNew)
		if atomic.CompareAndSwapUint64(&g.valueBits, n, nNew) {
			break
		}
	}
}

func (g *Gauge) marshalTo(prefix string, w io.Writer) {
	v := g.Get()
	if float64(int64(v)) == v {
		// Marshal integer values without scientific notation
		fmt.Fprintf(w, "%s %d\n", prefix, int64(v))
	} else {
		fmt.Fprintf(w, "%s %g\n", prefix, v)
	}
}

func (g *Gauge) metricType() string {
	return "gauge"
}

// GetOrCreateGauge returns registered gauge with the given name
// or creates new gauge if the registry doesn't contain gauge with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned gauge is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewGauge instead of GetOrCreateGauge.
//
// See also FloatCounter for working with floating-point values.
func GetOrCreateGauge(name string, f func() float64) *Gauge {
	return defaultSet.GetOrCreateGauge(name, f)
}
