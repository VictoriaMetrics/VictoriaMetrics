package apptest

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

func TestMultilevelSelect(t *testing.T) {
	t.Skip("currently fails")

	defer testRemoveAll(t)

	cli := newClient()
	defer cli.closeConnections()

	// Set up the following multi-level cluster configuration:
	//
	// vmselect (L2) -> vmselect (L1) -> vmstorage <- vminsert
	//
	// vmisert writes data into vmstorage.
	// vmselect (L2) reads that data via vmselect (L1).

	vmstorage := mustStartVmstorage(t, "vmstorage", []string{
		"-storageDataPath=" + t.Name() + "/vmstorage",
	}, cli)
	defer vmstorage.stop()
	vminsert := mustStartVminsert(t, "vminsert", []string{
		"-storageNode=" + vmstorage.vminsertAddr,
	}, cli)
	defer vminsert.stop()
	vmselectL1 := mustStartVmselect(t, "vmselect-level1", []string{
		"-storageNode=" + vmstorage.vmselectAddr,
	}, cli)
	defer vmselectL1.stop()
	vmselectL2 := mustStartVmselect(t, "vmselect-level2", []string{
		"-storageNode=" + vmselectL1.clusternativeListenAddr,
	}, cli)
	defer vmselectL2.stop()

	// Insert 1000 unique time series.Wait for 2 seconds to let vmstorage
	// flush pending items so they become searchable.

	const numMetrics = 1000
	records := make([]string, numMetrics)
	for i := range numMetrics {
		records[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.prometheusAPIV1ImportPrometheus(t, "0", records)
	time.Sleep(2 * time.Second)

	// Retrieve all time series and verify that vmselect (L1) serves the complete
	// set of time series.

	seriesL1 := vmselectL1.prometheusAPIV1Series(t, "0", `{__name__=~".*"}`)
	if got, want := len(seriesL1.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-1 series count: got %d, want %d", got, want)
	}

	// Retrieve all time series and verify that vmselect (L2) serves the complete
	// set of time series.

	seriesL2 := vmselectL2.prometheusAPIV1Series(t, "0", `{__name__=~".*"}`)
	if got, want := len(seriesL2.Data), numMetrics; got != want {
		t.Fatalf("unexpected level-2 series count: got %d, want %d", got, want)
	}
}
