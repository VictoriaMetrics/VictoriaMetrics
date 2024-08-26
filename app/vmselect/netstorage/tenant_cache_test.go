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

	tc.put(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 0}, []string{"1:1", "1:0"})
	tc.put(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: dayMs - 1}, []string{"1:1", "1:0"})
	tc.put(storage.TimeRange{MinTimestamp: dayMs, MaxTimestamp: 2*dayMs - 1}, []string{"2:1", "2:0"})
	tc.put(storage.TimeRange{MinTimestamp: 2 * dayMs, MaxTimestamp: 3*dayMs - 1}, []string{"3:1", "3:0"})

	f := func(tr storage.TimeRange, expectedTenants []string) {
		t.Helper()
		tenants := tc.get(tr)

		if len(tenants) == 0 && len(tenants) == len(expectedTenants) {
			return
		}
		sort.Strings(tenants)
		sort.Strings(expectedTenants)

		if !reflect.DeepEqual(tenants, expectedTenants) {
			t.Fatalf("unexpected tenants; got %v; want %v", tenants, expectedTenants)
		}
	}

	// Basic time range coverage
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 0}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: dayMs, MaxTimestamp: dayMs}, []string{"2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: 2 * dayMs, MaxTimestamp: 2 * dayMs}, []string{"3:1", "3:0"})
	f(storage.TimeRange{MinTimestamp: 3 * dayMs, MaxTimestamp: 3*dayMs + 1}, []string{})

	// Time range inside existing range
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs/2 + 100}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: dayMs + dayMs/2, MaxTimestamp: dayMs + dayMs/2 + 100}, []string{"2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: dayMs / 2}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs - 1}, []string{"1:1", "1:0"})

	// Overlapping time ranges
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 2*dayMs - 1}, []string{"1:1", "1:0", "2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: dayMs / 2, MaxTimestamp: dayMs + dayMs/2}, []string{"1:1", "1:0", "2:1", "2:0"})
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
