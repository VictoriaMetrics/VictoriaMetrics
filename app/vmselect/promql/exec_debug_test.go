package promql

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestSimulatedExec(t *testing.T) {
	accountID := uint32(123)
	projectID := uint32(567)
	start := int64(1000e3)
	end := int64(2000e3)
	step := int64(200e3)

	// Base EvalConfig that will be copied for each test
	baseEC := EvalConfig{
		AuthTokens: []*auth.Token{{
			AccountID: accountID,
			ProjectID: projectID,
		}},
		Start:              start,
		End:                end,
		Step:               step,
		MaxPointsPerSeries: 1e4,
		MaxSeries:          1000,
		Deadline:           searchutil.NewDeadline(time.Now(), time.Hour, ""),
		RoundDigits:        100,
		MayCache:           false,
	}

	t.Run(`simple_metric_exact_match`, func(t *testing.T) {
		ec := copyEvalConfig(&baseEC)
		mn := newMetric(accountID, projectID,
			"__name__", "test_metric",
			"a", "b",
		)

		ec.SimulatedSamples = []*storage.SimulatedSample{
			{
				MetricName: mn.metric,
				Timestamps: mn.timestamps,
				Value:      mn.values,
			},
		}

		q := `test_metric{a="b"}`
		result, err := Exec(nil, ec, q, false)
		if err != nil {
			t.Fatalf(`unexpected error when executing %q: %s`, q, err)
		}

		// Expected result
		expectedMN := storage.MetricName{
			AccountID:   accountID,
			ProjectID:   projectID,
			MetricGroup: []byte("test_metric"),
			Tags: []storage.Tag{
				{
					Key:   []byte("a"),
					Value: []byte("b"),
				},
			},
		}
		expectedResult := []netstorage.Result{
			{
				MetricName: expectedMN,
				Values:     mn.values,
				Timestamps: mn.timestamps,
			},
		}

		testResultsEqual(t, result, expectedResult, false)
	})

	// t.Run(`filtered_by_tag_value`, func(t *testing.T) {
	// 	// Create a copy of base EvalConfig
	// 	ec := copyEvalConfig(&baseEC)

	// 	// Create two simulated samples but query should match only one
	// 	mn1 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn1.AddTag("__name__", "test_metric")
	// 	mn1.AddTag("a", "b")
	// 	mn1.AddTag("region", "us-west")

	// 	mn2 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn2.AddTag("__name__", "test_metric")
	// 	mn2.AddTag("a", "b")
	// 	mn2.AddTag("region", "us-east")

	// 	ts := make([]int64, len(timestampsExpected))
	// 	copy(ts, timestampsExpected)

	// 	values1 := []float64{10, 20, 30, 40, 50, 60}
	// 	values2 := []float64{15, 25, 35, 45, 55, 65}

	// 	ec.SimulatedSamples = []*storage.SimulatedSample{
	// 		{
	// 			MetricName: mn1,
	// 			Timestamps: ts,
	// 			Value:      values1,
	// 		},
	// 		{
	// 			MetricName: mn2,
	// 			Timestamps: ts,
	// 			Value:      values2,
	// 		},
	// 	}

	// 	q := `test_metric{a="b", region="us-west"}`
	// 	result, err := Exec(nil, ec, q, false)
	// 	if err != nil {
	// 		t.Fatalf(`unexpected error when executing %q: %s`, q, err)
	// 	}

	// 	// Expected result
	// 	expectedMN := storage.MetricName{
	// 		AccountID:   accountID,
	// 		ProjectID:   projectID,
	// 		MetricGroup: []byte("test_metric"),
	// 		Tags: []storage.Tag{
	// 			{
	// 				Key:   []byte("a"),
	// 				Value: []byte("b"),
	// 			},
	// 			{
	// 				Key:   []byte("region"),
	// 				Value: []byte("us-west"),
	// 			},
	// 		},
	// 	}
	// 	expectedResult := []netstorage.Result{
	// 		{
	// 			MetricName: expectedMN,
	// 			Values:     values1,
	// 			Timestamps: timestampsExpected,
	// 		},
	// 	}

	// 	testResultsEqual(t, result, expectedResult, false)
	// })

	// t.Run(`function_on_simulated_data`, func(t *testing.T) {
	// 	// Create a copy of base EvalConfig
	// 	ec := copyEvalConfig(&baseEC)

	// 	// Test that functions work on simulated data
	// 	mn := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn.AddTag("__name__", "test_metric")
	// 	mn.AddTag("a", "b")

	// 	ts := make([]int64, len(timestampsExpected))
	// 	copy(ts, timestampsExpected)

	// 	// Creating values with a known rate (50 per 200s)
	// 	values := []float64{10, 20, 30, 40, 50, 60}

	// 	ec.SimulatedSamples = []*storage.SimulatedSample{
	// 		{
	// 			MetricName: mn,
	// 			Timestamps: ts,
	// 			Value:      values,
	// 		},
	// 	}

	// 	q := `rate(test_metric{a="b"}[200s])`
	// 	result, err := Exec(nil, ec, q, false)
	// 	if err != nil {
	// 		t.Fatalf(`unexpected error when executing %q: %s`, q, err)
	// 	}

	// 	// Expected result - the rate should be 50 per 200s
	// 	expectedMN := storage.MetricName{
	// 		AccountID:   accountID,
	// 		ProjectID:   projectID,
	// 		MetricGroup: []byte("test_metric"),
	// 		Tags: []storage.Tag{
	// 			{
	// 				Key:   []byte("a"),
	// 				Value: []byte("b"),
	// 			},
	// 		},
	// 	}

	// 	// The exact values might be implementation-dependent, so we'll verify the result has data
	// 	if len(result) == 0 {
	// 		t.Fatalf("expected non-empty result for rate function")
	// 	}

	// 	// Verify the metric name matches
	// 	testMetricNamesEqual(t, &result[0].MetricName, &expectedMN, false, 0)
	// })

	// t.Run(`regex_filter_on_simulated_data`, func(t *testing.T) {
	// 	// Create a copy of base EvalConfig
	// 	ec := copyEvalConfig(&baseEC)

	// 	// Test regex filters on simulated data
	// 	mn1 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn1.AddTag("__name__", "test_metric")
	// 	mn1.AddTag("instance", "server1")

	// 	mn2 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn2.AddTag("__name__", "test_metric")
	// 	mn2.AddTag("instance", "server2")

	// 	mn3 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn3.AddTag("__name__", "test_metric")
	// 	mn3.AddTag("instance", "backend1")

	// 	ts := make([]int64, len(timestampsExpected))
	// 	copy(ts, timestampsExpected)

	// 	values1 := []float64{10, 20, 30, 40, 50, 60}
	// 	values2 := []float64{15, 25, 35, 45, 55, 65}
	// 	values3 := []float64{5, 15, 25, 35, 45, 55}

	// 	ec.SimulatedSamples = []*storage.SimulatedSample{
	// 		{
	// 			MetricName: mn1,
	// 			Timestamps: ts,
	// 			Value:      values1,
	// 		},
	// 		{
	// 			MetricName: mn2,
	// 			Timestamps: ts,
	// 			Value:      values2,
	// 		},
	// 		{
	// 			MetricName: mn3,
	// 			Timestamps: ts,
	// 			Value:      values3,
	// 		},
	// 	}

	// 	q := `test_metric{instance=~"server.*"}`
	// 	result, err := Exec(nil, ec, q, false)
	// 	if err != nil {
	// 		t.Fatalf(`unexpected error when executing %q: %s`, q, err)
	// 	}

	// 	// Should match two series
	// 	if len(result) != 2 {
	// 		t.Fatalf("expected 2 results for regex filter, got %d", len(result))
	// 	}
	// })

	// t.Run(`negative_filter_on_simulated_data`, func(t *testing.T) {
	// 	// Create a copy of base EvalConfig
	// 	ec := copyEvalConfig(&baseEC)

	// 	// Test negative filters on simulated data
	// 	mn1 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn1.AddTag("__name__", "test_metric")
	// 	mn1.AddTag("env", "prod")

	// 	mn2 := storage.GetMetricNameNoCache(accountID, projectID)
	// 	mn2.AddTag("__name__", "test_metric")
	// 	mn2.AddTag("env", "dev")

	// 	ts := make([]int64, len(timestampsExpected))
	// 	copy(ts, timestampsExpected)

	// 	values1 := []float64{10, 20, 30, 40, 50, 60}
	// 	values2 := []float64{15, 25, 35, 45, 55, 65}

	// 	ec.SimulatedSamples = []*storage.SimulatedSample{
	// 		{
	// 			MetricName: mn1,
	// 			Timestamps: ts,
	// 			Value:      values1,
	// 		},
	// 		{
	// 			MetricName: mn2,
	// 			Timestamps: ts,
	// 			Value:      values2,
	// 		},
	// 	}

	// 	q := `test_metric{env!="dev"}`
	// 	result, err := Exec(nil, ec, q, false)
	// 	if err != nil {
	// 		t.Fatalf(`unexpected error when executing %q: %s`, q, err)
	// 	}

	// 	// Should match one series
	// 	if len(result) != 1 {
	// 		t.Fatalf("expected 1 result for negative filter, got %d", len(result))
	// 	}

	// 	// Expected result
	// 	expectedMN := storage.MetricName{
	// 		AccountID:   accountID,
	// 		ProjectID:   projectID,
	// 		MetricGroup: []byte("test_metric"),
	// 		Tags: []storage.Tag{
	// 			{
	// 				Key:   []byte("env"),
	// 				Value: []byte("prod"),
	// 			},
	// 		},
	// 	}

	// 	// Verify the metric name matches
	// 	testMetricNamesEqual(t, &result[0].MetricName, &expectedMN, false, 0)
	// })
}

type metricBuilder struct {
	metric     *storage.MetricName
	timestamps []int64
	values     []float64
}

func newMetric(accountID uint32, projectID uint32, pairs ...string) *metricBuilder {
	mn := &storage.MetricName{
		AccountID: accountID,
		ProjectID: projectID,
	}
	for i := 0; i < len(pairs); i += 2 {
		mn.AddTag(pairs[i], pairs[i+1])
	}
	return &metricBuilder{
		metric:     mn,
		values:     []float64{10, 20, 30, 40, 50, 60},
		timestamps: []int64{1000e3, 1200e3, 1400e3, 1600e3, 1800e3, 2000e3},
	}
}

func (b *metricBuilder) withTimestamps(unixTimestamps ...int64) *metricBuilder {
	b.timestamps = make([]int64, len(unixTimestamps))
	for i := range unixTimestamps {
		b.timestamps[i] = unixTimestamps[i] * 1e3
	}
	return b
}

func (b *metricBuilder) withValues(values ...float64) *metricBuilder {
	b.values = values
	return b
}
