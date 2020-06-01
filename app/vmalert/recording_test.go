package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestRecoridngRule_ToTimeSeries(t *testing.T) {
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
				newTimeSeries(10, map[string]string{
					"__name__": "foo",
				}, timestamp),
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
				newTimeSeries(1, map[string]string{
					"__name__": "foobarbaz",
					"job":      "foo",
				}, timestamp),
				newTimeSeries(2, map[string]string{
					"__name__": "foobarbaz",
					"job":      "bar",
				}, timestamp),
				newTimeSeries(3, map[string]string{
					"__name__": "foobarbaz",
					"job":      "baz",
				}, timestamp),
			},
		},
		{
			&RecordingRule{Name: "job:foo", Labels: map[string]string{
				"source": "test",
			}},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "foo"),
				metricWithValueAndLabels(t, 1, "__name__", "bar", "job", "bar")},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(2, map[string]string{
					"__name__": "job:foo",
					"job":      "foo",
					"source":   "test",
				}, timestamp),
				newTimeSeries(1, map[string]string{
					"__name__": "job:foo",
					"job":      "bar",
					"source":   "test",
				}, timestamp),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			fq.add(tc.metrics...)
			tss, err := tc.rule.Exec(context.TODO(), fq, true)
			if err != nil {
				t.Fatalf("unexpected Exec err: %s", err)
			}
			if err := compareTimeSeries(t, tc.expTS, tss); err != nil {
				t.Fatalf("timeseries missmatch: %s", err)
			}
		})
	}
}

func TestRecoridngRule_ToTimeSeriesNegative(t *testing.T) {
	rr := &RecordingRule{Name: "job:foo", Labels: map[string]string{
		"job": "test",
	}}

	fq := &fakeQuerier{}
	expErr := "connection reset by peer"
	fq.setErr(errors.New(expErr))

	_, err := rr.Exec(context.TODO(), fq, true)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to get err %q; got %q insterad", expErr, err)
	}

	fq.reset()

	// add metrics which differs only by `job` label
	// which will be overridden by rule
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "foo"))
	fq.add(metricWithValueAndLabels(t, 2, "__name__", "foo", "job", "bar"))

	_, err = rr.Exec(context.TODO(), fq, true)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), errDuplicate.Error()) {
		t.Fatalf("expected to get err %q; got %q insterad", errDuplicate, err)
	}
}
