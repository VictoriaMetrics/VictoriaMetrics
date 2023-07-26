package main

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
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
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
				}),
			},
		},
		{
			newTestAlertingRule("instant extra labels", 0),
			&notifier.Alert{State: notifier.StateFiring, Labels: map[string]string{
				"job":      "foo",
				"instance": "bar",
			}},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
					"job":           "foo",
					"instance":      "bar",
				}),
			},
		},
		{
			newTestAlertingRule("instant labels override", 0),
			&notifier.Alert{State: notifier.StateFiring, Labels: map[string]string{
				alertStateLabel: "foo",
				"__name__":      "bar",
			}},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
				}),
			},
		},
		{
			newTestAlertingRule("for", time.Second),
			&notifier.Alert{State: notifier.StateFiring, ActiveAt: timestamp.Add(time.Second)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StateFiring.String(),
				}),
				newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
					[]int64{timestamp.UnixNano()},
					map[string]string{
						"__name__": alertForStateMetricName,
					}),
			},
		},
		{
			newTestAlertingRule("for pending", 10*time.Second),
			&notifier.Alert{State: notifier.StatePending, ActiveAt: timestamp.Add(time.Second)},
			[]prompbmarshal.TimeSeries{
				newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, map[string]string{
					"__name__":      alertMetricName,
					alertStateLabel: notifier.StatePending.String(),
				}),
				newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
					[]int64{timestamp.UnixNano()},
					map[string]string{
						"__name__": alertForStateMetricName,
					}),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			tc.rule.alerts[tc.alert.ID] = tc.alert
			tss := tc.rule.toTimeSeries(timestamp.Unix())
			if err := compareTimeSeries(t, tc.expTS, tss); err != nil {
				t.Fatalf("timeseries missmatch: %s", err)
			}
		})
	}
}

func TestAlertingRule_Exec(t *testing.T) {
	const defaultStep = 5 * time.Millisecond
	type testAlert struct {
		labels []string
		alert  *notifier.Alert
	}
	testCases := []struct {
		rule      *AlertingRule
		steps     [][]datasource.Metric
		expAlerts map[int][]testAlert
	}{
		{
			newTestAlertingRule("empty", 0),
			[][]datasource.Metric{},
			nil,
		},
		{
			newTestAlertingRule("empty labels", 0),
			[][]datasource.Metric{
				{datasource.Metric{Values: []float64{1}, Timestamps: []int64{1}}},
			},
			map[int][]testAlert{
				0: {{alert: &notifier.Alert{State: notifier.StateFiring}}},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
				{},
				{},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>inactive=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
				{},
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				5: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
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
			map[int][]testAlert{
				0: {
					{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
					{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateFiring}},
					{labels: []string{"name", "foo2"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				},
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
			// 3: fire third alert, set second inactive
			map[int][]testAlert{
				0: {
					{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				},
				1: {
					{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
					{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				},
				2: {
					{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
					{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateInactive}},
					{labels: []string{"name", "foo2"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				},
			},
		},
		{
			newTestAlertingRule("for-pending", time.Minute),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
			},
		},
		{
			newTestAlertingRule("for-fired", defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
			},
		},
		{
			newTestAlertingRule("for-pending=>empty", time.Second),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to delete pending alerts
				{},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				2: {},
			},
		},
		{
			newTestAlertingRule("for-pending=>firing=>inactive=>pending=>firing", defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to set alert inactive
				{},
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
			},
		},
		{
			newTestAlertingRuleWithKeepFiring("for-pending=>firing=>keepfiring=>firing", defaultStep, defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to keep firing
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
			},
		},
		{
			newTestAlertingRuleWithKeepFiring("for-pending=>firing=>keepfiring=>keepfiring=>inactive=>pending=>firing", defaultStep, 2*defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to keep firing
				{},
				// another empty step to keep firing
				{},
				// empty step to set alert inactive
				{},
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			map[int][]testAlert{
				0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
				4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
				5: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
				6: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.q = fq
			tc.rule.GroupID = fakeGroup.ID()
			for i, step := range tc.steps {
				fq.reset()
				fq.add(step...)
				if _, err := tc.rule.Exec(context.TODO(), time.Now(), 0); err != nil {
					t.Fatalf("unexpected err: %s", err)
				}
				// artificial delay between applying steps
				time.Sleep(defaultStep)
				if _, ok := tc.expAlerts[i]; !ok {
					continue
				}
				if len(tc.rule.alerts) != len(tc.expAlerts[i]) {
					t.Fatalf("evalIndex %d: expected %d alerts; got %d", i, len(tc.expAlerts[i]), len(tc.rule.alerts))
				}
				expAlerts := make(map[uint64]*notifier.Alert)
				for _, ta := range tc.expAlerts[i] {
					labels := make(map[string]string)
					for i := 0; i < len(ta.labels); i += 2 {
						k, v := ta.labels[i], ta.labels[i+1]
						labels[k] = v
					}
					labels[alertNameLabel] = tc.rule.Name
					h := hash(labels)
					expAlerts[h] = ta.alert
				}
				for key, exp := range expAlerts {
					got, ok := tc.rule.alerts[key]
					if !ok {
						t.Fatalf("evalIndex %d: expected to have key %d", i, key)
					}
					if got.State != exp.State {
						t.Fatalf("evalIndex %d: expected state %d; got %d", i, exp.State, got.State)
					}
				}
			}
		})
	}
}

func TestAlertingRule_ExecRange(t *testing.T) {
	testCases := []struct {
		rule      *AlertingRule
		data      []datasource.Metric
		expAlerts []*notifier.Alert
	}{
		{
			newTestAlertingRule("empty", 0),
			[]datasource.Metric{},
			nil,
		},
		{
			newTestAlertingRule("empty labels", 0),
			[]datasource.Metric{
				{Values: []float64{1}, Timestamps: []int64{1}},
			},
			[]*notifier.Alert{
				{State: notifier.StateFiring},
			},
		},
		{
			newTestAlertingRule("single-firing", 0),
			[]datasource.Metric{
				metricWithLabels(t, "name", "foo"),
			},
			[]*notifier.Alert{
				{
					Labels: map[string]string{"name": "foo"},
					State:  notifier.StateFiring,
				},
			},
		},
		{
			newTestAlertingRule("single-firing-on-range", 0),
			[]datasource.Metric{
				{Values: []float64{1, 1, 1}, Timestamps: []int64{1e3, 2e3, 3e3}},
			},
			[]*notifier.Alert{
				{State: notifier.StateFiring},
				{State: notifier.StateFiring},
				{State: notifier.StateFiring},
			},
		},
		{
			newTestAlertingRule("for-pending", time.Second),
			[]datasource.Metric{
				{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
			},
			[]*notifier.Alert{
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(3, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(5, 0)},
			},
		},
		{
			newTestAlertingRule("for-firing", 3*time.Second),
			[]datasource.Metric{
				{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
			},
			[]*notifier.Alert{
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
			},
		},
		{
			newTestAlertingRule("for=>pending=>firing=>pending=>firing=>pending", time.Second),
			[]datasource.Metric{
				{Values: []float64{1, 1, 1, 1, 1}, Timestamps: []int64{1, 2, 5, 6, 20}},
			},
			[]*notifier.Alert{
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(5, 0)},
				{State: notifier.StateFiring, ActiveAt: time.Unix(5, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(20, 0)},
			},
		},
		{
			newTestAlertingRule("multi-series-for=>pending=>pending=>firing", 3*time.Second),
			[]datasource.Metric{
				{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
				{Values: []float64{1, 1}, Timestamps: []int64{1, 5},
					Labels: []datasource.Label{{Name: "foo", Value: "bar"}},
				},
			},
			[]*notifier.Alert{
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
				{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
				//
				{State: notifier.StatePending, ActiveAt: time.Unix(1, 0),
					Labels: map[string]string{
						"foo": "bar",
					}},
				{State: notifier.StatePending, ActiveAt: time.Unix(5, 0),
					Labels: map[string]string{
						"foo": "bar",
					}},
			},
		},
		{
			newTestRuleWithLabels("multi-series-firing", "source", "vm"),
			[]datasource.Metric{
				{Values: []float64{1, 1}, Timestamps: []int64{1, 100}},
				{Values: []float64{1, 1}, Timestamps: []int64{1, 5},
					Labels: []datasource.Label{{Name: "foo", Value: "bar"}},
				},
			},
			[]*notifier.Alert{
				{State: notifier.StateFiring, Labels: map[string]string{
					"source": "vm",
				}},
				{State: notifier.StateFiring, Labels: map[string]string{
					"source": "vm",
				}},
				//
				{State: notifier.StateFiring, Labels: map[string]string{
					"foo":    "bar",
					"source": "vm",
				}},
				{State: notifier.StateFiring, Labels: map[string]string{
					"foo":    "bar",
					"source": "vm",
				}},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_ExecRange"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.q = fq
			tc.rule.GroupID = fakeGroup.ID()
			fq.add(tc.data...)
			gotTS, err := tc.rule.ExecRange(context.TODO(), time.Now(), time.Now())
			if err != nil {
				t.Fatalf("unexpected err: %s", err)
			}
			var expTS []prompbmarshal.TimeSeries
			var j int
			for _, series := range tc.data {
				for _, timestamp := range series.Timestamps {
					a := tc.expAlerts[j]
					if a.Labels == nil {
						a.Labels = make(map[string]string)
					}
					a.Labels[alertNameLabel] = tc.rule.Name
					expTS = append(expTS, tc.rule.alertToTimeSeries(a, timestamp)...)
					j++
				}
			}
			if len(gotTS) != len(expTS) {
				t.Fatalf("expected %d time series; got %d", len(expTS), len(gotTS))
			}
			for i := range expTS {
				got, exp := gotTS[i], expTS[i]
				if !reflect.DeepEqual(got, exp) {
					t.Fatalf("%d: expected \n%v but got \n%v", i, exp, got)
				}
			}
		})
	}
}

func TestGroup_Restore(t *testing.T) {
	defaultTS := time.Now()
	fqr := &fakeQuerierWithRegistry{}
	fn := func(rules []config.Rule, expAlerts map[uint64]*notifier.Alert) {
		t.Helper()
		defer fqr.reset()

		for _, r := range rules {
			fqr.set(r.Expr, metricWithValueAndLabels(t, 0, "__name__", r.Alert))
		}

		fg := newGroup(config.Group{Name: "TestRestore", Rules: rules}, fqr, time.Second, nil)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			nts := func() []notifier.Notifier { return []notifier.Notifier{&fakeNotifier{}} }
			fg.start(context.Background(), nts, nil, fqr)
			wg.Done()
		}()
		fg.close()
		wg.Wait()

		gotAlerts := make(map[uint64]*notifier.Alert)
		for _, rs := range fg.Rules {
			alerts := rs.(*AlertingRule).alerts
			for k, v := range alerts {
				if !v.Restored {
					// set not restored alerts to predictable timestamp
					v.ActiveAt = defaultTS
				}
				gotAlerts[k] = v
			}
		}

		if len(gotAlerts) != len(expAlerts) {
			t.Fatalf("expected %d alerts; got %d", len(expAlerts), len(gotAlerts))
		}
		for key, exp := range expAlerts {
			got, ok := gotAlerts[key]
			if !ok {
				t.Fatalf("expected to have key %d", key)
			}
			if got.State != notifier.StatePending {
				t.Fatalf("expected state %d; got %d", notifier.StatePending, got.State)
			}
			if got.ActiveAt != exp.ActiveAt {
				t.Fatalf("expected ActiveAt %v; got %v", exp.ActiveAt, got.ActiveAt)
			}
		}
	}

	stateMetric := func(name string, value time.Time, labels ...string) datasource.Metric {
		labels = append(labels, "__name__", alertForStateMetricName)
		labels = append(labels, alertNameLabel, name)
		labels = append(labels, alertGroupNameLabel, "TestRestore")
		return metricWithValueAndLabels(t, float64(value.Unix()), labels...)
	}

	// one active alert, no previous state
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutils.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: defaultTS,
			},
		})
	fqr.reset()

	// one active alert with state restore
	ts := time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		stateMetric("foo", ts))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutils.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: ts},
		})

	// two rules, two active alerts, one with state restored
	ts = time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="bar"}[3600s])`,
		stateMetric("foo", ts))
	fn(
		[]config.Rule{
			{Alert: "foo", Expr: "foo", For: promutils.NewDuration(time.Second)},
			{Alert: "bar", Expr: "bar", For: promutils.NewDuration(time.Second)},
		},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: defaultTS,
			},
			hash(map[string]string{alertNameLabel: "bar", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: ts},
		})

	// two rules, two active alerts, two with state restored
	ts = time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		stateMetric("foo", ts))
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="bar"}[3600s])`,
		stateMetric("bar", ts))
	fn(
		[]config.Rule{
			{Alert: "foo", Expr: "foo", For: promutils.NewDuration(time.Second)},
			{Alert: "bar", Expr: "bar", For: promutils.NewDuration(time.Second)},
		},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: ts,
			},
			hash(map[string]string{alertNameLabel: "bar", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: ts},
		})

	// one active alert but wrong state restore
	ts = time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertname="bar",alertgroup="TestRestore"}[3600s])`,
		stateMetric("wrong alert", ts))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutils.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				ActiveAt: defaultTS,
			},
		})

	// one active alert with labels
	ts = time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo",env="dev"}[3600s])`,
		stateMetric("foo", ts, "env", "dev"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", Labels: map[string]string{"env": "dev"}, For: promutils.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				ActiveAt: ts,
			},
		})

	// one active alert with restore labels missmatch
	ts = time.Now().Truncate(time.Hour)
	fqr.set(`last_over_time(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo",env="dev"}[3600s])`,
		stateMetric("foo", ts, "env", "dev", "team", "foo"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", Labels: map[string]string{"env": "dev"}, For: promutils.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				ActiveAt: defaultTS,
			},
		})
}

func TestAlertingRule_Exec_Negative(t *testing.T) {
	fq := &fakeQuerier{}
	ar := newTestAlertingRule("test", 0)
	ar.Labels = map[string]string{"job": "test"}
	ar.q = fq

	// successful attempt
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
	_, err := ar.Exec(context.TODO(), time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}

	// label `job` will collide with rule extra label and will make both time series equal
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "baz"))
	_, err = ar.Exec(context.TODO(), time.Now(), 0)
	if !errors.Is(err, errDuplicate) {
		t.Fatalf("expected to have %s error; got %s", errDuplicate, err)
	}

	fq.reset()

	expErr := "connection reset by peer"
	fq.setErr(errors.New(expErr))
	_, err = ar.Exec(context.TODO(), time.Now(), 0)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to get err %q; got %q insterad", expErr, err)
	}
}

func TestAlertingRuleLimit(t *testing.T) {
	fq := &fakeQuerier{}
	ar := newTestAlertingRule("test", 0)
	ar.Labels = map[string]string{"job": "test"}
	ar.q = fq
	ar.For = time.Minute
	testCases := []struct {
		limit  int
		err    string
		tssNum int
	}{
		{
			limit:  0,
			tssNum: 4,
		},
		{
			limit:  -1,
			tssNum: 4,
		},
		{
			limit:  1,
			err:    "exec exceeded limit of 1 with 2 alerts",
			tssNum: 0,
		},
		{
			limit:  4,
			tssNum: 4,
		},
	}
	var (
		err       error
		timestamp = time.Now()
	)
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "bar", "job"))
	for _, testCase := range testCases {
		_, err = ar.Exec(context.TODO(), timestamp, testCase.limit)
		if err != nil && !strings.EqualFold(err.Error(), testCase.err) {
			t.Fatal(err)
		}
	}
	fq.reset()
}

func TestAlertingRule_Template(t *testing.T) {
	testCases := []struct {
		rule      *AlertingRule
		metrics   []datasource.Metric
		expAlerts map[uint64]*notifier.Alert
	}{
		{
			&AlertingRule{
				Name: "common",
				Labels: map[string]string{
					"region": "east",
				},
				Annotations: map[string]string{
					"summary": `{{ $labels.alertname }}: Too high connection number for "{{ $labels.instance }}"`,
				},
				alerts: make(map[uint64]*notifier.Alert),
			},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 1, "instance", "foo"),
				metricWithValueAndLabels(t, 1, "instance", "bar"),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{alertNameLabel: "common", "region": "east", "instance": "foo"}): {
					Annotations: map[string]string{
						"summary": `common: Too high connection number for "foo"`,
					},
					Labels: map[string]string{
						alertNameLabel: "common",
						"region":       "east",
						"instance":     "foo",
					},
				},
				hash(map[string]string{alertNameLabel: "common", "region": "east", "instance": "bar"}): {
					Annotations: map[string]string{
						"summary": `common: Too high connection number for "bar"`,
					},
					Labels: map[string]string{
						alertNameLabel: "common",
						"region":       "east",
						"instance":     "bar",
					},
				},
			},
		},
		{
			&AlertingRule{
				Name: "override label",
				Labels: map[string]string{
					"instance": "{{ $labels.instance }}",
				},
				Annotations: map[string]string{
					"summary":     `{{ $labels.__name__ }}: Too high connection number for "{{ $labels.instance }}"`,
					"description": `{{ $labels.alertname}}: It is {{ $value }} connections for "{{ $labels.instance }}"`,
				},
				alerts: make(map[uint64]*notifier.Alert),
			},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 2, "__name__", "first", "instance", "foo", alertNameLabel, "override"),
				metricWithValueAndLabels(t, 10, "__name__", "second", "instance", "bar", alertNameLabel, "override"),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{alertNameLabel: "override label", "instance": "foo"}): {
					Labels: map[string]string{
						alertNameLabel: "override label",
						"instance":     "foo",
					},
					Annotations: map[string]string{
						"summary":     `first: Too high connection number for "foo"`,
						"description": `override: It is 2 connections for "foo"`,
					},
				},
				hash(map[string]string{alertNameLabel: "override label", "instance": "bar"}): {
					Labels: map[string]string{
						alertNameLabel: "override label",
						"instance":     "bar",
					},
					Annotations: map[string]string{
						"summary":     `second: Too high connection number for "bar"`,
						"description": `override: It is 10 connections for "bar"`,
					},
				},
			},
		},
		{
			&AlertingRule{
				Name:      "OriginLabels",
				GroupName: "Testing",
				Labels: map[string]string{
					"instance": "{{ $labels.instance }}",
				},
				Annotations: map[string]string{
					"summary": `Alert "{{ $labels.alertname }}({{ $labels.alertgroup }})" for instance {{ $labels.instance }}`,
				},
				alerts: make(map[uint64]*notifier.Alert),
			},
			[]datasource.Metric{
				metricWithValueAndLabels(t, 1,
					alertNameLabel, "originAlertname",
					alertGroupNameLabel, "originGroupname",
					"instance", "foo"),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{
					alertNameLabel:      "OriginLabels",
					alertGroupNameLabel: "Testing",
					"instance":          "foo"}): {
					Labels: map[string]string{
						alertNameLabel:      "OriginLabels",
						alertGroupNameLabel: "Testing",
						"instance":          "foo",
					},
					Annotations: map[string]string{
						"summary": `Alert "originAlertname(originGroupname)" for instance foo`,
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
			tc.rule.q = fq
			tc.rule.state = newRuleState(10)
			fq.add(tc.metrics...)
			if _, err := tc.rule.Exec(context.TODO(), time.Now(), 0); err != nil {
				t.Fatalf("unexpected err: %s", err)
			}
			for hash, expAlert := range tc.expAlerts {
				gotAlert := tc.rule.alerts[hash]
				if gotAlert == nil {
					t.Fatalf("alert %d is missing; labels: %v; annotations: %v", hash, expAlert.Labels, expAlert.Annotations)
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

func TestAlertsToSend(t *testing.T) {
	ts := time.Now()
	f := func(alerts, expAlerts []*notifier.Alert, resolveDuration, resendDelay time.Duration) {
		t.Helper()
		ar := &AlertingRule{alerts: make(map[uint64]*notifier.Alert)}
		for i, a := range alerts {
			ar.alerts[uint64(i)] = a
		}
		gotAlerts := ar.alertsToSend(ts, resolveDuration, resendDelay)
		if gotAlerts == nil && expAlerts == nil {
			return
		}
		if len(gotAlerts) != len(expAlerts) {
			t.Fatalf("expected to get %d alerts; got %d instead",
				len(expAlerts), len(gotAlerts))
		}
		sort.Slice(expAlerts, func(i, j int) bool {
			return expAlerts[i].Name < expAlerts[j].Name
		})
		sort.Slice(gotAlerts, func(i, j int) bool {
			return gotAlerts[i].Name < gotAlerts[j].Name
		})
		for i, exp := range expAlerts {
			got := gotAlerts[i]
			if got.LastSent != exp.LastSent {
				t.Fatalf("expected LastSent to be %v; got %v", exp.LastSent, got.LastSent)
			}
			if got.End != exp.End {
				t.Fatalf("expected End to be %v; got %v", exp.End, got.End)
			}
		}
	}

	f( // send firing alert with custom resolve time
		[]*notifier.Alert{{State: notifier.StateFiring}},
		[]*notifier.Alert{{LastSent: ts, End: ts.Add(5 * time.Minute)}},
		5*time.Minute, time.Minute,
	)
	f( // resolve inactive alert at the current timestamp
		[]*notifier.Alert{{State: notifier.StateInactive, ResolvedAt: ts}},
		[]*notifier.Alert{{LastSent: ts, End: ts}},
		time.Minute, time.Minute,
	)
	f( // mixed case of firing and resolved alerts. Names are added for deterministic sorting
		[]*notifier.Alert{{Name: "a", State: notifier.StateFiring}, {Name: "b", State: notifier.StateInactive, ResolvedAt: ts}},
		[]*notifier.Alert{{Name: "a", LastSent: ts, End: ts.Add(5 * time.Minute)}, {Name: "b", LastSent: ts, End: ts}},
		5*time.Minute, time.Minute,
	)
	f( // mixed case of pending and resolved alerts. Names are added for deterministic sorting
		[]*notifier.Alert{{Name: "a", State: notifier.StatePending}, {Name: "b", State: notifier.StateInactive, ResolvedAt: ts}},
		[]*notifier.Alert{{Name: "b", LastSent: ts, End: ts}},
		5*time.Minute, time.Minute,
	)
	f( // attempt to send alert that was already sent in the resendDelay interval
		[]*notifier.Alert{{State: notifier.StateFiring, LastSent: ts.Add(-time.Second)}},
		nil,
		time.Minute, time.Minute,
	)
	f( // attempt to send alert that was sent out of the resendDelay interval
		[]*notifier.Alert{{State: notifier.StateFiring, LastSent: ts.Add(-2 * time.Minute)}},
		[]*notifier.Alert{{LastSent: ts, End: ts.Add(time.Minute)}},
		time.Minute, time.Minute,
	)
	f( // alert must be sent even if resendDelay interval is 0
		[]*notifier.Alert{{State: notifier.StateFiring, LastSent: ts.Add(-time.Second)}},
		[]*notifier.Alert{{LastSent: ts, End: ts.Add(time.Minute)}},
		time.Minute, 0,
	)
	f( // inactive alert which has been sent already
		[]*notifier.Alert{{State: notifier.StateInactive, LastSent: ts.Add(-time.Second), ResolvedAt: ts.Add(-2 * time.Second)}},
		nil,
		time.Minute, time.Minute,
	)
	f( // inactive alert which has been resolved after last send
		[]*notifier.Alert{{State: notifier.StateInactive, LastSent: ts.Add(-time.Second), ResolvedAt: ts}},
		[]*notifier.Alert{{LastSent: ts, End: ts}},
		time.Minute, time.Minute,
	)
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
	rule := AlertingRule{
		Name:         name,
		For:          waitFor,
		EvalInterval: waitFor,
		alerts:       make(map[uint64]*notifier.Alert),
		state:        newRuleState(10),
	}
	return &rule
}

func newTestAlertingRuleWithKeepFiring(name string, waitFor, keepFiringFor time.Duration) *AlertingRule {
	rule := AlertingRule{
		Name:          name,
		For:           waitFor,
		EvalInterval:  waitFor,
		alerts:        make(map[uint64]*notifier.Alert),
		state:         newRuleState(10),
		KeepFiringFor: keepFiringFor,
	}
	return &rule
}
