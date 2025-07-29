package vminsertapi

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

type API interface {
	WriteRows(rows []storage.MetricRow) error
	WriteMetadata(mrs []metricsmetadata.Row) error
	IsReadOnly() bool
}
