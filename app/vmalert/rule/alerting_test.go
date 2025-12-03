package rule

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/vmalertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestNewAlertingRule(t *testing.T) {
	f := func(group *Group, rule config.Rule, expectRule *AlertingRule) {
		t.Helper()

		r := NewAlertingRule(&datasource.FakeQuerier{}, group, rule)
		if err := CompareRules(t, expectRule, r); err != nil {
			t.Fatalf("unexpected rule mismatch: %s", err)
		}
	}

	f(&Group{Name: "foo"},
		config.Rule{
			Alert: "health",
			Expr:  "up == 0",
			Labels: map[string]string{
				"foo": "bar",
			},
		}, &AlertingRule{
			Name: "health",
			Expr: "up == 0",
			Labels: map[string]string{
				"foo": "bar",
			},
		})
}

func TestAlertingRuleToTimeSeries(t *testing.T) {
	timestamp := time.Now()

	f := func(rule *AlertingRule, alert *notifier.Alert, tssExpected []prompb.TimeSeries) {
		t.Helper()

		rule.alerts[alert.ID] = alert
		tss := rule.toTimeSeries(timestamp.Unix())
		if err := compareTimeSeries(t, tssExpected, tss); err != nil {
			t.Fatalf("timeseries mismatch for rule %q: %s", rule.Name, err)
		}
	}

	f(newTestAlertingRule("instant", 0), &notifier.Alert{
		State:    notifier.StateFiring,
		ActiveAt: timestamp.Add(time.Second),
	}, []prompb.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, []prompb.Label{
			{
				Name:  "__name__",
				Value: alertMetricName,
			},
			{
				Name:  alertStateLabel,
				Value: notifier.StateFiring.String(),
			},
		}),
		newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
			[]int64{timestamp.UnixNano()},
			[]prompb.Label{
				{
					Name:  "__name__",
					Value: alertForStateMetricName,
				},
			}),
	})

	f(newTestAlertingRule("instant extra labels", 0), &notifier.Alert{
		State: notifier.StateFiring, ActiveAt: timestamp.Add(time.Second),
		Labels: map[string]string{
			"job":      "foo",
			"instance": "bar",
		},
	}, []prompb.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()},
			[]prompb.Label{
				{
					Name:  "__name__",
					Value: alertMetricName,
				},
				{
					Name:  alertStateLabel,
					Value: notifier.StateFiring.String(),
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "instance",
					Value: "bar",
				},
			}),
		newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
			[]int64{timestamp.UnixNano()},
			[]prompb.Label{
				{
					Name:  "__name__",
					Value: alertForStateMetricName,
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "instance",
					Value: "bar",
				},
			}),
	})

	f(newTestAlertingRule("instant labels override", 0), &notifier.Alert{
		State: notifier.StateFiring, ActiveAt: timestamp.Add(time.Second),
		Labels: map[string]string{
			alertStateLabel: "foo",
		},
	}, []prompb.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, []prompb.Label{
			{
				Name:  "__name__",
				Value: alertMetricName,
			},
			{
				Name:  alertStateLabel,
				Value: notifier.StateFiring.String(),
			},
		}),
		newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
			[]int64{timestamp.UnixNano()},
			[]prompb.Label{
				{
					Name:  "__name__",
					Value: alertForStateMetricName,
				},
				{
					Name:  alertStateLabel,
					Value: "foo",
				},
			}),
	})

	f(newTestAlertingRule("for", time.Second), &notifier.Alert{
		State:    notifier.StateFiring,
		ActiveAt: timestamp.Add(time.Second),
	}, []prompb.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, []prompb.Label{
			{
				Name:  "__name__",
				Value: alertMetricName,
			},
			{
				Name:  alertStateLabel,
				Value: notifier.StateFiring.String(),
			},
		}),
		newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())},
			[]int64{timestamp.UnixNano()},
			[]prompb.Label{
				{
					Name:  "__name__",
					Value: alertForStateMetricName,
				},
			}),
	})

	f(newTestAlertingRule("for pending", 10*time.Second), &notifier.Alert{
		State:    notifier.StatePending,
		ActiveAt: timestamp.Add(time.Second),
	}, []prompb.TimeSeries{
		newTimeSeries([]float64{1}, []int64{timestamp.UnixNano()}, []prompb.Label{
			{
				Name:  "__name__",
				Value: alertMetricName,
			},
			{
				Name:  alertStateLabel,
				Value: notifier.StatePending.String(),
			},
		}),
		newTimeSeries([]float64{float64(timestamp.Add(time.Second).Unix())}, []int64{timestamp.UnixNano()}, []prompb.Label{
			{
				Name:  "__name__",
				Value: alertForStateMetricName,
			},
		}),
	})
}

func TestAlertingRule_Exec(t *testing.T) {
	const defaultStep = 5 * time.Millisecond
	type testAlert struct {
		labels []string
		alert  *notifier.Alert
	}

	ts, _ := time.Parse(time.RFC3339, "2024-10-29T00:00:00Z")

	f := func(rule *AlertingRule, steps [][]datasource.Metric, alertsExpected map[int][]testAlert, tssExpected map[int][]prompb.TimeSeries) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		rule.q = fq

		fakeGroup := Group{
			Name: "TestRule_Exec",
		}
		rule.GroupID = fakeGroup.GetID()
		for i, step := range steps {
			fq.Reset()
			fq.Add(step...)
			tss, err := rule.exec(context.TODO(), ts, 0)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			// check generate time series
			if _, ok := tssExpected[i]; ok {
				if err := compareTimeSeries(t, tssExpected[i], tss); err != nil {
					t.Fatalf("generated time series mismatch for rule %q in step %d: %s", rule.Name, i, err)
				}
			}

			// shift the execution timestamp before the next iteration
			ts = ts.Add(defaultStep)

			if _, ok := alertsExpected[i]; !ok {
				continue
			}
			if len(rule.alerts) != len(alertsExpected[i]) {
				t.Fatalf("evalIndex %d: expected %d alerts; got %d", i, len(alertsExpected[i]), len(rule.alerts))
			}
			expAlerts := make(map[uint64]*notifier.Alert)
			for _, ta := range alertsExpected[i] {
				labels := make(map[string]string)
				for i := 0; i < len(ta.labels); i += 2 {
					k, v := ta.labels[i], ta.labels[i+1]
					labels[k] = v
				}
				labels[alertNameLabel] = rule.Name
				h := hash(labels)
				expAlerts[h] = ta.alert
			}
			for key, exp := range expAlerts {
				got, ok := rule.alerts[key]
				if !ok {
					t.Fatalf("evalIndex %d: expected to have key %d", i, key)
				}
				if got.State != exp.State {
					t.Fatalf("evalIndex %d: expected state %d; got %d", i, exp.State, got.State)
				}
				if rule.Annotations != nil && exp.Annotations != nil {
					if !reflect.DeepEqual(got.Annotations, exp.Annotations) {
						t.Fatalf("evalIndex %d: expected annotations %v; got %v", i, exp.Annotations, got.Annotations)
					}
				}
			}
		}
		// reset ts for next test
		ts, _ = time.Parse(time.RFC3339, "2024-10-29T00:00:00Z")
	}

	f(newTestAlertingRule("empty", 0), [][]datasource.Metric{}, nil, nil)

	f(newTestAlertingRule("empty_labels", 0), [][]datasource.Metric{
		{datasource.Metric{Values: []float64{1}, Timestamps: []int64{1}}},
	}, map[int][]testAlert{
		0: {{alert: &notifier.Alert{State: notifier.StateFiring}}},
	},
		map[int][]prompb.TimeSeries{
			0: {
				{
					Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "empty_labels"}, {Name: "alertstate", Value: "firing"}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: ts.UnixNano() / 1e6}},
				},
				{
					Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "empty_labels"}},
					Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.UnixNano() / 1e6}},
				},
			},
		})

	f(newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>inactive", 0), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{},
		{metricWithLabels(t, "name", "foo")},
		{},
		{},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
	}, map[int][]prompb.TimeSeries{
		0: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.UnixNano() / 1e6}},
			},
		},
		1: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
		},
		2: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "single-firing=>inactive=>firing=>inactive=>inactive"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Add(2 * defaultStep).Unix()), Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
		},
	})

	f(newTestAlertingRule("single-firing=>inactive=>firing=>inactive=>inactive=>firing", 0), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{},
		{metricWithLabels(t, "name", "foo")},
		{},
		{},
		{metricWithLabels(t, "name", "foo")},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		5: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
	}, nil)

	f(newTestAlertingRule("multiple-firing", 0), [][]datasource.Metric{
		{
			metricWithLabels(t, "name", "foo"),
			metricWithLabels(t, "name", "foo1"),
			metricWithLabels(t, "name", "foo2"),
		},
	}, map[int][]testAlert{
		0: {
			{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
			{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateFiring}},
			{labels: []string{"name", "foo2"}, alert: &notifier.Alert{State: notifier.StateFiring}},
		},
	}, nil)

	// 1: fire first alert
	// 2: fire second alert, set first inactive
	// 3: fire third alert, set second inactive
	f(newTestAlertingRule("multiple-steps-firing", 0), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{metricWithLabels(t, "name", "foo1")},
		{metricWithLabels(t, "name", "foo2")},
	}, map[int][]testAlert{
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
	}, map[int][]prompb.TimeSeries{
		0: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.UnixNano() / 1e6}},
			},
		},
		1: {
			// stale time series for foo, `firing -> inactive`
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			// new time series for foo1
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo1"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "name", Value: "foo1"}},
				Samples: []prompb.Sample{{Value: float64(ts.Add(defaultStep).Unix()), Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
		},
		2: {
			// stale time series for foo1
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo1"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "name", Value: "foo1"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
			// new time series for foo2
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo2"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "multiple-steps-firing"}, {Name: "name", Value: "foo2"}},
				Samples: []prompb.Sample{{Value: float64(ts.Add(2 * defaultStep).Unix()), Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
		},
	})

	f(newTestAlertingRule("for-pending", time.Minute), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
	}, nil)

	f(newTestAlertingRule("for-fired", defaultStep), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{metricWithLabels(t, "name", "foo")},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
	}, map[int][]prompb.TimeSeries{
		0: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "for-fired"}, {Name: "alertstate", Value: "pending"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "for-fired"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.UnixNano() / 1e6}},
			},
		},
		1: {
			// stale time series for `pending -> firing`
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "for-fired"}, {Name: "alertstate", Value: "pending"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "alertname", Value: "for-fired"}, {Name: "alertstate", Value: "firing"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "alertname", Value: "for-fired"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Add(defaultStep).Unix()), Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
		},
	})

	f(newTestAlertingRule("for-pending=>empty", time.Second), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo", "a1", "b1", "a2", "b2", "a3", "b3")},
		{metricWithLabels(t, "name", "foo", "a1", "b1", "a2", "b2", "a3", "b3")},
		// empty step to delete pending alerts
		{},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo", "a1", "b1", "a2", "b2", "a3", "b3"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		1: {{labels: []string{"name", "foo", "a1", "b1", "a2", "b2", "a3", "b3"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		2: {},
	}, map[int][]prompb.TimeSeries{
		0: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "alertstate", Value: "pending"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.UnixNano() / 1e6}},
			},
		},
		1: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "alertstate", Value: "pending"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: float64(ts.Unix()), Timestamp: ts.Add(defaultStep).UnixNano() / 1e6}},
			},
		},
		// stale time series for `pending -> inactive`
		2: {
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "alertstate", Value: "pending"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: alertForStateMetricName}, {Name: "a1", Value: "b1"}, {Name: "a2", Value: "b2"}, {Name: "a3", Value: "b3"}, {Name: "alertname", Value: "for-pending=>empty"}, {Name: "name", Value: "foo"}},
				Samples: []prompb.Sample{{Value: decimal.StaleNaN, Timestamp: ts.Add(2*defaultStep).UnixNano() / 1e6}},
			},
		},
	})

	f(newTestAlertingRuleWithCustomFields("for-pending=>firing=>inactive=>pending=>firing", defaultStep, 0, 0, map[string]string{"activeAt": "{{ $activeAt.UnixMilli }}"}), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{metricWithLabels(t, "name", "foo")},
		// empty step to set alert inactive
		{},
		{metricWithLabels(t, "name", "foo")},
		{metricWithLabels(t, "name", "foo")},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending, Annotations: map[string]string{"activeAt": strconv.FormatInt(ts.UnixMilli(), 10)}}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring, Annotations: map[string]string{"activeAt": strconv.FormatInt(ts.UnixMilli(), 10)}}}},
		2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive, Annotations: map[string]string{"activeAt": strconv.FormatInt(ts.UnixMilli(), 10)}}}},
		3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending, Annotations: map[string]string{"activeAt": strconv.FormatInt(ts.Add(defaultStep*3).UnixMilli(), 10)}}}},
		4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring, Annotations: map[string]string{"activeAt": strconv.FormatInt(ts.Add(defaultStep*3).UnixMilli(), 10)}}}},
	}, nil)

	f(newTestAlertingRuleWithCustomFields("for-pending=>firing=>keepfiring=>firing", defaultStep, 0, defaultStep, nil), [][]datasource.Metric{
		{metricWithLabels(t, "name", "foo")},
		{metricWithLabels(t, "name", "foo")},
		// empty step to keep firing
		{},
		{metricWithLabels(t, "name", "foo")},
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
	}, nil)

	f(newTestAlertingRuleWithCustomFields("for-pending=>firing=>keepfiring=>keepfiring=>inactive=>pending=>firing", defaultStep, 0, 2*defaultStep, nil), [][]datasource.Metric{
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
	}, map[int][]testAlert{
		0: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		1: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		2: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		3: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
		4: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}}},
		5: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}}},
		6: {{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}}},
	}, nil)
}

func TestAlertingRuleExecRange(t *testing.T) {
	fakeGroup := Group{
		Name: "TestRule_ExecRange",
	}

	f := func(rule *AlertingRule, data []datasource.Metric, alertsExpected []*notifier.Alert, holdAlertStateAlertsExpected map[uint64]*notifier.Alert) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		rule.q = fq
		rule.GroupID = fakeGroup.GetID()
		fq.Add(data...)
		gotTS, err := rule.execRange(context.TODO(), time.Unix(1, 0), time.Unix(5, 0))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		var expTS []prompb.TimeSeries
		var j int
		for _, series := range data {
			for _, timestamp := range series.Timestamps {
				a := alertsExpected[j]
				if a.Labels == nil {
					a.Labels = make(map[string]string)
				}
				a.Labels[alertNameLabel] = rule.Name
				expTS = append(expTS, rule.alertToTimeSeries(a, timestamp)...)
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
		if holdAlertStateAlertsExpected != nil {
			if !reflect.DeepEqual(holdAlertStateAlertsExpected, rule.alerts) {
				t.Fatalf("expected hold alerts state: \n%v but got \n%v", holdAlertStateAlertsExpected, rule.alerts)
			}
		}
	}

	f(newTestAlertingRule("empty", 0), []datasource.Metric{}, nil, nil)

	f(newTestAlertingRule("empty labels", 0), []datasource.Metric{
		{Values: []float64{1}, Timestamps: []int64{1}},
	}, []*notifier.Alert{
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
	}, nil)

	f(newTestAlertingRule("single-firing", 0), []datasource.Metric{
		metricWithLabels(t, "name", "foo"),
	}, []*notifier.Alert{
		{
			Labels:   map[string]string{"name": "foo"},
			State:    notifier.StateFiring,
			ActiveAt: time.Unix(1, 0),
		},
	}, nil)

	f(newTestAlertingRule("single-firing-on-range", 0), []datasource.Metric{
		{Values: []float64{1, 1, 1}, Timestamps: []int64{1e3, 2e3, 3e3}},
	}, []*notifier.Alert{
		{State: notifier.StateFiring, ActiveAt: time.Unix(1e3, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(2e3, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(3e3, 0)},
	}, nil)

	f(newTestAlertingRuleWithCustomFields("for-pending", time.Second, 0, 0, map[string]string{"activeAt": "{{ $activeAt.UnixMilli }}"}), []datasource.Metric{
		{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
	}, []*notifier.Alert{
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(3, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(5, 0)},
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{"alertname": "for-pending"}): {
			GroupID:     fakeGroup.GetID(),
			Name:        "for-pending",
			Type:        config.NewPrometheusType().String(),
			Labels:      map[string]string{"alertname": "for-pending"},
			Annotations: map[string]string{"activeAt": "5000"},
			State:       notifier.StatePending,
			ActiveAt:    time.Unix(5, 0),
			Value:       1,
			For:         time.Second,
		},
	})

	f(newTestAlertingRuleWithCustomFields("for-firing", 3*time.Second, 0, 0, map[string]string{"activeAt": "{{ $activeAt.UnixMilli }}"}), []datasource.Metric{
		{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
	}, []*notifier.Alert{
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{"alertname": "for-firing"}): {
			GroupID:     fakeGroup.GetID(),
			Name:        "for-firing",
			Type:        config.NewPrometheusType().String(),
			Labels:      map[string]string{"alertname": "for-firing"},
			Annotations: map[string]string{"activeAt": "1000"},
			State:       notifier.StateFiring,
			ActiveAt:    time.Unix(1, 0),
			Start:       time.Unix(5, 0),
			Value:       1,
			For:         3 * time.Second,
		},
	})

	f(newTestAlertingRuleWithCustomFields("for-hold-pending", time.Second, 0, 0, map[string]string{"activeAt": "{{ $activeAt.UnixMilli }}"}), []datasource.Metric{
		{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 2, 5}},
	}, []*notifier.Alert{
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(5, 0)},
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{"alertname": "for-hold-pending"}): {
			GroupID:     fakeGroup.GetID(),
			Name:        "for-hold-pending",
			Type:        config.NewPrometheusType().String(),
			Labels:      map[string]string{"alertname": "for-hold-pending"},
			Annotations: map[string]string{"activeAt": "5000"},
			State:       notifier.StatePending,
			ActiveAt:    time.Unix(5, 0),
			Value:       1,
			For:         time.Second,
		},
	})

	f(newTestAlertingRuleWithCustomFields("firing=>inactive=>inactive=>firing=>firing", 0, time.Second, 0, nil), []datasource.Metric{
		{Values: []float64{1, 1, 1, 1}, Timestamps: []int64{1, 4, 5, 6}},
	}, []*notifier.Alert{
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
		// It is expected for ActiveAT to remain the same while rule continues to fire in each iteration
		{State: notifier.StateFiring, ActiveAt: time.Unix(4, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(4, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(4, 0)},
	}, nil)

	f(newTestAlertingRule("for=>pending=>firing=>pending=>firing=>pending", time.Second), []datasource.Metric{
		{Values: []float64{1, 1, 1, 1, 1}, Timestamps: []int64{1, 2, 5, 6, 20}},
	}, []*notifier.Alert{
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(5, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(5, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(20, 0)},
	}, nil)

	f(newTestAlertingRule("multi-series", 3*time.Second), []datasource.Metric{
		{Values: []float64{1, 1, 1}, Timestamps: []int64{1, 3, 5}},
		{
			Values: []float64{1, 1}, Timestamps: []int64{1, 5},
			Labels: []prompb.Label{{Name: "foo", Value: "bar"}},
		},
	}, []*notifier.Alert{
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StatePending, ActiveAt: time.Unix(1, 0)},
		{State: notifier.StateFiring, ActiveAt: time.Unix(1, 0)},
		{
			State: notifier.StatePending, ActiveAt: time.Unix(1, 0),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		{
			State: notifier.StatePending, ActiveAt: time.Unix(5, 0),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{"alertname": "multi-series"}): {
			GroupID:     fakeGroup.GetID(),
			Name:        "multi-series",
			Type:        config.NewPrometheusType().String(),
			Labels:      map[string]string{"alertname": "multi-series"},
			Annotations: map[string]string{},
			State:       notifier.StateFiring,
			ActiveAt:    time.Unix(1, 0),
			Start:       time.Unix(5, 0),
			Value:       1,
			For:         3 * time.Second,
		},
		hash(map[string]string{"alertname": "multi-series", "foo": "bar"}): {
			GroupID:     fakeGroup.GetID(),
			Name:        "multi-series",
			Type:        config.NewPrometheusType().String(),
			Labels:      map[string]string{"alertname": "multi-series", "foo": "bar"},
			Annotations: map[string]string{},
			State:       notifier.StatePending,
			ActiveAt:    time.Unix(5, 0),
			Value:       1,
			For:         3 * time.Second,
		},
	})

	f(newTestRuleWithLabels("multi-series-firing", "source", "vm"), []datasource.Metric{
		{Values: []float64{1, 1}, Timestamps: []int64{1, 100}},
		{
			Values: []float64{1, 1}, Timestamps: []int64{1, 5},
			Labels: []prompb.Label{{Name: "foo", Value: "bar"}},
		},
	}, []*notifier.Alert{
		{
			State: notifier.StateFiring, ActiveAt: time.Unix(1, 0),
			Labels: map[string]string{
				"source": "vm",
			},
		},
		{
			State: notifier.StateFiring, ActiveAt: time.Unix(100, 0),
			Labels: map[string]string{
				"source": "vm",
			},
		},
		{
			State: notifier.StateFiring, ActiveAt: time.Unix(1, 0),
			Labels: map[string]string{
				"foo":    "bar",
				"source": "vm",
			},
		},
		{
			State: notifier.StateFiring, ActiveAt: time.Unix(5, 0),
			Labels: map[string]string{
				"foo":    "bar",
				"source": "vm",
			},
		},
	}, nil)
}

func TestGroup_Restore(t *testing.T) {
	defaultTS := time.Now()
	fqr := &datasource.FakeQuerierWithRegistry{}
	fn := func(rules []config.Rule, expAlerts map[uint64]*notifier.Alert) {
		t.Helper()
		defer fqr.Reset()

		fg := NewGroup(config.Group{Name: "TestRestore", Rules: rules}, fqr, time.Second, nil)
		fg.Init()
		wg := sync.WaitGroup{}
		wg.Go(func() {
			fg.Start(context.Background(), nil, fqr)
		})
		fg.Close()
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
			if got.Name != exp.Name {
				t.Fatalf("expected alertname %q; got %q", exp.Name, got.Name)
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
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				Name:     "foo",
				ActiveAt: defaultTS,
			},
		})

	// one active alert with state restore
	ts := time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		stateMetric("foo", ts))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				Name:     "foo",
				ActiveAt: ts,
			},
		})

	// one rule, two active alerts, one with state restored
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo",
		metricWithValueAndLabels(t, 0, "__name__", "foo", "env", "prod"),
		metricWithValueAndLabels(t, 0, "__name__", "foo", "env", "dev"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		// only env=prod has state metric, so only it will have state restore
		stateMetric("foo", ts, "env", "prod"))
	fn(
		[]config.Rule{
			{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)},
		},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				Name:     "foo",
				ActiveAt: defaultTS,
			},
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "prod"}): {
				Name:     "foo",
				ActiveAt: ts,
			},
		})

	// two rules, two active alerts, one with state restored
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set("bar", metricWithValueAndLabels(t, 0, "__name__", "bar"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="bar"}[3600s])`,
		stateMetric("bar", ts))
	fn(
		[]config.Rule{
			{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)},
			{Alert: "bar", Expr: "bar", For: promutil.NewDuration(time.Second)},
		},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				Name:     "foo",
				ActiveAt: defaultTS,
			},
			hash(map[string]string{alertNameLabel: "bar", alertGroupNameLabel: "TestRestore"}): {
				Name:     "bar",
				ActiveAt: ts,
			},
		})

	// two rules, two active alerts, two with state restored
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set("bar", metricWithValueAndLabels(t, 0, "__name__", "bar"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		stateMetric("foo", ts))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="bar"}[3600s])`,
		stateMetric("bar", ts))
	fn(
		[]config.Rule{
			{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)},
			{Alert: "bar", Expr: "bar", For: promutil.NewDuration(time.Second)},
		},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				Name:     "foo",
				ActiveAt: ts,
			},
			hash(map[string]string{alertNameLabel: "bar", alertGroupNameLabel: "TestRestore"}): {
				Name:     "bar",
				ActiveAt: ts,
			},
		})

	// one active alert but wrong state restore
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertname="bar",alertgroup="TestRestore"}[3600s])`,
		stateMetric("wrong alert", ts))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore"}): {
				Name:     "foo",
				ActiveAt: defaultTS,
			},
		})

	// one active alert with labels
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo",env="dev"}[3600s])`,
		stateMetric("foo", ts, "env", "dev"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", Labels: map[string]string{"env": "dev"}, For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				Name:     "foo",
				ActiveAt: ts,
			},
		})

	// one active alert with restore labels mismatch
	ts = time.Now().Truncate(time.Hour)
	fqr.Set("foo", metricWithValueAndLabels(t, 0, "__name__", "foo"))
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo",env="dev"}[3600s])`,
		stateMetric("foo", ts, "env", "dev", "team", "foo"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", Labels: map[string]string{"env": "dev"}, For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				Name:     "foo",
				ActiveAt: defaultTS,
			},
		})

	// two active alerts with dynamic labels and restore
	ts = time.Now().Truncate(time.Hour)
	fqr.Set(`default_rollup(ALERTS_FOR_STATE{alertgroup="TestRestore",alertname="foo"}[3600s])`,
		stateMetric("foo", ts, "env", "dev"),
		stateMetric("foo", ts.Add(time.Second), "env", "prod"))
	fqr.Set("foo",
		metricWithValueAndLabels(t, 0, "__name__", "foo", "env", "dev"),
		metricWithValueAndLabels(t, 0, "__name__", "foo", "env", "prod"))
	fn(
		[]config.Rule{{Alert: "foo", Expr: "foo", Labels: map[string]string{"env": "{{$labels.env}}"}, For: promutil.NewDuration(time.Second)}},
		map[uint64]*notifier.Alert{
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "dev"}): {
				Name:     "foo",
				ActiveAt: ts,
			},
			hash(map[string]string{alertNameLabel: "foo", alertGroupNameLabel: "TestRestore", "env": "prod"}): {
				Name:     "foo",
				ActiveAt: ts.Add(time.Second),
			},
		})
}

func TestAlertingRule_Exec_Negative(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	ar := newTestAlertingRule("test", 0)
	ar.Labels = map[string]string{"job": "test"}
	ar.q = fq

	// successful attempt
	// label `job` will be overridden by rule extra label, the original value will be reserved by "exported_job"
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "baz"))
	_, err := ar.exec(context.TODO(), time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}

	// label `__name__` will be omitted and get duplicated results here
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo_1", "job", "bar"))
	_, err = ar.exec(context.TODO(), time.Now(), 0)
	if !errors.Is(err, errDuplicate) {
		t.Fatalf("expected to have %s error; got %s", errDuplicate, err)
	}

	fq.Reset()

	expErr := "connection reset by peer"
	fq.SetErr(errors.New(expErr))
	_, err = ar.exec(context.TODO(), time.Now(), 0)
	if err == nil {
		t.Fatalf("expected to get err; got nil")
	}
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to get err %q; got %q instead", expErr, err)
	}
}

func TestAlertingRuleLimit_Failure(t *testing.T) {
	f := func(limit int, errStrExpected string) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		ar := newTestAlertingRule("test", 0)
		ar.Labels = map[string]string{"job": "test"}
		ar.q = fq
		ar.For = time.Minute

		fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
		fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "bar", "job"))

		timestamp := time.Now()
		_, err := ar.exec(context.TODO(), timestamp, limit)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, errStrExpected) {
			t.Fatalf("missing %q in error %q", errStrExpected, errStr)
		}
		fq.Reset()
	}

	f(1, "exec exceeded limit of 1 with 2 alerts")
}

func TestAlertingRuleLimit_Success(t *testing.T) {
	f := func(limit int) {
		t.Helper()

		fq := &datasource.FakeQuerier{}
		ar := newTestAlertingRule("test", 0)
		ar.Labels = map[string]string{"job": "test"}
		ar.q = fq
		ar.For = time.Minute

		fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))
		fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "bar", "job"))

		timestamp := time.Now()
		_, err := ar.exec(context.TODO(), timestamp, limit)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fq.Reset()
	}

	f(0)
	f(-1)
	f(4)
}

func TestAlertingRule_Template(t *testing.T) {
	f := func(rule *AlertingRule, metrics []datasource.Metric, alertsExpected map[uint64]*notifier.Alert) {
		t.Helper()

		fakeGroup := Group{
			Name: "TestRule_Exec",
		}
		fq := &datasource.FakeQuerier{}
		rule.GroupID = fakeGroup.GetID()
		rule.q = fq
		rule.state = &ruleState{
			entries: make([]StateEntry, 10),
		}
		fq.Add(metrics...)

		if _, err := rule.exec(context.TODO(), time.Now(), 0); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		for hash, expAlert := range alertsExpected {
			gotAlert := rule.alerts[hash]
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
	}

	f(&AlertingRule{
		Name: "common",
		Labels: map[string]string{
			"region": "east",
		},
		Annotations: map[string]string{
			"summary": `{{ $labels.alertname }}: Too high connection number for "{{ $labels.instance }}"`,
		},
		alerts: make(map[uint64]*notifier.Alert),
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 1, "instance", "foo"),
		metricWithValueAndLabels(t, 1, "instance", "bar"),
	}, map[uint64]*notifier.Alert{
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
	})

	f(&AlertingRule{
		Name: "override label",
		Labels: map[string]string{
			"instance": "{{ $labels.instance }}",
		},
		Annotations: map[string]string{
			"summary":     `{{ $labels.__name__ }}: Too high connection number for "{{ $labels.instance }}"`,
			"description": `{{ $labels.alertname}}: It is {{ $value }} connections for "{{ $labels.instance }}"`,
		},
		alerts: make(map[uint64]*notifier.Alert),
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 2, "__name__", "first", "instance", "foo", alertNameLabel, "override"),
		metricWithValueAndLabels(t, 10, "__name__", "second", "instance", "bar", alertNameLabel, "override"),
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{alertNameLabel: "override label", "exported_alertname": "override", "instance": "foo"}): {
			Labels: map[string]string{
				alertNameLabel:       "override label",
				"exported_alertname": "override",
				"instance":           "foo",
			},
			Annotations: map[string]string{
				"summary":     `first: Too high connection number for "foo"`,
				"description": `override: It is 2 connections for "foo"`,
			},
		},
		hash(map[string]string{alertNameLabel: "override label", "exported_alertname": "override", "instance": "bar"}): {
			Labels: map[string]string{
				alertNameLabel:       "override label",
				"exported_alertname": "override",
				"instance":           "bar",
			},
			Annotations: map[string]string{
				"summary":     `second: Too high connection number for "bar"`,
				"description": `override: It is 10 connections for "bar"`,
			},
		},
	})

	f(&AlertingRule{
		Name:      "OriginLabels",
		GroupName: "Testing",
		Labels: map[string]string{
			"instance": "{{ $labels.instance }}",
		},
		Annotations: map[string]string{
			"summary": `Alert "{{ $labels.alertname }}({{ $labels.alertgroup }})" for instance {{ $labels.instance }}`,
		},
		alerts: make(map[uint64]*notifier.Alert),
	}, []datasource.Metric{
		metricWithValueAndLabels(t, 1,
			alertNameLabel, "originAlertname",
			alertGroupNameLabel, "originGroupname",
			"instance", "foo"),
	}, map[uint64]*notifier.Alert{
		hash(map[string]string{
			alertNameLabel:        "OriginLabels",
			"exported_alertname":  "originAlertname",
			alertGroupNameLabel:   "Testing",
			"exported_alertgroup": "originGroupname",
			"instance":            "foo",
		}): {
			Labels: map[string]string{
				alertNameLabel:        "OriginLabels",
				"exported_alertname":  "originAlertname",
				alertGroupNameLabel:   "Testing",
				"exported_alertgroup": "originGroupname",
				"instance":            "foo",
			},
			Annotations: map[string]string{
				"summary": `Alert "originAlertname(originGroupname)" for instance foo`,
			},
		},
	})
}

func TestAlertsToSend(t *testing.T) {
	f := func(alerts, expAlerts []*notifier.Alert, resolveDuration, resendDelay time.Duration) {
		t.Helper()

		ar := &AlertingRule{alerts: make(map[uint64]*notifier.Alert)}
		for i, a := range alerts {
			ar.alerts[uint64(i)] = a
		}
		gotAlerts := ar.alertsToSend(resolveDuration, resendDelay)
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
			if got.Name != exp.Name {
				t.Fatalf("expected Name to be %v; got %v", exp.Name, got.Name)
			}
		}
	}

	ts := time.Now()

	// check if firing alerts need to be sent with non-zero resendDelay
	f([]*notifier.Alert{
		{Name: "a", State: notifier.StateFiring, Start: ts},
		// no need to resend firing
		{Name: "b", State: notifier.StateFiring, Start: ts, LastSent: ts.Add(-30 * time.Second), End: ts.Add(5 * time.Minute)},
		// last message is for resolved, send firing message this time
		{Name: "c", State: notifier.StateFiring, Start: ts, LastSent: ts.Add(-30 * time.Second), End: ts.Add(-1 * time.Minute)},
		// resend firing
		{Name: "d", State: notifier.StateFiring, Start: ts, LastSent: ts.Add(-1 * time.Minute)},
	},
		[]*notifier.Alert{{Name: "a"}, {Name: "c"}, {Name: "d"}},
		5*time.Minute, time.Minute,
	)

	// check if resolved alerts need to be sent with non-zero resendDelay
	f([]*notifier.Alert{
		{Name: "a", State: notifier.StateInactive, ResolvedAt: ts, LastSent: ts.Add(-30 * time.Second)},
		// no need to resend resolved
		{Name: "b", State: notifier.StateInactive, ResolvedAt: ts, LastSent: ts},
		// resend resolved
		{Name: "c", State: notifier.StateInactive, ResolvedAt: ts.Add(-1 * time.Minute), LastSent: ts.Add(-1 * time.Minute)},
	},
		[]*notifier.Alert{{Name: "a"}, {Name: "c"}},
		5*time.Minute, time.Minute,
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
		state:        &ruleState{entries: make([]StateEntry, 10)},
		metrics:      getTestAlertingRuleMetrics(name),
	}
	return &rule
}

func getTestAlertingRuleMetrics(name string) *alertingRuleMetrics {
	m := &alertingRuleMetrics{}
	m.errors = vmalertutil.NewCounter(metrics.NewSet(), fmt.Sprintf(`vmalert_alerting_rules_errors_total{alertname=%q}`, name))
	return m
}

func newTestAlertingRuleWithCustomFields(name string, waitFor, evalInterval, keepFiringFor time.Duration, annotation map[string]string) *AlertingRule {
	rule := newTestAlertingRule(name, waitFor)
	if evalInterval != 0 {
		rule.EvalInterval = evalInterval
	}
	rule.KeepFiringFor = keepFiringFor
	rule.Annotations = annotation
	return rule
}

func TestAlertingRule_ToLabels(t *testing.T) {
	metric := datasource.Metric{
		Labels: []prompb.Label{
			{Name: "instance", Value: "0.0.0.0:8800"},
			{Name: "group", Value: "vmalert"},
			{Name: "alertname", Value: "ConfigurationReloadFailure"},
		},
		Values:     []float64{1},
		Timestamps: []int64{time.Now().UnixNano()},
	}

	ar := &AlertingRule{
		Labels: map[string]string{
			"instance":      "override", // this should override instance with new value
			"group":         "vmalert",  // this shouldn't have effect since value in metric is equal
			"invalid_label": "{{ .Values.mustRuntimeFail }}",
			"empty_label":   "", // this should be dropped
		},
		Expr:      "sum(vmalert_alerting_rules_error) by(instance, group, alertname) > 0",
		Name:      "AlertingRulesError",
		GroupName: "vmalert",
	}

	expectedOriginLabels := map[string]string{
		"instance":      "0.0.0.0:8800",
		"group":         "vmalert",
		"alertname":     "ConfigurationReloadFailure",
		"alertgroup":    "vmalert",
		"invalid_label": `error evaluating template: template: :1:268: executing "" at <.Values.mustRuntimeFail>: can't evaluate field Values in type notifier.tplData`,
	}

	expectedProcessedLabels := map[string]string{
		"instance":           "override",
		"exported_instance":  "0.0.0.0:8800",
		"alertname":          "AlertingRulesError",
		"exported_alertname": "ConfigurationReloadFailure",
		"group":              "vmalert",
		"alertgroup":         "vmalert",
		"invalid_label":      `error evaluating template: template: :1:268: executing "" at <.Values.mustRuntimeFail>: can't evaluate field Values in type notifier.tplData`,
	}

	ls, err := ar.toLabels(metric, nil)
	if err == nil || !strings.Contains(err.Error(), "error evaluating template") {
		t.Fatalf("unexpected error %q", err.Error())
	}

	if !reflect.DeepEqual(ls.origin, expectedOriginLabels) {
		t.Fatalf("origin labels mismatch, got: %v, want: %v", ls.origin, expectedOriginLabels)
	}

	if !reflect.DeepEqual(ls.processed, expectedProcessedLabels) {
		t.Fatalf("processed labels mismatch, got: %v, want: %v", ls.processed, expectedProcessedLabels)
	}
}

func TestAlertingRuleExec_Partial(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 10, "__name__", "bar"))
	fq.SetPartialResponse(true)

	ar := newTestAlertingRule("test", 0)
	ar.Debug = true
	ar.Labels = map[string]string{"job": "test"}
	ar.q = fq
	ar.For = time.Second

	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))

	ts := time.Now()
	_, err := ar.exec(context.TODO(), ts, 0)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestAlertingRule_QueryTemplateInLabels(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	fakeGroup := Group{
		Name: "TestQueryTemplateInLabels",
	}

	ar := &AlertingRule{
		Name: "test_alert",
		Labels: map[string]string{
			"suppress_for_mass_alert": `{{ if (printf "ALERTS{alertname='SomeAlert', alertstate='firing', device='%s'} == 1" $labels.device | query) }}true{{ else }}false{{ end }}`,
		},
		Annotations: map[string]string{
			"summary": "Test alert with query template in labels",
		},
		alerts: make(map[uint64]*notifier.Alert),
	}
	ar.GroupID = fakeGroup.GetID()
	ar.q = fq
	ar.state = &ruleState{
		entries: make([]StateEntry, 10),
	}

	// Add a metric that should trigger the alert
	fq.Add(metricWithValueAndLabels(t, 1, "device", "sda1"))

	ts := time.Now()
	_, err := ar.exec(context.TODO(), ts, 0)
	if err != nil {
		t.Fatalf("unexpected error with query template in labels: %s", err)
	}

	// Verify that the alert was created and the query template was executed
	if len(ar.alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(ar.alerts))
	}

	alert := ar.GetAlerts()[0]
	suppressLabel, exists := alert.Labels["suppress_for_mass_alert"]
	if !exists {
		t.Fatalf("expected 'suppress_for_mass_alert' label to exist")
	}
	// The query template should have been executed (even if it returns false due to mock data)
	if suppressLabel != "true" && suppressLabel != "false" {
		t.Fatalf("expected 'suppress_for_mass_alert' label to be 'true' or 'false', got '%s'", suppressLabel)
	}
}

// TestAlertingRule_ActiveAtPreservedInAnnotations ensures that the fix for
// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9543 is preserved
// while allowing query templates in labels (https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9783)
func TestAlertingRule_ActiveAtPreservedInAnnotations(t *testing.T) {
	// wrap into synctest because of time manipulations
	synctest.Test(t, func(t *testing.T) {
		fq := &datasource.FakeQuerier{}

		ar := &AlertingRule{
			Name: "TestActiveAtPreservation",
			Labels: map[string]string{
				"test_query_in_label": `{{ "static_value" }}`,
			},
			Annotations: map[string]string{
				"description": "Alert active since {{ $activeAt }}",
			},
			alerts: make(map[uint64]*notifier.Alert),
			q:      fq,
			state: &ruleState{
				entries: make([]StateEntry, 10),
			},
		}

		// Mock query result - return empty result to make suppress_for_mass_alert = false
		// (no need to add anything to fq for empty result)

		// Add a metric that should trigger the alert
		fq.Add(metricWithValueAndLabels(t, 1, "instance", "server1"))

		// First execution - creates new alert
		ts1 := time.Now()
		_, err := ar.exec(context.TODO(), ts1, 0)
		if err != nil {
			t.Fatalf("unexpected error on first exec: %s", err)
		}

		if len(ar.alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(ar.alerts))
		}

		firstAlert := ar.GetAlerts()[0]
		// Verify first execution: activeAt should be ts1 and annotation should reflect it
		if !firstAlert.ActiveAt.Equal(ts1) {
			t.Fatalf("expected activeAt to be %v, got %v", ts1, firstAlert.ActiveAt)
		}

		// Extract time from annotation (format will be like "Alert active since 2025-09-30 08:55:13.638551611 -0400 EDT m=+0.002928464")
		expectedTimeStr := ts1.Format("2006-01-02 15:04:05")
		if !strings.Contains(firstAlert.Annotations["description"], expectedTimeStr) {
			t.Fatalf("first exec annotation should contain time %s, got: %s", expectedTimeStr, firstAlert.Annotations["description"])
		}

		// Second execution - should preserve activeAt in annotation

		// Ensure different timestamp with different seconds
		// sleep is non-blocking thanks to synctest
		time.Sleep(2 * time.Second)
		ts2 := time.Now()
		_, err = ar.exec(context.TODO(), ts2, 0)
		if err != nil {
			t.Fatalf("unexpected error on second exec: %s", err)
		}

		// Get the alert again (should be the same alert)
		if len(ar.alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(ar.alerts))
		}
		secondAlert := ar.GetAlerts()[0]

		// Critical test: activeAt should still be ts1, not ts2
		if !secondAlert.ActiveAt.Equal(ts1) {
			t.Fatalf("activeAt should be preserved as %v, but got %v", ts1, secondAlert.ActiveAt)
		}

		// Critical test: annotation should still contain ts1 time, not ts2
		if !strings.Contains(secondAlert.Annotations["description"], expectedTimeStr) {
			t.Fatalf("second exec annotation should still contain original time %s, got: %s", expectedTimeStr, secondAlert.Annotations["description"])
		}

		// Additional verification: annotation should NOT contain ts2 time
		ts2TimeStr := ts2.Format("2006-01-02 15:04:05")
		if strings.Contains(secondAlert.Annotations["description"], ts2TimeStr) {
			t.Fatalf("annotation should NOT contain new eval time %s, got: %s", ts2TimeStr, secondAlert.Annotations["description"])
		}

		// Verify query template in labels still works (this would fail if query templates were broken)
		if firstAlert.Labels["test_query_in_label"] != "static_value" {
			t.Fatalf("expected test_query_in_label=static_value, got %s", firstAlert.Labels["test_query_in_label"])
		}
	})
}
