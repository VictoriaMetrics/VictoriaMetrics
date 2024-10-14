package apptest

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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

	vmstorage1 := mustStartVmstorage(t, "vmstorage-1", []string{
		"-storageDataPath=" + t.Name() + "/vmstorage-1",
	})
	defer vmstorage1.stop()
	vmstorage2 := mustStartVmstorage(t, "vmstorage-2", []string{
		"-storageDataPath=" + t.Name() + "/vmstorage-2",
	})
	defer vmstorage2.stop()
	vminsert := mustStartVminsert(t, "vminsert", []string{
		"-storageNode=" + vmstorage1.vminsertAddr + "," + vmstorage2.vminsertAddr,
	})
	defer vminsert.stop()
	vmselect := mustStartVmselect(t, "vmselect", []string{
		"-storageNode=" + vmstorage1.vmselectAddr + "," + vmstorage2.vmselectAddr,
	})
	defer vmselect.stop()

	cli := newClient()
	defer cli.closeConnections()

	// Insert 1000 unique time series and verify the that inserted data has been
	// indeed sharded by checking various metrics exposed by vminsert and
	// vmstorage.
	// Also wait for 2 seconds to let vminsert and vmstorage servers to update
	// the values of the metrics they expose and to let vmstorages flush pending
	// items so they become searchable.

	const numMetrics = 1000
	var records strings.Builder
	for i := range numMetrics {
		rec := fmt.Sprintf("metric_%d %d\n", i, rand.IntN(1000))
		records.WriteString(rec)
	}
	insertURL := fmt.Sprintf("http://%s/insert/0/prometheus/api/v1/import/prometheus", vminsert.httpListenAddr)
	cli.post(t, insertURL, "text/plain", records.String(), http.StatusNoContent)
	time.Sleep(2 * time.Second)

	numMetrics1 := int(cli.getMetric(t, vmstorage1.metricsURL, "vm_vminsert_metrics_read_total"))
	if numMetrics1 == 0 {
		t.Fatalf("storage-1 has no time series")
	}
	numMetrics2 := int(cli.getMetric(t, vmstorage2.metricsURL, "vm_vminsert_metrics_read_total"))
	if numMetrics2 == 0 {
		t.Fatalf("storage-2 has no time series")
	}
	if numMetrics1+numMetrics2 != numMetrics {
		t.Fatalf("unxepected total number of metrics: vmstorage-1 (%d) + vmstorage-2 (%d) != %d", numMetrics1, numMetrics2, numMetrics)
	}

	// Retrieve all time series and verify that vmselect serves the complete set
	//of time series.

	selectURL := fmt.Sprintf("http://%s/select/0/prometheus/api/v1/series", vmselect.httpListenAddr)
	series := cli.apiV1Series(t, selectURL, `{__name__=~".*"}`)
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

func testRemoveAll(t *testing.T) {
	if !t.Failed() {
		fs.MustRemoveAll(t.Name())
	}
}
