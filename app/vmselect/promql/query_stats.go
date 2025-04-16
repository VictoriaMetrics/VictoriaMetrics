package promql

import (
	"flag"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
)

var logQueryStatsDuration = flag.Duration("search.logSlowQueryStats", 0, "Log query statistics if execution time exceeding this value - see https://docs.victoriametrics.com/query-stats. Zero disables slow query statistics logging. This flag is available only in VictoriaMetrics enterprise. See https://docs.victoriametrics.com/enterprise/")

// QueryStats contains various stats of the query evaluation.
type QueryStats struct {
	// ExecutionDuration contains the time duration the query took to execute.
	ExecutionDuration atomic.Pointer[time.Duration]
	// SeriesFetched contains the number of series fetched from storage or cache.
	SeriesFetched atomic.Int64

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
