package tests

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleSeriesUpdate(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	originSamples := []prompb.Sample{
		{Timestamp: millis("2024-01-01T00:00:00Z"), Value: 0},
		{Timestamp: millis("2024-01-01T00:01:00Z"), Value: 1},
		{Timestamp: millis("2024-01-01T00:02:00Z"), Value: 2},
		{Timestamp: millis("2024-01-01T00:03:00Z"), Value: 3},
		{Timestamp: millis("2024-01-01T00:04:00Z"), Value: 4},
	}
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			// origin series
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
				},
				Samples: originSamples,
			},
			// updates must be applied in revision order, so rev=2 overrides rev=1 at minute 2
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "2"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:02:00Z"), Value: 20}, {Timestamp: millis("2024-01-01T00:03:00Z"), Value: 20}},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "1"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:01:00Z"), Value: 10}, {Timestamp: millis("2024-01-01T00:02:00Z"), Value: 10}},
			},
			// update adds new points after the origin series end
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "3"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:05:00Z"), Value: 30}, {Timestamp: millis("2024-01-01T00:06:00Z"), Value: 30}},
			},
			// series without updates must be returned as is
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "b"},
				},
				Samples: originSamples,
			},
			// update without origin series is dropped
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "c"},
					{Name: "rev", Value: "3"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:00:00Z"), Value: 1}},
			},
		},
	}
	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// default_rollup over merged series
	got := sut.PrometheusAPIV1QueryRange(t, `up`, apptest.QueryOpts{
		Start:            "2024-01-01T00:00:00Z",
		End:              "2024-01-01T00:06:00Z",
		Step:             "1m",
		MergeLatestLabel: "rev",
	})
	got.Sort()
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [
		{"metric": {"__name__": "up", "job": "a"}, "values": [[1704067200,"0"],[1704067260,"10"],[1704067320,"20"],[1704067380,"20"],[1704067440,"4"],[1704067500,"30"],[1704067560,"30"]]},
		{"metric": {"__name__": "up", "job": "b"}, "values": [[1704067200,"0"],[1704067260,"1"],[1704067320,"2"],[1704067380,"3"],[1704067440,"4"],[1704067500,"4"]]}
	]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// rollup function over merged raw samples: samples at minutes 4, 5 and 6 are in the window
	got = sut.PrometheusAPIV1Query(t, `sum_over_time(up{job="a"}[3m])`, apptest.QueryOpts{
		Time:             "2024-01-01T00:06:00Z",
		MergeLatestLabel: "rev",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [
		{"metric": {"job": "a"}, "value": [1704067560,"64"]}
	]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}

func TestClusterSeriesUpdate(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	originSamples := []prompb.Sample{
		{Timestamp: millis("2024-01-01T00:00:00Z"), Value: 0},
		{Timestamp: millis("2024-01-01T00:01:00Z"), Value: 1},
		{Timestamp: millis("2024-01-01T00:02:00Z"), Value: 2},
		{Timestamp: millis("2024-01-01T00:03:00Z"), Value: 3},
		{Timestamp: millis("2024-01-01T00:04:00Z"), Value: 4},
	}
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			// origin series
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
				},
				Samples: originSamples,
			},
			// updates must be applied in revision order, so rev=2 overrides rev=1 at minute 2
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "2"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:02:00Z"), Value: 20}, {Timestamp: millis("2024-01-01T00:03:00Z"), Value: 20}},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "1"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:01:00Z"), Value: 10}, {Timestamp: millis("2024-01-01T00:02:00Z"), Value: 10}},
			},
			// update adds new points after the origin series end
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "3"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:05:00Z"), Value: 30}, {Timestamp: millis("2024-01-01T00:06:00Z"), Value: 30}},
			},
			// series without updates must be returned as is
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "b"},
				},
				Samples: originSamples,
			},
			// update without origin series is dropped
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "c"},
					{Name: "rev", Value: "3"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:00:00Z"), Value: 1}},
			},
		},
	}
	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// default_rollup over merged series
	got := sut.PrometheusAPIV1QueryRange(t, `up`, apptest.QueryOpts{
		Start:            "2024-01-01T00:00:00Z",
		End:              "2024-01-01T00:06:00Z",
		Step:             "1m",
		MergeLatestLabel: "rev",
	})
	got.Sort()
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [
		{"metric": {"__name__": "up", "job": "a"}, "values": [[1704067200,"0"],[1704067260,"10"],[1704067320,"20"],[1704067380,"20"],[1704067440,"4"],[1704067500,"30"],[1704067560,"30"]]},
		{"metric": {"__name__": "up", "job": "b"}, "values": [[1704067200,"0"],[1704067260,"1"],[1704067320,"2"],[1704067380,"3"],[1704067440,"4"],[1704067500,"4"]]}
	]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// rollup function over merged raw samples: samples at minutes 4, 5 and 6 are in the window
	got = sut.PrometheusAPIV1Query(t, `sum_over_time(up{job="a"}[3m])`, apptest.QueryOpts{
		Time:             "2024-01-01T00:06:00Z",
		MergeLatestLabel: "rev",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [
		{"metric": {"job": "a"}, "value": [1704067560,"64"]}
	]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
