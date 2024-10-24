package apptest

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

func TestVminsertShardsDataVmselectBuildsFullResultFromShards(t *testing.T) {
	defer testRemoveAll(t)

	// Set up the following cluster configuration:
	//
	// - two vmstorage instances
	// - vminsert points to the two vmstorages, its replication setting
	//   is off which means it will only shard the incoming data across the two
	//   vmstorages.
	// - vmselect points to the two vmstorages and is expected to query both
	//   vmstorages and build the full result out of the two partial results.

	cli := newClient()
	defer cli.closeConnections()

	vmstorage1 := mustStartVmstorage(t, "vmstorage-1", []string{
		"-storageDataPath=" + t.Name() + "/vmstorage-1",
	}, cli)
	defer vmstorage1.stop()
	vmstorage2 := mustStartVmstorage(t, "vmstorage-2", []string{
		"-storageDataPath=" + t.Name() + "/vmstorage-2",
	}, cli)
	defer vmstorage2.stop()
	vminsert := mustStartVminsert(t, "vminsert", []string{
		"-storageNode=" + vmstorage1.vminsertAddr + "," + vmstorage2.vminsertAddr,
	}, cli)
	defer vminsert.stop()
	vmselect := mustStartVmselect(t, "vmselect", []string{
		"-storageNode=" + vmstorage1.vmselectAddr + "," + vmstorage2.vmselectAddr,
	}, cli)
	defer vmselect.stop()

	// Insert 1000 unique time series and verify the that inserted data has been
	// indeed sharded by checking various metrics exposed by vminsert and
	// vmstorage.
	// Also wait for 2 seconds to let vminsert and vmstorage servers to update
	// the values of the metrics they expose and to let vmstorages flush pending
	// items so they become searchable.

	const numMetrics = 1000
	records := make([]string, numMetrics)
	for i := range numMetrics {
		records[i] = fmt.Sprintf("metric_%d %d", i, rand.IntN(1000))
	}
	vminsert.prometheusAPIV1ImportPrometheus(t, "0", records)
	time.Sleep(2 * time.Second)

	numMetrics1 := vmstorage1.getIntMetric(t, "vm_vminsert_metrics_read_total")
	if numMetrics1 == 0 {
		t.Fatalf("storage-1 has no time series")
	}
	numMetrics2 := vmstorage2.getIntMetric(t, "vm_vminsert_metrics_read_total")
	if numMetrics2 == 0 {
		t.Fatalf("storage-2 has no time series")
	}
	if numMetrics1+numMetrics2 != numMetrics {
		t.Fatalf("unxepected total number of metrics: vmstorage-1 (%d) + vmstorage-2 (%d) != %d", numMetrics1, numMetrics2, numMetrics)
	}

	// Retrieve all time series and verify that vmselect serves the complete set
	//of time series.

	series := vmselect.prometheusAPIV1Series(t, "0", `{__name__=~".*"}`)
	if got, want := series.Status, "success"; got != want {
		t.Fatalf("unexpected /ap1/v1/series response status: got %s, want %s", got, want)
	}
	if got, want := series.IsPartial, false; got != want {
		t.Fatalf("unexpected /ap1/v1/series response isPartial value: got %t, want %t", got, want)
	}
	if got, want := len(series.Data), numMetrics; got != want {
		t.Fatalf("unexpected /ap1/v1/series response series count: got %d, want %d", got, want)
	}
}
