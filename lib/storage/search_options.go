package storage

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// searchOptions contains common options for Search.
type searchOptions struct {
	deadline uint64

	readMetricIDs atomic.Uint64
	source        string
}

// getSearchOptions returns new searchOptions.
// Note that the readMetricIDs metric is only flushed when the searchOptions is put back with putSearchOptions.
func getSearchOptions(deadline uint64, source string) *searchOptions {
	return &searchOptions{
		deadline: deadline,
		source:   source,

		readMetricIDs: atomic.Uint64{},
	}
}

// trackReadMetricIDs increments the readMetricIDs metric for the searchOptions.
func (so *searchOptions) trackReadMetricIDs(v uint64) {
	so.readMetricIDs.Add(v)
}

// putSearchOptions Flushes the readMetricIDs to metric and puts the searchOptions back to the pool.
func putSearchOptions(so *searchOptions) {
	summaryName := fmt.Sprintf(`vm_series_read_per_query{path=%q}`, so.source)

	metrics.GetOrCreateSummaryExt(summaryName, 1*time.Minute, []float64{0.5, 0.9, 0.99}).Update(float64(so.readMetricIDs.Load()))
}
