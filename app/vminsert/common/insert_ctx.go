package common

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// InsertCtx contains common bits for data points insertion.
type InsertCtx struct {
	Labels []prompb.Label

	mrs            []storage.MetricRow
	metricNamesBuf []byte
}

// Reset resets ctx for future fill with rowsLen rows.
func (ctx *InsertCtx) Reset(rowsLen int) {
	for _, label := range ctx.Labels {
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
}

func (ctx *InsertCtx) marshalMetricNameRaw(prefix []byte, labels []prompb.Label) []byte {
	start := len(ctx.metricNamesBuf)
	ctx.metricNamesBuf = append(ctx.metricNamesBuf, prefix...)
	ctx.metricNamesBuf = storage.MarshalMetricNameRaw(ctx.metricNamesBuf, labels)
	metricNameRaw := ctx.metricNamesBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

// WriteDataPoint writes (timestamp, value) with the given prefix and labels into ctx buffer.
func (ctx *InsertCtx) WriteDataPoint(prefix []byte, labels []prompb.Label, timestamp int64, value float64) {
	metricNameRaw := ctx.marshalMetricNameRaw(prefix, labels)
	ctx.addRow(metricNameRaw, timestamp, value)
}

// WriteDataPointExt writes (timestamp, value) with the given metricNameRaw and labels into ctx buffer.
//
// It returns metricNameRaw for the given labels if len(metricNameRaw) == 0.
func (ctx *InsertCtx) WriteDataPointExt(metricNameRaw []byte, labels []prompb.Label, timestamp int64, value float64) []byte {
	if len(metricNameRaw) == 0 {
		metricNameRaw = ctx.marshalMetricNameRaw(nil, labels)
	}
	ctx.addRow(metricNameRaw, timestamp, value)
	return metricNameRaw
}

func (ctx *InsertCtx) addRow(metricNameRaw []byte, timestamp int64, value float64) {
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
}

// AddLabelBytes adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabelBytes(name, value []byte) {
	labels := ctx.Labels
	if cap(labels) > len(labels) {
		labels = labels[:len(labels)+1]
	} else {
		labels = append(labels, prompb.Label{})
	}
	label := &labels[len(labels)-1]

	// Do not copy name and value contents for performance reasons.
	// This reduces GC overhead on the number of objects and allocations.
	label.Name = name
	label.Value = value

	ctx.Labels = labels
}

// AddLabel adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabel(name, value string) {
	labels := ctx.Labels
	if cap(labels) > len(labels) {
		labels = labels[:len(labels)+1]
	} else {
		labels = append(labels, prompb.Label{})
	}
	label := &labels[len(labels)-1]

	// Do not copy name and value contents for performance reasons.
	// This reduces GC overhead on the number of objects and allocations.
	label.Name = bytesutil.ToUnsafeBytes(name)
	label.Value = bytesutil.ToUnsafeBytes(value)

	ctx.Labels = labels
}

// FlushBufs flushes buffered rows to the underlying storage.
func (ctx *InsertCtx) FlushBufs() error {
	if err := vmstorage.AddRows(ctx.mrs); err != nil {
		return &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot store metrics: %w", err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return nil
}
