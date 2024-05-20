package common

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// InsertCtx contains common bits for data points insertion.
type InsertCtx struct {
	Labels sortedLabels

	mrs            []storage.MetricRow
	metricNamesBuf []byte

	relabelCtx    relabel.Ctx
	streamAggrCtx streamAggrCtx

	skipStreamAggr bool
}

// Reset resets ctx for future fill with rowsLen rows.
func (ctx *InsertCtx) Reset(rowsLen int) {
	labels := ctx.Labels
	for i := range labels {
		labels[i] = prompb.Label{}
	}
	ctx.Labels = labels[:0]

	mrs := ctx.mrs
	for i := range mrs {
		cleanMetricRow(&mrs[i])
	}
	mrs = slicesutil.SetLength(mrs, rowsLen)
	ctx.mrs = mrs[:0]

	ctx.metricNamesBuf = ctx.metricNamesBuf[:0]
	ctx.relabelCtx.Reset()
	ctx.streamAggrCtx.Reset()
	ctx.skipStreamAggr = false
}

func cleanMetricRow(mr *storage.MetricRow) {
	mr.MetricNameRaw = nil
}

func (ctx *InsertCtx) marshalMetricNameRaw(prefix []byte, labels []prompb.Label) []byte {
	start := len(ctx.metricNamesBuf)
	ctx.metricNamesBuf = append(ctx.metricNamesBuf, prefix...)
	ctx.metricNamesBuf = storage.MarshalMetricNameRaw(ctx.metricNamesBuf, labels)
	metricNameRaw := ctx.metricNamesBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

// WriteDataPoint writes (timestamp, value) with the given prefix and labels into ctx buffer.
func (ctx *InsertCtx) WriteDataPoint(prefix []byte, labels []prompb.Label, timestamp int64, value float64) error {
	metricNameRaw := ctx.marshalMetricNameRaw(prefix, labels)
	return ctx.addRow(metricNameRaw, timestamp, value)
}

// WriteDataPointExt writes (timestamp, value) with the given metricNameRaw and labels into ctx buffer.
//
// It returns metricNameRaw for the given labels if len(metricNameRaw) == 0.
func (ctx *InsertCtx) WriteDataPointExt(metricNameRaw []byte, labels []prompb.Label, timestamp int64, value float64) ([]byte, error) {
	if len(metricNameRaw) == 0 {
		metricNameRaw = ctx.marshalMetricNameRaw(nil, labels)
	}
	err := ctx.addRow(metricNameRaw, timestamp, value)
	return metricNameRaw, err
}

func (ctx *InsertCtx) addRow(metricNameRaw []byte, timestamp int64, value float64) error {
	mrs := ctx.mrs
	if cap(mrs) > len(mrs) {
		mrs = mrs[:len(mrs)+1]
	} else {
		mrs = append(mrs, storage.MetricRow{})
	}
	mr := &mrs[len(mrs)-1]
	ctx.mrs = mrs
	mr.MetricNameRaw = metricNameRaw
	mr.Timestamp = timestamp
	mr.Value = value
	if len(ctx.metricNamesBuf) > 16*1024*1024 {
		if err := ctx.FlushBufs(); err != nil {
			return err
		}
	}
	return nil
}

// AddLabelBytes adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabelBytes(name, value []byte) {
	if len(value) == 0 {
		// Skip labels without values, since they have no sense.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
		// Do not skip labels with empty name, since they are equal to __name__.
		return
	}
	ctx.Labels = append(ctx.Labels, prompb.Label{
		// Do not copy name and value contents for performance reasons.
		// This reduces GC overhead on the number of objects and allocations.
		Name:  bytesutil.ToUnsafeString(name),
		Value: bytesutil.ToUnsafeString(value),
	})
}

// AddLabel adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabel(name, value string) {
	if len(value) == 0 {
		// Skip labels without values, since they have no sense.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
		// Do not skip labels with empty name, since they are equal to __name__.
		return
	}
	ctx.Labels = append(ctx.Labels, prompb.Label{
		// Do not copy name and value contents for performance reasons.
		// This reduces GC overhead on the number of objects and allocations.
		Name:  name,
		Value: value,
	})
}

// ApplyRelabeling applies relabeling to ic.Labels.
func (ctx *InsertCtx) ApplyRelabeling() {
	ctx.Labels = ctx.relabelCtx.ApplyRelabeling(ctx.Labels)
}

// FlushBufs flushes buffered rows to the underlying storage.
func (ctx *InsertCtx) FlushBufs() error {
	sas := sasGlobal.Load()
	if (sas.IsEnabled() || deduplicator != nil) && !ctx.skipStreamAggr {
		matchIdxs := matchIdxsPool.Get()
		matchIdxs.B = ctx.streamAggrCtx.push(ctx.mrs, matchIdxs.B)
		if !*streamAggrKeepInput {
			// Remove aggregated rows from ctx.mrs
			ctx.dropAggregatedRows(matchIdxs.B)
		}
		matchIdxsPool.Put(matchIdxs)
	}
	// There is no need in limiting the number of concurrent calls to vmstorage.AddRows() here,
	// since the number of concurrent FlushBufs() calls should be already limited via writeconcurrencylimiter
	// used at every stream.Parse() call under lib/protoparser/*
	err := vmstorage.AddRows(ctx.mrs)
	ctx.Reset(0)
	if err == nil {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("cannot store metrics: %w", err),
		StatusCode: http.StatusServiceUnavailable,
	}
}

func (ctx *InsertCtx) dropAggregatedRows(matchIdxs []byte) {
	dst := ctx.mrs[:0]
	src := ctx.mrs
	if !*streamAggrDropInput {
		for idx, match := range matchIdxs {
			if match == 1 {
				continue
			}
			dst = append(dst, src[idx])
		}
	}
	tail := src[len(dst):]
	for i := range tail {
		cleanMetricRow(&tail[i])
	}
	ctx.mrs = dst
}

var matchIdxsPool bytesutil.ByteBufferPool
