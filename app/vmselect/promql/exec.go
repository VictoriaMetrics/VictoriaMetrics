package promql

import (
	"flag"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/querystats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	treatDotsAsIsInRegexps = flag.Bool("search.treatDotsAsIsInRegexps", false, "Whether to treat dots as is in regexp label filters used in queries. "+
		`For example, foo{bar=~"a.b.c"} will be automatically converted to foo{bar=~"a\\.b\\.c"}, i.e. all the dots in regexp filters will be automatically escaped `+
		`in order to match only dot char instead of matching any char. Dots in ".+", ".*" and ".{n}" regexps aren't escaped. `+
		`This option is DEPRECATED in favor of {__graphite__="a.*.c"} syntax for selecting metrics matching the given Graphite metrics filter`)
)

// Exec executes q for the given ec.
func Exec(ec *EvalConfig, q string, isFirstPointOnly bool) ([]netstorage.Result, error) {
	if querystats.Enabled() {
		startTime := time.Now()
		ac := ec.AuthToken
		defer querystats.RegisterQuery(ac.AccountID, ac.ProjectID, q, ec.End-ec.Start, startTime)
	}

	ec.validate()

	e, err := parsePromQLWithCache(q)
	if err != nil {
		return nil, err
	}

	qid := activeQueriesV.Add(ec, q)
	rv, err := evalExpr(ec, e)
	activeQueriesV.Remove(qid)
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
	if n := ec.RoundDigits; n < 100 {
		for i := range result {
			values := result[i].Values
			for j, v := range values {
				values[j] = decimal.RoundToDecimalDigits(v, n)
			}
		}
	}
	return result, err
}

func maySortResults(e metricsql.Expr, tss []*timeseries) bool {
	switch v := e.(type) {
	case *metricsql.FuncExpr:
		switch strings.ToLower(v.Name) {
		case "sort", "sort_desc",
			"sort_by_label", "sort_by_label_desc":
			return false
		}
	case *metricsql.AggrFuncExpr:
		switch strings.ToLower(v.Name) {
		case "topk", "bottomk", "outliersk",
			"topk_max", "topk_min", "topk_avg", "topk_median", "topk_last",
			"bottomk_max", "bottomk_min", "bottomk_avg", "bottomk_median", "bottomk_last":
			return false
		}
	}
	return true
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
		rs.MetricName.CopyFrom(&ts.MetricName)
		rs.Values = append(rs.Values[:0], ts.Values...)
		rs.Timestamps = append(rs.Timestamps[:0], ts.Timestamps...)
	}
	bbPool.Put(bb)

	if maySort {
		sort.Slice(result, func(i, j int) bool {
			return metricNameLess(&result[i].MetricName, &result[j].MetricName)
		})
	}

	return result, nil
}

func metricNameLess(a, b *storage.MetricName) bool {
	if string(a.MetricGroup) != string(b.MetricGroup) {
		return string(a.MetricGroup) < string(b.MetricGroup)
	}
	// Metric names for a and b match. Compare tags.
	// Tags must be already sorted by the caller, so just compare them.
	ats := a.Tags
	bts := b.Tags
	for i := range ats {
		if i >= len(bts) {
			// a contains more tags than b and all the previous tags were identical,
			// so a is considered bigger than b.
			return false
		}
		at := &ats[i]
		bt := &bts[i]
		if string(at.Key) != string(bt.Key) {
			return string(at.Key) < string(bt.Key)
		}
		if string(at.Value) != string(bt.Value) {
			return string(at.Value) < string(bt.Value)
		}
	}
	return len(ats) < len(bts)
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

func adjustCmpOps(e metricsql.Expr) metricsql.Expr {
	metricsql.VisitAll(e, func(expr metricsql.Expr) {
		be, ok := expr.(*metricsql.BinaryOpExpr)
		if !ok {
			return
		}
		if !metricsql.IsBinaryOpCmp(be.Op) {
			return
		}
		if isNumberExpr(be.Right) || !isScalarExpr(be.Left) {
			return
		}
		// Convert 'num cmpOp query' expression to `query reverseCmpOp num` expression
		// like Prometheus does. For instance, `0.5 < foo` must be converted to `foo > 0.5`
		// in order to return valid values for `foo` that are bigger than 0.5.
		be.Right, be.Left = be.Left, be.Right
		be.Op = getReverseCmpOp(be.Op)
	})
	return e
}

func isNumberExpr(e metricsql.Expr) bool {
	_, ok := e.(*metricsql.NumberExpr)
	return ok
}

func isScalarExpr(e metricsql.Expr) bool {
	if isNumberExpr(e) {
		return true
	}
	if fe, ok := e.(*metricsql.FuncExpr); ok {
		// time() returns scalar in PromQL - see https://prometheus.io/docs/prometheus/latest/querying/functions/#time
		return strings.ToLower(fe.Name) == "time"
	}
	return false
}

func getReverseCmpOp(op string) string {
	switch op {
	case ">":
		return "<"
	case "<":
		return ">"
	case ">=":
		return "<="
	case "<=":
		return ">="
	default:
		// there is no need in changing `==` and `!=`.
		return op
	}
}

func parsePromQLWithCache(q string) (metricsql.Expr, error) {
	pcv := parseCacheV.Get(q)
	if pcv == nil {
		e, err := metricsql.Parse(q)
		if err == nil {
			e = metricsql.Optimize(e)
			e = adjustCmpOps(e)
			if *treatDotsAsIsInRegexps {
				e = escapeDotsInRegexpLabelFilters(e)
			}
		}
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

func escapeDotsInRegexpLabelFilters(e metricsql.Expr) metricsql.Expr {
	metricsql.VisitAll(e, func(expr metricsql.Expr) {
		me, ok := expr.(*metricsql.MetricExpr)
		if !ok {
			return
		}
		for i := range me.LabelFilters {
			f := &me.LabelFilters[i]
			if f.IsRegexp {
				f.Value = escapeDots(f.Value)
			}
		}
	})
	return e
}

func escapeDots(s string) string {
	dotsCount := strings.Count(s, ".")
	if dotsCount <= 0 {
		return s
	}
	result := make([]byte, 0, len(s)+2*dotsCount)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && (i == 0 || s[i-1] != '\\') && (i+1 == len(s) || i+1 < len(s) && s[i+1] != '*' && s[i+1] != '+' && s[i+1] != '{') {
			// Escape a dot if the following conditions are met:
			// - if it isn't escaped already, i.e. if there is no `\` char before the dot.
			// - if there is no regexp modifiers such as '+', '*' or '{' after the dot.
			result = append(result, '\\', '.')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
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
