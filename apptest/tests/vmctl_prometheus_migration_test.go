package tests

import (
	"fmt"
	"io"
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
	testSnapshot := "./testdata/prometheus/snapshots/20250118T124506Z-59d1b952d7eaf547"
	_ = tc.MustStartVmctl("vmctl", []string{
		`prometheus`,
		`--prom-snapshot=` + testSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
	})

	vmsingleDst.ForceFlush(t)

	// open the expected series response file
	file, err := os.Open("./testdata/prometheus/expected_response.json")
	if err != nil {
		t.Fatalf("cannot open expected series response file: %s", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("cannot read expected series response file: %s", err)
	}

	wantResponse := apptest.NewPrometheusAPIV1QueryResponse(t, string(bytes))
	wantResponse.Sort()

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmsingle via the prometheus protocol`,
		Got: func() any {
			exported := vmsingleDst.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-01-18T00:45:00Z",
				End:   "2025-01-18T23:46:00Z",
			})
			exported.Sort()
			return exported
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{Data: wantResponse.Data},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
