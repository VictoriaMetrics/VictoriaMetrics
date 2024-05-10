package multitenant

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// ApplyFiltersToTenants applies the given filters to the given tenants.
// It returns the filtered tenants.
func ApplyFiltersToTenants(tenants, filters []string) ([]*auth.Token, error) {
	tokens := make([]*auth.Token, 0, len(tenants))
	for _, tenant := range tenants {
		t, err := auth.NewToken(tenant)
		if err != nil {
			return nil, fmt.Errorf("cannot construct auth token from tenant %q: %w", tenant, err)
		}
		tokens = append(tokens, t)
	}
	if len(filters) == 0 {
		return tokens, nil
	}

	resultingTokens := make([]*auth.Token, 0, len(tenants))
	lbs := make([][]prompbmarshal.Label, 0, len(filters))
	for _, token := range tokens {
		lbsL := make([]prompbmarshal.Label, 0)
		lbsL = append(lbsL, prompbmarshal.Label{
			Name:  "vm_account_id",
			Value: fmt.Sprintf("%d", token.AccountID),
		})

		if token.ProjectID != 0 {
			lbsL = append(lbsL, prompbmarshal.Label{
				Name:  "vm_project_id",
				Value: fmt.Sprintf("%d", token.ProjectID),
			})
		}

		lbs = append(lbs, lbsL)
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
				continue
			}
		}
	}

	return resultingTokens, nil
}

// IsTenancyLabel returns true if the given label name is used for tenancy.
func IsTenancyLabel(name string) bool {
	name = strings.ToLower(name)
	return name == "vm_account_id" || name == "vm_project_id"
}
