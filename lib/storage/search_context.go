package storage

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/metrics"
)

// searchContext contains common options for Search.
type searchContext struct {
	*querytracer.Tracer

	deadline uint64

	source string

	// parent is the parent searchContext, which can be used to propagate metrics.
	parent *searchContext

	// readMetricIDs is used to track the number of read metric IDs during the search.
	readMetricIDs atomic.Uint64
}

func getSearchContext(deadline uint64, source string, qt *querytracer.Tracer) *searchContext {
	return &searchContext{
		Tracer:   qt,
		deadline: deadline,
		source:   source,

		readMetricIDs: atomic.Uint64{},
	}
}

// trackReadMetricIDs increments the readMetricIDs metric for the searchOptions.
func (sc *searchContext) trackReadMetricIDs(v uint64) {
	sc.readMetricIDs.Add(v)
}

// putsSearchContext Flushes the readMetricIDs to metric and puts the searchOptions back to the pool.
func putSearchContext(sc *searchContext) {
	summaryName := fmt.Sprintf(`vm_series_read_per_query{path=%q}`, sc.source)
	v := sc.readMetricIDs.Load()
	if v == 0 {
		return
	}

	metrics.GetOrCreateSummaryExt(summaryName, 1*time.Minute, []float64{0.5, 0.9, 0.99}).Update(float64(v))
}
func (sc *searchContext) checkSearchDeadlineAndPace() error {
	return checkSearchDeadlineAndPace(sc.deadline)
}

func (sc *searchContext) NewChild(fmt string, args ...interface{}) *searchContext {
	if sc == nil {
		return nil
	}

	child := getSearchContext(sc.deadline, sc.source, sc.Tracer.NewChild(fmt, args...))
	child.parent = sc
	return child
}

// Done finalizes the searchContext and propagates metrics to parent if any.
func (sc *searchContext) Done() {
	if sc == nil {
		return
	}

	if sc.Tracer != nil {
		sc.Tracer.Done()
	}

	sc.propagateMetricsToParent()
}

// propagateMetricsToParent propagates accumulated metrics from this context to its parent.
func (sc *searchContext) propagateMetricsToParent() {
	if sc.parent == nil {
		return
	}

	readMetrics := sc.readMetricIDs.Load()
	if readMetrics > 0 {
		sc.parent.readMetricIDs.Add(readMetrics)
	}
}
