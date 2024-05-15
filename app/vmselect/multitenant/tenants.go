package multitenant

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// FetchTenants returns the list of tenants available in the storage.
func FetchTenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	qt.Printf("fetching tenants on timeRange=%s", tr.String())

	cached := tc.get(tr)
	qt.Printf("fetched %d tenants from cache", len(cached))
	if len(cached) > 0 {
		return cached, nil
	}

	tenants, err := netstorage.Tenants(qt, tr, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	qt.Printf("fetched %d tenants from storage", len(tenants))

	tc.put(tr, tenants)

	return tenants, nil
}

var tc = func() *tenantsCache {
	tc := newTenantsCache(time.Minute)
	tc.runCleanup()

	return tc
}()

type tenantsCacheItem struct {
	tenants []string
	tr      storage.TimeRange
	expires time.Time
}

type tenantsCache struct {
	items          []*tenantsCacheItem
	mtr            map[storage.TimeRange]*tenantsCacheItem
	itemExpiration time.Duration

	m    sync.RWMutex
	stop chan struct{}
}

func newTenantsCache(expiration time.Duration) *tenantsCache {
	tc := &tenantsCache{
		mtr:            make(map[storage.TimeRange]*tenantsCacheItem),
		items:          make([]*tenantsCacheItem, 0),
		itemExpiration: expiration,
	}

	return tc
}

func (tc *tenantsCache) runCleanup() {
	tc.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(tc.itemExpiration)
		defer ticker.Stop()

		for {
			select {
			case <-tc.stop:
				return
			case <-ticker.C:
				tc.cleanup()
			}
		}
	}()
}

func (tc *tenantsCache) cleanup() {
	tc.m.Lock()
	defer tc.m.Unlock()

	expires := time.Now().Add(tc.itemExpiration)
	for i := len(tc.items) - 1; i >= 0; i-- {
		if tc.items[i].expires.Before(expires) {
			delete(tc.mtr, tc.items[i].tr)
			tc.items = append(tc.items[:i], tc.items[i+1:]...)
		}
	}
}

func (tc *tenantsCache) close() {
	close(tc.stop)
}

func (tc *tenantsCache) put(tr storage.TimeRange, tenants []string) {
	tc.m.Lock()
	defer tc.m.Unlock()

	ci := &tenantsCacheItem{
		tenants: tenants,
		tr:      tr,
		expires: time.Now().Add(tc.itemExpiration),
	}

	tc.items = append(tc.items, ci)
	tc.mtr[tr] = ci

	sort.SliceStable(tc.items, func(i, j int) bool {
		return tc.items[i].tr.MinTimestamp >= tc.items[j].tr.MinTimestamp
	})
}

func (tc *tenantsCache) get(tr storage.TimeRange) []string {
	tc.m.RLock()
	defer tc.m.RUnlock()
	if len(tc.items) == 0 {
		return nil
	}

	if tr.MinTimestamp >= tc.items[0].tr.MaxTimestamp {
		return nil
	}

	if ci, ok := tc.mtr[tr]; ok {
		return ci.tenants
	}

	idx := sort.Search(len(tc.items), func(i int) bool {
		return tr.MinTimestamp >= tc.items[i].tr.MinTimestamp
	})

	if idx == len(tc.items) {
		return nil
	}

	result := make([]string, 0)
	for {
		if idx < 0 {
			break
		}
		ci := tc.items[idx]
		if hasIntersection(tr, ci.tr) {
			result = append(result, ci.tenants...)
		} else {
			break
		}
		idx--
	}
	return result
}

func hasIntersection(a, b storage.TimeRange) bool {
	return a.MinTimestamp <= b.MaxTimestamp && a.MaxTimestamp >= b.MinTimestamp
}
