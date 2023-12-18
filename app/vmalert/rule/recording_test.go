package rule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestRecordingRule_Exec(t *testing.T) {
	timestamp := time.Now()
	testCases := []struct {
		rule    *RecordingRule
		metrics []datasource.Metric
		expTS   []prompbmarshal.TimeSeries
	}{
		{
			&RecordingRule{Name: "foo"},
			[]datasource.Metric{metricWithValueAndLabels(t, 10,
				"__name__", "bar",
			)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{10}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "foo",
				}),
			},
		},
		{
			&RecordingRule{Name: "foobarbaz"},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "foo"),
				metricWithValueAndLabels(t, 2, "__name__", "bar", "job", "bar"),
				metricWithValueAndLabels(t, 3, "__name__", "baz", "job", "baz"),
			},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "foobarbaz",
					"job":      "foo",
				}),
				newTimeSeries([]float64{2}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "foobarbaz",
					"job":      "bar",
				}),
				newTimeSeries([]float64{3}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "foobarbaz",
					"job":      "baz",
				}),
			},
		},
		{
			&RecordingRule{
				Name: "job:foo",
				Labels: map[string]string{
					"source": "test",
				},
			},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
				metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar", "source", "origin"),
			},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{2}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "job:foo",
					"job":      "foo",
					"source":   "test",
				}),
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":        "job:foo",
					"job":             "bar",
					"source":          "test",
					"exported_source": "origin",
				}),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &datasource.FakeQuerier{}
			fq.Add(tc.metrics...)
			tc.rule.q = fq
			tc.rule.state = &ruleState{entries: make([]StateEntry, 10)}
			tss, err := tc.rule.exec(context.TODO(), time.Now(), 0)
			if err != nil {
				t.Fatalf("unexpected Exec err: %s", err)
			}
			if err := compareTimeSeries(t, tc.expTS, tss); err != nil {
				t.Fatalf("timeseries missmatch: %s", err)
			}
		})
	}
}

func TestRecordingRule_ExecRange(t *testing.T) {
	timestamp := time.Now()
	testCases := []struct {
		rule    *RecordingRule
		metrics []datasource.Metric
		expTS   []prompbmarshal.TimeSeries
	}{
		{
			&RecordingRule{Name: "foo"},
			[]datasource.Metric{metricWithValuesAndLabels(t, []float64{10, 20, 30},
				"__name__", "bar",
			)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{10, 20, 30},
					[]int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()},
					map[string]string{
						"__name__": "foo",
					}),
			},
		},
		{
			&RecordingRule{Name: "foobarbaz"},
			[]datasource.Metric{
				metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
				metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
				metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
			},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "foobarbaz",
					"job":      "foo",
				}),
				newTimeSeries([]float64{2, 3}, []int64{timestamp.UnixNano(), timestamp.UnixNano()}, map[string]string{
					"__name__": "foobarbaz",
					"job":      "bar",
				}),
				newTimeSeries([]float64{4, 5, 6},
					[]int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()},
					map[string]string{
						"__name__": "foobarbaz",
						"job":      "baz",
					}),
			},
		},
		{
			&RecordingRule{Name: "job:foo", Labels: map[string]string{
				"source": "test",
			}},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
				metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar"),
			},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{2}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "job:foo",
					"job":      "foo",
					"source":   "test",
				}),
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__": "job:foo",
					"job":      "bar",
					"source":   "test",
				}),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &datasource.FakeQuerier{}
			fq.Add(tc.metrics...)
			tc.rule.q = fq
			tss, err := tc.rule.execRange(context.TODO(), time.Now(), time.Now())
			if err != nil {
				t.Fatalf("unexpected Exec err: %s", err)
			}
			if err := compareTimeSeries(t, tc.expTS, tss); err != nil {
				t.Fatalf("timeseries missmatch: %s", err)
			}
		})
	}
}

func TestRecordingRuleLimit(t *testing.T) {
	timestamp := time.Now()
	testCases := []struct {
		limit int
		err   string
	}{
		{
			limit: 0,
		},
		{
			limit: -1,
		},
		{
			limit: 1,
			err:   "exec exceeded limit of 1 with 3 series",
		},
		{
			limit: 2,
			err:   "exec exceeded limit of 2 with 3 series",
		},
	}
	testMetrics := []datasource.Metric{
		metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
		metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
		metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
	}
	rule := &RecordingRule{Name: "job:foo",
		state: &ruleState{entries: make([]StateEntry, 10)},
		Labels: map[string]string{
			"source": "test_limit",
		},
		metrics: &recordingRuleMetrics{
			errors: utils.GetOrCreateCounter(`vmalert_recording_rules_errors_total{alertname="job:foo"}`),
		},
	}
	var err error
	for _, testCase := range testCases {
		fq := &datasource.FakeQuerier{}
		fq.Add(testMetrics...)
		rule.q = fq
		_, err = rule.exec(context.TODO(), timestamp, testCase.limit)
		if err != nil && !strings.EqualFold(err.Error(), testCase.err) {
			t.Fatal(err)
		}
	}
}

func TestRecordingRule_ExecNegative(t *testing.T) {
	rr := &RecordingRule{
		Name: "job:foo",
		Labels: map[string]string{
			"job": "test",
		},
		state: &ruleState{entries: make([]StateEntry, 10)},
		metrics: &recordingRuleMetrics{
			errors: utils.GetOrCreateCounter(`vmalert_recording_rules_errors_total{alertname="job:foo"}`),
		},
	}
	fq := &datasource.FakeQuerier{}
	expErr := "connection reset by peer"
	fq.SetErr(errors.New(expErr))
	rr.q = fq
	_, err := rr.exec(context.TODO(), time.Now(), 0)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to get err %q; got %q insterad", expErr, err)
	}

	fq.Reset()

	// add metrics which differs only by `job` label
	// which will be overridden by rule
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "foo"))
	fq.Add(metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "bar"))

	_, err = rr.exec(context.TODO(), time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
}
