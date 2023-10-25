package unittest

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metricsql"
)

// metricsqlTestCase holds metricsql_expr_test cases defined in test file
type metricsqlTestCase struct {
	Expr       string              `yaml:"expr"`
	EvalTime   *promutils.Duration `yaml:"eval_time"`
	ExpSamples []expSample         `yaml:"exp_samples"`
}

type expSample struct {
	Labels string  `yaml:"labels"`
	Value  float64 `yaml:"value"`
}

// checkMetricsqlCase will check metricsql_expr_test cases
func checkMetricsqlCase(cases []metricsqlTestCase, q datasource.QuerierBuilder) (checkErrs []error) {
	queries := q.BuildWithParams(datasource.QuerierParams{QueryParams: url.Values{"nocache": {"1"}, "latency_offset": {"1ms"}}, DataSourceType: "prometheus"})
Outer:
	for _, mt := range cases {
		result, _, err := queries.Query(context.Background(), mt.Expr, durationToTime(mt.EvalTime))
		if err != nil {
			checkErrs = append(checkErrs, fmt.Errorf("    expr: %q, time: %s, err: %w", mt.Expr,
				mt.EvalTime.Duration().String(), err))
			continue
		}
		var gotSamples []parsedSample
		for _, s := range result.Data {
			sort.Slice(s.Labels, func(i, j int) bool {
				return s.Labels[i].Name < s.Labels[j].Name
			})
			gotSamples = append(gotSamples, parsedSample{
				Labels: s.Labels,
				Value:  s.Values[0],
			})
		}
		var expSamples []parsedSample
		for _, s := range mt.ExpSamples {
			expLb := datasource.Labels{}
			if s.Labels != "" {
				metricsqlExpr, err := metricsql.Parse(s.Labels)
				if err != nil {
					checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s, err: %v", mt.Expr,
						mt.EvalTime.Duration().String(), fmt.Errorf("failed to parse labels %q: %w", s.Labels, err)))
					continue Outer
				}
				metricsqlMetricExpr, ok := metricsqlExpr.(*metricsql.MetricExpr)
				if !ok {
					checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s, err: %v", mt.Expr,
						mt.EvalTime.Duration().String(), fmt.Errorf("got unsupported metricsql type")))
					continue Outer
				}
				for _, l := range metricsqlMetricExpr.LabelFilterss[0] {
					expLb = append(expLb, datasource.Label{
						Name:  l.Label,
						Value: l.Value,
					})
				}
			}
			sort.Slice(expLb, func(i, j int) bool {
				return expLb[i].Name < expLb[j].Name
			})
			expSamples = append(expSamples, parsedSample{
				Labels: expLb,
				Value:  s.Value,
			})
		}
		sort.Slice(expSamples, func(i, j int) bool {
			return datasource.LabelCompare(expSamples[i].Labels, expSamples[j].Labels) <= 0
		})
		sort.Slice(gotSamples, func(i, j int) bool {
			return datasource.LabelCompare(gotSamples[i].Labels, gotSamples[j].Labels) <= 0
		})
		if !reflect.DeepEqual(expSamples, gotSamples) {
			checkErrs = append(checkErrs, fmt.Errorf("\n    expr: %q, time: %s,\n        exp: %v\n        got: %v", mt.Expr,
				mt.EvalTime.Duration().String(), parsedSamplesString(expSamples), parsedSamplesString(gotSamples)))
		}

	}
	return
}

func durationToTime(pd *promutils.Duration) time.Time {
	if pd == nil {
		return time.Time{}
	}
	return time.UnixMilli(pd.Duration().Milliseconds())
}
