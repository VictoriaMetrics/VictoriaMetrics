package tests

import (
	"testing"

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

// TestVmsingleKeyConceptsQuery verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
func TestVmsingleKeyConceptsQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Close()

	cli := tc.Client()

	vmsingle := apptest.MustStartVmsingle(t, "vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	}, cli)
	defer vmsingle.Stop()

	opts := apptest.QueryOpts{Timeout: "5s"}

	// Insert example data from documentation.
	vmsingle.PrometheusAPIV1ImportPrometheus(t, docData, opts)
	vmsingle.ForceFlush(t)

	testInstantQuery(t, vmsingle, opts)
	testRangeQuery(t, vmsingle, opts)
}

// TestClusterKeyConceptsQuery verifies cases from https://docs.victoriametrics.com/keyconcepts/#query-data
func TestClusterKeyConceptsQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Close()

	// Set up the following cluster configuration:
	//
	// - two vmstorage instances
	// - vminsert points to the two vmstorages, its replication setting
	//   is off which means it will only shard the incoming data across the two
	//   vmstorages.
	// - vmselect points to the two vmstorages and is expected to query both
	//   vmstorages and build the full result out of the two partial results.

	cli := tc.Client()

	vmstorage1 := apptest.MustStartVmstorage(t, "vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
	}, cli)
	defer vmstorage1.Stop()
	vmstorage2 := apptest.MustStartVmstorage(t, "vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
	}, cli)
	defer vmstorage2.Stop()
	vminsert := apptest.MustStartVminsert(t, "vminsert", []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr(),
	}, cli)
	defer vminsert.Stop()
	vmselect := apptest.MustStartVmselect(t, "vmselect", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	}, cli)
	defer vmselect.Stop()

	opts := apptest.QueryOpts{Timeout: "5s", Tenant: "0"}

	// Insert example data from documentation.
	vminsert.PrometheusAPIV1ImportPrometheus(t, docData, opts)
	vmstorage1.ForceFlush(t)
	vmstorage2.ForceFlush(t)

	testInstantQuery(t, vmselect, opts)
	testRangeQuery(t, vmselect, opts)
}

// vmsingleInstantQuery verifies the statements made in the
// `Instant query` section of the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#instant-query
func testInstantQuery(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	// Get the value of the foo_bar time series at 2022-05-10Z08:03:00Z with the
	// step of 5m and timeout 5s. There is no sample at exactly this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-5m..time] interval.
	got := q.PrometheusAPIV1Query(t, "foo_bar", "2022-05-10T08:03:00.000Z", "5m", opts)
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[{"metric":{"__name__":"foo_bar"},"value":[1652169780,"3"]}]}}`)
	opt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	if diff := cmp.Diff(got, want, opt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Get the value of the foo_bar time series at 2022-05-10Z08:18:00Z with the
	// step of 1m and timeout 5s. There is no sample at this timestamp.
	// Therefore, VictoriaMetrics will search for the nearest sample within the
	// [time-1m..time] interval. Since the nearest sample is 2m away and the
	// step is 1m, then the VictoriaMetrics must return empty response.
	got = q.PrometheusAPIV1Query(t, "foo_bar", "2022-05-10T08:18:00.000Z", "1m", opts)
	if len(got.Data.Result) > 0 {
		t.Errorf("unexpected response: got non-empty result, want empty result:\n%v", got)
	}
}

// vmsingleRangeQuery verifies the statements made in the
// `Range query` section of the VictoriaMetrics documentation. See:
// https://docs.victoriametrics.com/keyconcepts/#range-query
func testRangeQuery(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	// Get the values of the foo_bar time series for
	// [2022-05-10Z07:59:00Z..2022-05-10Z08:17:00Z] time interval with the step
	// of 1m and timeout 5s.
	got := q.PrometheusAPIV1QueryRange(t, "foo_bar", "2022-05-10T07:59:00.000Z", "2022-05-10T08:17:00.000Z", "1m", opts)
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "foo_bar"}, "values": []}]}}`)
	s := make([]*apptest.Sample, 17)
	// Sample for 2022-05-10T07:59:00Z is missing because the time series has
	// samples only starting from 8:00.
	s[0] = apptest.NewSample(t, "2022-05-10T08:00:00Z", 1)
	s[1] = apptest.NewSample(t, "2022-05-10T08:01:00Z", 2)
	s[2] = apptest.NewSample(t, "2022-05-10T08:02:00Z", 3)
	s[3] = apptest.NewSample(t, "2022-05-10T08:03:00Z", 3)
	s[4] = apptest.NewSample(t, "2022-05-10T08:04:00Z", 5)
	s[5] = apptest.NewSample(t, "2022-05-10T08:05:00Z", 5)
	s[6] = apptest.NewSample(t, "2022-05-10T08:06:00Z", 5.5)
	s[7] = apptest.NewSample(t, "2022-05-10T08:07:00Z", 5.5)
	s[8] = apptest.NewSample(t, "2022-05-10T08:08:00Z", 4)
	s[9] = apptest.NewSample(t, "2022-05-10T08:09:00Z", 4)
	// Sample for 2022-05-10T08:10:00Z is missing because there is no sample
	// within the [8:10 - 1m .. 8:10] interval.
	s[10] = apptest.NewSample(t, "2022-05-10T08:11:00Z", 3.5)
	s[11] = apptest.NewSample(t, "2022-05-10T08:12:00Z", 3.25)
	s[12] = apptest.NewSample(t, "2022-05-10T08:13:00Z", 3)
	s[13] = apptest.NewSample(t, "2022-05-10T08:14:00Z", 2)
	s[14] = apptest.NewSample(t, "2022-05-10T08:15:00Z", 1)
	s[15] = apptest.NewSample(t, "2022-05-10T08:16:00Z", 4)
	s[16] = apptest.NewSample(t, "2022-05-10T08:17:00Z", 4)
	want.Data.Result[0].Samples = s
	opt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	if diff := cmp.Diff(got, want, opt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
