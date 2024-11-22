package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func millis(s string) int64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(fmt.Sprintf("could not parse time %q: %v", s, err))
	}
	return t.UnixMilli()
}

var staleNaNsData = func() []pb.TimeSeries {
	return []pb.TimeSeries{
		{
			Labels: []pb.Label{
				{
					Name:  "__name__",
					Value: "metric",
				},
			},
			Samples: []pb.Sample{
				{
					Value:     1,
					Timestamp: millis("2024-01-01T00:01:00Z"),
				},
				{
					Value:     decimal.StaleNaN,
					Timestamp: millis("2024-01-01T00:02:00Z"),
				},
			},
		},
	}
}()

func TestSingleInstantQueryDoesNotReturnStaleNaNs(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testInstantQueryDoesNotReturnStaleNaNs(t, sut)
}

func TestClusterInstantQueryDoesNotReturnStaleNaNs(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testInstantQueryDoesNotReturnStaleNaNs(t, sut)
}

func testInstantQueryDoesNotReturnStaleNaNs(t *testing.T, sut apptest.PrometheusWriteQuerier) {
	opts := apptest.QueryOpts{Timeout: "5s", Tenant: "0"}

	sut.PrometheusAPIV1Write(t, staleNaNsData, opts)
	sut.ForceFlush(t)

	var got, want *apptest.PrometheusAPIV1QueryResponse
	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}

	// Verify that instant query returns the first point.

	got = sut.PrometheusAPIV1Query(t, "metric", "2024-01-01T00:01:00.000Z", "5m", opts)
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}}]}}`)
	want.Data.Result[0].Sample = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that instant query does not return stale NaN.

	got = sut.PrometheusAPIV1Query(t, "metric", "2024-01-01T00:02:00.000Z", "5m", opts)
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": []}}`)
	// Empty response, stale NaN is not included into response
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that instant query with default rollup function does not include stale NaN.

	got = sut.PrometheusAPIV1Query(t, "metric[2m]", "2024-01-01T00:02:00.000Z", "5m", opts)
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}, "values": []}]}}`)
	s := make([]*apptest.Sample, 1)
	s[0] = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	want.Data.Result[0].Samples = s
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that exported data contains stale NaN.

	got = sut.PrometheusAPIV1Export(t, `{__name__="metric"}`, "2024-01-01T00:01:00.000Z", "2024-01-01T00:02:00.000Z", opts)
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}, "values": []}]}}`)
	s = make([]*apptest.Sample, 2)
	s[0] = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	s[1] = apptest.NewSample(t, "2024-01-01T00:02:00Z", decimal.StaleNaN)
	want.Data.Result[0].Samples = s
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
