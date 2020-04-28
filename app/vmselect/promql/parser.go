package promql

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
)

// IsRollup verifies whether s is a rollup with non-empty window.
//
// It returns the wrapped query with the corresponding window, step and offset.
func IsRollup(s string) (childQuery string, window, step, offset string) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*metricsql.RollupExpr)
	if !ok || len(re.Window) == 0 {
		return
	}
	wrappedQuery := re.Expr.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Step, re.Offset
}

// IsMetricSelectorWithRollup verifies whether s contains PromQL metric selector
// wrapped into rollup.
//
// It returns the wrapped query with the corresponding window with offset.
func IsMetricSelectorWithRollup(s string) (childQuery string, window, offset string) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*metricsql.RollupExpr)
	if !ok || len(re.Window) == 0 || len(re.Step) > 0 {
		return
	}
	me, ok := re.Expr.(*metricsql.MetricExpr)
	if !ok || len(me.LabelFilters) == 0 {
		return
	}
	wrappedQuery := me.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Offset
}

// ParseMetricSelector parses s containing PromQL metric selector
// and returns the corresponding LabelFilters.
func ParseMetricSelector(s string) ([]storage.TagFilter, error) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("expecting metricSelector; got %q", expr.AppendString(nil))
	}
	if len(me.LabelFilters) == 0 {
		return nil, fmt.Errorf("labelFilters cannot be empty")
	}
	tfs := toTagFilters(me.LabelFilters)
	return tfs, nil
}
