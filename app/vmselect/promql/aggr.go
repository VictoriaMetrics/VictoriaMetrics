package promql

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/valyala/histogram"
)

var aggrFuncs = map[string]aggrFunc{
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators
	"sum":          newAggrFunc(aggrFuncSum),
	"min":          newAggrFunc(aggrFuncMin),
	"max":          newAggrFunc(aggrFuncMax),
	"avg":          newAggrFunc(aggrFuncAvg),
	"stddev":       newAggrFunc(aggrFuncStddev),
	"stdvar":       newAggrFunc(aggrFuncStdvar),
	"count":        newAggrFunc(aggrFuncCount),
	"count_values": aggrFuncCountValues,
	"bottomk":      newAggrFuncTopK(true),
	"topk":         newAggrFuncTopK(false),
	"quantile":     aggrFuncQuantile,

	// PromQL extension funcs
	"median":         aggrFuncMedian,
	"limitk":         aggrFuncLimitK,
	"distinct":       newAggrFunc(aggrFuncDistinct),
	"sum2":           newAggrFunc(aggrFuncSum2),
	"geomean":        newAggrFunc(aggrFuncGeomean),
	"histogram":      newAggrFunc(aggrFuncHistogram),
	"topk_min":       newAggrFuncRangeTopK(minValue, false),
	"topk_max":       newAggrFuncRangeTopK(maxValue, false),
	"topk_avg":       newAggrFuncRangeTopK(avgValue, false),
	"topk_median":    newAggrFuncRangeTopK(medianValue, false),
	"bottomk_min":    newAggrFuncRangeTopK(minValue, true),
	"bottomk_max":    newAggrFuncRangeTopK(maxValue, true),
	"bottomk_avg":    newAggrFuncRangeTopK(avgValue, true),
	"bottomk_median": newAggrFuncRangeTopK(medianValue, true),
	"any":            newAggrFunc(aggrFuncAny),
	"outliersk":      aggrFuncOutliersK,
}

type aggrFunc func(afa *aggrFuncArg) ([]*timeseries, error)

type aggrFuncArg struct {
	args [][]*timeseries
	ae   *metricsql.AggrFuncExpr
	ec   *EvalConfig
}

func getAggrFunc(s string) aggrFunc {
	s = strings.ToLower(s)
	return aggrFuncs[s]
}

func newAggrFunc(afe func(tss []*timeseries) []*timeseries) aggrFunc {
	return func(afa *aggrFuncArg) ([]*timeseries, error) {
		args := afa.args
		if err := expectTransformArgsNum(args, 1); err != nil {
			return nil, err
		}
		return aggrFuncExt(afe, args[0], &afa.ae.Modifier, afa.ae.Limit, false)
	}
}

func removeGroupTags(metricName *storage.MetricName, modifier *metricsql.ModifierExpr) {
	groupOp := strings.ToLower(modifier.Op)
	switch groupOp {
	case "", "by":
		metricName.RemoveTagsOn(modifier.Args)
	case "without":
		metricName.RemoveTagsIgnoring(modifier.Args)
	default:
		logger.Panicf("BUG: unknown group modifier: %q", groupOp)
	}
}

func aggrFuncExt(afe func(tss []*timeseries) []*timeseries, argOrig []*timeseries, modifier *metricsql.ModifierExpr, maxSeries int, keepOriginal bool) ([]*timeseries, error) {
	arg := copyTimeseriesMetricNames(argOrig)

	// Perform grouping.
	m := make(map[string][]*timeseries)
	bb := bbPool.Get()
	for i, ts := range arg {
		removeGroupTags(&ts.MetricName, modifier)
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		if keepOriginal {
			ts = argOrig[i]
		}
		tss := m[string(bb.B)]
		if tss == nil && maxSeries > 0 && len(m) >= maxSeries {
			// We already reached time series limit after grouping. Skip other time series.
			continue
		}
		tss = append(tss, ts)
		m[string(bb.B)] = tss
	}
	bbPool.Put(bb)

	srcTssCount := 0
	dstTssCount := 0
	rvs := make([]*timeseries, 0, len(m))
	for _, tss := range m {
		rv := afe(tss)
		rvs = append(rvs, rv...)
		srcTssCount += len(tss)
		dstTssCount += len(rv)
		if dstTssCount > 2000 && dstTssCount > 16*srcTssCount {
			// This looks like count_values explosion.
			return nil, fmt.Errorf(`too many timeseries after aggragation; got %d; want less than %d`, dstTssCount, 16*srcTssCount)
		}
	}
	return rvs, nil
}

func aggrFuncAny(tss []*timeseries) []*timeseries {
	return tss[:1]
}

func aggrFuncSum(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - nothing to sum.
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		sum := float64(0)
		count := 0
		for _, ts := range tss {
			if math.IsNaN(ts.Values[i]) {
				continue
			}
			sum += ts.Values[i]
			count++
		}
		if count == 0 {
			sum = nan
		}
		dst.Values[i] = sum
	}
	return tss[:1]
}

func aggrFuncSum2(tss []*timeseries) []*timeseries {
	dst := tss[0]
	for i := range dst.Values {
		sum2 := float64(0)
		count := 0
		for _, ts := range tss {
			v := ts.Values[i]
			if math.IsNaN(v) {
				continue
			}
			sum2 += v * v
			count++
		}
		if count == 0 {
			sum2 = nan
		}
		dst.Values[i] = sum2
	}
	return tss[:1]
}

func aggrFuncGeomean(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - nothing to geomean.
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		p := 1.0
		count := 0
		for _, ts := range tss {
			v := ts.Values[i]
			if math.IsNaN(v) {
				continue
			}
			p *= v
			count++
		}
		if count == 0 {
			p = nan
		}
		dst.Values[i] = math.Pow(p, 1/float64(count))
	}
	return tss[:1]
}

func aggrFuncHistogram(tss []*timeseries) []*timeseries {
	var h metrics.Histogram
	m := make(map[string]*timeseries)
	for i := range tss[0].Values {
		h.Reset()
		for _, ts := range tss {
			v := ts.Values[i]
			h.Update(v)
		}
		h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
			ts := m[vmrange]
			if ts == nil {
				ts = &timeseries{}
				ts.CopyFromShallowTimestamps(tss[0])
				ts.MetricName.RemoveTag("vmrange")
				ts.MetricName.AddTag("vmrange", vmrange)
				values := ts.Values
				for k := range values {
					values[k] = 0
				}
				m[vmrange] = ts
			}
			ts.Values[i] = float64(count)
		})
	}
	rvs := make([]*timeseries, 0, len(m))
	for _, ts := range m {
		rvs = append(rvs, ts)
	}
	return vmrangeBucketsToLE(rvs)
}

func aggrFuncMin(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - nothing to min.
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		min := dst.Values[i]
		for _, ts := range tss {
			if math.IsNaN(min) || ts.Values[i] < min {
				min = ts.Values[i]
			}
		}
		dst.Values[i] = min
	}
	return tss[:1]
}

func aggrFuncMax(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - nothing to max.
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		max := dst.Values[i]
		for _, ts := range tss {
			if math.IsNaN(max) || ts.Values[i] > max {
				max = ts.Values[i]
			}
		}
		dst.Values[i] = max
	}
	return tss[:1]
}

func aggrFuncAvg(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - nothing to avg.
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		// Do not use `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation,
		// since it is slower and has no obvious benefits in increased precision.
		var sum float64
		count := 0
		for _, ts := range tss {
			v := ts.Values[i]
			if math.IsNaN(v) {
				continue
			}
			count++
			sum += v
		}
		avg := nan
		if count > 0 {
			avg = sum / float64(count)
		}
		dst.Values[i] = avg
	}
	return tss[:1]
}

func aggrFuncStddev(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - stddev over a single time series is zero
		values := tss[0].Values
		for i, v := range values {
			if !math.IsNaN(v) {
				values[i] = 0
			}
		}
		return tss
	}
	rvs := aggrFuncStdvar(tss)
	dst := rvs[0]
	for i, v := range dst.Values {
		dst.Values[i] = math.Sqrt(v)
	}
	return rvs
}

func aggrFuncStdvar(tss []*timeseries) []*timeseries {
	if len(tss) == 1 {
		// Fast path - stdvar over a single time series is zero
		values := tss[0].Values
		for i, v := range values {
			if !math.IsNaN(v) {
				values[i] = 0
			}
		}
		return tss
	}
	dst := tss[0]
	for i := range dst.Values {
		// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
		var avg float64
		var count float64
		var q float64
		for _, ts := range tss {
			v := ts.Values[i]
			if math.IsNaN(v) {
				continue
			}
			count++
			avgNew := avg + (v-avg)/count
			q += (v - avg) * (v - avgNew)
			avg = avgNew
		}
		if count == 0 {
			q = nan
		}
		dst.Values[i] = q / count
	}
	return tss[:1]
}

func aggrFuncCount(tss []*timeseries) []*timeseries {
	dst := tss[0]
	for i := range dst.Values {
		count := 0
		for _, ts := range tss {
			if math.IsNaN(ts.Values[i]) {
				continue
			}
			count++
		}
		v := float64(count)
		if count == 0 {
			v = nan
		}
		dst.Values[i] = v
	}
	return tss[:1]
}

func aggrFuncDistinct(tss []*timeseries) []*timeseries {
	dst := tss[0]
	m := make(map[float64]struct{}, len(tss))
	for i := range dst.Values {
		for _, ts := range tss {
			v := ts.Values[i]
			if math.IsNaN(v) {
				continue
			}
			m[v] = struct{}{}
		}
		n := float64(len(m))
		if n == 0 {
			n = nan
		}
		dst.Values[i] = n
		for k := range m {
			delete(m, k)
		}
	}
	return tss[:1]
}

func aggrFuncCountValues(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	dstLabel, err := getString(args[0], 0)
	if err != nil {
		return nil, err
	}

	// Remove dstLabel from grouping like Prometheus does.
	modifier := &afa.ae.Modifier
	switch strings.ToLower(modifier.Op) {
	case "without":
		modifier.Args = append(modifier.Args, dstLabel)
	case "by":
		dstArgs := modifier.Args[:0]
		for _, arg := range modifier.Args {
			if arg == dstLabel {
				continue
			}
			dstArgs = append(dstArgs, arg)
		}
		modifier.Args = dstArgs
	default:
		// Do nothing
	}

	afe := func(tss []*timeseries) []*timeseries {
		m := make(map[float64]bool)
		for _, ts := range tss {
			for _, v := range ts.Values {
				if math.IsNaN(v) {
					continue
				}
				m[v] = true
			}
		}
		values := make([]float64, 0, len(m))
		for v := range m {
			values = append(values, v)
		}
		sort.Float64s(values)

		var rvs []*timeseries
		for _, v := range values {
			var dst timeseries
			dst.CopyFromShallowTimestamps(tss[0])
			dst.MetricName.RemoveTag(dstLabel)
			dst.MetricName.AddTag(dstLabel, strconv.FormatFloat(v, 'g', -1, 64))
			for i := range dst.Values {
				count := 0
				for _, ts := range tss {
					if ts.Values[i] == v {
						count++
					}
				}
				n := float64(count)
				if n == 0 {
					n = nan
				}
				dst.Values[i] = n
			}
			rvs = append(rvs, &dst)
		}
		return rvs
	}
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, false)
}

func newAggrFuncTopK(isReverse bool) aggrFunc {
	return func(afa *aggrFuncArg) ([]*timeseries, error) {
		args := afa.args
		if err := expectTransformArgsNum(args, 2); err != nil {
			return nil, err
		}
		ks, err := getScalar(args[0], 0)
		if err != nil {
			return nil, err
		}
		afe := func(tss []*timeseries) []*timeseries {
			for n := range tss[0].Values {
				sort.Slice(tss, func(i, j int) bool {
					a := tss[i].Values[n]
					b := tss[j].Values[n]
					if isReverse {
						a, b = b, a
					}
					return lessWithNaNs(a, b)
				})
				fillNaNsAtIdx(n, ks[n], tss)
			}
			return removeNaNs(tss)
		}
		return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, true)
	}
}

func newAggrFuncRangeTopK(f func(values []float64) float64, isReverse bool) aggrFunc {
	return func(afa *aggrFuncArg) ([]*timeseries, error) {
		args := afa.args
		if err := expectTransformArgsNum(args, 2); err != nil {
			return nil, err
		}
		ks, err := getScalar(args[0], 0)
		if err != nil {
			return nil, err
		}
		afe := func(tss []*timeseries) []*timeseries {
			return getRangeTopKTimeseries(tss, ks, f, isReverse)
		}
		return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, true)
	}
}

func getRangeTopKTimeseries(tss []*timeseries, ks []float64, f func(values []float64) float64, isReverse bool) []*timeseries {
	type tsWithValue struct {
		ts    *timeseries
		value float64
	}
	maxs := make([]tsWithValue, len(tss))
	for i, ts := range tss {
		value := f(ts.Values)
		maxs[i] = tsWithValue{
			ts:    ts,
			value: value,
		}
	}
	sort.Slice(maxs, func(i, j int) bool {
		a := maxs[i].value
		b := maxs[j].value
		if isReverse {
			a, b = b, a
		}
		return lessWithNaNs(a, b)
	})
	for i := range maxs {
		tss[i] = maxs[i].ts
	}
	for i, k := range ks {
		fillNaNsAtIdx(i, k, tss)
	}
	return removeNaNs(tss)
}

func fillNaNsAtIdx(idx int, k float64, tss []*timeseries) {
	if math.IsNaN(k) {
		k = 0
	}
	kn := int(k)
	if kn < 0 {
		kn = 0
	}
	if kn > len(tss) {
		kn = len(tss)
	}
	for _, ts := range tss[:len(tss)-kn] {
		ts.Values[idx] = nan
	}
}

func minValue(values []float64) float64 {
	if len(values) == 0 {
		return nan
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return nan
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func avgValue(values []float64) float64 {
	sum := float64(0)
	count := 0
	for _, v := range values {
		if math.IsNaN(v) {
			continue
		}
		count++
		sum += v
	}
	if count == 0 {
		return nan
	}
	return sum / float64(count)
}

func medianValue(values []float64) float64 {
	h := histogram.GetFast()
	for _, v := range values {
		if !math.IsNaN(v) {
			h.Update(v)
		}
	}
	value := h.Quantile(0.5)
	histogram.PutFast(h)
	return value
}

func aggrFuncOutliersK(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	ks, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	afe := func(tss []*timeseries) []*timeseries {
		// Calculate medians for each point across tss.
		medians := make([]float64, len(ks))
		h := histogram.GetFast()
		for n := range ks {
			h.Reset()
			for j := range tss {
				v := tss[j].Values[n]
				if !math.IsNaN(v) {
					h.Update(v)
				}
			}
			medians[n] = h.Quantile(0.5)
		}
		histogram.PutFast(h)

		// Return topK time series with the highest variance from median.
		f := func(values []float64) float64 {
			sum2 := float64(0)
			for n, v := range values {
				d := v - medians[n]
				sum2 += d * d
			}
			return sum2
		}
		return getRangeTopKTimeseries(tss, ks, f, false)
	}
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, true)
}

func aggrFuncLimitK(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	ks, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	maxK := 0
	for _, kf := range ks {
		k := int(kf)
		if k > maxK {
			maxK = k
		}
	}
	afe := func(tss []*timeseries) []*timeseries {
		if len(tss) > maxK {
			tss = tss[:maxK]
		}
		for i, kf := range ks {
			k := int(kf)
			if k < 0 {
				k = 0
			}
			for j := k; j < len(tss); j++ {
				tss[j].Values[i] = nan
			}
		}
		return tss
	}
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, true)
}

func aggrFuncQuantile(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 2); err != nil {
		return nil, err
	}
	phis, err := getScalar(args[0], 0)
	if err != nil {
		return nil, err
	}
	afe := newAggrQuantileFunc(phis)
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, afa.ae.Limit, false)
}

func aggrFuncMedian(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	phis := evalNumber(afa.ec, 0.5)[0].Values
	afe := newAggrQuantileFunc(phis)
	return aggrFuncExt(afe, args[0], &afa.ae.Modifier, afa.ae.Limit, false)
}

func newAggrQuantileFunc(phis []float64) func(tss []*timeseries) []*timeseries {
	return func(tss []*timeseries) []*timeseries {
		dst := tss[0]
		h := histogram.GetFast()
		defer histogram.PutFast(h)
		for n := range dst.Values {
			h.Reset()
			for j := range tss {
				v := tss[j].Values[n]
				if !math.IsNaN(v) {
					h.Update(v)
				}
			}
			phi := phis[n]
			dst.Values[n] = h.Quantile(phi)
		}
		tss[0] = dst
		return tss[:1]
	}
}

func lessWithNaNs(a, b float64) bool {
	if math.IsNaN(a) {
		return !math.IsNaN(b)
	}
	return a < b
}
