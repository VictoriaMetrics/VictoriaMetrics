package promql

import (
	"math"
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
		t.Skip()
		ec := copyEvalConfig(&baseEC)
		mn := newMetric(accountID, projectID,
			"__name__", "test_metric",
			"a", "b",
		)

		ec.SimulatedSamples = []*storage.SimulatedSamples{mn.build()}

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
				Values:     mn.Value,
				Timestamps: mn.Timestamps,
			},
		}

		testResultsEqual(t, result, expectedResult, false)
	})

	t.Run(`filtered_by_tag_value`, func(t *testing.T) {
		t.Skip()

		// Create a copy of base EvalConfig
		ec := copyEvalConfig(&baseEC)
		mn := metricBuilders{
			newMetric(accountID, projectID,
				"__name__", "test_metric",
				"a", "b",
				"region", "us-west",
			),
			newMetric(accountID, projectID,
				"__name__", "test_metric",
				"a", "b",
				"region", "us-east",
			),
		}
		ec.SimulatedSamples = mn.build()

		q := `test_metric{region="us-west"}`
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
				{
					Key:   []byte("region"),
					Value: []byte("us-west"),
				},
			},
		}
		expectedResult := []netstorage.Result{
			{
				MetricName: expectedMN,
				Values:     mn[0].Value,
				Timestamps: mn[0].Timestamps,
			},
		}

		testResultsEqual(t, result, expectedResult, false)
	})

	t.Run(`regex_match_on_tag`, func(t *testing.T) {
		ec := copyEvalConfig(&baseEC)
		mn := metricBuilders{
			newMetric(accountID, projectID,
				"__name__", "test_metric",
				"env", "prod",
			),
			newMetric(accountID, projectID,
				"__name__", "test_metric",
				"env", "staging",
			),
			newMetric(accountID, projectID,
				"__name__", "test_metric",
				"env", "dev",
			),
		}
		ec.SimulatedSamples = mn.build()

		q := `test_metric{env=~"prod|staging"}`
		result, err := Exec(nil, ec, q, false)
		if err != nil {
			t.Fatalf(`unexpected error when executing %q: %s`, q, err)
		}

		expectedResult := []netstorage.Result{mn[0].toResult(), mn[1].toResult()}
		testResultsEqual(t, result, expectedResult, false)
	})
}

func TestSumOverTime(t *testing.T) {
	accountID := uint32(123)
	projectID := uint32(567)
	start := int64(1000e3)
	end := int64(1300e3)
	step := int64(30e3)

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

	t.Run(`basic_sum_over_time`, func(t *testing.T) {
		ec := copyEvalConfig(&baseEC)

		metric := newMetric(accountID, projectID,
			"__name__", "test_metric",
			"app", "api-server",
		).withValues(1, 2, 3, 4, 5, 6).withUnix(1000, 1015, 1030, 1045, 1060, 1075)
		ec.SimulatedSamples = []*storage.SimulatedSamples{metric.build()}

		q := `sum_over_time(test_metric[30s])`
		result, err := Exec(nil, ec, q, false)
		if err != nil {
			t.Fatalf(`unexpected error when executing %q: %s`, q, err)
		}

		expectedResult := []netstorage.Result{
			newMetric(accountID, projectID,
				"app", "api-server",
			).withValues(1, 5, 9, 6).withUnix(1000, 1030, 1060, 1090).toResult(),
		}

		testSimulatedResultsEqual(t, result, expectedResult, false)
	})
}

type metricBuilder storage.SimulatedSamples

func newMetric(accountID uint32, projectID uint32, pairs ...string) *metricBuilder {
	mn := storage.MetricName{
		AccountID: accountID,
		ProjectID: projectID,
	}
	for i := 0; i < len(pairs); i += 2 {
		mn.AddTag(pairs[i], pairs[i+1])
	}
	return &metricBuilder{
		Name:       mn,
		Value:      []float64{10, 20, 30, 40, 50, 60},
		Timestamps: []int64{1000e3, 1200e3, 1400e3, 1600e3, 1800e3, 2000e3},
	}
}

func (b *metricBuilder) withUnix(unix ...int64) *metricBuilder {
	b.Timestamps = make([]int64, len(unix))
	for i := range unix {
		b.Timestamps[i] = unix[i] * 1e3
	}
	return b
}

func (b *metricBuilder) withValues(values ...float64) *metricBuilder {
	b.Value = values
	return b
}

func (b *metricBuilder) build() *storage.SimulatedSamples {
	return (*storage.SimulatedSamples)(b)
}

func (b *metricBuilder) toResult() netstorage.Result {
	return netstorage.Result{
		MetricName: b.Name,
		Values:     b.Value,
		Timestamps: b.Timestamps,
	}
}

type metricBuilders []*metricBuilder

func (b metricBuilders) build() []*storage.SimulatedSamples {
	ss := make([]*storage.SimulatedSamples, len(b))
	for i := range b {
		ss[i] = b[i].build()
	}
	return ss
}

func testSimulatedResultsEqual(t *testing.T, result, resultExpected []netstorage.Result, verifyTenant bool) {
	t.Helper()
	result = removeEmptyValuesAndTimeseries(result)

	if len(result) != len(resultExpected) {
		t.Fatalf(`unexpected timeseries count; got %d; want %d`, len(result), len(resultExpected))
	}
	for i := range result {
		r := &result[i]
		rExpected := &resultExpected[i]
		testMetricNamesEqual(t, &r.MetricName, &rExpected.MetricName, verifyTenant, i)
		testRowsEqual(t, r.Values, r.Timestamps, rExpected.Values, rExpected.Timestamps)
	}
}

func removeEmptyValuesAndTimeseries(tss []netstorage.Result) []netstorage.Result {
	dst := tss[:0]
	for i := range tss {
		ts := &tss[i]
		hasNaNs := false
		for _, v := range ts.Values {
			if math.IsNaN(v) {
				hasNaNs = true
				break
			}
		}
		if !hasNaNs {
			// Fast path: nothing to remove.
			if len(ts.Values) > 0 {
				dst = append(dst, *ts)
			}
			continue
		}

		// Slow path: remove NaNs.
		srcTimestamps := ts.Timestamps
		dstValues := ts.Values[:0]
		// Do not reuse ts.Timestamps for dstTimestamps, since ts.Timestamps
		// may be shared among multiple time series.
		dstTimestamps := make([]int64, 0, len(ts.Timestamps))
		for j, v := range ts.Values {
			if math.IsNaN(v) {
				continue
			}
			dstValues = append(dstValues, v)
			dstTimestamps = append(dstTimestamps, srcTimestamps[j])
		}
		ts.Values = dstValues
		ts.Timestamps = dstTimestamps
		if len(ts.Values) > 0 {
			dst = append(dst, *ts)
		}
	}
	return dst
}

func TestExtractMetricsFromQuery(t *testing.T) {
	query := `(vm_free_disk_space_bytes{job=~"$job", instance=~"$instance"}-vm_free_disk_space_limit_bytes{job=~"$job", instance=~"$instance"}) 
/ 
ignoring(path) (
    (rate(vm_rows_added_to_storage_total{job=~"$job", instance=~"$instance"}[1d]) - 
        sum(rate(vm_deduplicated_samples_total{job=~"$job", instance=~"$instance"}[1d])) without (type)) * 
    (
        sum(vm_data_size_bytes{job=~"$job", instance=~"$instance", type!~"indexdb.*"}) without(type) /
        sum(vm_rows{job=~"$job", instance=~"$instance", type!~"indexdb.*"}) without(type)
    )
    +
    rate(vm_new_timeseries_created_total{job=~"$job", instance=~"$instance"}[1d]) * 
    (
        sum(vm_data_size_bytes{job=~"$job", instance=~"$instance", type="indexdb/file"}) /
        sum(vm_rows{job=~"$job", instance=~"$instance", type="indexdb/file"})
    )
)`
	metrics, err := extractMetricsFromQuery(query)
	if err != nil {
		t.Fatalf(`unexpected error when extracting metrics from query: %s`, err)
	}
	t.Logf(`metrics: %v`, metrics)
}
