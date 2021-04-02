package common

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// InsertCtx contains common bits for data points insertion.
type InsertCtx struct {
	Labels sortedLabels

	mrs            []storage.MetricRow
	metricNamesBuf []byte

	relabelCtx relabel.Ctx
}

// Reset resets ctx for future fill with rowsLen rows.
func (ctx *InsertCtx) Reset(rowsLen int) {
	for i := range ctx.Labels {
		label := &ctx.Labels[i]
		label.Name = nil
		label.Value = nil
	}
	ctx.Labels = ctx.Labels[:0]

	for i := range ctx.mrs {
		mr := &ctx.mrs[i]
		mr.MetricNameRaw = nil
	}
	ctx.mrs = ctx.mrs[:0]
	if n := rowsLen - cap(ctx.mrs); n > 0 {
		ctx.mrs = append(ctx.mrs[:cap(ctx.mrs)], make([]storage.MetricRow, n)...)
	}
	ctx.mrs = ctx.mrs[:0]
	ctx.metricNamesBuf = ctx.metricNamesBuf[:0]
	ctx.relabelCtx.Reset()
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
		Name:  name,
		Value: value,
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
		Name:  bytesutil.ToUnsafeBytes(name),
		Value: bytesutil.ToUnsafeBytes(value),
	})
}

// ApplyRelabeling applies relabeling to ic.Labels.
func (ctx *InsertCtx) ApplyRelabeling() {
	ctx.Labels = ctx.relabelCtx.ApplyRelabeling(ctx.Labels)
}

// FlushBufs flushes buffered rows to the underlying storage.
func (ctx *InsertCtx) FlushBufs() error {
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
