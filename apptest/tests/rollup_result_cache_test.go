package tests

import (
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestClusterRollupResultCache(t *testing.T) {
	os.RemoveAll(t.Name())

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

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

	var tenantLabelsSamples = []string{
		`foo_bar{vm_account_id="5"} 1.00 1652169720000`,                    // 2022-05-10T08:00:00Z'
		`foo_bar{vm_account_id="5",vm_project_id="15"} 3.00 1652169720000`, // 2022-05-10T08:02:00Z
	}

	vminsert.PrometheusAPIV1ImportPrometheus(t, tenantLabelsSamples, apptest.QueryOpts{Tenant: "multitenant"})
	vmstorage.ForceFlush(t)

	want := apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
	   {"result":[
	        {"metric":{"__name__":"foo_bar","vm_account_id":"5","vm_project_id": "0"},"values":[[1652169720,"1"],[1652169780,"1"]]},
	        {"metric":{"__name__":"foo_bar","vm_account_id":"5","vm_project_id":"15"},"values":[[1652169720,"3"],[1652169780,"3"]]}
	               ]
	   }
	}`,
	)

	got := vmselect.PrometheusAPIV1QueryRange(t, `foo_bar{}`, apptest.QueryOpts{
		Tenant:       "multitenant",
		Start:        "2022-05-10T07:59:00.000Z",
		End:          "2022-05-10T08:05:00.000Z",
		Step:         "1m",
		ExtraFilters: []string{`{vm_account_id="5",vm_project_id="15"}`, `{vm_account_id="5",vm_project_id="0"}`},
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	want = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":
	   {"result":[]}
	}`,
	)

	got = vmselect.PrometheusAPIV1QueryRange(t, `foo_bar{}`, apptest.QueryOpts{
		Tenant:       "multitenant",
		Start:        "2022-05-10T07:59:00.000Z",
		End:          "2022-05-10T08:05:00.000Z",
		Step:         "1m",
		ExtraFilters: []string{`{vm_account_id="99",vm_project_id="99"}`},
	})
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

}
