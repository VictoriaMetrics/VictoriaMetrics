package multitenant

import (
	"reflect"
	"slices"
	"testing"
)

func TestApplyFiltersToTenants(t *testing.T) {
	f := func(filters, tenants, expectedTenants []string) {
		tenantsResult, err := ApplyFiltersToTenants(tenants, filters)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		resultStrings := make([]string, 0, len(tenantsResult))
		for _, t := range tenantsResult {
			resultStrings = append(resultStrings, t.String())
		}
		slices.Sort(resultStrings)
		slices.Sort(expectedTenants)

		if !reflect.DeepEqual(resultStrings, expectedTenants) {
			t.Fatalf("unexpected tenants result; got %v; want %v", tenantsResult, expectedTenants)
		}
	}

	f([]string{`{vm_account_id="1"}`}, []string{"1:1", "1:0"}, []string{"1:1", "1"})
	//f([]string{`{vm_account_id="1",vm_project_id="0"}`}, []string{"1:1", "1:0"}, []string{"1:0"}) // todo: undef behaviour

}

func TestIsTenancyLabel(t *testing.T) {
	f := func(label string, expected bool) {
		t.Helper()
		isTenancyLabel := IsTenancyLabel(label)
		if isTenancyLabel != expected {
			t.Fatalf("unexpected result for label %q; got %v; want %v", label, isTenancyLabel, expected)
		}
	}

	f("vm_account_id", true)
	f("vm_project_id", true)
	f("job", false)
	f("instance", false)
}
