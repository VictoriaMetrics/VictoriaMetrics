package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestVmctlPrometheusProtocolToVMSingle(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	vmsingleDst := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
	})

	// test for empty data request
	got := vmsingleDst.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: "2025-01-18T12:45:00Z",
	})

	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	testSnapshot := "./testdata/snapshots/20250118T124506Z-59d1b952d7eaf547"
	vmctl := tc.MustStartVmctl("vmctl", []string{
		`prometheus`,
		`--prom-snapshot=` + testSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
	})

	// Wait for vmctl to finish processing
	err := vmctl.Wait()
	if err != nil {
		t.Errorf("vmctl.Wait() failed with %s", err)
	}

	vmsingleDst.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmsingle via the prometheus protocol`,
		Got: func() any {
			exported := vmsingleDst.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-01-18T12:45:00Z",
				End:   "2025-01-18T12:46:00Z",
			})
			return len(exported.Data.Result)
		},
		// Expecting 2792 series to be imported
		// this value is stored in the apptest/tests/testdata/snapshots/20250118T124506Z-59d1b952d7eaf547/01JHWQ445Y2P1TDYB05AEKD6MC/meta.json
		Want: 2792,
	})
}
