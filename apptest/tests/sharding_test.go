package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"

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
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, apptest.QueryOpts{})
	vmstorage1.ForceFlush(t)
	vmstorage2.ForceFlush(t)

	// Verify that inserted data has been indeed sharded by checking metrics
	// exposed by vmstorage.

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
	// of time series.

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/series response",
		Got: func() any {
			res := vmselect.PrometheusAPIV1Series(t, `{__name__=~".*"}`, apptest.QueryOpts{})
			res.Sort()
			return res
		},
		Want: want,
	})
}
