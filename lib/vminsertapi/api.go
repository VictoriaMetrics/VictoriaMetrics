package vminsertapi

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

// API must implement vminsert API.
type API interface {
	WriteRows(rows []storage.MetricRow) error
	WriteMetadata(mrs []metricsmetadata.Row) error
	IsReadOnly() bool
}
