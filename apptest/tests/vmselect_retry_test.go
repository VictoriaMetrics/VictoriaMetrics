package tests

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
)

func TestClusterVmselectRetry(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-vminsertAddr=127.0.0.1:8401",
		"-vmselectAddr=127.0.0.1:8402",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

	const numMetrics = 10
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

	assertSeries := func(app *apptest.Vmselect) {
		t.Helper()
		got := app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, qopts)
		got.Sort()
		if got.Status == "error" && strings.Contains(got.Error, "connection refused") {
			return
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected /api/v1/series response (-want, +got):\n%s", diff)
		}
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			for {
				select {
				case <-stopCh:
					return
				default:
					assertSeries(vmselect)
					time.Sleep(10 * time.Millisecond)
				}
			}
		})
	}

	time.Sleep(200 * time.Millisecond)

	tc.StopApp("vmstorage")
	vmstorage = tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-vminsertAddr=127.0.0.1:8401",
		"-vmselectAddr=127.0.0.1:8402",
	})

	time.Sleep(200 * time.Millisecond)

	close(stopCh)
	wg.Wait()
}
