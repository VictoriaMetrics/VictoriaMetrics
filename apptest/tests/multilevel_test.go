package tests

import (
	"fmt"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// See: https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multi-level-cluster-setup
func TestClusterMultilevelSelect(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Set up the following multi-level cluster configuration:
	//
	// vmselect (L2) -> vmselect (L1) -> vmstorage <- vminsert
	//
	// vminsert writes data into vmstorage.
	// vmselect (L2) reads that data via vmselect (L1).

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-storageNode=" + vmselectL1.ClusternativeListenAddr(),
	})

	// Insert 1000 unique time series.

	const numMetrics = 1000
	records := make([]string, numMetrics)
	want := &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      make([]map[string]string, numMetrics),
	}
	for i := range numMetrics {
		name := fmt.Sprintf("metric_%d", i)
		records[i] = fmt.Sprintf("%s %d", name, rand.IntN(1000))
		want.Data[i] = map[string]string{"__name__": name}
	}
	want.Sort()
	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	// Retrieve all time series and verify that both vmselect (L1) and
	// vmselect (L2) serve the complete set of time series.

	assertSeries := func(app *apptest.Vmselect) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				res := app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, qopts)
				res.Sort()
				return res
			},
			Want: want,
		})
	}
	assertSeries(vmselectL1)
	assertSeries(vmselectL2)
}

// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10678.
func TestClusterMultilevelPartialResponse(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	// Set up the following multi-level cluster configuration:
	//
	//				  			     				|--> available vmstorage
	//					 	  |	------> vmselect1 --|
	//						  |	     				|--> available vmstorage
	// global-vmselect -------|
	//				  		  |	     				|--> available vmstorage
	// 					 	  |	------> vmselect2 --|
	//						  	     				|--> unavailable vmstorage

	vmstorage1 := tc.MustStartVmstorage("vmstorage1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage1",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage2",
	})
	regionalVmselect1 := tc.MustStartVmselect("regional-vmselect1", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	})
	regionalVmselect2 := tc.MustStartVmselect("regional-vmselect2", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + noopTCPServerAddr(t),
	})
	globalVmselect := tc.MustStartVmselect("global-vmselect", []string{
		"-storageNode=" + regionalVmselect1.ClusternativeListenAddr() + "," + regionalVmselect2.ClusternativeListenAddr(),
	})

	// 1. /api/v1/query
	qopts := apptest.QueryOpts{Tenant: "0"}
	assertQuery := func(app *apptest.Vmselect, want *apptest.PrometheusAPIV1QueryResponse) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				res := app.PrometheusAPIV1Query(t, `{__name__=~".*"}`, qopts)
				res.Sort()
				return res
			},
			Want: want,
		})
	}
	// regional-vmselect1 should return full response.
	assertQuery(regionalVmselect1, &apptest.PrometheusAPIV1QueryResponse{
		Status:    "success",
		IsPartial: false,
		Data:      &apptest.QueryData{ResultType: "vector", Result: []*apptest.QueryResult{}},
	})
	// regional-vmselect2 should return partial response.
	assertQuery(regionalVmselect2, &apptest.PrometheusAPIV1QueryResponse{
		Status:    "success",
		IsPartial: true,
		Data:      &apptest.QueryData{ResultType: "vector", Result: []*apptest.QueryResult{}},
	})
	// global-vmselect should return partial response.
	assertQuery(globalVmselect, &apptest.PrometheusAPIV1QueryResponse{
		Status:    "success",
		IsPartial: true,
		Data:      &apptest.QueryData{ResultType: "vector", Result: []*apptest.QueryResult{}},
	})

	// 2. /api/v1/labels
	start := time.Now().Unix()
	assertLabel := func(app *apptest.Vmselect, want *apptest.PrometheusAPIV1LabelsResponse) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/label response",

			Got: func() any {
				res := app.PrometheusAPIV1Labels(t, `{__name__="up"}`, apptest.QueryOpts{
					Start: fmt.Sprintf("%d", start-100),
					End:   fmt.Sprintf("%d", start),
				})
				return res
			},
			Want: want,
		})
	}

	// regional-vmselect1 should return full response.
	assertLabel(regionalVmselect1, &apptest.PrometheusAPIV1LabelsResponse{
		Status:    "success",
		IsPartial: false,
		Data:      make([]string, 0),
	})
	// regional-vmselect2 should return partial response.
	assertLabel(regionalVmselect2, &apptest.PrometheusAPIV1LabelsResponse{
		Status:    "success",
		IsPartial: true,
		Data:      make([]string, 0),
	})
	// global-vmselect should return partial response.
	assertLabel(globalVmselect, &apptest.PrometheusAPIV1LabelsResponse{
		Status:    "success",
		IsPartial: true,
		Data:      make([]string, 0),
	})

	// 3. /api/v1/label/%s/values
	assertSeries := func(app *apptest.Vmselect, want *apptest.PrometheusAPIV1SeriesResponse) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/series response",

			Got: func() any {
				res := app.PrometheusAPIV1Series(t, `{__name__="up"}`, apptest.QueryOpts{
					Start: fmt.Sprintf("%d", start-100),
					End:   fmt.Sprintf("%d", start),
				})
				return res
			},
			Want: want,
		})
	}

	// regional-vmselect1 should return full response.
	assertSeries(regionalVmselect1, &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      make([]map[string]string, 0),
	})
	// regional-vmselect2 should return partial response.
	assertSeries(regionalVmselect2, &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: true,
		Data:      make([]map[string]string, 0),
	})
	// global-vmselect should return partial response.
	assertSeries(globalVmselect, &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: true,
		Data:      make([]map[string]string, 0),
	})
}

// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11351.
func TestClusterMultilevelStorageNodeLabelIndex(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Set up the following multi-level cluster configuration:
	//
	//                  |--> vmselect-eu --> vmstorage-eu <-- vminsert-eu
	// global-vmselect -|
	//                  |--> unavailable vmselect-us
	//
	// global-vmselect must return full responses for queries, which select only EU series.

	vmstorageEU := tc.MustStartVmstorage("vmstorage-eu", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-eu",
	})
	vminsertEU := tc.MustStartVminsert("vminsert-eu", []string{
		"-storageNode=" + vmstorageEU.VminsertAddr(),
	})
	vmselectEU := tc.MustStartVmselect("vmselect-eu", []string{
		"-storageNode=" + vmstorageEU.VmselectAddr(),
	})
	globalVmselect := tc.MustStartVmselect("global-vmselect", []string{
		"-storageNode=" + vmselectEU.ClusternativeListenAddr() + "," + noopTCPServerAddr(t),
		"-storageNodeLabelIndex=region=eu-1,region=us-east^^region=us-west",
	})

	ts := time.Now().Add(-time.Minute).Unix() * 1000
	qopts := apptest.QueryOpts{
		Tenant: "0",
		Time:   fmt.Sprintf("%d", ts/1000),
	}
	vminsertEU.PrometheusAPIV1ImportPrometheus(t, []string{fmt.Sprintf(`up{region="eu-1"} 1 %d`, ts)}, qopts)
	vmstorageEU.ForceFlush(t)

	assertQuery := func(query string, isPartial bool) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return globalVmselect.PrometheusAPIV1Query(t, query, qopts)
			},
			Want: &apptest.PrometheusAPIV1QueryResponse{
				Status:    "success",
				IsPartial: isPartial,
				Data: &apptest.QueryData{
					ResultType: "vector",
					Result: []*apptest.QueryResult{{
						Metric: map[string]string{"__name__": "up", "region": "eu-1"},
						Sample: &apptest.Sample{Timestamp: ts, Value: 1},
					}},
				},
			},
		})
	}
	// vmselect-us is skipped, since its index cannot match the query.
	assertQuery(`up{region="eu-1"}`, false)
	assertQuery(`up{region=~"eu-.*"}`, false)
	// vmselect-us is queried, since its index may match the query.
	assertQuery(`up`, true)
	assertQuery(`up{region!="us-east"}`, true)

	wantSeries := &apptest.PrometheusAPIV1SeriesResponse{
		Status: "success",
		Data:   []map[string]string{{"__name__": "up", "region": "eu-1"}},
	}
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/series response",
		Got: func() any {
			return globalVmselect.PrometheusAPIV1Series(t, `{region="eu-1"}`, qopts)
		},
		Want: wantSeries,
	})

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/labels response",
		Got: func() any {
			return globalVmselect.PrometheusAPIV1Labels(t, `{region="eu-1"}`, qopts)
		},
		Want: &apptest.PrometheusAPIV1LabelsResponse{
			Status: "success",
			Data:   []string{"__name__", "region"},
		},
	})

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/label/region/values response",
		Got: func() any {
			return globalVmselect.PrometheusAPIV1LabelValues(t, "region", `{region="eu-1"}`, qopts)
		},
		Want: &apptest.PrometheusAPIV1LabelValuesResponse{
			Status: "success",
			Data:   []string{"eu-1"},
		},
	})
}

// noopTCPServerAddr start local tcp server,
// which immediately closes any incoming connections
// and return it's address
func noopTCPServerAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}
