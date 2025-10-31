package clusternative

import (
	"crypto/tls"
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
	"github.com/VictoriaMetrics/metrics"
)

var (
	vminsertConnsShutdownDuration = flag.Duration("clusternative.vminsertConnsShutdownDuration", 25*time.Second, "The time needed for gradual closing of upstream "+
		"vminsert connections during graceful shutdown. Bigger duration reduces spikes in CPU, RAM and disk IO load on the remaining lower-level clusters "+
		"during rolling restart. Smaller duration reduces the time needed to close all the upstream vminsert connections, thus reducing the time for graceful shutdown. "+
		"See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#improving-re-routing-performance-during-restart")
)

// NewVMinsertServer creates and start vminsert server at the given addr
func NewVMinsertServer(addr string, tc *tls.Config) (*vminsertapi.VMInsertServer, error) {
	api := &vminsertAPI{}
	return vminsertapi.NewVMInsertServer(addr, *vminsertConnsShutdownDuration, "clusternative", api, tc)
}

type vminsertAPI struct {
}

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="clusternative"}`)
	metadataInserted   = metrics.NewCounter(`vm_metadata_inserted_total{type="clusternative"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="clusternative"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="clusternative"}`)
)

// WriteRows implements lib/vminsertapi/API interface
func (v *vminsertAPI) WriteRows(rows []storage.MetricRow) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	hasRelabeling := relabel.HasRelabeling()
	var at auth.Token
	var rowsPerTenant *metrics.Counter
	var mn storage.MetricName
	for i := range rows {
		mr := &rows[i]
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			return fmt.Errorf("cannot unmarshal MetricNameRaw: %w", err)
		}
		if rowsPerTenant == nil || mn.AccountID != at.AccountID || mn.ProjectID != at.ProjectID {
			at.AccountID = mn.AccountID
			at.ProjectID = mn.ProjectID
			rowsPerTenant = rowsTenantInserted.Get(&at)
		}
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabelBytes(nil, mn.MetricGroup)
		for j := range mn.Tags {
			tag := &mn.Tags[j]
			ctx.AddLabelBytes(tag.Key, tag.Value)
		}
		if !ctx.TryPrepareLabels(hasRelabeling) {
			continue
		}
		if err := ctx.WriteDataPoint(&at, ctx.Labels, mr.Timestamp, mr.Value); err != nil {
			return err
		}
		rowsPerTenant.Inc()
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}

// WriteMetadata implements lib/vminsertapi/API interface
func (v *vminsertAPI) WriteMetadata(mrs []metricsmetadata.Row) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.ResetForMetricsMetadata() // This line is required for initializing ctx internals.
	for i := range mrs {
		row := &mrs[i]
		ctx.Buf = row.MarshalTo(ctx.Buf[:0])
		storageNodeIdx := ctx.GetStorageNodeIdxForMeta(ctx.Buf)
		if err := ctx.WriteMetadataExt(storageNodeIdx, ctx.Buf); err != nil {
			return err
		}
	}

	metadataInserted.Add(len(mrs))

	return ctx.FlushBufs()
}

// IsReadOnly implements lib/vminsertapi/API interface
func (v *vminsertAPI) IsReadOnly() bool {
	return false
}
