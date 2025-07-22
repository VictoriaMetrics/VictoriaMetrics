package metrics

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Set is a set of metrics.
//
// Metrics belonging to a set are exported separately from global metrics.
//
// Set.WritePrometheus must be called for exporting metrics from the set.
type Set struct {
	mu        sync.Mutex
	a         []*namedMetric
	m         map[string]*namedMetric
	summaries []*Summary

	metricsWriters []func(w io.Writer)
}

// NewSet creates new set of metrics.
//
// Pass the set to RegisterSet() function in order to export its metrics via global WritePrometheus() call.
func NewSet() *Set {
	return &Set{
		m: make(map[string]*namedMetric),
	}
}

// WritePrometheus writes all the metrics from s to w in Prometheus format.
func (s *Set) WritePrometheus(w io.Writer) {
	// Collect all the metrics in in-memory buffer in order to prevent from long locking due to slow w.
	var bb bytes.Buffer
	lessFunc := func(i, j int) bool {
		return s.a[i].name < s.a[j].name
	}
	s.mu.Lock()
	for _, sm := range s.summaries {
		sm.updateQuantiles()
	}
	if !sort.SliceIsSorted(s.a, lessFunc) {
		sort.Slice(s.a, lessFunc)
	}
	sa := append([]*namedMetric(nil), s.a...)
	metricsWriters := s.metricsWriters
	s.mu.Unlock()

	prevMetricFamily := ""
	for _, nm := range sa {
		metricFamily := getMetricFamily(nm.name)
		if metricFamily != prevMetricFamily {
			// write meta info only once per metric family
			metricType := nm.metric.metricType()
			WriteMetadataIfNeeded(&bb, nm.name, metricType)
			prevMetricFamily = metricFamily
		}
		// Call marshalTo without the global lock, since certain metric types such as Gauge
		// can call a callback, which, in turn, can try calling s.mu.Lock again.
		nm.metric.marshalTo(nm.name, &bb)
	}
	w.Write(bb.Bytes())

	for _, writeMetrics := range metricsWriters {
		writeMetrics(w)
	}
}

// NewHistogram creates and returns new histogram in s with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func (s *Set) NewHistogram(name string) *Histogram {
	h := &Histogram{}
	s.registerMetric(name, h)
	return h
}

// GetOrCreateHistogram returns registered histogram in s with the given name
// or creates new histogram if s doesn't contain histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewHistogram instead of GetOrCreateHistogram.
func (s *Set) GetOrCreateHistogram(name string) *Histogram {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing histogram.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name:   name,
			metric: &Histogram{},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	h, ok := nm.metric.(*Histogram)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Histogram. It is %T", name, nm.metric))
	}
	return h
}

// NewPrometheusHistogram creates and returns new PrometheusHistogram in s
// with the given name and PrometheusHistogramDefaultBuckets.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func (s *Set) NewPrometheusHistogram(name string) *PrometheusHistogram {
	return s.NewPrometheusHistogramExt(name, PrometheusHistogramDefaultBuckets)
}

// NewPrometheusHistogramExt creates and returns new PrometheusHistogram in s
// with the given name and upperBounds.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func (s *Set) NewPrometheusHistogramExt(name string, upperBounds []float64) *PrometheusHistogram {
	h := newPrometheusHistogram(upperBounds)
	s.registerMetric(name, h)
	return h
}

// GetOrCreatePrometheusHistogram returns registered prometheus histogram in s
// with the given name or creates new histogram if s doesn't contain histogram
// with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewPrometheusHistogram instead of GetOrCreatePrometheusHistogram.
func (s *Set) GetOrCreatePrometheusHistogram(name string) *PrometheusHistogram {
	return s.GetOrCreatePrometheusHistogramExt(name, PrometheusHistogramDefaultBuckets)
}

// GetOrCreatePrometheusHistogramExt returns registered prometheus histogram in
// s with the given name or creates new histogram if s doesn't contain
// histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewPrometheusHistogramExt instead of GetOrCreatePrometheusHistogramExt.
func (s *Set) GetOrCreatePrometheusHistogramExt(name string, upperBounds []float64) *PrometheusHistogram {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing histogram.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name:   name,
			metric: newPrometheusHistogram(upperBounds),
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	h, ok := nm.metric.(*PrometheusHistogram)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a PrometheusHistogram. It is %T", name, nm.metric))
	}
	return h
}

// NewCounter registers and returns new counter with the given name in the s.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned counter is safe to use from concurrent goroutines.
func (s *Set) NewCounter(name string) *Counter {
	c := &Counter{}
	s.registerMetric(name, c)
	return c
}

// GetOrCreateCounter returns registered counter in s with the given name
// or creates new counter if s doesn't contain counter with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned counter is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewCounter instead of GetOrCreateCounter.
func (s *Set) GetOrCreateCounter(name string) *Counter {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing counter.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name:   name,
			metric: &Counter{},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	c, ok := nm.metric.(*Counter)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Counter. It is %T", name, nm.metric))
	}
	return c
}

// NewFloatCounter registers and returns new FloatCounter with the given name in the s.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned FloatCounter is safe to use from concurrent goroutines.
func (s *Set) NewFloatCounter(name string) *FloatCounter {
	c := &FloatCounter{}
	s.registerMetric(name, c)
	return c
}

// GetOrCreateFloatCounter returns registered FloatCounter in s with the given name
// or creates new FloatCounter if s doesn't contain FloatCounter with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned FloatCounter is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewFloatCounter instead of GetOrCreateFloatCounter.
func (s *Set) GetOrCreateFloatCounter(name string) *FloatCounter {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing counter.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name:   name,
			metric: &FloatCounter{},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	c, ok := nm.metric.(*FloatCounter)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Counter. It is %T", name, nm.metric))
	}
	return c
}

// NewGauge registers and returns gauge with the given name in s, which calls f
// to obtain gauge value.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// f must be safe for concurrent calls.
//
// The returned gauge is safe to use from concurrent goroutines.
func (s *Set) NewGauge(name string, f func() float64) *Gauge {
	g := &Gauge{
		f: f,
	}
	s.registerMetric(name, g)
	return g
}

// GetOrCreateGauge returns registered gauge with the given name in s
// or creates new gauge if s doesn't contain gauge with the given name.
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
func (s *Set) GetOrCreateGauge(name string, f func() float64) *Gauge {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing gauge.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name: name,
			metric: &Gauge{
				f: f,
			},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	g, ok := nm.metric.(*Gauge)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Gauge. It is %T", name, nm.metric))
	}
	return g
}

// NewSummary creates and returns new summary with the given name in s.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func (s *Set) NewSummary(name string) *Summary {
	return s.NewSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// NewSummaryExt creates and returns new summary in s with the given name,
// window and quantiles.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func (s *Set) NewSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	if err := ValidateMetric(name); err != nil {
		panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
	}
	sm := newSummary(window, quantiles)

	s.mu.Lock()
	// defer will unlock in case of panic
	// checks in tests
	defer s.mu.Unlock()

	s.mustRegisterLocked(name, sm, false)
	registerSummaryLocked(sm)
	s.registerSummaryQuantilesLocked(name, sm)
	s.summaries = append(s.summaries, sm)
	return sm
}

// GetOrCreateSummary returns registered summary with the given name in s
// or creates new summary if s doesn't contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummary instead of GetOrCreateSummary.
func (s *Set) GetOrCreateSummary(name string) *Summary {
	return s.GetOrCreateSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// GetOrCreateSummaryExt returns registered summary with the given name,
// window and quantiles in s or creates new summary if s doesn't
// contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummaryExt instead of GetOrCreateSummaryExt.
func (s *Set) GetOrCreateSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing summary.
		if err := ValidateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		sm := newSummary(window, quantiles)
		nmNew := &namedMetric{
			name:   name,
			metric: sm,
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
			registerSummaryLocked(sm)
			s.registerSummaryQuantilesLocked(name, sm)
		}
		s.summaries = append(s.summaries, sm)
		s.mu.Unlock()
	}
	sm, ok := nm.metric.(*Summary)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Summary. It is %T", name, nm.metric))
	}
	if sm.window != window {
		panic(fmt.Errorf("BUG: invalid window requested for the summary %q; requested %s; need %s", name, window, sm.window))
	}
	if !isEqualQuantiles(sm.quantiles, quantiles) {
		panic(fmt.Errorf("BUG: invalid quantiles requested from the summary %q; requested %v; need %v", name, quantiles, sm.quantiles))
	}
	return sm
}

func (s *Set) registerSummaryQuantilesLocked(name string, sm *Summary) {
	for i, q := range sm.quantiles {
		quantileValueName := addTag(name, fmt.Sprintf(`quantile="%g"`, q))
		qv := &quantileValue{
			sm:  sm,
			idx: i,
		}
		s.mustRegisterLocked(quantileValueName, qv, true)
	}
}

func (s *Set) registerMetric(name string, m metric) {
	if err := ValidateMetric(name); err != nil {
		panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
	}
	s.mu.Lock()
	// defer will unlock in case of panic
	// checks in test
	defer s.mu.Unlock()
	s.mustRegisterLocked(name, m, false)
}

// mustRegisterLocked registers given metric with the given name.
//
// Panics if the given name was already registered before.
func (s *Set) mustRegisterLocked(name string, m metric, isAux bool) {
	nm, ok := s.m[name]
	if !ok {
		nm = &namedMetric{
			name:   name,
			metric: m,
			isAux:  isAux,
		}
		s.m[name] = nm
		s.a = append(s.a, nm)
	}
	if ok {
		panic(fmt.Errorf("BUG: metric %q is already registered", name))
	}
}

// UnregisterMetric removes metric with the given name from s.
//
// True is returned if the metric has been removed.
// False is returned if the given metric is missing in s.
func (s *Set) UnregisterMetric(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	nm, ok := s.m[name]
	if !ok {
		return false
	}
	if nm.isAux {
		// Do not allow deleting auxiliary metrics such as summary_metric{quantile="..."}
		// Such metrics must be deleted via parent metric name, e.g. summary_metric .
		return false
	}
	return s.unregisterMetricLocked(nm)
}

func (s *Set) unregisterMetricLocked(nm *namedMetric) bool {
	name := nm.name
	delete(s.m, name)

	deleteFromList := func(metricName string) {
		for i, nm := range s.a {
			if nm.name == metricName {
				s.a = append(s.a[:i], s.a[i+1:]...)
				return
			}
		}
		panic(fmt.Errorf("BUG: cannot find metric %q in the list of registered metrics", name))
	}

	// remove metric from s.a
	deleteFromList(name)

	sm, ok := nm.metric.(*Summary)
	if !ok {
		// There is no need in cleaning up non-summary metrics.
		return true
	}

	// cleanup registry from per-quantile metrics
	for _, q := range sm.quantiles {
		quantileValueName := addTag(name, fmt.Sprintf(`quantile="%g"`, q))
		delete(s.m, quantileValueName)
		deleteFromList(quantileValueName)
	}

	// Remove sm from s.summaries
	found := false
	for i, xsm := range s.summaries {
		if xsm == sm {
			s.summaries = append(s.summaries[:i], s.summaries[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Errorf("BUG: cannot find summary %q in the list of registered summaries", name))
	}
	unregisterSummary(sm)
	return true
}

// UnregisterAllMetrics de-registers all metrics registered in s.
//
// It also de-registers writeMetrics callbacks passed to RegisterMetricsWriter.
func (s *Set) UnregisterAllMetrics() {
	metricNames := s.ListMetricNames()
	for _, name := range metricNames {
		s.UnregisterMetric(name)
	}

	s.mu.Lock()
	s.metricsWriters = nil
	s.mu.Unlock()
}

// ListMetricNames returns sorted list of all the metrics in s.
//
// The returned list doesn't include metrics generated by metricsWriter passed to RegisterMetricsWriter.
func (s *Set) ListMetricNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	metricNames := make([]string, 0, len(s.m))
	for _, nm := range s.m {
		if nm.isAux {
			continue
		}
		metricNames = append(metricNames, nm.name)
	}
	sort.Strings(metricNames)
	return metricNames
}

// RegisterMetricsWriter registers writeMetrics callback for including metrics in the output generated by s.WritePrometheus.
//
// The writeMetrics callback must write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
// The last line generated by writeMetrics must end with \n.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is OK to reguster multiple writeMetrics callbacks - all of them will be called sequentially for gererating the output at s.WritePrometheus.
func (s *Set) RegisterMetricsWriter(writeMetrics func(w io.Writer)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metricsWriters = append(s.metricsWriters, writeMetrics)
}
