package apptest

import (
	"fmt"
	"slices"
)

type TestData struct {
	Samples              []string
	Step                 int64
	WantSeries           []map[string]string
	WantLabels           []string
	WantLabelValues      []string
	WantQueryResults     []*QueryResult
	WantMetadata         map[string][]MetadataEntry
	WantMetricNamesStats []MetricNamesStatsRecord
}

func GenerateTestData(prefix string, numMetrics, start, end int64) TestData {
	d := TestData{
		Samples:              []string{},
		Step:                 (end - start) / numMetrics,
		WantSeries:           make([]map[string]string, numMetrics),
		WantLabels:           make([]string, numMetrics),
		WantLabelValues:      make([]string, numMetrics),
		WantQueryResults:     make([]*QueryResult, numMetrics),
		WantMetadata:         make(map[string][]MetadataEntry),
		WantMetricNamesStats: make([]MetricNamesStatsRecord, numMetrics),
	}
	for i := range numMetrics {
		metricName := fmt.Sprintf("%s_%04d", prefix, i)
		metricHelp := fmt.Sprintf("# HELP %s some help message", metricName)
		metricType := fmt.Sprintf("# TYPE %s gauge", metricName)
		labelName := fmt.Sprintf("label_%04d", i)
		labelValue := fmt.Sprintf("value_%04d", i)
		value := i
		timestamp := start + i*d.Step
		sample := fmt.Sprintf(`%s{%s="value", label="%s"} %d %d`, metricName, labelName, labelValue, value, timestamp)

		d.Samples = append(d.Samples, metricHelp, metricType, sample)
		d.WantSeries[i] = map[string]string{
			"__name__": metricName,
			labelName:  "value",
			"label":    labelValue,
		}
		d.WantLabels[i] = labelName
		d.WantLabelValues[i] = labelValue
		d.WantQueryResults[i] = &QueryResult{
			Metric: map[string]string{
				"__name__": metricName,
				labelName:  "value",
				"label":    labelValue,
			},
			Samples: []*Sample{{Timestamp: timestamp, Value: float64(value)}},
		}
		d.WantMetadata[metricName] = []MetadataEntry{{Help: "some help message", Type: "gauge"}}
		d.WantMetricNamesStats[i].MetricName = metricName
	}
	d.WantLabels = append(d.WantLabels, "__name__", "label")
	slices.Sort(d.WantLabels)
	return d
}

// AssertSeries retrieves metric names from the storage and compares the result
// with the expected one.
func AssertSeries(tc *TestCase, app PrometheusQuerier, metricNameRE, tenantID string, start, end int64, want []map[string]string) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/series response",
		Got: func() any {
			tc.T().Helper()
			return app.PrometheusAPIV1Series(tc.T(), query, QueryOpts{
				Tenant: tenantID,
				Start:  fmt.Sprintf("%d", start),
				End:    fmt.Sprintf("%d", end),
			}).Sort()
		},
		Want: &PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   want,
		},
		Retries: 1000,
		FailNow: true,
	})
}

// AssertSeriesCount retrieves series count and compares it with expected one.
func AssertSeriesCount(tc *TestCase, app PrometheusQuerier, tenantID string, start, end int64, want uint64) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/series/count response",
		Got: func() any {
			tc.T().Helper()
			return app.PrometheusAPIV1SeriesCount(tc.T(), QueryOpts{
				Tenant: tenantID,
				Start:  fmt.Sprintf("%d", start),
				End:    fmt.Sprintf("%d", end),
			})
		},
		Want: &PrometheusAPIV1SeriesCountResponse{
			Status: "success",
			Data:   []uint64{want},
		},
		FailNow: true,
	})
}

// AssertLabels retrieves label names from the storage and compares the result
// with the expected one.
func AssertLabels(tc *TestCase, app PrometheusQuerier, metricNameRE, tenantID string, start, end int64, want []string) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/labels response",
		Got: func() any {
			tc.T().Helper()
			res := app.PrometheusAPIV1Labels(tc.T(), query, QueryOpts{
				Tenant: tenantID,
				Start:  fmt.Sprintf("%d", start),
				End:    fmt.Sprintf("%d", end),
			})
			slices.Sort(res.Data)
			return res
		},
		Want: &PrometheusAPIV1LabelsResponse{
			Status: "success",
			Data:   want,
		},
		FailNow: true,
	})
}

// AssertLabelValues retrieves values for the label whose name is labelName for
// the series whose name mathes metricNameRE, compares the result with the
// expected one.
func AssertLabelValues(tc *TestCase, app PrometheusQuerier, metricNameRE, labelName, tenantID string, start, end int64, want []string) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/labels/.../values response",
		Got: func() any {
			tc.T().Helper()
			res := app.PrometheusAPIV1LabelValues(tc.T(), labelName, query, QueryOpts{
				Tenant: tenantID,
				Start:  fmt.Sprintf("%d", start),
				End:    fmt.Sprintf("%d", end),
			})
			slices.Sort(res.Data)
			return res
		},
		Want: &PrometheusAPIV1LabelValuesResponse{
			Status: "success",
			Data:   want,
		},
		FailNow: true,
	})
}

// AssertQueryResults sends a data query to storage and compares the query
// result with the expected one.
func AssertQueryResults(tc *TestCase, app PrometheusQuerier, metricNameRE, tenantID string, start, end, step int64, want []*QueryResult) {
	tc.T().Helper()

	query := fmt.Sprintf(`{__name__=~"%s"}`, metricNameRE)
	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/query_range response",
		Got: func() any {
			tc.T().Helper()
			return app.PrometheusAPIV1QueryRange(tc.T(), query, QueryOpts{
				Tenant:      tenantID,
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

func AssertMetadata(tc *TestCase, app PrometheusQuerier, metricName, tenantID string, want map[string][]MetadataEntry) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/metadata response",
		Got: func() any {
			tc.T().Helper()
			return app.PrometheusAPIV1Metadata(tc.T(), metricName, 0, QueryOpts{
				Tenant: tenantID,
			})
		},
		Want: &PrometheusAPIV1Metadata{
			Status: "success",
			Data:   want,
		},
		FailNow: true,
	})
}

func AssertMetricNamesStats(tc *TestCase, app PrometheusQuerier, metricNameRE, tenantID string, want []MetricNamesStatsRecord) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /prometheus/api/v1/status/metric_names_stats response",
		Got: func() any {
			tc.T().Helper()
			return app.PrometheusAPIV1StatusMetricNamesStats(tc.T(), "", "", metricNameRE, QueryOpts{
				Tenant: tenantID,
			})
		},
		Want: MetricNamesStatsResponse{
			Records: want,
		},
		FailNow: true,
	})

}

// GraphiteTestData holds the data samples in Graphite Pickle format, distance
// between samples in milliseconds and expected responses for various Graphite
// API endpoints.
type GraphiteTestData struct {
	Samples             []string
	Step                int64
	WantMetricsIndex    []string
	WantMetricsFind     []GraphiteMetric
	WantMetricsExpand   []string
	WantRenderedTargets []GraphiteRenderedTarget
}

// GenerateGraphiteTestData generates Graphite test data.
func GenerateGraphiteTestData(prefix string, numMetrics, start, end int64) GraphiteTestData {
	d := GraphiteTestData{
		Samples:             make([]string, numMetrics),
		Step:                (end - start) / numMetrics,
		WantMetricsIndex:    make([]string, numMetrics),
		WantMetricsFind:     make([]GraphiteMetric, numMetrics),
		WantMetricsExpand:   make([]string, numMetrics),
		WantRenderedTargets: make([]GraphiteRenderedTarget, numMetrics),
	}

	datapoints := make([][2]float64, numMetrics)
	for i := range numMetrics {
		timestamp := (start + i*d.Step) / 1000
		datapoints[i][1] = float64(timestamp)
	}

	for i := range numMetrics {
		suffix := fmt.Sprintf("%04d", i)
		metricName := fmt.Sprintf("%s.%s", prefix, suffix)
		value := i
		timestamp := (start + i*d.Step) / 1000
		sample := fmt.Sprintf(`%s %d %d`, metricName, value, timestamp)

		d.Samples[i] = sample
		d.WantMetricsIndex[i] = metricName
		d.WantMetricsFind[i].Id = metricName
		d.WantMetricsFind[i].Text = suffix
		d.WantMetricsFind[i].Leaf = 1
		d.WantMetricsExpand[i] = metricName
		d.WantRenderedTargets[i].Target = metricName
		d.WantRenderedTargets[i].Datapoints = slices.Clone(datapoints)
		d.WantRenderedTargets[i].Datapoints[i][0] = float64(value)
	}
	return d
}

// AssertGraphiteMetricsIndex retrieves all metrics by sending a request to
// /graphite/metrics/index.json and compares the result with the expected one.
func AssertGraphiteMetricsIndex(tc *TestCase, app PrometheusQuerier, tenantID string, want []string) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /graphite/metrics/index.json response",
		Got: func() any {
			tc.T().Helper()
			return app.GraphiteMetricsIndex(tc.T(), QueryOpts{
				Tenant: tenantID,
			})
		},
		Want:    want,
		Retries: 30,
		FailNow: true,
	})

}

// AssertGraphiteMetricsFind finds metric names by sending a request to
// /graphite/metrics/find and compares the result with the expected one.
func AssertGraphiteMetricsFind(tc *TestCase, app PrometheusQuerier, query, tenantID string, want []GraphiteMetric) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /graphite/metrics/find response",
		Got: func() any {
			tc.T().Helper()
			return app.GraphiteMetricsFind(tc.T(), query, QueryOpts{
				Tenant: tenantID,
			})
		},
		Want:    want,
		FailNow: true,
	})

}

// AssertGraphiteMetricsFind expands metric names by sending a request to
// /graphite/metrics/expand and compares the result with the expected one.
func AssertGraphiteMetricsExpand(tc *TestCase, app PrometheusQuerier, query, tenantID string, want []string) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /graphite/metrics/expand response",
		Got: func() any {
			tc.T().Helper()
			return app.GraphiteMetricsExpand(tc.T(), query, QueryOpts{
				Tenant: tenantID,
			})
		},
		Want:    want,
		FailNow: true,
	})

}

// AssertGraphiteRender retieves metric raw data by sending a request to
// /graphite/render and compares the result with the expected one.
func AssertGraphiteRender(tc *TestCase, app PrometheusQuerier, target, tenantID string, from, until, step int64, want []GraphiteRenderedTarget) {
	tc.T().Helper()

	tc.Assert(&AssertOptions{
		Msg: "unexpected /graphite/render response",
		Got: func() any {
			tc.T().Helper()
			return app.GraphiteRender(tc.T(), target, QueryOpts{
				Tenant:      tenantID,
				From:        fmt.Sprintf("%d", from/1000),
				Until:       fmt.Sprintf("%d", until/1000),
				StorageStep: fmt.Sprintf("%dms", step),
			})
		},
		Want:    want,
		FailNow: true,
	})
}
