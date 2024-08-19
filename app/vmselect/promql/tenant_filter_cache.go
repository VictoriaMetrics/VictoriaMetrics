package promql

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/multitenant"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type tenantFiltersCacheItem struct {
	ne      metricsql.Expr
	filters [][]metricsql.LabelFilter
}

type tenantFiltersCache struct {
	requests atomic.Uint64
	misses   atomic.Uint64

	m  map[string]*tenantFiltersCacheItem
	mu sync.RWMutex
}

func (tfc *tenantFiltersCache) put(original, converted metricsql.Expr, filters [][]metricsql.LabelFilter) {
	tfc.mu.Lock()
	defer tfc.mu.Unlock()

	overflow := len(tfc.m) - parseCacheMaxLen
	if overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(tfc.m)) * 0.1)
		for k := range tfc.m {
			delete(tfc.m, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}

	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = original.AppendString(bb.B)
	tfc.m[string(bb.B)] = &tenantFiltersCacheItem{
		ne:      converted,
		filters: filters,
	}
}

func (tfc *tenantFiltersCache) get(e metricsql.Expr) *tenantFiltersCacheItem {
	tfc.mu.RLock()
	defer tfc.mu.RUnlock()
	tfc.requests.Add(1)

	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = e.AppendString(bb.B)
	result := tfc.m[string(bb.B)]

	if result == nil {
		tfc.misses.Add(1)
	}

	return result
}
func (tfc *tenantFiltersCache) Requests() uint64 {
	return tfc.requests.Load()
}

func (tfc *tenantFiltersCache) Misses() uint64 {
	return tfc.misses.Load()
}

func (tfc *tenantFiltersCache) Len() uint64 {
	tfc.mu.RLock()
	n := len(tfc.m)
	tfc.mu.RUnlock()
	return uint64(n)
}

var tentantsFilterCacheV = func() *tenantFiltersCache {
	m := make(map[string]*tenantFiltersCacheItem)

	tfc := &tenantFiltersCache{
		m: m,
	}

	metrics.GetOrCreateGauge(`vm_cache_requests_total{type="multitenancy/filters"}`, func() float64 {
		return float64(tfc.Requests())
	})
	metrics.GetOrCreateGauge(`vm_cache_misses_total{type="multitenancy/filters"}`, func() float64 {
		return float64(tfc.Misses())
	})
	metrics.GetOrCreateGauge(`vm_cache_entries{type="multitenancy/filters"}`, func() float64 {
		return float64(tfc.Len())
	})

	return tfc
}()

func extractTenantFilters(e metricsql.Expr, dst [][]metricsql.LabelFilter) ([][]metricsql.LabelFilter, metricsql.Expr) {
	if cache := tentantsFilterCacheV.get(e); cache != nil {
		return cache.filters, cache.ne
	}

	ne := metricsql.Clone(e)

	switch exp := ne.(type) {
	case *metricsql.MetricExpr:
		for idx, labels := range exp.LabelFilterss {
			if len(labels) == 0 {
				continue
			}

			newLabels := make([]metricsql.LabelFilter, 0, len(labels))
			newFilters := make([]metricsql.LabelFilter, 0)
			for _, label := range labels {
				if multitenant.IsTenancyLabel(label.Label) {
					newFilters = append(newFilters, label)
				} else {
					newLabels = append(newLabels, label)
				}
			}
			if len(newFilters) == 0 {
				continue
			}

			exp.LabelFilterss[idx] = newLabels
			dst = append(dst, newFilters)
		}
		tentantsFilterCacheV.put(e, ne, dst)
		return dst, exp

	case *metricsql.RollupExpr:
		dst, exp.Expr = extractTenantFilters(exp.Expr, dst)
		tentantsFilterCacheV.put(e, ne, dst)
		return dst, exp

	case *metricsql.BinaryOpExpr:
		var newL, newR metricsql.Expr

		dst, newL = extractTenantFilters(exp.Left, dst)
		dst, newR = extractTenantFilters(exp.Right, dst)

		exp.Left = newL
		exp.Right = newR
		tentantsFilterCacheV.put(e, ne, dst)
		return dst, exp

	case *metricsql.AggrFuncExpr:
		var newArg metricsql.Expr
		for i, arg := range exp.Args {
			dst, newArg = extractTenantFilters(arg, dst)
			exp.Args[i] = newArg
		}
		tentantsFilterCacheV.put(e, ne, dst)
		return dst, exp

	case *metricsql.FuncExpr:
		var newArg metricsql.Expr
		for i, arg := range exp.Args {
			dst, newArg = extractTenantFilters(arg, dst)
			exp.Args[i] = newArg
		}
		tentantsFilterCacheV.put(e, ne, dst)
		return dst, exp

	}

	tentantsFilterCacheV.put(e, ne, dst)
	return dst, ne
}

func extractTenantFiltersEc(ec *EvalConfig, dst [][]metricsql.LabelFilter) [][]metricsql.LabelFilter {
	if len(ec.EnforcedTagFilterss) == 0 {
		return dst
	}

	updTagFilters := make([][]storage.TagFilter, 0, len(ec.EnforcedTagFilterss))

	for _, filters := range ec.EnforcedTagFilterss {
		newFilters := make([]metricsql.LabelFilter, 0)
		newTagFilters := make([]storage.TagFilter, 0)
		for _, filter := range filters {
			if multitenant.IsTenancyLabel(string(filter.Key)) {
				newFilters = append(newFilters, metricsql.LabelFilter{
					Label:      string(filter.Key),
					Value:      string(filter.Value),
					IsRegexp:   filter.IsRegexp,
					IsNegative: filter.IsNegative,
				})
			} else {
				newTagFilters = append(newTagFilters, filter)
			}
		}
		if len(newFilters) > 0 {
			dst = append(dst, newFilters)
		}
		if len(newTagFilters) > 0 {
			updTagFilters = append(updTagFilters, newTagFilters)
		}
	}
	ec.EnforcedTagFilterss = updTagFilters

	return dst
}

func applyFiltersToTenants(tenants []string, filters [][]metricsql.LabelFilter) []*auth.Token {
	filtersStr := make([]string, 0, len(filters))
	for _, filter := range filters {
		fs := make([]byte, 0)
		fs = append(fs, '{')

		for idx, f := range filter {
			fs = f.AppendString(fs)
			if idx != len(filter)-1 {
				fs = append(fs, ',')
			}
		}
		fs = append(fs, '}')
		filtersStr = append(filtersStr, string(fs))
	}

	fts, err := multitenant.ApplyFiltersToTenants(tenants, filtersStr)
	if err != nil {
		logger.Panicf("unexpected error when applying filters to tenants: %s", err)
	}
	return fts
}
