package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func millis(s string) int64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(fmt.Sprintf("could not parse time %q: %v", s, err))
	}
	return t.UnixMilli()
}

func TestSingleInstantQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testInstantQueryWithUTFNames(t, sut)
	testInstantQueryDoesNotReturnStaleNaNs(t, sut)

	testQueryRangeWithAtModifier(t, sut)

	testLabelValuesWithUTFNames(t, sut)
}

func TestClusterInstantQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testInstantQueryWithUTFNames(t, sut)
	testInstantQueryDoesNotReturnStaleNaNs(t, sut)

	testQueryRangeWithAtModifier(t, sut)

	testLabelValuesWithUTFNames(t, sut)
}

func testInstantQueryWithUTFNames(t *testing.T, sut apptest.PrometheusWriteQuerier) {
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "3fooµ¥"},
					{Name: "3👋tfにちは", Value: "漢©®€£"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: millis("2024-01-01T00:01:00Z")},
				},
			},
		},
	}

	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)

	var got, want *apptest.PrometheusAPIV1QueryResponse
	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}

	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "3fooµ¥", "3👋tfにちは": "漢©®€£"}}]}}`)
	fn := func(query string) {
		got = sut.PrometheusAPIV1Query(t, query, apptest.QueryOpts{
			Step: "5m",
			Time: "2024-01-01T00:01:00.000Z",
		})
		want.Data.Result[0].Sample = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
		if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	fn(`{"3fooµ¥"}`)
	fn(`{__name__="3fooµ¥"}`)
	fn(`{__name__=~"3fo.*"}`)
	fn(`{__name__=~".*µ¥"}`)
	fn(`{"3fooµ¥", "3👋tfにちは"="漢©®€£"}`)
	fn(`{"3fooµ¥", "3👋tfにちは"=~"漢.*"}`)
	fn(`{"3👋tfにちは"="漢©®€£"}`)
}

var staleNaNsData = func() prompb.WriteRequest {
	return prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "metric",
					},
				},
				Samples: []prompb.Sample{
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
		},
	}
}()

func testInstantQueryDoesNotReturnStaleNaNs(t *testing.T, sut apptest.PrometheusWriteQuerier) {

	sut.PrometheusAPIV1Write(t, staleNaNsData, apptest.QueryOpts{})
	sut.ForceFlush(t)

	var got, want *apptest.PrometheusAPIV1QueryResponse
	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}

	// Verify that instant query returns the first point.

	got = sut.PrometheusAPIV1Query(t, "metric", apptest.QueryOpts{
		Step: "5m",
		Time: "2024-01-01T00:01:00.000Z",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}}]}}`)
	want.Data.Result[0].Sample = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that instant query does not return stale NaN.

	got = sut.PrometheusAPIV1Query(t, "metric", apptest.QueryOpts{
		Step: "5m",
		Time: "2024-01-01T00:02:00.000Z",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": []}}`)
	// Empty response, stale NaN is not included into response
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that instant query with default rollup function returns stale NaN
	// while it must not.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5806

	got = sut.PrometheusAPIV1Query(t, "metric[2m]", apptest.QueryOpts{
		Step: "5m",
		Time: "2024-01-01T00:02:00.000Z",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}, "values": []}]}}`)
	s := make([]*apptest.Sample, 2)
	s[0] = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	s[1] = apptest.NewSample(t, "2024-01-01T00:02:00Z", decimal.StaleNaN)
	want.Data.Result[0].Samples = s
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Verify that exported data contains stale NaN.

	got = sut.PrometheusAPIV1Export(t, `{__name__="metric"}`, apptest.QueryOpts{
		Start: "2024-01-01T00:01:00.000Z",
		End:   "2024-01-01T00:02:00.000Z",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "metric"}, "values": []}]}}`)
	s = make([]*apptest.Sample, 2)
	s[0] = apptest.NewSample(t, "2024-01-01T00:01:00Z", 1)
	s[1] = apptest.NewSample(t, "2024-01-01T00:02:00Z", decimal.StaleNaN)
	want.Data.Result[0].Samples = s
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}

// This test checks absence of panic after conversion of math.NaN to int64 in vmselect.
// See: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8444
// However, conversion of math.NaN to int64 could behave differently depending on platform and Go version.
// Hence, this test could succeed for some platforms even if fix is rolled back.
func testQueryRangeWithAtModifier(t *testing.T, sut apptest.PrometheusWriteQuerier) {
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: millis("2025-01-01T00:01:00Z")},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "metricNaN"},
				},
				Samples: []prompb.Sample{
					{Value: decimal.StaleNaN, Timestamp: millis("2025-01-01T00:01:00Z")},
				},
			},
		},
	}

	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)

	resp := sut.PrometheusAPIV1QueryRange(t, `vector(1) @ up`, apptest.QueryOpts{
		Start: "2025-01-01T00:00:00Z",
		End:   "2025-01-01T00:02:00Z",
		Step:  "10s",
	})

	if resp.Status != "success" {
		t.Fatalf("unexpected status: %q", resp.Status)
	}

	resp = sut.PrometheusAPIV1QueryRange(t, `vector(1) @ metricNaN`, apptest.QueryOpts{
		Start: "2025-01-01T00:00:00Z",
		End:   "2025-01-01T00:02:00Z",
		Step:  "10s",
	})

	if resp.Status != "error" {
		t.Fatalf("unexpected status: %q", resp.Status)
	}
	if !strings.Contains(resp.Error, "modifier must return a non-NaN value") {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

// This test checks that label values are decoded from UTF-8 according to Prometheus spec.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10446
// Spec: https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values
func testLabelValuesWithUTFNames(t *testing.T, sut apptest.PrometheusWriteQuerier) {

	timestamp := millis("2025-01-01T00:00:00Z")
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "labelvals"},
					{Name: "kubernetes_something/special&' chars", Value: "漢©®€£"},
					{Name: "3👋tfにちは", Value: "漢©®€£"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: timestamp},
				},
			},
		},
	}

	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)

	cmpOptions := []cmp.Option{}

	// encoded via prometheus model.EscapeName(string,model.ValueEncodingEscaping)
	want := map[string][]string{
		"__name__": {"labelvals"},
		"U__kubernetes__something_2f_special_26__27__20_chars": {"漢©®€£"},
		"U___33__1f44b_tf_306b__3061__306f_":                   {"漢©®€£"},
	}
	for labelName, expected := range want {
		got := sut.PrometheusAPIV1LabelValues(t, labelName, `{__name__="labelvals"}`, apptest.QueryOpts{
			Start: fmt.Sprintf("%d", timestamp),
			End:   fmt.Sprintf("%d", timestamp),
		})
		if diff := cmp.Diff(expected, got.Data, cmpOptions...); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}
}

func TestSingleMergeLatest(t *testing.T) {
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
			// patches must be applied in revision order, so rev=2 overrides rev=1 at minute 2
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
			// patch adds new points after the origin series end
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "a"},
					{Name: "rev", Value: "3"},
				},
				Samples: []prompb.Sample{{Timestamp: millis("2024-01-01T00:05:00Z"), Value: 30}, {Timestamp: millis("2024-01-01T00:06:00Z"), Value: 30}},
			},
			// series without patches must be returned as is
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "job", Value: "b"},
				},
				Samples: originSamples,
			},
			// patch without origin series is dropped
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
	got := sut.PrometheusAPIV1QueryRange(t, `merge_latest(up, "rev")`, apptest.QueryOpts{
		Start: "2024-01-01T00:00:00Z",
		End:   "2024-01-01T00:06:00Z",
		Step:  "1m",
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
	got = sut.PrometheusAPIV1Query(t, `sum_over_time(merge_latest(up{job="a"}, "rev")[3m])`, apptest.QueryOpts{
		Time: "2024-01-01T00:06:00Z",
	})
	want = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [
		{"metric": {"job": "a"}, "value": [1704067560,"64"]}
	]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
