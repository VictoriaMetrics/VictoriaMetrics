package tests

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_DataIsWrittenSeveralTimes(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		replicationFactor = 3
		vmstorageCount    = 2*replicationFactor + 1
	)

	vmstorages := make([]*at.Vmstorage, vmstorageCount)
	vminsertAddrs := make([]string, vmstorageCount)
	for i := range vmstorageCount {
		instance := fmt.Sprintf("vmstorage-%d", i)
		vmstorages[i] = tc.MustStartVmstorage(instance, []string{
			"-storageDataPath=" + tc.Dir() + "/" + instance,
			"-retentionPeriod=100y",
		})
		vminsertAddrs[i] = vmstorages[i].VminsertAddr()
	}

	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + strings.Join(vminsertAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(vmstorages...)

	// Verify that each strorage node has metrics and that total metric count across
	// all vmstorages is replicationFactor*numRecs.

	getMetricsReadTotal := func(app *at.Vmstorage) int {
		t.Helper()
		got := app.GetIntMetric(t, "vm_vminsert_metrics_read_total")
		if got <= 0 {
			t.Fatalf("%s unexpected metric count: got %d, want > 0", app.Name(), got)
		}
		return got
	}

	cnts := make([]int, vmstorageCount)
	var got int
	for i := range vmstorageCount {
		cnts[i] = getMetricsReadTotal(vmstorages[i])
		got += cnts[i]
	}
	want := replicationFactor * numRecs
	if got != want {
		t.Fatalf("unxepected metric count across all vmstorage replicas: got sum(%v) = %d, want %d*%d = %d", cnts, got, replicationFactor, numRecs, want)
	}
}

// TestClusterReplication_VmselectDeduplication checks now vmselect behaves when
// the data is replicated.
//
// When the data is replicated, vmselect's netstorage will receive duplicates.
// It can be instructed to remove duplicates by setting -dedup.minScrapeInterval
// flag. See mergeSortBlocks() in app/vmselect/netstorage/netstorage.go.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_VmselectDeduplication(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		replicationFactor = 2
		vmstorageCount    = 2*replicationFactor + 1
	)

	vmstorages := make([]*at.Vmstorage, vmstorageCount)
	vminsertAddrs := make([]string, vmstorageCount)
	vmselectAddrs := make([]string, vmstorageCount)
	for i := range vmstorageCount {
		instance := fmt.Sprintf("vmstorage-%d", i)
		vmstorages[i] = tc.MustStartVmstorage(instance, []string{
			"-storageDataPath=" + tc.Dir() + "/" + instance,
			"-retentionPeriod=100y",
		})
		vminsertAddrs[i] = vmstorages[i].VminsertAddr()
		vmselectAddrs[i] = vmstorages[i].VmselectAddr()
	}

	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + strings.Join(vminsertAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
	})
	vmselectDedup := tc.MustStartVmselect("vmselect-dedup", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-dedup.minScrapeInterval=1ms",
	})

	// Insert data.

	const (
		numMetrics = 4
		numSamples = 1000
		numRecs    = numMetrics * numSamples
	)
	var recs []string
	for m := range numMetrics {
		ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for s := range numSamples {
			recs = append(recs, fmt.Sprintf("metric_%d %d %d", m, s, ts.Unix()))
			ts = ts.Add(1 * time.Minute)
		}
	}
	vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(vmstorages...)

	// Check /api/v1/series response.
	//
	// vmselect is expected to return no duplicates regardless whether
	// -dedup.minScrapeInterval is set or not.

	assertSeries := func(app *at.Vmselect) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{
					Start: "2024-01-01T00:00:00Z",
					End:   "2024-01-31T00:00:00Z",
				}).Sort()
			},
			Want: &at.PrometheusAPIV1SeriesResponse{
				Status:    "success",
				IsPartial: false,
				Data: []map[string]string{
					{"__name__": "metric_0"},
					{"__name__": "metric_1"},
					{"__name__": "metric_2"},
					{"__name__": "metric_3"},
				},
			},
		})
	}
	assertSeries(vmselect)
	assertSeries(vmselectDedup)

	// Check /api/v1/query response.
	//
	// For queries that do not return range vector, vmselect returns no
	// duplicates regardless whether -dedup.minScrapeInterval is set or not.

	assertQuery := func(app *at.Vmselect) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return app.PrometheusAPIV1Query(t, "metric_1", at.QueryOpts{
					Time: "2024-01-01T00:05:00Z",
					Step: "5m",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "vector",
					Result: []*at.QueryResult{
						{
							Metric: map[string]string{"__name__": "metric_1"},
							Sample: at.NewSample(t, "2024-01-01T00:05:00Z", 5),
						},
					},
				},
			},
		})
	}
	assertQuery(vmselect)
	assertQuery(vmselectDedup)

	// Check /api/v1/query response (range vector queries)
	//
	// For queries that return range vector, vmselect is expected to
	// return duplicates when -dedup.minScrapeInterval is not set.

	duplicateNTimes := func(n int, samples []*at.Sample) []*at.Sample {
		dupedSamples := make([]*at.Sample, len(samples)*n)
		for i, s := range samples {
			for j := range n {
				dupedSamples[n*i+j] = s
			}
		}
		return dupedSamples
	}

	assertQueryRangeVector := func(app *at.Vmselect, wantDuplicates int) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return app.PrometheusAPIV1Query(t, "metric_1[5m]", at.QueryOpts{
					Time: "2024-01-01T00:05:00Z",
					Step: "5m",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result: []*at.QueryResult{
						{
							Metric: map[string]string{"__name__": "metric_1"},
							Samples: duplicateNTimes(wantDuplicates, []*at.Sample{
								at.NewSample(t, "2024-01-01T00:01:00Z", 1),
								at.NewSample(t, "2024-01-01T00:02:00Z", 2),
								at.NewSample(t, "2024-01-01T00:03:00Z", 3),
								at.NewSample(t, "2024-01-01T00:04:00Z", 4),
								at.NewSample(t, "2024-01-01T00:05:00Z", 5),
							}),
						},
					},
				},
			},
		})
	}
	assertQueryRangeVector(vmselect, replicationFactor)
	assertQueryRangeVector(vmselectDedup, 1)

	// Check /api/v1/query_range response.
	//
	// For range queries, vmselect is expected to return no duplicates
	// regardless whether -dedup.minScrapeInterval is set or not.

	assertQueryRange := func(app *at.Vmselect) {
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, "metric_1", at.QueryOpts{
					Start: "2024-01-01T00:00:00Z",
					End:   "2024-01-01T00:10:00Z",
					Step:  "5m",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result: []*at.QueryResult{
						{
							Metric: map[string]string{"__name__": "metric_1"},
							Samples: []*at.Sample{
								at.NewSample(t, "2024-01-01T00:00:00Z", 0),
								at.NewSample(t, "2024-01-01T00:05:00Z", 5),
								at.NewSample(t, "2024-01-01T00:10:00Z", 10),
							},
						},
					},
				},
			},
		})
	}
	assertQueryRange(vmselect)
	assertQueryRange(vmselectDedup)

	// Check /api/v1/export response.
	//
	// // vmselect is expected to return duplicates when
	// -dedup.minScrapeInterval is not set.

	assertExport := func(app *at.Vmselect, wantDuplicates int) {
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/export response",
			Got: func() any {
				return app.PrometheusAPIV1Export(t, `{__name__="metric_1"}`, at.QueryOpts{
					Start: "2024-01-01T00:00:00Z",
					End:   "2024-01-01T00:03:00Z",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result: []*at.QueryResult{
						{
							Metric: map[string]string{"__name__": "metric_1"},
							Samples: duplicateNTimes(wantDuplicates, []*at.Sample{
								at.NewSample(t, "2024-01-01T00:00:00Z", 0),
								at.NewSample(t, "2024-01-01T00:01:00Z", 1),
								at.NewSample(t, "2024-01-01T00:02:00Z", 2),
								at.NewSample(t, "2024-01-01T00:03:00Z", 3),
							}),
						},
					},
				},
			},
		})
	}
	assertExport(vmselect, replicationFactor)
	assertExport(vmselectDedup, 1)
}

// TestClusterReplication_NoPartialResponse checks how vmselect handles some
// vmstorage nodes being unavailable.
//
// By default in such cases, vmselect must mark responses as partial. However,
// passing -replicationFactor=N command-line flag to vmselect instructs it to
// not mark responses as partial if less than -replicationFactor vmstorage
// nodes are unavailable during the query.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_NoPartialResponse(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		replicationFactor = 2
		vmstorageCount    = 2*replicationFactor + 1
	)

	vmstorages := make([]*at.Vmstorage, vmstorageCount)
	vminsertAddrs := make([]string, vmstorageCount)
	vmselectAddrs := make([]string, vmstorageCount)
	for i := range vmstorageCount {
		instance := fmt.Sprintf("vmstorage-%d", i)
		vmstorages[i] = tc.MustStartVmstorage(instance, []string{
			"-storageDataPath=" + tc.Dir() + "/" + instance,
			"-retentionPeriod=100y",
		})
		vminsertAddrs[i] = vmstorages[i].VminsertAddr()
		vmselectAddrs[i] = vmstorages[i].VmselectAddr()
	}

	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + strings.Join(vminsertAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
	})
	vmselectRF := tc.MustStartVmselect("vmselect-rf", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(vmstorages...)

	// Verify partial vs full response.

	assertSeries := func(app *at.Vmselect, wantPartial bool) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{}).Sort()
			},
			Want: &at.PrometheusAPIV1SeriesResponse{
				Status:    "success",
				IsPartial: wantPartial,
			},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(apptest.PrometheusAPIV1SeriesResponse{}, "Data"),
			},
		})
	}

	mustReturnPartialResponse := true
	mustReturnFullResponse := false

	// All vmstorage replicates are available so both vmselects must return full
	// response.
	assertSeries(vmselect, mustReturnFullResponse)
	assertSeries(vmselectRF, mustReturnFullResponse)

	// Stop replicationFactor-1 vmstorage nodes.
	// vmselect is not aware about the replication factor and therefore must
	// return partial response.
	// vmselectRF is aware about the replication factor and therefore it knows
	// that the remaining vmstorage nodes must still be able to provide full
	// response.
	for i := range replicationFactor - 1 {
		tc.StopApp(vmstorages[i])
	}
	assertSeries(vmselect, mustReturnPartialResponse)
	assertSeries(vmselectRF, mustReturnFullResponse)

	// Stop one more vmstorage. At this point the remaining vmstorage nodes are
	// not enough to provide the full dataset. Therefore both vmselects must
	// return partial response.
	tc.StopApp(vmstorages[replicationFactor])
	assertSeries(vmselect, mustReturnPartialResponse)
	assertSeries(vmselectRF, mustReturnPartialResponse)
}

// See: https://docs.victoriametrics.com/cluster-victoriametrics/#vmstorage-groups-at-vmselect
func TestClusterReplicationGroups(t *testing.T) {
	t.Skip("not implemented")
}
