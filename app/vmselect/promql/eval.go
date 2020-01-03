package promql

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/metricsql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxPointsPerTimeseries = flag.Int("search.maxPointsPerTimeseries", 30e3, "The maximum points per a single timeseries returned from the search")
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
	points := (end-start)/step + 1
	if points < minTimeseriesPointsForTimeRounding {
		// Too small number of points for rounding.
		return start, end
	}

	// Round start and end to values divisible by step in order
	// to enable response caching (see EvalConfig.mayCache).

	// Round start to the nearest smaller value divisible by step.
	start -= start % step
	// Round end to the nearest bigger value divisible by step.
	adjust := end % step
	if adjust > 0 {
		end += step - adjust
	}

	// Make sure that the new number of points is the same as the initial number of points.
	newPoints := (end-start)/step + 1
	for newPoints > points {
		end -= step
		newPoints--
	}

	return start, end
}

// EvalConfig is the configuration required for query evaluation via Exec
type EvalConfig struct {
	Start int64
	End   int64
	Step  int64

	Deadline netstorage.Deadline

	MayCache bool

	// LookbackDelta is analog to `-query.lookback-delta` from Prometheus.
	LookbackDelta int64

	timestamps     []int64
	timestampsOnce sync.Once
}

// newEvalConfig returns new EvalConfig copy from src.
func newEvalConfig(src *EvalConfig) *EvalConfig {
	var ec EvalConfig
	ec.Start = src.Start
	ec.End = src.End
	ec.Step = src.Step
	ec.Deadline = src.Deadline
	ec.MayCache = src.MayCache
	ec.LookbackDelta = src.LookbackDelta

	// do not copy src.timestamps - they must be generated again.
	return &ec
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
			return nil, fmt.Errorf(`cannot evaluate %q: %s`, me.AppendString(nil), err)
		}
		return rv, nil
	}
	if re, ok := e.(*metricsql.RollupExpr); ok {
		rv, err := evalRollupFunc(ec, "default_rollup", rollupDefault, e, re, nil)
		if err != nil {
			return nil, fmt.Errorf(`cannot evaluate %q: %s`, re.AppendString(nil), err)
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
				return nil, fmt.Errorf(`cannot evaluate %q: %s`, fe.AppendString(nil), err)
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
			return nil, fmt.Errorf(`cannot evaluate %q: %s`, fe.AppendString(nil), err)
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
			return nil, fmt.Errorf(`cannot evaluate %q: %s`, ae.AppendString(nil), err)
		}
		return rv, nil
	}
	if be, ok := e.(*metricsql.BinaryOpExpr); ok {
		left, err := evalExpr(ec, be.Left)
		if err != nil {
			return nil, err
		}
		right, err := evalExpr(ec, be.Right)
		if err != nil {
			return nil, err
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
			return nil, fmt.Errorf(`cannot evaluate %q: %s`, be.AppendString(nil), err)
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
	rollupArgIdx := getRollupArgIdx(fe.Name)
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
	rollupArgIdx := getRollupArgIdx(fe.Name)
	args := make([]interface{}, len(fe.Args))
	for i, arg := range fe.Args {
		if i == rollupArgIdx {
			re = getRollupExprArg(arg)
			args[i] = re
			continue
		}
		ts, err := evalExpr(ec, arg)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot evaluate arg #%d for %q: %s", i+1, fe.AppendString(nil), err)
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

func evalRollupFunc(ec *EvalConfig, name string, rf rollupFunc, expr metricsql.Expr, re *metricsql.RollupExpr, iafc *incrementalAggrFuncContext) ([]*timeseries, error) {
	ecNew := ec
	var offset int64
	if len(re.Offset) > 0 {
		var err error
		offset, err = metricsql.DurationValue(re.Offset, ec.Step)
		if err != nil {
			return nil, err
		}
		ecNew = newEvalConfig(ec)
		ecNew.Start -= offset
		ecNew.End -= offset
		if ecNew.MayCache {
			start, end := AdjustStartEnd(ecNew.Start, ecNew.End, ecNew.Step)
			offset += ecNew.Start - start
			ecNew.Start = start
			ecNew.End = end
		}
	}
	var rvs []*timeseries
	var err error
	if me, ok := re.Expr.(*metricsql.MetricExpr); ok {
		rvs, err = evalRollupFuncWithMetricExpr(ecNew, name, rf, expr, me, iafc, re.Window)
	} else {
		if iafc != nil {
			logger.Panicf("BUG: iafc must be nil for rollup %q over subquery %q", name, re.AppendString(nil))
		}
		rvs, err = evalRollupFuncWithSubquery(ecNew, name, rf, re)
	}
	if err != nil {
		return nil, err
	}
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

func evalRollupFuncWithSubquery(ec *EvalConfig, name string, rf rollupFunc, re *metricsql.RollupExpr) ([]*timeseries, error) {
	// TODO: determine whether to use rollupResultCacheV here.
	var step int64
	if len(re.Step) > 0 {
		var err error
		step, err = metricsql.PositiveDurationValue(re.Step, ec.Step)
		if err != nil {
			return nil, err
		}
	} else {
		step = ec.Step
	}
	var window int64
	if len(re.Window) > 0 {
		var err error
		window, err = metricsql.PositiveDurationValue(re.Window, ec.Step)
		if err != nil {
			return nil, err
		}
	}

	ecSQ := newEvalConfig(ec)
	ecSQ.Start -= window + maxSilenceInterval + step
	ecSQ.Step = step
	if err := ValidateMaxPointsPerTimeseries(ecSQ.Start, ecSQ.End, ecSQ.Step); err != nil {
		return nil, err
	}
	ecSQ.Start, ecSQ.End = AdjustStartEnd(ecSQ.Start, ecSQ.End, ecSQ.Step)
	tssSQ, err := evalExpr(ecSQ, re.Expr)
	if err != nil {
		return nil, err
	}

	sharedTimestamps := getTimestamps(ec.Start, ec.End, ec.Step)
	preFunc, rcs := getRollupConfigs(name, rf, ec.Start, ec.End, ec.Step, window, ec.LookbackDelta, sharedTimestamps)
	tss := make([]*timeseries, 0, len(tssSQ)*len(rcs))
	var tssLock sync.Mutex
	removeMetricGroup := !rollupFuncsKeepMetricGroup[name]
	doParallel(tssSQ, func(tsSQ *timeseries, values []float64, timestamps []int64) ([]float64, []int64) {
		values, timestamps = removeNanValues(values[:0], timestamps[:0], tsSQ.Values, tsSQ.Timestamps)
		preFunc(values, timestamps)
		for _, rc := range rcs {
			var ts timeseries
			doRollupForTimeseries(rc, &ts, &tsSQ.MetricName, values, timestamps, sharedTimestamps, removeMetricGroup)
			tssLock.Lock()
			tss = append(tss, &ts)
			tssLock.Unlock()
		}
		return values, timestamps
	})
	return tss, nil
}

func doParallel(tss []*timeseries, f func(ts *timeseries, values []float64, timestamps []int64) ([]float64, []int64)) {
	concurrency := runtime.GOMAXPROCS(-1)
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

func evalRollupFuncWithMetricExpr(ec *EvalConfig, name string, rf rollupFunc,
	expr metricsql.Expr, me *metricsql.MetricExpr, iafc *incrementalAggrFuncContext, windowStr string) ([]*timeseries, error) {
	if me.IsEmpty() {
		return evalNumber(ec, nan), nil
	}
	var window int64
	if len(windowStr) > 0 {
		var err error
		window, err = metricsql.PositiveDurationValue(windowStr, ec.Step)
		if err != nil {
			return nil, err
		}
	}

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

	// Fetch the remaining part of the result.
	tfs := toTagFilters(me.LabelFilters)
	sq := &storage.SearchQuery{
		MinTimestamp: start - window - maxSilenceInterval,
		MaxTimestamp: ec.End + ec.Step,
		TagFilterss:  [][]storage.TagFilter{tfs},
	}
	rss, err := netstorage.ProcessSearchQuery(sq, true, ec.Deadline)
	if err != nil {
		return nil, err
	}
	rssLen := rss.Len()
	if rssLen == 0 {
		rss.Cancel()
		// Add missing points until ec.End.
		// Do not cache the result, since missing points
		// may be backfilled in the future.
		tss := mergeTimeseries(tssCached, nil, start, ec)
		return tss, nil
	}
	sharedTimestamps := getTimestamps(start, ec.End, ec.Step)
	preFunc, rcs := getRollupConfigs(name, rf, start, ec.End, ec.Step, window, ec.LookbackDelta, sharedTimestamps)

	// Verify timeseries fit available memory after the rollup.
	// Take into account points from tssCached.
	pointsPerTimeseries := 1 + (ec.End-ec.Start)/ec.Step
	timeseriesLen := rssLen
	if iafc != nil {
		// Incremental aggregates require hold only GOMAXPROCS timeseries in memory.
		timeseriesLen = runtime.GOMAXPROCS(-1)
		if iafc.ae.Modifier.Op != "" {
			// Increase the number of timeseries for non-empty group list: `aggr() by (something)`,
			// since each group can have own set of time series in memory.
			// Estimate the number of such groups is lower than 1000 :)
			timeseriesLen *= 1000
		}
	}
	rollupPoints := mulNoOverflow(pointsPerTimeseries, int64(timeseriesLen*len(rcs)))
	rollupMemorySize := mulNoOverflow(rollupPoints, 16)
	rml := getRollupMemoryLimiter()
	if !rml.Get(uint64(rollupMemorySize)) {
		rss.Cancel()
		return nil, fmt.Errorf("not enough memory for processing %d data points across %d time series with %d points in each time series; "+
			"possible solutions are: reducing the number of matching time series; switching to node with more RAM; "+
			"increasing -memory.allowedPercent; increasing `step` query arg (%gs)",
			rollupPoints, rssLen*len(rcs), pointsPerTimeseries, float64(ec.Step)/1e3)
	}
	defer rml.Put(uint64(rollupMemorySize))

	// Evaluate rollup
	removeMetricGroup := !rollupFuncsKeepMetricGroup[name]
	var tss []*timeseries
	if iafc != nil {
		tss, err = evalRollupWithIncrementalAggregate(iafc, rss, rcs, preFunc, sharedTimestamps, removeMetricGroup)
	} else {
		tss, err = evalRollupNoIncrementalAggregate(rss, rcs, preFunc, sharedTimestamps, removeMetricGroup)
	}
	if err != nil {
		return nil, err
	}
	tss = mergeTimeseries(tssCached, tss, start, ec)
	rollupResultCacheV.Put(ec, expr, window, tss)
	return tss, nil
}

var (
	rollupMemoryLimiter     memoryLimiter
	rollupMemoryLimiterOnce sync.Once
)

func getRollupMemoryLimiter() *memoryLimiter {
	rollupMemoryLimiterOnce.Do(func() {
		rollupMemoryLimiter.MaxSize = uint64(memory.Allowed()) / 4
	})
	return &rollupMemoryLimiter
}

func evalRollupWithIncrementalAggregate(iafc *incrementalAggrFuncContext, rss *netstorage.Results, rcs []*rollupConfig,
	preFunc func(values []float64, timestamps []int64), sharedTimestamps []int64, removeMetricGroup bool) ([]*timeseries, error) {
	err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
		preFunc(rs.Values, rs.Timestamps)
		ts := getTimeseries()
		defer putTimeseries(ts)
		for _, rc := range rcs {
			ts.Reset()
			doRollupForTimeseries(rc, ts, &rs.MetricName, rs.Values, rs.Timestamps, sharedTimestamps, removeMetricGroup)
			iafc.updateTimeseries(ts, workerID)

			// ts.Timestamps points to sharedTimestamps. Zero it, so it can be re-used.
			ts.Timestamps = nil
			ts.denyReuse = false
		}
	})
	if err != nil {
		return nil, err
	}
	tss := iafc.finalizeTimeseries()
	return tss, nil
}

func evalRollupNoIncrementalAggregate(rss *netstorage.Results, rcs []*rollupConfig,
	preFunc func(values []float64, timestamps []int64), sharedTimestamps []int64, removeMetricGroup bool) ([]*timeseries, error) {
	tss := make([]*timeseries, 0, rss.Len()*len(rcs))
	var tssLock sync.Mutex
	err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) {
		preFunc(rs.Values, rs.Timestamps)
		for _, rc := range rcs {
			var ts timeseries
			doRollupForTimeseries(rc, &ts, &rs.MetricName, rs.Values, rs.Timestamps, sharedTimestamps, removeMetricGroup)
			tssLock.Lock()
			tss = append(tss, &ts)
			tssLock.Unlock()
		}
	})
	if err != nil {
		return nil, err
	}
	return tss, nil
}

func doRollupForTimeseries(rc *rollupConfig, tsDst *timeseries, mnSrc *storage.MetricName, valuesSrc []float64, timestampsSrc []int64,
	sharedTimestamps []int64, removeMetricGroup bool) {
	tsDst.MetricName.CopyFrom(mnSrc)
	if len(rc.TagValue) > 0 {
		tsDst.MetricName.AddTag("rollup", rc.TagValue)
	}
	if removeMetricGroup {
		tsDst.MetricName.ResetMetricGroup()
	}
	tsDst.Values = rc.Do(tsDst.Values[:0], valuesSrc, timestampsSrc)
	tsDst.Timestamps = sharedTimestamps
	tsDst.denyReuse = true
}

func getRollupConfigs(name string, rf rollupFunc, start, end, step, window int64, lookbackDelta int64, sharedTimestamps []int64) (
	func(values []float64, timestamps []int64), []*rollupConfig) {
	preFunc := func(values []float64, timestamps []int64) {}
	if rollupFuncsRemoveCounterResets[name] {
		preFunc = func(values []float64, timestamps []int64) {
			removeCounterResets(values)
		}
	}
	newRollupConfig := func(rf rollupFunc, tagValue string) *rollupConfig {
		return &rollupConfig{
			TagValue:        tagValue,
			Func:            rf,
			Start:           start,
			End:             end,
			Step:            step,
			Window:          window,
			MayAdjustWindow: rollupFuncsMayAdjustWindow[name],
			LookbackDelta:   lookbackDelta,
			Timestamps:      sharedTimestamps,
		}
	}
	appendRollupConfigs := func(dst []*rollupConfig) []*rollupConfig {
		dst = append(dst, newRollupConfig(rollupMin, "min"))
		dst = append(dst, newRollupConfig(rollupMax, "max"))
		dst = append(dst, newRollupConfig(rollupAvg, "avg"))
		return dst
	}
	var rcs []*rollupConfig
	switch name {
	case "rollup":
		rcs = appendRollupConfigs(rcs)
	case "rollup_rate", "rollup_deriv":
		preFuncPrev := preFunc
		preFunc = func(values []float64, timestamps []int64) {
			preFuncPrev(values, timestamps)
			derivValues(values, timestamps)
		}
		rcs = appendRollupConfigs(rcs)
	case "rollup_increase", "rollup_delta":
		preFuncPrev := preFunc
		preFunc = func(values []float64, timestamps []int64) {
			preFuncPrev(values, timestamps)
			deltaValues(values)
		}
		rcs = appendRollupConfigs(rcs)
	case "rollup_candlestick":
		rcs = append(rcs, newRollupConfig(rollupFirst, "open"))
		rcs = append(rcs, newRollupConfig(rollupLast, "close"))
		rcs = append(rcs, newRollupConfig(rollupMin, "low"))
		rcs = append(rcs, newRollupConfig(rollupMax, "high"))
	default:
		rcs = append(rcs, newRollupConfig(rf, ""))
	}
	return preFunc, rcs
}

var bbPool bytesutil.ByteBufferPool

func evalNumber(ec *EvalConfig, n float64) []*timeseries {
	var ts timeseries
	ts.denyReuse = true
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
		values[i] = float64(ts) * 1e-3
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

func toTagFilters(lfs []metricsql.LabelFilter) []storage.TagFilter {
	tfs := make([]storage.TagFilter, len(lfs))
	for i := range lfs {
		toTagFilter(&tfs[i], &lfs[i])
	}
	return tfs
}

func toTagFilter(dst *storage.TagFilter, src *metricsql.LabelFilter) {
	if src.Label != "__name__" {
		dst.Key = []byte(src.Label)
	} else {
		// This is required for storage.Search.
		dst.Key = nil
	}
	dst.Value = []byte(src.Value)
	dst.IsRegexp = src.IsRegexp
	dst.IsNegative = src.IsNegative
}
