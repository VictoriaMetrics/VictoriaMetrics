package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// See: https://docs.victoriametrics.com/cluster-victoriametrics/#multi-level-cluster-setup
func TestClusterMultilevelSelect(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Set up the following multi-level cluster configuration:
	//
	// vmselect (L2) -> vmselect (L1) -> vmstorage <- vminsert
	//
	// vmisert writes data into vmstorage.
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
