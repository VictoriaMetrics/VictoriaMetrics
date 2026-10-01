package tests

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// TestSingleTenantIDs verifies that vmsingle returns an empty list of tenants
// at /select/tenant_ids.
func TestSingleTenantIDs(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	sut.PrometheusAPIV1ImportPrometheus(t, []string{`foo_bar 1.00 1652169600000`}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	got := sut.SelectTenantIDs(t, apptest.QueryOpts{})
	want := []apptest.TenantID{}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}

// TestClusterTenantIDs verifies that vmselect returns the tenants available
// for the request at /select/tenant_ids.
func TestClusterTenantIDs(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	apptest.EnsureBlockingIngestion(t, vminsert, []*apptest.Vmstorage{vmstorage})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
	})

	f := func(opts apptest.QueryOpts, want []apptest.TenantID) {
		t.Helper()
		got := vmselect.SelectTenantIDs(t, opts)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	newHeaders := func(accountID, projectID string) http.Header {
		h := make(http.Header)
		if accountID != "" {
			h.Set("AccountID", accountID)
		}
		if projectID != "" {
			h.Set("ProjectID", projectID)
		}
		return h
	}

	// no tenants in the storage yet
	f(apptest.QueryOpts{}, []apptest.TenantID{})
	f(apptest.QueryOpts{Tenant: "multitenant"}, []apptest.TenantID{})

	// the request restricted to a single tenant returns this tenant even if it has no data
	f(apptest.QueryOpts{Tenant: "5:7"}, []apptest.TenantID{{AccountID: 5, ProjectID: 7}})

	samples := []string{
		`foo_bar 1.00 1652169600000`, // 2022-05-10T08:00:00Z
	}
	for _, tenant := range []string{"1:1", "1:15", "2", "0:3"} {
		vminsert.PrometheusAPIV1ImportPrometheus(t, samples, apptest.QueryOpts{Tenant: tenant})
	}
	vmstorage.ForceFlush(t)

	allTenants := []apptest.TenantID{
		{AccountID: 0, ProjectID: 3},
		{AccountID: 1, ProjectID: 1},
		{AccountID: 1, ProjectID: 15},
		{AccountID: 2, ProjectID: 0},
	}

	// the request without tenant information returns all the tenants
	f(apptest.QueryOpts{}, allTenants)

	// multitenant request returns all the tenants
	f(apptest.QueryOpts{Tenant: "multitenant"}, allTenants)
	f(apptest.QueryOpts{Headers: newHeaders("multitenant", "")}, allTenants)

	// tenant in the request path
	f(apptest.QueryOpts{Tenant: "1:15"}, []apptest.TenantID{{AccountID: 1, ProjectID: 15}})
	f(apptest.QueryOpts{Tenant: "2"}, []apptest.TenantID{{AccountID: 2, ProjectID: 0}})

	// tenant in the request headers
	f(apptest.QueryOpts{Headers: newHeaders("1", "15")}, []apptest.TenantID{{AccountID: 1, ProjectID: 15}})
	f(apptest.QueryOpts{Headers: newHeaders("2", "")}, []apptest.TenantID{{AccountID: 2, ProjectID: 0}})
	f(apptest.QueryOpts{Headers: newHeaders("", "3")}, []apptest.TenantID{{AccountID: 0, ProjectID: 3}})

	// tenant in the request path has priority over the tenant in the request headers
	f(apptest.QueryOpts{Tenant: "1:1", Headers: newHeaders("2", "")}, []apptest.TenantID{{AccountID: 1, ProjectID: 1}})

	// multitenant request with tenant filters returns only the matching tenants
	f(apptest.QueryOpts{
		Tenant:       "multitenant",
		ExtraFilters: []string{`{vm_account_id="1"}`},
	}, []apptest.TenantID{
		{AccountID: 1, ProjectID: 1},
		{AccountID: 1, ProjectID: 15},
	})
	f(apptest.QueryOpts{
		ExtraFilters: []string{`{vm_account_id=~"0|2"}`},
	}, []apptest.TenantID{
		{AccountID: 0, ProjectID: 3},
		{AccountID: 2, ProjectID: 0},
	})
	f(apptest.QueryOpts{
		Tenant:      "multitenant",
		ExtraLabels: []string{`vm_project_id=15`},
	}, []apptest.TenantID{
		{AccountID: 1, ProjectID: 15},
	})

	// the time range without data
	f(apptest.QueryOpts{
		Start: "2023-01-01T00:00:00Z",
		End:   "2023-01-02T00:00:00Z",
	}, []apptest.TenantID{})
	// the time range with data
	f(apptest.QueryOpts{
		Start: "2022-05-10T00:00:00Z",
		End:   "2022-05-11T00:00:00Z",
	}, allTenants)

	// tenant headers are ignored if multitenancy via headers is disabled
	vmselect = tc.MustStartVmselect("vmselect-no-headers", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
		"-search.tenantCacheExpireDuration=0",
		"-enableMultitenancyViaHeaders=false",
	})
	f(apptest.QueryOpts{}, allTenants)
	f(apptest.QueryOpts{Headers: newHeaders("1", "15")}, allTenants)
	f(apptest.QueryOpts{Tenant: "1:15"}, []apptest.TenantID{{AccountID: 1, ProjectID: 15}})
	f(apptest.QueryOpts{Tenant: "multitenant"}, allTenants)
}
