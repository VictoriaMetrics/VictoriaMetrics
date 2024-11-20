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

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})

	opts := apptest.QueryOpts{Timeout: "5s"}

	vmsingle.PrometheusAPIV1Write(t, staleNaNsData, opts)
	vmsingle.ForceFlush(t)

	testInstantQueryStaleNaNs(t, vmsingle, opts)
}

func TestClusterInstantQueryStaleNaNs(t *testing.T) {
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

	vminsert.PrometheusAPIV1Write(t, staleNaNsData, opts)
	time.Sleep(2 * time.Second)

	vmstorage1.ForceFlush(t)
	vmstorage2.ForceFlush(t)

	testInstantQueryStaleNaNs(t, vmselect, opts)
}

func testInstantQueryStaleNaNs(t *testing.T, q apptest.PrometheusQuerier, opts apptest.QueryOpts) {
	got := q.PrometheusAPIV1Query(t, "metric[5m]", "2024-01-01T00:04:00.000Z", "5m", opts)
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
