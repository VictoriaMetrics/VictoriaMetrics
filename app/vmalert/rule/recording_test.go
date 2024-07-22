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
	f := func(rule *RecordingRule, metrics []datasource.Metric, tssExpected []prompbmarshal.TimeSeries) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		fq.Add(metrics...)
		rule.q = fq
		rule.state = &ruleState{
			entries: make([]StateEntry, 10),
		}
		tss, err := rule.exec(context.TODO(), time.Now(), 0)
		if err != nil {
			t.Fatalf("unexpected RecordingRule.exec error: %s", err)
		}
		if err := compareTimeSeries(t, tssExpected, tss); err != nil {
			t.Fatalf("timeseries missmatch: %s", err)
		}
	}

	timestamp := time.Now()

	f(&RecordingRule{
		Name: "foo",
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 10, "__name__", "bar"),
	}, []prompbmarshal.TimeSeries{
		newTimeSeries([]float64{10}, []int64{timestamp.UnixNano()}, map[string]string{
			"__name__": "foo",
		}),
	})

	f(&RecordingRule{
		Name: "foobarbaz",
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "foo"),
		metricWithValueAndLabels(t, 2, "__name__", "bar", "job", "bar"),
		metricWithValueAndLabels(t, 3, "__name__", "baz", "job", "baz"),
	}, []prompbmarshal.TimeSeries{
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
	})

	f(&RecordingRule{
		Name: "job:foo",
		Labels: map[string]string{
			"source": "test",
		},
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
		metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar", "source", "origin"),
	}, []prompbmarshal.TimeSeries{
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
	})
}

func TestRecordingRule_ExecRange(t *testing.T) {
	f := func(rule *RecordingRule, metrics []datasource.Metric, tssExpected []prompbmarshal.TimeSeries) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		fq.Add(metrics...)
		rule.q = fq
		tss, err := rule.execRange(context.TODO(), time.Now(), time.Now())
		if err != nil {
			t.Fatalf("unexpected RecordingRule.execRange error: %s", err)
		}
		if err := compareTimeSeries(t, tssExpected, tss); err != nil {
			t.Fatalf("timeseries missmatch: %s", err)
		}
	}

	timestamp := time.Now()

	f(&RecordingRule{
		Name: "foo",
	}, []datasource.Metric{
		metricWithValuesAndLabels(t, []float64{10, 20, 30}, "__name__", "bar"),
	}, []prompbmarshal.TimeSeries{
		newTimeSeries([]float64{10, 20, 30}, []int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()}, map[string]string{
			"__name__": "foo",
		}),
	})

	f(&RecordingRule{
		Name: "foobarbaz",
	}, []datasource.Metric{
		metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
		metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
		metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
	}, []prompbmarshal.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
			"__name__": "foobarbaz",
			"job":      "foo",
		}),
		newTimeSeries([]float64{2, 3}, []int64{timestamp.UnixNano(), timestamp.UnixNano()}, map[string]string{
			"__name__": "foobarbaz",
			"job":      "bar",
		}),
		newTimeSeries([]float64{4, 5, 6},
			[]int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()}, map[string]string{
				"__name__": "foobarbaz",
				"job":      "baz",
			}),
	})

	f(&RecordingRule{
		Name: "job:foo",
		Labels: map[string]string{
			"source": "test",
		},
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
		metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar"),
	}, []prompbmarshal.TimeSeries{
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
	})
}

func TestRecordingRuleLimit_Failure(t *testing.T) {
	f := func(limit int, errStrExpected string) {
		t.Helper()

		testMetrics := []datasource.Metric{
			metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
			metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
			metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
		}

		fq := &datasource.FakeQuerier{}
		fq.Add(testMetrics...)

		rule := &RecordingRule{Name: "job:foo",
			state: &ruleState{entries: make([]StateEntry, 10)},
			Labels: map[string]string{
				"source": "test_limit",
			},
			metrics: &recordingRuleMetrics{
				errors: utils.GetOrCreateCounter(`vmalert_recording_rules_errors_total{alertname="job:foo"}`),
			},
		}
		rule.q = fq

		_, err := rule.exec(context.TODO(), time.Now(), limit)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, errStrExpected) {
			t.Fatalf("missing %q in the error %q", errStrExpected, errStr)
		}
	}

	f(1, "exec exceeded limit of 1 with 3 series")
	f(2, "exec exceeded limit of 2 with 3 series")
}

func TestRecordingRuleLimit_Success(t *testing.T) {
	f := func(limit int) {
		t.Helper()

		testMetrics := []datasource.Metric{
			metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
			metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
			metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
		}

		fq := &datasource.FakeQuerier{}
		fq.Add(testMetrics...)

		rule := &RecordingRule{Name: "job:foo",
			state: &ruleState{entries: make([]StateEntry, 10)},
			Labels: map[string]string{
				"source": "test_limit",
			},
			metrics: &recordingRuleMetrics{
				errors: utils.GetOrCreateCounter(`vmalert_recording_rules_errors_total{alertname="job:foo"}`),
			},
		}
		rule.q = fq

		_, err := rule.exec(context.TODO(), time.Now(), limit)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	f(0)
	f(-1)
}

func TestRecordingRuleExec_Negative(t *testing.T) {
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
		t.Fatalf("cannot execute recroding rule: %s", err)
	}
}
