package main

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

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

func (fq *fakeQuerier) BuildWithParams(_ datasource.QuerierParams) datasource.Querier {
	return fq
}

func (fq *fakeQuerier) QueryRange(ctx context.Context, q string, _, _ time.Time) (datasource.Result, error) {
	req, _, err := fq.Query(ctx, q, time.Now())
	return req, err
}

func (fq *fakeQuerier) Query(_ context.Context, _ string, _ time.Time) (datasource.Result, *http.Request, error) {
	fq.Lock()
	defer fq.Unlock()
	if fq.err != nil {
		return datasource.Result{}, nil, fq.err
	}
	cp := make([]datasource.Metric, len(fq.metrics))
	copy(cp, fq.metrics)
	req, _ := http.NewRequest(http.MethodPost, "foo.com", nil)
	return datasource.Result{Data: cp}, req, nil
}

type fakeQuerierWithRegistry struct {
	sync.Mutex
	registry map[string][]datasource.Metric
}

func (fqr *fakeQuerierWithRegistry) set(key string, metrics ...datasource.Metric) {
	fqr.Lock()
	if fqr.registry == nil {
		fqr.registry = make(map[string][]datasource.Metric)
	}
	fqr.registry[key] = metrics
	fqr.Unlock()
}

func (fqr *fakeQuerierWithRegistry) reset() {
	fqr.Lock()
	fqr.registry = nil
	fqr.Unlock()
}

func (fqr *fakeQuerierWithRegistry) BuildWithParams(_ datasource.QuerierParams) datasource.Querier {
	return fqr
}

func (fqr *fakeQuerierWithRegistry) QueryRange(ctx context.Context, q string, _, _ time.Time) (datasource.Result, error) {
	req, _, err := fqr.Query(ctx, q, time.Now())
	return req, err
}

func (fqr *fakeQuerierWithRegistry) Query(_ context.Context, expr string, _ time.Time) (datasource.Result, *http.Request, error) {
	fqr.Lock()
	defer fqr.Unlock()

	req, _ := http.NewRequest(http.MethodPost, "foo.com", nil)
	metrics, ok := fqr.registry[expr]
	if !ok {
		return datasource.Result{}, req, nil
	}
	cp := make([]datasource.Metric, len(metrics))
	copy(cp, metrics)
	return datasource.Result{Data: cp}, req, nil
}

type fakeQuerierWithDelay struct {
	fakeQuerier
	delay time.Duration
}

func (fqd *fakeQuerierWithDelay) Query(ctx context.Context, expr string, ts time.Time) (datasource.Result, *http.Request, error) {
	timer := time.NewTimer(fqd.delay)
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	return fqd.fakeQuerier.Query(ctx, expr, ts)
}

func (fqd *fakeQuerierWithDelay) BuildWithParams(_ datasource.QuerierParams) datasource.Querier {
	return fqd
}

type fakeNotifier struct {
	sync.Mutex
	alerts []notifier.Alert
	// records number of received alerts in total
	counter int
}

func (*fakeNotifier) Close()       {}
func (*fakeNotifier) Addr() string { return "" }
func (fn *fakeNotifier) Send(_ context.Context, alerts []notifier.Alert, _ map[string]string) error {
	fn.Lock()
	defer fn.Unlock()
	fn.counter += len(alerts)
	fn.alerts = alerts
	return nil
}

func (fn *fakeNotifier) getCounter() int {
	fn.Lock()
	defer fn.Unlock()
	return fn.counter
}

func (fn *fakeNotifier) getAlerts() []notifier.Alert {
	fn.Lock()
	defer fn.Unlock()
	return fn.alerts
}

type faultyNotifier struct {
	fakeNotifier
}

func (fn *faultyNotifier) Send(ctx context.Context, _ []notifier.Alert, _ map[string]string) error {
	d, ok := ctx.Deadline()
	if ok {
		time.Sleep(time.Until(d))
	}
	return fmt.Errorf("send failed")
}

func metricWithValueAndLabels(t *testing.T, value float64, labels ...string) datasource.Metric {
	return metricWithValuesAndLabels(t, []float64{value}, labels...)
}

func metricWithValuesAndLabels(t *testing.T, values []float64, labels ...string) datasource.Metric {
	t.Helper()
	m := metricWithLabels(t, labels...)
	m.Values = values
	for i := range values {
		m.Timestamps = append(m.Timestamps, int64(i))
	}
	return m
}

func metricWithLabels(t *testing.T, labels ...string) datasource.Metric {
	t.Helper()
	if len(labels) == 0 || len(labels)%2 != 0 {
		t.Fatalf("expected to get even number of labels")
	}
	m := datasource.Metric{Values: []float64{1}, Timestamps: []int64{1}}
	for i := 0; i < len(labels); i += 2 {
		m.Labels = append(m.Labels, datasource.Label{
			Name:  labels[i],
			Value: labels[i+1],
		})
	}
	return m
}

func toPromLabels(t *testing.T, labels ...string) []prompbmarshal.Label {
	t.Helper()
	if len(labels) == 0 || len(labels)%2 != 0 {
		t.Fatalf("expected to get even number of labels")
	}
	var ls []prompbmarshal.Label
	for i := 0; i < len(labels); i += 2 {
		ls = append(ls, prompbmarshal.Label{
			Name:  labels[i],
			Value: labels[i+1],
		})
	}
	return ls
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
			t.Fatalf("comparison error: %s", err)
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
	if a.KeepFiringFor != b.KeepFiringFor {
		return fmt.Errorf("expected to have KeepFiringFor %q; got %q", a.KeepFiringFor, b.KeepFiringFor)
	}
	if !reflect.DeepEqual(a.Annotations, b.Annotations) {
		return fmt.Errorf("expected to have annotations %#v; got %#v", a.Annotations, b.Annotations)
	}
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return fmt.Errorf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
	}
	if a.Type.String() != b.Type.String() {
		return fmt.Errorf("expected to have Type %#v; got %#v", a.Type.String(), b.Type.String())
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
			return fmt.Errorf("expected number of labels %d (%v); got %d (%v)",
				len(expTS.Labels), expTS.Labels, len(gotTS.Labels), gotTS.Labels)
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
