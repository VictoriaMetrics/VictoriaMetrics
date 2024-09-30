package netstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// GetTenantTokensFromFilters returns the list of tenant tokens and the list of filters without tenant filters.
func GetTenantTokensFromFilters(qt *querytracer.Tracer, tr storage.TimeRange, tfs [][]storage.TagFilter, deadline searchutils.Deadline) ([]storage.TenantToken, [][]storage.TagFilter, error) {
	tenants, err := TenantsCached(qt, tr, deadline)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	tenantFilters := make([]string, 0, len(tfs))
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
			tenantFilters = append(tenantFilters, tagFiltersToString(ffs))
		}
		if len(offs) > 0 {
			otherFilters = append(otherFilters, offs)
		}
	}

	tts, err := applyFiltersToTenants(tenants, tenantFilters)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot apply filters to tenants: %w", err)
	}

	return tts, otherFilters, nil
}

func tagFiltersToString(tfs []storage.TagFilter) string {
	a := make([]string, len(tfs))
	for i, tf := range tfs {
		a[i] = tf.String()
	}
	return "{" + strings.Join(a, ",") + "}"
}

// applyFiltersToTenants applies the given filters to the given tenants.
// It returns the filtered tenants.
func applyFiltersToTenants(tenants, filters []string) ([]storage.TenantToken, error) {
	tokens := make([]storage.TenantToken, len(tenants))
	for i, tenant := range tenants {
		accountID, projectID, err := auth.ParseToken(tenant)
		if err != nil {
			return nil, fmt.Errorf("cannot construct auth token from tenant %q: %w", tenant, err)
		}
		tokens[i].AccountID = accountID
		tokens[i].ProjectID = projectID
	}
	if len(filters) == 0 {
		return tokens, nil
	}

	resultingTokens := make([]storage.TenantToken, 0, len(tenants))
	lbs := make([][]prompbmarshal.Label, 0, len(filters))
	lbsAux := make([]prompbmarshal.Label, 0, len(filters))
	for _, token := range tokens {
		lbsAuxLen := len(lbsAux)
		lbsAux = append(lbsAux, prompbmarshal.Label{
			Name:  "vm_account_id",
			Value: fmt.Sprintf("%d", token.AccountID),
		}, prompbmarshal.Label{
			Name:  "vm_project_id",
			Value: fmt.Sprintf("%d", token.ProjectID),
		})

		lbs = append(lbs, lbsAux[lbsAuxLen:])
	}

	promIfs := make([]promrelabel.IfExpression, len(filters))
	for i, filter := range filters {
		err := promIfs[i].Parse(filter)
		if err != nil {
			return nil, fmt.Errorf("cannot parse if expression from filters %v: %s", filter, err)
		}
	}

	for i, lb := range lbs {
		for _, promIf := range promIfs {
			if promIf.Match(lb) {
				resultingTokens = append(resultingTokens, tokens[i])
				break
			}
		}
	}

	return resultingTokens, nil
}

// isTenancyLabel returns true if the given label name is used for tenancy.
func isTenancyLabel(name string) bool {
	return name == "vm_account_id" || name == "vm_project_id"
}
