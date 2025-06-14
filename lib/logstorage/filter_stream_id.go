package logstorage

import (
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterStreamID is the filter for `_stream_id:id`
type filterStreamID struct {
	streamIDs []streamID

	// If q is non-nil, then streamIDs must be populated from q before filter execution.
	q *Query

	// qFieldName must be set to field name for obtaining values from if q is non-nil.
	qFieldName string

	streamIDsMap     map[string]struct{}
	streamIDsMapOnce sync.Once
}

func (fs *filterStreamID) String() string {
	if fs.q != nil {
		return "_stream_id:in(" + fs.q.String() + ")"
	}

	streamIDs := fs.streamIDs
	if len(streamIDs) == 1 {
		return "_stream_id:" + string(streamIDs[0].marshalString(nil))
	}

	a := make([]string, len(streamIDs))
	for i, streamID := range streamIDs {
		a[i] = string(streamID.marshalString(nil))
	}
	return "_stream_id:in(" + strings.Join(a, ",") + ")"
}

func (fs *filterStreamID) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_stream_id")
}

func (fs *filterStreamID) getStreamIDsMap() map[string]struct{} {
	fs.streamIDsMapOnce.Do(fs.initStreamIDsMap)
	return fs.streamIDsMap
}

func (fs *filterStreamID) initStreamIDsMap() {
	m := make(map[string]struct{}, len(fs.streamIDs))
	for _, streamID := range fs.streamIDs {
		k := streamID.marshalString(nil)
		m[string(k)] = struct{}{}
	}
	fs.streamIDsMap = m
}

func (fs *filterStreamID) applyToBlockResult(br *blockResult, bm *bitmap) {
	m := fs.getStreamIDsMap()

	if len(m) == 0 {
		bm.resetBits()
		return
	}

	c := br.getColumnByName("_stream_id")
	if c.isConst {
		v := c.valuesEncoded[0]
		if _, ok := m[v]; !ok {
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
			_, ok := m[v]
			return ok
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			ch := byte(0)
			_, ok := m[v]
			if ok {
				ch = 1
			}
			bb.B = append(bb.B, ch)
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

func (fs *filterStreamID) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	m := fs.getStreamIDsMap()
	if len(m) == 0 {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	bb.B = bs.bsw.bh.streamID.marshalString(bb.B)
	_, ok := m[string(bb.B)]
	bbPool.Put(bb)

	if !ok {
		bm.resetBits()
		return
	}
}
