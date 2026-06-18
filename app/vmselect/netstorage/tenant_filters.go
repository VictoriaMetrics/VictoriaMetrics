package netstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// GetTenantTokensFromFilters returns the list of tenant tokens and the list of filters without tenant filters.
func GetTenantTokensFromFilters(qt *querytracer.Tracer, tr storage.TimeRange, tfs [][]storage.TagFilter, deadline searchutil.Deadline, mayCache bool) ([]storage.TenantToken, [][]storage.TagFilter, error) {
	tenants, err := TenantsCached(qt, tr, deadline, mayCache)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	tenantFilters, otherFilters := splitFiltersByType(tfs)

	tts, err := applyFiltersToTenants(tenants, tenantFilters)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot apply filters to tenants: %w", err)
	}

	return tts, otherFilters, nil
}

func splitFiltersByType(tfs [][]storage.TagFilter) ([][]storage.TagFilter, [][]storage.TagFilter) {
	if len(tfs) == 0 {
		return nil, tfs
	}

	tenantFilters := make([][]storage.TagFilter, 0, len(tfs))
	otherFilters := make([][]storage.TagFilter, 0, len(tfs))
	for _, f := range tfs {
		ffs := make([]storage.TagFilter, 0, len(f))
		offs := make([]storage.TagFilter, 0, len(f))
		for _, tf := range f {
			if !isTenancyLabel(string(tf.Key)) {
				offs = append(offs, tf)
				continue
			}
			ffs = append(ffs, tf)
		}

		if len(ffs) > 0 {
			tenantFilters = append(tenantFilters, ffs)
		}
		if len(offs) > 0 {
			otherFilters = append(otherFilters, offs)
		}
	}
	return tenantFilters, otherFilters
}

// ApplyTenantFiltersToTagFilters applies the given tenant filters to the given tag filters.
func ApplyTenantFiltersToTagFilters(tts []storage.TenantToken, tfs [][]storage.TagFilter) ([]storage.TenantToken, [][]storage.TagFilter) {
	tenantFilters, otherFilters := splitFiltersByType(tfs)
	if len(tenantFilters) == 0 {
		return tts, otherFilters
	}

	tts, err := applyFiltersToTenants(tts, tenantFilters)
	if err != nil {
		return nil, nil
	}
	return tts, otherFilters
}

// applyFiltersToTenants applies the given filters to the given tenants.
// It returns the filtered tenants.
func applyFiltersToTenants(tenants []storage.TenantToken, filters [][]storage.TagFilter) ([]storage.TenantToken, error) {
	// fast path - return all tenants if no filters are given
	if len(filters) == 0 {
		return tenants, nil
	}

	resultingTokens := make([]storage.TenantToken, 0, len(tenants))
	lbs := make([][]prompb.Label, 0, len(filters))
	lbsAux := make([]prompb.Label, 0, len(filters))
	for _, token := range tenants {
		lbsAuxLen := len(lbsAux)
		lbsAux = append(lbsAux, prompb.Label{
			Name:  "vm_account_id",
			Value: fmt.Sprintf("%d", token.AccountID),
		}, prompb.Label{
			Name:  "vm_project_id",
			Value: fmt.Sprintf("%d", token.ProjectID),
		})

		lbs = append(lbs, lbsAux[lbsAuxLen:])
	}

	me := &metricsql.MetricExpr{
		LabelFilterss: toLabelFilterss(filters),
	}
	var promIf promrelabel.IfExpression
	if err := promIf.ParseFromMetricExpr(me); err != nil {
		return nil, fmt.Errorf("cannot parse if expression from filters %v: %w", filters, err)
	}
	for i, lb := range lbs {
		if promIf.Match(lb) {
			resultingTokens = append(resultingTokens, tenants[i])
		}
	}

	return resultingTokens, nil
}

// isTenancyLabel returns true if the given label name is used for tenancy.
func isTenancyLabel(name string) bool {
	return name == "vm_account_id" || name == "vm_project_id"
}

func toLabelFilterss(tfss [][]storage.TagFilter) [][]metricsql.LabelFilter {
	lfss := make([][]metricsql.LabelFilter, len(tfss))
	for i, tfs := range tfss {
		lfs := make([]metricsql.LabelFilter, len(tfs))
		for j := range tfs {
			toLabelFilter(&lfs[j], &tfs[j])
		}
		lfss[i] = lfs
	}
	return lfss
}

func toLabelFilter(dst *metricsql.LabelFilter, src *storage.TagFilter) {
	if src.Key == nil {
		dst.Label = "__name__"
	} else {
		dst.Label = string(src.Key)
	}
	dst.Value = string(src.Value)
	dst.IsRegexp = src.IsRegexp
	dst.IsNegative = src.IsNegative
}
