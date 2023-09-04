package metricsql

import (
	"strings"
)

var aggrFuncs = map[string]bool{
	"any":            true,
	"avg":            true,
	"bottomk":        true,
	"bottomk_avg":    true,
	"bottomk_max":    true,
	"bottomk_median": true,
	"bottomk_last":   true,
	"bottomk_min":    true,
	"count":          true,
	"count_values":   true,
	"distinct":       true,
	"geomean":        true,
	"group":          true,
	"histogram":      true,
	"limitk":         true,
	"mad":            true,
	"max":            true,
	"median":         true,
	"min":            true,
	"mode":           true,
	"outliers_mad":   true,
	"outliersk":      true,
	"quantile":       true,
	"quantiles":      true,
	"share":          true,
	"stddev":         true,
	"stdvar":         true,
	"sum":            true,
	"sum2":           true,
	"topk":           true,
	"topk_avg":       true,
	"topk_max":       true,
	"topk_median":    true,
	"topk_last":      true,
	"topk_min":       true,
	"zscore":         true,
}

func isAggrFunc(s string) bool {
	s = strings.ToLower(s)
	return aggrFuncs[s]
}

func isAggrFuncModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	case "by", "without":
		return true
	default:
		return false
	}
}
