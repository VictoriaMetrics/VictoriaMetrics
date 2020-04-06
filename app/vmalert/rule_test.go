package main

import (
	"context"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
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
			newTestRule("single-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("single-firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{metricWithLabels(t, "__name__", "foo")},
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive=>empty", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{},
			},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestRule("single-firing=>inactive=>firing=>inactive=>empty=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{metricWithLabels(t, "__name__", "foo")},
				{},
				{},
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("multiple-firing", 0),
			[][]datasource.Metric{
				{
					metricWithLabels(t, "__name__", "foo"),
					metricWithLabels(t, "__name__", "foo1"),
					metricWithLabels(t, "__name__", "foo2"),
				},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")):  {State: notifier.StateFiring},
				hash(metricWithLabels(t, "__name__", "foo1")): {State: notifier.StateFiring},
				hash(metricWithLabels(t, "__name__", "foo2")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("multiple-steps-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo1")},
				{metricWithLabels(t, "__name__", "foo2")},
			},
			// 1: fire first alert
			// 2: fire second alert, set first inactive
			// 3: fire third alert, set second inactive, delete first one
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo1")): {State: notifier.StateInactive},
				hash(metricWithLabels(t, "__name__", "foo2")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("duplicate", 0),
			[][]datasource.Metric{
				{
					// metrics with the same labelset should result in one alert
					metricWithLabels(t, "__name__", "foo", "type", "bar"),
					metricWithLabels(t, "type", "bar", "__name__", "foo"),
				},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo", "type", "bar")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("for-pending", time.Minute),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StatePending},
			},
		},
		{
			newTestRule("for-fired", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestRule("for-pending=>inactive", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
				// empty step to reset pending alerts
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("for-pending=>firing=>inactive", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
				// empty step to reset pending alerts
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestRule("for-pending=>firing=>inactive=>pending", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
				// empty step to reset pending alerts
				{},
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StatePending},
			},
		},
		{
			newTestRule("for-pending=>firing=>inactive=>pending=>firing", time.Millisecond),
			[][]datasource.Metric{
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
				// empty step to reset pending alerts
				{},
				{metricWithLabels(t, "__name__", "foo")},
				{metricWithLabels(t, "__name__", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "__name__", "foo")): {State: notifier.StateFiring},
			},
		},
	}
	fakeGroup := &Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.group = fakeGroup
			for _, step := range tc.steps {
				fq.reset()
				fq.add(t, step...)
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
	metrics []datasource.Metric
}

func (fq *fakeQuerier) reset() {
	fq.metrics = fq.metrics[:0]
}

func (fq *fakeQuerier) add(t *testing.T, metrics ...datasource.Metric) {
	fq.metrics = append(fq.metrics, metrics...)
}

func (fq fakeQuerier) Query(ctx context.Context, query string) ([]datasource.Metric, error) {
	return fq.metrics, nil
}
