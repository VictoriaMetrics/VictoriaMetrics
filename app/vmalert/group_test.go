package main

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

func init() {
	// Disable rand sleep on group start during tests in order to speed up test execution.
	// Rand sleep is needed only in prod code.
	skipRandSleepOnGroupStart = true
}

func TestUpdateWith(t *testing.T) {
	testCases := []struct {
		name         string
		currentRules []config.Rule
		newRules     []config.Rule
	}{
		{
			"new rule",
			nil,
			[]config.Rule{{Alert: "bar"}},
		},
		{
			"update alerting rule",
			[]config.Rule{{
				Alert: "foo",
				Expr:  "up > 0",
				For:   utils.NewPromDuration(time.Second),
				Labels: map[string]string{
					"bar": "baz",
				},
				Annotations: map[string]string{
					"summary":     "{{ $value|humanize }}",
					"description": "{{$labels}}",
				},
			}},
			[]config.Rule{{
				Alert: "foo",
				Expr:  "up > 10",
				For:   utils.NewPromDuration(time.Second),
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
			[]config.Rule{{
				Record: "foo",
				Expr:   "max(up)",
				Labels: map[string]string{
					"bar": "baz",
				},
			}},
			[]config.Rule{{
				Record: "foo",
				Expr:   "min(up)",
				Labels: map[string]string{
					"baz": "bar",
				},
			}},
		},
		{
			"empty rule",
			[]config.Rule{{Alert: "foo"}, {Record: "bar"}},
			nil,
		},
		{
			"multiple rules",
			[]config.Rule{
				{Alert: "bar"},
				{Alert: "baz"},
				{Alert: "foo"},
			},
			[]config.Rule{
				{Alert: "baz"},
				{Record: "foo"},
			},
		},
		{
			"replace rule",
			[]config.Rule{{Alert: "foo1"}},
			[]config.Rule{{Alert: "foo2"}},
		},
		{
			"replace multiple rules",
			[]config.Rule{
				{Alert: "foo1"},
				{Record: "foo2"},
				{Alert: "foo3"},
			},
			[]config.Rule{
				{Alert: "foo3"},
				{Alert: "foo4"},
				{Record: "foo5"},
			},
		},
		{
			"update datasource type",
			[]config.Rule{
				{Alert: "foo1", Type: datasource.NewPrometheusType()},
				{Alert: "foo3", Type: datasource.NewGraphiteType()},
			},
			[]config.Rule{
				{Alert: "foo1", Type: datasource.NewGraphiteType()},
				{Alert: "foo10", Type: datasource.NewPrometheusType()},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &Group{Name: "test"}
			qb := &fakeQuerier{}
			for _, r := range tc.currentRules {
				r.ID = config.HashRule(r)
				g.Rules = append(g.Rules, g.newRule(qb, r))
			}

			ng := &Group{Name: "test"}
			for _, r := range tc.newRules {
				r.ID = config.HashRule(r)
				ng.Rules = append(ng.Rules, ng.newRule(qb, r))
			}

			err := g.updateWith(ng)
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
			sort.Slice(ng.Rules, func(i, j int) bool {
				return ng.Rules[i].ID() < ng.Rules[j].ID()
			})
			for i, r := range g.Rules {
				got, want := r, ng.Rules[i]
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
	groups, err := config.Parse([]string{"config/testdata/rules1-good.rules"}, true, true)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}
	const evalInterval = time.Millisecond
	fs := &fakeQuerier{}
	fn := &fakeNotifier{}

	g := newGroup(groups[0], fs, evalInterval, map[string]string{"cluster": "east-1"})
	g.Concurrency = 2

	const inst1, inst2, job = "foo", "bar", "baz"
	m1 := metricWithLabels(t, "instance", inst1, "job", job)
	m2 := metricWithLabels(t, "instance", inst2, "job", job)

	r := g.Rules[0].(*AlertingRule)
	alert1, err := r.newAlert(m1, time.Now(), nil)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert1.State = notifier.StateFiring
	// add external label
	alert1.Labels["cluster"] = "east-1"
	// add rule labels - see config/testdata/rules1-good.rules
	alert1.Labels["label"] = "bar"
	alert1.Labels["host"] = inst1
	alert1.ID = hash(m1)

	alert2, err := r.newAlert(m2, time.Now(), nil)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert2.State = notifier.StateFiring
	// add external label
	alert2.Labels["cluster"] = "east-1"
	// add rule labels - see config/testdata/rules1-good.rules
	alert2.Labels["label"] = "bar"
	alert2.Labels["host"] = inst2
	alert2.ID = hash(m2)

	finished := make(chan struct{})
	fs.add(m1)
	fs.add(m2)
	go func() {
		g.start(context.Background(), []notifier.Notifier{fn}, nil)
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
