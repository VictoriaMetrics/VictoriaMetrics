package netstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestApplyFiltersToTenants(t *testing.T) {
	f := func(filters [][]storage.TagFilter, tenants []storage.TenantToken, expectedTenants []storage.TenantToken) {
		tenantsResult, err := applyFiltersToTenants(tenants, filters)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if !reflect.DeepEqual(tenantsResult, expectedTenants) {
			t.Fatalf("unexpected tenants result; got %v; want %v", tenantsResult, expectedTenants)
		}
	}

	f(nil, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})

	f([][]storage.TagFilter{{{Key: []byte("vm_account_id"), Value: []byte("1"), IsNegative: false, IsRegexp: false}}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f([][]storage.TagFilter{{{Key: []byte("vm_account_id"), Value: []byte("1"), IsNegative: false, IsRegexp: false}, {Key: []byte("vm_project_id"), Value: []byte("0"), IsNegative: false, IsRegexp: false}}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}}, []storage.TenantToken{{AccountID: 1, ProjectID: 0}})

	f([][]storage.TagFilter{{{Key: []byte("vm_account_id"), Value: []byte("1[0-9]+"), IsNegative: false, IsRegexp: true}}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 12323, ProjectID: 0}, {AccountID: 12323, ProjectID: 3}, {AccountID: 345, ProjectID: 0}}, []storage.TenantToken{{AccountID: 12323, ProjectID: 0}, {AccountID: 12323, ProjectID: 3}})

	f([][]storage.TagFilter{{{Key: []byte("vm_account_id"), Value: []byte("1"), IsNegative: false, IsRegexp: false}, {Key: []byte("vm_project_id"), Value: []byte("0"), IsNegative: true, IsRegexp: false}}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}})
}

func TestIsTenancyLabel(t *testing.T) {
	f := func(label string, expected bool) {
		t.Helper()
		isTenancyLabel := isTenancyLabel(label)
		if isTenancyLabel != expected {
			t.Fatalf("unexpected result for label %q; got %v; want %v", label, isTenancyLabel, expected)
		}
	}

	f("vm_account_id", true)
	f("vm_project_id", true)

	// Test that the label is case-insensitive
	f("VM_account_id", false)
	f("VM_project_id", false)

	// non-tenancy labels
	f("job", false)
	f("instance", false)

}
