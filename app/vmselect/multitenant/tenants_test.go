package multitenant

import (
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestFetchingTenants(t *testing.T) {
	tc := newTenantsCache(time.Minute)

	tc.put(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, []string{"1:1", "1:0"})
	tc.put(storage.TimeRange{MinTimestamp: 100, MaxTimestamp: 200}, []string{"2:1", "2:0"})
	tc.put(storage.TimeRange{MinTimestamp: 200, MaxTimestamp: 300}, []string{"3:1", "3:0"})

	f := func(tr storage.TimeRange, expectedTenants []string) {
		t.Helper()
		tenants := tc.get(tr)
		if !reflect.DeepEqual(tenants, expectedTenants) {
			t.Fatalf("unexpected tenants; got %v; want %v", tenants, expectedTenants)
		}
	}

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: 100, MaxTimestamp: 200}, []string{"2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: 200, MaxTimestamp: 300}, []string{"3:1", "3:0"})
	f(storage.TimeRange{MinTimestamp: 300, MaxTimestamp: 400}, []string{})

	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 80}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: 150, MaxTimestamp: 180}, []string{"2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 80}, []string{"1:1", "1:0"})
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 99}, []string{"1:1", "1:0"})

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 199}, []string{"1:1", "1:0", "2:1", "2:0"})
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 150}, []string{"1:1", "1:0", "2:1", "2:0"})
}

func Test_hasIntersection(t *testing.T) {
	f := func(inner, outer storage.TimeRange, expected bool) {
		t.Helper()
		if hasIntersection(inner, outer) != expected {
			t.Fatalf("unexpected result for inner=%+v, outer=%+v", inner, outer)
		}
	}

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, true)
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 100}, true)
	f(storage.TimeRange{MinTimestamp: 50, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 10, MaxTimestamp: 80}, true)

	f(storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 50}, storage.TimeRange{MinTimestamp: 60, MaxTimestamp: 100}, false)
	f(storage.TimeRange{MinTimestamp: 100, MaxTimestamp: 150}, storage.TimeRange{MinTimestamp: 60, MaxTimestamp: 80}, false)
}
