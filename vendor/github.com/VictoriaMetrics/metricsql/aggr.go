package metricsql

import (
	"strings"
)

var aggrFuncs = map[string]bool{
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators
	"sum":          true,
	"min":          true,
	"max":          true,
	"avg":          true,
	"stddev":       true,
	"stdvar":       true,
	"count":        true,
	"count_values": true,
	"bottomk":      true,
	"topk":         true,
	"quantile":     true,

	// MetricsQL extension funcs
	"median":         true,
	"limitk":         true,
	"distinct":       true,
	"sum2":           true,
	"geomean":        true,
	"histogram":      true,
	"topk_min":       true,
	"topk_max":       true,
	"topk_avg":       true,
	"topk_median":    true,
	"bottomk_min":    true,
	"bottomk_max":    true,
	"bottomk_avg":    true,
	"bottomk_median": true,
	"any":            true,
	"outliersk":      true,
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
