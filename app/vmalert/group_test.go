package main

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestUpdateWith(t *testing.T) {
	testCases := []struct {
		name         string
		currentRules []Rule
		// rules must be sorted by ID
		newRules []Rule
	}{
		{
			"new rule",
			[]Rule{},
			[]Rule{&AlertingRule{Name: "bar"}},
		},
		{
			"update alerting rule",
			[]Rule{&AlertingRule{
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
			[]Rule{&AlertingRule{
				Name: "foo",
				Expr: "up > 10",
				For:  time.Second,
				Labels: map[string]string{
					"baz": "bar",
				},
				Annotations: map[string]string{
					"summary": "none",
				},
			}},
		},
		{
			"update recording rule",
			[]Rule{&RecordingRule{
				Name: "foo",
				Expr: "max(up)",
				Labels: map[string]string{
					"bar": "baz",
				},
			}},
			[]Rule{&RecordingRule{
				Name: "foo",
				Expr: "min(up)",
				Labels: map[string]string{
					"baz": "bar",
				},
			}},
		},
		{
			"empty rule",
			[]Rule{&AlertingRule{Name: "foo"}, &RecordingRule{Name: "bar"}},
			[]Rule{},
		},
		{
			"multiple rules",
			[]Rule{
				&AlertingRule{Name: "bar"},
				&AlertingRule{Name: "baz"},
				&RecordingRule{Name: "foo"},
			},
			[]Rule{
				&AlertingRule{Name: "baz"},
				&RecordingRule{Name: "foo"},
			},
		},
		{
			"replace rule",
			[]Rule{&AlertingRule{Name: "foo1"}},
			[]Rule{&AlertingRule{Name: "foo2"}},
		},
		{
			"replace multiple rules",
			[]Rule{
				&AlertingRule{Name: "foo1"},
				&RecordingRule{Name: "foo2"},
				&AlertingRule{Name: "foo3"},
			},
			[]Rule{
				&AlertingRule{Name: "foo3"},
				&AlertingRule{Name: "foo4"},
				&RecordingRule{Name: "foo5"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &Group{Rules: tc.currentRules}
			err := g.updateWith(&Group{Rules: tc.newRules})
			if err != nil {
				t.Fatal(err)
			}

			if len(g.Rules) != len(tc.newRules) {
				t.Fatalf("expected to have %d rules; got: %d",
					len(g.Rules), len(tc.newRules))
			}
			sort.Slice(g.Rules, func(i, j int) bool {
				return g.Rules[i].ID() < g.Rules[j].ID()
			})
			for i, r := range g.Rules {
				got, want := r, tc.newRules[i]
				if got.ID() != want.ID() {
					t.Fatalf("expected to have rule %q; got %q", want, got)
				}
				if err := compareRules(t, got, want); err != nil {
					t.Fatalf("comparsion error: %s", err)
				}
			}
		})
	}
}

func TestGroupStart(t *testing.T) {
	// TODO: make parsing from string instead of file
	groups, err := config.Parse([]string{"config/testdata/rules1-good.rules"}, true)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}
	const evalInterval = time.Millisecond
	g := newGroup(groups[0], evalInterval)

	fn := &fakeNotifier{}
	fs := &fakeQuerier{}

	const inst1, inst2, job = "foo", "bar", "baz"
	m1 := metricWithLabels(t, "instance", inst1, "job", job)
	m2 := metricWithLabels(t, "instance", inst2, "job", job)

	r := g.Rules[0].(*AlertingRule)
	alert1, err := r.newAlert(m1, time.Now())
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert1.State = notifier.StateFiring
	alert1.ID = hash(m1)

	alert2, err := r.newAlert(m2, time.Now())
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
	time.Sleep(20 * evalInterval)

	gotAlerts := fn.getAlerts()
	expectedAlerts := []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	// reset previous data
	fs.reset()
	// and set only one datapoint for response
	fs.add(m1)

	// wait for multiple evals
	time.Sleep(20 * evalInterval)

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
