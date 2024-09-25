package netstorage

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestFetchingTenants(t *testing.T) {
	tc := newTenantsCache(5 * time.Second)

	dayMs := (time.Hour * 24 * 1000).Milliseconds()

	tc.put(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 0}, []storage.TenantToken{
		{AccountID: 1, ProjectID: 1},
		{AccountID: 1, ProjectID: 0},
	})
	tc.put(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: dayMs - 1}, []storage.TenantToken{
		{AccountID: 1, ProjectID: 1},
		{AccountID: 1, ProjectID: 0},
	})
	tc.put(storage.TimeRange{MinTimestamp: dayMs, MaxTimestamp: 2*dayMs - 1}, []storage.TenantToken{
		{AccountID: 2, ProjectID: 1},
		{AccountID: 2, ProjectID: 0},
	})
	tc.put(storage.TimeRange{MinTimestamp: 2 * dayMs, MaxTimestamp: 3*dayMs - 1}, []storage.TenantToken{
		{AccountID: 3, ProjectID: 1},
		{AccountID: 3, ProjectID: 0},
	})

	f := func(tr storage.TimeRange, expectedTenants []storage.TenantToken) {
		t.Helper()
		tenants := tc.get(tr)

		if len(tenants) == 0 && len(tenants) == len(expectedTenants) {
			return
		}

		sortTenants := func(t []storage.TenantToken) func(i, j int) bool {
			return func(i, j int) bool {
				if t[i].AccountID == t[j].AccountID {
					return t[i].ProjectID < t[j].ProjectID
				}
				return t[i].AccountID < t[j].AccountID
			}
		}
		sort.Slice(tenants, sortTenants(tenants))
		sort.Slice(expectedTenants, sortTenants(expectedTenants))

		if !reflect.DeepEqual(tenants, expectedTenants) {
			t.Fatalf("unexpected tenants; got %v; want %v", tenants, expectedTenants)
		}
	}

	// Basic time range coverage
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 0}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: dayMs, MaxTimestamp: dayMs}, []storage.TenantToken{{AccountID: 2, ProjectID: 1}, {AccountID: 2, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: 2 * dayMs, MaxTimestamp: 2 * dayMs}, []storage.TenantToken{{AccountID: 3, ProjectID: 1}, {AccountID: 3, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: 3 * dayMs, MaxTimestamp: 3*dayMs + 1}, []storage.TenantToken{})

	// Time range inside existing range
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs/2 + 100}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: dayMs + dayMs/2, MaxTimestamp: dayMs + dayMs/2 + 100}, []storage.TenantToken{{AccountID: 2, ProjectID: 1}, {AccountID: 2, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: dayMs / 2}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs - 1}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}})

	// Overlapping time ranges
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 2*dayMs - 1}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}, {AccountID: 2, ProjectID: 1}, {AccountID: 2, ProjectID: 0}})
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs + dayMs/2}, []storage.TenantToken{{AccountID: 1, ProjectID: 1}, {AccountID: 1, ProjectID: 0}, {AccountID: 2, ProjectID: 1}, {AccountID: 2, ProjectID: 0}})
}

func TestHasIntersection(t *testing.T) {
	f := func(inner, outer storage.TimeRange, expected bool) {
		t.Helper()
		if hasIntersection(inner, outer) != expected {
			t.Fatalf("unexpected result for inner=%+v, outer=%+v", inner, outer)
		}
	}

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 0}, true)
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, true)
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, true)
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 10, MaxTimestamp: 80}, true)

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 50}, storage.TimeRange{MinTimestamp: 60, MaxTimestamp: 100}, false)
	f(storage.TimeRange{MinTimestamp: 100, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 60, MaxTimestamp: 80}, false)
}
