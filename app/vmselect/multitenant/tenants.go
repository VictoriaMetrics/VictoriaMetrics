package multitenant

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// FetchTenants returns the list of tenants available in the storage.
func FetchTenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	// todo: cache tenants and periodically fetch from storage
	tenants, err := netstorage.Tenants(qt, tr, deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain tenants: %w", err)
	}

	qt.Printf("fetched %d tenants", len(tenants))

	return tenants, nil
}
