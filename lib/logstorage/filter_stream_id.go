package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterStreamID is the filter for `_stream_id:id`
type filterStreamID struct {
	streamIDStr string
}

func (fs *filterStreamID) String() string {
	return "_stream_id:" + quoteTokenIfNeeded(fs.streamIDStr)
}

func (fs *filterStreamID) updateNeededFields(neededFields fieldsSet) {
	neededFields.add("_stream_id")
}

func (fs *filterStreamID) applyToBlockResult(br *blockResult, bm *bitmap) {
	c := br.getColumnByName("_stream_id")
	if c.isConst {
		v := c.valuesEncoded[0]
		if fs.streamIDStr != v {
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
			return fs.streamIDStr == v
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if fs.streamIDStr == v {
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
	bb := bbPool.Get()
	bb.B = bs.bsw.bh.streamID.marshalString(bb.B)
	ok := fs.streamIDStr == string(bb.B)
	bbPool.Put(bb)
	if !ok {
		bm.resetBits()
		return
	}
}
