package metricnamestats

import (
	"sort"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMetricsTracker(t *testing.T) {
	type testOp struct {
		o  byte
		mg string
		ts uint64
	}
	f := func(ops []testOp, predicate uint64, expected StatsResult) {
		umt := MustLoadFrom("testdata/unused_metrics_cache", 100_000)
		for _, op := range ops {
			umt.getCurrentTs = func() uint64 {
				return op.ts
			}
			switch op.o {
			case 'i':
				umt.RegisterIngestRequest([]byte(op.mg))
			case 'r':
				umt.RegisterQueryRequest([]byte(op.mg))
			}
		}

		got := umt.GetStats(100, predicate)
		gotR := got.Records
		expectedR := expected.Records
		sort.Slice(gotR, func(i, j int) bool {
			return gotR[i].MetricName < gotR[j].MetricName
		})
		sort.Slice(expectedR, func(i, j int) bool {
			return expectedR[i].MetricName < expectedR[j].MetricName
		})
		if !cmp.Equal(expectedR, gotR) {
			t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expectedR, gotR))
		}
	}
	f([]testOp{{'i', "metric_1", 1}, {'r', "metric_2", 2}, {'r', "metric_1", 2}, {'i', "metric_3", 2}},
		0,
		StatsResult{
			Records: []StatRecord{
				{
					MetricName: "metric_3",
				},
			},
		})
	f([]testOp{{'i', "metric_1", 1}, {'r', "metric_2", 2}, {'r', "metric_2", 3}, {'r', "metric_1", 2}, {'i', "metric_3", 2}},
		1,
		StatsResult{
			Records: []StatRecord{
				{
					MetricName:    "metric_1",
					RequestCount:  1,
					LastRequestTs: 2,
				},
				{
					MetricName: "metric_3",
				},
			},
		})

}

func TestMetricsTrackerConcurrent(t *testing.T) {
	type testOp struct {
		o  byte
		mg string
	}
	const concurrency = 3
	f := func(ops []testOp, predicate uint64, expected StatsResult) {
		umt := MustLoadFrom("testdata/unused_metrics_cache_concurrent", 100_000)
		umt.getCurrentTs = func() uint64 {
			return 1
		}
		umt.creationTs.Store(0)
		for _, op := range ops {
			switch op.o {
			case 'i':
				umt.RegisterIngestRequest([]byte(op.mg))
			case 'r':
				umt.RegisterQueryRequest([]byte(op.mg))
			}
		}

		var wg sync.WaitGroup
		for range concurrency {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, op := range ops {
					switch op.o {
					case 'i':
						umt.RegisterIngestRequest([]byte(op.mg))
					case 'r':
						umt.RegisterQueryRequest([]byte(op.mg))
					}
				}
			}()
		}
		wg.Wait()

		got := umt.GetStats(100, predicate)
		gotR := got.Records
		expectedR := expected.Records
		sort.Slice(gotR, func(i, j int) bool {
			return gotR[i].MetricName < gotR[j].MetricName
		})
		sort.Slice(expectedR, func(i, j int) bool {
			return expectedR[i].MetricName < expectedR[j].MetricName
		})
		if !cmp.Equal(expectedR, gotR) {
			t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expectedR, gotR))
		}
	}
	f([]testOp{{'i', "metric_1"}, {'r', "metric_2"}, {'r', "metric_1"}, {'i', "metric_3"}},
		0,
		StatsResult{
			Records: []StatRecord{
				{
					MetricName: "metric_3",
				},
			},
		})
	f([]testOp{{'i', "metric_1"}, {'r', "metric_2"}, {'r', "metric_2"}, {'r', "metric_1"}, {'i', "metric_3"}},
		10,
		StatsResult{
			Records: []StatRecord{
				{
					MetricName:    "metric_1",
					RequestCount:  1 + concurrency,
					LastRequestTs: 1,
				},
				{
					MetricName:    "metric_2",
					RequestCount:  2 + 2*concurrency,
					LastRequestTs: 1,
				},
				{
					MetricName:    "metric_3",
					LastRequestTs: 0,
				},
			},
		})

}
