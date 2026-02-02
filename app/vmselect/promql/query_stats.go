package promql

import (
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
)

// QueryStats contains various stats of the query evaluation.
type QueryStats struct {
	// ExecutionDuration contains the time duration the query took to execute.
	ExecutionDuration atomic.Pointer[time.Duration]
	// SeriesFetched contains the number of series fetched from storage or cache.
	SeriesFetched atomic.Int64
	// MemoryUsage contains the estimated memory consumption of the query
	MemoryUsage atomic.Int64

	at *auth.Token

	query     string
	queryType string
	start     int64
	end       int64
	step      int64
}

// NewQueryStats creates a new QueryStats object.
func NewQueryStats(query string, at *auth.Token, ec *EvalConfig) *QueryStats {
	qs := &QueryStats{
		at:        at,
		query:     query,
		step:      ec.Step,
		start:     ec.Start,
		end:       ec.End,
		queryType: "range",
	}
	if qs.start == qs.end {
		qs.queryType = "instant"
	}
	return qs
}

func (qs *QueryStats) addSeriesFetched(n int) {
	if qs == nil {
		return
	}
	qs.SeriesFetched.Add(int64(n))
}

func (qs *QueryStats) addExecutionTimeMsec(startTime time.Time) {
	if qs == nil {
		return
	}
	d := time.Since(startTime)
	qs.ExecutionDuration.Store(&d)
}

func (qs *QueryStats) addMemoryUsage(memoryUsage int64) {
	if qs == nil {
		return
	}
	qs.MemoryUsage.Store(memoryUsage)
}

func (qs *QueryStats) memoryUsage() int64 {
	if qs == nil {
		return 0
	}
	return qs.MemoryUsage.Load()
}
