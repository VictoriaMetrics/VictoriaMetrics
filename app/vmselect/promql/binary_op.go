package promql

import (
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var binaryOpFuncs = map[string]binaryOpFunc{
	"+": newBinaryOpArithFunc(binaryOpPlus),
	"-": newBinaryOpArithFunc(binaryOpMinus),
	"*": newBinaryOpArithFunc(binaryOpMul),
	"/": newBinaryOpArithFunc(binaryOpDiv),
	"%": newBinaryOpArithFunc(binaryOpMod),
	"^": newBinaryOpArithFunc(binaryOpPow),

	// cmp ops
	"==": newBinaryOpCmpFunc(binaryOpEq),
	"!=": newBinaryOpCmpFunc(binaryOpNeq),
	">":  newBinaryOpCmpFunc(binaryOpGt),
	"<":  newBinaryOpCmpFunc(binaryOpLt),
	">=": newBinaryOpCmpFunc(binaryOpGte),
	"<=": newBinaryOpCmpFunc(binaryOpLte),

	// logical set ops
	"and":    binaryOpAnd,
	"or":     binaryOpOr,
	"unless": binaryOpUnless,

	// New op
	"if":      newBinaryOpArithFunc(binaryOpIf),
	"ifnot":   newBinaryOpArithFunc(binaryOpIfnot),
	"default": newBinaryOpArithFunc(binaryOpDefault),
}

func getBinaryOpFunc(op string) binaryOpFunc {
	op = strings.ToLower(op)
	return binaryOpFuncs[op]
}

func isBinaryOpCmp(op string) bool {
	switch op {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

func binaryOpConstants(op string, left, right float64, isBool bool) float64 {
	if isBinaryOpCmp(op) {
		evalCmp := func(cf func(left, right float64) bool) float64 {
			if isBool {
				if cf(left, right) {
					return 1
				}
				return 0
			}
			if cf(left, right) {
				return left
			}
			return nan
		}
		switch op {
		case "==":
			left = evalCmp(binaryOpEq)
		case "!=":
			left = evalCmp(binaryOpNeq)
		case ">":
			left = evalCmp(binaryOpGt)
		case "<":
			left = evalCmp(binaryOpLt)
		case ">=":
			left = evalCmp(binaryOpGte)
		case "<=":
			left = evalCmp(binaryOpLte)
		default:
			logger.Panicf("BUG: unexpected comparison binaryOp: %q", op)
		}
	} else {
		switch op {
		case "+":
			left = binaryOpPlus(left, right)
		case "-":
			left = binaryOpMinus(left, right)
		case "*":
			left = binaryOpMul(left, right)
		case "/":
			left = binaryOpDiv(left, right)
		case "%":
			left = binaryOpMod(left, right)
		case "^":
			left = binaryOpPow(left, right)
		case "and":
			// Nothing to do
		case "or":
			// Nothing to do
		case "unless":
			left = nan
		case "default":
			left = binaryOpDefault(left, right)
		case "if":
			left = binaryOpIf(left, right)
		case "ifnot":
			left = binaryOpIfnot(left, right)
		default:
			logger.Panicf("BUG: unexpected non-comparison binaryOp: %q", op)
		}
	}
	return left
}

type binaryOpFuncArg struct {
	be    *promql.BinaryOpExpr
	left  []*timeseries
	right []*timeseries
}

type binaryOpFunc func(bfa *binaryOpFuncArg) ([]*timeseries, error)

func newBinaryOpCmpFunc(cf func(left, right float64) bool) binaryOpFunc {
	cfe := func(left, right float64, isBool bool) float64 {
		if !isBool {
			if cf(left, right) {
				return left
			}
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
	afe := func(left, right float64, isBool bool) float64 {
		return af(left, right)
	}
	return newBinaryOpFunc(afe)
}

func newBinaryOpFunc(bf func(left, right float64, isBool bool) float64) binaryOpFunc {
	return func(bfa *binaryOpFuncArg) ([]*timeseries, error) {
		isBool := bfa.be.Bool
		left, right, dst, err := adjustBinaryOpTags(bfa.be, bfa.left, bfa.right)
		if err != nil {
			return nil, err
		}
		if len(left) != len(right) || len(left) != len(dst) {
			logger.Panicf("BUG: len(left) must match len(right) and len(dst); got %d vs %d vs %d", len(left), len(right), len(dst))
		}
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
		// Optimization: remove time series containing only NaNs.
		// This is quite common after applying filters like `q > 0`.
		dst = removeNaNs(dst)
		return dst, nil
	}
}

func adjustBinaryOpTags(be *promql.BinaryOpExpr, left, right []*timeseries) ([]*timeseries, []*timeseries, []*timeseries, error) {
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

func ensureSingleTimeseries(side string, be *promql.BinaryOpExpr, tss []*timeseries) error {
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

func groupJoin(singleTimeseriesSide string, be *promql.BinaryOpExpr, rvsLeft, rvsRight, tssLeft, tssRight []*timeseries) ([]*timeseries, []*timeseries, error) {
	joinTags := be.JoinModifier.Args
	var m map[string]*timeseries
	for _, tsLeft := range tssLeft {
		resetMetricGroupIfRequired(be, tsLeft)
		if len(tssRight) == 1 {
			// Easy case - right part contains only a single matching time series.
			tsLeft.MetricName.AddMissingTags(joinTags, &tssRight[0].MetricName)
			rvsLeft = append(rvsLeft, tsLeft)
			rvsRight = append(rvsRight, tssRight[0])
			continue
		}

		// Hard case - right part contains multiple matching time series.
		// Verify it doesn't result in duplicate MetricName values after adding missing tags.
		if m == nil {
			m = make(map[string]*timeseries, len(tssRight))
		} else {
			for k := range m {
				delete(m, k)
			}
		}
		bb := bbPool.Get()
		for _, tsRight := range tssRight {
			var tsCopy timeseries
			tsCopy.CopyFromShallowTimestamps(tsLeft)
			tsCopy.MetricName.AddMissingTags(joinTags, &tsRight.MetricName)
			bb.B = marshalMetricTagsSorted(bb.B[:0], &tsCopy.MetricName)
			if tsExisting := m[string(bb.B)]; tsExisting != nil {
				// Try merging tsExisting with tsRight if they don't overlap.
				if mergeNonOverlappingTimeseries(tsExisting, tsRight) {
					continue
				}
				return nil, nil, fmt.Errorf("duplicate time series on the %s side of `%s %s %s`: %s and %s",
					singleTimeseriesSide, be.Op, be.GroupModifier.AppendString(nil), be.JoinModifier.AppendString(nil),
					stringMetricTags(&tsExisting.MetricName), stringMetricTags(&tsRight.MetricName))
			}
			m[string(bb.B)] = tsRight
			rvsLeft = append(rvsLeft, &tsCopy)
			rvsRight = append(rvsRight, tsRight)
		}
		bbPool.Put(bb)
	}
	return rvsLeft, rvsRight, nil
}

func mergeNonOverlappingTimeseries(dst, src *timeseries) bool {
	// Verify whether the time series can be merged.
	srcValues := src.Values
	dstValues := dst.Values
	_ = dstValues[len(srcValues)-1]
	for i, v := range srcValues {
		if math.IsNaN(v) {
			continue
		}
		if !math.IsNaN(dstValues[i]) {
			return false
		}
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

func resetMetricGroupIfRequired(be *promql.BinaryOpExpr, ts *timeseries) {
	if isBinaryOpCmp(be.Op) && !be.Bool {
		// Do not reset MetricGroup for non-boolean `compare` binary ops like Prometheus does.
		return
	}
	switch be.Op {
	case "default", "if", "ifnot":
		// Do not reset MetricGroup for these ops.
		return
	}
	ts.MetricName.ResetMetricGroup()
}

func binaryOpPlus(left, right float64) float64 {
	return left + right
}

func binaryOpMinus(left, right float64) float64 {
	return left - right
}

func binaryOpMul(left, right float64) float64 {
	return left * right
}

func binaryOpDiv(left, right float64) float64 {
	return left / right
}

func binaryOpMod(left, right float64) float64 {
	return math.Mod(left, right)
}

func binaryOpPow(left, right float64) float64 {
	return math.Pow(left, right)
}

func binaryOpDefault(left, right float64) float64 {
	if math.IsNaN(left) {
		return right
	}
	return left
}

func binaryOpIf(left, right float64) float64 {
	if math.IsNaN(right) {
		return nan
	}
	return left
}

func binaryOpIfnot(left, right float64) float64 {
	if math.IsNaN(right) {
		return left
	}
	return nan
}

func binaryOpEq(left, right float64) bool {
	// Special handling for nan == nan.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150 .
	if math.IsNaN(left) {
		return math.IsNaN(right)
	}

	return left == right
}

func binaryOpNeq(left, right float64) bool {
	// Special handling for comparison with nan.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150 .
	if math.IsNaN(left) {
		return !math.IsNaN(right)
	}
	if math.IsNaN(right) {
		return true
	}

	return left != right
}

func binaryOpGt(left, right float64) bool {
	return left > right
}

func binaryOpLt(left, right float64) bool {
	return left < right
}

func binaryOpGte(left, right float64) bool {
	return left >= right
}

func binaryOpLte(left, right float64) bool {
	return left <= right
}

func binaryOpAnd(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k := range mRight {
		if tss := mLeft[k]; tss != nil {
			rvs = append(rvs, tss...)
		}
	}
	return rvs, nil
}

func binaryOpOr(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for _, tss := range mLeft {
		rvs = append(rvs, tss...)
	}
	for k, tss := range mRight {
		if mLeft[k] == nil {
			rvs = append(rvs, tss...)
		}
	}
	return rvs, nil
}

func binaryOpUnless(bfa *binaryOpFuncArg) ([]*timeseries, error) {
	mLeft, mRight := createTimeseriesMapByTagSet(bfa.be, bfa.left, bfa.right)
	var rvs []*timeseries
	for k, tss := range mLeft {
		if mRight[k] == nil {
			rvs = append(rvs, tss...)
		}
	}
	return rvs, nil
}

func createTimeseriesMapByTagSet(be *promql.BinaryOpExpr, left, right []*timeseries) (map[string][]*timeseries, map[string][]*timeseries) {
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
			mn.ResetMetricGroup()
			switch groupOp {
			case "on":
				mn.RemoveTagsOn(groupTags)
			case "ignoring":
				mn.RemoveTagsIgnoring(groupTags)
			default:
				logger.Panicf("BUG: unexpected binary op modifier %q", groupOp)
			}
			bb.B = marshalMetricTagsSorted(bb.B[:0], mn)
			m[string(bb.B)] = append(m[string(bb.B)], ts)
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
