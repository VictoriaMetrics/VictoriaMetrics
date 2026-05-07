package tests

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestClusterMultiTenantSelectViaHeaders(t *testing.T) {
	fs.MustRemoveDir(t.Name())

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
		"-enableMultitenancyViaHeaders",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-enableMultitenancyViaHeaders",
	})

	multitenant := make(http.Header)
	multitenant.Set("AccountID", "multitenant")

	// test for empty tenants request
	got := vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
		Headers: multitenant,
		Step:    "5m",
		Time:    "2022-05-10T08:03:00.000Z",
	})
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// ingest per tenant data and verify it with search
	samples := []string{
		`foo_bar 1.00 1652169600000`, // 2022-05-10T08:00:00Z
		`foo_bar 2.00 1652169660000`, // 2022-05-10T08:01:00Z
		`foo_bar 3.00 1652169720000`, // 2022-05-10T08:02:00Z
	}
	tenantHeaders := []map[string]string{
		{"AccountID": "1", "ProjectID": "1"},
		{"AccountID": "1", "ProjectID": "15"},
		{"AccountID": "2"},
		{"ProjectID": "3"},
	}
	instantCT := "2022-05-10T08:05:00.000Z" // 1652169900 Unix seconds
	for _, headers := range tenantHeaders {
		h := make(http.Header)
		for k, v := range headers {
			h.Set(k, v)
		}
		vminsert.PrometheusAPIV1ImportPrometheus(t, samples, apptest.QueryOpts{Headers: h})
		vmstorage.ForceFlush(t)

		// verify tenants are searchable via tenantID in headers
		got := vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
			Headers: h, Time: instantCT,
		})
		want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[{"metric":{"__name__":"foo_bar"},"value":[1652169900,"3"]}]}}`)
		if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	// verify all tenants searchable with multitenant header

	//  /api/v1/query
	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
       {"result":[
          {"metric":{"__name__":"foo_bar","vm_account_id":"0","vm_project_id":"3"},"value":[1652169900,"3"]},
          {"metric":{"__name__":"foo_bar","vm_account_id":"1","vm_project_id": "1"},"value":[1652169900,"3"]},
          {"metric":{"__name__":"foo_bar","vm_account_id":"1","vm_project_id":"15"},"value":[1652169900,"3"]},
          {"metric":{"__name__":"foo_bar","vm_account_id":"2","vm_project_id":"0"},"value":[1652169900,"3"]}
                 ]
       }
     }`,
	)

	got = vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
		Headers: multitenant,
		Time:    instantCT,
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// /api/v1/query_range aggregated by tenant labels
	query := "sum(foo_bar) by(vm_account_id,vm_project_id)"
	got = vmselect.PrometheusAPIV1QueryRange(t, query, apptest.QueryOpts{
		Headers: multitenant,
		Start:   "2022-05-10T07:59:00.000Z",
		End:     "2022-05-10T08:05:00.000Z",
		Step:    "1m",
	})

	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data": 
        {"result": [
          {"metric": {"vm_account_id": "0","vm_project_id":"3"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]},
          {"metric": {"vm_account_id": "1","vm_project_id":"1"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]},
          {"metric": {"vm_account_id": "1","vm_project_id":"15"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]},
          {"metric": {"vm_account_id": "2","vm_project_id":"0"}, "values": [[1652169600,"1"],[1652169660,"2"],[1652169720,"3"],[1652169780,"3"]]}
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
        {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"15"},
        {"__name__":"foo_bar", "vm_account_id":"2", "vm_project_id":"0"},
        {"__name__":"foo_bar", "vm_account_id":"0", "vm_project_id":"3"}
              ]
     }`)
	wantSR.Sort()

	gotSR := vmselect.PrometheusAPIV1Series(t, "foo_bar", apptest.QueryOpts{
		Headers: multitenant,
		Start:   "2022-05-10T08:03:00.000Z",
	})
	gotSR.Sort()
	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// test ingestion with multitenant header, tenants must be populated from labels
	//
	var tenantLabelsSamples = []string{
		`foo_bar{vm_account_id="5"} 1.00 1652169720000`,                    // 2022-05-10T08:02:00Z'
		`foo_bar{vm_project_id="10"} 2.00 1652169660000`,                   // 2022-05-10T08:01:00Z
		`foo_bar{vm_account_id="5",vm_project_id="15"} 3.00 1652169720000`, // 2022-05-10T08:02:00Z
	}

	vminsert.PrometheusAPIV1ImportPrometheus(t, tenantLabelsSamples, apptest.QueryOpts{Headers: multitenant})
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
		Time:    instantCT,
		Headers: multitenant,
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
		Headers:      multitenant,
	})
	gotSR.Sort()

	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// /api/v1/label/../value with extra_filters

	wantVR := apptest.NewPrometheusAPIV1LabelValuesResponse(t,
		`{"data": [
	       "5"
	             ]
	   }`)
	// matchQuery is ignored for /api/v1/label/<labelName>/values lookups with multitenant token
	gotVR := vmselect.PrometheusAPIV1LabelValues(t, "vm_account_id", "xxx", apptest.QueryOpts{
		Start:        "2022-05-10T08:00:00.000Z",
		End:          "2022-05-10T08:30:00.000Z",
		ExtraFilters: []string{`{vm_account_id="5"}`},
		Headers:      multitenant,
	})
	gotSR.Sort()

	if diff := cmp.Diff(wantVR, gotVR, cmpopts.IgnoreFields(apptest.PrometheusAPIV1LabelValuesResponse{}, "Status", "IsPartial")); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Delete series from specific tenant
	tenantID := make(http.Header)
	tenantID.Set("AccountID", "5")
	tenantID.Set("ProjectID", "15")
	vmselect.APIV1AdminTSDBDeleteSeries(t, "foo_bar", apptest.QueryOpts{
		Headers: tenantID,
	})
	wantSR = apptest.NewPrometheusAPIV1SeriesResponse(t,
		`{"data": [
        {"__name__":"foo_bar", "vm_account_id":"0", "vm_project_id":"3"},
        {"__name__":"foo_bar", "vm_account_id":"0", "vm_project_id":"10"},
        {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"1"},
        {"__name__":"foo_bar", "vm_account_id":"1", "vm_project_id":"15"},
        {"__name__":"foo_bar", "vm_account_id":"2", "vm_project_id":"0"},
        {"__name__":"foo_bar", "vm_account_id":"5", "vm_project_id":"0"}
              ]
     }`)
	wantSR.Sort()

	gotSR = vmselect.PrometheusAPIV1Series(t, "foo_bar", apptest.QueryOpts{
		Headers: multitenant,
		Start:   "2022-05-10T08:03:00.000Z",
	})
	gotSR.Sort()
	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Delete series for multitenant with tenant filter
	vmselect.APIV1AdminTSDBDeleteSeries(t, `foo_bar{vm_account_id="1"}`, apptest.QueryOpts{
		Headers: multitenant,
	})

	wantSR = apptest.NewPrometheusAPIV1SeriesResponse(t,
		`{"data": [
        {"__name__":"foo_bar", "vm_account_id":"0", "vm_project_id":"3"},
        {"__name__":"foo_bar", "vm_account_id":"0", "vm_project_id":"10"},
        {"__name__":"foo_bar", "vm_account_id":"2", "vm_project_id":"0"},
        {"__name__":"foo_bar", "vm_account_id":"5", "vm_project_id":"0"}
              ]
     }`)
	wantSR.Sort()

	gotSR = vmselect.PrometheusAPIV1Series(t, `foo_bar`, apptest.QueryOpts{
		Headers: multitenant,
		Start:   "2022-05-10T08:03:00.000Z",
	})
	gotSR.Sort()
	if diff := cmp.Diff(wantSR, gotSR, cmpSROpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	if got := vmselect.GetIntMetric(t, `vm_cache_requests_total{type="multitenancy/tenants"}`); got != 0 {
		t.Errorf("unexpected multitenancy tenants cache requests; got %d; want 0", got)
	}

	if got := vmselect.GetIntMetric(t, `vm_cache_misses_total{type="multitenancy/tenants"}`); got != 0 {
		t.Errorf("unexpected multitenancy tenants cache misses; got %d; want 0", got)
	}

	if got := vmselect.GetIntMetric(t, `vm_cache_entries{type="multitenancy/tenants"}`); got != 0 {
		t.Errorf("unexpected multitenancy tenants cache entries; got %d; want 0", got)
	}

	// verify that tenant in path has priority over tenant specified in headers

	// /api/v1/import/prometheus

	tenantInHeader := make(http.Header)
	tenantInHeader.Set("AccountID", "42")
	tenantInPath := "112"
	vminsert.PrometheusAPIV1ImportPrometheus(t, samples, apptest.QueryOpts{
		// tenants in header and path clash - path should have higher priority on ingestion
		Headers: tenantInHeader,
		Tenant:  "112",
	})
	vmstorage.ForceFlush(t)

	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
       {"result":[
          {"metric":{"__name__":"foo_bar"},"value":[1652169900,"3"]}
                 ]
       }
     }`,
	)
	got = vmselect.PrometheusAPIV1Query(t, "foo_bar", apptest.QueryOpts{
		// tenants in header and path clash - path should have higher priority on ingestion
		Headers: multitenant,
		Tenant:  tenantInPath,
		Time:    instantCT,
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
