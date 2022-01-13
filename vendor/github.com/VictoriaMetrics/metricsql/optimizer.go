package metricsql

import (
	"sort"
	"strings"
)

// Optimize optimizes e in order to improve its performance.
//
// It performs the following optimizations:
//
// - Adds missing filters to `foo{filters1} op bar{filters2}`
//   according to https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization
//   I.e. such query is converted to `foo{filters1, filters2} op bar{filters1, filters2}`
func Optimize(e Expr) Expr {
	switch t := e.(type) {
	case *BinaryOpExpr:
		// Convert `foo{filters1} op bar{filters2}` to `foo{filters1, filters2} op bar{filters1, filters2}`.
		// This should reduce the number of operations
		// See https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization
		// for details.
		if !canOptimizeBinaryOp(t) {
			return optimizeBinaryOpArgs(t)
		}
		meLeft := getMetricExprForOptimization(t.Left)
		if meLeft == nil || !meLeft.hasNonEmptyMetricGroup() {
			return optimizeBinaryOpArgs(t)
		}
		meRight := getMetricExprForOptimization(t.Right)
		if meRight == nil || !meRight.hasNonEmptyMetricGroup() {
			return optimizeBinaryOpArgs(t)
		}
		lfs := intersectLabelFilters(meLeft.LabelFilters[1:], meRight.LabelFilters[1:])
		meLeft.LabelFilters = append(meLeft.LabelFilters[:1], lfs...)
		meRight.LabelFilters = append(meRight.LabelFilters[:1], lfs...)
		return t
	case *FuncExpr:
		for i := range t.Args {
			t.Args[i] = Optimize(t.Args[i])
		}
		return t
	case *AggrFuncExpr:
		for i := range t.Args {
			t.Args[i] = Optimize(t.Args[i])
		}
		return t
	default:
		return e
	}
}

func canOptimizeBinaryOp(be *BinaryOpExpr) bool {
	if be.JoinModifier.Op != "" || be.GroupModifier.Op != "" {
		return false
	}
	switch be.Op {
	case "+", "-", "*", "/", "%", "^",
		"==", "!=", ">", "<", ">=", "<=",
		"and", "if", "ifnot", "default":
		return true
	default:
		return false
	}
}

func optimizeBinaryOpArgs(be *BinaryOpExpr) *BinaryOpExpr {
	be.Left = Optimize(be.Left)
	be.Right = Optimize(be.Right)
	return be
}

func getMetricExprForOptimization(e Expr) *MetricExpr {
	re, ok := e.(*RollupExpr)
	if ok {
		// Try optimizing the inner expression in RollupExpr.
		return getMetricExprForOptimization(re.Expr)
	}
	me, ok := e.(*MetricExpr)
	if ok {
		// Ordinary metric expression, i.e. `foo{bar="baz"}`
		return me
	}
	be, ok := e.(*BinaryOpExpr)
	if ok {
		if !canOptimizeBinaryOp(be) {
			return nil
		}
		if me, ok := be.Left.(*MetricExpr); ok && isNumberOrScalar(be.Right) {
			// foo{bar="baz"} * num_or_scalar
			return me
		}
		if me, ok := be.Right.(*MetricExpr); ok && isNumberOrScalar(be.Left) {
			// num_or_scalar * foo{bar="baz"}
			return me
		}
		return nil
	}
	fe, ok := e.(*FuncExpr)
	if !ok {
		return nil
	}
	if IsRollupFunc(fe.Name) {
		argIdx := GetRollupArgIdx(fe)
		if argIdx >= len(fe.Args) {
			return nil
		}
		arg := fe.Args[argIdx]
		return getMetricExprForOptimization(arg)
	}
	if IsTransformFunc(fe.Name) {
		switch strings.ToLower(fe.Name) {
		case "absent", "histogram_quantile", "label_join", "label_replace", "scalar", "vector",
			"label_set", "label_map", "label_uppercase", "label_lowercase", "label_del", "label_keep", "label_copy",
			"label_move", "label_transform", "label_value", "label_match", "label_mismatch", "label_graphite_group",
			"prometheus_buckets", "buckets_limit", "histogram_share", "histogram_avg", "histogram_stdvar", "histogram_stddev", "union", "":
			// metric expressions for these functions cannot be optimized.
			return nil
		}
		for _, arg := range fe.Args {
			if me, ok := arg.(*MetricExpr); ok {
				// transform_func(foo{bar="baz"})
				return me
			}
		}
		return nil
	}
	return nil
}

func isNumberOrScalar(e Expr) bool {
	if _, ok := e.(*NumberExpr); ok {
		return true
	}
	if fe, ok := e.(*FuncExpr); ok && strings.ToLower(fe.Name) == "scalar" {
		return true
	}
	return false
}

func intersectLabelFilters(a, b []LabelFilter) []LabelFilter {
	m := make(map[string]LabelFilter, len(a)+len(b))
	var buf []byte
	for _, lf := range a {
		buf = lf.AppendString(buf[:0])
		m[string(buf)] = lf
	}
	for _, lf := range b {
		buf = lf.AppendString(buf[:0])
		m[string(buf)] = lf
	}
	ss := make([]string, 0, len(m))
	for s := range m {
		ss = append(ss, s)
	}
	sort.Strings(ss)
	lfs := make([]LabelFilter, 0, len(ss))
	for _, s := range ss {
		lfs = append(lfs, m[s])
	}
	return lfs
}
