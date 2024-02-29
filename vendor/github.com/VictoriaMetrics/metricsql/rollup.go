package metricsql

import (
	"strings"
)

var rollupFuncs = map[string]bool{
	"absent_over_time":        true,
	"aggr_over_time":          true,
	"ascent_over_time":        true,
	"avg_over_time":           true,
	"changes":                 true,
	"changes_prometheus":      true,
	"count_eq_over_time":      true,
	"count_gt_over_time":      true,
	"count_le_over_time":      true,
	"count_ne_over_time":      true,
	"count_over_time":         true,
	"count_values_over_time":  true,
	"decreases_over_time":     true,
	"default_rollup":          true,
	"delta":                   true,
	"delta_prometheus":        true,
	"deriv":                   true,
	"deriv_fast":              true,
	"descent_over_time":       true,
	"distinct_over_time":      true,
	"duration_over_time":      true,
	"first_over_time":         true,
	"geomean_over_time":       true,
	"histogram_over_time":     true,
	"hoeffding_bound_lower":   true,
	"hoeffding_bound_upper":   true,
	"holt_winters":            true,
	"idelta":                  true,
	"ideriv":                  true,
	"increase":                true,
	"increase_prometheus":     true,
	"increase_pure":           true,
	"increases_over_time":     true,
	"integrate":               true,
	"irate":                   true,
	"lag":                     true,
	"last_over_time":          true,
	"lifetime":                true,
	"mad_over_time":           true,
	"max_over_time":           true,
	"median_over_time":        true,
	"min_over_time":           true,
	"mode_over_time":          true,
	"outlier_iqr_over_time":   true,
	"predict_linear":          true,
	"present_over_time":       true,
	"quantile_over_time":      true,
	"quantiles_over_time":     true,
	"range_over_time":         true,
	"rate":                    true,
	"rate_over_sum":           true,
	"resets":                  true,
	"rollup":                  true,
	"rollup_candlestick":      true,
	"rollup_delta":            true,
	"rollup_deriv":            true,
	"rollup_increase":         true,
	"rollup_rate":             true,
	"rollup_scrape_interval":  true,
	"scrape_interval":         true,
	"share_gt_over_time":      true,
	"share_le_over_time":      true,
	"share_eq_over_time":      true,
	"stale_samples_over_time": true,
	"stddev_over_time":        true,
	"stdvar_over_time":        true,
	"sum_eq_over_time":        true,
	"sum_gt_over_time":        true,
	"sum_le_over_time":        true,
	"sum_over_time":           true,
	"sum2_over_time":          true,
	"tfirst_over_time":        true,
	// `timestamp` function must return timestamp for the last datapoint on the current window
	// in order to properly handle offset and timestamps unaligned to the current step.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/415 for details.
	"timestamp":              true,
	"timestamp_with_name":    true,
	"tlast_change_over_time": true,
	"tlast_over_time":        true,
	"tmax_over_time":         true,
	"tmin_over_time":         true,
	"zscore_over_time":       true,
}

// IsRollupFunc returns whether funcName is known rollup function.
func IsRollupFunc(funcName string) bool {
	s := strings.ToLower(funcName)
	return rollupFuncs[s]
}

// GetRollupArgIdx returns the argument index for the given fe, which accepts the rollup argument.
//
// -1 is returned if fe isn't a rollup function.
func GetRollupArgIdx(fe *FuncExpr) int {
	funcName := strings.ToLower(fe.Name)
	if !rollupFuncs[funcName] {
		return -1
	}
	switch funcName {
	case "quantile_over_time", "aggr_over_time", "count_values_over_time",
		"hoeffding_bound_lower", "hoeffding_bound_upper":
		return 1
	case "quantiles_over_time":
		return len(fe.Args) - 1
	default:
		return 0
	}
}
