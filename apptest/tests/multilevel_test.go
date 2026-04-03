package tests

import (
	"fmt"
	"math/rand/v2"
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
	//						  |	     				|--> unavailable vmstorage
	// global-vmselect -------|
	//				  		  |	     				|--> available vmstorage
	// 					 	  |	------> vmselect2 --|
	//						  	     				|--> available vmstorage

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	regionalVmselect1 := tc.MustStartVmselect("regional-vmselect1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	regionalVmselect2 := tc.MustStartVmselect("regional-vmselect2", []string{
		"-storageNode=" + vmstorage.VmselectAddr() + ",1.1.1.1:1111",
	})
	globalVmselect := tc.MustStartVmselect("global-vmselect", []string{
		"-storageNode=" + regionalVmselect1.ClusternativeListenAddr() + "," + regionalVmselect2.ClusternativeListenAddr(),
	})

	// 1. /api/v1/query
	queryWant := &apptest.PrometheusAPIV1QueryResponse{
		Status:    "success",
		IsPartial: false,
		Data:      &apptest.QueryData{ResultType: "vector", Result: []*apptest.QueryResult{}},
	}
	queryWant.Sort()
	qopts := apptest.QueryOpts{Tenant: "0"}
	assertQuery := func(app *apptest.Vmselect) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				res := app.PrometheusAPIV1Query(t, `{__name__=~".*"}`, qopts)
				res.Sort()
				return res
			},
			Want: queryWant,
		})
	}
	// regional-vmselect1 should return full response.
	assertQuery(regionalVmselect1)
	queryWant.IsPartial = true
	// regional-vmselect2 should return partial response.
	assertQuery(regionalVmselect2)
	// global-vmselect should return partial response.
	assertQuery(globalVmselect)

	// 2. /api/v1/labels
	labelWant := &apptest.PrometheusAPIV1LabelsResponse{
		Status:    "success",
		IsPartial: false,
		Data:      make([]string, 0),
	}
	start := time.Now().Unix()
	assertLabel := func(app *apptest.Vmselect) {
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
			Want: labelWant,
		})
	}

	// regional-vmselect1 should return full response.
	assertLabel(regionalVmselect1)
	labelWant.IsPartial = true
	// regional-vmselect2 should return partial response.
	assertLabel(regionalVmselect2)
	// global-vmselect should return partial response.
	assertLabel(globalVmselect)

	// 3. /api/v1/label/%s/values
	seriesWant := &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      make([]map[string]string, 0),
	}
	assertSeries := func(app *apptest.Vmselect) {
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
			Want: seriesWant,
		})
	}

	// regional-vmselect1 should return full response.
	assertSeries(regionalVmselect1)
	seriesWant.IsPartial = true
	// regional-vmselect2 should return partial response.
	assertSeries(regionalVmselect2)
	// global-vmselect should return partial response.
	assertSeries(globalVmselect)
}
