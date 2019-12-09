package promql

import (
	"strings"
)

var rollupFuncs = map[string]bool{
	"default_rollup": true, // default rollup func

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

	// Additional rollup funcs.
	"sum2_over_time":      true,
	"geomean_over_time":   true,
	"first_over_time":     true,
	"last_over_time":      true,
	"distinct_over_time":  true,
	"increases_over_time": true,
	"decreases_over_time": true,
	"integrate":           true,
	"ideriv":              true,
	"lifetime":            true,
	"lag":                 true,
	"scrape_interval":     true,
	"rollup":              true,
	"rollup_rate":         true,
	"rollup_deriv":        true,
	"rollup_delta":        true,
	"rollup_increase":     true,
	"rollup_candlestick":  true,
}

func isRollupFunc(funcName string) bool {
	funcName = strings.ToLower(funcName)
	return rollupFuncs[funcName]
}
