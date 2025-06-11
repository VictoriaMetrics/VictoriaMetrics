package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterDayRange filters by day range.
//
// It is expressed as `_time:day_range[start, end] offset d` in LogsQL.
type filterDayRange struct {
	// start is the offset in nanoseconds from the beginning of the day for the day range start.
	start int64

	// end is the offset in nanoseconds from the beginning of the day for the day range end.
	end int64

	// offset is the offset, which must be applied to _time before applying [start, end] filter to it.
	offset int64

	// stringRepr is string representation of the filter.
	stringRepr string
}

func (fr *filterDayRange) String() string {
	return "_time:day_range" + fr.stringRepr
}

func (fr *filterDayRange) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_time")
}

func (fr *filterDayRange) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fr.start > fr.end {
		bm.resetBits()
		return
	}
	if fr.start == 0 && fr.end == nsecsPerDay-1 {
		return
	}

	c := br.getColumnByName("_time")
	if c.isConst {
		v := c.valuesEncoded[0]
		if !fr.matchTimestampString(v) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		bm.forEachSetBit(func(idx int) bool {
			timestamp := timestamps[idx]
			return fr.matchTimestampValue(timestamp)
		})
		return
	}

	switch c.valueType {
	case valueTypeString:
		values := c.getValues(br)
		bm.forEachSetBit(func(idx int) bool {
			v := values[idx]
			return fr.matchTimestampString(v)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if fr.matchTimestampString(v) {
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
			return fr.matchTimestampValue(timestamp)
		})
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fr *filterDayRange) matchTimestampString(v string) bool {
	timestamp, ok := TryParseTimestampRFC3339Nano(v)
	if !ok {
		return false
	}
	return fr.matchTimestampValue(timestamp)
}

func (fr *filterDayRange) matchTimestampValue(timestamp int64) bool {
	dayOffset := fr.dayRangeOffset(timestamp)
	return dayOffset >= fr.start && dayOffset <= fr.end
}

func (fr *filterDayRange) dayRangeOffset(timestamp int64) int64 {
	timestamp -= fr.offset
	return timestamp % nsecsPerDay
}

func (fr *filterDayRange) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fr.start > fr.end {
		bm.resetBits()
		return
	}
	if fr.start == 0 && fr.end == nsecsPerDay-1 {
		return
	}

	timestamps := bs.getTimestamps()
	bm.forEachSetBit(func(idx int) bool {
		return fr.matchTimestampValue(timestamps[idx])
	})
}
