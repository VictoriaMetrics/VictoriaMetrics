package tests

import (
	"testing"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Data used in examples in
// https://docs.victoriametrics.com/keyconcepts/#instant-query and
// https://docs.victoriametrics.com/keyconcepts/#range-query
var docData = []string{
	"foo_bar 1.00 1652169600000", // 2022-05-10T08:00:00Z
	"foo_bar 2.00 1652169660000", // 2022-05-10T08:01:00Z
	"foo_bar 3.00 1652169720000", // 2022-05-10T08:02:00Z
	"foo_bar 5.00 1652169840000", // 2022-05-10T08:04:00Z, one point missed
	"foo_bar 5.50 1652169960000", // 2022-05-10T08:06:00Z, one point missed
	"foo_bar 5.50 1652170020000", // 2022-05-10T08:07:00Z
	"foo_bar 4.00 1652170080000", // 2022-05-10T08:08:00Z
	"foo_bar 3.50 1652170260000", // 2022-05-10T08:11:00Z, two points missed
	"foo_bar 3.25 1652170320000", // 2022-05-10T08:12:00Z
	"foo_bar 3.00 1652170380000", // 2022-05-10T08:13:00Z
	"foo_bar 2.00 1652170440000", // 2022-05-10T08:14:00Z
	"foo_bar 1.00 1652170500000", // 2022-05-10T08:15:00Z
	"foo_bar 4.00 1652170560000", // 2022-05-10T08:16:00Z
}

// TestSingleKeyConceptsQuery verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
// for vm-single.
func TestSingleKeyConceptsQuery(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testKeyConceptsQueryData(t, sut)
}

// TestClusterKeyConceptsQueryData verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
// for vm-cluster.
func TestClusterKeyConceptsQueryData(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testKeyConceptsQueryData(t, sut)
}

// testClusterKeyConceptsQuery verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
func testKeyConceptsQueryData(t *testing.T, sut at.PrometheusWriteQuerier) {

	// Insert example data from documentation.
	sut.PrometheusAPIV1ImportPrometheus(t, docData, at.QueryOpts{})
	sut.ForceFlush(t)

	testInstantQuery(t, sut)
	testRangeQuery(t, sut)
	testRangeQueryIsEquivalentToManyInstantQueries(t, sut)
}

// testInstantQuery verifies the statements made in the `Instant query` section
// of the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#instant-query
func testInstantQuery(t *testing.T, q at.PrometheusQuerier) {
	// Get the value of the foo_bar time series at 2022-05-10T08:03:00Z with the
	// step of 5m and timeout 5s. There is no sample at exactly this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-5m..time] interval.
	got := q.PrometheusAPIV1Query(t, "foo_bar", at.QueryOpts{Time: "2022-05-10T08:03:00.000Z", Step: "5m"})
	want := at.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[{"metric":{"__name__":"foo_bar"},"value":[1652169780,"3"]}]}}`)
	opt := cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	if diff := cmp.Diff(want, got, opt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Get the value of the foo_bar time series at 2022-05-10T08:18:00Z with the
	// step of 1m and timeout 5s. There is no sample at this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-1m..time] interval. Since the nearest sample is 2m away and the
	// step is 1m, then the VictoriaMetrics must return empty response.
	got = q.PrometheusAPIV1Query(t, "foo_bar", at.QueryOpts{Time: "2022-05-10T08:18:00.000Z", Step: "1m"})
	if len(got.Data.Result) > 0 {
		t.Errorf("unexpected response: got non-empty result, want empty result:\n%v", got)
	}
}

// testRangeQuery verifies the statements made in the `Range query` section of
// the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#range-query
func testRangeQuery(t *testing.T, q at.PrometheusQuerier) {
	f := func(start, end, step string, wantSamples []*at.Sample) {
		t.Helper()

		got := q.PrometheusAPIV1QueryRange(t, "foo_bar", at.QueryOpts{Start: start, End: end, Step: step})
		want := at.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "foo_bar"}, "values": []}]}}`)
		want.Data.Result[0].Samples = wantSamples
		opt := cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
		if diff := cmp.Diff(want, got, opt); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	// Verify the statement that the query result for
	// [2022-05-10T07:59:00Z..2022-05-10T08:17:00Z] time range and 1m step will
	// contain 17 points.
	f("2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "1m", []*at.Sample{
		// Sample for 2022-05-10T07:59:00Z is missing because the time series has
		// samples only starting from 8:00.
		at.NewSample(t, "2022-05-10T08:00:00Z", 1),
		at.NewSample(t, "2022-05-10T08:01:00Z", 2),
		at.NewSample(t, "2022-05-10T08:02:00Z", 3),
		at.NewSample(t, "2022-05-10T08:03:00Z", 3),
		at.NewSample(t, "2022-05-10T08:04:00Z", 5),
		at.NewSample(t, "2022-05-10T08:05:00Z", 5),
		at.NewSample(t, "2022-05-10T08:06:00Z", 5.5),
		at.NewSample(t, "2022-05-10T08:07:00Z", 5.5),
		at.NewSample(t, "2022-05-10T08:08:00Z", 4),
		at.NewSample(t, "2022-05-10T08:09:00Z", 4),
		// Sample for 2022-05-10T08:10:00Z is missing because there is no sample
		// within the [8:10 - 1m .. 8:10] interval.
		at.NewSample(t, "2022-05-10T08:11:00Z", 3.5),
		at.NewSample(t, "2022-05-10T08:12:00Z", 3.25),
		at.NewSample(t, "2022-05-10T08:13:00Z", 3),
		at.NewSample(t, "2022-05-10T08:14:00Z", 2),
		at.NewSample(t, "2022-05-10T08:15:00Z", 1),
		at.NewSample(t, "2022-05-10T08:16:00Z", 4),
		at.NewSample(t, "2022-05-10T08:17:00Z", 4),
	})

	// Verify the statement that a query is executed at start, start+step,
	// start+2*step, …, step+N*step timestamps, where N is the whole number
	// of steps that fit between start and end.
	f("2022-05-10T08:00:01.000Z", "2022-05-10T08:02:00.000Z", "1m", []*at.Sample{
		at.NewSample(t, "2022-05-10T08:00:01Z", 1),
		at.NewSample(t, "2022-05-10T08:01:01Z", 2),
	})

	// Verify the statement that a query is executed at start, start+step,
	// start+2*step, …, end timestamps, when end = start + N*step.
	f("2022-05-10T08:00:00.000Z", "2022-05-10T08:02:00.000Z", "1m", []*at.Sample{
		at.NewSample(t, "2022-05-10T08:00:00Z", 1),
		at.NewSample(t, "2022-05-10T08:01:00Z", 2),
		at.NewSample(t, "2022-05-10T08:02:00Z", 3),
	})

	// If the step isn’t set, then it defaults to 5m (5 minutes).
	f("2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "", []*at.Sample{
		// Sample for 2022-05-10T07:59:00Z is missing because the time series has
		// samples only starting from 8:00.
		at.NewSample(t, "2022-05-10T08:04:00Z", 5),
		at.NewSample(t, "2022-05-10T08:09:00Z", 4),
		at.NewSample(t, "2022-05-10T08:14:00Z", 2),
	})
}

// testRangeQueryIsEquivalentToManyInstantQueries verifies the statement made in
// the `Range query` section of the VictoriaMetrics documentation that a range
// query is actually an instant query executed 1 + (start-end)/step times on the
// time range from start to end. The only difference is that instant queries
// will not procude ephemeral points.
//
// See: https://docs.victoriametrics.com/keyconcepts/#range-query
func testRangeQueryIsEquivalentToManyInstantQueries(t *testing.T, q at.PrometheusQuerier) {
	f := func(timestamp string, want *at.Sample) {
		t.Helper()

		gotInstant := q.PrometheusAPIV1Query(t, "foo_bar", at.QueryOpts{Time: timestamp, Step: "1m"})
		if want == nil {
			if got, want := len(gotInstant.Data.Result), 0; got != want {
				t.Errorf("unexpected instant result size: got %d, want %d", got, want)
			}
		} else {
			got := gotInstant.Data.Result[0].Sample
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unexpected instant sample (-want, +got):\n%s", diff)
			}
		}
	}

	rangeRes := q.PrometheusAPIV1QueryRange(t, "foo_bar", at.QueryOpts{
		Start: "2022-05-10T07:59:00.000Z",
		End:   "2022-05-10T08:17:00.000Z",
		Step:  "1m",
	})
	rangeSamples := rangeRes.Data.Result[0].Samples

	f("2022-05-10T07:59:00.000Z", nil)
	f("2022-05-10T08:00:00.000Z", rangeSamples[0])
	f("2022-05-10T08:01:00.000Z", rangeSamples[1])
	f("2022-05-10T08:02:00.000Z", rangeSamples[2])
	f("2022-05-10T08:03:00.000Z", nil)
	f("2022-05-10T08:04:00.000Z", rangeSamples[4])
	f("2022-05-10T08:05:00.000Z", nil)
	f("2022-05-10T08:06:00.000Z", rangeSamples[6])
	f("2022-05-10T08:07:00.000Z", rangeSamples[7])
	f("2022-05-10T08:08:00.000Z", rangeSamples[8])
	f("2022-05-10T08:09:00.000Z", nil)
	f("2022-05-10T08:10:00.000Z", nil)
	f("2022-05-10T08:11:00.000Z", rangeSamples[10])
	f("2022-05-10T08:12:00.000Z", rangeSamples[11])
	f("2022-05-10T08:13:00.000Z", rangeSamples[12])
	f("2022-05-10T08:14:00.000Z", rangeSamples[13])
	f("2022-05-10T08:15:00.000Z", rangeSamples[14])
	f("2022-05-10T08:16:00.000Z", rangeSamples[15])
	f("2022-05-10T08:17:00.000Z", nil)
}

func TestSingleMillisecondPrecisionInInstantQueries(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testMillisecondPrecisionInInstantQueries(tc, sut)
}

func TestClusterMillisecondPrecisionInInstantQueries(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testMillisecondPrecisionInInstantQueries(tc, sut)
}

func testMillisecondPrecisionInInstantQueries(tc *at.TestCase, sut at.PrometheusWriteQuerier) {
	t := tc.T()

	type opts struct {
		query       string
		qtime       string
		step        string
		wantMetric  map[string]string
		wantSample  *at.Sample
		wantSamples []*at.Sample
	}
	f := func(sut at.PrometheusQuerier, opts *opts) {
		t.Helper()
		wantResult := []*at.QueryResult{}
		if opts.wantMetric != nil && (opts.wantSample != nil || len(opts.wantSamples) > 0) {
			wantResult = append(wantResult, &at.QueryResult{
				Metric:  opts.wantMetric,
				Sample:  opts.wantSample,
				Samples: opts.wantSamples,
			})
		}
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return sut.PrometheusAPIV1Query(t, opts.query, at.QueryOpts{
					Time: opts.qtime,
					Step: opts.step,
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{Data: &at.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`series1{label="foo"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`series1{label="foo"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{})
	sut.ForceFlush(t)

	// Verify that both points were created correctly. Fetch both points by
	// passing a duration into the instant query: the difference between the two
	// timestamps, plus 1ms (to include both points), so 101ms:
	f(sut, &opts{
		query:      "series1[101ms]",
		qtime:      "1707123456800", // 2024-02-05T08:57:36.800Z
		wantMetric: map[string]string{"__name__": "series1", "label": "foo"},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 20},
		},
	})

	// Search the first point at its own timestamp with step 1ms.
	f(sut, &opts{
		query:      "series1",
		qtime:      "1707123456700", // 2024-02-05T08:57:36.700Z
		step:       "1ms",
		wantMetric: map[string]string{"__name__": "series1", "label": "foo"},
		wantSample: &at.Sample{Timestamp: 1707123456700, Value: 10},
	})

	// Search the first point at 1ms past its own timestamp.
	// Expect empty result because the search interval (qtime-step, qtime]
	// excludes the timestamp of the first point.
	f(sut, &opts{
		query: "series1",
		qtime: "1707123456701", // 2024-02-05T08:57:36.701Z
		step:  "1ms",
	})

	// Fetch the last point at its timestamp with step 1ms.
	f(sut, &opts{
		query:      "series1",
		qtime:      "1707123456800", // 2024-02-05T08:57:36.800Z
		step:       "1ms",
		wantMetric: map[string]string{"__name__": "series1", "label": "foo"},
		wantSample: &at.Sample{Timestamp: 1707123456800, Value: 20},
	})

	// Fetch the last point at its timestamp with step 1ms.
	// Expect empty result because the search interval (qtime-step, qtime]
	// excludes the timestamp of the first point.
	f(sut, &opts{
		query: "series1",
		qtime: "1707123456801", // 2024-02-05T08:57:36.801Z
		step:  "1ms",
		// wantMetric: map[string]string{"__name__": "series1", "label": "foo"},
		// wantSample: &at.Sample{Timestamp: 1707123456801, Value: 20},
	})

	// Insert samples with different dates. The difference in ms between the two
	// timestamps is 4236579304.
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`series2{label="foo"} 10 1638564958042`, // 2021-12-03T20:55:58.042Z
		`series2{label="foo"} 20 1642801537346`, // 2022-01-21T21:45:37.346Z
	}, at.QueryOpts{})
	sut.ForceFlush(t)

	// Both Prometheus and VictoriaMetrics exclude the leftmost millisecond,
	// thus the following queries must return only one sample.
	f(sut, &opts{
		query:      "series2[4236579304ms]",
		qtime:      "1642801537346",
		step:       "1ms",
		wantMetric: map[string]string{"__name__": "series2", "label": "foo"},
		wantSamples: []*at.Sample{
			{Timestamp: 1642801537346, Value: 20},
		},
	})
	f(sut, &opts{
		query:      "count_over_time(series2[4236579304ms])",
		qtime:      "1642801537346", // 2022-01-21T21:45:37.346Z
		step:       "1ms",
		wantMetric: map[string]string{"label": "foo"},
		wantSample: &at.Sample{Timestamp: 1642801537346, Value: 1},
	})

	// Adding 1ms to the duration (4236579305ms) causes queries to return 2
	// samples.
	f(sut, &opts{
		query:      "series2[4236579305ms]",
		qtime:      "1642801537346",
		step:       "1ms",
		wantMetric: map[string]string{"__name__": "series2", "label": "foo"},
		wantSamples: []*at.Sample{
			{Timestamp: 1638564958042, Value: 10}, // 2021-12-03T20:55:58.042Z
			{Timestamp: 1642801537346, Value: 20},
		},
	})
	f(sut, &opts{
		query:      "count_over_time(series2[4236579305ms])",
		qtime:      "1642801537346", // 2022-01-21T21:45:37.346Z
		step:       "1ms",
		wantMetric: map[string]string{"label": "foo"},
		wantSample: &at.Sample{Timestamp: 1642801537346, Value: 2},
	})
}
