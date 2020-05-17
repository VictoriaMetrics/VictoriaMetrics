package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestRule_Validate(t *testing.T) {
	if err := (&Rule{}).Validate(); err == nil {
		t.Errorf("exptected empty name error")
	}
	if err := (&Rule{Name: "alert"}).Validate(); err == nil {
		t.Errorf("exptected empty expr error")
	}
	if err := (&Rule{Name: "alert", Expr: "test{"}).Validate(); err == nil {
		t.Errorf("exptected invalid expr error")
	}
	if err := (&Rule{Name: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Errorf("exptected valid rule got %s", err)
	}
}

func TestRule_AlertToTimeSeries(t *testing.T) {
	timestamp := time.Now()
	testCases := []struct {
		rule  *Rule
		alert *notifier.Alert
		expTS []prompbmarshal.TimeSeries
	}{
		{
			newTestRule("instant", 0),
			&notifier.Alert{State: notifier.StateFiring},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(1, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
					alertNameLabel:  "instant",
				}, timestamp),
			},
		},
		{
			newTestRule("instant extra labels", 0),
			&notifier.Alert{State: notifier.StateFiring, Labels: map[string]string{
				"job":      "foo",
				"instance": "bar",
			}},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(1, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
					alertNameLabel:  "instant extra labels",
					"job":           "foo",
					"instance":      "bar",
				}, timestamp),
			},
		},
		{
			newTestRule("instant labels override", 0),
			&notifier.Alert{State: notifier.StateFiring, Labels: map[string]string{
				alertStateLabel: "foo",
				"__name__":      "bar",
			}},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(1, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
					alertNameLabel:  "instant labels override",
				}, timestamp),
			},
		},
		{
			newTestRule("for", time.Second),
			&notifier.Alert{State: notifier.StateFiring, Start: timestamp.Add(time.Second)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(1, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
					alertNameLabel:  "for",
				}, timestamp),
				newTimeSeries(float64(timestamp.Add(time.Second).Unix()), map[string]string{
					"__name__":     alertForStateMetricName,
					alertNameLabel: "for",
				}, timestamp),
			},
		},
		{
			newTestRule("for pending", 10*time.Second),
			&notifier.Alert{State: notifier.StatePending, Start: timestamp.Add(time.Second)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries(1, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StatePending.String(),
					alertNameLabel:  "for pending",
				}, timestamp),
				newTimeSeries(float64(timestamp.Add(time.Second).Unix()), map[string]string{
					"__name__":     alertForStateMetricName,
					alertNameLabel: "for pending",
				}, timestamp),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			tss := tc.rule.AlertToTimeSeries(tc.alert, timestamp)
			if len(tc.expTS) != len(tss) {
				t.Fatalf("expected number of timeseries %d; got %d", len(tc.expTS), len(tss))
			}
			for i := range tc.expTS {
				expTS, gotTS := tc.expTS[i], tss[i]
				if len(expTS.Samples) != len(gotTS.Samples) {
					t.Fatalf("expected number of samples %d; got %d", len(expTS.Samples), len(gotTS.Samples))
				}
				for i, exp := range expTS.Samples {
					got := gotTS.Samples[i]
					if got.Value != exp.Value {
						t.Errorf("expected value %.2f; got %.2f", exp.Value, got.Value)
					}
					if got.Timestamp != exp.Timestamp {
						t.Errorf("expected timestamp %d; got %d", exp.Timestamp, got.Timestamp)
					}
				}
				if len(expTS.Labels) != len(gotTS.Labels) {
					t.Fatalf("expected number of labels %d; got %d", len(expTS.Labels), len(gotTS.Labels))
				}
				for i, exp := range expTS.Labels {
					got := gotTS.Labels[i]
					if got.Name != exp.Name {
						t.Errorf("expected label name %q; got %q", exp.Name, got.Name)
					}
					if got.Value != exp.Value {
						t.Errorf("expected label value %q; got %q", exp.Value, got.Value)
					}
				}
			}
		})
	}
}

func newTestRule(name string, waitFor time.Duration) *Rule {
	return &Rule{Name: name, alerts: make(map[uint64]*notifier.Alert), For: waitFor}
}

func TestRule_Exec(t *testing.T) {
	testCases := []struct {
		rule      *Rule
		steps     [][]datasource.Metric
		expAlerts map[uint64]*notifier.Alert
	}{
		{
			newTestRule("empty", 0),
			[][]datasource.Metric{},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestRule("empty labels", 0),
			[][]datasource.Metric{
				{datasource.Metric{}},
			},
			map[uint64]*notifier.Alert{
				hash(datasource.Metric{}): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("single-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("single-firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive=>empty", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
				{},
				{},
			},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive=>empty=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
				{},
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("multiple-firing", 0),
			[][]datasource.Metric{
				{
					metricWithLabels(t, "name", "foo"),
					metricWithLabels(t, "name", "foo1"),
					metricWithLabels(t, "name", "foo2"),
				},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")):  {State: notifier.StateFiring},
				hash(metricWithLabels(t, "name", "foo1")): {State: notifier.StateFiring},
				hash(metricWithLabels(t, "name", "foo2")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("multiple-steps-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo1")},
				{metricWithLabels(t, "name", "foo2")},
			},
			// 1: fire first alert
			// 2: fire second alert, set first inactive
			// 3: fire third alert, set second inactive, delete first one
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo1")): {State: notifier.StateInactive},
				hash(metricWithLabels(t, "name", "foo2")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("duplicate", 0),
			[][]datasource.Metric{
				{
					// metrics with the same labelset should result in one alert
					metricWithLabels(t, "name", "foo", "type", "bar"),
					metricWithLabels(t, "type", "bar", "name", "foo"),
				},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo", "type", "bar")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("for-pending", time.Minute),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StatePending},
			},
		},
		{
			newTestRule("for-fired", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("for-pending=>empty", time.Second),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset and delete pending alerts
				{},
			},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestRule("for-pending=>firing=>inactive", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset pending alerts
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("for-pending=>firing=>inactive=>pending", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset pending alerts
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StatePending},
			},
		},
		{
			newTestRule("for-pending=>firing=>inactive=>pending=>firing", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset pending alerts
				{},
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.group = fakeGroup
			for _, step := range tc.steps {
				fq.reset()
				fq.add(step...)
				if err := tc.rule.Exec(context.TODO(), fq); err != nil {
					t.Fatalf("unexpected err: %s", err)
				}
				// artificial delay between applying steps
				time.Sleep(time.Millisecond)
			}
			if len(tc.rule.alerts) != len(tc.expAlerts) {
				t.Fatalf("expected %d alerts; got %d", len(tc.expAlerts), len(tc.rule.alerts))
			}
			for key, exp := range tc.expAlerts {
				got, ok := tc.rule.alerts[key]
				if !ok {
					t.Fatalf("expected to have key %d", key)
				}
				if got.State != exp.State {
					t.Fatalf("expected state %d; got %d", exp.State, got.State)
				}
			}
		})
	}
}

func metricWithLabels(t *testing.T, labels ...string) datasource.Metric {
	t.Helper()
	if len(labels) == 0 || len(labels)%2 != 0 {
		t.Fatalf("expected to get even number of labels")
	}
	m := datasource.Metric{}
	for i := 0; i < len(labels); i += 2 {
		m.Labels = append(m.Labels, datasource.Label{
			Name:  labels[i],
			Value: labels[i+1],
		})
	}
	return m
}

type fakeQuerier struct {
	sync.Mutex
	metrics []datasource.Metric
}

func (fq *fakeQuerier) reset() {
	fq.Lock()
	fq.metrics = fq.metrics[:0]
	fq.Unlock()
}

func (fq *fakeQuerier) add(metrics ...datasource.Metric) {
	fq.Lock()
	fq.metrics = append(fq.metrics, metrics...)
	fq.Unlock()
}

func (fq *fakeQuerier) Query(_ context.Context, _ string) ([]datasource.Metric, error) {
	fq.Lock()
	cpy := make([]datasource.Metric, len(fq.metrics))
	copy(cpy, fq.metrics)
	fq.Unlock()
	return cpy, nil
}

func TestRule_Restore(t *testing.T) {
	testCases := []struct {
		rule      *Rule
		metrics   []datasource.Metric
		expAlerts map[uint64]*notifier.Alert
	}{
		{
			newTestRuleWithLabels("no extra labels"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					alertNameLabel, "",
				),
			},
			map[uint64]*notifier.Alert{
				hash(datasource.Metric{}): {State: notifier.StatePending,
					Start: time.Now().Truncate(time.Hour)},
			},
		},
		{
			newTestRuleWithLabels("metric labels"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					alertNameLabel, "",
					"foo", "bar",
					"namespace", "baz",
				),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t,
					"foo", "bar",
					"namespace", "baz",
				)): {State: notifier.StatePending,
					Start: time.Now().Truncate(time.Hour)},
			},
		},
		{
			newTestRuleWithLabels("rule labels", "source", "vm"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					alertNameLabel, "",
					"foo", "bar",
					"namespace", "baz",
					// following pair supposed to be dropped
					"source", "vm",
				),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t,
					"foo", "bar",
					"namespace", "baz",
				)): {State: notifier.StatePending,
					Start: time.Now().Truncate(time.Hour)},
			},
		},
		{
			newTestRuleWithLabels("multiple alerts"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					"host", "localhost-1",
				),
				metricWithValueAndLabels(t, float64(time.Now().Truncate(2*time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					"host", "localhost-2",
				),
				metricWithValueAndLabels(t, float64(time.Now().Truncate(3*time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					"host", "localhost-3",
				),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "host", "localhost-1")): {State: notifier.StatePending,
					Start: time.Now().Truncate(time.Hour)},
				hash(metricWithLabels(t, "host", "localhost-2")): {State: notifier.StatePending,
					Start: time.Now().Truncate(2 * time.Hour)},
				hash(metricWithLabels(t, "host", "localhost-3")): {State: notifier.StatePending,
					Start: time.Now().Truncate(3 * time.Hour)},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.group = fakeGroup
			fq.add(tc.metrics...)
			if err := tc.rule.Restore(context.TODO(), fq, time.Hour); err != nil {
				t.Fatalf("unexpected err: %s", err)
			}
			if len(tc.rule.alerts) != len(tc.expAlerts) {
				t.Fatalf("expected %d alerts; got %d", len(tc.expAlerts), len(tc.rule.alerts))
			}
			for key, exp := range tc.expAlerts {
				got, ok := tc.rule.alerts[key]
				if !ok {
					t.Fatalf("expected to have key %d", key)
				}
				if got.State != exp.State {
					t.Fatalf("expected state %d; got %d", exp.State, got.State)
				}
				if got.Start != exp.Start {
					t.Fatalf("expected Start %v; got %v", exp.Start, got.Start)
				}
			}
		})
	}
}

func newTestRuleWithLabels(name string, labels ...string) *Rule {
	r := newTestRule(name, 0)
	r.Labels = make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		r.Labels[labels[i]] = labels[i+1]
	}
	return r
}

func metricWithValueAndLabels(t *testing.T, value float64, labels ...string) datasource.Metric {
	t.Helper()
	m := metricWithLabels(t, labels...)
	m.Value = value
	return m
}
