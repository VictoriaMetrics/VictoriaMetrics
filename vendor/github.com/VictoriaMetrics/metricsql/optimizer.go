package metricsql

import (
	"sort"
)

// Optimize optimizes e in order to improve its performance.
func Optimize(e Expr) Expr {
	switch t := e.(type) {
	case *BinaryOpExpr:
		// Convert `foo{filters1} op bar{filters2}` to `foo{filters1, filters2} op bar{filters1, filters2}`.
		// This should reduce the number of operations
		// See https://utcc.utoronto.ca/~cks/space/blog/sysadmin/PrometheusLabelNonOptimization
		// for details.
		switch t.Op {
		case "+", "-", "*", "/", "%", "^",
			"==", "!=", ">", "<", ">=", "<=",
			"if", "ifnot", "default":
			// The optimization can be applied only to these operations.
		default:
			return optimizeBinaryOpArgs(t)
		}
		if t.JoinModifier.Op != "" {
			return optimizeBinaryOpArgs(t)
		}
		if t.GroupModifier.Op != "" {
			return optimizeBinaryOpArgs(t)
		}
		meLeft, ok := t.Left.(*MetricExpr)
		if !ok || !meLeft.hasNonEmptyMetricGroup() {
			return optimizeBinaryOpArgs(t)
		}
		meRight, ok := t.Right.(*MetricExpr)
		if !ok || !meRight.hasNonEmptyMetricGroup() {
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

func optimizeBinaryOpArgs(be *BinaryOpExpr) *BinaryOpExpr {
	be.Left = Optimize(be.Left)
	be.Right = Optimize(be.Right)
	return be
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
