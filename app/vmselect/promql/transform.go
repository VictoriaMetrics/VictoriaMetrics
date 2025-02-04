package promql

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var transformFuncs = map[string]transformFunc{
	"":                           transformUnion, // empty func is a synonym to union
	"abs":                        newTransformFuncOneArg(transformAbs),
	"absent":                     transformAbsent,
	"acos":                       newTransformFuncOneArg(transformAcos),
	"acosh":                      newTransformFuncOneArg(transformAcosh),
	"asin":                       newTransformFuncOneArg(transformAsin),
	"asinh":                      newTransformFuncOneArg(transformAsinh),
	"atan":                       newTransformFuncOneArg(transformAtan),
	"atanh":                      newTransformFuncOneArg(transformAtanh),
	"bitmap_and":                 newTransformBitmap(bitmapAnd),
	"bitmap_or":                  newTransformBitmap(bitmapOr),
	"bitmap_xor":                 newTransformBitmap(bitmapXor),
	"buckets_limit":              transformBucketsLimit,
	"ceil":                       newTransformFuncOneArg(transformCeil),
	"clamp":                      transformClamp,
	"clamp_max":                  transformClampMax,
	"clamp_min":                  transformClampMin,
	"cos":                        newTransformFuncOneArg(transformCos),
	"cosh":                       newTransformFuncOneArg(transformCosh),
	"day_of_month":               newTransformFuncDateTime(transformDayOfMonth),
	"day_of_week":                newTransformFuncDateTime(transformDayOfWeek),
	"day_of_year":                newTransformFuncDateTime(transformDayOfYear),
	"days_in_month":              newTransformFuncDateTime(transformDaysInMonth),
	"deg":                        newTransformFuncOneArg(transformDeg),
	"drop_common_labels":         transformDropCommonLabels,
	"drop_empty_series":          transformDropEmptySeries,
	"end":                        newTransformFuncZeroArgs(transformEnd),
	"exp":                        newTransformFuncOneArg(transformExp),
	"floor":                      newTransformFuncOneArg(transformFloor),
	"histogram_avg":              transformHistogramAvg,
	"histogram_quantile":         transformHistogramQuantile,
	"histogram_quantiles":        transformHistogramQuantiles,
	"histogram_share":            transformHistogramShare,
	"histogram_stddev":           transformHistogramStddev,
	"histogram_stdvar":           transformHistogramStdvar,
	"hour":                       newTransformFuncDateTime(transformHour),
	"interpolate":                transformInterpolate,
	"keep_last_value":            transformKeepLastValue,
	"keep_next_value":            transformKeepNextValue,
	"label_copy":                 transformLabelCopy,
	"label_del":                  transformLabelDel,
	"label_graphite_group":       transformLabelGraphiteGroup,
	"label_join":                 transformLabelJoin,
	"label_keep":                 transformLabelKeep,
	"label_lowercase":            transformLabelLowercase,
	"label_map":                  transformLabelMap,
	"label_match":                transformLabelMatch,
	"label_mismatch":             transformLabelMismatch,
	"label_move":                 transformLabelMove,
	"label_replace":              transformLabelReplace,
	"label_set":                  transformLabelSet,
	"label_transform":            transformLabelTransform,
	"label_uppercase":            transformLabelUppercase,
	"label_value":                transformLabelValue,
	"limit_offset":               transformLimitOffset,
	"labels_equal":               transformLabelsEqual,
	"ln":                         newTransformFuncOneArg(transformLn),
	"log2":                       newTransformFuncOneArg(transformLog2),
	"log10":                      newTransformFuncOneArg(transformLog10),
	"minute":                     newTransformFuncDateTime(transformMinute),
	"month":                      newTransformFuncDateTime(transformMonth),
	"now":                        transformNow,
	"pi":                         transformPi,
	"prometheus_buckets":         transformPrometheusBuckets,
	"rad":                        newTransformFuncOneArg(transformRad),
	"rand":                       newTransformRand(newRandFloat64),
	"rand_exponential":           newTransformRand(newRandExpFloat64),
	"rand_normal":                newTransformRand(newRandNormFloat64),
	"range_avg":                  newTransformFuncRange(runningAvg),
	"range_first":                transformRangeFirst,
	"range_last":                 transformRangeLast,
	"range_linear_regression":    transformRangeLinearRegression,
	"range_mad":                  transformRangeMAD,
	"range_max":                  newTransformFuncRange(runningMax),
	"range_min":                  newTransformFuncRange(runningMin),
	"range_normalize":            transformRangeNormalize,
	"range_quantile":             transformRangeQuantile,
	"range_stddev":               transformRangeStddev,
	"range_stdvar":               transformRangeStdvar,
	"range_sum":                  newTransformFuncRange(runningSum),
	"range_trim_outliers":        transformRangeTrimOutliers,
	"range_trim_spikes":          transformRangeTrimSpikes,
	"range_trim_zscore":          transformRangeTrimZscore,
	"range_zscore":               transformRangeZscore,
	"remove_resets":              transformRemoveResets,
	"round":                      transformRound,
	"running_avg":                newTransformFuncRunning(runningAvg),
	"running_max":                newTransformFuncRunning(runningMax),
	"running_min":                newTransformFuncRunning(runningMin),
	"running_sum":                newTransformFuncRunning(runningSum),
	"scalar":                     transformScalar,
	"sgn":                        transformSgn,
	"sin":                        newTransformFuncOneArg(transformSin),
	"sinh":                       newTransformFuncOneArg(transformSinh),
	"smooth_exponential":         transformSmoothExponential,
	"sort":                       newTransformFuncSort(false),
	"sort_by_label":              newTransformFuncSortByLabel(false),
	"sort_by_label_desc":         newTransformFuncSortByLabel(true),
	"sort_by_label_numeric":      newTransformFuncNumericSort(false),
	"sort_by_label_numeric_desc": newTransformFuncNumericSort(true),
	"sort_desc":                  newTransformFuncSort(true),
	"sqrt":                       newTransformFuncOneArg(transformSqrt),
	"start":                      newTransformFuncZeroArgs(transformStart),
	"step":                       newTransformFuncZeroArgs(transformStep),
	"tan":                        newTransformFuncOneArg(transformTan),
	"tanh":                       newTransformFuncOneArg(transformTanh),
	"time":                       transformTime,
	// "timestamp" has been moved to rollup funcs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/415
	"timezone_offset": transformTimezoneOffset,
	"union":           transformUnion,
	"vector":          transformVector,
	"year":            newTransformFuncDateTime(transformYear),
}

// These functions don't change physical meaning of input time series,
// so they don't drop metric name
var transformFuncsKeepMetricName = map[string]bool{
	"ceil":                    true,
	"clamp":                   true,
	"clamp_max":               true,
	"clamp_min":               true,
	"floor":                   true,
	"interpolate":             true,
	"keep_last_value":         true,
	"keep_next_value":         true,
	"range_avg":               true,
	"range_first":             true,
	"range_last":              true,
	"range_linear_regression": true,
	"range_max":               true,
	"range_min":               true,
	"range_normalize":         true,
	"range_quantile":          true,
	"range_stdvar":            true,
	"range_sddev":             true,
	"round":                   true,
	"running_avg":             true,
	"running_max":             true,
	"running_min":             true,
	"smooth_exponential":      true,
}

func getTransformFunc(s string) transformFunc {
	s = strings.ToLower(s)
	return transformFuncs[s]
}

type transformFuncArg struct {
	ec   *EvalConfig
	fe   *metricsql.FuncExpr
	args [][]*timeseries
}

type transformFunc func(tfa *transformFuncArg) ([]*timeseries, error)

func newTransformFuncOneArg(tf func(v float64) float64) transformFunc {
	tfe := func(values []float64) {
		for i, v := range values {
			values[i] = tf(v)
		}
	}
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if err := expectTransformArgsNum(args, 1); err != nil {
			return nil, err
		}
		return doTransformValues(args[0], tfe, tfa.fe)
	}
}

func doTransformValues(arg []*timeseries, tf func(values []float64), fe *metricsql.FuncExpr) ([]*timeseries, error) {
	name := strings.ToLower(fe.Name)
	keepMetricNames := fe.KeepMetricNames
	if transformFuncsKeepMetricName[name] {
		keepMetricNames = true
	}
	for _, ts := range arg {
		if !keepMetricNames {
			ts.MetricName.ResetMetricGroup()
		}
		tf(ts.Values)
	}
	return arg, nil
}

func transformAbs(v float64) float64 {
	return math.Abs(v)
}

func transformAbsent(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args

	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tss := args[0]
	rvs := getAbsentTimeseries(tfa.ec, tfa.fe.Args[0])
	if len(tss) == 0 {
		return rvs, nil
	}
	for i := range tss[0].Values {
		isAbsent := true
		for _, ts := range tss {
			if !math.IsNaN(ts.Values[i]) {
				isAbsent = false
				break
			}
		}
		if !isAbsent {
			rvs[0].Values[i] = nan
		}
	}
	return rvs, nil
}

func getAbsentTimeseries(ec *EvalConfig, arg metricsql.Expr) []*timeseries {
	// Copy tags from arg
	rvs := evalNumber(ec, 1)
	rv := rvs[0]
	me, ok := arg.(*metricsql.MetricExpr)
	if !ok {
		return rvs
	}
	tfss := searchutils.ToTagFilterss(me.LabelFilterss)
	if len(tfss) != 1 {
		return rvs
	}
	tfs := tfss[0]
	for i := range tfs {
		tf := &tfs[i]
		if len(tf.Key) == 0 {
			continue
		}
		if tf.IsRegexp || tf.IsNegative {
			continue
		}
		rv.MetricName.AddTagBytes(tf.Key, tf.Value)
	}
	return rvs
}

func transformCeil(v float64) float64 {
	return math.Ceil(v)
}

func transformClamp(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 3); err != nil {
		return nil, err
	}
	mins, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	maxs, err := getScalar(args[2], 2)
	if err != nil {
		return nil, err
	}
	tf := func(values []float64) {
		for i, v := range values {
			if v > maxs[i] {
				values[i] = maxs[i]
			} else if v < mins[i] {
				values[i] = mins[i]
			}
		}
	}
	return doTransformValues(args[0], tf, tfa.fe)
}

func transformClampMax(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	maxs, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	tf := func(values []float64) {
		for i, v := range values {
			if v > maxs[i] {
				values[i] = maxs[i]
			}
		}
	}
	return doTransformValues(args[0], tf, tfa.fe)
}

func transformClampMin(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	mins, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	tf := func(values []float64) {
		for i, v := range values {
			if v < mins[i] {
				values[i] = mins[i]
			}
		}
	}
	return doTransformValues(args[0], tf, tfa.fe)
}

func newTransformFuncDateTime(f func(t time.Time) int) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if len(args) > 1 {
			return nil, fmt.Errorf(`too many args; got %d; want up to %d`, len(args), 1)
		}
		var arg []*timeseries
		if len(args) == 0 {
			arg = evalTime(tfa.ec)
		} else {
			arg = args[0]
		}
		tf := func(values []float64) {
			for i, v := range values {
				if math.IsNaN(v) {
					continue
				}
				t := time.Unix(int64(v), 0).UTC()
				values[i] = float64(f(t))
			}
		}
		return doTransformValues(arg, tf, tfa.fe)
	}
}

func transformDayOfYear(t time.Time) int {
	return t.YearDay()
}

func transformDayOfMonth(t time.Time) int {
	return t.Day()
}

func transformDayOfWeek(t time.Time) int {
	return int(t.Weekday())
}

func transformDaysInMonth(t time.Time) int {
	m := t.Month()
	if m == 2 && isLeapYear(uint32(t.Year())) {
		return 29
	}
	return daysInMonth[m]
}

func transformExp(v float64) float64 {
	return math.Exp(v)
}

func transformFloor(v float64) float64 {
	return math.Floor(v)
}

func transformBucketsLimit(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	limit, err := getIntNumber(args[0], 0)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	if limit < 3 {
		// Preserve the first and the last bucket for better accuracy for min and max values.
		limit = 3
	}
	tss := vmrangeBucketsToLE(args[1])
	if len(tss) == 0 {
		return nil, nil
	}
	pointsCount := len(tss[0].Values)

	// Group timeseries by all MetricGroup+tags excluding `le` tag.
	type x struct {
		le   float64
		hits float64
		ts   *timeseries
	}
	m := make(map[string][]x)
	var b []byte
	var mn storage.MetricName
	for _, ts := range tss {
		leStr := ts.MetricName.GetTagValue("le")
		if len(leStr) == 0 {
			// Skip time series without `le` tag.
			continue
		}
		le, err := strconv.ParseFloat(string(leStr), 64)
		if err != nil {
			// Skip time series with invalid `le` tag.
			continue
		}
		mn.CopyFrom(&ts.MetricName)
		mn.RemoveTag("le")
		b = marshalMetricNameSorted(b[:0], &mn)
		k := string(b)
		m[k] = append(m[k], x{
			le: le,
			ts: ts,
		})
	}

	// Remove buckets with the smallest counters.
	rvs := make([]*timeseries, 0, len(tss))
	for _, leGroup := range m {
		if len(leGroup) <= limit {
			// Fast path - the number of buckets doesn't exceed the given limit.
			// Keep all the buckets as is.
			for _, xx := range leGroup {
				rvs = append(rvs, xx.ts)
			}
			continue
		}
		// Slow path - remove buckets with the smallest number of hits until their count reaches the limit.

		// Calculate per-bucket hits.
		sort.Slice(leGroup, func(i, j int) bool {
			return leGroup[i].le < leGroup[j].le
		})
		for n := 0; n < pointsCount; n++ {
			prevValue := float64(0)
			for i := range leGroup {
				xx := &leGroup[i]
				value := xx.ts.Values[n]
				xx.hits += value - prevValue
				prevValue = value
			}
		}
		for len(leGroup) > limit {
			// Preserve the first and the last bucket for better accuracy for min and max values
			xxMinIdx := 1
			minMergeHits := leGroup[1].hits + leGroup[2].hits
			for i := range leGroup[1 : len(leGroup)-2] {
				mergeHits := leGroup[i+1].hits + leGroup[i+2].hits
				if mergeHits < minMergeHits {
					xxMinIdx = i + 1
					minMergeHits = mergeHits
				}
			}
			leGroup[xxMinIdx+1].hits += leGroup[xxMinIdx].hits
			leGroup = append(leGroup[:xxMinIdx], leGroup[xxMinIdx+1:]...)
		}
		for _, xx := range leGroup {
			rvs = append(rvs, xx.ts)
		}
	}
	return rvs, nil
}

func transformPrometheusBuckets(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := vmrangeBucketsToLE(args[0])
	return rvs, nil
}

func vmrangeBucketsToLE(tss []*timeseries) []*timeseries {
	rvs := make([]*timeseries, 0, len(tss))

	// Group timeseries by MetricGroup+tags excluding `vmrange` tag.
	type x struct {
		startStr string
		endStr   string
		start    float64
		end      float64
		ts       *timeseries
	}
	m := make(map[string][]x)
	bb := bbPool.Get()
	defer bbPool.Put(bb)
	for _, ts := range tss {
		vmrange := ts.MetricName.GetTagValue("vmrange")
		if len(vmrange) == 0 {
			if le := ts.MetricName.GetTagValue("le"); len(le) > 0 {
				// Keep Prometheus-compatible buckets.
				rvs = append(rvs, ts)
			}
			continue
		}
		n := strings.Index(bytesutil.ToUnsafeString(vmrange), "...")
		if n < 0 {
			continue
		}
		startStr := string(vmrange[:n])
		start, err := strconv.ParseFloat(startStr, 64)
		if err != nil {
			continue
		}
		endStr := string(vmrange[n+len("..."):])
		end, err := strconv.ParseFloat(endStr, 64)
		if err != nil {
			continue
		}
		ts.MetricName.RemoveTag("le")
		ts.MetricName.RemoveTag("vmrange")
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		k := string(bb.B)
		m[k] = append(m[k], x{
			startStr: startStr,
			endStr:   endStr,
			start:    start,
			end:      end,
			ts:       ts,
		})
	}

	// Convert `vmrange` label in each group of time series to `le` label.
	copyTS := func(src *timeseries, leStr string) *timeseries {
		var ts timeseries
		ts.CopyFromShallowTimestamps(src)
		values := ts.Values
		for i := range values {
			values[i] = 0
		}
		ts.MetricName.RemoveTag("le")
		ts.MetricName.AddTag("le", leStr)
		return &ts
	}
	isZeroTS := func(ts *timeseries) bool {
		for _, v := range ts.Values {
			if v > 0 {
				return false
			}
		}
		return true
	}
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool { return xss[i].end < xss[j].end })
		xssNew := make([]x, 0, len(xss)+2)
		var xsPrev x
		uniqTs := make(map[string]*timeseries, len(xss))
		for _, xs := range xss {
			ts := xs.ts
			if isZeroTS(ts) {
				// Skip buckets with zero values - they will be merged into a single bucket
				// when the next non-zero bucket appears.

				// Do not store xs in xsPrev in order to properly create `le` time series
				// for zero buckets.
				// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4021
				continue
			}
			if xs.start != xsPrev.end {
				// There is a gap between the previous bucket and the current bucket
				// or the previous bucket is skipped because it was zero.
				// Fill it with a time series with le=xs.start.
				if uniqTs[xs.startStr] == nil {
					uniqTs[xs.startStr] = xs.ts
					xssNew = append(xssNew, x{
						endStr: xs.startStr,
						end:    xs.start,
						ts:     copyTS(ts, xs.startStr),
					})
				}
			}
			// Convert the current time series to a time series with le=xs.end
			ts.MetricName.AddTag("le", xs.endStr)
			prevTs := uniqTs[xs.endStr]
			if prevTs != nil {
				// the end of the current bucket is not unique, need to merge it with the existing bucket.
				_ = mergeNonOverlappingTimeseries(prevTs, xs.ts)
			} else {
				xssNew = append(xssNew, xs)
				uniqTs[xs.endStr] = xs.ts
			}
			xsPrev = xs
		}
		if xsPrev.ts != nil && !math.IsInf(xsPrev.end, 1) && !isZeroTS(xsPrev.ts) {
			xssNew = append(xssNew, x{
				endStr: "+Inf",
				end:    math.Inf(1),
				ts:     copyTS(xsPrev.ts, "+Inf"),
			})
		}
		xss = xssNew
		if len(xss) == 0 {
			continue
		}
		for i := range xss[0].ts.Values {
			count := float64(0)
			for _, xs := range xss {
				ts := xs.ts
				v := ts.Values[i]
				if !math.IsNaN(v) && v > 0 {
					count += v
				}
				ts.Values[i] = count
			}
		}
		for _, xs := range xss {
			rvs = append(rvs, xs.ts)
		}
	}
	return rvs
}

func transformHistogramShare(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2...3", len(args))
	}
	les, err := getScalar(args[0], 0)
	if err != nil {
		return nil, fmt.Errorf("cannot parse le: %w", err)
	}

	// Convert buckets with `vmrange` labels to buckets with `le` labels.
	tss := vmrangeBucketsToLE(args[1])

	// Parse boundsLabel. See https://github.com/prometheus/prometheus/issues/5706 for details.
	var boundsLabel string
	if len(args) > 2 {
		s, err := getString(args[2], 2)
		if err != nil {
			return nil, fmt.Errorf("cannot parse boundsLabel (arg #3): %w", err)
		}
		boundsLabel = s
	}

	// Group metrics by all tags excluding "le"
	m := groupLeTimeseries(tss)

	// Calculate share for les
	share := func(i int, les []float64, xss []leTimeseries) (q, lower, upper float64) {
		leReq := les[i]
		if math.IsNaN(leReq) || len(xss) == 0 {
			return nan, nan, nan
		}
		fixBrokenBuckets(i, xss)
		if leReq < 0 {
			return 0, 0, 0
		}
		if math.IsInf(leReq, 1) {
			return 1, 1, 1
		}
		var vPrev, lePrev float64
		for _, xs := range xss {
			v := xs.ts.Values[i]
			le := xs.le
			if leReq >= le {
				vPrev = v
				lePrev = le
				continue
			}
			// precondition: lePrev <= leReq < le
			vLast := xss[len(xss)-1].ts.Values[i]
			lower = vPrev / vLast
			if math.IsInf(le, 1) {
				return lower, lower, 1
			}
			if lePrev == leReq {
				return lower, lower, lower
			}
			upper = v / vLast
			q = lower + (v-vPrev)/vLast*(leReq-lePrev)/(le-lePrev)
			return q, lower, upper
		}
		// precondition: leReq > leLast
		return 1, 1, 1
	}
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		xss = mergeSameLE(xss)
		dst := xss[0].ts
		var tsLower, tsUpper *timeseries
		if len(boundsLabel) > 0 {
			tsLower = &timeseries{}
			tsLower.CopyFromShallowTimestamps(dst)
			tsLower.MetricName.RemoveTag(boundsLabel)
			tsLower.MetricName.AddTag(boundsLabel, "lower")
			tsUpper = &timeseries{}
			tsUpper.CopyFromShallowTimestamps(dst)
			tsUpper.MetricName.RemoveTag(boundsLabel)
			tsUpper.MetricName.AddTag(boundsLabel, "upper")
		}
		for i := range dst.Values {
			q, lower, upper := share(i, les, xss)
			dst.Values[i] = q
			if len(boundsLabel) > 0 {
				tsLower.Values[i] = lower
				tsUpper.Values[i] = upper
			}
		}
		rvs = append(rvs, dst)
		if len(boundsLabel) > 0 {
			rvs = append(rvs, tsLower)
			rvs = append(rvs, tsUpper)
		}
	}
	return rvs, nil
}

func transformHistogramAvg(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tss := vmrangeBucketsToLE(args[0])
	m := groupLeTimeseries(tss)
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		dst := xss[0].ts
		for i := range dst.Values {
			dst.Values[i] = avgForLeTimeseries(i, xss)
		}
		rvs = append(rvs, dst)
	}
	return rvs, nil
}

func transformHistogramStddev(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tss := vmrangeBucketsToLE(args[0])
	m := groupLeTimeseries(tss)
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		dst := xss[0].ts
		for i := range dst.Values {
			v := stdvarForLeTimeseries(i, xss)
			dst.Values[i] = math.Sqrt(v)
		}
		rvs = append(rvs, dst)
	}
	return rvs, nil
}

func transformHistogramStdvar(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tss := vmrangeBucketsToLE(args[0])
	m := groupLeTimeseries(tss)
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		dst := xss[0].ts
		for i := range dst.Values {
			dst.Values[i] = stdvarForLeTimeseries(i, xss)
		}
		rvs = append(rvs, dst)
	}
	return rvs, nil
}

func avgForLeTimeseries(i int, xss []leTimeseries) float64 {
	lePrev := float64(0)
	vPrev := float64(0)
	sum := float64(0)
	weightTotal := float64(0)
	for _, xs := range xss {
		if math.IsInf(xs.le, 0) {
			continue
		}
		le := xs.le
		n := (le + lePrev) / 2
		v := xs.ts.Values[i]
		weight := v - vPrev
		sum += n * weight
		weightTotal += weight
		lePrev = le
		vPrev = v
	}
	if weightTotal == 0 {
		return nan
	}
	return sum / weightTotal
}

func stdvarForLeTimeseries(i int, xss []leTimeseries) float64 {
	lePrev := float64(0)
	vPrev := float64(0)
	sum := float64(0)
	sum2 := float64(0)
	weightTotal := float64(0)
	for _, xs := range xss {
		if math.IsInf(xs.le, 0) {
			continue
		}
		le := xs.le
		n := (le + lePrev) / 2
		v := xs.ts.Values[i]
		weight := v - vPrev
		sum += n * weight
		sum2 += n * n * weight
		weightTotal += weight
		lePrev = le
		vPrev = v
	}
	if weightTotal == 0 {
		return nan
	}
	avg := sum / weightTotal
	avg2 := sum2 / weightTotal
	stdvar := avg2 - avg*avg
	if stdvar < 0 {
		// Correct possible calculation error.
		stdvar = 0
	}
	return stdvar
}

func transformHistogramQuantiles(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting at least 3 args", len(args))
	}
	dstLabel, err := getString(args[0], 0)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain dstLabel: %w", err)
	}
	phiArgs := args[1 : len(args)-1]
	tssOrig := args[len(args)-1]
	// Calculate quantile individually per each phi.
	var rvs []*timeseries
	for i, phiArg := range phiArgs {
		phis, err := getScalar(phiArg, i)
		if err != nil {
			return nil, fmt.Errorf("cannot parse phi: %w", err)
		}
		phiStr := fmt.Sprintf("%g", phis[0])
		tss := copyTimeseries(tssOrig)
		tfaTmp := &transformFuncArg{
			ec: tfa.ec,
			fe: tfa.fe,
			args: [][]*timeseries{
				phiArg,
				tss,
			},
		}
		tssTmp, err := transformHistogramQuantile(tfaTmp)
		if err != nil {
			return nil, fmt.Errorf("cannot calculate quantile %s: %w", phiStr, err)
		}
		for _, ts := range tssTmp {
			ts.MetricName.RemoveTag(dstLabel)
			ts.MetricName.AddTag(dstLabel, phiStr)
		}
		rvs = append(rvs, tssTmp...)
	}
	return rvs, nil
}

func transformHistogramQuantile(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2...3", len(args))
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, fmt.Errorf("cannot parse phi: %w", err)
	}

	// Convert buckets with `vmrange` labels to buckets with `le` labels.
	tss := vmrangeBucketsToLE(args[1])

	// Parse boundsLabel. See https://github.com/prometheus/prometheus/issues/5706 for details.
	var boundsLabel string
	if len(args) > 2 {
		s, err := getString(args[2], 2)
		if err != nil {
			return nil, fmt.Errorf("cannot parse boundsLabel (arg #3): %w", err)
		}
		boundsLabel = s
	}

	// Group metrics by all tags excluding "le"
	m := groupLeTimeseries(tss)

	// Calculate quantile for each group in m
	lastNonInf := func(_ int, xss []leTimeseries) float64 {
		for len(xss) > 0 {
			xsLast := xss[len(xss)-1]
			if !math.IsInf(xsLast.le, 0) {
				return xsLast.le
			}
			xss = xss[:len(xss)-1]
		}
		return nan
	}
	quantile := func(i int, phis []float64, xss []leTimeseries) (q, lower, upper float64) {
		phi := phis[i]
		if math.IsNaN(phi) {
			return nan, nan, nan
		}
		fixBrokenBuckets(i, xss)
		vLast := float64(0)
		if len(xss) > 0 {
			vLast = xss[len(xss)-1].ts.Values[i]
		}
		if vLast == 0 {
			return nan, nan, nan
		}
		if phi < 0 {
			return -inf, -inf, xss[0].ts.Values[i]
		}
		if phi > 1 {
			return inf, vLast, inf
		}
		vReq := vLast * phi
		vPrev := float64(0)
		lePrev := float64(0)
		for _, xs := range xss {
			v := xs.ts.Values[i]
			le := xs.le
			if v <= 0 {
				// Skip zero buckets.
				lePrev = le
				continue
			}
			if v < vReq {
				vPrev = v
				lePrev = le
				continue
			}
			if math.IsInf(le, 0) {
				break
			}
			if v == vPrev {
				return lePrev, lePrev, v
			}
			vv := lePrev + (le-lePrev)*(vReq-vPrev)/(v-vPrev)
			return vv, lePrev, le
		}
		vv := lastNonInf(i, xss)
		return vv, vv, inf
	}
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		xss = mergeSameLE(xss)
		dst := xss[0].ts
		var tsLower, tsUpper *timeseries
		if len(boundsLabel) > 0 {
			tsLower = &timeseries{}
			tsLower.CopyFromShallowTimestamps(dst)
			tsLower.MetricName.RemoveTag(boundsLabel)
			tsLower.MetricName.AddTag(boundsLabel, "lower")
			tsUpper = &timeseries{}
			tsUpper.CopyFromShallowTimestamps(dst)
			tsUpper.MetricName.RemoveTag(boundsLabel)
			tsUpper.MetricName.AddTag(boundsLabel, "upper")
		}
		for i := range dst.Values {
			v, lower, upper := quantile(i, phis, xss)
			dst.Values[i] = v
			if len(boundsLabel) > 0 {
				tsLower.Values[i] = lower
				tsUpper.Values[i] = upper
			}
		}
		rvs = append(rvs, dst)
		if len(boundsLabel) > 0 {
			rvs = append(rvs, tsLower)
			rvs = append(rvs, tsUpper)
		}
	}
	return rvs, nil
}

type leTimeseries struct {
	le float64
	ts *timeseries
}

func groupLeTimeseries(tss []*timeseries) map[string][]leTimeseries {
	m := make(map[string][]leTimeseries)
	bb := bbPool.Get()
	for _, ts := range tss {
		tagValue := ts.MetricName.GetTagValue("le")
		if len(tagValue) == 0 {
			continue
		}
		le, err := strconv.ParseFloat(bytesutil.ToUnsafeString(tagValue), 64)
		if err != nil {
			continue
		}
		ts.MetricName.ResetMetricGroup()
		ts.MetricName.RemoveTag("le")
		bb.B = marshalMetricTagsSorted(bb.B[:0], &ts.MetricName)
		k := string(bb.B)
		m[k] = append(m[k], leTimeseries{
			le: le,
			ts: ts,
		})
	}
	bbPool.Put(bb)
	return m
}

func fixBrokenBuckets(i int, xss []leTimeseries) {
	// Buckets are already sorted by le, so their values must be in ascending order,
	// since the next bucket includes all the previous buckets.
	// If the next bucket has lower value than the current bucket,
	// then the next bucket must be substituted with the current bucket value.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4580#issuecomment-2186659102
	if len(xss) < 2 {
		return
	}

	// Substitute upper bucket values with lower bucket values if the upper values are NaN
	// or are bigger than the lower bucket values.
	vNext := xss[0].ts.Values[i]
	for j := 1; j < len(xss); j++ {
		v := xss[j].ts.Values[i]
		if math.IsNaN(v) || vNext > v {
			xss[j].ts.Values[i] = vNext
		} else {
			vNext = v
		}
	}
}

func mergeSameLE(xss []leTimeseries) []leTimeseries {
	// Merge buckets with identical le values.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3225
	xsDst := xss[0]
	dst := xss[:1]
	for j := 1; j < len(xss); j++ {
		xs := xss[j]
		if xs.le != xsDst.le {
			dst = append(dst, xs)
			xsDst = xs
			continue
		}
		dstValues := xsDst.ts.Values
		for k, v := range xs.ts.Values {
			dstValues[k] += v
		}
	}
	return dst
}

func transformHour(t time.Time) int {
	return t.Hour()
}

func runningSum(a, b float64, _ int) float64 {
	return a + b
}

func runningMax(a, b float64, _ int) float64 {
	if a > b {
		return a
	}
	return b
}

func runningMin(a, b float64, _ int) float64 {
	if a < b {
		return a
	}
	return b
}

func runningAvg(a, b float64, idx int) float64 {
	// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
	return a + (b-a)/float64(idx+1)
}

func skipLeadingNaNs(values []float64) []float64 {
	i := 0
	for i < len(values) && math.IsNaN(values[i]) {
		i++
	}
	return values[i:]
}

func skipTrailingNaNs(values []float64) []float64 {
	i := len(values) - 1
	for i >= 0 && math.IsNaN(values[i]) {
		i--
	}
	return values[:i+1]
}

func transformKeepLastValue(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		if len(values) == 0 {
			continue
		}
		lastValue := values[0]
		for i, v := range values {
			if !math.IsNaN(v) {
				lastValue = v
				continue
			}
			values[i] = lastValue
		}
	}
	return rvs, nil
}

func transformKeepNextValue(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		if len(values) == 0 {
			continue
		}
		nextValue := values[len(values)-1]
		for i := len(values) - 1; i >= 0; i-- {
			v := values[i]
			if !math.IsNaN(v) {
				nextValue = v
				continue
			}
			values[i] = nextValue
		}
	}
	return rvs, nil
}

func transformInterpolate(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := skipLeadingNaNs(ts.Values)
		values = skipTrailingNaNs(values)
		if len(values) == 0 {
			continue
		}
		prevValue := nan
		var nextValue float64
		for i := 0; i < len(values); i++ {
			if !math.IsNaN(values[i]) {
				continue
			}
			if i > 0 {
				prevValue = values[i-1]
			}
			j := i + 1
			for j < len(values) {
				if !math.IsNaN(values[j]) {
					break
				}
				j++
			}
			if j >= len(values) {
				nextValue = prevValue
			} else {
				nextValue = values[j]
			}
			if math.IsNaN(prevValue) {
				prevValue = nextValue
			}
			delta := (nextValue - prevValue) / float64(j-i+1)
			for i < j {
				prevValue += delta
				values[i] = prevValue
				i++
			}
		}
	}
	return rvs, nil
}

func newTransformFuncRunning(rf func(a, b float64, idx int) float64) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if err := expectTransformArgsNum(args, 1); err != nil {
			return nil, err
		}

		rvs := args[0]
		for _, ts := range rvs {
			ts.MetricName.ResetMetricGroup()
			values := skipLeadingNaNs(ts.Values)
			if len(values) == 0 {
				continue
			}
			prevValue := values[0]
			values = values[1:]
			for i, v := range values {
				if !math.IsNaN(v) {
					prevValue = rf(prevValue, v, i+1)
				}
				values[i] = prevValue
			}
		}
		return rvs, nil
	}
}

func newTransformFuncRange(rf func(a, b float64, idx int) float64) transformFunc {
	tfr := newTransformFuncRunning(rf)
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		rvs, err := tfr(tfa)
		if err != nil {
			return nil, err
		}
		setLastValues(rvs)
		return rvs, nil
	}
}

func transformRangeNormalize(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	var rvs []*timeseries
	for _, tss := range args {
		for _, ts := range tss {
			values := ts.Values
			vMin := inf
			vMax := -inf
			for _, v := range values {
				if math.IsNaN(v) {
					continue
				}
				if v < vMin {
					vMin = v
				}
				if v > vMax {
					vMax = v
				}
			}
			d := vMax - vMin
			if math.IsInf(d, 0) {
				continue
			}
			for i, v := range values {
				values[i] = (v - vMin) / d
			}
			rvs = append(rvs, ts)
		}
	}
	return rvs, nil
}

func transformRangeTrimZscore(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	zs, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	z := float64(0)
	if len(zs) > 0 {
		z = math.Abs(zs[0])
	}
	// Trim samples with z-score above z.
	rvs := args[1]
	for _, ts := range rvs {
		values := ts.Values
		qStddev := stddev(values)
		avg := mean(values)
		for i, v := range values {
			zCurr := math.Abs(v-avg) / qStddev
			if zCurr > z {
				values[i] = nan
			}
		}
	}
	return rvs, nil
}

func transformRangeZscore(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		qStddev := stddev(values)
		avg := mean(values)
		for i, v := range values {
			values[i] = (v - avg) / qStddev
		}
	}
	return rvs, nil
}

func mean(values []float64) float64 {
	var sum float64
	var n int
	for _, v := range values {
		if !math.IsNaN(v) {
			sum += v
			n++
		}
	}
	return sum / float64(n)
}

func transformRangeTrimOutliers(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	ks, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	k := float64(0)
	if len(ks) > 0 {
		k = ks[0]
	}
	// Trim samples satisfying the `abs(v - range_median(q)) > k*range_mad(q)`
	rvs := args[1]
	for _, ts := range rvs {
		values := ts.Values
		dMax := k * mad(values)
		qMedian := quantile(0.5, values)
		for i, v := range values {
			if math.Abs(v-qMedian) > dMax {
				values[i] = nan
			}
		}
	}
	return rvs, nil
}

func transformRangeTrimSpikes(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	phi := float64(0)
	if len(phis) > 0 {
		phi = phis[0]
	}
	// Trim 100% * (phi / 2) samples with the lowest / highest values per each time series
	phi /= 2
	phiUpper := 1 - phi
	phiLower := phi
	rvs := args[1]
	a := getFloat64s()
	values := a.A[:0]
	for _, ts := range rvs {
		values := values[:0]
		originValues := ts.Values
		for _, v := range originValues {
			if math.IsNaN(v) {
				continue
			}
			values = append(values, v)
		}
		sort.Float64s(values)
		vMax := quantileSorted(phiUpper, values)
		vMin := quantileSorted(phiLower, values)
		for i, v := range originValues {
			if math.IsNaN(v) {
				continue
			}
			if v > vMax {
				originValues[i] = nan
			} else if v < vMin {
				originValues[i] = nan
			}
		}
	}
	a.A = values
	putFloat64s(a)
	return rvs, nil
}

func transformRangeLinearRegression(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		timestamps := ts.Timestamps
		if len(timestamps) == 0 {
			continue
		}
		interceptTimestamp := timestamps[0]
		v, k := linearRegression(values, timestamps, interceptTimestamp)
		for i, t := range timestamps {
			values[i] = v + k*float64(t-interceptTimestamp)/1e3
		}
	}
	return rvs, nil
}

func transformRangeMAD(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		v := mad(values)
		for i := range values {
			values[i] = v
		}
	}
	return rvs, nil
}

func transformRangeStddev(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		v := stddev(values)
		for i := range values {
			values[i] = v
		}
	}
	return rvs, nil
}

func transformRangeStdvar(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := ts.Values
		v := stdvar(values)
		for i := range values {
			values[i] = v
		}
	}
	return rvs, nil
}

func transformRangeQuantile(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	phi := float64(0)
	if len(phis) > 0 {
		phi = phis[0]
	}
	rvs := args[1]
	a := getFloat64s()
	values := a.A[:0]
	for _, ts := range rvs {
		lastIdx := -1
		originValues := ts.Values
		values = values[:0]
		for i, v := range originValues {
			if math.IsNaN(v) {
				continue
			}
			values = append(values, v)
			lastIdx = i
		}
		if lastIdx >= 0 {
			sort.Float64s(values)
			originValues[lastIdx] = quantileSorted(phi, values)
		}
	}
	a.A = values
	putFloat64s(a)
	setLastValues(rvs)
	return rvs, nil
}

func transformRangeFirst(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := skipLeadingNaNs(ts.Values)
		if len(values) == 0 {
			continue
		}
		vFirst := values[0]
		values = ts.Values
		for i := range values {
			values[i] = vFirst
		}
	}
	return rvs, nil
}

func transformRangeLast(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	setLastValues(rvs)
	return rvs, nil
}

func setLastValues(tss []*timeseries) {
	for _, ts := range tss {
		values := skipTrailingNaNs(ts.Values)
		if len(values) == 0 {
			continue
		}
		vLast := values[len(values)-1]
		values = ts.Values
		for i := range values {
			values[i] = vLast
		}
	}
}

func transformSmoothExponential(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	sfs, err := getScalar(args[1], 1)
	if err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		values := skipLeadingNaNs(ts.Values)
		for i, v := range values {
			if !math.IsInf(v, 0) {
				values = values[i:]
				break
			}
		}
		if len(values) == 0 {
			continue
		}
		avg := values[0]
		values = values[1:]
		sfsX := sfs[len(ts.Values)-len(values):]
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
			if math.IsInf(v, 0) {
				values[i] = avg
				continue
			}
			sf := sfsX[i]
			if math.IsNaN(sf) {
				sf = 1
			}
			if sf < 0 {
				sf = 0
			}
			if sf > 1 {
				sf = 1
			}
			avg = avg*(1-sf) + v*sf
			values[i] = avg
		}
	}
	return rvs, nil
}

func transformRemoveResets(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		removeCounterResetsMaybeNaNs(ts.Values)
	}
	return rvs, nil
}

func transformUnion(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return evalNumber(tfa.ec, nan), nil
	}

	if areAllArgsScalar(args) {
		// Special case for (v1,...,vN) where vX are scalars - return all the scalars as time series.
		// This is needed for "q == (v1,...,vN)" and "q != (v1,...,vN)" cases, where vX are numeric constants.
		rvs := make([]*timeseries, len(args))
		for i, arg := range args {
			rvs[i] = arg[0]
		}
		return rvs, nil
	}

	rvs := make([]*timeseries, 0, len(args[0]))
	m := make(map[string]bool, len(args[0]))
	bb := bbPool.Get()
	for _, arg := range args {
		for _, ts := range arg {
			bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
			k := string(bb.B)
			if m[k] {
				continue
			}
			m[k] = true
			rvs = append(rvs, ts)
		}
	}
	bbPool.Put(bb)
	return rvs, nil
}

func areAllArgsScalar(args [][]*timeseries) bool {
	for _, arg := range args {
		if !isScalar(arg) {
			return false
		}
	}
	return true
}

func transformLabelKeep(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 1)
	}
	var keepLabels []string
	for i := 1; i < len(args); i++ {
		keepLabel, err := getString(args[i], i)
		if err != nil {
			return nil, err
		}
		keepLabels = append(keepLabels, keepLabel)
	}

	rvs := args[0]
	for _, ts := range rvs {
		ts.MetricName.RemoveTagsOn(keepLabels)
	}
	return rvs, nil
}

func transformLabelDel(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 1)
	}
	var delLabels []string
	for i := 1; i < len(args); i++ {
		delLabel, err := getString(args[i], i)
		if err != nil {
			return nil, err
		}
		delLabels = append(delLabels, delLabel)
	}

	rvs := args[0]
	for _, ts := range rvs {
		ts.MetricName.RemoveTagsIgnoring(delLabels)
	}
	return rvs, nil
}

func transformLabelSet(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 1)
	}
	dstLabels, dstValues, err := getStringPairs(args[1:])
	if err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		mn := &ts.MetricName
		for i, dstLabel := range dstLabels {
			value := dstValues[i]
			dstValue := getDstValue(mn, dstLabel)
			*dstValue = append((*dstValue)[:0], value...)
			if len(value) == 0 {
				mn.RemoveTag(dstLabel)
			}
		}
	}
	return rvs, nil
}

func transformLabelUppercase(tfa *transformFuncArg) ([]*timeseries, error) {
	return transformLabelValueFunc(tfa, strings.ToUpper)
}

func transformLabelLowercase(tfa *transformFuncArg) ([]*timeseries, error) {
	return transformLabelValueFunc(tfa, strings.ToLower)
}

func transformLabelValueFunc(tfa *transformFuncArg, f func(string) string) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 2 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 2)
	}
	labels := make([]string, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		label, err := getString(args[i], i)
		if err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}

	rvs := args[0]
	for _, ts := range rvs {
		mn := &ts.MetricName
		for _, label := range labels {
			dstValue := getDstValue(mn, label)
			*dstValue = append((*dstValue)[:0], f(string(*dstValue))...)
			if len(*dstValue) == 0 {
				mn.RemoveTag(label)
			}
		}
	}
	return rvs, nil
}

func transformLabelMap(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 2 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 2)
	}
	label, err := getString(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot read label name: %w", err)
	}
	srcValues, dstValues, err := getStringPairs(args[2:])
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(srcValues))
	for i, srcValue := range srcValues {
		m[srcValue] = dstValues[i]
	}
	rvs := args[0]
	for _, ts := range rvs {
		mn := &ts.MetricName
		dstValue := getDstValue(mn, label)
		value, ok := m[string(*dstValue)]
		if ok {
			*dstValue = append((*dstValue)[:0], value...)
		}
		if len(*dstValue) == 0 {
			mn.RemoveTag(label)
		}
	}
	return rvs, nil
}

func transformDropCommonLabels(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 1)
	}
	rvs := args[0]
	for _, tss := range args[1:] {
		rvs = append(rvs, tss...)
	}
	m := make(map[string]map[string]int)
	countLabel := func(name, value string) {
		x := m[name]
		if x == nil {
			x = make(map[string]int)
			m[name] = x
		}
		x[value]++
	}
	for _, ts := range rvs {
		countLabel("__name__", string(ts.MetricName.MetricGroup))
		for _, tag := range ts.MetricName.Tags {
			countLabel(string(tag.Key), string(tag.Value))
		}
	}
	for labelName, x := range m {
		for _, count := range x {
			if count != len(rvs) {
				continue
			}
			for _, ts := range rvs {
				ts.MetricName.RemoveTag(labelName)
			}
		}
	}
	return rvs, nil
}

func transformDropEmptySeries(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := removeEmptySeries(args[0])
	return rvs, nil
}

func transformLabelCopy(tfa *transformFuncArg) ([]*timeseries, error) {
	return transformLabelCopyExt(tfa, false)
}

func transformLabelMove(tfa *transformFuncArg) ([]*timeseries, error) {
	return transformLabelCopyExt(tfa, true)
}

func transformLabelCopyExt(tfa *transformFuncArg, removeSrcLabels bool) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 1 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 1)
	}
	srcLabels, dstLabels, err := getStringPairs(args[1:])
	if err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		mn := &ts.MetricName
		for i, srcLabel := range srcLabels {
			dstLabel := dstLabels[i]
			value := mn.GetTagValue(srcLabel)
			if len(value) == 0 {
				// Do not remove destination label if the source label doesn't exist.
				continue
			}
			dstValue := getDstValue(mn, dstLabel)
			*dstValue = append((*dstValue)[:0], value...)
			if removeSrcLabels && srcLabel != dstLabel {
				mn.RemoveTag(srcLabel)
			}
		}
	}
	return rvs, nil
}

func getStringPairs(args [][]*timeseries) ([]string, []string, error) {
	if len(args)%2 != 0 {
		return nil, nil, fmt.Errorf(`the number of string args must be even; got %d`, len(args))
	}
	var ks, vs []string
	for i := 0; i < len(args); i += 2 {
		k, err := getString(args[i], i)
		if err != nil {
			return nil, nil, err
		}
		ks = append(ks, k)

		v, err := getString(args[i+1], i+1)
		if err != nil {
			return nil, nil, err
		}
		vs = append(vs, v)
	}
	return ks, vs, nil
}

func transformLabelJoin(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 3 {
		return nil, fmt.Errorf(`not enough args; got %d; want at least %d`, len(args), 3)
	}
	dstLabel, err := getString(args[1], 1)
	if err != nil {
		return nil, err
	}
	separator, err := getString(args[2], 2)
	if err != nil {
		return nil, err
	}
	var srcLabels []string
	for i := 3; i < len(args); i++ {
		srcLabel, err := getString(args[i], i)
		if err != nil {
			return nil, err
		}
		srcLabels = append(srcLabels, srcLabel)
	}

	rvs := args[0]
	for _, ts := range rvs {
		mn := &ts.MetricName
		dstValue := getDstValue(mn, dstLabel)
		var b []byte
		for j, srcLabel := range srcLabels {
			srcValue := mn.GetTagValue(srcLabel)
			b = append(b, srcValue...)
			if j+1 < len(srcLabels) {
				b = append(b, separator...)
			}
		}
		*dstValue = b
		if len(b) == 0 {
			mn.RemoveTag(dstLabel)
		}
	}
	return rvs, nil
}

func transformLabelTransform(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 4); err != nil {
		return nil, err
	}
	label, err := getString(args[1], 1)
	if err != nil {
		return nil, err
	}
	regex, err := getString(args[2], 2)
	if err != nil {
		return nil, err
	}
	replacement, err := getString(args[3], 3)
	if err != nil {
		return nil, err
	}

	r, err := metricsql.CompileRegexp(regex)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regex %q: %w`, regex, err)
	}
	return labelReplace(args[0], label, r, label, replacement)
}

func transformLabelReplace(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 5); err != nil {
		return nil, err
	}
	dstLabel, err := getString(args[1], 1)
	if err != nil {
		return nil, err
	}
	replacement, err := getString(args[2], 2)
	if err != nil {
		return nil, err
	}
	srcLabel, err := getString(args[3], 3)
	if err != nil {
		return nil, err
	}
	regex, err := getString(args[4], 4)
	if err != nil {
		return nil, err
	}

	r, err := metricsql.CompileRegexpAnchored(regex)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regex %q: %w`, regex, err)
	}
	return labelReplace(args[0], srcLabel, r, dstLabel, replacement)
}

func labelReplace(tss []*timeseries, srcLabel string, r *regexp.Regexp, dstLabel, replacement string) ([]*timeseries, error) {
	replacementBytes := []byte(replacement)
	for _, ts := range tss {
		mn := &ts.MetricName
		srcValue := mn.GetTagValue(srcLabel)
		if !r.Match(srcValue) {
			continue
		}
		b := r.ReplaceAll(srcValue, replacementBytes)
		dstValue := getDstValue(mn, dstLabel)
		*dstValue = append((*dstValue)[:0], b...)
		if len(b) == 0 {
			mn.RemoveTag(dstLabel)
		}
	}
	return tss, nil
}

func transformLabelsEqual(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 3", len(args))
	}
	tss := args[0]
	var labelNames []string
	for i, ts := range args[1:] {
		labelName, err := getString(ts, i+1)
		if err != nil {
			return nil, fmt.Errorf("cannot get label name: %w", err)
		}
		labelNames = append(labelNames, labelName)
	}
	rvs := tss[:0]
	for _, ts := range tss {
		if hasIdenticalLabelValues(&ts.MetricName, labelNames) {
			rvs = append(rvs, ts)
		}
	}
	return rvs, nil
}

func hasIdenticalLabelValues(mn *storage.MetricName, labelNames []string) bool {
	if len(labelNames) < 2 {
		return true
	}
	labelValue := mn.GetTagValue(labelNames[0])
	for _, labelName := range labelNames[1:] {
		b := mn.GetTagValue(labelName)
		if string(labelValue) != string(b) {
			return false
		}
	}
	return true
}

func transformLabelValue(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	labelName, err := getString(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot get label name: %w", err)
	}
	rvs := args[0]
	for _, ts := range rvs {
		ts.MetricName.ResetMetricGroup()
		labelValue := ts.MetricName.GetTagValue(labelName)
		v, err := strconv.ParseFloat(string(labelValue), 64)
		if err != nil {
			v = nan
		}
		values := ts.Values
		for i, vOrig := range values {
			if !math.IsNaN(vOrig) {
				values[i] = v
			}
		}
	}
	// Do not remove timeseries with only NaN values, so `default` could be applied to them:
	// label_value(q, "label") default 123
	return rvs, nil
}

func transformLabelMatch(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 3); err != nil {
		return nil, err
	}
	labelName, err := getString(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot get label name: %w", err)
	}
	labelRe, err := getString(args[2], 2)
	if err != nil {
		return nil, fmt.Errorf("cannot get regexp: %w", err)
	}
	r, err := metricsql.CompileRegexpAnchored(labelRe)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regexp %q: %w`, labelRe, err)
	}
	tss := args[0]
	rvs := tss[:0]
	for _, ts := range tss {
		labelValue := ts.MetricName.GetTagValue(labelName)
		if r.Match(labelValue) {
			rvs = append(rvs, ts)
		}
	}
	return rvs, nil
}

func transformLabelMismatch(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 3); err != nil {
		return nil, err
	}
	labelName, err := getString(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot get label name: %w", err)
	}
	labelRe, err := getString(args[2], 2)
	if err != nil {
		return nil, fmt.Errorf("cannot get regexp: %w", err)
	}
	r, err := metricsql.CompileRegexpAnchored(labelRe)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regexp %q: %w`, labelRe, err)
	}
	tss := args[0]
	rvs := tss[:0]
	for _, ts := range tss {
		labelValue := ts.MetricName.GetTagValue(labelName)
		if !r.Match(labelValue) {
			rvs = append(rvs, ts)
		}
	}
	return rvs, nil
}

func transformLabelGraphiteGroup(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) < 2 {
		return nil, fmt.Errorf("unexpected number of args: %d; want at least 2 args", len(args))
	}
	tss := args[0]
	groupArgs := args[1:]
	groupIDs := make([]int, len(groupArgs))
	for i, arg := range groupArgs {
		groupID, err := getIntNumber(arg, i+1)
		if err != nil {
			return nil, fmt.Errorf("cannot get group name from arg #%d: %w", i+1, err)
		}
		groupIDs[i] = groupID
	}
	for _, ts := range tss {
		groups := bytes.Split(ts.MetricName.MetricGroup, dotSeparator)
		groupName := ts.MetricName.MetricGroup[:0]
		for j, groupID := range groupIDs {
			if groupID >= 0 && groupID < len(groups) {
				groupName = append(groupName, groups[groupID]...)
			}
			if j < len(groupIDs)-1 {
				groupName = append(groupName, '.')
			}
		}
		ts.MetricName.MetricGroup = groupName
	}
	return tss, nil
}

var dotSeparator = []byte(".")

func transformLimitOffset(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 3); err != nil {
		return nil, err
	}
	limit, err := getIntNumber(args[0], 0)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain limit arg: %w", err)
	}
	offset, err := getIntNumber(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain offset arg: %w", err)
	}
	// removeEmptySeries so offset will be calculated after empty series
	// were filtered out.
	rvs := removeEmptySeries(args[2])
	if len(rvs) >= offset {
		rvs = rvs[offset:]
	} else {
		rvs = nil
	}
	if len(rvs) > limit {
		rvs = rvs[:limit]
	}
	return rvs, nil
}

func transformLn(v float64) float64 {
	return math.Log(v)
}

func transformLog2(v float64) float64 {
	return math.Log2(v)
}

func transformLog10(v float64) float64 {
	return math.Log10(v)
}

func transformMinute(t time.Time) int {
	return t.Minute()
}

func transformMonth(t time.Time) int {
	return int(t.Month())
}

func transformRound(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf(`unexpected number of args: %d; want 1 or 2`, len(args))
	}
	var nearestArg []*timeseries
	if len(args) == 1 {
		nearestArg = evalNumber(tfa.ec, 1)
	} else {
		nearestArg = args[1]
	}
	nearest, err := getScalar(nearestArg, 1)
	if err != nil {
		return nil, err
	}
	tf := func(values []float64) {
		var nPrev float64
		var p10 float64
		for i, v := range values {
			n := nearest[i]
			if n != nPrev {
				nPrev = n
				_, e := decimal.FromFloat(n)
				p10 = math.Pow10(int(-e))
			}
			v += 0.5 * math.Copysign(n, v)
			v -= math.Mod(v, n)
			v, _ = math.Modf(v * p10)
			values[i] = v / p10
		}
	}
	return doTransformValues(args[0], tf, tfa.fe)
}

func transformSgn(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tf := func(values []float64) {
		for i, v := range values {
			sign := float64(0)
			if v < 0 {
				sign = -1
			} else if v > 0 {
				sign = 1
			}
			values[i] = sign
		}
	}
	return doTransformValues(args[0], tf, tfa.fe)
}

func transformScalar(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}

	// Verify whether the arg is a string.
	// Then try converting the string to number.
	if se, ok := tfa.fe.Args[0].(*metricsql.StringExpr); ok {
		n, err := strconv.ParseFloat(se.S, 64)
		if err != nil {
			n = nan
		}
		return evalNumber(tfa.ec, n), nil
	}

	// The arg isn't a string. Extract scalar from it.
	arg := args[0]
	if len(arg) != 1 {
		return evalNumber(tfa.ec, nan), nil
	}
	arg[0].MetricName.Reset()
	return arg, nil
}

func newTransformFuncSortByLabel(isDesc bool) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if len(args) < 2 {
			return nil, fmt.Errorf("expecting at least 2 args; got %d args", len(args))
		}
		var labels []string
		for i, arg := range args[1:] {
			label, err := getString(arg, 1)
			if err != nil {
				return nil, fmt.Errorf("cannot parse label #%d for sorting: %w", i+1, err)
			}
			labels = append(labels, label)
		}
		rvs := args[0]
		sort.SliceStable(rvs, func(i, j int) bool {
			for _, label := range labels {
				a := rvs[i].MetricName.GetTagValue(label)
				b := rvs[j].MetricName.GetTagValue(label)
				if string(a) == string(b) {
					continue
				}
				if isDesc {
					return string(b) < string(a)
				}
				return string(a) < string(b)
			}
			return false
		})
		return rvs, nil
	}
}

func newTransformFuncNumericSort(isDesc bool) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if len(args) < 2 {
			return nil, fmt.Errorf("expecting at least 2 args; got %d args", len(args))
		}
		var labels []string
		for i, arg := range args[1:] {
			label, err := getString(arg, i+1)
			if err != nil {
				return nil, fmt.Errorf("cannot parse label #%d for sorting: %w", i+1, err)
			}
			labels = append(labels, label)
		}
		rvs := args[0]
		sort.SliceStable(rvs, func(i, j int) bool {
			for _, label := range labels {
				a := rvs[i].MetricName.GetTagValue(label)
				b := rvs[j].MetricName.GetTagValue(label)
				if string(a) == string(b) {
					continue
				}
				aStr := bytesutil.ToUnsafeString(a)
				bStr := bytesutil.ToUnsafeString(b)
				if isDesc {
					return numericLess(bStr, aStr)
				}
				return numericLess(aStr, bStr)
			}
			return false
		})
		return rvs, nil
	}
}

func numericLess(a, b string) bool {
	for {
		if len(b) == 0 {
			return false
		}
		if len(a) == 0 {
			return true
		}
		aPrefix := getNumPrefix(a)
		bPrefix := getNumPrefix(b)
		a = a[len(aPrefix):]
		b = b[len(bPrefix):]
		if len(aPrefix) > 0 || len(bPrefix) > 0 {
			if len(aPrefix) == 0 {
				return false
			}
			if len(bPrefix) == 0 {
				return true
			}
			aNum := mustParseNum(aPrefix)
			bNum := mustParseNum(bPrefix)
			if aNum != bNum {
				return aNum < bNum
			}
		}
		aPrefix = getNonNumPrefix(a)
		bPrefix = getNonNumPrefix(b)
		a = a[len(aPrefix):]
		b = b[len(bPrefix):]
		if aPrefix != bPrefix {
			return aPrefix < bPrefix
		}
	}
}

func getNumPrefix(s string) string {
	i := 0
	if len(s) > 0 {
		switch s[0] {
		case '-', '+':
			i++
		}
	}
	hasNum := false
	hasDot := false
	for i < len(s) {
		if !isDecimalChar(s[i]) {
			if !hasDot && s[i] == '.' {
				hasDot = true
				i++
				continue
			}
			if !hasNum {
				return ""
			}
			return s[:i]
		}
		hasNum = true
		i++
	}
	if !hasNum {
		return ""
	}
	return s
}

func getNonNumPrefix(s string) string {
	i := 0
	for i < len(s) {
		if isDecimalChar(s[i]) {
			return s[:i]
		}
		i++
	}
	return s
}

func isDecimalChar(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func mustParseNum(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing the number %q: %s", s, err)
	}
	return f
}

func newTransformFuncSort(isDesc bool) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if err := expectTransformArgsNum(args, 1); err != nil {
			return nil, err
		}
		rvs := args[0]
		sort.Slice(rvs, func(i, j int) bool {
			a := rvs[i].Values
			b := rvs[j].Values
			n := len(a) - 1
			for n >= 0 {
				if !math.IsNaN(a[n]) {
					if math.IsNaN(b[n]) {
						return false
					}
					if a[n] != b[n] {
						break
					}
				} else if !math.IsNaN(b[n]) {
					return true
				}
				n--
			}
			if n < 0 {
				return false
			}
			if isDesc {
				return b[n] < a[n]
			}
			return a[n] < b[n]
		})
		return rvs, nil
	}
}

func transformSqrt(v float64) float64 {
	return math.Sqrt(v)
}

func transformSin(v float64) float64 {
	return math.Sin(v)
}

func transformSinh(v float64) float64 {
	return math.Sinh(v)
}

func transformCos(v float64) float64 {
	return math.Cos(v)
}

func transformCosh(v float64) float64 {
	return math.Cosh(v)
}

func transformTan(v float64) float64 {
	return math.Tan(v)
}

func transformTanh(v float64) float64 {
	return math.Tanh(v)
}

func transformAsin(v float64) float64 {
	return math.Asin(v)
}

func transformAsinh(v float64) float64 {
	return math.Asinh(v)
}

func transformAtan(v float64) float64 {
	return math.Atan(v)
}

func transformAtanh(v float64) float64 {
	return math.Atanh(v)
}

func transformAcos(v float64) float64 {
	return math.Acos(v)
}

func transformAcosh(v float64) float64 {
	return math.Acosh(v)
}

func transformDeg(v float64) float64 {
	return v * 180 / math.Pi
}

func transformRad(v float64) float64 {
	return v * math.Pi / 180
}

func newTransformRand(newRandFunc func(r *rand.Rand) func() float64) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if len(args) > 1 {
			return nil, fmt.Errorf(`unexpected number of args; got %d; want 0 or 1`, len(args))
		}
		var seed int64
		if len(args) == 1 {
			tmp, err := getScalar(args[0], 0)
			if err != nil {
				return nil, err
			}
			if len(tmp) > 0 {
				seed = int64(tmp[0])
			}
		} else {
			seed = time.Now().UnixNano()
		}
		source := rand.NewSource(seed)
		r := rand.New(source)
		randFunc := newRandFunc(r)
		tss := evalNumber(tfa.ec, 0)
		values := tss[0].Values
		for i := range values {
			values[i] = randFunc()
		}
		return tss, nil
	}
}

func newRandFloat64(r *rand.Rand) func() float64 {
	return r.Float64
}

func newRandNormFloat64(r *rand.Rand) func() float64 {
	return r.NormFloat64
}

func newRandExpFloat64(r *rand.Rand) func() float64 {
	return r.ExpFloat64
}

func transformPi(tfa *transformFuncArg) ([]*timeseries, error) {
	if err := expectTransformArgsNum(tfa.args, 0); err != nil {
		return nil, err
	}
	return evalNumber(tfa.ec, math.Pi), nil
}

func transformNow(tfa *transformFuncArg) ([]*timeseries, error) {
	if err := expectTransformArgsNum(tfa.args, 0); err != nil {
		return nil, err
	}
	now := float64(time.Now().UnixNano()) / 1e9
	return evalNumber(tfa.ec, now), nil
}

func bitmapAnd(a, b uint64) uint64 {
	return a & b
}

func bitmapOr(a, b uint64) uint64 {
	return a | b
}

func bitmapXor(a, b uint64) uint64 {
	return a ^ b
}

func newTransformBitmap(bitmapFunc func(a, b uint64) uint64) func(tfa *transformFuncArg) ([]*timeseries, error) {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		args := tfa.args
		if err := expectTransformArgsNum(args, 2); err != nil {
			return nil, err
		}
		ns, err := getScalar(args[1], 1)
		if err != nil {
			return nil, err
		}
		tf := func(values []float64) {
			for i, v := range values {
				w := ns[i]
				result := nan
				if !math.IsNaN(v) && !math.IsNaN(w) {
					result = float64(bitmapFunc(uint64(v), uint64(w)))
				}
				values[i] = result
			}
		}
		return doTransformValues(args[0], tf, tfa.fe)
	}
}

func transformTimezoneOffset(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	tzString, err := getString(args[0], 0)
	if err != nil {
		return nil, fmt.Errorf("cannot get timezone name: %w", err)
	}
	loc, err := time.LoadLocation(tzString)
	if err != nil {
		return nil, fmt.Errorf("cannot load timezone %q: %w", tzString, err)
	}

	tss := evalNumber(tfa.ec, nan)
	ts := tss[0]
	for i, timestamp := range ts.Timestamps {
		_, offset := time.Unix(timestamp/1000, 0).In(loc).Zone()
		ts.Values[i] = float64(offset)
	}
	return tss, nil
}

func transformTime(tfa *transformFuncArg) ([]*timeseries, error) {
	if err := expectTransformArgsNum(tfa.args, 0); err != nil {
		return nil, err
	}
	return evalTime(tfa.ec), nil
}

func transformVector(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	return rvs, nil
}

func transformYear(t time.Time) int {
	return t.Year()
}

func newTransformFuncZeroArgs(f func(tfa *transformFuncArg) float64) transformFunc {
	return func(tfa *transformFuncArg) ([]*timeseries, error) {
		if err := expectTransformArgsNum(tfa.args, 0); err != nil {
			return nil, err
		}
		v := f(tfa)
		return evalNumber(tfa.ec, v), nil
	}
}

func transformStep(tfa *transformFuncArg) float64 {
	return float64(tfa.ec.Step) / 1e3
}

func transformStart(tfa *transformFuncArg) float64 {
	return float64(tfa.ec.Start) / 1e3
}

func transformEnd(tfa *transformFuncArg) float64 {
	return float64(tfa.ec.End) / 1e3
}

// copyTimeseries returns a copy of tss.
func copyTimeseries(tss []*timeseries) []*timeseries {
	rvs := make([]*timeseries, len(tss))
	for i, src := range tss {
		var dst timeseries
		dst.CopyFromShallowTimestamps(src)
		rvs[i] = &dst
	}
	return rvs
}

// copyTimeseriesMetricNames returns a copy of tss with real copy of MetricNames,
// but with shallow copy of Timestamps and Values if makeCopy is set.
//
// Otherwise tss is returned.
func copyTimeseriesMetricNames(tss []*timeseries, makeCopy bool) []*timeseries {
	if !makeCopy {
		return tss
	}
	rvs := make([]*timeseries, len(tss))
	for i, src := range tss {
		var dst timeseries
		dst.CopyFromMetricNames(src)
		rvs[i] = &dst
	}
	return rvs
}

// copyTimeseriesShallow returns a copy of src with shallow copies of MetricNames, Timestamps and Values.
func copyTimeseriesShallow(src []*timeseries) []*timeseries {
	tss := make([]timeseries, len(src))
	for i, src := range src {
		tss[i].CopyShallow(src)
	}
	rvs := make([]*timeseries, len(tss))
	for i := range tss {
		rvs[i] = &tss[i]
	}
	return rvs
}

func getDstValue(mn *storage.MetricName, dstLabel string) *[]byte {
	if dstLabel == "__name__" {
		return &mn.MetricGroup
	}
	tags := mn.Tags
	for i := range tags {
		tag := &tags[i]
		if string(tag.Key) == dstLabel {
			return &tag.Value
		}
	}
	if len(tags) < cap(tags) {
		tags = tags[:len(tags)+1]
	} else {
		tags = append(tags, storage.Tag{})
	}
	mn.Tags = tags
	tag := &tags[len(tags)-1]
	tag.Key = append(tag.Key[:0], dstLabel...)
	return &tag.Value
}

func isLeapYear(y uint32) bool {
	if y%4 != 0 {
		return false
	}
	if y%100 != 0 {
		return true
	}
	return y%400 == 0
}

var daysInMonth = [...]int{
	time.January:   31,
	time.February:  28,
	time.March:     31,
	time.April:     30,
	time.May:       31,
	time.June:      30,
	time.July:      31,
	time.August:    31,
	time.September: 30,
	time.October:   31,
	time.November:  30,
	time.December:  31,
}

func expectTransformArgsNum(args [][]*timeseries, expectedNum int) error {
	if len(args) == expectedNum {
		return nil
	}
	return fmt.Errorf(`unexpected number of args; got %d; want %d`, len(args), expectedNum)
}

func removeCounterResetsMaybeNaNs(values []float64) {
	values = skipLeadingNaNs(values)
	if len(values) == 0 {
		return
	}
	var correction float64
	prevValue := values[0]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		d := v - prevValue
		if d < 0 {
			if (-d * 8) < prevValue {
				// This is likely a partial counter reset.
				// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2787
				correction += prevValue - v
			} else {
				correction += prevValue
			}
		}
		prevValue = v
		values[i] = v + correction
	}
}
