package main

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestUpdateWith(t *testing.T) {
	testCases := []struct {
		name         string
		currentRules []*Rule
		// rules must be sorted by name
		newRules []*Rule
	}{
		{
			"new rule",
			[]*Rule{},
			[]*Rule{{Name: "bar"}},
		},
		{
			"update rule",
			[]*Rule{{
				Name: "foo",
				Expr: "up > 0",
				For:  time.Second,
				Labels: map[string]string{
					"bar": "baz",
				},
				Annotations: map[string]string{
					"summary":     "{{ $value|humanize }}",
					"description": "{{$labels}}",
				},
			}},
			[]*Rule{{
				Name: "bar",
				Expr: "up > 10",
				Labels: map[string]string{
					"baz": "bar",
				},
				Annotations: map[string]string{
					"summary": "none",
				},
			}},
		},
		{
			"empty rule",
			[]*Rule{{Name: "foo"}},
			[]*Rule{},
		},
		{
			"multiple rules",
			[]*Rule{{Name: "bar"}, {Name: "baz"}, {Name: "foo"}},
			[]*Rule{{Name: "baz"}, {Name: "foo"}},
		},
		{
			"replace rule",
			[]*Rule{{Name: "foo1"}},
			[]*Rule{{Name: "foo2"}},
		},
		{
			"replace multiple rules",
			[]*Rule{{Name: "foo1"}, {Name: "foo2"}},
			[]*Rule{{Name: "foo3"}, {Name: "foo4"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &Group{Rules: tc.currentRules,Interval:tc.interval}
			g.updateWith(Group{Rules: tc.newRules})

			if len(g.Rules) != len(tc.newRules) {
				t.Fatalf("expected to have %d rules; got: %d",
					len(g.Rules), len(tc.newRules))
			}
			if g.Interval != tc.interval{
				t.Fatalf("expected group interval %v; got: %v",
					g.Interval, tc.interval)
			}
			sort.Slice(g.Rules, func(i, j int) bool {
				return g.Rules[i].Name < g.Rules[j].Name
			})
			for i, r := range g.Rules {
				got, want := r, tc.newRules[i]
				if got.Name != want.Name {
					t.Fatalf("expected to have rule %q; got %q", want.Name, got.Name)
				}
				if got.Expr != want.Expr {
					t.Fatalf("expected to have expression %q; got %q", want.Expr, got.Expr)
				}
				if got.For != want.For {
					t.Fatalf("expected to have for %q; got %q", want.For, got.For)
				}
				if !reflect.DeepEqual(got.Annotations, want.Annotations) {
					t.Fatalf("expected to have annotations %#v; got %#v", want.Annotations, got.Annotations)
				}
				if !reflect.DeepEqual(got.Labels, want.Labels) {
					t.Fatalf("expected to have labels %#v; got %#v", want.Labels, got.Labels)
				}
			}
		})
	}
}

func TestGroupStart(t *testing.T) {
	// TODO: make parsing from string instead of file
	groups, err := Parse([]string{"testdata/rules1-good.rules"}, true)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}
	g := groups[0]
	g.Interval = 1 * time.Millisecond

	fn := &fakeNotifier{}
	fs := &fakeQuerier{}

	const inst1, inst2, job = "foo", "bar", "baz"
	m1 := metricWithLabels(t, "instance", inst1, "job", job)
	m2 := metricWithLabels(t, "instance", inst2, "job", job)

	r := g.Rules[0]
	alert1, err := r.newAlert(m1)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert1.State = notifier.StateFiring
	alert1.ID = hash(m1)

	alert2, err := r.newAlert(m2)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert2.State = notifier.StateFiring
	alert2.ID = hash(m2)

	finished := make(chan struct{})
	fs.add(m1)
	fs.add(m2)
	go func() {
		g.start(context.Background(), fs, fn, nil)
		close(finished)
	}()

	// wait for multiple evals
	time.Sleep(20 * g.Interval)

	gotAlerts := fn.getAlerts()
	expectedAlerts := []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	// reset previous data
	fs.reset()
	// and set only one datapoint for response
	fs.add(m1)

	// wait for multiple evals
	time.Sleep(20 * g.Interval)

	gotAlerts = fn.getAlerts()
	expectedAlerts = []notifier.Alert{*alert1}
	compareAlerts(t, expectedAlerts, gotAlerts)

	g.close()
	<-finished
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
