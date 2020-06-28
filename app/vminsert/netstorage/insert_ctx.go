package netstorage

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	xxhash "github.com/cespare/xxhash/v2"
	jump "github.com/lithammer/go-jump-consistent-hash"
)

// InsertCtx is a generic context for inserting data.
//
// InsertCtx.Reset must be called before the first usage.
type InsertCtx struct {
	Labels        []prompb.Label
	MetricNameBuf []byte

	groupBufRowss           map[string][]bufRows
	labelsBuf               []byte
	failedReplicationGroups map[string]bool
}

type bufRows struct {
	buf  []byte
	rows int
}

func (br *bufRows) reset() {
	br.buf = br.buf[:0]
	br.rows = 0
}

func (br *bufRows) pushTo(sn *storageNode) error {
	bufLen := len(br.buf)
	err := sn.push(br.buf, br.rows)
	br.reset()
	if err != nil {
		return &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot send %d bytes to storageNode %q: %s", bufLen, sn.dialer.Addr(), err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return nil
}

// Reset resets ctx.
func (ctx *InsertCtx) Reset() {
	for _, label := range ctx.Labels {
		label.Name = nil
		label.Value = nil
	}
	ctx.Labels = ctx.Labels[:0]
	ctx.MetricNameBuf = ctx.MetricNameBuf[:0]
	if ctx.groupBufRowss == nil {
		ctx.groupBufRowss = make(map[string][]bufRows)
	}
	for rgID, rg := range replicationGroups {
		bufRowss, ok := ctx.groupBufRowss[rgID]
		if !ok {
			ctx.groupBufRowss[rgID] = make([]bufRows, len(rg.storageNodes))
		} else {
			for i := range bufRowss {
				bufRowss[i].reset()
			}
		}
	}
	ctx.labelsBuf = ctx.labelsBuf[:0]
	ctx.failedReplicationGroups = make(map[string]bool)
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

// WriteDataPoint writes (timestamp, value) data point with the given at and labels to ctx buffer.
func (ctx *InsertCtx) WriteDataPoint(at *auth.Token, labels []prompb.Label, timestamp int64, value float64) error {
	ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, labels)
	storageNodeGroupIDs := ctx.GetStorageNodeGroupIds(at, labels)
	for _, storageNodeGroupID := range storageNodeGroupIDs {
		if err := ctx.WriteDataPointExt(at, storageNodeGroupID, ctx.MetricNameBuf, timestamp, value); err != nil {
			if ctx.AllReplicationGroupsFailed(storageNodeGroupID.Group) {
				logger.Errorf("All replication groups have failed")
				return err
			}
			logger.Errorf("Replciation group down: %s error: %s", storageNodeGroupID.Group, err)
		}
	}

	return nil
}

// WriteDataPointExt writes the given metricNameRaw with (timestmap, value) to ctx buffer with the given storageNodeIdx.
func (ctx *InsertCtx) WriteDataPointExt(at *auth.Token, sngID StorageNodeGroupID, metricNameRaw []byte, timestamp int64, value float64) error {
	rg := replicationGroups[sngID.Group]
	br := &ctx.groupBufRowss[sngID.Group][sngID.Idx]
	sn := rg.storageNodes[sngID.Idx]
	bufNew := storage.MarshalMetricRow(br.buf, metricNameRaw, timestamp, value)
	if len(bufNew) >= rg.maxBufSizePerStorageNode {
		// Send buf to storageNode, since it is too big.
		if err := br.pushTo(sn); err != nil {
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
	for rgID, rg := range replicationGroups {
		bufRowss := ctx.groupBufRowss[rgID]
		for i := range bufRowss {
			br := &bufRowss[i]
			if len(br.buf) == 0 {
				continue
			}
			if err := br.pushTo(rg.storageNodes[i]); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// StorageNodeGroupID keeps the integer id of a node and it's group
type StorageNodeGroupID struct {
	Group string
	Idx   int
}

// GetStorageNodeGroupIds returns one storage node per replication group
func (ctx *InsertCtx) GetStorageNodeGroupIds(at *auth.Token, labels []prompb.Label) []StorageNodeGroupID {
	groupPairs := []StorageNodeGroupID{}

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
	for rgID, rg := range replicationGroups {
		if len(rg.storageNodes) == 1 {
			// Fast path - only a single storage node.
			groupPairs = append(groupPairs, StorageNodeGroupID{Group: rgID, Idx: 0})
			continue
		}

		idx := int(jump.Hash(h, int32(len(rg.storageNodes))))
		groupPairs = append(groupPairs, StorageNodeGroupID{Group: rgID, Idx: idx})
	}
	return groupPairs
}

// AllReplicationGroupsFailed - check if all replication groups have failed
func (ctx *InsertCtx) AllReplicationGroupsFailed(replicationGroup string) bool {
	ctx.failedReplicationGroups[replicationGroup] = true

	return len(replicationGroups) == len(ctx.failedReplicationGroups)
}

func marshalBytesFast(dst []byte, s []byte) []byte {
	dst = encoding.MarshalUint16(dst, uint16(len(s)))
	dst = append(dst, s...)
	return dst
}
