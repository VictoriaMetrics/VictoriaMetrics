package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type fakeReplayQuerier struct {
	datasource.FakeQuerier
	registry map[string]map[string][]datasource.Metric
}

func (fr *fakeReplayQuerier) BuildWithParams(_ datasource.QuerierParams) datasource.Querier {
	return fr
}

type fakeRWClient struct{}

func (fc *fakeRWClient) Push(_ prompb.TimeSeries) error {
	return nil
}

func (fc *fakeRWClient) Close() error {
	return nil
}

func (fr *fakeReplayQuerier) QueryRange(_ context.Context, q string, from, to time.Time) (res datasource.Result, err error) {
	key := fmt.Sprintf("%s+%s", from.Format("15:04:05"), to.Format("15:04:05"))
	dps, ok := fr.registry[q]
	if !ok {
		return res, fmt.Errorf("unexpected query received: %q", q)
	}
	metrics, ok := dps[key]
	if !ok {
		return res, fmt.Errorf("unexpected time range received: %q", key)
	}
	res.Data = metrics
	return res, nil
}

func TestReplay(t *testing.T) {
	f := func(from, to string, maxDP, ruleConcurrency int, ruleDelay time.Duration, cfg []config.Group, qb *fakeReplayQuerier, expectTotalRows int) {
		t.Helper()

		fromOrig, toOrig, maxDatapointsOrig := *replayFrom, *replayTo, *replayMaxDatapoints
		retriesOrig, delayOrig := *replayRuleRetryAttempts, *replayRulesDelay
		defer func() {
			*replayFrom, *replayTo = fromOrig, toOrig
			*replayMaxDatapoints, *replayRuleRetryAttempts = maxDatapointsOrig, retriesOrig
			*replayRulesDelay = delayOrig
		}()

		*replayRuleRetryAttempts = 1
		*replayRulesDelay = ruleDelay
		rwb := &fakeRWClient{}
		*replayFrom = from
		*replayTo = to
		*ruleEvaluationConcurrency = ruleConcurrency
		*replayMaxDatapoints = maxDP
		totalRows, _, err := replay(cfg, qb, rwb)
		if err != nil {
			t.Fatalf("replay failed: %s", err)
		}
		if totalRows != expectTotalRows {
			t.Fatalf("unexpected total rows count: got %d, want %d", totalRows, expectTotalRows)
		}
	}

	// one rule + one response
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:00.000Z", 10, 1, time.Millisecond, []config.Group{
		{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up)": {"12:00:00+12:02:00": {
				{
					Timestamps: []int64{1},
					Values:     []float64{1},
				},
			}},
		},
	}, 1)

	// one rule + multiple responses
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 1, time.Millisecond, []config.Group{
		{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up)": {
				"12:00:00+12:01:00": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
				"12:01:00+12:02:00": {},
				"12:02:00+12:02:30": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
			},
		},
	}, 2)

	// datapoints per step
	f("2021-01-01T12:00:00.000Z", "2021-01-01T15:02:30.000Z", 60, 1, time.Millisecond, []config.Group{
		{Interval: promutil.NewDuration(time.Minute), Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up)": {
				"12:00:00+13:00:00": {
					{
						Timestamps: []int64{1, 2},
						Values:     []float64{1, 2},
					},
				},
				"13:00:00+14:00:00": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
				"14:00:00+15:00:00": {},
				"15:00:00+15:02:30": {},
			},
		},
	}, 3)

	// multiple recording rules + multiple responses
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 1, time.Millisecond, []config.Group{
		{Rules: []config.Rule{{Record: "foo", Expr: "sum(up)"}}},
		{Rules: []config.Rule{{Record: "bar", Expr: "max(up)"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up)": {
				"12:00:00+12:01:00": {
					{
						Timestamps: []int64{1, 2},
						Values:     []float64{1, 2},
					},
				},
				"12:01:00+12:02:00": {},
				"12:02:00+12:02:30": {},
			},
			"max(up)": {
				"12:00:00+12:01:00": {},
				"12:01:00+12:02:00": {
					{
						Timestamps: []int64{1, 2},
						Values:     []float64{1, 2},
					},
				},
				"12:02:00+12:02:30": {},
			},
		},
	}, 4)

	// multiple alerting rules + multiple responses
	// alerting rule generates two series `ALERTS` and `ALERTS_FOR_STATE` when triggered
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 1, time.Millisecond, []config.Group{
		{Rules: []config.Rule{{Alert: "foo", Expr: "sum(up) > 1"}}},
		{Rules: []config.Rule{{Alert: "bar", Expr: "max(up) < 1"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up) > 1": {
				"12:00:00+12:01:00": {
					{
						Timestamps: []int64{1, 2},
						Values:     []float64{1, 2},
					},
				},
				"12:01:00+12:02:00": {},
				"12:02:00+12:02:30": {},
			},
			"max(up) < 1": {
				"12:00:00+12:01:00": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
				"12:01:00+12:02:00": {},
				"12:02:00+12:02:30": {},
			},
		},
	}, 6)

	// multiple recording rules in one group+ multiple responses + concurrency
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 1, 0, []config.Group{
		{Rules: []config.Rule{{Record: "foo", Expr: "sum(up) > 1"}, {Record: "bar", Expr: "max(up) < 1"}}, Concurrency: 2}}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up) > 1": {
				"12:00:00+12:01:00": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
				"12:01:00+12:02:00": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
				"12:02:00+12:02:30": {
					{
						Timestamps: []int64{1},
						Values:     []float64{1},
					},
				},
			},
			"max(up) < 1": {
				"12:00:00+12:01:00": {},
				"12:01:00+12:02:00": {{
					Timestamps: []int64{1},
					Values:     []float64{1},
				}},
				"12:02:00+12:02:30": {},
			},
		},
	}, 4)

	// single rule + rule concurrency
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 3, time.Millisecond, []config.Group{
		{Rules: []config.Rule{{Record: "foo-concurrent", Expr: "sum(up)"}}},
	}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up)": {
				"12:00:00+12:01:00": {},
				"12:01:00+12:02:00": {{
					Timestamps: []int64{1},
					Values:     []float64{1},
				}},
				"12:02:00+12:02:30": {},
			},
		},
	}, 1)

	// multiple rules + rule concurrency + group concurrency
	f("2021-01-01T12:00:00.000Z", "2021-01-01T12:02:30.000Z", 1, 3, 0, []config.Group{
		{Rules: []config.Rule{{Alert: "foo-group-single-concurrent", For: promutil.NewDuration(30 * time.Second), Expr: "sum(up) > 1"}, {Alert: "bar-group-single-concurrent", Expr: "max(up) < 1"}}, Concurrency: 2}}, &fakeReplayQuerier{
		registry: map[string]map[string][]datasource.Metric{
			"sum(up) > 1": {
				"12:00:00+12:01:00": {{
					Timestamps: []int64{1609502460},
					Values:     []float64{1},
				}},
				"12:01:00+12:02:00": {{
					Timestamps: []int64{1609502520},
					Values:     []float64{1},
				}},
				"12:02:00+12:02:30": {{
					Timestamps: []int64{1609502580},
					Values:     []float64{1},
				}},
			},
			"max(up) < 1": {
				"12:00:00+12:01:00": {{
					Timestamps: []int64{1609502460},
					Values:     []float64{1},
				}},
				"12:01:00+12:02:00": {{
					Timestamps: []int64{1609502520},
					Values:     []float64{1},
				}},
				"12:02:00+12:02:30": {},
			},
		},
	}, 10)
}
