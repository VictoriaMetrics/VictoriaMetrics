package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type fakeReplayQuerier struct {
	fakeQuerier
	registry map[string]map[string]struct{}
}

func (fr *fakeReplayQuerier) BuildWithParams(_ datasource.QuerierParams) datasource.Querier {
	return fr
}

func (fr *fakeReplayQuerier) QueryRange(_ context.Context, q string, from, to time.Time) (res datasource.Result, err error) {
	key := fmt.Sprintf("%s+%s", from.Format("15:04:05"), to.Format("15:04:05"))
	dps, ok := fr.registry[q]
	if !ok {
		return res, fmt.Errorf("unexpected query received: %q", q)
	}
	_, ok = dps[key]
	if !ok {
		return res, fmt.Errorf("unexpected time range received: %q", key)
	}
	delete(dps, key)
	if len(fr.registry[q]) < 1 {
		delete(fr.registry, q)
	}
	return res, nil
}

func TestReplay(t *testing.T) {
	testCases := []struct {
		name     string
		from, to string
		maxDP    int
		cfg      []config.Group
		qb       *fakeReplayQuerier
	}{
		{
			name:  "one rule + one response",
			from:  "2021-01-01T12:00:00.000Z",
			to:    "2021-01-01T12:02:00.000Z",
			maxDP: 10,
			cfg: []config.Group{
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
			},
			qb: &fakeReplayQuerier{
				registry: map[string]map[string]struct{}{
					"sum(up)": {"12:00:00+12:02:00": {}},
				},
			},
		},
		{
			name:  "one rule + multiple responses",
			from:  "2021-01-01T12:00:00.000Z",
			to:    "2021-01-01T12:02:30.000Z",
			maxDP: 1,
			cfg: []config.Group{
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
			},
			qb: &fakeReplayQuerier{
				registry: map[string]map[string]struct{}{
					"sum(up)": {
						"12:00:00+12:01:00": {},
						"12:01:00+12:02:00": {},
						"12:02:00+12:02:30": {},
					},
				},
			},
		},
		{
			name:  "datapoints per step",
			from:  "2021-01-01T12:00:00.000Z",
			to:    "2021-01-01T15:02:30.000Z",
			maxDP: 60,
			cfg: []config.Group{
				{Interval: promutils.NewDuration(time.Minute), Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
			},
			qb: &fakeReplayQuerier{
				registry: map[string]map[string]struct{}{
					"sum(up)": {
						"12:00:00+13:00:00": {},
						"13:00:00+14:00:00": {},
						"14:00:00+15:00:00": {},
						"15:00:00+15:02:30": {},
					},
				},
			},
		},
		{
			name:  "multiple recording rules + multiple responses",
			from:  "2021-01-01T12:00:00.000Z",
			to:    "2021-01-01T12:02:30.000Z",
			maxDP: 1,
			cfg: []config.Group{
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Record: "bar", Expr: "max(up)"}}},
			},
			qb: &fakeReplayQuerier{
				registry: map[string]map[string]struct{}{
					"sum(up)": {
						"12:00:00+12:01:00": {},
						"12:01:00+12:02:00": {},
						"12:02:00+12:02:30": {},
					},
					"max(up)": {
						"12:00:00+12:01:00": {},
						"12:01:00+12:02:00": {},
						"12:02:00+12:02:30": {},
					},
				},
			},
		},
		{
			name:  "multiple alerting rules + multiple responses",
			from:  "2021-01-01T12:00:00.000Z",
			to:    "2021-01-01T12:02:30.000Z",
			maxDP: 1,
			cfg: []config.Group{
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Alert: "foo", Expr: "sum(up) > 1"}}},
				{Interval: promutils.NewDuration(*evaluationInterval), Rules: []config.Rule{{Alert: "bar", Expr: "max(up) < 1"}}},
			},
			qb: &fakeReplayQuerier{
				registry: map[string]map[string]struct{}{
					"sum(up) > 1": {
						"12:00:00+12:01:00": {},
						"12:01:00+12:02:00": {},
						"12:02:00+12:02:30": {},
					},
					"max(up) < 1": {
						"12:00:00+12:01:00": {},
						"12:01:00+12:02:00": {},
						"12:02:00+12:02:30": {},
					},
				},
			},
		},
	}

	from, to, maxDP := *replayFrom, *replayTo, *replayMaxDatapoints
	retries, delay := *replayRuleRetryAttempts, *replayRulesDelay
	defer func() {
		*replayFrom, *replayTo = from, to
		*replayMaxDatapoints, *replayRuleRetryAttempts = maxDP, retries
		*replayRulesDelay = delay
	}()

	*replayRuleRetryAttempts = 1
	*replayRulesDelay = time.Millisecond
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			*replayFrom = tc.from
			*replayTo = tc.to
			*replayMaxDatapoints = tc.maxDP
			if err := replay(tc.cfg, tc.qb, nil); err != nil {
				t.Fatalf("replay failed: %s", err)
			}
			if len(tc.qb.registry) > 0 {
				t.Fatalf("not all requests were sent: %#v", tc.qb.registry)
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
