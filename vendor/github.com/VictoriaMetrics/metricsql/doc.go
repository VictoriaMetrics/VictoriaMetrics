// Package metricsql implements MetricsQL parser.
//
// This parser can parse PromQL. Additionally it can parse all the MetricsQL extensions.
// See https://docs.victoriametrics.com/metricsql/ for details about MetricsQL.
//
// Usage:
//
//	expr, err := metricsql.Parse(`sum(rate(foo{bar="baz"}[5m])) by (job)`)
//	if err != nil {
//	    // parse error
//	}
//	// Now expr contains parsed MetricsQL as `*Expr` structs.
//	// See Parse examples for more details.
package metricsql
