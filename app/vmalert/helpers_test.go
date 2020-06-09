package main

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

type fakeQuerier struct {
	sync.Mutex
	metrics []datasource.Metric
	err     error
}

func (fq *fakeQuerier) setErr(err error) {
	fq.Lock()
	fq.err = err
	fq.Unlock()
}

func (fq *fakeQuerier) reset() {
	fq.Lock()
	fq.err = nil
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
	defer fq.Unlock()
	if fq.err != nil {
		return nil, fq.err
	}
	cp := make([]datasource.Metric, len(fq.metrics))
	copy(cp, fq.metrics)
	return cp, nil
}

type fakeNotifier struct {
	sync.Mutex
	alerts []notifier.Alert
}

func (fn *fakeNotifier) Send(_ context.Context, alerts []notifier.Alert) error {
	fn.Lock()
	defer fn.Unlock()
	fn.alerts = alerts
	return nil
}

func (fn *fakeNotifier) getAlerts() []notifier.Alert {
	fn.Lock()
	defer fn.Unlock()
	return fn.alerts
}

func metricWithValueAndLabels(t *testing.T, value float64, labels ...string) datasource.Metric {
	t.Helper()
	m := metricWithLabels(t, labels...)
	m.Value = value
	return m
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

func compareGroups(t *testing.T, a, b *Group) {
	t.Helper()
	if a.Name != b.Name {
		t.Fatalf("expected group name %q; got %q", a.Name, b.Name)
	}
	if a.File != b.File {
		t.Fatalf("expected group %q file name %q; got %q", a.Name, a.File, b.File)
	}
	if a.Interval != b.Interval {
		t.Fatalf("expected group %q interval %v; got %v", a.Name, a.Interval, b.Interval)
	}
	if len(a.Rules) != len(b.Rules) {
		t.Fatalf("expected group %s to have %d rules; got: %d",
			a.Name, len(a.Rules), len(b.Rules))
	}
	for i, r := range a.Rules {
		got, want := r, b.Rules[i]
		if a.ID() != b.ID() {
			t.Fatalf("expected to have rule %q; got %q", want.ID(), got.ID())
		}
		if err := compareRules(t, want, got); err != nil {
			t.Fatalf("comparsion error: %s", err)
		}
	}
}

func compareRules(t *testing.T, a, b Rule) error {
	t.Helper()
	switch v := a.(type) {
	case *AlertingRule:
		br, ok := b.(*AlertingRule)
		if !ok {
			return fmt.Errorf("rule %q supposed to be of type AlertingRule", b.ID())
		}
		return compareAlertingRules(t, v, br)
	case *RecordingRule:
		br, ok := b.(*RecordingRule)
		if !ok {
			return fmt.Errorf("rule %q supposed to be of type RecordingRule", b.ID())
		}
		return compareRecordingRules(t, v, br)
	default:
		return fmt.Errorf("unexpected rule type received %T", a)
	}
}

func compareRecordingRules(t *testing.T, a, b *RecordingRule) error {
	t.Helper()
	if a.Expr != b.Expr {
		return fmt.Errorf("expected to have expression %q; got %q", a.Expr, b.Expr)
	}
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return fmt.Errorf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
	}
	return nil
}

func compareAlertingRules(t *testing.T, a, b *AlertingRule) error {
	t.Helper()
	if a.Expr != b.Expr {
		return fmt.Errorf("expected to have expression %q; got %q", a.Expr, b.Expr)
	}
	if a.For != b.For {
		return fmt.Errorf("expected to have for %q; got %q", a.For, b.For)
	}
	if !reflect.DeepEqual(a.Annotations, b.Annotations) {
		return fmt.Errorf("expected to have annotations %#v; got %#v", a.Annotations, b.Annotations)
	}
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return fmt.Errorf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
	}
	return nil
}

func compareTimeSeries(t *testing.T, a, b []prompbmarshal.TimeSeries) error {
	t.Helper()
	if len(a) != len(b) {
		return fmt.Errorf("expected number of timeseries %d; got %d", len(a), len(b))
	}
	for i := range a {
		expTS, gotTS := a[i], b[i]
		if len(expTS.Samples) != len(gotTS.Samples) {
			return fmt.Errorf("expected number of samples %d; got %d", len(expTS.Samples), len(gotTS.Samples))
		}
		for i, exp := range expTS.Samples {
			got := gotTS.Samples[i]
			if got.Value != exp.Value {
				return fmt.Errorf("expected value %.2f; got %.2f", exp.Value, got.Value)
			}
			// timestamp validation isn't always correct for now.
			// this must be improved with time mock.
			/*if got.Timestamp != exp.Timestamp {
				return fmt.Errorf("expected timestamp %d; got %d", exp.Timestamp, got.Timestamp)
			}*/
		}
		if len(expTS.Labels) != len(gotTS.Labels) {
			return fmt.Errorf("expected number of labels %d; got %d", len(expTS.Labels), len(gotTS.Labels))
		}
		for i, exp := range expTS.Labels {
			got := gotTS.Labels[i]
			if got.Name != exp.Name {
				return fmt.Errorf("expected label name %q; got %q", exp.Name, got.Name)
			}
			if got.Value != exp.Value {
				return fmt.Errorf("expected label value %q; got %q", exp.Value, got.Value)
			}
		}
	}
	return nil
}

func compareAlerts(t *testing.T, as, bs []notifier.Alert) {
	t.Helper()
	if len(as) != len(bs) {
		t.Fatalf("expected to have length %d; got %d", len(as), len(bs))
	}
	sort.Slice(as, func(i, j int) bool {
		return as[i].ID < as[j].ID
	})
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].ID < bs[j].ID
	})
	for i := range as {
		a, b := as[i], bs[i]
		if a.Name != b.Name {
			t.Fatalf("expected t have Name %q; got %q", a.Name, b.Name)
		}
		if a.State != b.State {
			t.Fatalf("expected t have State %q; got %q", a.State, b.State)
		}
		if a.Value != b.Value {
			t.Fatalf("expected t have Value %f; got %f", a.Value, b.Value)
		}
		if !reflect.DeepEqual(a.Annotations, b.Annotations) {
			t.Fatalf("expected to have annotations %#v; got %#v", a.Annotations, b.Annotations)
		}
		if !reflect.DeepEqual(a.Labels, b.Labels) {
			t.Fatalf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
		}
	}
}
