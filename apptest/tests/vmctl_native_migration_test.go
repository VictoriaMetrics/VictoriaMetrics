package tests

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestSingleToSingleVmctlNativeProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleSrc := tc.MustStartVmsingle("vmsingle_src", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle_src",
		"-retentionPeriod=100y",
	})
	// we need a separate vmsingle for the destination to avoid conflicts
	vmsingleDst := tc.MustStartVmsingle("vmsingle_dst", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle_dst",
		"-retentionPeriod=100y",
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", vmsingleSrc.HTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	// Backfill source with data
	const numSamples = 1000
	const ingestTimestamp = 1748623176000 // 2025-05-30T16:39:36Z
	dataSet := make([]string, numSamples)
	for i := range numSamples {
		metricsName := fmt.Sprintf("metric_%03d", i)
		dataSet[i] = fmt.Sprintf("%s %d %d", metricsName, i, ingestTimestamp)
	}
	vmsingleSrc.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{})
	vmsingleSrc.ForceFlush(t)

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--vm-native-filter-time-end=2025-05-30T16:40:00Z`,
		`--disable-progress-bar=true`,
		// Store the migration state in the test dir, so it is removed together with the test dir.
		`--state-dir=` + tc.Dir(),
	}

	testVmctlNativeProtocol(tc, vmsingleSrc, vmsingleDst, flags)
	// repeated test shouldn't produce duplicates as vmctl tracks the migration state
	// and skips metrics it previously migrated.
	testVmctlNativeProtocol(tc, vmsingleSrc, vmsingleDst, flags)
}

func TestClusterTenantsToTenantsVmctlNativeProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	clusterSrc := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorageSrc1",
		Vmstorage1Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage1-src"),
			"-retentionPeriod=100y",
		},
		Vmstorage2Instance: "vmstorageSrc2",
		Vmstorage2Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage2-src"),
			"-retentionPeriod=100y",
		},
		VminsertInstance: "vminsertSrc",
		VmselectInstance: "vmselectSrc",
	})
	clusterDst := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorageDst1",
		Vmstorage1Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage1-dst"),
			"-retentionPeriod=100y",
		},
		Vmstorage2Instance: "vmstorageDst2",
		Vmstorage2Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage2-dst"),
			"-retentionPeriod=100y",
		},
		VminsertInstance: "vminsertDst",
		VmselectInstance: "vmselectDst",
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", clusterSrc.Vmselect.HTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", clusterDst.Vminsert.HTTPAddr())

	// Backfill source with data
	const numSamples = 1000
	const ingestTimestamp = 1748623176000 // 2025-05-30T16:39:36Z
	dataSet := make([]string, numSamples)
	for i := range numSamples {
		metricsName := fmt.Sprintf("metric_%03d", i)
		dataSet[i] = fmt.Sprintf("%s %d %d", metricsName, i, ingestTimestamp)
	}
	clusterSrc.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{})
	clusterSrc.ForceFlush(t)

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--vm-native-filter-time-end=2025-05-30T16:40:00Z`,
		`--disable-progress-bar=true`,
		// Store the migration state in the test dir, so it is removed together with the test dir.
		`--state-dir=` + tc.Dir(),
		`--vm-intercluster`,
	}

	testVmctlNativeProtocol(tc, clusterSrc, clusterDst, flags)
	// repeated test shouldn't produce duplicates as vmctl tracks the migration state
	// and skips metrics it previously migrated.
	testVmctlNativeProtocol(tc, clusterSrc, clusterDst, flags)
}

func testVmctlNativeProtocol(tc *apptest.TestCase, srcSut apptest.PrometheusWriteQuerier, dstSut apptest.PrometheusWriteQuerier, vmctlFlags []string) {
	t := tc.T()
	t.Helper()

	want := srcSut.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Start: "2025-05-30T16:39:36Z",
		End:   "2025-05-30T16:39:37Z",
	})
	want.Sort()

	tc.MustStartVmctl("vmctl", vmctlFlags)
	dstSut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /prometheus/api/v1/export response",
		Got: func() any {
			exported := dstSut.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-05-30T16:39:36Z",
				End:   "2025-05-30T16:39:37Z",
			})
			exported.Sort()
			return exported.Data.Result
		},
		Want: want.Data.Result,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
