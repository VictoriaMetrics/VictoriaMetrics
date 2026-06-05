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
	// thanosSnapshot contains both raw (resolution=0) and downsampled (resolution>0) blocks
	// with Thanos metadata in meta.json.
	thanosSnapshot = "./testdata/thanos-snapshot"

	// thanosExpectedAllAggrResponse is the expected response when all aggregate types are imported
	// (default behavior without --thanos-aggr-types flag).
	thanosExpectedAllAggrResponse = "./testdata/thanos-snapshot/expected_all_aggr_response.json"

	// thanosExpectedFilteredAggrResponse is the expected response when only specific aggregate
	// types are imported via --thanos-aggr-types flag.
	thanosExpectedFilteredAggrResponse = "./testdata/thanos-snapshot/expected_filtered_aggr_response.json"

	// thanosQueryFilter is the PromQL query to select the test metrics.
	thanosQueryFilter = `{__name__=~"thanos_test.*"}`

	// thanosQueryTimeStart and thanosQueryTimeEnd define the time range for querying imported data.
	thanosQueryTimeStart = "2025-01-01T00:00:00Z"
	thanosQueryTimeEnd   = "2025-01-01T02:00:00Z"
)

// TestSingleVmctlThanosMigrationAllAggr tests migration of Thanos blocks without
// --thanos-aggr-types flag. All aggregate types should be imported by default.
func TestSingleVmctlThanosMigrationAllAggr(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	vmctlFlags := []string{
		`thanos`,
		`--thanos-snapshot=` + thanosSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
	}

	testThanosMigration(tc, vmsingleDst, vmctlFlags, thanosExpectedAllAggrResponse)
}

// TestClusterVmctlThanosMigrationAllAggr tests migration of Thanos blocks to cluster
// without --thanos-aggr-types flag. All aggregate types should be imported by default.
func TestClusterVmctlThanosMigrationAllAggr(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.HTTPAddr())
	vmctlFlags := []string{
		`thanos`,
		`--thanos-snapshot=` + thanosSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--vm-account-id=0`,
	}

	testThanosMigration(tc, cluster, vmctlFlags, thanosExpectedAllAggrResponse)
}

// TestSingleVmctlThanosMigrationFilteredAggr tests migration of Thanos blocks with
// --thanos-aggr-types flag set to specific types (e.g., count,sum).
func TestSingleVmctlThanosMigrationFilteredAggr(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	vmctlFlags := []string{
		`thanos`,
		`--thanos-snapshot=` + thanosSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--thanos-aggr-types=count`,
		`--thanos-aggr-types=sum`,
	}

	testThanosMigration(tc, vmsingleDst, vmctlFlags, thanosExpectedFilteredAggrResponse)
}

// TestClusterVmctlThanosMigrationFilteredAggr tests migration of Thanos blocks to cluster
// with --thanos-aggr-types flag set to specific types (e.g., count,sum).
func TestClusterVmctlThanosMigrationFilteredAggr(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.HTTPAddr())
	vmctlFlags := []string{
		`thanos`,
		`--thanos-snapshot=` + thanosSnapshot,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`--vm-account-id=0`,
		`--thanos-aggr-types=count`,
		`--thanos-aggr-types=sum`,
	}

	testThanosMigration(tc, cluster, vmctlFlags, thanosExpectedFilteredAggrResponse)
}

func testThanosMigration(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, vmctlFlags []string, expectedFile string) {
	t := tc.T()
	t.Helper()

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// Verify no data exists before migration
	got := sut.PrometheusAPIV1Query(t, thanosQueryFilter, apptest.QueryOpts{
		Step: "5m",
		Time: thanosQueryTimeStart,
	})

	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response before migration (-want, +got):\n%s", diff)
	}

	// Run vmctl migration
	tc.MustStartVmctl("vmctl", vmctlFlags)

	sut.ForceFlush(t)

	// Load expected response
	file, err := os.Open(expectedFile)
	if err != nil {
		t.Fatalf("cannot open expected response file: %s", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("cannot read expected response file: %s", err)
	}

	var wantResponse apptest.PrometheusAPIV1QueryResponse
	if err := json.Unmarshal(bytes, &wantResponse); err != nil {
		t.Fatalf("cannot unmarshal expected response file: %s", err)
	}
	wantResponse.Sort()

	tc.Assert(&apptest.AssertOptions{
		Retries: 300,
		Msg:     "unexpected metrics stored after Thanos migration",
		Got: func() any {
			result := sut.PrometheusAPIV1Export(t, thanosQueryFilter, apptest.QueryOpts{
				Start: thanosQueryTimeStart,
				End:   thanosQueryTimeEnd,
			})
			result.Sort()
			return result.Data.Result
		},
		Want: wantResponse.Data.Result,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
