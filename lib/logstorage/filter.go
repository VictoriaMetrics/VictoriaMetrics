package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type filter interface {
	// String returns string representation of the filter
	String() string

	// apply must update bm according to the filter applied to the given bs block
	apply(bs *blockSearch, bm *bitmap)
}

// streamFilter is the filter for `_stream:{...}`
type streamFilter struct {
	// f is the filter to apply
	f *StreamFilter

	// tenantIDs is the list of tenantIDs to search for streamIDs.
	tenantIDs []TenantID

	// idb is the indexdb to search for streamIDs.
	idb *indexdb

	streamIDsOnce sync.Once
	streamIDs     map[streamID]struct{}
}

func (fs *streamFilter) String() string {
	s := fs.f.String()
	if s == "{}" {
		return ""
	}
	return "_stream:" + s
}

func (fs *streamFilter) getStreamIDs() map[streamID]struct{} {
	fs.streamIDsOnce.Do(fs.initStreamIDs)
	return fs.streamIDs
}

func (fs *streamFilter) initStreamIDs() {
	streamIDs := fs.idb.searchStreamIDs(fs.tenantIDs, fs.f)
	m := make(map[streamID]struct{}, len(streamIDs))
	for i := range streamIDs {
		m[streamIDs[i]] = struct{}{}
	}
	fs.streamIDs = m
}

func (fs *streamFilter) apply(bs *blockSearch, bm *bitmap) {
	if fs.f.isEmpty() {
		return
	}
	streamIDs := fs.getStreamIDs()
	if _, ok := streamIDs[bs.bsw.bh.streamID]; !ok {
		bm.resetBits()
		return
	}
}

func toFloat64StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
	}
	bb.B = toFloat64String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}

func toIPv4StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 4 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of IPv4: got %d; want 4", bs.partPath(), len(v))
	}
	bb.B = toIPv4String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}

func toTimestampISO8601StringExt(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of ISO8601 timestamp: got %d; want 8", bs.partPath(), len(v))
	}
	bb.B = toTimestampISO8601String(bb.B[:0], v)
	return bytesutil.ToUnsafeString(bb.B)
}
