package tests

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestClusterMaxUniqueTimeseries(t *testing.T) {
	os.RemoveAll(t.Name())

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-search.maxUniqueTimeseries=2",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectNoLimit := tc.MustStartVmselect("vmselect1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
	})
	vmselectSmallLimit := tc.MustStartVmselect("vmselect2", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-search.maxUniqueTimeseries=1",
	})
	vmselectBigLimit := tc.MustStartVmselect("vmselect3", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-search.maxUniqueTimeseries=3",
	})

	var commonSamples = []string{
		`foo_bar1{instance="a"} 1.00 1652169660000`,

		`foo_bar2{instance="a"} 1.00 1652169660000`,
		`foo_bar2{instance="b"} 2.00 1652169660000`,

		`foo_bar3{instance="a"} 1.00 1652169660000`,
		`foo_bar3{instance="b"} 2.00 1652169660000`,
		`foo_bar3{instance="c"} 3.00 1652169660000`,
	}

	// write data to two tenants
	tenantIDs := []string{"0:0", "1:15"}
	for _, tenantID := range tenantIDs {
		vminsert.PrometheusAPIV1ImportPrometheus(t, commonSamples, apptest.QueryOpts{Tenant: tenantID})
		vmstorage.ForceFlush(t)
	}

	instantCT := "2022-05-10T08:05:00.000Z"

	// success - `/api/v1/query`
	want := apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
       {"result":[
          {"metric":{"__name__":"foo_bar1","instance":"a"},"value":[1652169900,"1"]}
        ]
       }
     }`,
	)
	queryRes := vmselectSmallLimit.PrometheusAPIV1Query(t, "foo_bar1", apptest.QueryOpts{
		Time: instantCT,
	})
	if diff := cmp.Diff(want, queryRes, cmpOpt); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}

	// success - multitenant `/api/v1/query`
	// query is split into two queries for each tenant, so the final result can exceed the limit.
	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
       {"result":[
          {"metric":{"__name__":"foo_bar1","instance":"a","vm_account_id":"0","vm_project_id":"0"},"value":[1652169900,"1"]},
          {"metric":{"__name__":"foo_bar1","instance":"a","vm_account_id":"1","vm_project_id":"15"},"value":[1652169900,"1"]}
        ]
       }
     }`,
	)
	queryRes = vmselectSmallLimit.PrometheusAPIV1Query(t, "foo_bar1", apptest.QueryOpts{
		Time:   instantCT,
		Tenant: "multitenant",
	})
	if diff := cmp.Diff(want, queryRes, cmpOpt); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}

	// fail - `/api/v1/query`, exceed vmselect `maxUniqueTimeseries`
	queryRes = vmselectSmallLimit.PrometheusAPIV1Query(t, "foo_bar2", apptest.QueryOpts{
		Time: instantCT,
	})
	if queryRes.ErrorType != "422" {
		t.Fatalf("unexpected status code, got %s, want %d, error message is: %v", queryRes.ErrorType, 422, queryRes.Error)
	}

	// fail - `/api/v1/query`, exceed vmstorage `maxUniqueTimeseries`
	queryRes = vmselectNoLimit.PrometheusAPIV1Query(t, "foo_bar3", apptest.QueryOpts{
		Time: instantCT,
	})
	if queryRes.ErrorType != "422" {
		t.Fatalf("unexpected status code, got %s, want %d, error message is: %v", queryRes.ErrorType, 422, queryRes.Error)
	}

	// fail - `/api/v1/query`, vmselect `maxUniqueTimeseries` cannot exceed vmstorage `maxUniqueTimeseries`
	queryRes = vmselectBigLimit.PrometheusAPIV1Query(t, "foo_bar3", apptest.QueryOpts{
		Time: instantCT,
	})
	if queryRes.ErrorType != "422" {
		t.Fatalf("unexpected status code, got %s, want %d, error message is: %v", queryRes.ErrorType, 422, queryRes.Error)
	}
}

func TestClusterMaxSeries(t *testing.T) {
	os.RemoveAll(t.Name())

	cmpSROpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1SeriesResponse{}, "Status", "IsPartial")

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-search.maxUniqueTimeseries=2",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectBigLimit := tc.MustStartVmselect("vmselect2", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-search.maxSeries=3",
	})
	vmselectSmallLimit := tc.MustStartVmselect("vmselect1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-search.maxSeries=1",
	})

	var commonSamples = []string{
		`foo_bar3{instance="a"} 1.00 1652169660000`,
		`foo_bar3{instance="b"} 2.00 1652169660000`,
		`foo_bar3{instance="c"} 3.00 1652169660000`,
	}

	// write data
	vminsert.PrometheusAPIV1ImportPrometheus(t, commonSamples, apptest.QueryOpts{})
	vmstorage.ForceFlush(t)

	// success - `/api/v1/series`, vmselect `maxLabelsAPISeries` can exceed vmstorage `maxLabelsAPISeries``
	wantSR := apptest.NewPrometheusAPIV1SeriesResponse(t,
		`{"data": [
			{"__name__":"foo_bar3","instance":"a"},
			{"__name__":"foo_bar3","instance":"b"},
			{"__name__":"foo_bar3","instance":"c"}
			]
		 }`)
	seriesRes := vmselectBigLimit.PrometheusAPIV1Series(t, "foo_bar3", apptest.QueryOpts{
		Start: "2022-05-10T08:03:00.000Z",
	})
	if diff := cmp.Diff(wantSR.Sort(), seriesRes.Sort(), cmpSROpt); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}

	// fail - `/api/v1/series`, exceed vmselect `maxSeries`
	seriesRes1 := vmselectSmallLimit.PrometheusAPIV1Series(t, "foo_bar3", apptest.QueryOpts{
		Start: "2022-05-10T08:03:00.000Z",
	})
	if seriesRes1.ErrorType != "422" {
		t.Fatalf("unexpected status code, got %s, want %d, error message is: %v", seriesRes.ErrorType, 422, seriesRes1.Error)
	}

}
