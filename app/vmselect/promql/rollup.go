package promql

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/valyala/histogram"
)

var rollupFuncs = map[string]newRollupFunc{
	"default_rollup": newRollupFuncOneArg(rollupDefault), // default rollup func

	// Standard rollup funcs from PromQL.
	// See funcs accepting range-vector on https://prometheus.io/docs/prometheus/latest/querying/functions/ .
	"changes":            newRollupFuncOneArg(rollupChanges),
	"delta":              newRollupFuncOneArg(rollupDelta),
	"deriv":              newRollupFuncOneArg(rollupDerivSlow),
	"deriv_fast":         newRollupFuncOneArg(rollupDerivFast),
	"holt_winters":       newRollupHoltWinters,
	"idelta":             newRollupFuncOneArg(rollupIdelta),
	"increase":           newRollupFuncOneArg(rollupDelta),  // + rollupFuncsRemoveCounterResets
	"irate":              newRollupFuncOneArg(rollupIderiv), // + rollupFuncsRemoveCounterResets
	"predict_linear":     newRollupPredictLinear,
	"rate":               newRollupFuncOneArg(rollupDerivFast), // + rollupFuncsRemoveCounterResets
	"resets":             newRollupFuncOneArg(rollupResets),
	"avg_over_time":      newRollupFuncOneArg(rollupAvg),
	"min_over_time":      newRollupFuncOneArg(rollupMin),
	"max_over_time":      newRollupFuncOneArg(rollupMax),
	"sum_over_time":      newRollupFuncOneArg(rollupSum),
	"count_over_time":    newRollupFuncOneArg(rollupCount),
	"quantile_over_time": newRollupQuantile,
	"stddev_over_time":   newRollupFuncOneArg(rollupStddev),
	"stdvar_over_time":   newRollupFuncOneArg(rollupStdvar),

	// Additional rollup funcs.
	"sum2_over_time":      newRollupFuncOneArg(rollupSum2),
	"geomean_over_time":   newRollupFuncOneArg(rollupGeomean),
	"first_over_time":     newRollupFuncOneArg(rollupFirst),
	"last_over_time":      newRollupFuncOneArg(rollupLast),
	"distinct_over_time":  newRollupFuncOneArg(rollupDistinct),
	"increases_over_time": newRollupFuncOneArg(rollupIncreases),
	"decreases_over_time": newRollupFuncOneArg(rollupDecreases),
	"integrate":           newRollupFuncOneArg(rollupIntegrate),
	"ideriv":              newRollupFuncOneArg(rollupIderiv),
	"lifetime":            newRollupFuncOneArg(rollupLifetime),
	"scrape_interval":     newRollupFuncOneArg(rollupScrapeInterval),
	"rollup":              newRollupFuncOneArg(rollupFake),
	"rollup_rate":         newRollupFuncOneArg(rollupFake), // + rollupFuncsRemoveCounterResets
	"rollup_deriv":        newRollupFuncOneArg(rollupFake),
	"rollup_delta":        newRollupFuncOneArg(rollupFake),
	"rollup_increase":     newRollupFuncOneArg(rollupFake), // + rollupFuncsRemoveCounterResets
	"rollup_candlestick":  newRollupFuncOneArg(rollupFake),
}

var rollupFuncsMayAdjustWindow = map[string]bool{
	"default_rollup":  true,
	"first_over_time": true,
	"last_over_time":  true,
	"deriv":           true,
	"deriv_fast":      true,
	"irate":           true,
	"rate":            true,
	"lifetime":        true,
	"scrape_interval": true,
}

var rollupFuncsRemoveCounterResets = map[string]bool{
	"increase":        true,
	"irate":           true,
	"rate":            true,
	"rollup_rate":     true,
	"rollup_increase": true,
}

var rollupFuncsKeepMetricGroup = map[string]bool{
	"default_rollup":     true,
	"avg_over_time":      true,
	"min_over_time":      true,
	"max_over_time":      true,
	"quantile_over_time": true,
	"rollup":             true,
	"geomean_over_time":  true,
}

func getRollupArgIdx(funcName string) int {
	funcName = strings.ToLower(funcName)
	if rollupFuncs[funcName] == nil {
		logger.Panicf("BUG: getRollupArgIdx is called for non-rollup func %q", funcName)
	}
	if funcName == "quantile_over_time" {
		return 1
	}
	return 0
}

func getRollupFunc(funcName string) newRollupFunc {
	funcName = strings.ToLower(funcName)
	return rollupFuncs[funcName]
}

func isRollupFunc(funcName string) bool {
	return getRollupFunc(funcName) != nil
}

type rollupFuncArg struct {
	prevValue     float64
	prevTimestamp int64
	values        []float64
	timestamps    []int64

	idx  int
	step int64
}

func (rfa *rollupFuncArg) reset() {
	rfa.prevValue = 0
	rfa.prevTimestamp = 0
	rfa.values = nil
	rfa.timestamps = nil
	rfa.idx = 0
	rfa.step = 0
}

// rollupFunc must return rollup value for the given rfa.
//
// prevValue may be nan, values and timestamps may be empty.
type rollupFunc func(rfa *rollupFuncArg) float64

type rollupConfig struct {
	// This tag value must be added to "rollup" tag if non-empty.
	TagValue string

	Func   rollupFunc
	Start  int64
	End    int64
	Step   int64
	Window int64

	// Whether window may be adjusted to 2 x interval between data points.
	// This is needed for functions which have dt in the denominator
	// such as rate, deriv, etc.
	// Without the adjustement their value would jump in unexpected directions
	// when using window smaller than 2 x scrape_interval.
	MayAdjustWindow bool

	Timestamps []int64

	// LoookbackDelta is the analog to `-query.lookback-delta` from Prometheus world.
	LookbackDelta int64
}

var (
	nan = math.NaN()
	inf = math.Inf(1)
)

// The maximum interval without previous rows.
const maxSilenceInterval = 5 * 60 * 1000

// Do calculates rollups for the given timestamps and values, appends
// them to dstValues and returns results.
//
// rc.Timestamps are used as timestamps for dstValues.
//
// timestamps must cover time range [rc.Start - rc.Window - maxSilenceInterval ... rc.End + rc.Step].
//
// Cannot be called from concurrent goroutines.
func (rc *rollupConfig) Do(dstValues []float64, values []float64, timestamps []int64) []float64 {
	// Sanity checks.
	if rc.Step <= 0 {
		logger.Panicf("BUG: Step must be bigger than 0; got %d", rc.Step)
	}
	if rc.Start > rc.End {
		logger.Panicf("BUG: Start cannot exceed End; got %d vs %d", rc.Start, rc.End)
	}
	if rc.Window < 0 {
		logger.Panicf("BUG: Window must be non-negative; got %d", rc.Window)
	}
	if err := ValidateMaxPointsPerTimeseries(rc.Start, rc.End, rc.Step); err != nil {
		logger.Panicf("BUG: %s; this must be validated before the call to rollupConfig.Do", err)
	}

	// Extend dstValues in order to remove mallocs below.
	dstValues = decimal.ExtendFloat64sCapacity(dstValues, len(rc.Timestamps))

	maxPrevInterval := getMaxPrevInterval(timestamps)
	if rc.LookbackDelta > 0 && maxPrevInterval > rc.LookbackDelta {
		maxPrevInterval = rc.LookbackDelta
	}
	window := rc.Window
	if window <= 0 {
		window = rc.Step
	}
	if rc.MayAdjustWindow && window < maxPrevInterval {
		window = maxPrevInterval
	}
	rfa := getRollupFuncArg()
	rfa.idx = 0
	rfa.step = rc.Step

	i := 0
	j := 0
	ni := 0
	nj := 0
	for _, tEnd := range rc.Timestamps {
		tStart := tEnd - window
		ni = seekFirstTimestampIdxAfter(timestamps[i:], tStart, ni)
		i += ni
		if j < i {
			j = i
		}
		nj = seekFirstTimestampIdxAfter(timestamps[j:], tEnd, nj)
		j += nj

		rfa.prevValue = nan
		rfa.prevTimestamp = tStart - maxPrevInterval
		if i < len(timestamps) && i > 0 && timestamps[i-1] > rfa.prevTimestamp {
			rfa.prevValue = values[i-1]
			rfa.prevTimestamp = timestamps[i-1]
		}

		rfa.values = values[i:j]
		rfa.timestamps = timestamps[i:j]
		value := rc.Func(rfa)
		rfa.idx++
		dstValues = append(dstValues, value)
	}
	putRollupFuncArg(rfa)

	return dstValues
}

func seekFirstTimestampIdxAfter(timestamps []int64, seekTimestamp int64, nHint int) int {
	if len(timestamps) == 0 || timestamps[0] > seekTimestamp {
		return 0
	}
	startIdx := nHint - 2
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(timestamps) {
		startIdx = len(timestamps) - 1
	}
	endIdx := nHint + 2
	if endIdx > len(timestamps) {
		endIdx = len(timestamps)
	}
	if startIdx > 0 && timestamps[startIdx] <= seekTimestamp {
		timestamps = timestamps[startIdx:]
		endIdx -= startIdx
	} else {
		startIdx = 0
	}
	if endIdx < len(timestamps) && timestamps[endIdx] > seekTimestamp {
		timestamps = timestamps[:endIdx]
	}
	if len(timestamps) < 16 {
		// Fast path: the number of timestamps to search is small, so scan them all.
		for i, timestamp := range timestamps {
			if timestamp > seekTimestamp {
				return startIdx + i
			}
		}
		return startIdx + len(timestamps)
	}
	// Slow path: too big len(timestamps), so use binary search.
	i := sort.Search(len(timestamps), func(n int) bool {
		return n >= 0 && n < len(timestamps) && timestamps[n] > seekTimestamp
	})
	return startIdx + i
}

func getMaxPrevInterval(timestamps []int64) int64 {
	if len(timestamps) < 2 {
		return int64(maxSilenceInterval)
	}
	d := (timestamps[len(timestamps)-1] - timestamps[0]) / int64(len(timestamps)-1)
	if d <= 0 {
		return int64(maxSilenceInterval)
	}
	// Increase d more for smaller scrape intervals in order to hide possible gaps
	// when high jitter is present.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/139 .
	if d <= 2*1000 {
		return d + 4*d
	}
	if d <= 4*1000 {
		return d + 2*d
	}
	if d <= 8*1000 {
		return d + d
	}
	if d <= 16*1000 {
		return d + d/2
	}
	if d <= 32*1000 {
		return d + d/4
	}
	return d + d/8
}

func removeCounterResets(values []float64) {
	// There is no need in handling NaNs here, since they are impossible
	// on values from vmstorage.
	if len(values) == 0 {
		return
	}
	var correction float64
	prevValue := values[0]
	for i, v := range values {
		d := v - prevValue
		if d < 0 {
			if (-d * 8) < prevValue {
				// This is likely jitter from `Prometheus HA pairs`.
				// Just substitute v with prevValue.
				v = prevValue
			} else {
				correction += prevValue
			}
		}
		prevValue = v
		values[i] = v + correction
	}
}

func deltaValues(values []float64) {
	// There is no need in handling NaNs here, since they are impossible
	// on values from vmstorage.
	if len(values) == 0 {
		return
	}
	prevDelta := float64(0)
	prevValue := values[0]
	for i, v := range values[1:] {
		prevDelta = v - prevValue
		values[i] = prevDelta
		prevValue = v
	}
	values[len(values)-1] = prevDelta
}

func derivValues(values []float64, timestamps []int64) {
	// There is no need in handling NaNs here, since they are impossible
	// on values from vmstorage.
	if len(values) == 0 {
		return
	}
	prevDeriv := float64(0)
	prevValue := values[0]
	prevTs := timestamps[0]
	for i, v := range values[1:] {
		ts := timestamps[i+1]
		if ts == prevTs {
			// Use the previous value for duplicate timestamps.
			values[i] = prevDeriv
			continue
		}
		dt := float64(ts-prevTs) * 1e-3
		prevDeriv = (v - prevValue) / dt
		values[i] = prevDeriv
		prevValue = v
		prevTs = ts
	}
	values[len(values)-1] = prevDeriv
}

type newRollupFunc func(args []interface{}) (rollupFunc, error)

func newRollupFuncOneArg(rf rollupFunc) newRollupFunc {
	return func(args []interface{}) (rollupFunc, error) {
		if err := expectRollupArgsNum(args, 1); err != nil {
			return nil, err
		}
		return rf, nil
	}
}

func newRollupHoltWinters(args []interface{}) (rollupFunc, error) {
	if err := expectRollupArgsNum(args, 3); err != nil {
		return nil, err
	}
	sfs, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	tfs, err := getScalar(args[2], 2)
	if err != nil {
		return nil, err
	}
	rf := func(rfa *rollupFuncArg) float64 {
		// There is no need in handling NaNs here, since they must be cleaned up
		// before calling rollup funcs.
		values := rfa.values
		if len(values) == 0 {
			return rfa.prevValue
		}
		sf := sfs[rfa.idx]
		if sf <= 0 || sf >= 1 {
			return nan
		}
		tf := tfs[rfa.idx]
		if tf <= 0 || tf >= 1 {
			return nan
		}

		// See https://en.wikipedia.org/wiki/Exponential_smoothing#Double_exponential_smoothing .
		// TODO: determine whether this shit really works.
		s0 := rfa.prevValue
		if math.IsNaN(s0) {
			s0 = values[0]
			values = values[1:]
			if len(values) == 0 {
				return s0
			}
		}
		b0 := values[0] - s0
		for _, v := range values {
			s1 := sf*v + (1-sf)*(s0+b0)
			b1 := tf*(s1-s0) + (1-tf)*b0
			s0 = s1
			b0 = b1
		}
		return s0
	}
	return rf, nil
}

func newRollupPredictLinear(args []interface{}) (rollupFunc, error) {
	if err := expectRollupArgsNum(args, 2); err != nil {
		return nil, err
	}
	secs, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	rf := func(rfa *rollupFuncArg) float64 {
		v, k := linearRegression(rfa)
		if math.IsNaN(v) {
			return nan
		}
		sec := secs[rfa.idx]
		return v + k*sec
	}
	return rf, nil
}

func linearRegression(rfa *rollupFuncArg) (float64, float64) {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	timestamps := rfa.timestamps
	if len(values) == 0 {
		return rfa.prevValue, 0
	}

	// See https://en.wikipedia.org/wiki/Simple_linear_regression#Numerical_example
	tFirst := rfa.prevTimestamp
	vSum := rfa.prevValue
	tSum := float64(0)
	tvSum := float64(0)
	ttSum := float64(0)
	n := 1.0
	if math.IsNaN(rfa.prevValue) {
		tFirst = timestamps[0]
		vSum = 0
		n = 0
	}
	for i, v := range values {
		dt := float64(timestamps[i]-tFirst) * 1e-3
		vSum += v
		tSum += dt
		tvSum += dt * v
		ttSum += dt * dt
	}
	n += float64(len(values))
	if n == 1 {
		return vSum, 0
	}
	k := (n*tvSum - tSum*vSum) / (n*ttSum - tSum*tSum)
	v := (vSum - k*tSum) / n
	// Adjust v to the last timestamp on the given time range.
	v += k * (float64(timestamps[len(timestamps)-1]-tFirst) * 1e-3)
	return v, k
}

func newRollupQuantile(args []interface{}) (rollupFunc, error) {
	if err := expectRollupArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	rf := func(rfa *rollupFuncArg) float64 {
		// There is no need in handling NaNs here, since they must be cleaned up
		// before calling rollup funcs.
		values := rfa.values
		if len(values) == 0 {
			return rfa.prevValue
		}
		if len(values) == 1 {
			// Fast path - only a single value.
			return values[0]
		}
		hf := histogram.GetFast()
		for _, v := range values {
			hf.Update(v)
		}
		phi := phis[rfa.idx]
		qv := hf.Quantile(phi)
		histogram.PutFast(hf)
		return qv
	}
	return rf, nil
}

func rollupAvg(rfa *rollupFuncArg) float64 {
	// Do not use `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation,
	// since it is slower and has no significant benefits in precision.

	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		return rfa.prevValue
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func rollupMin(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	minValue := rfa.prevValue
	values := rfa.values
	if math.IsNaN(minValue) {
		if len(values) == 0 {
			return nan
		}
		minValue = values[0]
	}
	for _, v := range values {
		if v < minValue {
			minValue = v
		}
	}
	return minValue
}

func rollupMax(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	maxValue := rfa.prevValue
	values := rfa.values
	if math.IsNaN(maxValue) {
		if len(values) == 0 {
			return nan
		}
		maxValue = values[0]
	}
	for _, v := range values {
		if v > maxValue {
			maxValue = v
		}
	}
	return maxValue
}

func rollupSum(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum
}

func rollupSum2(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		return rfa.prevValue * rfa.prevValue
	}
	var sum2 float64
	for _, v := range values {
		sum2 += v * v
	}
	return sum2
}

func rollupGeomean(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		return rfa.prevValue
	}
	p := 1.0
	for _, v := range values {
		p *= v
	}
	return math.Pow(p, 1/float64(len(values)))
}

func rollupCount(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	return float64(len(values))
}

func rollupStddev(rfa *rollupFuncArg) float64 {
	stdvar := rollupStdvar(rfa)
	return math.Sqrt(stdvar)
}

func rollupStdvar(rfa *rollupFuncArg) float64 {
	// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation

	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	if len(values) == 1 {
		// Fast path.
		return values[0]
	}
	var avg float64
	var count float64
	var q float64
	for _, v := range values {
		count++
		avgNew := avg + (v-avg)/count
		q += (v - avg) * (v - avgNew)
		avg = avgNew
	}
	return q / count
}

func rollupDelta(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	prevValue := rfa.prevValue
	if math.IsNaN(prevValue) {
		if len(values) == 0 {
			return nan
		}
		if len(values) == 1 {
			// Assume that the previous non-existing value was 0.
			return values[0]
		}
		prevValue = values[0]
		values = values[1:]
	}
	if len(values) == 0 {
		// Assume that the value didn't change on the given interval.
		return 0
	}
	return values[len(values)-1] - prevValue
}

func rollupIdelta(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		// Assume that the value didn't change on the given interval.
		return 0
	}
	lastValue := values[len(values)-1]
	values = values[:len(values)-1]
	if len(values) == 0 {
		prevValue := rfa.prevValue
		if math.IsNaN(prevValue) {
			// Assume that the previous non-existing value was 0.
			return lastValue
		}
		return lastValue - prevValue
	}
	return lastValue - values[len(values)-1]
}

func rollupDerivSlow(rfa *rollupFuncArg) float64 {
	// Use linear regression like Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/73
	_, k := linearRegression(rfa)
	return k
}

func rollupDerivFast(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	timestamps := rfa.timestamps
	prevValue := rfa.prevValue
	prevTimestamp := rfa.prevTimestamp
	if math.IsNaN(prevValue) {
		if len(values) < 2 {
			// It is impossible to calculate derivative on 0 or 1 values.
			return nan
		}
		prevValue = values[0]
		prevTimestamp = timestamps[0]
		values = values[1:]
		timestamps = timestamps[1:]
	}
	if len(values) == 0 {
		// Assume that the value didn't change on the given interval.
		return 0
	}
	vEnd := values[len(values)-1]
	tEnd := timestamps[len(timestamps)-1]
	dv := vEnd - prevValue
	dt := float64(tEnd-prevTimestamp) * 1e-3
	return dv / dt
}

func rollupIderiv(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	timestamps := rfa.timestamps
	if len(values) < 2 {
		if len(values) == 0 || math.IsNaN(rfa.prevValue) {
			// It is impossible to calculate derivative on 0 or 1 values.
			return nan
		}
		return (values[0] - rfa.prevValue) / (float64(timestamps[0]-rfa.prevTimestamp) * 1e-3)
	}
	vEnd := values[len(values)-1]
	tEnd := timestamps[len(timestamps)-1]
	values = values[:len(values)-1]
	timestamps = timestamps[:len(timestamps)-1]
	// Skip data points with duplicate timestamps.
	for len(timestamps) > 0 && timestamps[len(timestamps)-1] >= tEnd {
		timestamps = timestamps[:len(timestamps)-1]
	}
	var tStart int64
	var vStart float64
	if len(timestamps) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return 0
		}
		tStart = rfa.prevTimestamp
		vStart = rfa.prevValue
	} else {
		tStart = timestamps[len(timestamps)-1]
		vStart = values[len(timestamps)-1]
	}
	dv := vEnd - vStart
	dt := tEnd - tStart
	return dv / (float64(dt) * 1e-3)
}

func rollupLifetime(rfa *rollupFuncArg) float64 {
	// Calculate the duration between the first and the last data points.
	timestamps := rfa.timestamps
	if math.IsNaN(rfa.prevValue) {
		if len(timestamps) < 2 {
			return nan
		}
		return float64(timestamps[len(timestamps)-1]-timestamps[0]) * 1e-3
	}
	if len(timestamps) == 0 {
		return nan
	}
	return float64(timestamps[len(timestamps)-1]-rfa.prevTimestamp) * 1e-3
}

func rollupScrapeInterval(rfa *rollupFuncArg) float64 {
	// Calculate the average interval between data points.
	timestamps := rfa.timestamps
	if math.IsNaN(rfa.prevValue) {
		if len(timestamps) < 2 {
			return nan
		}
		return float64(timestamps[len(timestamps)-1]-timestamps[0]) * 1e-3 / float64(len(timestamps)-1)
	}
	if len(timestamps) == 0 {
		return nan
	}
	return (float64(timestamps[len(timestamps)-1]-rfa.prevTimestamp) * 1e-3) / float64(len(timestamps))
}

func rollupChanges(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	prevValue := rfa.prevValue
	n := 0
	if math.IsNaN(prevValue) {
		if len(values) == 0 {
			return nan
		}
		prevValue = values[0]
		values = values[1:]
		n++
	}
	for _, v := range values {
		if v != prevValue {
			n++
			prevValue = v
		}
	}
	return float64(n)
}

func rollupIncreases(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	prevValue := rfa.prevValue
	if math.IsNaN(prevValue) {
		prevValue = values[0]
		values = values[1:]
	}
	if len(values) == 0 {
		return 0
	}
	n := 0
	for _, v := range values {
		if v > prevValue {
			n++
		}
		prevValue = v
	}
	return float64(n)
}

// `decreases_over_time` logic is the same as `resets` logic.
var rollupDecreases = rollupResets

func rollupResets(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	prevValue := rfa.prevValue
	if math.IsNaN(prevValue) {
		prevValue = values[0]
		values = values[1:]
	}
	if len(values) == 0 {
		return 0
	}
	n := 0
	for _, v := range values {
		if v < prevValue {
			n++
		}
		prevValue = v
	}
	return float64(n)
}

func rollupFirst(rfa *rollupFuncArg) float64 {
	// See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness
	v := rfa.prevValue
	if !math.IsNaN(v) {
		return v
	}

	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		return nan
	}
	return values[0]
}

var rollupDefault = rollupLast

func rollupLast(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		return rfa.prevValue
	}
	return values[len(values)-1]
}

func rollupDistinct(rfa *rollupFuncArg) float64 {
	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	m := make(map[float64]struct{})
	for _, v := range values {
		m[v] = struct{}{}
	}
	return float64(len(m))
}

func rollupIntegrate(rfa *rollupFuncArg) float64 {
	prevTimestamp := rfa.prevTimestamp

	// There is no need in handling NaNs here, since they must be cleaned up
	// before calling rollup funcs.
	values := rfa.values
	timestamps := rfa.timestamps
	if len(values) == 0 {
		if math.IsNaN(rfa.prevValue) {
			return nan
		}
		return 0
	}
	prevValue := rfa.prevValue
	if math.IsNaN(prevValue) {
		prevValue = values[0]
		prevTimestamp = timestamps[0]
		values = values[1:]
		timestamps = timestamps[1:]
	}
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for i, v := range values {
		timestamp := timestamps[i]
		dt := float64(timestamp-prevTimestamp) * 1e-3
		sum += 0.5 * (v + prevValue) * dt
	}
	return sum
}

func rollupFake(rfa *rollupFuncArg) float64 {
	logger.Panicf("BUG: rollupFake shouldn't be called")
	return 0
}

func getScalar(arg interface{}, argNum int) ([]float64, error) {
	ts, ok := arg.([]*timeseries)
	if !ok {
		return nil, fmt.Errorf(`unexpected type for arg #%d; got %T; want %T`, argNum+1, arg, ts)
	}
	if len(ts) != 1 {
		return nil, fmt.Errorf(`arg #%d must contain a single timeseries; got %d timeseries`, argNum+1, len(ts))
	}
	return ts[0].Values, nil
}

func getString(tss []*timeseries, argNum int) (string, error) {
	if len(tss) != 1 {
		return "", fmt.Errorf(`arg #%d must contain a single timeseries; got %d timeseries`, argNum+1, len(tss))
	}
	ts := tss[0]
	for _, v := range ts.Values {
		if !math.IsNaN(v) {
			return "", fmt.Errorf(`arg #%d contains non-string timeseries`, argNum+1)
		}
	}
	return string(ts.MetricName.MetricGroup), nil
}

func expectRollupArgsNum(args []interface{}, expectedNum int) error {
	if len(args) == expectedNum {
		return nil
	}
	return fmt.Errorf(`unexpected number of args; got %d; want %d`, len(args), expectedNum)
}

func getRollupFuncArg() *rollupFuncArg {
	v := rfaPool.Get()
	if v == nil {
		return &rollupFuncArg{}
	}
	return v.(*rollupFuncArg)
}

func putRollupFuncArg(rfa *rollupFuncArg) {
	rfa.reset()
	rfaPool.Put(rfa)
}

var rfaPool sync.Pool
