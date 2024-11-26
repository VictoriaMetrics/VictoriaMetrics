package tests

import (
	"fmt"
	"testing"
	"time"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// See: https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety
func TestClusterReplication_Deduplication(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	// Set up the following cluster configuration:
	//
	// - 3 vmstorages
	// - 1 vminsert points to 3 vmstorages. Its -replicationFactor is 2.
	//   With this configuration, vminsert will shard data across 3 vmstorages
	//   and each shard will be written to 2 vmstorages.
	// - vmselect points to 3 vmstorages.
	// - vmselectDedup points to 3 vmstorages. Its -dedup.minScrapeInterval is
	//   1ms which allows to ignore duplicates caused by replication.

	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
		"-retentionPeriod=100y",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
		"-retentionPeriod=100y",
	})
	vmstorage3 := tc.MustStartVmstorage("vmstorage-3", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-3",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr() + "," + vmstorage3.VminsertAddr(),
		"-replicationFactor=2",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr() + "," + vmstorage3.VmselectAddr(),
	})
	vmselectDedup := tc.MustStartVmselect("vmselect-dedup", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr() + "," + vmstorage3.VmselectAddr(),
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
	qopts := at.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, recs, qopts)
	tc.ForceFlush(vmstorage1, vmstorage2, vmstorage3)

	// Verify that each strorage node has metrics and that metric count across
	// all vmstorages is 2*numRecs.

	cnt1 := vmstorage1.GetIntMetricGt(t, "vm_vminsert_metrics_read_total", 0)
	cnt2 := vmstorage2.GetIntMetricGt(t, "vm_vminsert_metrics_read_total", 0)
	cnt3 := vmstorage3.GetIntMetricGt(t, "vm_vminsert_metrics_read_total", 0)
	if cnt1+cnt2+cnt3 != 2*numRecs {
		t.Fatalf("unxepected metric count: %d+%d+%d != 2*%d", cnt1, cnt2, cnt3, numRecs)
	}

	// Check /api/v1/series response.
	// vmselect is expected to return no duplicates regardless whether
	// -dedup.minScrapeInterval is set or not.

	assertSeries := func(app *at.Vmselect) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, "2024-01-01T00:00:00Z", "2024-01-31T00:00:00Z", qopts).Sort()
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
	// For queries that do not return range vector, vmselect is expected to
	// return no duplicates regardless whether -dedup.minScrapeInterval is
	// set or not:
	// - When -dedup.minScrapeInterval isn't set, the vmselect's netstorage will
	//   receive duplicates and will do nothing about them. However, the promql
	//   code will remove duplicates.
	// - When -dedup.minScrapeInterval is set, the vmselect's netstorage will
	//   perform the deduplication. See mergeSortBlocks() in
	//   app/vmselect/netstorage/netstorage.go

	assertQuery := func(app *at.Vmselect) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return app.PrometheusAPIV1Query(t, "metric_1", "2024-01-01T00:05:00Z", "5m", qopts)
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
	// return duplicates when -dedup.minScrapeInterval is
	// set or not:
	// - When -dedup.minScrapeInterval isn't set, the vmselect's netstorage will
	//   receive duplicates and will do nothing about them. Queries that return
	//   range vector are handled by the same code as export queries. Export
	//   queries return raw data, therefore no post-processing will be done and
	//   the duplicates will be returned to the caller. See QueryHandler() in
	//   app/vmselect/prometheus/prometheus.go.
	// - When -dedup.minScrapeInterval is set, the vmselect's netstorage will
	//   perform the deduplication. See mergeSortBlocks() in
	//   app/vmselect/netstorage/netstorage.go

	assertQueryRangeVector := func(app *at.Vmselect, want []*at.Sample) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return app.PrometheusAPIV1Query(t, "metric_1[5m]", "2024-01-01T00:05:00Z", "5m", qopts)
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result: []*at.QueryResult{
						{
							Metric:  map[string]string{"__name__": "metric_1"},
							Samples: want,
						},
					},
				},
			},
		})
	}
	assertQueryRangeVector(vmselect, []*at.Sample{
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
		at.NewSample(t, "2024-01-01T00:04:00Z", 4),
		at.NewSample(t, "2024-01-01T00:04:00Z", 4),
		at.NewSample(t, "2024-01-01T00:05:00Z", 5),
		at.NewSample(t, "2024-01-01T00:05:00Z", 5),
	})
	assertQueryRangeVector(vmselectDedup, []*at.Sample{
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
		at.NewSample(t, "2024-01-01T00:04:00Z", 4),
		at.NewSample(t, "2024-01-01T00:05:00Z", 5),
	})

	// Check /api/v1/query_range response.
	//
	// For range queries, vmselect is expected to return no duplicates
	// regardless whether -dedup.minScrapeInterval is set or not:
	// - When -dedup.minScrapeInterval isn't set, the vmselect's netstorage will
	//   receive duplicates and will do nothing about them. However, the promql
	//   code will remove duplicates.
	// - When -dedup.minScrapeInterval is set, the vmselect's netstorage will
	//   perform the deduplication. See mergeSortBlocks() in
	//   app/vmselect/netstorage/netstorage.go

	assertQueryRange := func(app *at.Vmselect) {
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, "metric_1", "2024-01-01T00:00:00Z", "2024-01-01T00:10:00Z", "5m", qopts)
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
	// - When -dedup.minScrapeInterval isn't set, the vmselect's netstorage will
	//   receive duplicates and will do nothing about them. Export queries
	//   return raw data, therefore no post-processing will be done and the
	//   duplicates will be returned to the caller. See ExportHandler() in
	//   app/vmselect/prometheus/prometheus.go.
	// - When -dedup.minScrapeInterval is set, the vmselect's netstorage will
	//   perform the deduplication. See mergeSortBlocks() in
	//   app/vmselect/netstorage/netstorage.go

	assertExport := func(app *at.Vmselect, want []*at.Sample) {
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/export response",
			Got: func() any {
				return app.PrometheusAPIV1Export(t, `{__name__="metric_1"}`, "2024-01-01T00:00:00Z", "2024-01-01T00:03:00Z", qopts)
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result: []*at.QueryResult{
						{
							Metric:  map[string]string{"__name__": "metric_1"},
							Samples: want,
						},
					},
				},
			},
		})
	}
	assertExport(vmselect, []*at.Sample{
		at.NewSample(t, "2024-01-01T00:00:00Z", 0),
		at.NewSample(t, "2024-01-01T00:00:00Z", 0),
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
	})
	assertExport(vmselectDedup, []*at.Sample{
		at.NewSample(t, "2024-01-01T00:00:00Z", 0),
		at.NewSample(t, "2024-01-01T00:01:00Z", 1),
		at.NewSample(t, "2024-01-01T00:02:00Z", 2),
		at.NewSample(t, "2024-01-01T00:03:00Z", 3),
	})
}

// See: https://docs.victoriametrics.com/cluster-victoriametrics/#vmstorage-groups-at-vmselect
func TestClusterReplicationGroups(t *testing.T) {
	t.Skip("not implemented")
}
