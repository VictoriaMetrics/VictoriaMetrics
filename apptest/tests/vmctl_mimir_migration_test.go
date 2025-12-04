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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

const (
	testMimirPath             = "testdata/mimir-tsdb"
	expectedMimirResponseFile = "./testdata/mimir-tsdb/expected_response.json"
)

func TestSingleVmctlMimirProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get current working directory: %s", err)
	}

	path := fmt.Sprintf("fs://%s/%s", dir, testMimirPath)
	vmctlFlags := []string{
		`mimir`,
		`--mimir-tenant-id=anonymous`,
		`--mimir-filter-time-start=2024-12-01T00:00:00Z`,
		`--mimir-filter-time-end=2024-12-31T23:59:59Z`,
		`--mimir-custom-s3-endpoint=http://localhost:9000`,
		`--mimir-path=` + path,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--vm-concurrency=6`,
		`--mimir-concurrency=6`,
	}

	testMimirProtocol(tc, vmsingleDst, vmctlFlags)
}

func TestClusterVmctlMimirProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.HTTPAddr())
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get current working directory: %s", err)
	}

	path := fmt.Sprintf("fs://%s/%s", dir, testMimirPath)

	vmctlFlags := []string{
		`mimir`,
		`--mimir-tenant-id=anonymous`,
		`--mimir-filter-time-start=2024-12-01T00:00:00Z`,
		`--mimir-filter-time-end=2024-12-31T23:59:59Z`,
		`--mimir-custom-s3-endpoint=http://localhost:9000`,
		`--mimir-path=` + path,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--vm-concurrency=6`,
		`--mimir-concurrency=6`,
	}

	testMimirProtocol(tc, cluster, vmctlFlags)
}

func testMimirProtocol(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, vmctlFlags []string) {
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

	tc.MustStartVmctl("vmctl", vmctlFlags)

	sut.ForceFlush(t)

	// open the expected series response file
	file, err := os.Open(expectedMimirResponseFile)
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
			expected := sut.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2024-12-01T15:31:10Z",
				End:   "2024-12-31T15:32:20Z",
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
