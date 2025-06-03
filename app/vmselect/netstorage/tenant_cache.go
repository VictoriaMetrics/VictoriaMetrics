package netstorage

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	tenantsCacheDuration = flag.Duration("search.tenantCacheExpireDuration", 5*time.Minute, "Expiry duration for caching tenants in memory. A zero value disables caching, causing tenants to be fetched from storage nodes on every query.")
)

// TenantsCached returns the list of tenants available in the storage.
func TenantsCached(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutil.Deadline, mayCache bool) ([]storage.TenantToken, error) {
	qtL := qt.NewChild("fetching tenants on timeRange=%s", tr.String())
	defer qtL.Done()

	initTenantsCacheVOnce.Do(func() {
		tenantsCacheV = newTenantsCache(*tenantsCacheDuration)
	})

	useCache := mayCache && tenantsCacheDuration.Seconds() > 0

	if useCache {
		cached := tenantsCacheV.get(tr)
		qtL.Printf("fetched %d tenants from cache", len(cached))
		if len(cached) > 0 {
			return cached, nil
		}
	} else {
		qtL.Printf("do not fetch list of tenants from cache")
	}

	tenants, err := Tenants(qtL, tr, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	qtL.Printf("fetched %d tenants from storage", len(tenants))

	tt := make([]storage.TenantToken, len(tenants))
	for i, t := range tenants {
		accountID, projectID, err := auth.ParseToken(t)
		if err != nil {
			return nil, fmt.Errorf("cannot parse tenant token %q: %w", t, err)
		}
		tt[i].AccountID = accountID
		tt[i].ProjectID = projectID
	}

	if useCache {
		tenantsCacheV.put(tr, tt)
		qtL.Printf("put %d tenants into cache", len(tenants))
	} else {
		qtL.Printf("do not put list of tenants into cache")
	}

	return tt, nil
}

var (
	initTenantsCacheVOnce sync.Once
	tenantsCacheV         *tenantsCache
)

type tenantsCacheItem struct {
	tenants []storage.TenantToken
	tr      storage.TimeRange
	expires time.Time
}

type tenantsCache struct {
	// items is used for intersection matches lookup
	items []*tenantsCacheItem

	itemExpiration time.Duration

	requests atomic.Uint64
	misses   atomic.Uint64

	mu sync.Mutex
}

func newTenantsCache(expiration time.Duration) *tenantsCache {
	tc := &tenantsCache{
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

func (tc *tenantsCache) cleanupLocked() {
	expires := time.Now().Add(tc.itemExpiration)
	itemsTmp := tc.items[:0]
	for _, item := range tc.items {
		if item.expires.After(expires) {
			itemsTmp = append(itemsTmp, item)
		}
	}

	tc.items = itemsTmp
}

func (tc *tenantsCache) put(tr storage.TimeRange, tenants []storage.TenantToken) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	alignTrToDay(&tr)

	exp := time.Now().Add(timeutil.AddJitterToDuration(tc.itemExpiration))

	ci := &tenantsCacheItem{
		tenants: tenants,
		tr:      tr,
		expires: exp,
	}

	tc.items = append(tc.items, ci)
}
func (tc *tenantsCache) Requests() uint64 {
	return tc.requests.Load()
}

func (tc *tenantsCache) Misses() uint64 {
	return tc.misses.Load()
}

func (tc *tenantsCache) Len() uint64 {
	tc.mu.Lock()
	n := len(tc.items)
	tc.mu.Unlock()
	return uint64(n)
}

func (tc *tenantsCache) get(tr storage.TimeRange) []storage.TenantToken {
	tc.requests.Add(1)

	alignTrToDay(&tr)
	return tc.getInternal(tr)
}

func (tc *tenantsCache) getInternal(tr storage.TimeRange) []storage.TenantToken {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if len(tc.items) == 0 {
		tc.misses.Add(1)
		return nil
	}
	ct := time.Now()

	result := make(map[storage.TenantToken]struct{})
	cleanupNeeded := false
	for idx := range tc.items {
		ci := tc.items[idx]
		if ci.expires.Before(ct) {
			cleanupNeeded = true
			continue
		}

		if hasIntersection(tr, ci.tr) {
			for _, t := range ci.tenants {
				result[t] = struct{}{}
			}
		}
	}

	if cleanupNeeded {
		tc.cleanupLocked()
	}

	tenants := make([]storage.TenantToken, 0, len(result))
	for t := range result {
		tenants = append(tenants, t)
	}
	if len(tenants) == 0 {
		tc.misses.Add(1)
	}

	return tenants
}

// alignTrToDay aligns the given time range to the day boundaries
// tr.minTimestamp will be set to the start of the day
// tr.maxTimestamp will be set to the end of the day
func alignTrToDay(tr *storage.TimeRange) {
	tr.MinTimestamp = timeutil.StartOfDay(tr.MinTimestamp)
	tr.MaxTimestamp = timeutil.EndOfDay(tr.MaxTimestamp)
}

// hasIntersection checks if there is any intersection of the given time ranges
func hasIntersection(a, b storage.TimeRange) bool {
	return a.MinTimestamp <= b.MaxTimestamp && a.MaxTimestamp >= b.MinTimestamp
}
