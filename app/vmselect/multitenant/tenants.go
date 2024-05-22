package multitenant

import (
	"flag"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	tenantsCacheDuration = flag.Duration("search.tenantCacheExpireDuration", 5*time.Minute, "The expiry duration for list of tenants for multi-tenant queries.")
)

// FetchTenants returns the list of tenants available in the storage.
func FetchTenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	qt.Printf("fetching tenants on timeRange=%s", tr.String())

	cached := tenantsCacheV.get(tr)
	qt.Printf("fetched %d tenants from cache", len(cached))
	if len(cached) > 0 {
		return cached, nil
	}

	tenants, err := netstorage.Tenants(qt, tr, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	qt.Printf("fetched %d tenants from storage", len(tenants))

	tenantsCacheV.put(tr, tenants)
	qt.Printf("put %d tenants into cache", len(tenants))

	return tenants, nil
}

// Stop stops background processes required for multitenancy
func Stop() {
	tenantsCacheV.close()
}

var tenantsCacheV = func() *tenantsCache {
	tc := newTenantsCache(*tenantsCacheDuration)
	tc.runCleanup()

	return tc
}()

type tenantsCacheItem struct {
	tenants []string
	tr      storage.TimeRange
	expires time.Time
}

type tenantsCache struct {
	// mtr is used for exact matches lookup
	mtr map[string]*tenantsCacheItem
	// items is used for intersection matches lookup
	items []*tenantsCacheItem

	itemExpiration time.Duration

	requests atomic.Uint64
	misses   atomic.Uint64

	mu   sync.RWMutex
	stop chan struct{}
}

func newTenantsCache(expiration time.Duration) *tenantsCache {
	tc := &tenantsCache{
		mtr:            make(map[string]*tenantsCacheItem),
		items:          make([]*tenantsCacheItem, 0),
		itemExpiration: expiration,
	}

	metrics.GetOrCreateGauge(`vm_cache_requests_total{type="multitenancy/tenants"}`, func() float64 {
		return float64(tc.Requests())
	})
	metrics.GetOrCreateGauge(`vm_cache_misses_total{type="multitenancy/tenants"}`, func() float64 {
		return float64(tc.Misses())
	})
	metrics.GetOrCreateGauge(`vm_cache_entries{type="multitenancy/tenants"}`, func() float64 {
		return float64(tc.Len())
	})

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
	tc.mu.Lock()
	defer tc.mu.Unlock()

	expires := time.Now().Add(tc.itemExpiration)
	for i := len(tc.items) - 1; i >= 0; i-- {
		if tc.items[i].expires.Before(expires) {
			delete(tc.mtr, tc.items[i].tr.String())
			tc.items = append(tc.items[:i], tc.items[i+1:]...)
		}
	}
}

func (tc *tenantsCache) close() {
	close(tc.stop)
}

func (tc *tenantsCache) put(tr storage.TimeRange, tenants []string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	exp := time.Now().Add(timeutil.AddJitterToDuration(tc.itemExpiration))

	ci := &tenantsCacheItem{
		tenants: tenants,
		tr:      tr,
		expires: exp,
	}

	tc.items = append(tc.items, ci)
	tc.mtr[tr.String()] = ci

	sort.SliceStable(tc.items, func(i, j int) bool {
		return tc.items[i].tr.MinTimestamp >= tc.items[j].tr.MinTimestamp
	})
}
func (tc *tenantsCache) Requests() uint64 {
	return tc.requests.Load()
}

func (tc *tenantsCache) Misses() uint64 {
	return tc.misses.Load()
}

func (tc *tenantsCache) Len() uint64 {
	tc.mu.RLock()
	n := len(tc.items)
	tc.mu.RUnlock()
	return uint64(n)
}

func (tc *tenantsCache) get(tr storage.TimeRange) []string {
	tc.requests.Add(1)

	result := tc.getInternal(tr)
	if len(result) == 0 {
		// Try to widen the search window before giving up
		// It is common for instant queries to have tr of 1s for the latest data
		// so checking widening the cache window helps to improve cache hit rate
		result = tc.getInternal(storage.TimeRange{
			MinTimestamp: tr.MinTimestamp - tc.itemExpiration.Milliseconds(),
			MaxTimestamp: tr.MaxTimestamp,
		})

		if len(result) == 0 {
			tc.misses.Add(1)
		}
	}

	return result
}

func (tc *tenantsCache) getInternal(tr storage.TimeRange) []string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	if len(tc.items) == 0 {
		return nil
	}

	// Fast path - exact match of tr
	if ci, ok := tc.mtr[tr.String()]; ok {
		return ci.tenants
	}

	// Slow path - find matches which intersect with the requested tr
	idx := sort.Search(len(tc.items), func(i int) bool {
		return tr.MinTimestamp >= tc.items[i].tr.MinTimestamp
	})

	if idx == len(tc.items) {
		idx--
	}

	result := make(map[string]struct{})
	for {
		if idx < 0 {
			break
		}
		ci := tc.items[idx]
		if hasIntersection(tr, ci.tr) {
			for _, t := range ci.tenants {
				result[t] = struct{}{}
			}
		} else {
			break
		}
		idx--
	}

	tenants := make([]string, 0, len(result))
	for t := range result {
		tenants = append(tenants, t)
	}

	return tenants
}

// hasIntersection checks if there is any intersection of the given time ranges
func hasIntersection(a, b storage.TimeRange) bool {
	return a.MinTimestamp <= b.MaxTimestamp && a.MaxTimestamp >= b.MinTimestamp
}
