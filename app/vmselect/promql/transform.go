package promql

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/valyala/histogram"
)

var transformFuncsKeepMetricGroup = map[string]bool{
	"ceil":      true,
	"clamp_max": true,
	"clamp_min": true,
	"floor":     true,
	"round":     true,
}

var transformFuncs = map[string]transformFunc{
	// Standard promql funcs
	// See funcs accepting instant-vector on https://prometheus.io/docs/prometheus/latest/querying/functions/ .
	"abs":                newTransformFuncOneArg(transformAbs),
	"absent":             transformAbsent,
	"ceil":               newTransformFuncOneArg(transformCeil),
	"clamp_max":          transformClampMax,
	"clamp_min":          transformClampMin,
	"day_of_month":       newTransformFuncDateTime(transformDayOfMonth),
	"day_of_week":        newTransformFuncDateTime(transformDayOfWeek),
	"days_in_month":      newTransformFuncDateTime(transformDaysInMonth),
	"exp":                newTransformFuncOneArg(transformExp),
	"floor":              newTransformFuncOneArg(transformFloor),
	"histogram_quantile": transformHistogramQuantile,
	"hour":               newTransformFuncDateTime(transformHour),
	"label_join":         transformLabelJoin,
	"label_replace":      transformLabelReplace,
	"ln":                 newTransformFuncOneArg(transformLn),
	"log2":               newTransformFuncOneArg(transformLog2),
	"log10":              newTransformFuncOneArg(transformLog10),
	"minute":             newTransformFuncDateTime(transformMinute),
	"month":              newTransformFuncDateTime(transformMonth),
	"round":              transformRound,
	"scalar":             transformScalar,
	"sort":               newTransformFuncSort(false),
	"sort_desc":          newTransformFuncSort(true),
	"sqrt":               newTransformFuncOneArg(transformSqrt),
	"time":               transformTime,
	"timestamp":          transformTimestamp,
	"vector":             transformVector,
	"year":               newTransformFuncDateTime(transformYear),

	// New funcs
	"label_set":          transformLabelSet,
	"label_del":          transformLabelDel,
	"label_keep":         transformLabelKeep,
	"label_copy":         transformLabelCopy,
	"label_move":         transformLabelMove,
	"label_transform":    transformLabelTransform,
	"label_value":        transformLabelValue,
	"union":              transformUnion,
	"":                   transformUnion, // empty func is a synonim to union
	"keep_last_value":    transformKeepLastValue,
	"start":              newTransformFuncZeroArgs(transformStart),
	"end":                newTransformFuncZeroArgs(transformEnd),
	"step":               newTransformFuncZeroArgs(transformStep),
	"running_sum":        newTransformFuncRunning(runningSum),
	"running_max":        newTransformFuncRunning(runningMax),
	"running_min":        newTransformFuncRunning(runningMin),
	"running_avg":        newTransformFuncRunning(runningAvg),
	"range_sum":          newTransformFuncRange(runningSum),
	"range_max":          newTransformFuncRange(runningMax),
	"range_min":          newTransformFuncRange(runningMin),
	"range_avg":          newTransformFuncRange(runningAvg),
	"range_first":        transformRangeFirst,
	"range_last":         transformRangeLast,
	"range_quantile":     transformRangeQuantile,
	"smooth_exponential": transformSmoothExponential,
	"remove_resets":      transformRemoveResets,
	"rand":               newTransformRand(newRandFloat64),
	"rand_normal":        newTransformRand(newRandNormFloat64),
	"rand_exponential":   newTransformRand(newRandExpFloat64),
	"pi":                 transformPi,
	"sin":                newTransformFuncOneArg(transformSin),
	"cos":                newTransformFuncOneArg(transformCos),
	"asin":               newTransformFuncOneArg(transformAsin),
	"acos":               newTransformFuncOneArg(transformAcos),
	"prometheus_buckets": transformPrometheusBuckets,
}

func getTransformFunc(s string) transformFunc {
	s = strings.ToLower(s)
	return transformFuncs[s]
}

func isTransformFunc(s string) bool {
	return getTransformFunc(s) != nil
}

type transformFuncArg struct {
	ec   *EvalConfig
	fe   *funcExpr
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

func doTransformValues(arg []*timeseries, tf func(values []float64), fe *funcExpr) ([]*timeseries, error) {
	name := strings.ToLower(fe.Name)
	keepMetricGroup := transformFuncsKeepMetricGroup[name]
	for _, ts := range arg {
		if !keepMetricGroup {
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
	arg := args[0]

	if len(arg) == 0 {
		// Copy tags from arg
		rvs := evalNumber(tfa.ec, 1)
		rv := rvs[0]
		me, ok := tfa.fe.Args[0].(*metricExpr)
		if !ok {
			return rvs, nil
		}
		for i := range me.TagFilters {
			tf := &me.TagFilters[i]
			if len(tf.Key) == 0 {
				continue
			}
			if tf.IsRegexp || tf.IsNegative {
				continue
			}
			rv.MetricName.AddTagBytes(tf.Key, tf.Value)
		}
		return rvs, nil
	}

	for _, ts := range arg {
		ts.MetricName.ResetMetricGroup()
		for i, v := range ts.Values {
			if !math.IsNaN(v) {
				v = nan
			} else {
				v = 1
			}
			ts.Values[i] = v
		}
	}
	return arg, nil
}

func transformCeil(v float64) float64 {
	return math.Ceil(v)
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
				t := time.Unix(int64(v), 0).UTC()
				values[i] = float64(f(t))
			}
		}
		return doTransformValues(arg, tf, tfa.fe)
	}
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
		m[string(bb.B)] = append(m[string(bb.B)], x{
			startStr: startStr,
			endStr:   endStr,
			start:    start,
			end:      end,
			ts:       ts,
		})
	}

	// Convert `vmrange` label in each group of time series to `le` label.
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool { return xss[i].end < xss[j].end })
		xssNew := make([]x, 0, len(xss))
		endStrPrev := "0"
		for _, xs := range xss {
			ts := xs.ts
			if xs.startStr != endStrPrev {
				var tsDummy timeseries
				tsDummy.CopyFromShallowTimestamps(ts)
				values := tsDummy.Values
				for i := range values {
					values[i] = 0
				}
				tsDummy.MetricName.AddTag("le", xs.startStr)
				xssNew = append(xssNew, x{
					endStr: xs.startStr,
					end:    xs.start,
					ts:     &tsDummy,
				})
			}
			ts.MetricName.AddTag("le", xs.endStr)
			xssNew = append(xssNew, xs)
			endStrPrev = xs.endStr
		}
		xss = xssNew
		for i := range xss[0].ts.Values {
			count := float64(0)
			for _, xs := range xss {
				ts := xs.ts
				v := ts.Values[i]
				if !math.IsNaN(v) {
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

func transformHistogramQuantile(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}

	// Convert buckets with `vmrange` labels to buckets with `le` labels.
	tss := vmrangeBucketsToLE(args[1])

	// Group metrics by all tags excluding "le"
	type x struct {
		le float64
		ts *timeseries
	}
	m := make(map[string][]x)
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
		m[string(bb.B)] = append(m[string(bb.B)], x{
			le: le,
			ts: ts,
		})
	}
	bbPool.Put(bb)

	// Calculate quantile for each group in m

	lastNonInf := func(i int, xss []x) float64 {
		for len(xss) > 0 {
			xsLast := xss[len(xss)-1]
			v := xsLast.ts.Values[i]
			if v == 0 {
				return nan
			}
			if !math.IsNaN(v) && !math.IsInf(xsLast.le, 0) {
				return xsLast.le
			}
			xss = xss[:len(xss)-1]
		}
		return nan
	}
	quantile := func(i int, phis []float64, xss []x) float64 {
		phi := phis[i]
		if math.IsNaN(phi) {
			return nan
		}
		// Fix broken buckets.
		// They are already sorted by le, so their values must be in ascending order,
		// since the next bucket value includes all the previous buckets.
		vPrev := float64(0)
		for _, xs := range xss {
			v := xs.ts.Values[i]
			if v < vPrev {
				xs.ts.Values[i] = vPrev
			} else if !math.IsNaN(v) {
				vPrev = v
			}
		}
		if len(xss) == 0 {
			return nan
		}
		if phi < 0 {
			return -inf
		}
		if phi > 1 {
			return inf
		}
		vLast := xss[len(xss)-1].ts.Values[i]
		if vLast == 0 {
			return nan
		}
		vReq := vLast * phi
		vPrev = 0
		lePrev := float64(0)
		for _, xs := range xss {
			v := xs.ts.Values[i]
			if math.IsNaN(v) {
				// Skip NaNs - they may appear if the selected time range
				// contains multiple different bucket sets.
				continue
			}
			le := xs.le
			if v < vReq {
				vPrev = v
				lePrev = le
				continue
			}
			if math.IsInf(le, 0) {
				return lastNonInf(i, xss)
			}
			if v == vPrev {
				return lePrev
			}
			return lePrev + (le-lePrev)*(vReq-vPrev)/(v-vPrev)
		}
		return lastNonInf(i, xss)
	}
	rvs := make([]*timeseries, 0, len(m))
	for _, xss := range m {
		sort.Slice(xss, func(i, j int) bool {
			return xss[i].le < xss[j].le
		})
		dst := xss[0].ts
		for i := range dst.Values {
			dst.Values[i] = quantile(i, phis, xss)
		}
		rvs = append(rvs, dst)
	}
	return rvs, nil
}

func transformHour(t time.Time) int {
	return t.Hour()
}

func runningSum(a, b float64, idx int) float64 {
	return a + b
}

func runningMax(a, b float64, idx int) float64 {
	if a > b {
		return a
	}
	return b
}

func runningMin(a, b float64, idx int) float64 {
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
		prevValue := values[0]
		for i, v := range values {
			if math.IsNaN(v) {
				v = prevValue
			}
			values[i] = v
			prevValue = v
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
				if math.IsNaN(v) {
					continue
				}
				prevValue = rf(prevValue, v, i+1)
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

func transformRangeQuantile(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	if len(phis) == 0 {
		return nil, nil
	}
	phi := phis[0]
	rvs := args[1]
	hf := histogram.GetFast()
	for _, ts := range rvs {
		hf.Reset()
		lastIdx := -1
		values := ts.Values
		if len(values) > 0 {
			// Ignore the last value. See Exec func for details.
			values = values[:len(values)-1]
		}
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
			hf.Update(v)
			lastIdx = i
		}
		if lastIdx >= 0 {
			values[lastIdx] = hf.Quantile(phi)
		}
	}
	histogram.PutFast(hf)
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
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
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
		values := ts.Values
		if len(values) < 2 {
			continue
		}
		// Do not take into account the last value, since it shouldn't be included
		// in the range. See Exec func for details.
		values = values[:len(values)-1]
		values = skipTrailingNaNs(values)
		if len(values) == 0 {
			continue
		}
		vLast := values[len(values)-1]
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
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

	rvs := make([]*timeseries, 0, len(args[0]))
	m := make(map[string]bool, len(args[0]))
	bb := bbPool.Get()
	for _, arg := range args {
		for _, ts := range arg {
			bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
			if m[string(bb.B)] {
				continue
			}
			m[string(bb.B)] = true
			rvs = append(rvs, ts)
		}
	}
	bbPool.Put(bb)
	return rvs, nil
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
			dstValue := getDstValue(mn, dstLabel)
			*dstValue = append((*dstValue)[:0], value...)
			if len(value) == 0 {
				mn.RemoveTag(dstLabel)
			}
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
		b := *dstValue
		b = b[:0]
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

	r, err := compileRegexp(regex)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regex %q: %s`, regex, err)
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

	r, err := compileRegexpAnchored(regex)
	if err != nil {
		return nil, fmt.Errorf(`cannot compile regex %q: %s`, regex, err)
	}
	return labelReplace(args[0], srcLabel, r, dstLabel, replacement)
}

func labelReplace(tss []*timeseries, srcLabel string, r *regexp.Regexp, dstLabel, replacement string) ([]*timeseries, error) {
	replacementBytes := []byte(replacement)
	for _, ts := range tss {
		mn := &ts.MetricName
		dstValue := getDstValue(mn, dstLabel)
		srcValue := mn.GetTagValue(srcLabel)
		b := r.ReplaceAll(srcValue, replacementBytes)
		*dstValue = append((*dstValue)[:0], b...)
		if len(b) == 0 {
			mn.RemoveTag(dstLabel)
		}
	}
	return tss, nil
}

func transformLabelValue(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	labelName, err := getString(args[1], 1)
	if err != nil {
		return nil, fmt.Errorf("cannot get label name: %s", err)
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
		for i := range values {
			values[i] = v
		}
	}
	// Do not remove timeseries with only NaN values, so `default` could be applied to them:
	// label_value(q, "label") default 123
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

func transformScalar(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}

	// Verify whether the arg is a string.
	// Then try converting the string to number.
	if se, ok := tfa.fe.Args[0].(*stringExpr); ok {
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
				if !math.IsNaN(a[n]) && !math.IsNaN(b[n]) {
					break
				}
				n--
			}
			if n < 0 {
				return false
			}
			cmp := a[n] < b[n]
			if isDesc {
				cmp = !cmp
			}
			return cmp
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

func transformCos(v float64) float64 {
	return math.Cos(v)
}

func transformAsin(v float64) float64 {
	return math.Asin(v)
}

func transformAcos(v float64) float64 {
	return math.Acos(v)
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
			seed = int64(tmp[0])
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

func transformTime(tfa *transformFuncArg) ([]*timeseries, error) {
	if err := expectTransformArgsNum(tfa.args, 0); err != nil {
		return nil, err
	}
	return evalTime(tfa.ec), nil
}

func transformTimestamp(tfa *transformFuncArg) ([]*timeseries, error) {
	args := tfa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	rvs := args[0]
	for _, ts := range rvs {
		ts.MetricName.ResetMetricGroup()
		values := ts.Values
		for i, t := range ts.Timestamps {
			v := values[i]
			if !math.IsNaN(v) {
				values[i] = float64(t) / 1e3
			}
		}
	}
	return rvs, nil
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
	return float64(tfa.ec.Step) * 1e-3
}

func transformStart(tfa *transformFuncArg) float64 {
	return float64(tfa.ec.Start) * 1e-3
}

func transformEnd(tfa *transformFuncArg) float64 {
	// Subtract step from end, since it shouldn't go to the range.
	// See Exec func for details.
	return float64(tfa.ec.End-tfa.ec.Step) * 1e-3
}

// copyTimeseriesMetricNames returns a copy of arg with real copy of MetricNames,
// but with shallow copy of Timestamps and Values.
func copyTimeseriesMetricNames(arg []*timeseries) []*timeseries {
	rvs := make([]*timeseries, len(arg))
	for i, src := range arg {
		var dst timeseries
		dst.CopyFromMetricNames(src)
		rvs[i] = &dst
	}
	return rvs
}

// copyShallow returns a copy of arg with shallow copies of MetricNames,
// Timestamps and Values.
func copyTimeseriesShallow(arg []*timeseries) []*timeseries {
	rvs := make([]*timeseries, len(arg))
	for i, src := range arg {
		var dst timeseries
		dst.CopyShallow(src)
		rvs[i] = &dst
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
