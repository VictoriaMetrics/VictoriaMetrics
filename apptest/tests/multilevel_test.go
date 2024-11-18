package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

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

	// Insert 1000 unique time series.Wait for 2 seconds to let vmstorage
	// flush pending items so they become searchable.

	const numMetrics = 1000
	records := make([]string, numMetrics)
	for i := range numMetrics {
		records[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, apptest.QueryOpts{Tenant: "0"})
	time.Sleep(2 * time.Second)

	// Retrieve all time series and verify that vmselect (L1) serves the complete
	// set of time series.

	seriesL1 := vmselectL1.PrometheusAPIV1Series(t, `{__name__=~".*"}`, apptest.QueryOpts{Tenant: "0"})
	if got, want := len(seriesL1.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-1 series count: got %d, want %d", got, want)
	}

	// Retrieve all time series and verify that vmselect (L2) serves the complete
	// set of time series.

	seriesL2 := vmselectL2.PrometheusAPIV1Series(t, `{__name__=~".*"}`, apptest.QueryOpts{Tenant: "0"})
	if got, want := len(seriesL2.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-2 series count: got %d, want %d", got, want)
	}
}
