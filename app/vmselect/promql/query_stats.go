package promql

import (
	"flag"
	"hash/fnv"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// QueryStats contains various stats of the query evaluation.
type QueryStats struct {
	// ExecutionDuration contains the time duration the query took to execute.
	ExecutionDuration atomic.Pointer[time.Duration]
	// DataFetchDuration contains the time spent fetching data from storage via ProcessSearchQuery.
	DataFetchDuration atomic.Int64
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

func (qs *QueryStats) addDataFetchDuration(d time.Duration) {
	if qs == nil {
		return
	}
	qs.DataFetchDuration.Add(d.Nanoseconds())
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

func (qs *QueryStats) getDataFetchDuration() time.Duration {
	if qs == nil {
		return 0
	}
	return time.Duration(qs.DataFetchDuration.Load())
}

var logQueryStatsDuration = flag.Duration("search.logSlowQueryStats", 5*time.Second, "Log query statistics if execution time exceeding this value - see https://docs.victoriametrics.com/victoriametrics/query-stats/ . Zero disables slow query statistics logging. See https://docs.victoriametrics.com/victoriametrics/enterprise/")

func (qs *QueryStats) maybeLogQueryStats(startTime time.Time) {
	if qs == nil {
		return
	}
	if *logQueryStatsDuration == 0 {
		return
	}
	d := time.Since(startTime)
	if d < *logQueryStatsDuration {
		return
	}
	executionDurationMs := d.Milliseconds()
	dataFetchDurationMs := qs.getDataFetchDuration().Milliseconds()
	// Ensure dataFetchDurationMs does not exceed executionDurationMs due to clock skew or concurrent updates
	if dataFetchDurationMs > executionDurationMs {
		dataFetchDurationMs = executionDurationMs
	}
	queryHash := hashQuery(qs.query)
	tenant := "0"
	if qs.at != nil {
		tenant = qs.at.String()
	}
	rangeMs := qs.end - qs.start
	// Use Info level to match query-stats logging; vm_slow_query_stats prefix is required for filtering
	logger.Infof("vm_slow_query_stats type=%s query=%q query_hash=%d start_ms=%d end_ms=%d step_ms=%d range_ms=%d tenant=%q execution_duration_ms=%d data_fetch_duration_ms=%d series_fetched=%d memory_estimated_bytes=%d",
		qs.queryType, qs.query, queryHash, qs.start, qs.end, qs.step, rangeMs, tenant, executionDurationMs, dataFetchDurationMs, qs.SeriesFetched.Load(), qs.MemoryUsage.Load())
}

func hashQuery(q string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(q))
	return h.Sum64()
}
