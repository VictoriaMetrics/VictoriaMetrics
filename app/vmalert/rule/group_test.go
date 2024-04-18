package rule

import (
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func init() {
	// Disable rand sleep on group start during tests in order to speed up test execution.
	// Rand sleep is needed only in prod code.
	SkipRandSleepOnGroupStart = true
}

func TestMain(m *testing.M) {
	if err := templates.Load([]string{}, true); err != nil {
		fmt.Println("failed to load template for test")
		os.Exit(1)
	}
	os.Exit(m.Run())
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
			[]config.Rule{
				{
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
				},
				{
					Alert: "bar",
					Expr:  "up > 0",
					For:   promutils.NewDuration(time.Second),
					Labels: map[string]string{
						"bar": "baz",
					},
				},
			},
			[]config.Rule{
				{
					Alert: "foo",
					Expr:  "up > 10",
					For:   promutils.NewDuration(time.Second),
					Labels: map[string]string{
						"baz": "bar",
					},
					Annotations: map[string]string{
						"summary": "none",
					},
				},
				{
					Alert:         "bar",
					Expr:          "up > 0",
					For:           promutils.NewDuration(2 * time.Second),
					KeepFiringFor: promutils.NewDuration(time.Minute),
					Labels: map[string]string{
						"bar": "baz",
					},
				},
			},
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
			qb := &datasource.FakeQuerier{}
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
				if err := CompareRules(t, got, want); err != nil {
					t.Fatalf("comparison error: %s", err)
				}
			}
		})
	}
}

func TestGroupStart(t *testing.T) {
	const (
		rules = `
  - name: groupTest
    rules:
      - alert: VMRows
        for: 1ms
        expr: vm_rows > 0
        labels:
          label: bar
          host: "{{ $labels.instance }}"
        annotations:
          summary: "{{ $value }}"
`
	)
	var groups []config.Group
	err := yaml.Unmarshal([]byte(rules), &groups)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}

	fs := &datasource.FakeQuerier{}
	fn := &notifier.FakeNotifier{}

	const evalInterval = time.Millisecond
	g := NewGroup(groups[0], fs, evalInterval, map[string]string{"cluster": "east-1"})

	const inst1, inst2, job = "foo", "bar", "baz"
	m1 := metricWithLabels(t, "instance", inst1, "job", job)
	m2 := metricWithLabels(t, "instance", inst2, "job", job)

	r := g.Rules[0].(*AlertingRule)
	alert1 := r.newAlert(m1, time.Now(), nil, nil)
	alert1.State = notifier.StateFiring
	// add annotations
	alert1.Annotations["summary"] = "1"
	// add external label
	alert1.Labels["cluster"] = "east-1"
	// add labels from response
	alert1.Labels["job"] = job
	alert1.Labels["instance"] = inst1
	// add rule labels
	alert1.Labels["label"] = "bar"
	alert1.Labels["host"] = inst1
	// add service labels
	alert1.Labels[alertNameLabel] = alert1.Name
	alert1.Labels[alertGroupNameLabel] = g.Name
	alert1.ID = hash(alert1.Labels)

	alert2 := r.newAlert(m2, time.Now(), nil, nil)
	alert2.State = notifier.StateFiring
	// add annotations
	alert2.Annotations["summary"] = "1"
	// add external label
	alert2.Labels["cluster"] = "east-1"
	// add labels from response
	alert2.Labels["job"] = job
	alert2.Labels["instance"] = inst2
	// add rule labels
	alert2.Labels["label"] = "bar"
	alert2.Labels["host"] = inst2
	// add service labels
	alert2.Labels[alertNameLabel] = alert2.Name
	alert2.Labels[alertGroupNameLabel] = g.Name
	alert2.ID = hash(alert2.Labels)

	finished := make(chan struct{})
	fs.Add(m1)
	fs.Add(m2)
	go func() {
		g.Start(context.Background(), func() []notifier.Notifier { return []notifier.Notifier{fn} }, nil, fs)
		close(finished)
	}()

	waitForIterations := func(n int, interval time.Duration) {
		t.Helper()

		var cur uint64
		prev := g.metrics.iterationTotal.Get()
		for i := 0; ; i++ {
			if i > 40 {
				t.Fatalf("group wasn't able to perform %d evaluations during %d eval intervals", n, i)
			}
			cur = g.metrics.iterationTotal.Get()
			if int(cur-prev) >= n {
				return
			}
			time.Sleep(interval)
		}
	}

	// wait for multiple evaluation iterations
	waitForIterations(4, evalInterval)

	gotAlerts := fn.GetAlerts()
	expectedAlerts := []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	gotAlertsNum := fn.GetCounter()
	if gotAlertsNum < len(expectedAlerts)*2 {
		t.Fatalf("expected to receive at least %d alerts; got %d instead",
			len(expectedAlerts)*2, gotAlertsNum)
	}

	// reset previous data
	fs.Reset()
	// and set only one datapoint for response
	fs.Add(m1)

	// wait for multiple evaluation iterations
	waitForIterations(4, evalInterval)

	gotAlerts = fn.GetAlerts()
	alert2.State = notifier.StateInactive
	expectedAlerts = []notifier.Alert{*alert1, *alert2}
	compareAlerts(t, expectedAlerts, gotAlerts)

	g.Close()
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
	f := func(r Rule, labels, expLabels [][]prompbmarshal.Label) {
		t.Helper()
		var tss []prompbmarshal.TimeSeries
		for _, l := range labels {
			tss = append(tss, newTimeSeriesPB([]float64{1}, []int64{ts.Unix()}, l))
		}
		staleS := e.getStaleSeries(r, tss, ts)
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
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))

	r := newTestAlertingRule("instant", 0)
	r.q = fq

	fn := &notifier.FakeNotifier{}
	e := &executor{
		Notifiers: func() []notifier.Notifier {
			return []notifier.Notifier{
				&notifier.FaultyNotifier{},
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
		if fn.GetCounter() > 0 {
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
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 1, "__name__", "foo", "job", "bar"))

	r := &RecordingRule{
		Name:  "test",
		q:     fq,
		state: &ruleState{entries: make([]StateEntry, 10)},
	}

	e := &executor{
		Rw:                       &remotewrite.Client{},
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}

	err := e.exec(context.Background(), r, time.Now(), 0, 10)
	if err == nil {
		t.Fatalf("expected to get an error from faulty RW client, got nil instead")
	}
}

func TestCloseWithEvalInterruption(t *testing.T) {
	const (
		rules = `
  - name: groupTest
    rules:
      - alert: VMRows
        for: 1ms
        expr: vm_rows > 0
        labels:
          label: bar
          host: "{{ $labels.instance }}"
        annotations:
          summary: "{{ $value }}"
`
	)
	var groups []config.Group
	err := yaml.Unmarshal([]byte(rules), &groups)
	if err != nil {
		t.Fatalf("failed to parse rules: %s", err)
	}

	const delay = time.Second * 2
	fq := &datasource.FakeQuerierWithDelay{Delay: delay}

	const evalInterval = time.Millisecond
	g := NewGroup(groups[0], fq, evalInterval, nil)

	go g.Start(context.Background(), nil, nil, nil)

	time.Sleep(evalInterval * 20)

	go func() {
		g.Close()
	}()

	deadline := time.Tick(delay / 2)
	select {
	case <-deadline:
		t.Fatalf("deadline for close exceeded")
	case <-g.finishedCh:
	}
}

func TestGroupStartDelay(t *testing.T) {
	g := &Group{}
	// interval of 5min and key generate a static delay of 30s
	g.Interval = time.Minute * 5
	key := uint64(math.MaxUint64 / 10)

	f := func(atS, expS string) {
		t.Helper()
		at, err := time.Parse(time.RFC3339Nano, atS)
		if err != nil {
			t.Fatal(err)
		}
		expTS, err := time.Parse(time.RFC3339Nano, expS)
		if err != nil {
			t.Fatal(err)
		}
		delay := delayBeforeStart(at, key, g.Interval, g.EvalOffset)
		gotStart := at.Add(delay)
		if expTS != gotStart {
			t.Errorf("expected to get %v; got %v instead", expTS, gotStart)
		}
	}

	// test group without offset
	f("2023-01-01T00:00:00.000+00:00", "2023-01-01T00:00:30.000+00:00")
	f("2023-01-01T00:00:00.999+00:00", "2023-01-01T00:00:30.000+00:00")
	f("2023-01-01T00:00:29.000+00:00", "2023-01-01T00:00:30.000+00:00")
	f("2023-01-01T00:00:31.000+00:00", "2023-01-01T00:05:30.000+00:00")

	// test group with offset smaller than above fixed randSleep,
	// this way randSleep will always be enough
	offset := 20 * time.Second
	g.EvalOffset = &offset

	f("2023-01-01T00:00:00.000+00:00", "2023-01-01T00:00:30.000+00:00")
	f("2023-01-01T00:00:29.000+00:00", "2023-01-01T00:00:30.000+00:00")
	f("2023-01-01T00:00:31.000+00:00", "2023-01-01T00:05:30.000+00:00")

	// test group with offset bigger than above fixed randSleep,
	// this way offset will be added to delay
	offset = 3 * time.Minute
	g.EvalOffset = &offset

	f("2023-01-01T00:00:00.000+00:00", "2023-01-01T00:03:30.000+00:00")
	f("2023-01-01T00:00:29.000+00:00", "2023-01-01T00:03:30.000+00:00")
	f("2023-01-01T00:01:00.000+00:00", "2023-01-01T00:08:30.000+00:00")
	f("2023-01-01T00:03:30.000+00:00", "2023-01-01T00:08:30.000+00:00")
	f("2023-01-01T00:07:30.000+00:00", "2023-01-01T00:13:30.000+00:00")

	offset = 10 * time.Minute
	g.EvalOffset = &offset
	// interval of 1h and key generate a static delay of 6m
	g.Interval = time.Hour

	f("2023-01-01T00:00:00.000+00:00", "2023-01-01T00:16:00.000+00:00")
	f("2023-01-01T00:05:00.000+00:00", "2023-01-01T00:16:00.000+00:00")
	f("2023-01-01T00:30:00.000+00:00", "2023-01-01T01:16:00.000+00:00")
}

func TestGetPrometheusReqTimestamp(t *testing.T) {
	offset := 30 * time.Minute
	evalDelay := 1 * time.Minute
	disableAlign := false
	testCases := []struct {
		name            string
		g               *Group
		originTS, expTS string
	}{
		{
			"with query align + default evalDelay",
			&Group{
				Interval: time.Hour,
			},
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T11:00:00+00:00",
		},
		{
			"without query align + default evalDelay",
			&Group{
				Interval:      time.Hour,
				evalAlignment: &disableAlign,
			},
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T11:10:30+00:00",
		},
		{
			"with eval_offset, find previous offset point + default evalDelay",
			&Group{
				EvalOffset: &offset,
				Interval:   time.Hour,
			},
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T10:30:00+00:00",
		},
		{
			"with eval_offset + default evalDelay",
			&Group{
				EvalOffset: &offset,
				Interval:   time.Hour,
			},
			"2023-08-28T11:41:00+00:00",
			"2023-08-28T11:30:00+00:00",
		},
		{
			"1h interval with eval_delay",
			&Group{
				EvalDelay: &evalDelay,
				Interval:  time.Hour,
			},
			"2023-08-28T11:41:00+00:00",
			"2023-08-28T11:00:00+00:00",
		},
		{
			"1m interval with eval_delay",
			&Group{
				EvalDelay: &evalDelay,
				Interval:  time.Minute,
			},
			"2023-08-28T11:41:13+00:00",
			"2023-08-28T11:40:00+00:00",
		},
		{
			"disable alignment with eval_delay",
			&Group{
				EvalDelay:     &evalDelay,
				Interval:      time.Hour,
				evalAlignment: &disableAlign,
			},
			"2023-08-28T11:41:00+00:00",
			"2023-08-28T11:40:00+00:00",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originT, _ := time.Parse(time.RFC3339, tc.originTS)
			expT, _ := time.Parse(time.RFC3339, tc.expTS)
			gotTS := tc.g.adjustReqTimestamp(originT)
			if !gotTS.Equal(expT) {
				t.Fatalf("get wrong prometheus request timestamp, expect %s, got %s", expT, gotTS)
			}
		})
	}
}

func TestRangeIterator(t *testing.T) {
	testCases := []struct {
		ri     rangeIterator
		result [][2]time.Time
	}{
		{
			ri: rangeIterator{
				start: parseTime(t, "2021-01-01T12:00:00.000Z"),
				end:   parseTime(t, "2021-01-01T12:30:00.000Z"),
				step:  5 * time.Minute,
			},
			result: [][2]time.Time{
				{parseTime(t, "2021-01-01T12:00:00.000Z"), parseTime(t, "2021-01-01T12:05:00.000Z")},
				{parseTime(t, "2021-01-01T12:05:00.000Z"), parseTime(t, "2021-01-01T12:10:00.000Z")},
				{parseTime(t, "2021-01-01T12:10:00.000Z"), parseTime(t, "2021-01-01T12:15:00.000Z")},
				{parseTime(t, "2021-01-01T12:15:00.000Z"), parseTime(t, "2021-01-01T12:20:00.000Z")},
				{parseTime(t, "2021-01-01T12:20:00.000Z"), parseTime(t, "2021-01-01T12:25:00.000Z")},
				{parseTime(t, "2021-01-01T12:25:00.000Z"), parseTime(t, "2021-01-01T12:30:00.000Z")},
			},
		},
		{
			ri: rangeIterator{
				start: parseTime(t, "2021-01-01T12:00:00.000Z"),
				end:   parseTime(t, "2021-01-01T12:30:00.000Z"),
				step:  45 * time.Minute,
			},
			result: [][2]time.Time{
				{parseTime(t, "2021-01-01T12:00:00.000Z"), parseTime(t, "2021-01-01T12:30:00.000Z")},
				{parseTime(t, "2021-01-01T12:30:00.000Z"), parseTime(t, "2021-01-01T12:30:00.000Z")},
			},
		},
		{
			ri: rangeIterator{
				start: parseTime(t, "2021-01-01T12:00:12.000Z"),
				end:   parseTime(t, "2021-01-01T12:00:17.000Z"),
				step:  time.Second,
			},
			result: [][2]time.Time{
				{parseTime(t, "2021-01-01T12:00:12.000Z"), parseTime(t, "2021-01-01T12:00:13.000Z")},
				{parseTime(t, "2021-01-01T12:00:13.000Z"), parseTime(t, "2021-01-01T12:00:14.000Z")},
				{parseTime(t, "2021-01-01T12:00:14.000Z"), parseTime(t, "2021-01-01T12:00:15.000Z")},
				{parseTime(t, "2021-01-01T12:00:15.000Z"), parseTime(t, "2021-01-01T12:00:16.000Z")},
				{parseTime(t, "2021-01-01T12:00:16.000Z"), parseTime(t, "2021-01-01T12:00:17.000Z")},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			var j int
			for tc.ri.next() {
				if len(tc.result) < j+1 {
					t.Fatalf("unexpected result for iterator on step %d: %v - %v",
						j, tc.ri.s, tc.ri.e)
				}
				s, e := tc.ri.s, tc.ri.e
				expS, expE := tc.result[j][0], tc.result[j][1]
				if s != expS {
					t.Fatalf("expected to get start=%v; got %v", expS, s)
				}
				if e != expE {
					t.Fatalf("expected to get end=%v; got %v", expE, e)
				}
				j++
			}
		})
	}
}

func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse("2006-01-02T15:04:05.000Z", s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}
