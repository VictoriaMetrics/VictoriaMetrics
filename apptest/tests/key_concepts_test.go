package tests

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
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
func TestSingleKeyConceptsQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})

	opts := apptest.QueryOpts{Timeout: "5s"}

	// Insert example data from documentation.
	vmsingle.PrometheusAPIV1ImportPrometheus(t, docData, opts)
	vmsingle.ForceFlush(t)

	testInstantQuery(t, vmsingle, opts)
	testRangeQuery(t, vmsingle, opts)
	testRangeQueryIsEquivalentToManyInstantQueries(t, vmsingle, opts)
}

// TestClusterKeyConceptsQuery verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
func TestClusterKeyConceptsQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Set up the following cluster configuration:
	//
	// - two vmstorage instances
	// - vminsert points to the two vmstorages, its replication setting
	//   is off which means it will only shard the incoming data across the two
	//   vmstorages.
	// - vmselect points to the two vmstorages and is expected to query both
	//   vmstorages and build the full result out of the two partial results.

	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
		"-retentionPeriod=100y",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	})

	opts := apptest.QueryOpts{Timeout: "5s", Tenant: "0"}

	// Insert example data from documentation.
	vminsert.PrometheusAPIV1ImportPrometheus(t, docData, opts)
	time.Sleep(2 * time.Second)

	vmstorage1.ForceFlush(t)
	vmstorage2.ForceFlush(t)

	testInstantQuery(t, vmselect, opts)
	testRangeQuery(t, vmselect, opts)
	testRangeQueryIsEquivalentToManyInstantQueries(t, vmselect, opts)
}

// testInstantQuery verifies the statements made in the `Instant query` section
// of the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#instant-query
func testInstantQuery(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	// Get the value of the foo_bar time series at 2022-05-10T08:03:00Z with the
	// step of 5m and timeout 5s. There is no sample at exactly this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-5m..time] interval.
	got := q.PrometheusAPIV1Query(t, "foo_bar", "2022-05-10T08:03:00.000Z", "5m", opts)
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[{"metric":{"__name__":"foo_bar"},"value":[1652169780,"3"]}]}}`)
	opt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	if diff := cmp.Diff(want, got, opt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Get the value of the foo_bar time series at 2022-05-10T08:18:00Z with the
	// step of 1m and timeout 5s. There is no sample at this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-1m..time] interval. Since the nearest sample is 2m away and the
	// step is 1m, then the VictoriaMetrics must return empty response.
	got = q.PrometheusAPIV1Query(t, "foo_bar", "2022-05-10T08:18:00.000Z", "1m", opts)
	if len(got.Data.Result) > 0 {
		t.Errorf("unexpected response: got non-empty result, want empty result:\n%v", got)
	}
}

// testRangeQuery verifies the statements made in the `Range query` section of
// the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#range-query
func testRangeQuery(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	f := func(start, end, step string, wantSamples []*apptest.Sample) {
		t.Helper()

		got := q.PrometheusAPIV1QueryRange(t, "foo_bar", start, end, step, opts)
		want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "foo_bar"}, "values": []}]}}`)
		want.Data.Result[0].Samples = wantSamples
		opt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
		if diff := cmp.Diff(want, got, opt); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	// Verify the statement that the query result for
	// [2022-05-10T07:59:00Z..2022-05-10T08:17:00Z] time range and 1m step will
	// contain 17 points.
	f("2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "1m", []*apptest.Sample{
		// Sample for 2022-05-10T07:59:00Z is missing because the time series has
		// samples only starting from 8:00.
		apptest.NewSample(t, "2022-05-10T08:00:00Z", 1),
		apptest.NewSample(t, "2022-05-10T08:01:00Z", 2),
		apptest.NewSample(t, "2022-05-10T08:02:00Z", 3),
		apptest.NewSample(t, "2022-05-10T08:03:00Z", 3),
		apptest.NewSample(t, "2022-05-10T08:04:00Z", 5),
		apptest.NewSample(t, "2022-05-10T08:05:00Z", 5),
		apptest.NewSample(t, "2022-05-10T08:06:00Z", 5.5),
		apptest.NewSample(t, "2022-05-10T08:07:00Z", 5.5),
		apptest.NewSample(t, "2022-05-10T08:08:00Z", 4),
		apptest.NewSample(t, "2022-05-10T08:09:00Z", 4),
		// Sample for 2022-05-10T08:10:00Z is missing because there is no sample
		// within the [8:10 - 1m .. 8:10] interval.
		apptest.NewSample(t, "2022-05-10T08:11:00Z", 3.5),
		apptest.NewSample(t, "2022-05-10T08:12:00Z", 3.25),
		apptest.NewSample(t, "2022-05-10T08:13:00Z", 3),
		apptest.NewSample(t, "2022-05-10T08:14:00Z", 2),
		apptest.NewSample(t, "2022-05-10T08:15:00Z", 1),
		apptest.NewSample(t, "2022-05-10T08:16:00Z", 4),
		apptest.NewSample(t, "2022-05-10T08:17:00Z", 4),
	})

	// Verify the statement that a query is executed at start, start+step,
	// start+2*step, …, step+N*step timestamps, where N is the whole number
	// of steps that fit between start and end.
	f("2022-05-10T08:00:01.000Z", "2022-05-10T08:02:00.000Z", "1m", []*apptest.Sample{
		apptest.NewSample(t, "2022-05-10T08:00:01Z", 1),
		apptest.NewSample(t, "2022-05-10T08:01:01Z", 2),
	})

	// Verify the statement that a query is executed at start, start+step,
	// start+2*step, …, end timestamps, when end = start + N*step.
	f("2022-05-10T08:00:00.000Z", "2022-05-10T08:02:00.000Z", "1m", []*apptest.Sample{
		apptest.NewSample(t, "2022-05-10T08:00:00Z", 1),
		apptest.NewSample(t, "2022-05-10T08:01:00Z", 2),
		apptest.NewSample(t, "2022-05-10T08:02:00Z", 3),
	})

	// If the step isn’t set, then it defaults to 5m (5 minutes).
	f("2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "", []*apptest.Sample{
		// Sample for 2022-05-10T07:59:00Z is missing because the time series has
		// samples only starting from 8:00.
		apptest.NewSample(t, "2022-05-10T08:04:00Z", 5),
		apptest.NewSample(t, "2022-05-10T08:09:00Z", 4),
		apptest.NewSample(t, "2022-05-10T08:14:00Z", 2),
	})
}

// testRangeQueryIsEquivalentToManyInstantQueries verifies the statement made in
// the `Range query` section of the VictoriaMetrics documentation that a range
// query is actually an instant query executed 1 + (start-end)/step times on the
// time range from start to end. See:
// https://docs.victoriametrics.com/keyconcepts/#range-query
func testRangeQueryIsEquivalentToManyInstantQueries(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	f := func(timestamp string, want *apptest.Sample) {
		t.Helper()

		gotInstant := q.PrometheusAPIV1Query(t, "foo_bar", timestamp, "1m", opts)
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

	rangeRes := q.PrometheusAPIV1QueryRange(t, "foo_bar", "2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "1m", opts)
	rangeSamples := rangeRes.Data.Result[0].Samples

	f("2022-05-10T07:59:00.000Z", nil)
	f("2022-05-10T08:00:00.000Z", rangeSamples[0])
	f("2022-05-10T08:01:00.000Z", rangeSamples[1])
	f("2022-05-10T08:02:00.000Z", rangeSamples[2])
	f("2022-05-10T08:03:00.000Z", rangeSamples[3])
	f("2022-05-10T08:04:00.000Z", rangeSamples[4])
	f("2022-05-10T08:05:00.000Z", rangeSamples[5])
	f("2022-05-10T08:06:00.000Z", rangeSamples[6])
	f("2022-05-10T08:07:00.000Z", rangeSamples[7])
	f("2022-05-10T08:08:00.000Z", rangeSamples[8])
	f("2022-05-10T08:09:00.000Z", rangeSamples[9])
	f("2022-05-10T08:10:00.000Z", nil)
	f("2022-05-10T08:11:00.000Z", rangeSamples[10])
	f("2022-05-10T08:12:00.000Z", rangeSamples[11])
	f("2022-05-10T08:13:00.000Z", rangeSamples[12])
	f("2022-05-10T08:14:00.000Z", rangeSamples[13])
	f("2022-05-10T08:15:00.000Z", rangeSamples[14])
	f("2022-05-10T08:16:00.000Z", rangeSamples[15])
	f("2022-05-10T08:17:00.000Z", rangeSamples[16])
}
