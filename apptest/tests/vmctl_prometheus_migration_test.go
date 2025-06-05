package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

const (
	testSnapshot         = "./testdata/prometheus/snapshots/20250602T205846Z-7e03e43cf46dda03"
	expectedResponseFile = "./testdata/prometheus/expected_response.json"
)

func TestSingleVmctlPrometheusProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	vmctlFlags := []string{
		`prometheus`,
		`--prom-snapshot=` + testSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
	}

	testPrometheusProtocol(tc, vmsingleDst, vmctlFlags)
}

func TestClusterVmctlPrometheusProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.InsertHTTPAddr())
	vmctlFlags := []string{
		`prometheus`,
		`--prom-snapshot=` + testSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--vm-account-id=0`,
	}

	testPrometheusProtocol(tc, cluster, vmctlFlags)
}

func testPrometheusProtocol(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, vmctlFlags []string) {
	t := tc.T()
	t.Helper()

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// test for empty data request
	got := sut.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: "2025-06-02T17:14:00Z",
	})

	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	_ = tc.MustStartVmctl("vmctl", vmctlFlags)

	sut.ForceFlush(t)

	// open the expected series response file
	file, err := os.Open(expectedResponseFile)
	if err != nil {
		t.Fatalf("cannot open expected series response file: %s", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("cannot read expected series response file: %s", err)
	}

	var wantResponse apptest.PrometheusAPIV1QueryResponse
	if err := json.Unmarshal(bytes, &wantResponse); err != nil {
		t.Fatalf("cannot unmarshal expected series response file: %s", err)
	}
	wantResponse.Sort()

	tc.Assert(&apptest.AssertOptions{
		// For cluster version, we need to wait longer for the metrics to be stored
		Retries: 300,
		Msg:     `unexpected metrics stored on vmsingle via the prometheus protocol`,
		Got: func() any {
			expected := sut.PrometheusAPIV1Export(t, `{__name__="vm_log_messages_total", location=~"VictoriaMetrics/lib/ingestserver/opentsdb/server.go:(48|59)"}`, apptest.QueryOpts{
				Start: "2025-06-02T00:00:00Z",
				End:   "2025-06-02T23:59:59Z",
			})
			expected.Sort()
			return expected.Data.Result
		},
		Want: wantResponse.Data.Result,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
