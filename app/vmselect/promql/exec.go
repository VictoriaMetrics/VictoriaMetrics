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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	maxResponseSeries = flag.Int("search.maxResponseSeries", 0, "The maximum number of time series which can be returned from /api/v1/query and /api/v1/query_range . "+
		"The limit is disabled if it equals to 0. See also -search.maxPointsPerTimeseries and -search.maxUniqueTimeseries")
	treatDotsAsIsInRegexps = flag.Bool("search.treatDotsAsIsInRegexps", false, "Whether to treat dots as is in regexp label filters used in queries. "+
		`For example, foo{bar=~"a.b.c"} will be automatically converted to foo{bar=~"a\\.b\\.c"}, i.e. all the dots in regexp filters will be automatically escaped `+
		`in order to match only dot char instead of matching any char. Dots in ".+", ".*" and ".{n}" regexps aren't escaped. `+
		`This option is DEPRECATED in favor of {__graphite__="a.*.c"} syntax for selecting metrics matching the given Graphite metrics filter`)
	disableImplicitConversion = flag.Bool("search.disableImplicitConversion", false, "Whether to return an error for queries that rely on implicit subquery conversions, "+
		"see https://docs.victoriametrics.com/metricsql/#subqueries for details. "+
		"See also -search.logImplicitConversion.")
	logImplicitConversion = flag.Bool("search.logImplicitConversion", false, "Whether to log queries with implicit subquery conversions, "+
		"see https://docs.victoriametrics.com/metricsql/#subqueries for details. "+
		"Such conversion can be disabled using -search.disableImplicitConversion.")
)

// Exec executes q for the given ec.
func Exec(qt *querytracer.Tracer, ec *EvalConfig, q string, isFirstPointOnly bool) ([]netstorage.Result, error) {
	if querystats.Enabled() {
		startTime := time.Now()
		defer func() {
			querystats.RegisterQuery(q, ec.End-ec.Start, startTime)
			ec.QueryStats.addExecutionTimeMsec(startTime)
		}()
	}

	ec.validate()

	e, err := parsePromQLWithCache(q)
	if err != nil {
		return nil, err
	}

	if *disableImplicitConversion || *logImplicitConversion {
		isInvalid := metricsql.IsLikelyInvalid(e)
		if isInvalid && *disableImplicitConversion {
			// we don't add query=%q to err message as it will be added by the caller
			return nil, fmt.Errorf("query requires implicit conversion and is rejected according to -search.disableImplicitConversion command-line flag. " +
				"See https://docs.victoriametrics.com/metricsql/#implicit-query-conversions for details")
		}
		if isInvalid && *logImplicitConversion {
			logger.Warnf("query=%q requires implicit conversion, see https://docs.victoriametrics.com/metricsql/#implicit-query-conversions for details", e.AppendString(nil))
		}
	}

	qid := activeQueriesV.Add(ec, q)
	rv, err := evalExpr(qt, ec, e)
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
		qt.Printf("leave only the first point in every series")
	}
	maySort := maySortResults(e)
	result, err := timeseriesToResult(rv, maySort)
	if *maxResponseSeries > 0 && len(result) > *maxResponseSeries {
		return nil, fmt.Errorf("the response contains more than -search.maxResponseSeries=%d time series: %d series; either increase -search.maxResponseSeries "+
			"or change the query in order to return smaller number of series", *maxResponseSeries, len(result))
	}
	if err != nil {
		return nil, err
	}
	if maySort {
		qt.Printf("sort series by metric name and labels")
	} else {
		qt.Printf("do not sort series by metric name and labels")
	}
	if n := ec.RoundDigits; n < 100 {
		for i := range result {
			values := result[i].Values
			for j, v := range values {
				values[j] = decimal.RoundToDecimalDigits(v, n)
			}
		}
		qt.Printf("round series values to %d decimal digits after the point", n)
	}
	return result, nil
}

func maySortResults(e metricsql.Expr) bool {
	switch v := e.(type) {
	case *metricsql.FuncExpr:
		switch strings.ToLower(v.Name) {
		case "sort", "sort_desc",
			"sort_by_label", "sort_by_label_desc",
			"sort_by_label_numeric", "sort_by_label_numeric_desc":
			// Results already sorted
			return false
		}
	case *metricsql.AggrFuncExpr:
		switch strings.ToLower(v.Name) {
		case "topk", "bottomk", "outliersk",
			"topk_max", "topk_min", "topk_avg", "topk_median", "topk_last",
			"bottomk_max", "bottomk_min", "bottomk_avg", "bottomk_median", "bottomk_last":
			// Results already sorted
			return false
		}
	case *metricsql.BinaryOpExpr:
		if strings.EqualFold(v.Op, "or") {
			// Do not sort results for `a or b` in the same way as Prometheus does.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4763
			return false
		}
	}
	return true
}

func timeseriesToResult(tss []*timeseries, maySort bool) ([]netstorage.Result, error) {
	tss = removeEmptySeries(tss)
	if maySort {
		sortSeriesByMetricName(tss)
	}

	result := make([]netstorage.Result, len(tss))
	m := make(map[string]struct{}, len(tss))
	bb := bbPool.Get()
	for i, ts := range tss {
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		k := string(bb.B)
		if _, ok := m[k]; ok {
			return nil, fmt.Errorf(`duplicate output timeseries: %s`, stringMetricName(&ts.MetricName))
		}
		m[k] = struct{}{}

		rs := &result[i]
		rs.MetricName.MoveFrom(&ts.MetricName)
		rs.Values = ts.Values
		ts.Values = nil
		rs.Timestamps = ts.Timestamps
		ts.Timestamps = nil
	}
	bbPool.Put(bb)

	return result, nil
}

func sortSeriesByMetricName(tss []*timeseries) {
	sort.Slice(tss, func(i, j int) bool {
		return metricNameLess(&tss[i].MetricName, &tss[j].MetricName)
	})
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

func removeEmptySeries(tss []*timeseries) []*timeseries {
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
		for _, lfs := range me.LabelFilterss {
			for i := range lfs {
				f := &lfs[i]
				if f.IsRegexp {
					f.Value = escapeDots(f.Value)
				}
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
	requests atomic.Uint64
	misses   atomic.Uint64

	m  map[string]*parseCacheValue
	mu sync.RWMutex
}

func (pc *parseCache) Requests() uint64 {
	return pc.requests.Load()
}

func (pc *parseCache) Misses() uint64 {
	return pc.misses.Load()
}

func (pc *parseCache) Len() uint64 {
	pc.mu.RLock()
	n := len(pc.m)
	pc.mu.RUnlock()
	return uint64(n)
}

func (pc *parseCache) Get(q string) *parseCacheValue {
	pc.requests.Add(1)

	pc.mu.RLock()
	pcv := pc.m[q]
	pc.mu.RUnlock()

	if pcv == nil {
		pc.misses.Add(1)
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
