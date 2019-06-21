package promql

import (
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

var binaryOpPriorities = map[string]int{
	"default": -1,

	"if":    0,
	"ifnot": 0,

	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operator-precedence
	"or": 1,

	"and":    2,
	"unless": 2,

	"==": 3,
	"!=": 3,
	"<":  3,
	">":  3,
	"<=": 3,
	">=": 3,

	"+": 4,
	"-": 4,

	"*": 5,
	"/": 5,
	"%": 5,

	"^": 6,
}

func getBinaryOpFunc(op string) binaryOpFunc {
	op = strings.ToLower(op)
	return binaryOpFuncs[op]
}

func isBinaryOp(op string) bool {
	return getBinaryOpFunc(op) != nil
}

func binaryOpPriority(op string) int {
	op = strings.ToLower(op)
	return binaryOpPriorities[op]
}

func scanBinaryOpPrefix(s string) int {
	n := 0
	for op := range binaryOpFuncs {
		if len(s) < len(op) {
			continue
		}
		ss := strings.ToLower(s[:len(op)])
		if ss == op && len(op) > n {
			n = len(op)
		}
	}
	return n
}

func isRightAssociativeBinaryOp(op string) bool {
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operator-precedence
	return op == "^"
}

func isBinaryOpGroupModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching
	case "on", "ignoring":
		return true
	default:
		return false
	}
}

func isBinaryOpJoinModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	case "group_left", "group_right":
		return true
	default:
		return false
	}
}

func isBinaryOpBoolModifier(s string) bool {
	s = strings.ToLower(s)
	return s == "bool"
}

func isBinaryOpCmp(op string) bool {
	switch op {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

func isBinaryOpLogicalSet(op string) bool {
	op = strings.ToLower(op)
	switch op {
	case "and", "or", "unless":
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
	be    *binaryOpExpr
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
		return dst, nil
	}
}

func adjustBinaryOpTags(be *binaryOpExpr, left, right []*timeseries) ([]*timeseries, []*timeseries, []*timeseries, error) {
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
	ensureOneX := func(side string, tss []*timeseries) error {
		if len(tss) == 0 {
			logger.Panicf("BUG: tss must contain at least one value")
		}
		if len(tss) == 1 {
			return nil
		}
		if mergeNonOverlappingTimeseries(tss) {
			return nil
		}
		return fmt.Errorf(`duplicate timeseries on the %s side of %s %s: %s and %s`, side, be.Op, be.GroupModifier.AppendString(nil),
			stringMetricTags(&tss[0].MetricName), stringMetricTags(&tss[1].MetricName))
	}

	var rvsLeft, rvsRight []*timeseries
	mLeft, mRight := createTimeseriesMapByTagSet(be, left, right)
	joinOp := strings.ToLower(be.JoinModifier.Op)
	joinTags := be.JoinModifier.Args
	for k, tssLeft := range mLeft {
		tssRight := mRight[k]
		if len(tssRight) == 0 {
			continue
		}
		switch joinOp {
		case "group_left":
			if err := ensureOneX("right", tssRight); err != nil {
				return nil, nil, nil, err
			}
			src := tssRight[0]
			for _, ts := range tssLeft {
				ts.MetricName.AddMissingTags(joinTags, &src.MetricName)
				rvsLeft = append(rvsLeft, ts)
				rvsRight = append(rvsRight, src)
			}
		case "group_right":
			if err := ensureOneX("left", tssLeft); err != nil {
				return nil, nil, nil, err
			}
			src := tssLeft[0]
			for _, ts := range tssRight {
				ts.MetricName.AddMissingTags(joinTags, &src.MetricName)
				rvsLeft = append(rvsLeft, src)
				rvsRight = append(rvsRight, ts)
			}
		case "":
			if err := ensureOneX("left", tssLeft); err != nil {
				return nil, nil, nil, err
			}
			if err := ensureOneX("right", tssRight); err != nil {
				return nil, nil, nil, err
			}
			resetMetricGroupIfRequired(be, tssLeft[0])
			rvsLeft = append(rvsLeft, tssLeft[0])
			rvsRight = append(rvsRight, tssRight[0])
		default:
			return nil, nil, nil, fmt.Errorf(`unexpected join modifier %q`, joinOp)
		}
	}
	dst := rvsLeft
	if joinOp == "group_right" {
		dst = rvsRight
	}
	return rvsLeft, rvsRight, dst, nil
}

func resetMetricGroupIfRequired(be *binaryOpExpr, ts *timeseries) {
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
	return left == right
}

func binaryOpNeq(left, right float64) bool {
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

func createTimeseriesMapByTagSet(be *binaryOpExpr, left, right []*timeseries) (map[string][]*timeseries, map[string][]*timeseries) {
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

func mergeNonOverlappingTimeseries(tss []*timeseries) bool {
	if len(tss) < 2 {
		logger.Panicf("BUG: expecting at least two timeseries. Got %d", len(tss))
	}

	// Check whether time series in tss overlap.
	var dst timeseries
	dst.CopyFromShallowTimestamps(tss[0])
	dstValues := dst.Values
	for _, ts := range tss[1:] {
		for i, value := range ts.Values {
			if math.IsNaN(dstValues[i]) {
				dstValues[i] = value
			} else if !math.IsNaN(value) {
				// Time series overlap.
				return false
			}
		}
	}
	tss[0].CopyFromShallowTimestamps(&dst)
	return true
}
