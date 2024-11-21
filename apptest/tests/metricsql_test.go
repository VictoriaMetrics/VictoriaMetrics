package tests

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var staleNaNsData = func() []pb.TimeSeries {
	ts1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	ts2 := time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC).UnixMilli()
	return []pb.TimeSeries{
		{
			Labels: []pb.Label{{"__name__", "metric"}},
			Samples: []pb.Sample{
				{1.0, ts1},
				{decimal.StaleNaN, ts2}},
		},
	}
}()

func TestSingleInstantQueryStaleNaNs(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})

	testInstantQueryStaleNaNs(t, sut)
}

func TestClusterInstantQueryStaleNaNs(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartCluster()

	testInstantQueryStaleNaNs(t, sut)
}

func testInstantQueryStaleNaNs(t *testing.T, sut apptest.PrometheusWriteQuerier) {
	opts := apptest.QueryOpts{Timeout: "5s", Tenant: "0"}

	sut.PrometheusAPIV1Write(t, staleNaNsData, opts)
	sut.ForceFlush(t)

	got := sut.PrometheusAPIV1Query(t, "metric[5m]", "2024-01-01T00:04:00.000Z", "5m", opts)
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}, "values": []}]}}`)
	s := make([]*apptest.Sample, 2)
	s[0] = apptest.NewSample(t, "2024-01-01T00:00:00Z", 1.0)
	s[1] = apptest.NewSample(t, "2024-01-01T00:01:00Z", decimal.StaleNaN)
	want.Data.Result[0].Samples = s
	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
