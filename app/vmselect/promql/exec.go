package promql

import (
	"flag"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var logSlowQueryDuration = flag.Duration("search.logSlowQueryDuration", 5*time.Second, "Log queries with execution time exceeding this value. Zero disables slow query logging")

var slowQueries = metrics.NewCounter(`vm_slow_queries_total`)

// Exec executes q for the given ec.
func Exec(ec *EvalConfig, q string, isFirstPointOnly bool) ([]netstorage.Result, error) {
	if *logSlowQueryDuration > 0 {
		startTime := time.Now()
		defer func() {
			d := time.Since(startTime)
			if d >= *logSlowQueryDuration {
				logger.Infof("slow query according to -search.logSlowQueryDuration=%s: duration=%.3f seconds, start=%d, end=%d, step=%d, query=%q",
					*logSlowQueryDuration, d.Seconds(), ec.Start/1000, ec.End/1000, ec.Step/1000, q)
				slowQueries.Inc()
			}
		}()
	}

	ec.validate()

	e, err := parsePromQLWithCache(q)
	if err != nil {
		return nil, err
	}

	rv, err := evalExpr(ec, e)
	if err != nil {
		return nil, err
	}

	if isFirstPointOnly {
		// Remove all the points except the first one from every time series.
		for _, ts := range rv {
			ts.Values = ts.Values[:1]
			ts.Timestamps = ts.Timestamps[:1]
		}
	}

	maySort := maySortResults(e, rv)
	result, err := timeseriesToResult(rv, maySort)
	if err != nil {
		return nil, err
	}
	return result, err
}

func maySortResults(e metricsql.Expr, tss []*timeseries) bool {
	if len(tss) > 100 {
		// There is no sense in sorting a lot of results
		return false
	}
	fe, ok := e.(*metricsql.FuncExpr)
	if !ok {
		return true
	}
	switch fe.Name {
	case "sort", "sort_desc",
		"sort_by_label", "sort_by_label_desc":
		return false
	default:
		return true
	}
}

func timeseriesToResult(tss []*timeseries, maySort bool) ([]netstorage.Result, error) {
	tss = removeNaNs(tss)
	result := make([]netstorage.Result, len(tss))
	m := make(map[string]struct{}, len(tss))
	bb := bbPool.Get()
	for i, ts := range tss {
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		if _, ok := m[string(bb.B)]; ok {
			return nil, fmt.Errorf(`duplicate output timeseries: %s`, stringMetricName(&ts.MetricName))
		}
		m[string(bb.B)] = struct{}{}

		rs := &result[i]
		rs.MetricNameMarshaled = append(rs.MetricNameMarshaled[:0], bb.B...)
		rs.MetricName.CopyFrom(&ts.MetricName)
		rs.Values = append(rs.Values[:0], ts.Values...)
		rs.Timestamps = append(rs.Timestamps[:0], ts.Timestamps...)
	}
	bbPool.Put(bb)

	if maySort {
		sort.Slice(result, func(i, j int) bool {
			return string(result[i].MetricNameMarshaled) < string(result[j].MetricNameMarshaled)
		})
	}

	return result, nil
}

func removeNaNs(tss []*timeseries) []*timeseries {
	rvs := tss[:0]
	for _, ts := range tss {
		allNans := true
		for _, v := range ts.Values {
			if !math.IsNaN(v) {
				allNans = false
				break
			}
		}
		if allNans {
			// Skip timeseries with all NaNs.
			continue
		}
		rvs = append(rvs, ts)
	}
	for i := len(rvs); i < len(tss); i++ {
		// Zero unused time series, so GC could reclaim them.
		tss[i] = nil
	}
	return rvs
}

func parsePromQLWithCache(q string) (metricsql.Expr, error) {
	pcv := parseCacheV.Get(q)
	if pcv == nil {
		e, err := metricsql.Parse(q)
		pcv = &parseCacheValue{
			e:   e,
			err: err,
		}
		parseCacheV.Put(q, pcv)
	}
	if pcv.err != nil {
		return nil, pcv.err
	}
	return pcv.e, nil
}

var parseCacheV = func() *parseCache {
	pc := &parseCache{
		m: make(map[string]*parseCacheValue),
	}
	metrics.NewGauge(`vm_cache_requests_total{type="promql/parse"}`, func() float64 {
		return float64(pc.Requests())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/parse"}`, func() float64 {
		return float64(pc.Misses())
	})
	metrics.NewGauge(`vm_cache_entries{type="promql/parse"}`, func() float64 {
		return float64(pc.Len())
	})
	return pc
}()

const parseCacheMaxLen = 10e3

type parseCacheValue struct {
	e   metricsql.Expr
	err error
}

type parseCache struct {
	// Move atomic counters to the top of struct for 8-byte alignment on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	requests uint64
	misses   uint64

	m  map[string]*parseCacheValue
	mu sync.RWMutex
}

func (pc *parseCache) Requests() uint64 {
	return atomic.LoadUint64(&pc.requests)
}

func (pc *parseCache) Misses() uint64 {
	return atomic.LoadUint64(&pc.misses)
}

func (pc *parseCache) Len() uint64 {
	pc.mu.RLock()
	n := len(pc.m)
	pc.mu.RUnlock()
	return uint64(n)
}

func (pc *parseCache) Get(q string) *parseCacheValue {
	atomic.AddUint64(&pc.requests, 1)

	pc.mu.RLock()
	pcv := pc.m[q]
	pc.mu.RUnlock()

	if pcv == nil {
		atomic.AddUint64(&pc.misses, 1)
	}
	return pcv
}

func (pc *parseCache) Put(q string, pcv *parseCacheValue) {
	pc.mu.Lock()
	overflow := len(pc.m) - parseCacheMaxLen
	if overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(pc.m)) * 0.1)
		for k := range pc.m {
			delete(pc.m, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}
	pc.m[q] = pcv
	pc.mu.Unlock()
}
