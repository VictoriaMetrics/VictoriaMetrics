package tests

import (
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestClusterMultiTenantSelect(t *testing.T) {
	os.RemoveAll(t.Name())

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	cmpSROpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1SeriesResponse{}, "Status", "IsPartial")

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
	})

	var commonSamples = []string{
		`foo_bar 1.00 1652169600000`, // 2022-05-10T08:00:00Z
		`foo_bar 2.00 1652169660000`, // 2022-05-10T08:01:00Z
		`foo_bar 3.00 1652169720000`, // 2022-05-10T08:02:00Z
	}

	// test for empty tenants request
	got := vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
		Tenant: "multitenant",
		Step:   "5m",
		Time:   "2022-05-10T08:03:00.000Z",
	})
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// ingest per tenant data and verify it with search
	tenantIDs := []string{"1:1", "1:15"}
	instantCT := "2022-05-10T08:05:00.000Z"
	for _, tenantID := range tenantIDs {
		vminsert.PrometheusAPIV1ImportPrometheus(t, commonSamples, apptest.QueryOpts{Tenant: tenantID})
		vmstorage.ForceFlush(t)
		got := vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
			Tenant: tenantID, Time: instantCT,
		})
		want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[{"metric":{"__name__":"foo_bar"},"value":[1652169900,"3"]}]}}`)
		if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	// verify all tenants searchable with multitenant APIs

	//  /api/v1/query
	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
       {"result":[
          {"metric":{"__name__":"foo_bar","vm_account_id":"1","vm_project_id": "1"},"value":[1652169900,"3"]},
          {"metric":{"__name__":"foo_bar","vm_account_id":"1","vm_project_id":"15"},"value":[1652169900,"3"]}
                 ]
       }
     }`,
	)
	got = vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
		Tenant: "multitenant",
		Time:   instantCT,
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// /api/v1/query_range aggregated by tenant labels
	query := "sum(foo_bar) by(vm_account_id,vm_project_id)"
	got = vmselect.PrometheusAPIV1QueryRange(t, query, apptest.QueryOpts{
		Tenant: "multitenant",
		Start:  "2022-05-10T07:59:00.000Z",
		End:    "2022-05-10T08:05:00.000Z",
		Step:   "1m",
	})

	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data": 
        {"result": [
          {"metric": {"vm_account_id": "1","vm_project_id":"1"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]},
          {"metric": {"vm_account_id": "1","vm_project_id":"15"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]}
                   ]
        }
     }`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// verify /api/v1/series response

	wantSR := apptest.NewPrometheusAPIV1SeriesResponse(t,
		`{"data": [
        {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"1"},
        {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"15"}
              ]
     }`)
	wantSR.Sort()

	gotSR := vmselect.PrometheusAPIV1Series(t, "foo_bar", apptest.QueryOpts{
		Tenant: "multitenant",
		Start:  "2022-05-10T08:03:00.000Z",
	})
	gotSR.Sort()
	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// test multitenant ingest path, tenants must be populated from labels
	//
	var tenantLabelsSamples = []string{
		`foo_bar{vm_account_id="5"} 1.00 1652169600000`,                    // 2022-05-10T08:00:00Z'
		`foo_bar{vm_project_id="10"} 2.00 1652169660000`,                   // 2022-05-10T08:01:00Z
		`foo_bar{vm_account_id="5",vm_project_id="15"} 3.00 1652169720000`, // 2022-05-10T08:02:00Z
	}

	vminsert.PrometheusAPIV1ImportPrometheus(t, tenantLabelsSamples, apptest.QueryOpts{Tenant: "multitenant"})
	vmstorage.ForceFlush(t)

	//  /api/v1/query with query filters
	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
        {"result":[
	         {"metric":{"__name__":"foo_bar","vm_account_id":"5","vm_project_id": "0"},"value":[1652169900,"1"]},
	         {"metric":{"__name__":"foo_bar","vm_account_id":"5","vm_project_id":"15"},"value":[1652169900,"3"]}
	                ]
        }
    }`,
	)
	got = vmselect.PrometheusAPIV1Query(t, `foo_bar{vm_account_id="5"}`, apptest.QueryOpts{
		Time:   instantCT,
		Tenant: "multitenant",
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// /api/v1/series with extra_filters

	wantSR = apptest.NewPrometheusAPIV1SeriesResponse(t,
		`{"data": [
	       {"__name__":"foo_bar", "vm_account_id":"5", "vm_project_id":"15"},
	       {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"15"}
	             ]
	   }`)
	wantSR.Sort()
	gotSR = vmselect.PrometheusAPIV1Series(t, "foo_bar", apptest.QueryOpts{
		Start:        "2022-05-10T08:00:00.000Z",
		End:          "2022-05-10T08:30:00.000Z",
		ExtraFilters: []string{`{vm_project_id="15"}`},
		Tenant:       "multitenant",
	})
	gotSR.Sort()

	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

}
