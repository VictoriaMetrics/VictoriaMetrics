package metricnamestats

import (
	"path"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var statsResultCmpOpts = cmpopts.IgnoreFields(StatsResult{}, "CollectedSinceTs", "MaxSizeBytes", "CurrentSizeBytes")

func TestMetricsTracker(t *testing.T) {
	type testOp struct {
		aID uint32
		pID uint32
		o   byte
		mg  string
		ts  uint64
	}
	type queryOpts struct {
		accountID     uint32
		projectID     uint32
		isTenantEmpty bool
		limit         int
		lte           int
		matchPattern  string
	}
	cmpOpts := cmpopts.IgnoreFields(StatsResult{}, "CollectedSinceTs", "MaxSizeBytes", "CurrentSizeBytes")
	cachePath := path.Join(t.TempDir(), t.Name())
	f := func(ops []testOp, qo queryOpts, expected StatsResult) {
		t.Helper()
		expected.sort()
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
				mt.RegisterIngestRequest(op.aID, op.pID, []byte(op.mg))
			case 'r':
				mt.RegisterQueryRequest(op.aID, op.pID, []byte(op.mg))
			}
		}
		var got StatsResult
		if qo.isTenantEmpty {
			got = mt.GetStats(qo.limit, qo.lte, qo.matchPattern)
			got.sort()
			got.DeduplicateMergeRecords()
		} else {
			got = mt.GetStatsForTenant(qo.accountID, qo.projectID, qo.limit, qo.lte, qo.matchPattern)
			got.sort()
		}
		if !cmp.Equal(expected, got, cmpOpts) {
			t.Fatalf("unexpected GetStatsForTenant result: %s", cmp.Diff(expected, got, cmpOpts))
		}
		if err := mt.saveLocked(); err != nil {
			t.Fatalf("cannot save in-memory state: %s", err)
		}
		loadedUmt, err := loadFrom(cachePath, 100_000)
		if err != nil {
			t.Fatalf("cannot load restore state from disk: %s", err)
		}
		if qo.isTenantEmpty {
			got = loadedUmt.GetStats(qo.limit, qo.lte, qo.matchPattern)
			got.sort()
			got.DeduplicateMergeRecords()
		} else {
			got = loadedUmt.GetStatsForTenant(qo.accountID, qo.projectID, qo.limit, qo.lte, qo.matchPattern)
			got.sort()
		}
		if !cmp.Equal(expected, got, cmpOpts) {
			t.Fatalf("unexpected GetStatsForTenant result after load state from disk: %s", cmp.Diff(expected, got, cmpOpts))
		}
		mt.Reset(func() {})
	}

	dataSet := []testOp{
		{1, 1, 'i', "metric_1", 1},
		{1, 1, 'i', "metric_1", 1},
		{1, 1, 'r', "metric_1", 1},
		{1, 1, 'i', "metric_2", 1},
		{1, 1, 'r', "metric_2", 1},
		{1, 1, 'r', "metric_2", 1},
		{15, 15, 'i', "metric_1", 1},
		{15, 15, 'i', "metric_2", 1},
		{15, 15, 'i', "metric_3", 1},
		{15, 15, 'r', "metric_3", 1},
		{15, 15, 'r', "metric_2", 1},
	}
	qOpts := queryOpts{
		limit: 100,
		lte:   -1,
	}
	// query empty tenant
	expected := StatsResult{
		TotalRecords: 5,
	}
	f(dataSet, qOpts, expected)

	// query single tenant
	qOpts = queryOpts{
		accountID: 1,
		projectID: 1,
		limit:     100,
		lte:       -1,
	}
	expected = StatsResult{
		TotalRecords: 5,
		Records: []StatRecord{
			{"metric_1", 1, 1},
			{"metric_2", 2, 1},
		},
	}
	f(dataSet, qOpts, expected)

	// query all tenants
	qOpts = queryOpts{
		isTenantEmpty: true,
		limit:         100,
		lte:           -1,
	}
	expected = StatsResult{
		TotalRecords: 5,
		Records: []StatRecord{
			{"metric_1", 1, 1},
			{"metric_2", 3, 1},
			{"metric_3", 1, 1},
		},
	}
	f(dataSet, qOpts, expected)
}

func TestMetricsTrackerConcurrent(t *testing.T) {
	type testOp struct {
		o  byte
		mg string
	}
	const concurrency = 3
	f := func(ops []testOp, predicate int, expected StatsResult) {
		t.Helper()
		umt, err := loadFrom(t.TempDir()+t.Name(), 1024)
		if err != nil {
			t.Fatalf("cannot load: %s", err)
		}
		umt.creationTs.Store(0)
		umt.getCurrentTs = func() uint64 { return 1 }
		for _, op := range ops {
			switch op.o {
			case 'i':
				umt.RegisterIngestRequest(0, 0, []byte(op.mg))
			case 'r':
				umt.RegisterQueryRequest(0, 0, []byte(op.mg))
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
						umt.RegisterIngestRequest(0, 0, []byte(op.mg))
					case 'r':
						umt.RegisterQueryRequest(0, 0, []byte(op.mg))
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
	f([]testOp{{'i', "metric_1"}, {'i', "metric_2"}, {'r', "metric_2"}, {'r', "metric_2"}, {'r', "metric_1"}, {'i', "metric_3"}},
		10,
		StatsResult{
			Records: []StatRecord{
				{
					MetricName:    "metric_1",
					RequestsCount: 1 + concurrency,
					LastRequestTs: 1,
				},
				{
					MetricName:    "metric_2",
					RequestsCount: 2 + 2*concurrency,
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

	umt, err := loadFrom(t.TempDir()+t.Name(), storeOverhead+10*2)
	if err != nil {
		t.Fatalf("cannot load tracker: %s", err)
	}
	umt.getCurrentTs = func() uint64 { return 1 }
	ops := []testOp{
		{'i', "metric_1"},
		{'r', "metric_2"},
		{'r', "metric_1"},
		{'i', "metric_2"},
		{'i', "metric_3"},
		{'i', "metric_4"},
		{'r', "metric_1"},
		{'r', "metric_2"},
		{'r', "metric_2"},
		{'r', "metric_2"},
	}
	for _, op := range ops {
		switch op.o {
		case 'i':
			umt.RegisterIngestRequest(0, 0, []byte(op.mg))
		case 'r':
			umt.RegisterQueryRequest(0, 0, []byte(op.mg))
		}
	}
	got := umt.GetStats(100, -1, "")
	got.sort()
	expected := StatsResult{
		Records: []StatRecord{
			{
				MetricName:    "metric_1",
				RequestsCount: 2,
				LastRequestTs: 1,
			},
			{
				MetricName:    "metric_2",
				RequestsCount: 3,
				LastRequestTs: 1,
			},
		},
	}

	if !cmp.Equal(expected.Records, got.Records) {
		t.Fatalf("unexpected unusedMetricNames result: %s", cmp.Diff(expected.Records, got.Records))
	}
}

func TestDeduplicateRecords(t *testing.T) {
	f := func(result StatsResult, expected StatsResult) {
		t.Helper()
		expected.sort()
		result.sort()
		result.DeduplicateMergeRecords()
		if !cmp.Equal(result, expected, statsResultCmpOpts) {
			t.Fatalf("unexpected deduplicate result: %s", cmp.Diff(result, expected, statsResultCmpOpts))
		}
	}

	// single record
	dataSet := StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
		},
	}
	expected := StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
		},
	}
	f(dataSet, expected)

	// no duplicates
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	f(dataSet, expected)

	// 2 duplicates
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 1},
		},
	}
	f(dataSet, expected)

	// duplicates on start
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	f(dataSet, expected)

	// duplicates on end
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 30, LastRequestTs: 4},
		},
	}
	f(dataSet, expected)

	// duplicates start end
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 13, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 30, LastRequestTs: 4},
		},
	}
	f(dataSet, expected)

	// duplicates mixed
	dataSet = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 10, LastRequestTs: 3},
			{MetricName: "mn3", RequestsCount: 10, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
			{MetricName: "mn4", RequestsCount: 15, LastRequestTs: 4},
			{MetricName: "mn5", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 1},
			{MetricName: "mn2", RequestsCount: 12, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 3},
			{MetricName: "mn4", RequestsCount: 30, LastRequestTs: 4},
			{MetricName: "mn5", RequestsCount: 15, LastRequestTs: 4},
		},
	}
	f(dataSet, expected)
}

func TestStatsResultMerge(t *testing.T) {
	f := func(left, right StatsResult, expected StatsResult) {
		t.Helper()
		expected.sort()
		left.sort()
		right.sort()
		left.Merge(&right)
		if !cmp.Equal(left, expected, statsResultCmpOpts) {
			t.Fatalf("unexpected deduplicate result: %s", cmp.Diff(left, expected, statsResultCmpOpts))
		}
	}

	// empty src
	dst := StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
		},
	}
	src := StatsResult{}
	expected := StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

	// empty dst
	dst = StatsResult{}
	src = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

	// all duplicates
	dst = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	src = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 40, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 60, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

	// no duplicates
	dst = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	src = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

	// mixed
	dst = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 2},
		},
	}
	src = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 40, LastRequestTs: 2},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

	// mixed
	dst = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 1},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	src = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 20, LastRequestTs: 2},
		},
	}
	expected = StatsResult{
		Records: []StatRecord{
			{MetricName: "mn1", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn2", RequestsCount: 20, LastRequestTs: 2},
			{MetricName: "mn3", RequestsCount: 30, LastRequestTs: 2},
			{MetricName: "mn4", RequestsCount: 10, LastRequestTs: 2},
			{MetricName: "mn5", RequestsCount: 40, LastRequestTs: 2},
			{MetricName: "mn6", RequestsCount: 30, LastRequestTs: 2},
		},
	}
	f(dst, src, expected)

}
