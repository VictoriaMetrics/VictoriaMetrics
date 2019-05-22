package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/valyala/histogram"
)

const defaultSummaryWindow = 5 * time.Minute

var defaultSummaryQuantiles = []float64{0.5, 0.9, 0.97, 0.99, 1}

// Summary implements summary.
type Summary struct {
	mu sync.Mutex

	curr *histogram.Fast
	next *histogram.Fast

	quantiles      []float64
	quantileValues []float64

	window time.Duration
}

// NewSummary creates and returns new summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummary(name string) *Summary {
	return NewSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// NewSummaryExt creates and returns new summary with the given name,
// window and quantiles.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	s := newSummary(window, quantiles)
	registerMetric(name, s)
	registerSummary(s)
	registerSummaryQuantiles(name, s)
	return s
}

func newSummary(window time.Duration, quantiles []float64) *Summary {
	// Make a copy of quantiles in order to prevent from their modification by the caller.
	quantiles = append([]float64{}, quantiles...)
	validateQuantiles(quantiles)
	s := &Summary{
		curr:           histogram.NewFast(),
		next:           histogram.NewFast(),
		quantiles:      quantiles,
		quantileValues: make([]float64, len(quantiles)),
		window:         window,
	}
	return s
}

func registerSummaryQuantiles(name string, s *Summary) {
	for i, q := range s.quantiles {
		quantileValueName := addTag(name, fmt.Sprintf(`quantile="%g"`, q))
		qv := &quantileValue{
			s:   s,
			idx: i,
		}
		registerMetric(quantileValueName, qv)
	}
}

func validateQuantiles(quantiles []float64) {
	for _, q := range quantiles {
		if q < 0 || q > 1 {
			panic(fmt.Errorf("BUG: quantile must be in the range [0..1]; got %v", q))
		}
	}
}

// Update updates the summary.
func (s *Summary) Update(v float64) {
	s.mu.Lock()
	s.curr.Update(v)
	s.next.Update(v)
	s.mu.Unlock()
}

// UpdateDuration updates request duration based on the given startTime.
func (s *Summary) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	s.Update(d)
}

func (s *Summary) marshalTo(prefix string, w io.Writer) {
	// Just update s.quantileValues and don't write anything to w.
	// s.quantileValues will be marshaled later via quantileValue.marshalTo.
	s.updateQuantiles()
}

func (s *Summary) updateQuantiles() {
	s.mu.Lock()
	s.quantileValues = s.curr.Quantiles(s.quantileValues[:0], s.quantiles)
	s.mu.Unlock()
}

// GetOrCreateSummary returns registered summary with the given name
// or creates new summary if the registry doesn't contain summary with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummary instead of GetOrCreateSummary.
func GetOrCreateSummary(name string) *Summary {
	return GetOrCreateSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// GetOrCreateSummaryExt returns registered summary with the given name,
// window and quantiles or creates new summary if the registry doesn't
// contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummaryExt instead of GetOrCreateSummaryExt.
func GetOrCreateSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	metricsMapLock.Lock()
	nm := metricsMap[name]
	metricsMapLock.Unlock()
	if nm == nil {
		// Slow path - create and register missing summary.
		if err := validateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		s := newSummary(window, quantiles)
		nmNew := &namedMetric{
			name:   name,
			metric: s,
		}
		mustRegisterQuantiles := false
		metricsMapLock.Lock()
		nm = metricsMap[name]
		if nm == nil {
			nm = nmNew
			metricsMap[name] = nm
			metricsList = append(metricsList, nm)
			registerSummary(s)
			mustRegisterQuantiles = true
		}
		metricsMapLock.Unlock()
		if mustRegisterQuantiles {
			registerSummaryQuantiles(name, s)
		}
	}
	s, ok := nm.metric.(*Summary)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Summary. It is %T", name, nm.metric))
	}
	if s.window != window {
		panic(fmt.Errorf("BUG: invalid window requested for the summary %q; requested %s; need %s", name, window, s.window))
	}
	if !isEqualQuantiles(s.quantiles, quantiles) {
		panic(fmt.Errorf("BUG: invalid quantiles requested from the summary %q; requested %v; need %v", name, quantiles, s.quantiles))
	}
	return s
}

func isEqualQuantiles(a, b []float64) bool {
	// Do not use relfect.DeepEqual, since it is slower than the direct comparison.
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type quantileValue struct {
	s   *Summary
	idx int
}

func (qv *quantileValue) marshalTo(prefix string, w io.Writer) {
	qv.s.mu.Lock()
	v := qv.s.quantileValues[qv.idx]
	qv.s.mu.Unlock()
	if !math.IsNaN(v) {
		fmt.Fprintf(w, "%s %g\n", prefix, v)
	}
}

func addTag(name, tag string) string {
	if len(name) == 0 || name[len(name)-1] != '}' {
		return fmt.Sprintf("%s{%s}", name, tag)
	}
	return fmt.Sprintf("%s,%s}", name[:len(name)-1], tag)
}

func registerSummary(s *Summary) {
	window := s.window
	summariesLock.Lock()
	summaries[window] = append(summaries[window], s)
	if len(summaries[window]) == 1 {
		go summariesSwapCron(window)
	}
	summariesLock.Unlock()
}

func summariesSwapCron(window time.Duration) {
	for {
		time.Sleep(window / 2)
		summariesLock.Lock()
		for _, s := range summaries[window] {
			s.mu.Lock()
			tmp := s.curr
			s.curr = s.next
			s.next = tmp
			s.next.Reset()
			s.mu.Unlock()
		}
		summariesLock.Unlock()
	}
}

var (
	summaries     = map[time.Duration][]*Summary{}
	summariesLock sync.Mutex
)
