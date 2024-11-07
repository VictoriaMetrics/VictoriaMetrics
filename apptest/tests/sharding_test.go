package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestClusterVminsertShardsDataVmselectBuildsFullResultFromShards(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Set up the following cluster configuration:
	//
	// - two vmstorage instances
	// - vminsert points to the two vmstorages, its replication setting
	//   is off which means it will only shard the incoming data across the two
	//   vmstorages.
	// - vmselect points to the two vmstorages and is expected to query both
	//   vmstorages and build the full result out of the two partial results.

	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	})

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
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, apptest.QueryOpts{Tenant: "0"})
	time.Sleep(2 * time.Second)

	numMetrics1 := vmstorage1.GetIntMetric(t, "vm_vminsert_metrics_read_total")
	if numMetrics1 == 0 {
		t.Fatalf("storage-1 has no time series")
	}
	numMetrics2 := vmstorage2.GetIntMetric(t, "vm_vminsert_metrics_read_total")
	if numMetrics2 == 0 {
		t.Fatalf("storage-2 has no time series")
	}
	if numMetrics1+numMetrics2 != numMetrics {
		t.Fatalf("unxepected total number of metrics: vmstorage-1 (%d) + vmstorage-2 (%d) != %d", numMetrics1, numMetrics2, numMetrics)
	}

	// Retrieve all time series and verify that vmselect serves the complete set
	//of time series.

	series := vmselect.PrometheusAPIV1Series(t, `{__name__=~".*"}`, apptest.QueryOpts{Tenant: "0"})
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
