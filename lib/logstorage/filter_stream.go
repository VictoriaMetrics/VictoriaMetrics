package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterStream is the filter for `{}` aka `_stream:{...}`
type filterStream struct {
	// f is the filter to apply
	f *StreamFilter

	// tenantIDs is the list of tenantIDs to search for streamIDs.
	//
	// This field is initialized just before the search.
	tenantIDs []TenantID

	// idb is the indexdb to search for streamIDs.
	//
	// This field is initialized just before the search.
	idb *indexdb

	streamIDsOnce sync.Once
	streamIDs     map[streamID]struct{}
}

func (fs *filterStream) String() string {
	return fs.f.String()
}

func (fs *filterStream) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_stream")
}

func (fs *filterStream) getStreamIDs() map[streamID]struct{} {
	fs.streamIDsOnce.Do(fs.initStreamIDs)
	return fs.streamIDs
}

func (fs *filterStream) initStreamIDs() {
	streamIDs := fs.idb.searchStreamIDs(fs.tenantIDs, fs.f)
	m := make(map[streamID]struct{}, len(streamIDs))
	for i := range streamIDs {
		m[streamIDs[i]] = struct{}{}
	}
	fs.streamIDs = m
}

func (fs *filterStream) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fs.f.isEmpty() {
		return
	}

	c := br.getColumnByName("_stream")
	if c.isConst {
		v := c.valuesEncoded[0]
		if !fs.f.matchStreamName(v) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		bm.resetBits()
		return
	}

	switch c.valueType {
	case valueTypeString:
		values := c.getValues(br)
		bm.forEachSetBit(func(idx int) bool {
			v := values[idx]
			return fs.f.matchStreamName(v)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if fs.f.matchStreamName(v) {
				c = 1
			}
			bb.B = append(bb.B, c)
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := valuesEncoded[idx][0]
			return bb.B[n] == 1
		})
		bbPool.Put(bb)
	case valueTypeUint8:
		bm.resetBits()
	case valueTypeUint16:
		bm.resetBits()
	case valueTypeUint32:
		bm.resetBits()
	case valueTypeUint64:
		bm.resetBits()
	case valueTypeInt64:
		bm.resetBits()
	case valueTypeFloat64:
		bm.resetBits()
	case valueTypeIPv4:
		bm.resetBits()
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fs *filterStream) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fs.f.isEmpty() {
		return
	}
	streamIDs := fs.getStreamIDs()
	if _, ok := streamIDs[bs.bsw.bh.streamID]; !ok {
		bm.resetBits()
		return
	}
}
