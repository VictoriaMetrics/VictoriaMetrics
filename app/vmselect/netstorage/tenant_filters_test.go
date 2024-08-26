package netstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestApplyFiltersToTenants(t *testing.T) {
	f := func(filters, tenants []string, expectedTenants []storage.TenantToken) {
		tenantsResult, err := applyFiltersToTenants(tenants, filters)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if !reflect.DeepEqual(tenantsResult, expectedTenants) {
			t.Fatalf("unexpected tenants result; got %v; want %v", tenantsResult, expectedTenants)
		}
	}

	f([]string{`{vm_account_id="1"}`}, []string{"1:1", "1:0"}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f([]string{`{vm_account_id="1",vm_project_id="0"}`}, []string{"1:1", "1:0"}, []storage.TenantToken{{AccountID: 1, ProjectID: 0}})

	f([]string{`{vm_account_id=~"1[0-9]+"}`}, []string{"1:1", "12323:0", "12323:3", "345:0"}, []storage.TenantToken{{AccountID: 12323, ProjectID: 0}, {AccountID: 12323, ProjectID: 3}})

	f([]string{`{vm_account_id="1",vm_project_id!="0"}`}, []string{"1:1", "1:0"}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}})
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
