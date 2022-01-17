package promql

import (
	"flag"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	disableCache           = flag.Bool("search.disableCache", false, "Whether to disable response caching. This may be useful during data backfilling")
	maxPointsPerTimeseries = flag.Int("search.maxPointsPerTimeseries", 30e3, "The maximum points per a single timeseries returned from /api/v1/query_range. "+
		"This option doesn't limit the number of scanned raw samples in the database. The main purpose of this option is to limit the number of per-series points "+
		"returned to graphing UI such as Grafana. There is no sense in setting this limit to values bigger than the horizontal resolution of the graph")
	noStaleMarkers = flag.Bool("search.noStaleMarkers", false, "Set this flag to true if the database doesn't contain Prometheus stale markers, so there is no need in spending additional CPU time on its handling. Staleness markers may exist only in data obtained from Prometheus scrape targets")
)

// The minimum number of points per timeseries for enabling time rounding.
// This improves cache hit ratio for frequently requested queries over
// big time ranges.
const minTimeseriesPointsForTimeRounding = 50

// ValidateMaxPointsPerTimeseries checks the maximum number of points that
// may be returned per each time series.
//
// The number mustn't exceed -search.maxPointsPerTimeseries.
func ValidateMaxPointsPerTimeseries(start, end, step int64) error {
	points := (end-start)/step + 1
	if uint64(points) > uint64(*maxPointsPerTimeseries) {
		return fmt.Errorf(`too many points for the given step=%d, start=%d and end=%d: %d; cannot exceed -search.maxPointsPerTimeseries=%d`,
			step, start, end, uint64(points), *maxPointsPerTimeseries)
	}
	return nil
}

// AdjustStartEnd adjusts start and end values, so response caching may be enabled.
//
// See EvalConfig.mayCache for details.
func AdjustStartEnd(start, end, step int64) (int64, int64) {
	if *disableCache {
		// Do not adjust start and end values when cache is disabled.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/563
		return start, end
	}
	points := (end-start)/step + 1
	if points < minTimeseriesPointsForTimeRounding {
		// Too small number of points for rounding.
		return start, end
	}

	// Round start and end to values divisible by step in order
	// to enable response caching (see EvalConfig.mayCache).
	start, end = alignStartEnd(start, end, step)

	// Make sure that the new number of points is the same as the initial number of points.
	newPoints := (end-start)/step + 1
	for newPoints > points {
		end -= step
		newPoints--
	}

	return start, end
}

func alignStartEnd(start, end, step int64) (int64, int64) {
	// Round start to the nearest smaller value divisible by step.
	start -= start % step
	// Round end to the nearest bigger value divisible by step.
	adjust := end % step
	if adjust > 0 {
		end += step - adjust
	}
	return start, end
}

// EvalConfig is the configuration required for query evaluation via Exec
type EvalConfig struct {
	AuthToken *auth.Token
	Start     int64
	End       int64
	Step      int64

	// QuotedRemoteAddr contains quoted remote address.
	QuotedRemoteAddr string

	Deadline searchutils.Deadline

	MayCache bool

	// LookbackDelta is analog to `-query.lookback-delta` from Prometheus.
	LookbackDelta int64

	// How many decimal digits after the point to leave in response.
	RoundDigits int

	// EnforcedTagFilterss may contain additional label filters to use in the query.
	EnforcedTagFilterss [][]storage.TagFilter

	// Whether to deny partial response.
	DenyPartialResponse bool

	// IsPartialResponse is set during query execution and can be used by Exec caller after query execution.
	IsPartialResponse bool

	timestamps     []int64
	timestampsOnce sync.Once
}

// newEvalConfig returns new EvalConfig copy from src.
func newEvalConfig(src *EvalConfig) *EvalConfig {
	var ec EvalConfig
	ec.AuthToken = src.AuthToken
	ec.Start = src.Start
	ec.End = src.End
	ec.Step = src.Step
	ec.Deadline = src.Deadline
	ec.MayCache = src.MayCache
	ec.LookbackDelta = src.LookbackDelta
	ec.RoundDigits = src.RoundDigits
	ec.EnforcedTagFilterss = src.EnforcedTagFilterss
	ec.DenyPartialResponse = src.DenyPartialResponse
	ec.IsPartialResponse = src.IsPartialResponse

	// do not copy src.timestamps - they must be generated again.
	return &ec
}

func (ec *EvalConfig) updateIsPartialResponse(isPartialResponse bool) {
	if !ec.IsPartialResponse {
		ec.IsPartialResponse = isPartialResponse
	}
}

func (ec *EvalConfig) validate() {
	if ec.Start > ec.End {
		logger.Panicf("BUG: start cannot exceed end; got %d vs %d", ec.Start, ec.End)
	}
	if ec.Step <= 0 {
		logger.Panicf("BUG: step must be greater than 0; got %d", ec.Step)
	}
}

func (ec *EvalConfig) mayCache() bool {
	if *disableCache {
		return false
	}
	if !ec.MayCache {
		return false
	}
	if ec.Start%ec.Step != 0 {
		return false
	}
	if ec.End%ec.Step != 0 {
		return false
	}
	return true
}

func (ec *EvalConfig) getSharedTimestamps() []int64 {
	ec.timestampsOnce.Do(ec.timestampsInit)
	return ec.timestamps
}

func (ec *EvalConfig) timestampsInit() {
	ec.timestamps = getTimestamps(ec.Start, ec.End, ec.Step)
}

func getTimestamps(start, end, step int64) []int64 {
	// Sanity checks.
	if step <= 0 {
		logger.Panicf("BUG: Step must be bigger than 0; got %d", step)
	}
	if start > end {
		logger.Panicf("BUG: Start cannot exceed End; got %d vs %d", start, end)
	}
	if err := ValidateMaxPointsPerTimeseries(start, end, step); err != nil {
		logger.Panicf("BUG: %s; this must be validated before the call to getTimestamps", err)
	}

	// Prepare timestamps.
	points := 1 + (end-start)/step
	timestamps := make([]int64, points)
	for i := range timestamps {
		timestamps[i] = start
		start += step
	}
	return timestamps
}

func evalExpr(ec *EvalConfig, e metricsql.Expr) ([]*timeseries, error) {
	if me, ok := e.(*metricsql.MetricExpr); ok {
		re := &metricsql.RollupExpr{
			Expr: me,
		}
		rv, err := evalRollupFunc(ec, "default_rollup", rollupDefault, e, re, nil)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %w`, me.AppendString(nil), err)
		}
		return rv, nil
	}
	if re, ok := e.(*metricsql.RollupExpr); ok {
		rv, err := evalRollupFunc(ec, "default_rollup", rollupDefault, e, re, nil)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %w`, re.AppendString(nil), err)
		}
		return rv, nil
	}
	if fe, ok := e.(*metricsql.FuncExpr); ok {
		nrf := getRollupFunc(fe.Name)
		if nrf == nil {
			args, err := evalExprs(ec, fe.Args)
			if err != nil {
				return nil, err
			}
			tf := getTransformFunc(fe.Name)
			if tf == nil {
				return nil, fmt.Errorf(`unknown func %q`, fe.Name)
			}
			tfa := &transformFuncArg{
				ec:   ec,
				fe:   fe,
				args: args,
			}
			rv, err := tf(tfa)
			if err != nil {
				return nil, fmt.Errorf(`cannot evaluate %q: %w`, fe.AppendString(nil), err)
			}
			return rv, nil
		}
		args, re, err := evalRollupFuncArgs(ec, fe)
		if err != nil {
			return nil, err
		}
		rf, err := nrf(args)
		if err != nil {
			return nil, err
		}
		rv, err := evalRollupFunc(ec, fe.Name, rf, e, re, nil)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %w`, fe.AppendString(nil), err)
		}
		return rv, nil
	}
	if ae, ok := e.(*metricsql.AggrFuncExpr); ok {
		if callbacks := getIncrementalAggrFuncCallbacks(ae.Name); callbacks != nil {
			fe, nrf := tryGetArgRollupFuncWithMetricExpr(ae)
			if fe != nil {
				// There is an optimized path for calculating metricsql.AggrFuncExpr over rollupFunc over metricsql.MetricExpr.
				// The optimized path saves RAM for aggregates over big number of time series.
				args, re, err := evalRollupFuncArgs(ec, fe)
				if err != nil {
					return nil, err
				}
				rf, err := nrf(args)
				if err != nil {
					return nil, err
				}
				iafc := newIncrementalAggrFuncContext(ae, callbacks)
				return evalRollupFunc(ec, fe.Name, rf, e, re, iafc)
			}
		}
		args, err := evalExprs(ec, ae.Args)
		if err != nil {
			return nil, err
		}
		af := getAggrFunc(ae.Name)
		if af == nil {
			return nil, fmt.Errorf(`unknown func %q`, ae.Name)
		}
		afa := &aggrFuncArg{
			ae:   ae,
			args: args,
			ec:   ec,
		}
		rv, err := af(afa)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %w`, ae.AppendString(nil), err)
		}
		return rv, nil
	}
	if be, ok := e.(*metricsql.BinaryOpExpr); ok {
		// Execute left and right sides of the binary operation in parallel.
		// This should reduce execution times for heavy queries.
		// On the other side this can increase CPU and RAM usage when executing heavy queries.
		// TODO: think on how to limit CPU and RAM usage while leaving short execution times.
		var left, right []*timeseries
		var mu sync.Mutex
		var wg sync.WaitGroup
		var errGlobal error
		wg.Add(1)
		go func() {
			defer wg.Done()
			ecCopy := newEvalConfig(ec)
			tss, err := evalExpr(ecCopy, be.Left)
			mu.Lock()
			if err != nil {
				if errGlobal == nil {
					errGlobal = err
				}
			}
			left = tss
			mu.Unlock()
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			ecCopy := newEvalConfig(ec)
			tss, err := evalExpr(ecCopy, be.Right)
			mu.Lock()
			if err != nil {
				if errGlobal == nil {
					errGlobal = err
				}
			}
			right = tss
			mu.Unlock()
		}()
		wg.Wait()
		if errGlobal != nil {
			return nil, errGlobal
		}

		bf := getBinaryOpFunc(be.Op)
		if bf == nil {
			return nil, fmt.Errorf(`unknown binary op %q`, be.Op)
		}
		bfa := &binaryOpFuncArg{
			be:    be,
			left:  left,
			right: right,
		}
		rv, err := bf(bfa)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %w`, be.AppendString(nil), err)
		}
		return rv, nil
	}
	if ne, ok := e.(*metricsql.NumberExpr); ok {
		rv := evalNumber(ec, ne.N)
		return rv, nil
	}
	if se, ok := e.(*metricsql.StringExpr); ok {
		rv := evalString(ec, se.S)
		return rv, nil
	}
	if de, ok := e.(*metricsql.DurationExpr); ok {
		d := de.Duration(ec.Step)
		dSec := float64(d) / 1000
		rv := evalNumber(ec, dSec)
		return rv, nil
	}
	return nil, fmt.Errorf("unexpected expression %q", e.AppendString(nil))
}

func tryGetArgRollupFuncWithMetricExpr(ae *metricsql.AggrFuncExpr) (*metricsql.FuncExpr, newRollupFunc) {
	if len(ae.Args) != 1 {
		return nil, nil
	}
	e := ae.Args[0]
	// Make sure e contains one of the following:
	// - metricExpr
	// - metricExpr[d]
	// - rollupFunc(metricExpr)
	// - rollupFunc(metricExpr[d])

	if me, ok := e.(*metricsql.MetricExpr); ok {
		// e = metricExpr
		if me.IsEmpty() {
			return nil, nil
		}
		fe := &metricsql.FuncExpr{
			Name: "default_rollup",
			Args: []metricsql.Expr{me},
		}
		nrf := getRollupFunc(fe.Name)
		return fe, nrf
	}
	if re, ok := e.(*metricsql.RollupExpr); ok {
		if me, ok := re.Expr.(*metricsql.MetricExpr); !ok || me.IsEmpty() || re.ForSubquery() {
			return nil, nil
		}
		// e = metricExpr[d]
		fe := &metricsql.FuncExpr{
			Name: "default_rollup",
			Args: []metricsql.Expr{re},
		}
		nrf := getRollupFunc(fe.Name)
		return fe, nrf
	}
	fe, ok := e.(*metricsql.FuncExpr)
	if !ok {
		return nil, nil
	}
	nrf := getRollupFunc(fe.Name)
	if nrf == nil {
		return nil, nil
	}
	rollupArgIdx := metricsql.GetRollupArgIdx(fe)
	if rollupArgIdx >= len(fe.Args) {
		// Incorrect number of args for rollup func.
		return nil, nil
	}
	arg := fe.Args[rollupArgIdx]
	if me, ok := arg.(*metricsql.MetricExpr); ok {
		if me.IsEmpty() {
			return nil, nil
		}
		// e = rollupFunc(metricExpr)
		return &metricsql.FuncExpr{
			Name: fe.Name,
			Args: []metricsql.Expr{me},
		}, nrf
	}
	if re, ok := arg.(*metricsql.RollupExpr); ok {
		if me, ok := re.Expr.(*metricsql.MetricExpr); !ok || me.IsEmpty() || re.ForSubquery() {
			return nil, nil
		}
		// e = rollupFunc(metricExpr[d])
		return fe, nrf
	}
	return nil, nil
}

func evalExprs(ec *EvalConfig, es []metricsql.Expr) ([][]*timeseries, error) {
	var rvs [][]*timeseries
	for _, e := range es {
		rv, err := evalExpr(ec, e)
		if err != nil {
			return nil, err
		}
		rvs = append(rvs, rv)
	}
	return rvs, nil
}

func evalRollupFuncArgs(ec *EvalConfig, fe *metricsql.FuncExpr) ([]interface{}, *metricsql.RollupExpr, error) {
	var re *metricsql.RollupExpr
	rollupArgIdx := metricsql.GetRollupArgIdx(fe)
	if len(fe.Args) <= rollupArgIdx {
		return nil, nil, fmt.Errorf("expecting at least %d args to %q; got %d args; expr: %q", rollupArgIdx+1, fe.Name, len(fe.Args), fe.AppendString(nil))
	}
	args := make([]interface{}, len(fe.Args))
	for i, arg := range fe.Args {
		if i == rollupArgIdx {
			re = getRollupExprArg(arg)
			args[i] = re
			continue
		}
		ts, err := evalExpr(ec, arg)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot evaluate arg #%d for %q: %w", i+1, fe.AppendString(nil), err)
		}
		args[i] = ts
	}
	return args, re, nil
}

func getRollupExprArg(arg metricsql.Expr) *metricsql.RollupExpr {
	re, ok := arg.(*metricsql.RollupExpr)
	if !ok {
		// Wrap non-rollup arg into metricsql.RollupExpr.
		return &metricsql.RollupExpr{
			Expr: arg,
		}
	}
	if !re.ForSubquery() {
		// Return standard rollup if it doesn't contain subquery.
		return re
	}
	me, ok := re.Expr.(*metricsql.MetricExpr)
	if !ok {
		// arg contains subquery.
		return re
	}
	// Convert me[w:step] -> default_rollup(me)[w:step]
	reNew := *re
	reNew.Expr = &metricsql.FuncExpr{
		Name: "default_rollup",
		Args: []metricsql.Expr{
			&metricsql.RollupExpr{Expr: me},
		},
	}
	return &reNew
}

// expr may contain:
// - rollupFunc(m) if iafc is nil
// - aggrFunc(rollupFunc(m)) if iafc isn't nil
func evalRollupFunc(ec *EvalConfig, funcName string, rf rollupFunc, expr metricsql.Expr, re *metricsql.RollupExpr, iafc *incrementalAggrFuncContext) ([]*timeseries, error) {
	if re.At == nil {
		return evalRollupFuncWithoutAt(ec, funcName, rf, expr, re, iafc)
	}
	tssAt, err := evalExpr(ec, re.At)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate `@` modifier: %w", err)
	}
	if len(tssAt) != 1 {
		return nil, fmt.Errorf("`@` modifier must return a single series; it returns %d series instead", len(tssAt))
	}
	atTimestamp := int64(tssAt[0].Values[0] * 1000)
	ecNew := newEvalConfig(ec)
	ecNew.Start = atTimestamp
	ecNew.End = atTimestamp
	tss, err := evalRollupFuncWithoutAt(ecNew, funcName, rf, expr, re, iafc)
	if err != nil {
		return nil, err
	}
	// expand single-point tss to the original time range.
	timestamps := ec.getSharedTimestamps()
	for _, ts := range tss {
		v := ts.Values[0]
		values := make([]float64, len(timestamps))
		for i := range timestamps {
			values[i] = v
		}
		ts.Timestamps = timestamps
		ts.Values = values
	}
	return tss, nil
}

func evalRollupFuncWithoutAt(ec *EvalConfig, funcName string, rf rollupFunc, expr metricsql.Expr, re *metricsql.RollupExpr, iafc *incrementalAggrFuncContext) ([]*timeseries, error) {
	funcName = strings.ToLower(funcName)
	ecNew := ec
	var offset int64
	if re.Offset != nil {
		offset = re.Offset.Duration(ec.Step)
		ecNew = newEvalConfig(ecNew)
		ecNew.Start -= offset
		ecNew.End -= offset
		// There is no need in calling AdjustStartEnd() on ecNew if ecNew.MayCache is set to true,
		// since the time range alignment has been already performed by the caller,
		// so cache hit rate should be quite good.
		// See also https://github.com/VictoriaMetrics/VictoriaMetrics/issues/976
	}
	if funcName == "rollup_candlestick" {
		// Automatically apply `offset -step` to `rollup_candlestick` function
		// in order to obtain expected OHLC results.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/309#issuecomment-582113462
		step := ecNew.Step
		ecNew = newEvalConfig(ecNew)
		ecNew.Start += step
		ecNew.End += step
		offset -= step
	}
	var rvs []*timeseries
	var err error
	if me, ok := re.Expr.(*metricsql.MetricExpr); ok {
		rvs, err = evalRollupFuncWithMetricExpr(ecNew, funcName, rf, expr, me, iafc, re.Window)
	} else {
		if iafc != nil {
			logger.Panicf("BUG: iafc must be nil for rollup %q over subquery %q", funcName, re.AppendString(nil))
		}
		rvs, err = evalRollupFuncWithSubquery(ecNew, funcName, rf, expr, re)
	}
	if err != nil {
		return nil, err
	}
	ec.updateIsPartialResponse(ecNew.IsPartialResponse)
	if offset != 0 && len(rvs) > 0 {
		// Make a copy of timestamps, since they may be used in other values.
		srcTimestamps := rvs[0].Timestamps
		dstTimestamps := append([]int64{}, srcTimestamps...)
		for i := range dstTimestamps {
			dstTimestamps[i] += offset
		}
		for _, ts := range rvs {
			ts.Timestamps = dstTimestamps
		}
	}
	return rvs, nil
}

func evalRollupFuncWithSubquery(ec *EvalConfig, funcName string, rf rollupFunc, expr metricsql.Expr, re *metricsql.RollupExpr) ([]*timeseries, error) {
	// TODO: determine whether to use rollupResultCacheV here.
	step := re.Step.Duration(ec.Step)
	if step == 0 {
		step = ec.Step
	}
	window := re.Window.Duration(ec.Step)

	ecSQ := newEvalConfig(ec)
	ecSQ.Start -= window + maxSilenceInterval + step
	ecSQ.End += step
	ecSQ.Step = step
	if err := ValidateMaxPointsPerTimeseries(ecSQ.Start, ecSQ.End, ecSQ.Step); err != nil {
		return nil, err
	}
	// unconditionally align start and end args to step for subquery as Prometheus does.
	ecSQ.Start, ecSQ.End = alignStartEnd(ecSQ.Start, ecSQ.End, ecSQ.Step)
	tssSQ, err := evalExpr(ecSQ, re.Expr)
	if err != nil {
		return nil, err
	}
	ec.updateIsPartialResponse(ecSQ.IsPartialResponse)
	if len(tssSQ) == 0 {
		if funcName == "absent_over_time" {
			tss := evalNumber(ec, 1)
			return tss, nil
		}
		return nil, nil
	}
	sharedTimestamps := getTimestamps(ec.Start, ec.End, ec.Step)
	preFunc, rcs, err := getRollupConfigs(funcName, rf, expr, ec.Start, ec.End, ec.Step, window, ec.LookbackDelta, sharedTimestamps)
	if err != nil {
		return nil, err
	}
	tss := make([]*timeseries, 0, len(tssSQ)*len(rcs))
	var tssLock sync.Mutex
	keepMetricNames := getKeepMetricNames(expr)
	doParallel(tssSQ, func(tsSQ *timeseries, values []float64, timestamps []int64) ([]float64, []int64) {
		values, timestamps = removeNanValues(values[:0], timestamps[:0], tsSQ.Values, tsSQ.Timestamps)
		preFunc(values, timestamps)
		for _, rc := range rcs {
			if tsm := newTimeseriesMap(funcName, keepMetricNames, sharedTimestamps, &tsSQ.MetricName); tsm != nil {
				rc.DoTimeseriesMap(tsm, values, timestamps)
				tssLock.Lock()
				tss = tsm.AppendTimeseriesTo(tss)
				tssLock.Unlock()
				continue
			}
			var ts timeseries
			doRollupForTimeseries(funcName, keepMetricNames, rc, &ts, &tsSQ.MetricName, values, timestamps, sharedTimestamps)
			tssLock.Lock()
			tss = append(tss, &ts)
			tssLock.Unlock()
		}
		return values, timestamps
	})
	return tss, nil
}

func getKeepMetricNames(expr metricsql.Expr) bool {
	if ae, ok := expr.(*metricsql.AggrFuncExpr); ok {
		// Extract rollupFunc(...) from aggrFunc(rollupFunc(...)).
		// This case is possible when optimized aggrFunc calculations are used
		// such as `sum(rate(...))`
		if len(ae.Args) != 1 {
			return false
		}
		expr = ae.Args[0]
	}
	if fe, ok := expr.(*metricsql.FuncExpr); ok {
		return fe.KeepMetricNames
	}
	return false
}

func doParallel(tss []*timeseries, f func(ts *timeseries, values []float64, timestamps []int64) ([]float64, []int64)) {
	concurrency := cgroup.AvailableCPUs()
	if concurrency > len(tss) {
		concurrency = len(tss)
	}
	workCh := make(chan *timeseries, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			var tmpValues []float64
			var tmpTimestamps []int64
			for ts := range workCh {
				tmpValues, tmpTimestamps = f(ts, tmpValues, tmpTimestamps)
			}
		}()
	}
	for _, ts := range tss {
		workCh <- ts
	}
	close(workCh)
	wg.Wait()
}

func removeNanValues(dstValues []float64, dstTimestamps []int64, values []float64, timestamps []int64) ([]float64, []int64) {
	hasNan := false
	for _, v := range values {
		if math.IsNaN(v) {
			hasNan = true
		}
	}
	if !hasNan {
		// Fast path - no NaNs.
		dstValues = append(dstValues, values...)
		dstTimestamps = append(dstTimestamps, timestamps...)
		return dstValues, dstTimestamps
	}

	// Slow path - remove NaNs.
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		dstValues = append(dstValues, v)
		dstTimestamps = append(dstTimestamps, timestamps[i])
	}
	return dstValues, dstTimestamps
}

var (
	rollupResultCacheFullHits    = metrics.NewCounter(`vm_rollup_result_cache_full_hits_total`)
	rollupResultCachePartialHits = metrics.NewCounter(`vm_rollup_result_cache_partial_hits_total`)
	rollupResultCacheMiss        = metrics.NewCounter(`vm_rollup_result_cache_miss_total`)
)

func evalRollupFuncWithMetricExpr(ec *EvalConfig, funcName string, rf rollupFunc,
	expr metricsql.Expr, me *metricsql.MetricExpr, iafc *incrementalAggrFuncContext, windowExpr *metricsql.DurationExpr) ([]*timeseries, error) {
	if me.IsEmpty() {
		return evalNumber(ec, nan), nil
	}
	window := windowExpr.Duration(ec.Step)

	// Search for partial results in cache.
	tssCached, start := rollupResultCacheV.Get(ec, expr, window)
	if start > ec.End {
		// The result is fully cached.
		rollupResultCacheFullHits.Inc()
		return tssCached, nil
	}
	if start > ec.Start {
		rollupResultCachePartialHits.Inc()
	} else {
		rollupResultCacheMiss.Inc()
	}

	// Obtain rollup configs before fetching data from db,
	// so type errors can be caught earlier.
	sharedTimestamps := getTimestamps(start, ec.End, ec.Step)
	preFunc, rcs, err := getRollupConfigs(funcName, rf, expr, start, ec.End, ec.Step, window, ec.LookbackDelta, sharedTimestamps)
	if err != nil {
		return nil, err
	}

	// Fetch the remaining part of the result.
	tfs := searchutils.ToTagFilters(me.LabelFilters)
	tfss := searchutils.JoinTagFilterss([][]storage.TagFilter{tfs}, ec.EnforcedTagFilterss)
	minTimestamp := start - maxSilenceInterval
	if window > ec.Step {
		minTimestamp -= window
	} else {
		minTimestamp -= ec.Step
	}
	sq := storage.NewSearchQuery(ec.AuthToken.AccountID, ec.AuthToken.ProjectID, minTimestamp, ec.End, tfss)
	rss, isPartial, err := netstorage.ProcessSearchQuery(ec.AuthToken, ec.DenyPartialResponse, sq, true, ec.Deadline)
	if err != nil {
		return nil, err
	}
	ec.updateIsPartialResponse(isPartial)
	rssLen := rss.Len()
	if rssLen == 0 {
		rss.Cancel()
		var tss []*timeseries
		if funcName == "absent_over_time" {
			tss = getAbsentTimeseries(ec, me)
		}
		// Add missing points until ec.End.
		// Do not cache the result, since missing points
		// may be backfilled in the future.
		tss = mergeTimeseries(tssCached, tss, start, ec)
		return tss, nil
	}

	// Verify timeseries fit available memory after the rollup.
	// Take into account points from tssCached.
	pointsPerTimeseries := 1 + (ec.End-ec.Start)/ec.Step
	timeseriesLen := rssLen
	if iafc != nil {
		// Incremental aggregates require holding only GOMAXPROCS timeseries in memory.
		timeseriesLen = cgroup.AvailableCPUs()
		if iafc.ae.Modifier.Op != "" {
			if iafc.ae.Limit > 0 {
				// There is an explicit limit on the number of output time series.
				timeseriesLen *= iafc.ae.Limit
			} else {
				// Increase the number of timeseries for non-empty group list: `aggr() by (something)`,
				// since each group can have own set of time series in memory.
				timeseriesLen *= 1000
			}
		}
		// The maximum number of output time series is limited by rssLen.
		if timeseriesLen > rssLen {
			timeseriesLen = rssLen
		}
	}
	rollupPoints := mulNoOverflow(pointsPerTimeseries, int64(timeseriesLen*len(rcs)))
	rollupMemorySize := mulNoOverflow(rollupPoints, 16)
	rml := getRollupMemoryLimiter()
	if !rml.Get(uint64(rollupMemorySize)) {
		rss.Cancel()
		return nil, fmt.Errorf("not enough memory for processing %d data points across %d time series with %d points in each time series; "+
			"total available memory for concurrent requests: %d bytes; "+
			"requested memory: %d bytes; "+
			"possible solutions are: reducing the number of matching time series; switching to node with more RAM; "+
			"increasing -memory.allowedPercent; increasing `step` query arg (%gs)",
			rollupPoints, timeseriesLen*len(rcs), pointsPerTimeseries, rml.MaxSize, uint64(rollupMemorySize), float64(ec.Step)/1e3)
	}
	defer rml.Put(uint64(rollupMemorySize))

	// Evaluate rollup
	keepMetricNames := getKeepMetricNames(expr)
	var tss []*timeseries
	if iafc != nil {
		tss, err = evalRollupWithIncrementalAggregate(funcName, keepMetricNames, iafc, rss, rcs, preFunc, sharedTimestamps)
	} else {
		tss, err = evalRollupNoIncrementalAggregate(funcName, keepMetricNames, rss, rcs, preFunc, sharedTimestamps)
	}
	if err != nil {
		return nil, err
	}
	tss = mergeTimeseries(tssCached, tss, start, ec)
	if !isPartial {
		rollupResultCacheV.Put(ec, expr, window, tss)
	}
	return tss, nil
}

var (
	rollupMemoryLimiter     memoryLimiter
	rollupMemoryLimiterOnce sync.Once
)

func getRollupMemoryLimiter() *memoryLimiter {
	rollupMemoryLimiterOnce.Do(func() {
		rollupMemoryLimiter.MaxSize = uint64(memory.Allowed()) / 2
	})
	return &rollupMemoryLimiter
}

func evalRollupWithIncrementalAggregate(funcName string, keepMetricNames bool, iafc *incrementalAggrFuncContext, rss *netstorage.Results, rcs []*rollupConfig,
	preFunc func(values []float64, timestamps []int64), sharedTimestamps []int64) ([]*timeseries, error) {
	err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
		rs.Values, rs.Timestamps = dropStaleNaNs(funcName, rs.Values, rs.Timestamps)
		preFunc(rs.Values, rs.Timestamps)
		ts := getTimeseries()
		defer putTimeseries(ts)
		for _, rc := range rcs {
			if tsm := newTimeseriesMap(funcName, keepMetricNames, sharedTimestamps, &rs.MetricName); tsm != nil {
				rc.DoTimeseriesMap(tsm, rs.Values, rs.Timestamps)
				for _, ts := range tsm.m {
					iafc.updateTimeseries(ts, workerID)
				}
				continue
			}
			ts.Reset()
			doRollupForTimeseries(funcName, keepMetricNames, rc, ts, &rs.MetricName, rs.Values, rs.Timestamps, sharedTimestamps)
			iafc.updateTimeseries(ts, workerID)

			// ts.Timestamps points to sharedTimestamps. Zero it, so it can be re-used.
			ts.Timestamps = nil
			ts.denyReuse = false
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	tss := iafc.finalizeTimeseries()
	return tss, nil
}

func evalRollupNoIncrementalAggregate(funcName string, keepMetricNames bool, rss *netstorage.Results, rcs []*rollupConfig,
	preFunc func(values []float64, timestamps []int64), sharedTimestamps []int64) ([]*timeseries, error) {
	tss := make([]*timeseries, 0, rss.Len()*len(rcs))
	var tssLock sync.Mutex
	err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
		rs.Values, rs.Timestamps = dropStaleNaNs(funcName, rs.Values, rs.Timestamps)
		preFunc(rs.Values, rs.Timestamps)
		for _, rc := range rcs {
			if tsm := newTimeseriesMap(funcName, keepMetricNames, sharedTimestamps, &rs.MetricName); tsm != nil {
				rc.DoTimeseriesMap(tsm, rs.Values, rs.Timestamps)
				tssLock.Lock()
				tss = tsm.AppendTimeseriesTo(tss)
				tssLock.Unlock()
				continue
			}
			var ts timeseries
			doRollupForTimeseries(funcName, keepMetricNames, rc, &ts, &rs.MetricName, rs.Values, rs.Timestamps, sharedTimestamps)
			tssLock.Lock()
			tss = append(tss, &ts)
			tssLock.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tss, nil
}

func doRollupForTimeseries(funcName string, keepMetricNames bool, rc *rollupConfig, tsDst *timeseries, mnSrc *storage.MetricName,
	valuesSrc []float64, timestampsSrc []int64, sharedTimestamps []int64) {
	tsDst.MetricName.CopyFrom(mnSrc)
	if len(rc.TagValue) > 0 {
		tsDst.MetricName.AddTag("rollup", rc.TagValue)
	}
	if !keepMetricNames && !rollupFuncsKeepMetricName[funcName] {
		tsDst.MetricName.ResetMetricGroup()
	}
	tsDst.Values = rc.Do(tsDst.Values[:0], valuesSrc, timestampsSrc)
	tsDst.Timestamps = sharedTimestamps
	tsDst.denyReuse = true
}

var bbPool bytesutil.ByteBufferPool

func evalNumber(ec *EvalConfig, n float64) []*timeseries {
	var ts timeseries
	ts.denyReuse = true
	ts.MetricName.AccountID = ec.AuthToken.AccountID
	ts.MetricName.ProjectID = ec.AuthToken.ProjectID
	timestamps := ec.getSharedTimestamps()
	values := make([]float64, len(timestamps))
	for i := range timestamps {
		values[i] = n
	}
	ts.Values = values
	ts.Timestamps = timestamps
	return []*timeseries{&ts}
}

func evalString(ec *EvalConfig, s string) []*timeseries {
	rv := evalNumber(ec, nan)
	rv[0].MetricName.MetricGroup = append(rv[0].MetricName.MetricGroup[:0], s...)
	return rv
}

func evalTime(ec *EvalConfig) []*timeseries {
	rv := evalNumber(ec, nan)
	timestamps := rv[0].Timestamps
	values := rv[0].Values
	for i, ts := range timestamps {
		values[i] = float64(ts) / 1e3
	}
	return rv
}

func mulNoOverflow(a, b int64) int64 {
	if math.MaxInt64/b < a {
		// Overflow
		return math.MaxInt64
	}
	return a * b
}

func dropStaleNaNs(funcName string, values []float64, timestamps []int64) ([]float64, []int64) {
	if *noStaleMarkers || funcName == "default_rollup" || funcName == "stale_samples_over_time" {
		// Do not drop Prometheus staleness marks (aka stale NaNs) for default_rollup() function,
		// since it uses them for Prometheus-style staleness detection.
		// Do not drop staleness marks for stale_samples_over_time() function, since it needs
		// to calculate the number of staleness markers.
		return values, timestamps
	}
	// Remove Prometheus staleness marks, so non-default rollup functions don't hit NaN values.
	hasStaleSamples := false
	for _, v := range values {
		if decimal.IsStaleNaN(v) {
			hasStaleSamples = true
			break
		}
	}
	if !hasStaleSamples {
		// Fast path: values have no Prometheus staleness marks.
		return values, timestamps
	}
	// Slow path: drop Prometheus staleness marks from values.
	dstValues := values[:0]
	dstTimestamps := timestamps[:0]
	for i, v := range values {
		if decimal.IsStaleNaN(v) {
			continue
		}
		dstValues = append(dstValues, v)
		dstTimestamps = append(dstTimestamps, timestamps[i])
	}
	return dstValues, dstTimestamps
}
