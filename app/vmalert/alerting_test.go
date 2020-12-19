package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestAlertingRule_ToTimeSeries(t *testing.T) {
	timestamp := time.Now()
	testCases := []struct {
		rule  *AlertingRule
		alert *notifier.Alert
		expTS []prompbmarshal.TimeSeries
	}{
		{
			newTestAlertingRule("instant", 0),
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
			newTestAlertingRule("instant extra labels", 0),
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
			newTestAlertingRule("instant labels override", 0),
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
			newTestAlertingRule("for", time.Second),
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
			newTestAlertingRule("for pending", 10*time.Second),
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
			tc.rule.alerts[tc.alert.ID] = tc.alert
			tss := tc.rule.toTimeSeries(timestamp)
			if err := compareTimeSeries(t, tc.expTS, tss); err != nil {
				t.Fatalf("timeseries missmatch: %s", err)
			}
		})
	}
}

func TestAlertingRule_Exec(t *testing.T) {
	const defaultStep = 5 * time.Millisecond
	testCases := []struct {
		rule      *AlertingRule
		steps     [][]datasource.Metric
		expAlerts map[uint64]*notifier.Alert
	}{
		{
			newTestAlertingRule("empty", 0),
			[][]datasource.Metric{},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestAlertingRule("empty labels", 0),
			[][]datasource.Metric{
				{datasource.Metric{}},
			},
			map[uint64]*notifier.Alert{
				hash(datasource.Metric{}): {State: notifier.StateFiring},
			},
		},
		{
			newTestAlertingRule("single-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateInactive},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive=>firing", 0),
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
			newTestAlertingRule("single-firing=>inactive=>firing=>inactive", 0),
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
			newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>empty", 0),
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
			newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>empty=>firing", 0),
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
			newTestAlertingRule("multiple-firing", 0),
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
			newTestAlertingRule("multiple-steps-firing", 0),
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
			newTestAlertingRule("for-pending", time.Minute),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StatePending},
			},
		},
		{
			newTestAlertingRule("for-fired", defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "name", "foo")): {State: notifier.StateFiring},
			},
		},
		{
			newTestAlertingRule("for-pending=>empty", time.Second),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset and delete pending alerts
				{},
			},
			map[uint64]*notifier.Alert{},
		},
		{
			newTestAlertingRule("for-pending=>firing=>inactive", defaultStep),
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
			newTestAlertingRule("for-pending=>firing=>inactive=>pending", defaultStep),
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
			newTestAlertingRule("for-pending=>firing=>inactive=>pending=>firing", defaultStep),
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
			tc.rule.GroupID = fakeGroup.ID()
			for _, step := range tc.steps {
				fq.reset()
				fq.add(step...)
				if _, err := tc.rule.Exec(context.TODO(), fq, false); err != nil {
					t.Fatalf("unexpected err: %s", err)
				}
				// artificial delay between applying steps
				time.Sleep(defaultStep)
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

func TestAlertingRule_Restore(t *testing.T) {
	testCases := []struct {
		rule      *AlertingRule
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
					alertGroupNameLabel, "groupID",
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
					// extra labels set by rule
					"source", "vm",
				),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t,
					"foo", "bar",
					"namespace", "baz",
					"source", "vm",
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
			tc.rule.GroupID = fakeGroup.ID()
			fq.add(tc.metrics...)
			if err := tc.rule.Restore(context.TODO(), fq, time.Hour, nil); err != nil {
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

func TestAlertingRule_Exec_Negative(t *testing.T) {
	fq := &fakeQuerier{}
	ar := newTestAlertingRule("test", 0)
	ar.Labels = map[string]string{"job": "test"}

	// successful attempt
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
	_, err := ar.Exec(context.TODO(), fq, false)
	if err != nil {
		t.Fatal(err)
	}

	// label `job` will collide with rule extra label and will make both time series equal
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "baz"))
	_, err = ar.Exec(context.TODO(), fq, false)
	if !errors.Is(err, errDuplicate) {
		t.Fatalf("expected to have %s error; got %s", errDuplicate, err)
	}

	fq.reset()

	expErr := "connection reset by peer"
	fq.setErr(errors.New(expErr))
	_, err = ar.Exec(context.TODO(), fq, false)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to get err %q; got %q insterad", expErr, err)
	}
}

func TestAlertingRule_Template(t *testing.T) {
	testCases := []struct {
		rule      *AlertingRule
		metrics   []datasource.Metric
		expAlerts map[uint64]*notifier.Alert
	}{
		{
			newTestRuleWithLabels("common", "region", "east"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, 1, "instance", "foo"),
				metricWithValueAndLabels(t, 1, "instance", "bar"),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "region", "east", "instance", "foo")): {
					Annotations: map[string]string{},
					Labels: map[string]string{
						alertGroupNameLabel: "",
						"region":            "east",
						"instance":          "foo",
					},
				},
				hash(metricWithLabels(t, "region", "east", "instance", "bar")): {
					Annotations: map[string]string{},
					Labels: map[string]string{
						alertGroupNameLabel: "",
						"region":            "east",
						"instance":          "bar",
					},
				},
			},
		},
		{
			&AlertingRule{
				Name: "override label",
				Labels: map[string]string{
					"instance": "{{ $labels.instance }}",
					"region":   "east",
				},
				Annotations: map[string]string{
					"summary":     `Too high connection number for "{{ $labels.instance }}" for region {{ $labels.region }}`,
					"description": `It is {{ $value }} connections for "{{ $labels.instance }}"`,
				},
				alerts: make(map[uint64]*notifier.Alert),
			},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 2, "instance", "foo"),
				metricWithValueAndLabels(t, 10, "instance", "bar"),
			},
			map[uint64]*notifier.Alert{
				hash(metricWithLabels(t, "region", "east", "instance", "foo")): {
					Labels: map[string]string{
						alertGroupNameLabel: "",
						"instance":          "foo",
						"region":            "east",
					},
					Annotations: map[string]string{
						"summary":     `Too high connection number for "foo" for region east`,
						"description": `It is 2 connections for "foo"`,
					},
				},
				hash(metricWithLabels(t, "region", "east", "instance", "bar")): {
					Labels: map[string]string{
						alertGroupNameLabel: "",
						"instance":          "bar",
						"region":            "east",
					},
					Annotations: map[string]string{
						"summary":     `Too high connection number for "bar" for region east`,
						"description": `It is 10 connections for "bar"`,
					},
				},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.GroupID = fakeGroup.ID()
			fq.add(tc.metrics...)
			if _, err := tc.rule.Exec(context.TODO(), fq, false); err != nil {
				t.Fatalf("unexpected err: %s", err)
			}
			for hash, expAlert := range tc.expAlerts {
				gotAlert := tc.rule.alerts[hash]
				if gotAlert == nil {
					t.Fatalf("alert %d is missing; labels: %v; annotations: %v",
						hash, expAlert.Labels, expAlert.Annotations)
				}
				if !reflect.DeepEqual(expAlert.Annotations, gotAlert.Annotations) {
					t.Fatalf("expected to have annotations %#v; got %#v", expAlert.Annotations, gotAlert.Annotations)
				}
				if !reflect.DeepEqual(expAlert.Labels, gotAlert.Labels) {
					t.Fatalf("expected to have labels %#v; got %#v", expAlert.Labels, gotAlert.Labels)
				}
			}
		})
	}
}

func newTestRuleWithLabels(name string, labels ...string) *AlertingRule {
	r := newTestAlertingRule(name, 0)
	r.Labels = make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		r.Labels[labels[i]] = labels[i+1]
	}
	return r
}

func newTestAlertingRule(name string, waitFor time.Duration) *AlertingRule {
	return &AlertingRule{Name: name, alerts: make(map[uint64]*notifier.Alert), For: waitFor}
}
