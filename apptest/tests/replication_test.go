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

type clusterWithReplication struct {
	vmstorages     []*at.Vmstorage
	vminsert       *at.Vminsert
	vmselect       *at.Vmselect
	vmselectDedup  *at.Vmselect
	vmselectRF     *at.Vmselect
	vmselectRFSkip *at.Vmselect
}

func newClusterWithReplication(tc *at.TestCase, replicationFactor int) *clusterWithReplication {
	tc.T().Helper()

	c := &clusterWithReplication{}

	vmstorageCount := 2*replicationFactor + 1

	c.vmstorages = make([]*at.Vmstorage, vmstorageCount)
	vminsertAddrs := make([]string, vmstorageCount)
	vmselectAddrs := make([]string, vmstorageCount)
	for i := range vmstorageCount {
		instance := fmt.Sprintf("vmstorage-%d", i)
		c.vmstorages[i] = tc.MustStartVmstorage(instance, []string{
			"-storageDataPath=" + tc.Dir() + "/" + instance,
			"-retentionPeriod=100y",
		})
		vminsertAddrs[i] = c.vmstorages[i].VminsertAddr()
		vmselectAddrs[i] = c.vmstorages[i].VmselectAddr()
	}

	c.vminsert = tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + strings.Join(vminsertAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})

	// An instace of vmselect that knows nothing about data replication.
	c.vmselect = tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
	})

	// An instance of vmselect that deduplicates data retrieved from the
	// storage.
	c.vmselectDedup = tc.MustStartVmselect("vmselect-dedup", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-dedup.minScrapeInterval=1ms",
	})

	// An instance of vmselect that knows about the data replication factor.
	c.vmselectRF = tc.MustStartVmselect("vmselect-rf", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
	})

	// An instance of vmselect that knows about the data replication factor
	// and skips slow replicas.
	c.vmselectRFSkip = tc.MustStartVmselect("vmselect-rf-skip", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
		"-search.skipSlowReplicas",
	})

	return c
}

// TestClusterReplication_DataIsWrittenSeveralTimes verifies that with
// replication enabled, several copies of data are stored in the database.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_DataIsWrittenSeveralTimes(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const replicationFactor = 2
	c := newClusterWithReplication(tc, replicationFactor)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(c.vmstorages...)

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

	cnts := make([]int, len(c.vmstorages))
	var got int
	for i, vmstorage := range c.vmstorages {
		cnts[i] = getMetricsReadTotal(vmstorage)
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
func TestClusterReplication_Deduplication(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const replicationFactor = 2
	c := newClusterWithReplication(tc, replicationFactor)

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
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(c.vmstorages...)

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
	assertSeries(c.vmselect)
	assertSeries(c.vmselectDedup)

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
	assertQuery(c.vmselect)
	assertQuery(c.vmselectDedup)

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
	assertQueryRangeVector(c.vmselect, replicationFactor)
	assertQueryRangeVector(c.vmselectDedup, 1)

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
	assertQueryRange(c.vmselect)
	assertQueryRange(c.vmselectDedup)

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
	assertExport(c.vmselect, replicationFactor)
	assertExport(c.vmselectDedup, 1)
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
func TestClusterReplication_PartialResponse(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const replicationFactor = 2
	c := newClusterWithReplication(tc, replicationFactor)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(c.vmstorages...)

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

	// All vmstorage replicas are available so both vmselects must return full
	// response.
	assertSeries(c.vmselect, mustReturnFullResponse)
	assertSeries(c.vmselectRF, mustReturnFullResponse)

	// Stop replicationFactor-1 vmstorage nodes.
	// vmselect is not aware about the replication factor and therefore must
	// return partial response.
	// vmselectRF is aware about the replication factor and therefore it knows
	// that the remaining vmstorage nodes must still be able to provide full
	// response.
	for i := range replicationFactor - 1 {
		tc.StopApp(c.vmstorages[i].Name())
	}
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectRF, mustReturnFullResponse)

	// Stop one more vmstorage. At this point the remaining vmstorage nodes are
	// not enough to provide the full dataset. Therefore both vmselects must
	// return partial response.
	tc.StopApp(c.vmstorages[replicationFactor].Name())
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectRF, mustReturnPartialResponse)
}

// TestClusterReplication_SkipSlowReplicas checks that vmselect skips the
// results from slower replicas if the results that have been received from
// other replicas are enough to construct full response.
//
// By default, even if a vmselect knows about the vmstorage replication (via
// -replicationFactor flag) it will still wait for results from all the
// vmstorage nodes. A vmselect can be configured to skip slow replicas
// using -search.skipSlowReplicas flag.
//
// Say a vmselect points to N vmstorage nodes and its -replicationFactor is R.
// Then only R nodes out of N will contain the searched data, while N-R node
// will not contain it. Therefore the vmselect must receive responses from at
// least N-R+1 nodes to construct the full response. The responses from the rest
// of the nodes (R-1) can be skipped.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_SkipSlowReplicas(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const replicationFactor = 2
	c := newClusterWithReplication(tc, replicationFactor)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	wantSeries := &at.PrometheusAPIV1SeriesResponse{
		Status: "success",
		Data:   make([]map[string]string, numRecs),
	}
	for i := range numRecs {
		name := fmt.Sprintf("metric_%d", i)
		recs[i] = fmt.Sprintf("%s %d", name, rand.IntN(1000))
		wantSeries.Data[i] = map[string]string{"__name__": name}
	}
	wantSeries.Sort()
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	tc.ForceFlush(c.vmstorages...)

	// Verify skipping slow replicas by counting the number of skipSlowReplicas
	// messages in request trace.

	assertSeries := func(app *at.Vmselect, want int) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{}).Sort()
			},
			Want: wantSeries,
		})

		res := app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{Trace: "1"})
		got := res.Trace.Contains("cancel request because -search.skipSlowReplicas is set and every group returned the needed number of responses according to replicationFactor")
		if got != want {
			t.Errorf("unexpected number of skipSlowReplicas messages in request trace: got %d, want %d (full trace:\n%v)", got, want, res.Trace)
		}

	}
	assertSeries(c.vmselectRF, 0)
	assertSeries(c.vmselectRFSkip, replicationFactor-1)
}

type storageGroup struct {
	vmstorages []*at.Vmstorage
	vminsert   *at.Vminsert
}

func (g *storageGroup) stopNodes(tc *at.TestCase, n int) {
	if n > len(g.vmstorages) {
		n = len(g.vmstorages)
	}
	for i := range n {
		tc.StopApp(g.vmstorages[i].Name())
	}
	g.vmstorages = g.vmstorages[n:]
}

type clusterWithGroupReplication struct {
	storageGroups             []*storageGroup
	vminsert                  *at.Vminsert
	vmselect                  *at.Vmselect
	vmselectDedup             *at.Vmselect
	vmselectGroupRF           *at.Vmselect
	vmselectGlobalRF          *at.Vmselect
	vmselectGroupGlobalRF     *at.Vmselect
	vmselectGroupRFSkip       *at.Vmselect
	vmselectGlobalRFSkip      *at.Vmselect
	vmselectGroupGlobalRFSkip *at.Vmselect
}

func (c *clusterWithGroupReplication) forceFlush(tc *at.TestCase) {
	for _, g := range c.storageGroups {
		tc.ForceFlush(g.vmstorages...)
	}
}

func newClusterWithGroupReplication(tc *at.TestCase, groupRFs []int, globalRF int) *clusterWithGroupReplication {
	tc.T().Helper()

	if len(groupRFs) < 1 {
		tc.T().Fatalf("group count must be > 0")
	}
	for _, rf := range groupRFs {
		if rf < 0 {
			tc.T().Fatalf("group replication factors must be > 0: %v", groupRFs)
		}
	}

	c := &clusterWithGroupReplication{
		storageGroups: make([]*storageGroup, len(groupRFs)),
	}
	vminsertAddrs := make([]string, len(groupRFs))
	vmselectAddrs := make([]string, len(groupRFs))
	rfs := make([]string, len(groupRFs))

	for g, rf := range groupRFs {
		groupName := fmt.Sprintf("group%d", g)
		vmstorageCount := 2*rf + 1
		c.storageGroups[g] = &storageGroup{
			vmstorages: make([]*at.Vmstorage, vmstorageCount),
		}
		groupVminsertAddrs := make([]string, vmstorageCount)
		groupVmselectAddrs := make([]string, vmstorageCount)

		for s := range vmstorageCount {
			vmstorageInstance := fmt.Sprintf("vmstorage-%s-%d", groupName, s)
			c.storageGroups[g].vmstorages[s] = tc.MustStartVmstorage(vmstorageInstance, []string{
				"-storageDataPath=" + tc.Dir() + "/" + vmstorageInstance,
				"-retentionPeriod=100y",
			})
			groupVminsertAddrs[s] = c.storageGroups[g].vmstorages[s].VminsertAddr()
			groupVmselectAddrs[s] = groupName + "/" + c.storageGroups[g].vmstorages[s].VmselectAddr()
		}

		vminsertInstance := fmt.Sprintf("vminsert-%s", groupName)
		c.storageGroups[g].vminsert = tc.MustStartVminsert(vminsertInstance, []string{
			"-storageNode=" + strings.Join(groupVminsertAddrs, ","),
			fmt.Sprintf("-replicationFactor=%d", rf),
		})
		vminsertAddrs[g] = c.storageGroups[g].vminsert.ClusternativeListenAddr()
		vmselectAddrs[g] = strings.Join(groupVmselectAddrs, ",")
		rfs[g] = fmt.Sprintf("%s:%d", groupName, rf)
	}
	c.vminsert = tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + strings.Join(vminsertAddrs, ","),
		fmt.Sprintf("-replicationFactor=%d", globalRF),
	})

	// An instace of vmselect that knows nothing about data replication.
	c.vmselect = tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
	})

	// An instance of vmselect that deduplicates data retrieved from the
	// storage.
	c.vmselectDedup = tc.MustStartVmselect("vmselect-dedup", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-dedup.minScrapeInterval=1ms",
	})

	// An instance of vmselect that knows about group replication factor.
	c.vmselectGroupRF = tc.MustStartVmselect("vmselect-group-rf", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-replicationFactor=" + strings.Join(rfs, ","),
	})

	// An instance of vmselect that knows about global replication factor.
	c.vmselectGlobalRF = tc.MustStartVmselect("vmselect-global-rf", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-globalReplicationFactor=%d", globalRF),
	})

	// An instance of vmselect that knows about global and group replication
	// factor.
	c.vmselectGroupGlobalRF = tc.MustStartVmselect("vmselect-group-global-rf", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-replicationFactor=" + strings.Join(rfs, ","),
		fmt.Sprintf("-globalReplicationFactor=%d", globalRF),
	})

	// An instance of vmselect that knows about group replication factor and
	// skips slow replicas within the group.
	c.vmselectGroupRFSkip = tc.MustStartVmselect("vmselect-group-rf-skip", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		"-replicationFactor=" + strings.Join(rfs, ","),
		"-search.skipSlowReplicas",
	})

	// An instance of vmselect that knows about global replication factor and
	// skips slow groups.
	c.vmselectGlobalRFSkip = tc.MustStartVmselect("vmselect-global-rf-skip", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-globalReplicationFactor=%d", globalRF),
		"-search.skipSlowReplicas",
	})

	// An instance of vmselect that knows about group and global replication
	// factor and skips slow groups and slow replicas within a group.
	c.vmselectGroupGlobalRFSkip = tc.MustStartVmselect("vmselect-group-global-rf-skip", []string{
		"-storageNode=" + strings.Join(vmselectAddrs, ","),
		fmt.Sprintf("-globalReplicationFactor=%d", globalRF),
		"-replicationFactor=" + strings.Join(rfs, ","),
		"-search.skipSlowReplicas",
	})

	return c
}

// TestClusterGroupReplication_DataIsWrittenSeveralTimes verifies that with
// global and group replication enabled, several copies of data are stored in
// the database.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#vmstorage-groups-at-vmselect
// and https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterGroupReplication_DataIsWrittenSeveralTimes(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const globalRF = 2
	groupRFs := []int{1, 2, 3}
	c := newClusterWithGroupReplication(tc, groupRFs, globalRF)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	c.forceFlush(tc)

	// Verify that each strorage node has metrics and that total metric count across
	// all vmstorages is at least globalRF*numRecs. It is impossible to estimate the
	// exact number in general due to uneven distribution of metrics across replicas
	// and different RF within storage groups.

	getMetricsReadTotal := func(app *at.Vmstorage) int {
		t.Helper()
		got := app.GetIntMetric(t, "vm_vminsert_metrics_read_total")
		if got <= 0 {
			t.Fatalf("%s unexpected metric count: got %d, want > 0", app.Name(), got)
		}
		return got
	}

	cnts := make([][]int, len(c.storageGroups))
	total := 0
	for i, g := range c.storageGroups {
		cnts[i] = make([]int, len(g.vmstorages))
		groupCnt := 0
		for j, s := range g.vmstorages {
			cnt := getMetricsReadTotal(s)
			cnts[i][j] = cnt
			groupCnt += cnt
		}
		total += groupCnt / groupRFs[i]
	}
	if got, want := total, globalRF*numRecs; got != want {
		t.Fatalf("unxepected metric count: got %d, want %d (counts per group per replica: %v)", got, want, cnts)
	}
}

// TestClusterGroupReplication_VmselectDeduplication checks now vmselect behaves
// when the data is replicated across storage groups and within each group.
//
// When the data is replicated, vmselect's netstorage will receive duplicates.
// It can be instructed to remove duplicates by setting -dedup.minScrapeInterval
// flag. See mergeSortBlocks() in app/vmselect/netstorage/netstorage.go.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#vmstorage-groups-at-vmselect
// and https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterGroupReplication_Deduplication(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		globalRF = 2
		groupRF  = 2
	)

	groupRFs := []int{groupRF, groupRF, groupRF}
	c := newClusterWithGroupReplication(tc, groupRFs, globalRF)

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
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	c.forceFlush(tc)

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
	assertSeries(c.vmselect)
	assertSeries(c.vmselectDedup)

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
	assertQuery(c.vmselect)
	assertQuery(c.vmselectDedup)

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
	assertQueryRangeVector(c.vmselect, groupRF*globalRF)
	assertQueryRangeVector(c.vmselectDedup, 1)

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
	assertQueryRange(c.vmselect)
	assertQueryRange(c.vmselectDedup)

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
	assertExport(c.vmselect, groupRF*globalRF)
	assertExport(c.vmselectDedup, 1)
}

// TestClusterGroupReplication_NoPartialResponse checks how vmselect handles
// some vmstorage nodes being unavailable.
//
// By default in such cases, vmselect must mark responses as partial. However,
// passing -replicationFactor=N and -globalReplicationFactor command-line flag
// to vmselect instructs it to not mark responses as partial even if less
// vmstorage nodes are unavailable during the query.
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#vmstorage-groups-at-vmselect
// and https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterGroupReplication_PartialResponse(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		globalRF  = 2
		numGroups = 2*globalRF + 1
		groupRF   = 2
	)

	groupRFs := make([]int, numGroups)
	for i := range numGroups {
		groupRFs[i] = groupRF
	}
	c := newClusterWithGroupReplication(tc, groupRFs, globalRF)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	for i := range numRecs {
		recs[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	c.forceFlush(tc)

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

	// All vmstorage replicas are available so both vmselects must return full
	// response.
	assertSeries(c.vmselect, mustReturnFullResponse)
	assertSeries(c.vmselectGroupRF, mustReturnFullResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnFullResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnFullResponse)

	// Stop groupRF-1 vmstorage nodes in first group.
	//
	// vmselect is not aware about the replication factor and therefore must
	// return partial response.
	// vmselectGroupRF, vmselectGlobalRF, and vmselectGroupGlobalRF are aware
	// about the replication factor and therefore they must still be able to
	// return full dataset.
	c.storageGroups[0].stopNodes(tc, groupRF-1)
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupRF, mustReturnFullResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnFullResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnFullResponse)

	// Stop groupRF-1 vmstorages in the remaining groups.
	//
	// vmselectGroupRF and vmselectGroupGlobalRF are still capable of returning
	// full dataset, while vmselectGlobalRF will start returning partial dataset
	// because it is unaware of replication within groups.
	for g := 1; g < len(c.storageGroups); g++ {
		c.storageGroups[g].stopNodes(tc, groupRF-1)
	}
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupRF, mustReturnFullResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnFullResponse)

	// Stop one more vmstorage in the first group.
	//
	// At this point vmselectGroupRF will start returning partial dataset
	// because it is unaware of replication across groups. vmselectGroupGlobalRF
	// will continue retuning full dataset.
	c.storageGroups[0].stopNodes(tc, 1)
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnFullResponse)

	// Stop one more vmstoarge in remaining globarRF-1 groups.
	//
	// This is the extreme case when vmselectGroupGlobalRF is still able to
	// return full dataset.
	for g := 1; g < globalRF-1; g++ {
		c.storageGroups[g].stopNodes(tc, 1)
	}
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnFullResponse)

	// Stop one more vmstoarge in one more group.
	//
	// vmselectGroupGlobalRF must now return partial dataset.
	c.storageGroups[globalRF].stopNodes(tc, 1)
	assertSeries(c.vmselect, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGlobalRF, mustReturnPartialResponse)
	assertSeries(c.vmselectGroupGlobalRF, mustReturnPartialResponse)
}

// TestClusterReplication_SkipSlowReplicas checks that vmselect skips the
// results from slower replicas within and across storage groups if the results
// that have been received from other replicas/group are enough to construct
// full response.
//
// By default, even if a vmselect knows about the vmstorage replication (via
// -replicationFactor and -globalReplicationFactor flags) it will still wait for
// results from all the vmstorage nodes. A vmselect can be configured to skip
// slow replicas using -search.skipSlowReplicas flag.
//
// Say a vmselect points to M groups with N vmstorage nodes each, its
// -globalReplicationFactor is Rm and the -replicationFactor of each group is Rn.
// Then only Rm out of M groups will contain the searched data while M-Rm groups
// will not contain it. Therefore the vmselect must receive responses from at
// least M-Rm+1 nodes to construct the full response. The responses from the rest
// of the group (Rm-1) can be skipped. The same with the nodes within each of Rm
// groups: Rn-1 nodes can be skipped. The total number of nodes to skip is:
// (Rm-1)*N + (M-Rm+1)*(Rn-1)
//
// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
// and https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterGroupReplication_SkipSlowReplicas(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		globalRF  = 2
		numGroups = 2*globalRF + 1
		groupRF   = 3
		numNodes  = 2*groupRF + 1
	)

	groupRFs := make([]int, numGroups)
	for i := range numGroups {
		groupRFs[i] = groupRF
	}
	c := newClusterWithGroupReplication(tc, groupRFs, globalRF)

	// Insert data.

	const numRecs = 1000
	recs := make([]string, numRecs)
	wantSeries := &at.PrometheusAPIV1SeriesResponse{
		Status: "success",
		Data:   make([]map[string]string, numRecs),
	}
	for i := range numRecs {
		name := fmt.Sprintf("metric_%d", i)
		recs[i] = fmt.Sprintf("%s %d", name, rand.IntN(1000))
		wantSeries.Data[i] = map[string]string{"__name__": name}
	}
	wantSeries.Sort()
	c.vminsert.PrometheusAPIV1ImportPrometheus(t, recs, at.QueryOpts{})
	c.forceFlush(tc)

	// Verify skipping slow replicas by counting the number of skipSlowReplicas
	// messages in request trace.

	assertSeries := func(app *at.Vmselect, want int) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{}).Sort()
			},
			Want: wantSeries,
		})

		res := app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{Trace: "1"})
		got := res.Trace.Contains("cancel request because -search.skipSlowReplicas is set and every group returned the needed number of responses according to replicationFactor")
		if got != want {
			t.Errorf("unexpected number of skipSlowReplicas messages in request trace: got %d, want %d (full trace:\n%v)", got, want, res.Trace)
		}

	}
	assertSeries(c.vmselectGroupRF, 0)
	assertSeries(c.vmselectGlobalRF, 0)
	assertSeries(c.vmselectGroupGlobalRF, 0)
	// assertSeries(c.vmselectGroupRFSkip, numGroups*(groupRF-1))
	// assertSeries(c.vmselectGlobalRFSkip, numNodes*(globalRF-1))
	// assertSeries(c.vmselectGroupGlobalRFSkip, numNodes*(globalRF-1))
}
