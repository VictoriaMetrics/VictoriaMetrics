package metricsql

import (
	"strings"
)

var rollupFuncs = map[string]bool{
	// Standard rollup funcs from PromQL.
	// See funcs accepting range-vector on https://prometheus.io/docs/prometheus/latest/querying/functions/ .
	"changes":            true,
	"delta":              true,
	"deriv":              true,
	"deriv_fast":         true,
	"holt_winters":       true,
	"idelta":             true,
	"increase":           true,
	"irate":              true,
	"predict_linear":     true,
	"rate":               true,
	"resets":             true,
	"avg_over_time":      true,
	"min_over_time":      true,
	"max_over_time":      true,
	"sum_over_time":      true,
	"count_over_time":    true,
	"quantile_over_time": true,
	"stddev_over_time":   true,
	"stdvar_over_time":   true,
	"absent_over_time":   true,

	// Additional rollup funcs.
	"default_rollup":        true,
	"range_over_time":       true,
	"sum2_over_time":        true,
	"geomean_over_time":     true,
	"first_over_time":       true,
	"last_over_time":        true,
	"distinct_over_time":    true,
	"increases_over_time":   true,
	"decreases_over_time":   true,
	"integrate":             true,
	"ideriv":                true,
	"lifetime":              true,
	"lag":                   true,
	"scrape_interval":       true,
	"tmin_over_time":        true,
	"tmax_over_time":        true,
	"share_le_over_time":    true,
	"share_gt_over_time":    true,
	"histogram_over_time":   true,
	"rollup":                true,
	"rollup_rate":           true,
	"rollup_deriv":          true,
	"rollup_delta":          true,
	"rollup_increase":       true,
	"rollup_candlestick":    true,
	"aggr_over_time":        true,
	"hoeffding_bound_upper": true,
	"hoeffding_bound_lower": true,
	"ascent_over_time":      true,
	"descent_over_time":     true,

	// `timestamp` func has been moved here because it must work properly with offsets and samples unaligned to the current step.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/415 for details.
	"timestamp": true,
}

// IsRollupFunc returns whether funcName is known rollup function.
func IsRollupFunc(funcName string) bool {
	s := strings.ToLower(funcName)
	return rollupFuncs[s]
}
