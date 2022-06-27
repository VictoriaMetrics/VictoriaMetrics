package main

import (
	"context"
	"errors"
	"reflect"
	"sort"
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
		expAlerts []testAlert
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
			[]testAlert{
				{alert: &notifier.Alert{State: notifier.StateFiring}},
			},
		},
		{
			newTestAlertingRule("single-firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
			},
		},
		{
			newTestAlertingRule("single-firing=>inactive=>firing", 0),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{},
				{metricWithLabels(t, "name", "foo")},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
			},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateFiring}},
				{labels: []string{"name", "foo2"}, alert: &notifier.Alert{State: notifier.StateFiring}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
				{labels: []string{"name", "foo1"}, alert: &notifier.Alert{State: notifier.StateInactive}},
				{labels: []string{"name", "foo2"}, alert: &notifier.Alert{State: notifier.StateFiring}},
			},
		},
		{
			newTestAlertingRule("for-pending", time.Minute),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}},
			},
		},
		{
			newTestAlertingRule("for-fired", defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
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
			nil,
		},
		{
			newTestAlertingRule("for-pending=>firing=>inactive", defaultStep),
			[][]datasource.Metric{
				{metricWithLabels(t, "name", "foo")},
				{metricWithLabels(t, "name", "foo")},
				// empty step to reset pending alerts
				{},
			},
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateInactive}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StatePending}},
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
			[]testAlert{
				{labels: []string{"name", "foo"}, alert: &notifier.Alert{State: notifier.StateFiring}},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.q = fq
			tc.rule.GroupID = fakeGroup.ID()
			for _, step := range tc.steps {
				fq.reset()
				fq.add(step...)
				if _, err := tc.rule.Exec(context.TODO(), time.Now(), 0); err != nil {
					t.Fatalf("unexpected err: %s", err)
				}
				// artificial delay between applying steps
				time.Sleep(defaultStep)
			}
			if len(tc.rule.alerts) != len(tc.expAlerts) {
				t.Fatalf("expected %d alerts; got %d", len(tc.expAlerts), len(tc.rule.alerts))
			}
			expAlerts := make(map[uint64]*notifier.Alert)
			for _, ta := range tc.expAlerts {
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
					t.Fatalf("expected to have key %d", key)
				}
				if got.State != exp.State {
					t.Fatalf("expected state %d; got %d", exp.State, got.State)
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
				),
			},
			map[uint64]*notifier.Alert{
				hash(nil): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(time.Hour)},
			},
		},
		{
			newTestRuleWithLabels("metric labels"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					alertNameLabel, "metric labels",
					alertGroupNameLabel, "groupID",
					"foo", "bar",
					"namespace", "baz",
				),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{
					alertNameLabel:      "metric labels",
					alertGroupNameLabel: "groupID",
					"foo":               "bar",
					"namespace":         "baz",
				}): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(time.Hour)},
			},
		},
		{
			newTestRuleWithLabels("rule labels", "source", "vm"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, float64(time.Now().Truncate(time.Hour).Unix()),
					"__name__", alertForStateMetricName,
					"foo", "bar",
					"namespace", "baz",
					// extra labels set by rule
					"source", "vm",
				),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{
					"foo":       "bar",
					"namespace": "baz",
					"source":    "vm",
				}): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(time.Hour)},
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
				hash(map[string]string{"host": "localhost-1"}): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(time.Hour)},
				hash(map[string]string{"host": "localhost-2"}): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(2 * time.Hour)},
				hash(map[string]string{"host": "localhost-3"}): {State: notifier.StatePending,
					ActiveAt: time.Now().Truncate(3 * time.Hour)},
			},
		},
	}
	fakeGroup := Group{Name: "TestRule_Exec"}
	for _, tc := range testCases {
		t.Run(tc.rule.Name, func(t *testing.T) {
			fq := &fakeQuerier{}
			tc.rule.GroupID = fakeGroup.ID()
			tc.rule.q = fq
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
				if got.ActiveAt != exp.ActiveAt {
					t.Fatalf("expected ActiveAt %v; got %v", exp.ActiveAt, got.ActiveAt)
				}
			}
		})
	}
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
			newTestRuleWithLabels("common", "region", "east"),
			[]datasource.Metric{
				metricWithValueAndLabels(t, 1, "instance", "foo"),
				metricWithValueAndLabels(t, 1, "instance", "bar"),
			},
			map[uint64]*notifier.Alert{
				hash(map[string]string{alertNameLabel: "common", "region": "east", "instance": "foo"}): {
					Annotations: map[string]string{},
					Labels: map[string]string{
						alertNameLabel: "common",
						"region":       "east",
						"instance":     "foo",
					},
				},
				hash(map[string]string{alertNameLabel: "common", "region": "east", "instance": "bar"}): {
					Annotations: map[string]string{},
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
			fq.add(tc.metrics...)
			if _, err := tc.rule.Exec(context.TODO(), time.Now(), 0); err != nil {
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
	return &AlertingRule{Name: name, alerts: make(map[uint64]*notifier.Alert), For: waitFor, EvalInterval: waitFor}
}
