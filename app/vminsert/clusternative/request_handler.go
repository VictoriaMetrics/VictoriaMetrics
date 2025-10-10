package clusternative

import (
	"fmt"
	"net"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="clusternative"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="clusternative"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="clusternative"}`)
)

// InsertHandler processes data from vminsert nodes.
func InsertHandler(c net.Conn) error {
	// There is no need in response compression, since
	// lower-level vminsert sends only small packets to upper-level vminsert.
	bc, err := handshake.VMInsertServer(c, 0)
	if err != nil {
		if handshake.IsTCPHealthcheck(err) {
			return nil
		}
		if handshake.IsTimeoutNetworkError(err) {
			logger.Warnf("cannot complete vminsert handshake due to network timeout error with client %q: %s. "+
				"If errors are transient and infrequent increase -rpc.handshakeTimeout and -vmstorageDialTimeout on client and server side. Check vminsert logs for errors", c.RemoteAddr(), err)
			return nil
		}
		if handshake.IsClientNetworkError(err) {
			logger.Warnf("cannot complete vminsert handshake due to network error with client %q: %s. "+
				"Check vminsert logs for errors", c.RemoteAddr(), err)
			return nil
		}

		return fmt.Errorf("cannot perform vminsert handshake with client %q: %w", c.RemoteAddr(), err)
	}
	if bc.IsNotRPCCompatible {
		// fallback to the prev api version
		return stream.Parse(bc, func(rows []storage.MetricRow) error {
			return insertRows(rows)
		}, nil)
	}
	if bc.IsStreamingMode {
		logger.Panicf("BUG: clusternative connection in streaming mode cannot process RPC calls")
	}
	ctx := vminsertapi.NewRequestCtx(bc)
	rpcName, err := ctx.ReadRPCName()
	if err != nil {
		return fmt.Errorf("cannot read rpcName: %w", err)
	}
	switch string(rpcName) {
	case vminsertapi.MetricRowsRpcCall.VersionedName:
		// send back empty error message
		if err := ctx.WriteString(""); err != nil {
			return fmt.Errorf("cannot send empty error message: %w", err)
		}
		bc.IsStreamingMode = true
		return stream.Parse(bc, func(rows []storage.MetricRow) error {
			return insertRows(rows)
		}, nil)
	case vminsertapi.MetricMetadataRpcCall.VersionedName:
		// send back empty error message
		if err := ctx.WriteString(""); err != nil {
			return fmt.Errorf("cannot send empty error message: %w", err)
		}
		// TODO: implement me
		return nil
	default:
		err := fmt.Errorf("unsupported rpcName: %q", rpcName)
		if writeErr := ctx.WriteErrorMessage(err); writeErr != nil {
			err = fmt.Errorf("cannot write rpcName error back to client: %s: %w", writeErr, err)
		}
		return err
	}
}

func insertRows(rows []storage.MetricRow) error {
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
