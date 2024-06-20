package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type fakeReplayQuerier struct {
	datasource.FakeQuerier
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
				{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
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
				{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
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
				{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
				{Rules: []config.Rule{{Record: "bar", Expr: "max(up)"}}},
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
				{Rules: []config.Rule{{Alert: "foo", Expr: "sum(up) > 1"}}},
				{Rules: []config.Rule{{Alert: "bar", Expr: "max(up) < 1"}}},
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
			if err := replay(tc.cfg, tc.qb, &remotewrite.DebugClient{}); err != nil {
				t.Fatalf("replay failed: %s", err)
			}
			if len(tc.qb.registry) > 0 {
				t.Fatalf("not all requests were sent: %#v", tc.qb.registry)
			}
		})
	}
}
