package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestVmctlVMSingleToVMSingle(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	vmsingleSrc := tc.MustStartVmsingle("vmsingle_src", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle_src",
		"-retentionPeriod=100y",
	})

	// test for empty data request in the source vmsingle
	got := vmsingleSrc.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: "2025-05-30T12:45:00Z",
	})

	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	vmsingleDst := tc.MustStartVmsingle("vmsingle_dst", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle_dst",
		"-retentionPeriod=100y",
	})

	// test for empty data request in the destination vmsingle
	got = vmsingleDst.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: "2025-01-18T12:45:00Z",
	})

	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Prepare the source vmsingle with some data
	// Insert some data.
	const numSamples = 1000
	const ingestTimestamp = " 1748623176000" // 2025-05-30T16:39:36Z
	dataSet := make([]string, numSamples)
	for i := range numSamples {
		dataSet[i] = fmt.Sprintf("metric_%03d %d %s", i, i, ingestTimestamp)
	}

	vmsingleSrc.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{})
	vmsingleSrc.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmsingle source`,
		Got: func() any {
			exported := vmsingleSrc.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-05-30T16:39:36Z",
				End:   "2025-05-30T16:39:37Z",
			})
			return len(exported.Data.Result)
		},
		Want: numSamples,
	})

	vmSrcAddr := fmt.Sprintf("http://%s/", vmsingleSrc.HTTPAddr())
	vmDstAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	_ = tc.MustStartVmctl("vmctl", []string{
		`vm-native`,
		`--vm-native-src-addr=` + vmSrcAddr,
		`--vm-native-dst-addr=` + vmDstAddr,
		`--vm-native-filter-match={__name__=~".*"}`,
		`--vm-native-filter-time-start=2025-05-30T16:39:00Z`,
		`--disable-progress-bar=true`,
	})

	vmsingleDst.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmsingle via the prometheus protocol`,
		Got: func() any {
			exported := vmsingleSrc.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-05-30T16:39:36Z",
				End:   "2025-05-30T16:39:37Z",
			})
			return len(exported.Data.Result)
		},
		Want: numSamples,
	})
}
