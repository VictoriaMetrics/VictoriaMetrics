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
	case *RollupExpr:
		t.Expr = Optimize(t.Expr)
		t.At = Optimize(t.At)
	case *FuncExpr:
		optimizeFuncArgs(t.Args)
	case *AggrFuncExpr:
		optimizeFuncArgs(t.Args)
	case *BinaryOpExpr:
		t.Left = Optimize(t.Left)
		t.Right = Optimize(t.Right)
		lfs := getCommonLabelFilters(t)
		pushdownLabelFilters(t, lfs)
	}
	return e
}

func optimizeFuncArgs(args []Expr) {
	for i := range args {
		args[i] = Optimize(args[i])
	}
}

func getCommonLabelFilters(e Expr) []LabelFilter {
	switch t := e.(type) {
	case *MetricExpr:
		return getLabelFiltersWithoutMetricName(t.LabelFilters)
	case *RollupExpr:
		return getCommonLabelFilters(t.Expr)
	case *FuncExpr:
		arg := getFuncArgForOptimization(t.Name, t.Args)
		if arg == nil {
			return nil
		}
		return getCommonLabelFilters(arg)
	case *AggrFuncExpr:
		arg := getFuncArgForOptimization(t.Name, t.Args)
		if arg == nil {
			return nil
		}
		lfs := getCommonLabelFilters(arg)
		return filterLabelFiltersByAggrModifier(lfs, t)
	case *BinaryOpExpr:
		if !canOptimizeBinaryOp(t) {
			return nil
		}
		lfsLeft := getCommonLabelFilters(t.Left)
		lfsRight := getCommonLabelFilters(t.Right)
		lfs := unionLabelFilters(lfsLeft, lfsRight)
		return filterLabelFiltersByGroupModifier(lfs, t)
	default:
		return nil
	}
}

func filterLabelFiltersByAggrModifier(lfs []LabelFilter, afe *AggrFuncExpr) []LabelFilter {
	switch strings.ToLower(afe.Modifier.Op) {
	case "by":
		return filterLabelFiltersOn(lfs, afe.Modifier.Args)
	case "without":
		return filterLabelFiltersIgnoring(lfs, afe.Modifier.Args)
	default:
		return nil
	}
}

func filterLabelFiltersByGroupModifier(lfs []LabelFilter, be *BinaryOpExpr) []LabelFilter {
	switch strings.ToLower(be.GroupModifier.Op) {
	case "on":
		return filterLabelFiltersOn(lfs, be.GroupModifier.Args)
	case "ignoring":
		return filterLabelFiltersIgnoring(lfs, be.GroupModifier.Args)
	default:
		return lfs
	}
}

func getLabelFiltersWithoutMetricName(lfs []LabelFilter) []LabelFilter {
	lfsNew := make([]LabelFilter, 0, len(lfs))
	for _, lf := range lfs {
		if lf.Label != "__name__" {
			lfsNew = append(lfsNew, lf)
		}
	}
	return lfsNew
}

func pushdownLabelFilters(e Expr, lfs []LabelFilter) {
	if len(lfs) == 0 {
		return
	}
	switch t := e.(type) {
	case *MetricExpr:
		t.LabelFilters = unionLabelFilters(t.LabelFilters, lfs)
		sortLabelFilters(t.LabelFilters)
	case *RollupExpr:
		pushdownLabelFilters(t.Expr, lfs)
	case *FuncExpr:
		arg := getFuncArgForOptimization(t.Name, t.Args)
		if arg != nil {
			pushdownLabelFilters(arg, lfs)
		}
	case *AggrFuncExpr:
		lfs = filterLabelFiltersByAggrModifier(lfs, t)
		arg := getFuncArgForOptimization(t.Name, t.Args)
		if arg != nil {
			pushdownLabelFilters(arg, lfs)
		}
	case *BinaryOpExpr:
		if canOptimizeBinaryOp(t) {
			lfs = filterLabelFiltersByGroupModifier(lfs, t)
			pushdownLabelFilters(t.Left, lfs)
			pushdownLabelFilters(t.Right, lfs)
		}
	}
}

func unionLabelFilters(lfsA, lfsB []LabelFilter) []LabelFilter {
	if len(lfsA) == 0 {
		return lfsB
	}
	if len(lfsB) == 0 {
		return lfsA
	}
	m := make(map[string]struct{}, len(lfsA))
	var b []byte
	for _, lf := range lfsA {
		b = lf.AppendString(b[:0])
		m[string(b)] = struct{}{}
	}
	lfs := append([]LabelFilter{}, lfsA...)
	for _, lf := range lfsB {
		b = lf.AppendString(b[:0])
		if _, ok := m[string(b)]; !ok {
			lfs = append(lfs, lf)
		}
	}
	return lfs
}

func sortLabelFilters(lfs []LabelFilter) {
	// Make sure the first label filter is __name__ (if any)
	if len(lfs) > 0 && lfs[0].isMetricNameFilter() {
		lfs = lfs[1:]
	}
	sort.Slice(lfs, func(i, j int) bool {
		a, b := lfs[i], lfs[j]
		if a.Label != b.Label {
			return a.Label < b.Label
		}
		return a.Value < b.Value
	})
}

func filterLabelFiltersOn(lfs []LabelFilter, args []string) []LabelFilter {
	if len(args) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(args))
	for _, arg := range args {
		m[arg] = struct{}{}
	}
	var lfsNew []LabelFilter
	for _, lf := range lfs {
		if _, ok := m[lf.Label]; ok {
			lfsNew = append(lfsNew, lf)
		}
	}
	return lfsNew
}

func filterLabelFiltersIgnoring(lfs []LabelFilter, args []string) []LabelFilter {
	if len(args) == 0 {
		return lfs
	}
	m := make(map[string]struct{}, len(args))
	for _, arg := range args {
		m[arg] = struct{}{}
	}
	var lfsNew []LabelFilter
	for _, lf := range lfs {
		if _, ok := m[lf.Label]; !ok {
			lfsNew = append(lfsNew, lf)
		}
	}
	return lfsNew
}

func canOptimizeBinaryOp(be *BinaryOpExpr) bool {
	switch be.Op {
	case "+", "-", "*", "/", "%", "^",
		"==", "!=", ">", "<", ">=", "<=",
		"and", "if", "ifnot", "default":
		return true
	default:
		return false
	}
}

func getFuncArgForOptimization(funcName string, args []Expr) Expr {
	idx := getFuncArgIdxForOptimization(funcName, args)
	if idx < 0 || idx >= len(args) {
		return nil
	}
	return args[idx]
}

func getFuncArgIdxForOptimization(funcName string, args []Expr) int {
	funcName = strings.ToLower(funcName)
	if IsRollupFunc(funcName) {
		return getRollupArgIdxForOptimization(funcName, args)
	}
	if IsTransformFunc(funcName) {
		return getTransformArgIdxForOptimization(funcName, args)
	}
	if isAggrFunc(funcName) {
		return getAggrArgIdxForOptimization(funcName, args)
	}
	return -1
}

func getAggrArgIdxForOptimization(funcName string, args []Expr) int {
	switch strings.ToLower(funcName) {
	case "bottomk", "bottomk_avg", "bottomk_max", "bottomk_median", "bottomk_last", "bottomk_min",
		"limitk", "outliers_mad", "outliersk", "quantile",
		"topk", "topk_avg", "topk_max", "topk_median", "topk_last", "topk_min":
		return 1
	case "count_values":
		return -1
	case "quantiles":
		return len(args) - 1
	default:
		return 0
	}
}

func getRollupArgIdxForOptimization(funcName string, args []Expr) int {
	// This must be kept in sync with GetRollupArgIdx()
	switch strings.ToLower(funcName) {
	case "absent_over_time":
		return -1
	case "quantile_over_time", "aggr_over_time",
		"hoeffding_bound_lower", "hoeffding_bound_upper":
		return 1
	case "quantiles_over_time":
		return len(args) - 1
	default:
		return 0
	}
}

func getTransformArgIdxForOptimization(funcName string, args []Expr) int {
	funcName = strings.ToLower(funcName)
	if isLabelManipulationFunc(funcName) {
		return -1
	}
	switch funcName {
	case "", "absent", "scalar", "union":
		return -1
	case "end", "now", "pi", "ru", "start", "step", "time":
		return -1
	case "limit_offset":
		return 2
	case "buckets_limit", "histogram_quantile", "histogram_share", "range_quantile":
		return 1
	case "histogram_quantiles":
		return len(args) - 1
	default:
		return 0
	}
}

func isLabelManipulationFunc(funcName string) bool {
	switch strings.ToLower(funcName) {
	case "alias", "label_copy", "label_del", "label_graphite_group", "label_join", "label_keep", "label_lowercase",
		"label_map", "label_match", "label_mismatch", "label_move", "label_replace", "label_set", "label_transform",
		"label_uppercase", "label_value":
		return true
	default:
		return false
	}
}
