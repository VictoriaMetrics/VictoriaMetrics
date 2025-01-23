package metricnamestats

import (
	"path"
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
	type queryOpts struct {
		limit        int
		lte          int
		matchPattern string
	}
	cachePath := path.Join(t.TempDir(), "metric_names_usage")
	f := func(ops []testOp, qo queryOpts, expected StatsResult) {
		t.Helper()
		mt, err := loadFrom(cachePath, 100_000)
		if err != nil {
			t.Fatalf("cannot load state from disk on init: %s", err)
		}
		for _, op := range ops {
			mt.getCurrentTs = func() uint64 {
				return op.ts
			}
			switch op.o {
			case 'i':
				mt.RegisterIngestRequest([]byte(op.mg))
			case 'r':
				mt.RegisterQueryRequest([]byte(op.mg))
			}
		}

		got := mt.GetStats(qo.limit, qo.lte, qo.matchPattern)
		got.sort()
		expected.sort()
		if !cmp.Equal(expected.Records, got.Records) {
			t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expected.Records, got.Records))
		}
		if err := mt.save(); err != nil {
			t.Fatalf("cannot save in-memory state: %s", err)
		}
		loadedUmt, err := loadFrom(cachePath, 100_00)
		if err != nil {
			t.Fatalf("cannot load restore state from disk: %s", err)
		}
		got = loadedUmt.GetStats(qo.limit, qo.lte, qo.matchPattern)
		got.sort()
		if !cmp.Equal(expected.Records, got.Records) {
			t.Fatalf("unexpected unusedMetricNames result after load state from disk: %s", cmp.Diff(expected.Records, got.Records))
		}
		mt.Reset()
	}

	f([]testOp{{'i', "metric_1", 1}, {'r', "metric_2", 2}, {'r', "metric_1", 2}, {'i', "metric_3", 2}},
		queryOpts{limit: 100, lte: 0},
		StatsResult{
			Records: []StatRecord{
				{
					MetricName: "metric_3",
				},
			},
		})
	f([]testOp{{'i', "metric_1", 1}, {'r', "metric_2", 2}, {'r', "metric_2", 3}, {'r', "metric_1", 2}, {'i', "metric_3", 2}},
		queryOpts{limit: 100, lte: 1},
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

	f([]testOp{{'i', "go_proc", 1}, {'r', "go_proc", 2}, {'r', "go_cpu", 3}, {'r', "go_cpu", 2}, {'i', "go_sched", 2}, {'r', "go_proc", 3}},
		queryOpts{limit: 100, lte: -1, matchPattern: "go_proc"},
		StatsResult{
			Records: []StatRecord{
				{
					MetricName:    "go_proc",
					RequestCount:  2,
					LastRequestTs: 3,
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
	f := func(ops []testOp, predicate int, expected StatsResult) {
		umt := &Tracker{
			maxSizeBytes: 100_000,
			store:        &sync.Map{},
			getCurrentTs: func() uint64 {
				return 1
			},
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

		got := umt.GetStats(100, predicate, "")
		got.sort()
		expected.sort()
		if !cmp.Equal(expected.Records, got.Records) {
			t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expected.Records, got.Records))
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

func TestMetricsTrackerMaxSize(t *testing.T) {
	type testOp struct {
		o  byte
		mg string
	}
	umt := &Tracker{
		maxSizeBytes: 12,
		store:        &sync.Map{},
		getCurrentTs: func() uint64 {
			return 1
		},
	}
	ops := []testOp{{'i', "metric_1"}, {'r', "metric_2"}, {'r', "metric_1"}, {'i', "metric_3"}, {'i', "metric_4"}, {'r', "metric_1"}, {'r', "metric_2"}, {'r', "metric_2"}}
	for _, op := range ops {
		switch op.o {
		case 'i':
			umt.RegisterIngestRequest([]byte(op.mg))
		case 'r':
			umt.RegisterQueryRequest([]byte(op.mg))
		}
	}
	got := umt.GetStats(100, -1, "")
	got.sort()
	expected := StatsResult{
		Records: []StatRecord{
			{
				MetricName:    "metric_1",
				RequestCount:  2,
				LastRequestTs: 1,
			},
			{
				MetricName:    "metric_2",
				RequestCount:  3,
				LastRequestTs: 1,
			},
		},
	}

	if !cmp.Equal(expected.Records, got.Records) {
		t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expected.Records, got.Records))
	}
}
