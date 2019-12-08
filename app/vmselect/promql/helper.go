package promql

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// IsMetricSelectorWithRollup verifies whether s contains PromQL metric selector
// wrapped into rollup.
//
// It returns the wrapped query with the corresponding window with offset.
func IsMetricSelectorWithRollup(s string) (childQuery string, window, offset string) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*promql.RollupExpr)
	if !ok || len(re.Window) == 0 || len(re.Step) > 0 {
		return
	}
	me, ok := re.Expr.(*promql.MetricExpr)
	if !ok || len(me.TagFilters) == 0 {
		return
	}
	wrappedQuery := me.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Offset
}

// ParseMetricSelector parses s containing PromQL metric selector
// and returns the corresponding TagFilters.
func ParseMetricSelector(s string) ([]storage.TagFilter, error) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*promql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("expecting metricSelector; got %q", expr.AppendString(nil))
	}
	if len(me.TagFilters) == 0 {
		return nil, fmt.Errorf("tagFilters cannot be empty")
	}
	return *(*[]storage.TagFilter)(unsafe.Pointer(&me.TagFilters)), nil
}
