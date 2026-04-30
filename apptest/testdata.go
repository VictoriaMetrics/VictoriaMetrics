package apptest

import "fmt"

type TestData struct {
	Samples          []string
	Step             int64
	WantSeries       []map[string]string
	WantQueryResults []*QueryResult
}

func GenerateTestData(prefix string, numMetrics, start, end int64) TestData {
	samples := make([]string, numMetrics)
	step := (end - start) / numMetrics
	wantSeries := make([]map[string]string, numMetrics)
	wantQueryResults := make([]*QueryResult, numMetrics)
	for i := range numMetrics {
		metricName := fmt.Sprintf("%s_%04d", prefix, i)
		labelName := fmt.Sprintf("label_%04d", i)
		labelValue := fmt.Sprintf("value_%04d", i)
		value := i
		timestamp := start + i*step
		samples[i] = fmt.Sprintf(`%s{%s="value", label="%s"} %d %d`, metricName, labelName, labelValue, value, timestamp)
		wantSeries[i] = map[string]string{
			"__name__": metricName,
			labelName:  "value",
			"label":    labelValue,
		}
		wantQueryResults[i] = &QueryResult{
			Metric: map[string]string{
				"__name__": metricName,
				labelName:  "value",
				"label":    labelValue,
			},
			Samples: []*Sample{{Timestamp: timestamp, Value: float64(value)}},
		}
	}
	return TestData{samples, step, wantSeries, wantQueryResults}
}

// AssertSeries retrieves metric names from the storage and compares the result
// with the expected one.
func AssertSeries(tc *TestCase, app PrometheusQuerier, metricNameRE string, start, end int64, want []map[string]string) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /api/v1/series response",
		Got: func() any {
			return app.PrometheusAPIV1Series(tc.T(), query, QueryOpts{
				Start: fmt.Sprintf("%d", start),
				End:   fmt.Sprintf("%d", end),
			}).Sort()
		},
		Want: &PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   want,
		},
		FailNow: true,
	})
}

// AssertQueryResults sends a data query to storage and compares the query
// result with the expected one.
func AssertQueryResults(tc *TestCase, app PrometheusQuerier, metricNameRE string, start, end, step int64, want []*QueryResult) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return app.PrometheusAPIV1QueryRange(tc.T(), query, QueryOpts{
				Start:       fmt.Sprintf("%d", start),
				End:         fmt.Sprintf("%d", end),
				Step:        fmt.Sprintf("%dms", step),
				MaxLookback: fmt.Sprintf("%dms", step-1),
				NoCache:     "1",
			})
		},
		Want: &PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &QueryData{
				ResultType: "matrix",
				Result:     want,
			},
		},
		FailNow: true,
	})
}
