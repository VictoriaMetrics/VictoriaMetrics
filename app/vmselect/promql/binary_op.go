package promql

import (
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/VictoriaMetrics/metricsql/binaryop"
)

var binaryOpFuncs = map[string]binaryOpFunc{
	"+": newBinaryOpArithFunc(binaryop.Plus),
	"-": newBinaryOpArithFunc(binaryop.Minus),
	"*": newBinaryOpArithFunc(binaryop.Mul),
	"/": newBinaryOpArithFunc(binaryop.Div),
	"%": newBinaryOpArithFunc(binaryop.Mod),
	"^": newBinaryOpArithFunc(binaryop.Pow),

	// See https://github.com/prometheus/prometheus/pull/9248
	"atan2": newBinaryOpArithFunc(binaryop.Atan2),

	// cmp ops
	"==": binaryOpEqFunc,
	"!=": binaryOpNeqFunc,
	">":  newBinaryOpCmpFunc(binaryop.Gt),
	"<":  newBinaryOpCmpFunc(binaryop.Lt),
	">=": newBinaryOpCmpFunc(binaryop.Gte),
	"<=": newBinaryOpCmpFunc(binaryop.Lte),

	// logical set ops
	"and":    binaryOpAnd,
	"or":     binaryOpOr,
	"unless": binaryOpUnless,

	// New ops
	"if":      binaryOpIf,
	"ifnot":   binaryOpIfnot,
	"default": binaryOpDefault,
}

func getBinaryOpFunc(op string) binaryOpFunc {
	op = strings.ToLower(op)
	return binaryOpFuncs[op]
}

type binaryOpFuncArg struct {
	be    *metricsql.BinaryOpExpr
	left  []*timeseries
	right []*timeseries
}

type binaryOpFunc func(bfa *binaryOpFuncArg) ([]*timeseries, error)

func binaryOpEqFunc(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	if !isUnionFunc(bfa.be.Left) && !isUnionFunc(bfa.be.Right) {
		return binaryOpEqStdFunc(bfa)
	}

	// Special case for `q == (1,2,3)`
	left := bfa.left
	right := bfa.right
	if isUnionFunc(bfa.be.Left) {
		left, right = right, left
	}
	if len(left) == 0 || len(right) == 0 {
		return nil, nil
	}
	for _, tsLeft := range left {
		values := tsLeft.Values
		for j, v := range values {
			if !containsValueAt(right, v, j) {
				values[j] = nan
			}
		}
	}
	// Do not remove time series containing only NaNs, since then the `(foo op bar) default N`
	// won't work as expected if `(foo op bar)` results to NaN series.
	return left, nil
}

func binaryOpNeqFunc(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	if !isUnionFunc(bfa.be.Left) && !isUnionFunc(bfa.be.Right) {
		return binaryOpNeqStdFunc(bfa)
	}

	// Special case for `q != (1,2,3)`
	left := bfa.left
	right := bfa.right
	if isUnionFunc(bfa.be.Left) {
		left, right = right, left
	}
	if len(left) == 0 {
		return nil, nil
	}
	if len(right) == 0 {
		return left, nil
	}
	for _, tsLeft := range left {
		values := tsLeft.Values
		for j, v := range values {
			if containsValueAt(right, v, j) {
				values[j] = nan
			}
		}
	}
	// Do not remove time series containing only NaNs, since then the `(foo op bar) default N`
	// won't work as expected if `(foo op bar)` results to NaN series.
	return left, nil
}

func isUnionFunc(e metricsql.Expr) bool {
	if fe, ok := e.(*metricsql.FuncExpr); ok && (fe.Name == "" || strings.EqualFold(fe.Name, "union")) {
		return true
	}
	return false
}

func containsValueAt(tss []*timeseries, v float64, idx int) bool {
	for _, ts := range tss {
		if ts.Values[idx] == v {
			return true
		}
	}
	return false
}

var (
	binaryOpEqStdFunc  = newBinaryOpCmpFunc(binaryop.Eq)
	binaryOpNeqStdFunc = newBinaryOpCmpFunc(binaryop.Neq)
)

func newBinaryOpCmpFunc(cf func(left, right float64) bool) binaryOpFunc {
	cfe := func(left, right float64, isBool bool) float64 {
		if !isBool {
			if cf(left, right) {
				return left
			}
			return nan
		}
		if math.IsNaN(left) {
			return nan
		}
		if cf(left, right) {
			return 1
		}
		return 0
	}
	return newBinaryOpFunc(cfe)
}

func newBinaryOpArithFunc(af func(left, right float64) float64) binaryOpFunc {
	afe := func(left, right float64, _ bool) float64 {
		return af(left, right)
	}
	return newBinaryOpFunc(afe)
}

func newBinaryOpFunc(bf func(left, right float64, isBool bool) float64) binaryOpFunc {
	return func(bfa *binaryOpFuncArg) ([]*timeseries, error) {
		left := bfa.left
		right := bfa.right
		op := bfa.be.Op
		switch true {
		case metricsql.IsBinaryOpCmp(op):
			// Do not remove empty series for comparison operations,
			// since this may lead to missing result.
		default:
			left = removeEmptySeries(left)
			right = removeEmptySeries(right)
		}
		if len(left) == 0 || len(right) == 0 {
			return nil, nil
		}
		left, right, dst, err := adjustBinaryOpTags(bfa.be, left, right)
		if err != nil {
			return nil, err
		}
		if len(left) != len(right) || len(left) != len(dst) {
			logger.Panicf("BUG: len(left) must match len(right) and len(dst); got %d vs %d vs %d", len(left), len(right), len(dst))
		}
		isBool := bfa.be.Bool
		for i, tsLeft := range left {
			leftValues := tsLeft.Values
			rightValues := right[i].Values
			dstValues := dst[i].Values
			if len(leftValues) != len(rightValues) || len(leftValues) != len(dstValues) {
				logger.Panicf("BUG: len(leftVaues) must match len(rightValues) and len(dstValues); got %d vs %d vs %d",
					len(leftValues), len(rightValues), len(dstValues))
			}
			for j, a := range leftValues {
				b := rightValues[j]
				dstValues[j] = bf(a, b, isBool)
			}
		}
		// Do not remove time series containing only NaNs, since then the `(foo op bar) default N`
		// won't work as expected if `(foo op bar)` results to NaN series.
		return dst, nil
	}
}

func adjustBinaryOpTags(be *metricsql.BinaryOpExpr, left, right []*timeseries) ([]*timeseries, []*timeseries, []*timeseries, error) {
	if len(be.GroupModifier.Op) == 0 && len(be.JoinModifier.Op) == 0 {
		if isScalar(left) {
			// Fast path: `scalar op vector`
			rvsLeft := make([]*timeseries, len(right))
			tsLeft := left[0]
			for i, tsRight := range right {
				resetMetricGroupIfRequired(be, tsRight)
				rvsLeft[i] = tsLeft
			}
			return rvsLeft, right, right, nil
		}
		if isScalar(right) {
			// Fast path: `vector op scalar`
			rvsRight := make([]*timeseries, len(left))
			tsRight := right[0]
			for i, tsLeft := range left {
				resetMetricGroupIfRequired(be, tsLeft)
				rvsRight[i] = tsRight
			}
			return left, rvsRight, left, nil
		}
	}

	// Slow path: `vector op vector` or `a op {on|ignoring} {group_left|group_right} b`
	var rvsLeft, rvsRight []*timeseries
	mLeft, mRight := createTimeseriesMapByTagSet(be, left, right)
	joinOp := strings.ToLower(be.JoinModifier.Op)
	groupOp := strings.ToLower(be.GroupModifier.Op)
	if len(groupOp) == 0 {
		groupOp = "ignoring"
	}
	groupTags := be.GroupModifier.Args
	if be.KeepMetricNames && groupOp == "on" {
		// Add __name__ to groupTags if metric name must be preserved.
		groupTags = append(groupTags[:len(groupTags):len(groupTags)], "__name__")
	}
	for k, tssLeft := range mLeft {
		tssRight := mRight[k]
		if len(tssRight) == 0 {
			continue
		}
		switch joinOp {
		case "group_left":
			var err error
			rvsLeft, rvsRight, err = groupJoin("right", be, rvsLeft, rvsRight, tssLeft, tssRight)
			if err != nil {
				return nil, nil, nil, err
			}
		case "group_right":
			var err error
			rvsRight, rvsLeft, err = groupJoin("left", be, rvsRight, rvsLeft, tssRight, tssLeft)
			if err != nil {
				return nil, nil, nil, err
			}
		case "":
			if err := ensureSingleTimeseries("left", be, tssLeft); err != nil {
				return nil, nil, nil, err
			}
			if err := ensureSingleTimeseries("right", be, tssRight); err != nil {
				return nil, nil, nil, err
			}
			tsLeft := tssLeft[0]
			resetMetricGroupIfRequired(be, tsLeft)
			switch groupOp {
			case "on":
				tsLeft.MetricName.RemoveTagsOn(groupTags)
			case "ignoring":
				tsLeft.MetricName.RemoveTagsIgnoring(groupTags)
			default:
				logger.Panicf("BUG: unexpected binary op modifier %q", groupOp)
			}
			rvsLeft = append(rvsLeft, tsLeft)
			rvsRight = append(rvsRight, tssRight[0])
		default:
			logger.Panicf("BUG: unexpected join modifier %q", joinOp)
		}
	}
	dst := rvsLeft
	if joinOp == "group_right" {
		dst = rvsRight
	}
	return rvsLeft, rvsRight, dst, nil
}

func ensureSingleTimeseries(side string, be *metricsql.BinaryOpExpr, tss []*timeseries) error {
	if len(tss) == 0 {
		logger.Panicf("BUG: tss must contain at least one value")
	}
	for len(tss) > 1 {
		if !mergeNonOverlappingTimeseries(tss[0], tss[len(tss)-1]) {
			return fmt.Errorf(`duplicate time series on the %s side of %s %s: %s and %s`, side, be.Op, be.GroupModifier.AppendString(nil),
				stringMetricTags(&tss[0].MetricName), stringMetricTags(&tss[len(tss)-1].MetricName))
		}
		tss = tss[:len(tss)-1]
	}
	return nil
}

func groupJoin(singleTimeseriesSide string, be *metricsql.BinaryOpExpr, rvsLeft, rvsRight, tssLeft, tssRight []*timeseries) ([]*timeseries, []*timeseries, error) {
	joinTags := be.JoinModifier.Args
	var skipTags []string
	if strings.EqualFold(be.GroupModifier.Op, "on") {
		skipTags = be.GroupModifier.Args
	}
	joinPrefix := ""
	if be.JoinModifierPrefix != nil {
		joinPrefix = be.JoinModifierPrefix.S
	}
	type tsPair struct {
		left  *timeseries
		right *timeseries
	}
	m := make(map[string]*tsPair)
	for _, tsLeft := range tssLeft {
		resetMetricGroupIfRequired(be, tsLeft)
		if len(tssRight) == 1 {
			// Easy case - right part contains only a single matching time series.
			tsLeft.MetricName.SetTags(joinTags, joinPrefix, skipTags, &tssRight[0].MetricName)
			rvsLeft = append(rvsLeft, tsLeft)
			rvsRight = append(rvsRight, tssRight[0])
			continue
		}

		// Hard case - right part contains multiple matching time series.
		// Verify it doesn't result in duplicate MetricName values after adding missing tags.
		for k := range m {
			delete(m, k)
		}
		bb := bbPool.Get()
		for _, tsRight := range tssRight {
			var tsCopy timeseries
			tsCopy.CopyFromShallowTimestamps(tsLeft)
			tsCopy.MetricName.SetTags(joinTags, joinPrefix, skipTags, &tsRight.MetricName)
			bb.B = marshalMetricNameSorted(bb.B[:0], &tsCopy.MetricName)
			pair, ok := m[string(bb.B)]
			if !ok {
				m[string(bb.B)] = &tsPair{
					left:  &tsCopy,
					right: tsRight,
				}
				continue
			}
			// Try merging pair.right with tsRight if they don't overlap.
			var tmp timeseries
			tmp.CopyFromShallowTimestamps(pair.right)
			if !mergeNonOverlappingTimeseries(&tmp, tsRight) {
				return nil, nil, fmt.Errorf("duplicate time series on the %s side of `%s %s %s`: %s and %s",
					singleTimeseriesSide, be.Op, be.GroupModifier.AppendString(nil), be.JoinModifier.AppendString(nil),
					stringMetricTags(&tmp.MetricName), stringMetricTags(&tsRight.MetricName))
			}
			pair.right = &tmp
		}
		bbPool.Put(bb)
		for _, pair := range m {
			rvsLeft = append(rvsLeft, pair.left)
			rvsRight = append(rvsRight, pair.right)
		}
	}
	return rvsLeft, rvsRight, nil
}

func mergeNonOverlappingTimeseries(dst, src *timeseries) bool {
	// Verify whether the time series can be merged.
	srcValues := src.Values
	dstValues := dst.Values
	overlaps := 0
	_ = dstValues[len(srcValues)-1]
	for i, v := range srcValues {
		if math.IsNaN(v) {
			continue
		}
		if !math.IsNaN(dstValues[i]) {
			overlaps++
		}
	}
	// Allow up to two overlapping datapoints, which can appear due to staleness algorithm,
	// which can add a few datapoints in the end of time series.
	if overlaps > 2 {
		return false
	}
	// Do not merge time series with too small number of datapoints.
	// This can be the case during evaluation of instant queries (alerting or recording rules).
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1141
	if len(srcValues) <= 2 && len(dstValues) <= 2 {
		return false
	}
	// Time series can be merged. Merge them.
	for i, v := range srcValues {
		if math.IsNaN(v) {
			continue
		}
		dstValues[i] = v
	}
	return true
}

func resetMetricGroupIfRequired(be *metricsql.BinaryOpExpr, ts *timeseries) {
	if metricsql.IsBinaryOpCmp(be.Op) && !be.Bool {
		// Do not reset MetricGroup for non-boolean `compare` binary ops like Prometheus does.
		return
	}
	if be.KeepMetricNames {
		// Do not reset MetricGroup if it is explicitly requested via `a op b keep_metric_names`
		// See https://docs.victoriametrics.com/metricsql/#keep_metric_names
		return
	}

	ts.MetricName.ResetMetricGroup()
}

func binaryOpIf(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k, tssLeft := range mLeft {
		tssRight := seriesByKey(mRight, k)
		if tssRight == nil {
			continue
		}
		tssLeft = addRightNaNsToLeft(tssLeft, tssRight)
		rvs = append(rvs, tssLeft...)
	}
	return rvs, nil
}

func binaryOpAnd(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k, tssRight := range mRight {
		tssLeft := mLeft[k]
		if tssLeft == nil {
			continue
		}
		tssLeft = addRightNaNsToLeft(tssLeft, tssRight)
		rvs = append(rvs, tssLeft...)
	}
	return rvs, nil
}

func addRightNaNsToLeft(tssLeft, tssRight []*timeseries) []*timeseries {
	for _, tsLeft := range tssLeft {
		valuesLeft := tsLeft.Values
		for i := range valuesLeft {
			hasValue := false
			for _, tsRight := range tssRight {
				if !math.IsNaN(tsRight.Values[i]) {
					hasValue = true
					break
				}
			}
			if !hasValue {
				valuesLeft[i] = nan
			}
		}
	}
	return removeEmptySeries(tssLeft)
}

func binaryOpDefault(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	if len(mLeft) == 0 {
		for _, tss := range mRight {
			rvs = append(rvs, tss...)
		}
		return rvs, nil
	}
	for k, tssLeft := range mLeft {
		rvs = append(rvs, tssLeft...)
		tssRight := seriesByKey(mRight, k)
		if tssRight == nil {
			continue
		}
		fillLeftNaNsWithRightValues(tssLeft, tssRight)
	}
	return rvs, nil
}

func binaryOpOr(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries

	for _, tss := range mLeft {
		rvs = append(rvs, tss...)
	}
	// Sort left-hand-side series by metric name as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5393
	sortSeriesByMetricName(rvs)
	rvsLen := len(rvs)

	for k, tssRight := range mRight {
		tssLeft := mLeft[k]
		if tssLeft == nil {
			rvs = append(rvs, tssRight...)
			continue
		}
		fillLeftNaNsWithRightValues(tssLeft, tssRight)
	}
	// Sort the added right-hand-side series by metric name as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5393
	sortSeriesByMetricName(rvs[rvsLen:])

	return rvs, nil
}

func fillLeftNaNsWithRightValues(tssLeft, tssRight []*timeseries) {
	// Fill gaps in tssLeft with values from tssRight as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/552
	for _, tsLeft := range tssLeft {
		valuesLeft := tsLeft.Values
		for i, v := range valuesLeft {
			if !math.IsNaN(v) {
				continue
			}
			for _, tsRight := range tssRight {
				vRight := tsRight.Values[i]
				if !math.IsNaN(vRight) {
					valuesLeft[i] = vRight
					break
				}
			}
		}
	}
}

func binaryOpIfnot(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k, tssLeft := range mLeft {
		tssRight := seriesByKey(mRight, k)
		if tssRight == nil {
			rvs = append(rvs, tssLeft...)
			continue
		}
		tssLeft = addLeftNaNsIfNoRightNaNs(tssLeft, tssRight)
		rvs = append(rvs, tssLeft...)
	}
	return rvs, nil
}

func binaryOpUnless(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k, tssLeft := range mLeft {
		tssRight := mRight[k]
		if tssRight == nil {
			rvs = append(rvs, tssLeft...)
			continue
		}
		tssLeft = addLeftNaNsIfNoRightNaNs(tssLeft, tssRight)
		rvs = append(rvs, tssLeft...)
	}
	return rvs, nil
}

func addLeftNaNsIfNoRightNaNs(tssLeft, tssRight []*timeseries) []*timeseries {
	for _, tsLeft := range tssLeft {
		valuesLeft := tsLeft.Values
		for i := range valuesLeft {
			for _, tsRight := range tssRight {
				if !math.IsNaN(tsRight.Values[i]) {
					valuesLeft[i] = nan
					break
				}
			}
		}
	}
	return removeEmptySeries(tssLeft)
}

func seriesByKey(m map[string][]*timeseries, key string) []*timeseries {
	tss := m[key]
	if tss != nil {
		return tss
	}
	if len(m) != 1 {
		return nil
	}
	for _, tss := range m {
		if isScalar(tss) {
			return tss
		}
		return nil
	}
	return nil
}

func createTimeseriesMapByTagSet(be *metricsql.BinaryOpExpr, left, right []*timeseries) (map[string][]*timeseries, map[string][]*timeseries) {
	groupTags := be.GroupModifier.Args
	groupOp := strings.ToLower(be.GroupModifier.Op)
	if len(groupOp) == 0 {
		groupOp = "ignoring"
	}
	getTagsMap := func(arg []*timeseries) map[string][]*timeseries {
		bb := bbPool.Get()
		m := make(map[string][]*timeseries, len(arg))
		mn := storage.GetMetricName()
		for _, ts := range arg {
			mn.CopyFrom(&ts.MetricName)
			if !be.KeepMetricNames {
				mn.ResetMetricGroup()
			}
			switch groupOp {
			case "on":
				mn.RemoveTagsOn(groupTags)
			case "ignoring":
				mn.RemoveTagsIgnoring(groupTags)
			default:
				logger.Panicf("BUG: unexpected binary op modifier %q", groupOp)
			}
			bb.B = marshalMetricNameSorted(bb.B[:0], mn)
			k := string(bb.B)
			m[k] = append(m[k], ts)
		}
		storage.PutMetricName(mn)
		bbPool.Put(bb)
		return m
	}
	mLeft := getTagsMap(left)
	mRight := getTagsMap(right)
	return mLeft, mRight
}

func isScalar(arg []*timeseries) bool {
	if len(arg) != 1 {
		return false
	}
	mn := &arg[0].MetricName
	if len(mn.MetricGroup) > 0 {
		return false
	}
	return len(mn.Tags) == 0
}
