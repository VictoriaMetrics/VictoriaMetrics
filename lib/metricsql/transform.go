package metricsql

import (
	"strings"
)

var transformFuncs = map[string]bool{
	// Standard promql funcs
	// See funcs accepting instant-vector on https://prometheus.io/docs/prometheus/latest/querying/functions/ .
	"abs":                true,
	"absent":             true,
	"ceil":               true,
	"clamp_max":          true,
	"clamp_min":          true,
	"day_of_month":       true,
	"day_of_week":        true,
	"days_in_month":      true,
	"exp":                true,
	"floor":              true,
	"histogram_quantile": true,
	"hour":               true,
	"label_join":         true,
	"label_replace":      true,
	"ln":                 true,
	"log2":               true,
	"log10":              true,
	"minute":             true,
	"month":              true,
	"round":              true,
	"scalar":             true,
	"sort":               true,
	"sort_desc":          true,
	"sqrt":               true,
	"time":               true,
	// "timestamp" has been moved to rollup funcs. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/415
	"vector": true,
	"year":   true,

	// New funcs from MetricsQL
	"label_set":          true,
	"label_map":          true,
	"label_del":          true,
	"label_keep":         true,
	"label_copy":         true,
	"label_move":         true,
	"label_transform":    true,
	"label_value":        true,
	"label_match":        true,
	"label_mismatch":     true,
	"union":              true,
	"":                   true, // empty func is a synonim to union
	"keep_last_value":    true,
	"keep_next_value":    true,
	"start":              true,
	"end":                true,
	"step":               true,
	"running_sum":        true,
	"running_max":        true,
	"running_min":        true,
	"running_avg":        true,
	"range_sum":          true,
	"range_max":          true,
	"range_min":          true,
	"range_avg":          true,
	"range_first":        true,
	"range_last":         true,
	"range_quantile":     true,
	"smooth_exponential": true,
	"remove_resets":      true,
	"rand":               true,
	"rand_normal":        true,
	"rand_exponential":   true,
	"pi":                 true,
	"sin":                true,
	"cos":                true,
	"asin":               true,
	"acos":               true,
	"prometheus_buckets": true,
	"histogram_share":    true,
	"sort_by_label":      true,
	"sort_by_label_desc": true,
}

// IsTransformFunc returns whether funcName is known transform function.
func IsTransformFunc(funcName string) bool {
	s := strings.ToLower(funcName)
	return transformFuncs[s]

}
