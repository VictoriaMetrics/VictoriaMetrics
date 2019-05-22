package promql

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
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

	// Extended PromQL funcs
	"median":   aggrFuncMedian,
	"limitk":   aggrFuncLimitK,
	"distinct": newAggrFunc(aggrFuncDistinct),
}

type aggrFunc func(afa *aggrFuncArg) ([]*timeseries, error)

type aggrFuncArg struct {
	args [][]*timeseries
	ae   *aggrFuncExpr
	ec   *EvalConfig
}

func getAggrFunc(s string) aggrFunc {
	s = strings.ToLower(s)
	return aggrFuncs[s]
}

func isAggrFunc(s string) bool {
	return getAggrFunc(s) != nil
}

func isAggrFuncModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	case "by", "without":
		return true
	default:
		return false
	}
}

func newAggrFunc(afe func(tss []*timeseries) []*timeseries) aggrFunc {
	return func(afa *aggrFuncArg) ([]*timeseries, error) {
		args := afa.args
		if err := expectTransformArgsNum(args, 1); err != nil {
			return nil, err
		}
		return aggrFuncExt(afe, args[0], &afa.ae.Modifier, false)
	}
}

func aggrFuncExt(afe func(tss []*timeseries) []*timeseries, argOrig []*timeseries, modifier *modifierExpr, keepOriginal bool) ([]*timeseries, error) {
	arg := copyTimeseriesMetricNames(argOrig)

	// Filter out superflouos tags.
	var groupTags []string
	groupOp := "by"
	if modifier.Op != "" {
		groupTags = modifier.Args
		groupOp = strings.ToLower(modifier.Op)
	}
	switch groupOp {
	case "by":
		for _, ts := range arg {
			ts.MetricName.RemoveTagsOn(groupTags)
		}
	case "without":
		for _, ts := range arg {
			ts.MetricName.RemoveTagsIgnoring(groupTags)
		}
	default:
		return nil, fmt.Errorf(`unknown modifier: %q`, groupOp)
	}

	// Perform grouping.
	m := make(map[string][]*timeseries)
	bb := bbPool.Get()
	for i, ts := range arg {
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		if keepOriginal {
			ts = argOrig[i]
		}
		m[string(bb.B)] = append(m[string(bb.B)], ts)
	}
	bbPool.Put(bb)

	rvs := make([]*timeseries, 0, len(m))
	for _, tss := range m {
		rv := afe(tss)
		rvs = append(rvs, rv...)
	}
	return rvs, nil
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
		dst.Values[i] = float64(count)
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
	afe := func(tss []*timeseries) []*timeseries {
		m := make(map[float64]bool)
		for _, ts := range tss {
			for _, v := range ts.Values {
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
			dst.CopyFrom(tss[0])
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
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, false)
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
			rvs := tss
			for n := range rvs[0].Values {
				sort.Slice(rvs, func(i, j int) bool {
					a := rvs[i].Values[n]
					b := rvs[j].Values[n]
					cmp := lessWithNaNs(a, b)
					if isReverse {
						cmp = !cmp
					}
					return cmp
				})
				if math.IsNaN(ks[n]) {
					ks[n] = 0
				}
				k := int(ks[n])
				if k < 0 {
					k = 0
				}
				if k > len(rvs) {
					k = len(rvs)
				}
				for _, ts := range rvs[:len(rvs)-k] {
					ts.Values[n] = nan
				}
			}
			return rvs
		}
		return aggrFuncExt(afe, args[1], &afa.ae.Modifier, true)
	}
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
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, true)
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
	return aggrFuncExt(afe, args[1], &afa.ae.Modifier, false)
}

func aggrFuncMedian(afa *aggrFuncArg) ([]*timeseries, error) {
	args := afa.args
	if err := expectTransformArgsNum(args, 1); err != nil {
		return nil, err
	}
	phis := evalNumber(afa.ec, 0.5)[0].Values
	afe := newAggrQuantileFunc(phis)
	return aggrFuncExt(afe, args[0], &afa.ae.Modifier, false)
}

func newAggrQuantileFunc(phis []float64) func(tss []*timeseries) []*timeseries {
	return func(tss []*timeseries) []*timeseries {
		dst := tss[0]
		for n := range dst.Values {
			sort.Slice(tss, func(i, j int) bool {
				a := tss[i].Values[n]
				b := tss[j].Values[n]
				return lessWithNaNs(a, b)
			})
			phi := phis[n]
			if math.IsNaN(phi) {
				phi = 1
			}
			if phi < 0 {
				phi = 0
			}
			if phi > 1 {
				phi = 1
			}
			idx := int(math.Round(float64(len(tss)-1) * phi))
			dst.Values[n] = tss[idx].Values[n]
		}
		return tss[:1]
	}
}

func lessWithNaNs(a, b float64) bool {
	if math.IsNaN(a) {
		return !math.IsNaN(b)
	}
	return a < b
}
