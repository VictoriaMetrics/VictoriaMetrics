package rule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestRecordingRule_Exec(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2024-10-29T00:00:00Z")
	const defaultStep = 5 * time.Millisecond

	f := func(rule *RecordingRule, steps [][]datasource.Metric, tssExpected [][]prompbmarshal.TimeSeries) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		for i, step := range steps {
			fq.Reset()
			fq.Add(step...)
			rule.q = fq
			rule.state = &ruleState{
				entries: make([]StateEntry, 10),
			}
			tss, err := rule.exec(context.TODO(), ts, 0)
			if err != nil {
				t.Fatalf("fail to test rule %s: unexpected error: %s", rule.Name, err)
			}
			if err := compareTimeSeries(t, tssExpected[i], tss); err != nil {
				t.Fatalf("fail to test rule %s: time series mismatch on step %d: %s", rule.Name, i, err)
			}

			ts = ts.Add(defaultStep)
		}
	}

	f(&RecordingRule{
		Name: "foo",
	}, [][]datasource.Metric{{
		metricWithValueAndLabels(t, 10, "__name__", "bar"),
	}}, [][]prompbmarshal.TimeSeries{{
		newTimeSeries([]float64{10}, []int64{ts.UnixNano()}, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foo",
			},
		}),
	}})

	f(&RecordingRule{
		Name: "foobarbaz",
	}, [][]datasource.Metric{
		{
			metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "foo"),
			metricWithValueAndLabels(t, 2, "__name__", "bar", "job", "bar"),
		},
		{
			metricWithValueAndLabels(t, 10, "__name__", "foo", "job", "foo"),
		},
		{
			metricWithValueAndLabels(t, 10, "__name__", "foo", "job", "bar"),
		},
	}, [][]prompbmarshal.TimeSeries{
		{
			newTimeSeries([]float64{1}, []int64{ts.UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			}),
			newTimeSeries([]float64{2}, []int64{ts.UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "bar",
				},
			}),
		},
		{
			newTimeSeries([]float64{10}, []int64{ts.Add(defaultStep).UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			}),
			// stale time series
			newTimeSeries([]float64{decimal.StaleNaN}, []int64{ts.Add(defaultStep).UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "bar",
				},
			}),
		},
		{
			newTimeSeries([]float64{10}, []int64{ts.Add(2 * defaultStep).UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "bar",
				},
			}),
			newTimeSeries([]float64{decimal.StaleNaN}, []int64{ts.Add(2 * defaultStep).UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			}),
		},
	})

	f(&RecordingRule{
		Name: "job:foo",
		Labels: map[string]string{
			"source": "test",
		},
	}, [][]datasource.Metric{{
		metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
		metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar", "source", "origin"),
	}}, [][]prompbmarshal.TimeSeries{{
		newTimeSeries([]float64{2}, []int64{ts.UnixNano()}, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "job:foo",
			},
			{
				Name:  "job",
				Value: "foo",
			},
			{
				Name:  "source",
				Value: "test",
			},
		}),
		newTimeSeries([]float64{1}, []int64{ts.UnixNano()},
			[]prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "job:foo",
				},
				{
					Name:  "job",
					Value: "bar",
				},
				{
					Name:  "source",
					Value: "test",
				},
				{
					Name:  "exported_source",
					Value: "origin",
				},
			}),
	}})
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
		newTimeSeries([]float64{10, 20, 30}, []int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()},
			[]prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foo",
				},
			}),
	})

	f(&RecordingRule{
		Name: "foobarbaz",
	}, []datasource.Metric{
		metricWithValuesAndLabels(t, []float64{1}, "__name__", "foo", "job", "foo"),
		metricWithValuesAndLabels(t, []float64{2, 3}, "__name__", "bar", "job", "bar"),
		metricWithValuesAndLabels(t, []float64{4, 5, 6}, "__name__", "baz", "job", "baz"),
	}, []prompbmarshal.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foobarbaz",
			},
			{
				Name:  "job",
				Value: "foo",
			},
		}),
		newTimeSeries([]float64{2, 3}, []int64{timestamp.UnixNano(), timestamp.UnixNano()}, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "foobarbaz",
			},
			{
				Name:  "job",
				Value: "bar",
			},
		}),
		newTimeSeries([]float64{4, 5, 6},
			[]int64{timestamp.UnixNano(), timestamp.UnixNano(), timestamp.UnixNano()}, []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "foobarbaz",
				},
				{
					Name:  "job",
					Value: "baz",
				},
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
		newTimeSeries([]float64{2}, []int64{timestamp.UnixNano()}, []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "job:foo",
			},
			{
				Name:  "job",
				Value: "foo",
			},
			{
				Name:  "source",
				Value: "test",
			},
		}),
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()},
			[]prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "job:foo",
				},
				{
					Name:  "job",
					Value: "bar",
				},
				{
					Name:  "source",
					Value: "test",
				},
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

func TestSetIntervalAsTimeFilter(t *testing.T) {
	f := func(s, dType string, expected bool) {
		t.Helper()

		if setIntervalAsTimeFilter(dType, s) != expected {
			t.Fatalf("unexpected result for hasTimeFilter(%q);  want %v", s, expected)
		}
	}

	f(`* | count()`, "prometheus", false)

	f(`* | count()`, "vlogs", true)
	f(`error OR _time:5m  | count()`, "vlogs", true)
	f(`(_time: 5m AND error) OR (_time: 5m AND warn) | count()`, "vlogs", true)
	f(`* | error OR _time:5m | count()`, "vlogs", true)

	f(`_time:5m | count()`, "vlogs", false)
	f(`_time:2023-04-25T22:45:59Z | count()`, "vlogs", false)
	f(`error AND _time:5m | count()`, "vlogs", false)
	f(`* | error AND _time:5m | count()`, "vlogs", false)
}
