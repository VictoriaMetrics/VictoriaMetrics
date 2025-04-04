package tests

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestSingleVMAgentZstdRemoteWrite verifies that vmagent can successfully perform
// a remote write to vmsingle using VM protocol (zstd).
func TestSingleVMAgentZstdRemoteWrite(t *testing.T) {
	testSingleVMAgentRemoteWrite(t, false)
}

// TestSingleVMAgentSnappyRemoteWrite verifies that vmagent can successfully perform
// a remote write to vmsingle using Prometheus protocol (snappy).
func TestSingleVMAgentSnappyRemoteWrite(t *testing.T) {
	testSingleVMAgentRemoteWrite(t, true)
}

func testSingleVMAgentRemoteWrite(t *testing.T, forcePromProto bool) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartDefaultVmsingle()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		fmt.Sprintf(`-remoteWrite.forcePromProto=%v`, forcePromProto),
		fmt.Sprintf(`-remoteWrite.url=http://%s/api/v1/write`, vmsingle.HTTPAddr()),
	}, ``)

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmsingle.ForceFlush(t)

	tc.Assert(&at.AssertOptions{
		Msg: `unexpected metrics stored on vmagent remote write`,
		Got: func() any {
			return vmsingle.PrometheusAPIV1Series(t, `{__name__="foo_bar"}`, at.QueryOpts{
				Start: "2022-05-10T00:00:00Z",
				End:   "2022-05-10T23:59:59Z",
			}).Sort()
		},
		Want: &at.PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   []map[string]string{{"__name__": "foo_bar"}},
		},
	})
}
