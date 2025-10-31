package servers

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
)

var (
	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression "+
		"at the cost of precision loss")
	vminsertConnsShutdownDuration = flag.Duration("storage.vminsertConnsShutdownDuration", 25*time.Second, "The time needed for gradual closing of vminsert connections during "+
		"graceful shutdown. Bigger duration reduces spikes in CPU, RAM and disk IO load on the remaining vmstorage nodes during rolling restart. "+
		"Smaller duration reduces the time needed to close all the vminsert connections, thus reducing the time for graceful shutdown. "+
		"See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#improving-re-routing-performance-during-restart")
)

// NewVMInsertServer starts vminsertapi.VMInsertServer at the given addr serving the given storage.
func NewVMInsertServer(addr string, storage *storage.Storage) (*vminsertapi.VMInsertServer, error) {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		return nil, fmt.Errorf("invalid -precisionBits: %w", err)
	}
	api := &vminsertAPI{
		storage: storage,
	}

	return vminsertapi.NewVMInsertServer(addr, *vminsertConnsShutdownDuration, "vminsert", api, nil)
}

type vminsertAPI struct {
	storage *storage.Storage
}

// WriteRows implements lib/vminsertapi.API interface
func (v *vminsertAPI) WriteRows(rows []storage.MetricRow) error {
	v.storage.AddRows(rows, uint8(*precisionBits))
	return nil
}

// WriteMetadata implements lib/vminsertapi.API interface
func (v *vminsertAPI) WriteMetadata(rows []metricsmetadata.Row) error {
	v.storage.AddMetadataRows(rows)
	return nil
}

// IsReadOnly implements lib/vminsertapi.API interface
func (v *vminsertAPI) IsReadOnly() bool {
	return v.storage.IsReadOnly()
}
