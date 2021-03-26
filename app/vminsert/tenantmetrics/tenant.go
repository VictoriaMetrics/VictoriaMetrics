package tenantmetrics

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"

var (
	// RowsInsertedByTenant represents per tenant metric rows ingestion statistic.
	RowsInsertedByTenant = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total`)
)
