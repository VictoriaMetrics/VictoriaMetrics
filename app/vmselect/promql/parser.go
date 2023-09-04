package promql

import (
	"github.com/VictoriaMetrics/metricsql"
)

// IsRollup verifies whether s is a rollup with non-empty window.
//
// It returns the wrapped query with the corresponding window, step and offset.
func IsRollup(s string) (childQuery string, window, step, offset *metricsql.DurationExpr) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*metricsql.RollupExpr)
	if !ok || re.Window == nil {
		return
	}
	wrappedQuery := re.Expr.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Step, re.Offset
}

// IsMetricSelectorWithRollup verifies whether s contains PromQL metric selector
// wrapped into rollup.
//
// It returns the wrapped query with the corresponding window with offset.
func IsMetricSelectorWithRollup(s string) (childQuery string, window, offset *metricsql.DurationExpr) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*metricsql.RollupExpr)
	if !ok || re.Window == nil || re.Step != nil {
		return
	}
	me, ok := re.Expr.(*metricsql.MetricExpr)
	if !ok || len(me.LabelFilterss) == 0 {
		return
	}
	wrappedQuery := me.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Offset
}
