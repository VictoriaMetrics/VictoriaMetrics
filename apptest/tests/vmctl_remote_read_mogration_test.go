package tests

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/prometheus/prompb"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestSingleVmctlRemoteReadProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	vmctlFlags := []string{
		`remote-read`,
		`--remote-read-filter-time-start=2025-06-11T15:31:10Z`,
		`--remote-read-filter-time-end=2025-06-11T15:31:20Z`,
		`--remote-read-step-interval=minute`,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
	}

	testRemoteReadProtocol(tc, vmsingleDst, newRemoteReadServer, vmctlFlags)
}

func TestSingleVmctlRemoteReadStreamProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())
	vmctlFlags := []string{
		`remote-read`,
		`--remote-read-filter-time-start=2025-06-11T15:31:10Z`,
		`--remote-read-filter-time-end=2025-06-11T15:31:20Z`,
		`--remote-read-step-interval=minute`,
		`--vm-addr=` + vmAddr,
		`--remote-read-use-stream=true`,
		`--disable-progress-bar=true`,
	}

	testRemoteReadProtocol(tc, vmsingleDst, newRemoteReadStreamServer, vmctlFlags)
}

func TestClusterVmctlRemoteReadProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	clusterDst := tc.MustStartDefaultCluster()

	vmAddr := fmt.Sprintf("http://%s/", clusterDst.Vminsert.HTTPAddr())
	vmctlFlags := []string{
		`remote-read`,
		`--remote-read-filter-time-start=2025-06-11T15:31:10Z`,
		`--remote-read-filter-time-end=2025-06-11T15:31:20Z`,
		`--remote-read-step-interval=minute`,
		`--vm-addr=` + vmAddr,
		`--vm-account-id=0`,
		`--disable-progress-bar=true`,
	}

	testRemoteReadProtocol(tc, clusterDst, newRemoteReadServer, vmctlFlags)
}

func testRemoteReadProtocol(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, newRemoteReadServer func(t *testing.T) *RemoteReadServer, vmctlFlags []string) {
	t := tc.T()
	t.Helper()

	rrs := newRemoteReadServer(t)
	defer rrs.Close()

	expectedResult := transformSeriesToQueryResult(rrs.storage.store)

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

	vmctlFlags = append(vmctlFlags, `--remote-read-src-addr=`+rrs.HTTPAddr())
	tc.MustStartVmctl("vmctl", vmctlFlags)

	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		// For cluster version, we need to wait longer for the metrics to be stored
		Retries: 300,
		Msg:     `unexpected metrics stored on vmsingle via the prometheus protocol`,
		Got: func() any {
			expected := sut.PrometheusAPIV1Export(t, `{__name__=~".*"}`, apptest.QueryOpts{
				Start: "2025-06-11T15:31:10Z",
				End:   "2025-06-11T15:32:20Z",
			})
			expected.Sort()
			return expected.Data.Result
		},
		Want: expectedResult,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func newRemoteReadServer(t *testing.T) *RemoteReadServer {
	t.Helper()

	series := GenerateRemoteReadSeries(1749655870, 1749655880, 10, 10)

	rrServer := NewRemoteReadServer(t, series)

	return rrServer
}

func newRemoteReadStreamServer(t *testing.T) *RemoteReadServer {
	t.Helper()

	series := GenerateRemoteReadSeries(1749655870, 1749655880, 10, 10)

	rrServer := NewRemoteReadStreamServer(t, series)

	return rrServer
}

func transformSeriesToQueryResult(series []*prompb.TimeSeries) []*apptest.QueryResult {
	result := make([]*apptest.QueryResult, len(series))
	for i, s := range series {
		metric := make(map[string]string, len(s.Labels))
		for _, label := range s.Labels {
			metric[label.Name] = label.Value
		}
		samples := make([]*apptest.Sample, len(s.Samples))
		for j, sample := range s.Samples {
			samples[j] = &apptest.Sample{Timestamp: sample.Timestamp, Value: sample.Value}
		}
		result[i] = &apptest.QueryResult{Metric: metric, Samples: samples}
	}
	return result
}
