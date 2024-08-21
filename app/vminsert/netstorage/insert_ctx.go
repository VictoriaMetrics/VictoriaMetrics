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
	snb           *storageNodesBucket
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

func (br *bufRows) pushTo(snb *storageNodesBucket, sn *storageNode) error {
	bufLen := len(br.buf)
	err := sn.push(snb, br.buf, br.rows)
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
	ctx.snb = getStorageNodesBucket()

	labels := ctx.Labels
	for i := range labels {
		labels[i] = prompb.Label{}
	}
	ctx.Labels = labels[:0]

	ctx.MetricNameBuf = ctx.MetricNameBuf[:0]

	if ctx.bufRowss == nil || len(ctx.bufRowss) != len(ctx.snb.sns) {
		ctx.bufRowss = make([]bufRows, len(ctx.snb.sns))
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

// ApplyRelabeling applies relabeling to ctx.Labels.
func (ctx *InsertCtx) ApplyRelabeling() {
	ctx.Labels = ctx.relabelCtx.ApplyRelabeling(ctx.Labels)
}

// WriteDataPoint writes (timestamp, value) data point with the given at and labels to ctx buffer.
func (ctx *InsertCtx) WriteDataPoint(at *auth.Token, labels []prompb.Label, timestamp int64, value float64) error {
	ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, labels)
	storageNodeIdx := ctx.GetStorageNodeIdx(at, labels)
	return ctx.writeDataPointToReplicas(storageNodeIdx, ctx.MetricNameBuf, timestamp, value)
}

// WriteDataPointExt writes the given metricNameRaw with (timestmap, value) to ctx buffer with the given storageNodeIdx.
func (ctx *InsertCtx) WriteDataPointExt(storageNodeIdx int, metricNameRaw []byte, timestamp int64, value float64) error {
	return ctx.writeDataPointToReplicas(storageNodeIdx, metricNameRaw, timestamp, value)
}

func (ctx *InsertCtx) writeDataPointToReplicas(storageNodeIdx int, metricNameRaw []byte, timestamp int64, value float64) error {
	var firstErr error
	var failsCount int
	for i := 0; i < replicas; i++ {
		snIdx := storageNodeIdx + i
		if snIdx >= len(ctx.snb.sns) {
			snIdx %= len(ctx.snb.sns)
		}

		if err := ctx.writeDataPointExt(snIdx, metricNameRaw, timestamp, value); err != nil {
			if replicas == 1 {
				return fmt.Errorf("cannot write datapoint: %w", err)
			}
			if firstErr == nil {
				firstErr = err
			}
			failsCount++
			// The data is partially replicated, so just emit a warning and return true.
			// We could retry sending the data again, but this may result in uncontrolled duplicate data.
			// So it is better returning true.
			br := &ctx.bufRowss[snIdx]
			rowsIncompletelyReplicatedTotal.Add(br.rows)
			incompleteReplicationLogger.Warnf("cannot make a copy #%d out of %d copies according to -replicationFactor=%d, used_nodes=%d for %d bytes with %d rows, "+
				"since a part of storage nodes is temporarily unavailable", i+1, replicas, *replicationFactor, len(br.buf), br.rows)
			continue
		}
	}
	if failsCount == replicas {
		return fmt.Errorf("cannot write datapoint to any replicas: %w", firstErr)
	}
	return nil
}

func (ctx *InsertCtx) writeDataPointExt(storageNodeIdx int, metricNameRaw []byte, timestamp int64, value float64) error {
	br := &ctx.bufRowss[storageNodeIdx]
	snb := ctx.snb
	sn := snb.sns[storageNodeIdx]
	bufNew := storage.MarshalMetricRow(br.buf, metricNameRaw, timestamp, value)
	if len(bufNew) >= maxBufSizePerStorageNode {
		// Send buf to sn, since it is too big.
		if err := br.pushTo(snb, sn); err != nil {
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
	snb := ctx.snb
	sns := snb.sns
	for i := range ctx.bufRowss {
		br := &ctx.bufRowss[i]
		if len(br.buf) == 0 {
			continue
		}
		if err := br.pushTo(snb, sns[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetStorageNodeIdx returns storage node index for the given at and labels.
//
// The returned index must be passed to WriteDataPoint.
func (ctx *InsertCtx) GetStorageNodeIdx(at *auth.Token, labels []prompb.Label) int {
	if len(ctx.snb.sns) == 1 {
		// Fast path - only a single storage node.
		return 0
	}

	buf := ctx.labelsBuf[:0]
	buf = encoding.MarshalUint32(buf, at.AccountID)
	buf = encoding.MarshalUint32(buf, at.ProjectID)
	for i := range labels {
		label := &labels[i]
		buf = marshalStringFast(buf, label.Name)
		buf = marshalStringFast(buf, label.Value)
	}
	h := xxhash.Sum64(buf)
	ctx.labelsBuf = buf

	// Do not exclude unavailable storage nodes in order to properly account for rerouted rows in storageNode.push().
	idx := ctx.snb.nodesHash.getNodeIdx(h, nil)
	return idx
}

func marshalStringFast(dst []byte, s string) []byte {
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
		switch string(label.Name) {
		case "vm_account_id":
			accountID = parseUint32(label.Value)
			continue
		case "vm_project_id":
			projectID = parseUint32(label.Value)
			continue
		// do not remove labels from labelSet for backward-compatibility
		// previous realisation kept it
		case "VictoriaMetrics_AccountID":
			accountID = parseUint32(label.Value)
		case "VictoriaMetrics_ProjectID":
			projectID = parseUint32(label.Value)
		}
		tmpLabels = append(tmpLabels, label)
	}
	cleanLabels := ctx.Labels[len(tmpLabels):]
	for i := range cleanLabels {
		cleanLabels[i] = prompb.Label{}
	}
	ctx.Labels = tmpLabels
	ctx.at.Set(accountID, projectID)
	return &ctx.at
}

func parseUint32(s string) uint32 {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}
