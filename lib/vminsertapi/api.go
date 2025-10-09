package vminsertapi

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// RPCCall defines rpc call from vminsert to vmstorage
type RPCCall struct {
	Name          string
	VersionedName string
}

var (
	MetricRowsRpcCall = RPCCall{
		Name:          "metric_rows",
		VersionedName: "writeRows_v1",
	}
	MetricMetadataRpcCall = RPCCall{
		Name:          "metricmetadata_rows",
		VersionedName: "writeMetadata_v1",
	}
)

// API must implement vminsert API.
type API interface {
	WriteRows(rows []storage.MetricRow) error
	WriteMetadata(mrs []storage.MetricMetadataRow) error
	IsReadOnly() bool
}
