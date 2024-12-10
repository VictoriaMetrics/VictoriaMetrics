package storage

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// searchOptions contains common options for Search.
type searchOptions struct {
	deadline uint64

	readMetricIDs *atomic.Uint64
	queryType     string
}

// getSearchOptions returns new searchOptions.
// Note that the readMetricIDs metric is only flushed when the searchOptions is put back with putSearchOptions.
func getSearchOptions(deadline uint64, queryType string) *searchOptions {
	if v := searchOptionsPool.Get(); v != nil {
		so := v.(*searchOptions)
		so.deadline = deadline
		so.queryType = queryType
		return so
	}

	return &searchOptions{
		deadline:  deadline,
		queryType: queryType,

		readMetricIDs: &atomic.Uint64{},
	}
}

// trackReadMetricIDs increments the readMetricIDs metric for the searchOptions.
func (so *searchOptions) trackReadMetricIDs(v uint64) {
	so.readMetricIDs.Add(v)
}

// putSearchOptions Flushes the readMetricIDs to metric and puts the searchOptions back to the pool.
func putSearchOptions(so *searchOptions) {
	metrics.GetOrCreateHistogram(fmt.Sprintf(`vm_series_read_per_query{type=%q}`, so.queryType)).Update(float64(so.readMetricIDs.Load()))

	so.readMetricIDs.Store(0)
	searchOptionsPool.Put(so)
}

var searchOptionsPool = sync.Pool{}
