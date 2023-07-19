package main

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
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
				For:   promutils.NewDuration(time.Second),
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
				For:   promutils.NewDuration(time.Second),
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
					t.Fatalf("comparison error: %s", err)
				}
			}
		})
	}
}

func TestGroupStart(t *testing.T) {
	// TODO: make parsing from string instead of file
	groups, err := config.Parse([]string{"config/testdata/rules/rules1-good.rules"}, notifier.ValidateTemplates, true)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}

	fs := &fakeQuerier{}
	fn := &fakeNotifier{}

	const evalInterval = time.Millisecond
	g := newGroup(groups[0], fs, evalInterval, map[string]string{"cluster": "east-1"})
	g.Concurrency = 2

	const inst1, inst2, job = "foo", "bar", "baz"
	m1 := metricWithLabels(t, "instance", inst1, "job", job)
	m2 := metricWithLabels(t, "instance", inst2, "job", job)

	r := g.Rules[0].(*AlertingRule)
	alert1, err := r.newAlert(m1, nil, time.Now(), nil)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert1.State = notifier.StateFiring
	// add external label
	alert1.Labels["cluster"] = "east-1"
	// add rule labels - see config/testdata/rules1-good.rules
	alert1.Labels["label"] = "bar"
	alert1.Labels["host"] = inst1
	// add service labels
	alert1.Labels[alertNameLabel] = alert1.Name
	alert1.Labels[alertGroupNameLabel] = g.Name
	alert1.ID = hash(alert1.Labels)

	alert2, err := r.newAlert(m2, nil, time.Now(), nil)
	if err != nil {
		t.Fatalf("faield to create alert: %s", err)
	}
	alert2.State = notifier.StateFiring
	// add external label
	alert2.Labels["cluster"] = "east-1"
	// add rule labels - see config/testdata/rules1-good.rules
	alert2.Labels["label"] = "bar"
	alert2.Labels["host"] = inst2
	// add service labels
	alert2.Labels[alertNameLabel] = alert2.Name
	alert2.Labels[alertGroupNameLabel] = g.Name
	alert2.ID = hash(alert2.Labels)

	finished := make(chan struct{})
	fs.add(m1)
	fs.add(m2)
	go func() {
		g.start(context.Background(), func() []notifier.Notifier { return []notifier.Notifier{fn} }, nil, fs)
		close(finished)
	}()

	// wait for multiple evals
	time.Sleep(20 * evalInterval)

	gotAlerts := fn.getAlerts()
	expectedAlerts := []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	gotAlertsNum := fn.getCounter()
	if gotAlertsNum < len(expectedAlerts)*2 {
		t.Fatalf("expected to receive at least %d alerts; got %d instead",
			len(expectedAlerts)*2, gotAlertsNum)
	}

	// reset previous data
	fs.reset()
	// and set only one datapoint for response
	fs.add(m1)

	// wait for multiple evals
	time.Sleep(20 * evalInterval)

	gotAlerts = fn.getAlerts()
	alert2.State = notifier.StateInactive
	expectedAlerts = []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	g.close()
	<-finished
}

func TestResolveDuration(t *testing.T) {
	testCases := []struct {
		groupInterval time.Duration
		maxDuration   time.Duration
		resendDelay   time.Duration
		expected      time.Duration
	}{
		{time.Minute, 0, 0, 4 * time.Minute},
		{time.Minute, 0, 2 * time.Minute, 8 * time.Minute},
		{time.Minute, 4 * time.Minute, 4 * time.Minute, 4 * time.Minute},
		{2 * time.Minute, time.Minute, 2 * time.Minute, time.Minute},
		{time.Minute, 2 * time.Minute, 1 * time.Minute, 2 * time.Minute},
		{2 * time.Minute, 0, 1 * time.Minute, 8 * time.Minute},
		{0, 0, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v-%v-%v", tc.groupInterval, tc.expected, tc.maxDuration), func(t *testing.T) {
			got := getResolveDuration(tc.groupInterval, tc.resendDelay, tc.maxDuration)
			if got != tc.expected {
				t.Errorf("expected to have %v; got %v", tc.expected, got)
			}
		})
	}
}

func TestGetStaleSeries(t *testing.T) {
	ts := time.Now()
	e := &executor{
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}
	f := func(rule Rule, labels, expLabels [][]prompbmarshal.Label) {
		t.Helper()
		var tss []prompbmarshal.TimeSeries
		for _, l := range labels {
			tss = append(tss, newTimeSeriesPB([]float64{1}, []int64{ts.Unix()}, l))
		}
		staleS := e.getStaleSeries(rule, tss, ts)
		if staleS == nil && expLabels == nil {
			return
		}
		if len(staleS) != len(expLabels) {
			t.Fatalf("expected to get %d stale series, got %d",
				len(expLabels), len(staleS))
		}
		for i, exp := range expLabels {
			got := staleS[i]
			if !reflect.DeepEqual(exp, got.Labels) {
				t.Fatalf("expected to get labels: \n%v;\ngot instead: \n%v",
					exp, got.Labels)
			}
			if len(got.Samples) != 1 {
				t.Fatalf("expected to have 1 sample; got %d", len(got.Samples))
			}
			if !decimal.IsStaleNaN(got.Samples[0].Value) {
				t.Fatalf("expected sample value to be %v; got %v", decimal.StaleNaN, got.Samples[0].Value)
			}
		}
	}

	// warn: keep in mind, that executor holds the state, so sequence of f calls matters

	// single series
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "foo")},
		nil)
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "foo")},
		nil)
	f(&AlertingRule{RuleID: 1},
		nil,
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "foo")})
	f(&AlertingRule{RuleID: 1},
		nil,
		nil)

	// multiple series
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{
			toPromLabels(t, "__name__", "job:foo", "job", "foo"),
			toPromLabels(t, "__name__", "job:foo", "job", "bar"),
		},
		nil)
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "bar")},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "foo")})
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "bar")},
		nil)
	f(&AlertingRule{RuleID: 1},
		nil,
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "bar")})

	// multiple rules and series
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{
			toPromLabels(t, "__name__", "job:foo", "job", "foo"),
			toPromLabels(t, "__name__", "job:foo", "job", "bar"),
		},
		nil)
	f(&AlertingRule{RuleID: 2},
		[][]prompbmarshal.Label{
			toPromLabels(t, "__name__", "job:foo", "job", "foo"),
			toPromLabels(t, "__name__", "job:foo", "job", "bar"),
		},
		nil)
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "bar")},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "foo")})
	f(&AlertingRule{RuleID: 1},
		[][]prompbmarshal.Label{toPromLabels(t, "__name__", "job:foo", "job", "bar")},
		nil)
}

func TestPurgeStaleSeries(t *testing.T) {
	ts := time.Now()
	labels := toPromLabels(t, "__name__", "job:foo", "job", "foo")
	tss := []prompbmarshal.TimeSeries{newTimeSeriesPB([]float64{1}, []int64{ts.Unix()}, labels)}

	f := func(curRules, newRules, expStaleRules []Rule) {
		t.Helper()
		e := &executor{
			previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
		}
		// seed executor with series for
		// current rules
		for _, rule := range curRules {
			e.getStaleSeries(rule, tss, ts)
		}

		e.purgeStaleSeries(newRules)

		if len(e.previouslySentSeriesToRW) != len(expStaleRules) {
			t.Fatalf("expected to get %d stale series, got %d",
				len(expStaleRules), len(e.previouslySentSeriesToRW))
		}

		for _, exp := range expStaleRules {
			if _, ok := e.previouslySentSeriesToRW[exp.ID()]; !ok {
				t.Fatalf("expected to have rule %d; got nil instead", exp.ID())
			}
		}
	}

	f(nil, nil, nil)
	f(
		nil,
		[]Rule{&AlertingRule{RuleID: 1}},
		nil,
	)
	f(
		[]Rule{&AlertingRule{RuleID: 1}},
		nil,
		nil,
	)
	f(
		[]Rule{&AlertingRule{RuleID: 1}},
		[]Rule{&AlertingRule{RuleID: 2}},
		nil,
	)
	f(
		[]Rule{&AlertingRule{RuleID: 1}, &AlertingRule{RuleID: 2}},
		[]Rule{&AlertingRule{RuleID: 2}},
		[]Rule{&AlertingRule{RuleID: 2}},
	)
	f(
		[]Rule{&AlertingRule{RuleID: 1}, &AlertingRule{RuleID: 2}},
		[]Rule{&AlertingRule{RuleID: 1}, &AlertingRule{RuleID: 2}},
		[]Rule{&AlertingRule{RuleID: 1}, &AlertingRule{RuleID: 2}},
	)
}

func TestFaultyNotifier(t *testing.T) {
	fq := &fakeQuerier{}
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))

	r := newTestAlertingRule("instant", 0, false)
	r.q = fq

	fn := &fakeNotifier{}
	e := &executor{
		notifiers: func() []notifier.Notifier {
			return []notifier.Notifier{
				&faultyNotifier{},
				fn,
			}
		},
	}
	delay := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), delay)
	defer cancel()

	go func() {
		_ = e.exec(ctx, r, time.Now(), 0, 10)
	}()

	tn := time.Now()
	deadline := tn.Add(delay / 2)
	for {
		if fn.getCounter() > 0 {
			return
		}
		if tn.After(deadline) {
			break
		}
		tn = time.Now()
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("alive notifier didn't receive notification by %v", deadline)
}

func TestFaultyRW(t *testing.T) {
	fq := &fakeQuerier{}
	fq.add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))

	r := &RecordingRule{
		Name:  "test",
		state: newRuleState(10),
		q:     fq,
	}

	e := &executor{
		rw:                       &remotewrite.Client{},
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}

	err := e.exec(context.Background(), r, time.Now(), 0, 10)
	if err == nil {
		t.Fatalf("expected to get an error from faulty RW client, got nil instead")
	}
}

func TestCloseWithEvalInterruption(t *testing.T) {
	groups, err := config.Parse([]string{"config/testdata/rules/rules1-good.rules"}, notifier.ValidateTemplates, true)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}

	const delay = time.Second * 2
	fq := &fakeQuerierWithDelay{delay: delay}

	const evalInterval = time.Millisecond
	g := newGroup(groups[0], fq, evalInterval, nil)

	go g.start(context.Background(), nil, nil, nil)

	time.Sleep(evalInterval * 20)

	go func() {
		g.close()
	}()

	deadline := time.Tick(delay / 2)
	select {
	case <-deadline:
		t.Fatalf("deadline for close exceeded")
	case <-g.finishedCh:
	}
}
