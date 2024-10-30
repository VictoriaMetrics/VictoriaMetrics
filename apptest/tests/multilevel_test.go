package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestMultilevelSelect(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Close()

	// Set up the following multi-level cluster configuration:
	//
	// vmselect (L2) -> vmselect (L1) -> vmstorage <- vminsert
	//
	// vmisert writes data into vmstorage.
	// vmselect (L2) reads that data via vmselect (L1).

	cli := tc.Client()

	vmstorage := apptest.MustStartVmstorage(t, "vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	}, cli)
	defer vmstorage.Stop()
	vminsert := apptest.MustStartVminsert(t, "vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	}, cli)
	defer vminsert.Stop()
	vmselectL1 := apptest.MustStartVmselect(t, "vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	}, cli)
	defer vmselectL1.Stop()
	vmselectL2 := apptest.MustStartVmselect(t, "vmselect-level2", []string{
		"-storageNode=" + vmselectL1.ClusternativeListenAddr(),
	}, cli)
	defer vmselectL2.Stop()

	// Insert 1000 unique time series.Wait for 2 seconds to let vmstorage
	// flush pending items so they become searchable.

	const numMetrics = 1000
	records := make([]string, numMetrics)
	for i := range numMetrics {
		records[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.PrometheusAPIV1ImportPrometheus(t, "0", records)
	time.Sleep(2 * time.Second)

	// Retrieve all time series and verify that vmselect (L1) serves the complete
	// set of time series.

	seriesL1 := vmselectL1.PrometheusAPIV1Series(t, "0", `{__name__=~".*"}`)
	if got, want := len(seriesL1.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-1 series count: got %d, want %d", got, want)
	}

	// Retrieve all time series and verify that vmselect (L2) serves the complete
	// set of time series.

	seriesL2 := vmselectL2.PrometheusAPIV1Series(t, "0", `{__name__=~".*"}`)
	if got, want := len(seriesL2.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-2 series count: got %d, want %d", got, want)
	}
}
