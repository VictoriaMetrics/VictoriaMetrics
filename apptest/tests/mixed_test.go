package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

var (
	vmselectPath = os.Getenv("VM_VMSELECT_PATH")
)

func TestMixedDataRetrieval(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const numMetrics = 1000
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	data := apptest.GenerateTestData("metric", numMetrics, start, end)

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + filepath.Join(tc.Dir(), "vmsingle"),
		"-retentionPeriod=100y",
	})
	vmselect := tc.MustStartVmselectAt("vmselect", vmselectPath, []string{
		"-storageNode=" + vmsingle.VmselectAddr(),
	})

	vmsingle.PrometheusAPIV1ImportPrometheus(tc.T(), data.Samples, apptest.QueryOpts{})
	vmsingle.ForceFlush(t)
	apptest.AssertSeries(tc, vmsingle, "metric.*", start, end, data.WantSeries)
	apptest.AssertQueryResults(tc, vmsingle, "metric.*", start, end, data.Step, data.WantQueryResults)

	apptest.AssertSeries(tc, vmselect, "metric.*", start, end, data.WantSeries)
	apptest.AssertQueryResults(tc, vmselect, "metric.*", start, end, data.Step, data.WantQueryResults)
}
