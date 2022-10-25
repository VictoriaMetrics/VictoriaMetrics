package netstorage

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/cespare/xxhash/v2"
)

// InsertCtx is a generic context for inserting data.
//
// InsertCtx.Reset must be called before the first usage.
type InsertCtx struct {
	sns           []*storageNode
	Labels        sortedLabels
	MetricNameBuf []byte

	bufRowss  []bufRows
	labelsBuf []byte

	relabelCtx relabel.Ctx

	at auth.Token
}

type bufRows struct {
	buf  []byte
	rows int
}

func (br *bufRows) reset() {
	br.buf = br.buf[:0]
	br.rows = 0
}

func (br *bufRows) pushTo(sns []*storageNode, sn *storageNode) error {
	bufLen := len(br.buf)
	err := sn.push(sns, br.buf, br.rows)
	br.reset()
	if err != nil {
		return &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot send %d bytes to storageNode %q: %w", bufLen, sn.dialer.Addr(), err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return nil
}

// Reset resets ctx.
func (ctx *InsertCtx) Reset() {
	ctx.sns = getStorageNodes()
	for i := range ctx.Labels {
		label := &ctx.Labels[i]
		label.Name = nil
		label.Value = nil
	}
	ctx.Labels = ctx.Labels[:0]
	ctx.MetricNameBuf = ctx.MetricNameBuf[:0]

	if ctx.bufRowss == nil {
		ctx.bufRowss = make([]bufRows, len(ctx.sns))
	}
	for i := range ctx.bufRowss {
		ctx.bufRowss[i].reset()
	}
	ctx.labelsBuf = ctx.labelsBuf[:0]
	ctx.relabelCtx.Reset()
	ctx.at.Set(0, 0)
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

// ApplyRelabeling applies relabeling to ctx.Labels.
func (ctx *InsertCtx) ApplyRelabeling() {
	ctx.Labels = ctx.relabelCtx.ApplyRelabeling(ctx.Labels)
}

// WriteDataPoint writes (timestamp, value) data point with the given at and labels to ctx buffer.
func (ctx *InsertCtx) WriteDataPoint(at *auth.Token, labels []prompb.Label, timestamp int64, value float64) error {
	ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, labels)
	storageNodeIdx := ctx.GetStorageNodeIdx(at, labels)
	return ctx.WriteDataPointExt(storageNodeIdx, ctx.MetricNameBuf, timestamp, value)
}

// WriteDataPointExt writes the given metricNameRaw with (timestmap, value) to ctx buffer with the given storageNodeIdx.
func (ctx *InsertCtx) WriteDataPointExt(storageNodeIdx int, metricNameRaw []byte, timestamp int64, value float64) error {
	br := &ctx.bufRowss[storageNodeIdx]
	sns := ctx.sns
	sn := sns[storageNodeIdx]
	bufNew := storage.MarshalMetricRow(br.buf, metricNameRaw, timestamp, value)
	if len(bufNew) >= maxBufSizePerStorageNode {
		// Send buf to sn, since it is too big.
		if err := br.pushTo(sns, sn); err != nil {
			return err
		}
		br.buf = storage.MarshalMetricRow(bufNew[:0], metricNameRaw, timestamp, value)
	} else {
		br.buf = bufNew
	}
	br.rows++
	return nil
}

// FlushBufs flushes ctx bufs to remote storage nodes.
func (ctx *InsertCtx) FlushBufs() error {
	var firstErr error
	for i := range ctx.bufRowss {
		br := &ctx.bufRowss[i]
		if len(br.buf) == 0 {
			continue
		}
		if err := br.pushTo(ctx.sns, ctx.sns[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetStorageNodeIdx returns storage node index for the given at and labels.
//
// The returned index must be passed to WriteDataPoint.
func (ctx *InsertCtx) GetStorageNodeIdx(at *auth.Token, labels []prompb.Label) int {
	if len(ctx.sns) == 1 {
		// Fast path - only a single storage node.
		return 0
	}

	buf := ctx.labelsBuf[:0]
	buf = encoding.MarshalUint32(buf, at.AccountID)
	buf = encoding.MarshalUint32(buf, at.ProjectID)
	for i := range labels {
		label := &labels[i]
		buf = marshalBytesFast(buf, label.Name)
		buf = marshalBytesFast(buf, label.Value)
	}
	h := xxhash.Sum64(buf)
	ctx.labelsBuf = buf

	// Do not exclude unavailable storage nodes in order to properly account for rerouted rows in storageNode.push().
	idx := nodesHash.getNodeIdx(h, nil)
	return idx
}

func marshalBytesFast(dst []byte, s []byte) []byte {
	dst = encoding.MarshalUint16(dst, uint16(len(s)))
	dst = append(dst, s...)
	return dst
}

// GetLocalAuthToken obtains auth.Token from context labels vm_account_id and vm_project_id if at is nil.
//
// At is returned as is if it isn't nil.
//
// The vm_account_id and vm_project_id labels are automatically removed from the ctx.
func (ctx *InsertCtx) GetLocalAuthToken(at *auth.Token) *auth.Token {
	if at != nil {
		return at
	}
	accountID := uint32(0)
	projectID := uint32(0)
	tmpLabels := ctx.Labels[:0]
	for _, label := range ctx.Labels {
		if string(label.Name) == "vm_account_id" {
			accountID = parseUint32(label.Value)
			continue
		}
		if string(label.Name) == "vm_project_id" {
			projectID = parseUint32(label.Value)
			continue
		}
		tmpLabels = append(tmpLabels, label)
	}
	cleanLabels := ctx.Labels[len(tmpLabels):]
	for i := range cleanLabels {
		label := &cleanLabels[i]
		label.Name = nil
		label.Value = nil
	}
	ctx.Labels = tmpLabels
	ctx.at.Set(accountID, projectID)
	return &ctx.at
}

func parseUint32(b []byte) uint32 {
	s := bytesutil.ToUnsafeString(b)
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}
