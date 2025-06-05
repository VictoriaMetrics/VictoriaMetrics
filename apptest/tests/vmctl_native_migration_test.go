package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleToSingleVmctlNativeProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleSrc := tc.MustStartDefaultVmsingle()
	// we need a separate vmsingle for the destination to avoid conflicts
	vmsingleDst := tc.MustStartVmsingle("vmsingle_dst", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle_dst",
		"-retentionPeriod=100y",
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", vmsingleSrc.HTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
	}

	testNativeProtocol(tc, vmsingleSrc, vmsingleDst, flags)
}

func TestSingleToClusterVmctlNativeProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleSrc := tc.MustStartDefaultVmsingle()
	clusterDst := tc.MustStartDefaultCluster()

	vmSrcAddr := fmt.Sprintf("http://%s/", vmsingleSrc.HTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", clusterDst.InsertHTTPAddr())

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr + `insert/0/prometheus`,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
	}

	testNativeProtocol(tc, vmsingleSrc, clusterDst, flags)
}

func TestClusterToSingleVmctlNativeProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	clusterSrc := tc.MustStartDefaultCluster()
	vmsingleDst := tc.MustStartDefaultVmsingle()

	vmSrcAddr := fmt.Sprintf("http://%s/", clusterSrc.SelectHTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr + `select/0/prometheus`,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
	}

	testNativeProtocol(tc, clusterSrc, vmsingleDst, flags)
}

func TestClusterToClusterVmctlNativeProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	clusterSrc := tc.MustStartDefaultCluster()
	clusterDst := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorageDst1",
		Vmstorage2Instance: "vmstorageDst2",
		VminsertInstance:   "vminsertDst",
		VmselectInstance:   "vmselectDst",
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", clusterSrc.SelectHTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", clusterDst.InsertHTTPAddr())

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr + `select/0/prometheus`,
		`--vm-native-dst-addr=` + vmDstAddr + `insert/0/prometheus`,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
	}

	testNativeProtocol(tc, clusterSrc, clusterDst, flags)
}

func TestClusterTenantsToTenantsvmctlNativeProtocol(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	clusterSrc := tc.MustStartDefaultCluster()
	clusterDst := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorageDst1",
		Vmstorage2Instance: "vmstorageDst2",
		VminsertInstance:   "vminsertDst",
		VmselectInstance:   "vmselectDst",
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", clusterSrc.SelectHTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", clusterDst.InsertHTTPAddr())

	flags := []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
		`--vm-intercluster`,
	}

	testNativeProtocol(tc, clusterSrc, clusterDst, flags)
}

func testNativeProtocol(tc *apptest.TestCase, srcSut apptest.PrometheusWriteQuerier, dstSut apptest.PrometheusWriteQuerier, vmctlFlags []string) {
	t := tc.T()
	t.Helper()

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// test for empty data request in the source
	got := srcSut.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: "2025-05-30T12:45:00Z",
	})

	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Prepare the source vmsingle with some data
	// Insert some data.
	const numSamples = 1000
	const ingestTimestamp = " 1748623176000" // 2025-05-30T16:39:36Z

	expectedQueryData := apptest.QueryData{
		ResultType: "matrix",
		Result:     make([]*apptest.QueryResult, 0, numSamples),
	}
	dataSet := make([]string, numSamples)
	for i := range numSamples {
		metricsName := fmt.Sprintf("metric_%03d", i)
		metrics := map[string]string{"__name__": metricsName}
		sample := &apptest.Sample{Value: float64(i), Timestamp: 1748623176000}
		expectedQueryData.Result = append(expectedQueryData.Result, &apptest.QueryResult{
			Metric:  metrics,
			Samples: []*apptest.Sample{sample},
		})

		dataSet[i] = fmt.Sprintf("%s %d %s", metricsName, i, ingestTimestamp)
	}

	wantResponse := apptest.PrometheusAPIV1QueryResponse{
		Status: "success",
		Data:   &expectedQueryData,
	}

	wantResponse.Sort()

	srcSut.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{})
	srcSut.ForceFlush(t)

	_ = tc.MustStartVmctl("vmctl", vmctlFlags)

	dstSut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Retries: 300,
		Msg:     `unexpected metrics stored on vmsingle via the native protocol`,
		Got: func() any {
			exported := dstSut.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-05-30T16:39:36Z",
				End:   "2025-05-30T16:39:37Z",
			})
			exported.Sort()
			return exported.Data.Result
		},
		Want: wantResponse.Data.Result,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
