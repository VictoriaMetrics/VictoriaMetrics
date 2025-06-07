package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterTime filters by time.
//
// It is expressed as `_time:[start, end]` in LogsQL.
type filterTime struct {
	// mintimestamp is the minimum timestamp in nanoseconds to find
	minTimestamp int64

	// maxTimestamp is the maximum timestamp in nanoseconds to find
	maxTimestamp int64

	// stringRepr is string representation of the filter
	stringRepr string
}

func (ft *filterTime) String() string {
	return "_time:" + ft.stringRepr
}

func (ft *filterTime) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_time")
}

func (ft *filterTime) applyToBlockResult(br *blockResult, bm *bitmap) {
	if ft.minTimestamp > ft.maxTimestamp {
		bm.resetBits()
		return
	}

	c := br.getColumnByName("_time")
	if c.isConst {
		v := c.valuesEncoded[0]
		if !ft.matchTimestampString(v) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		bm.forEachSetBit(func(idx int) bool {
			timestamp := timestamps[idx]
			return ft.matchTimestampValue(timestamp)
		})
		return
	}

	switch c.valueType {
	case valueTypeString:
		values := c.getValues(br)
		bm.forEachSetBit(func(idx int) bool {
			v := values[idx]
			return ft.matchTimestampString(v)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if ft.matchTimestampString(v) {
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
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			timestamp := unmarshalTimestampISO8601(v)
			return ft.matchTimestampValue(timestamp)
		})
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (ft *filterTime) matchTimestampString(v string) bool {
	timestamp, ok := TryParseTimestampRFC3339Nano(v)
	if !ok {
		return false
	}
	return ft.matchTimestampValue(timestamp)
}

func (ft *filterTime) matchTimestampValue(timestamp int64) bool {
	return timestamp >= ft.minTimestamp && timestamp <= ft.maxTimestamp
}

func (ft *filterTime) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	minTimestamp := ft.minTimestamp
	maxTimestamp := ft.maxTimestamp

	if minTimestamp > maxTimestamp {
		bm.resetBits()
		return
	}

	th := bs.bsw.bh.timestampsHeader
	if minTimestamp > th.maxTimestamp || maxTimestamp < th.minTimestamp {
		bm.resetBits()
		return
	}
	if minTimestamp <= th.minTimestamp && maxTimestamp >= th.maxTimestamp {
		return
	}

	timestamps := bs.getTimestamps()
	bm.forEachSetBit(func(idx int) bool {
		ts := timestamps[idx]
		return ts >= minTimestamp && ts <= maxTimestamp
	})
}
