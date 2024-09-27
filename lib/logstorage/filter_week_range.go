package logstorage

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterWeekRange filters by week range.
//
// It is expressed as `_time:week_range[start, end] offset d` in LogsQL.
type filterWeekRange struct {
	// startDay is the starting day of the week.
	startDay time.Weekday

	// endDay is the ending day of the week.
	endDay time.Weekday

	// offset is the offset, which must be applied to _time before applying [start, end] filter to it.
	offset int64

	// stringRepr is string representation of the filter.
	stringRepr string
}

func (fr *filterWeekRange) String() string {
	return "_time:week_range" + fr.stringRepr
}

func (fr *filterWeekRange) updateNeededFields(neededFields fieldsSet) {
	neededFields.add("_time")
}

func (fr *filterWeekRange) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fr.startDay > fr.endDay || fr.startDay > time.Saturday || fr.endDay < time.Monday {
		bm.resetBits()
		return
	}
	if fr.startDay <= time.Sunday && fr.endDay >= time.Saturday {
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

func (fr *filterWeekRange) matchTimestampString(v string) bool {
	timestamp, ok := TryParseTimestampRFC3339Nano(v)
	if !ok {
		return false
	}
	return fr.matchTimestampValue(timestamp)
}

func (fr *filterWeekRange) matchTimestampValue(timestamp int64) bool {
	d := fr.weekday(timestamp)
	return d >= fr.startDay && d <= fr.endDay
}

func (fr *filterWeekRange) weekday(timestamp int64) time.Weekday {
	timestamp -= fr.offset
	return time.Unix(0, timestamp).UTC().Weekday()
}

func (fr *filterWeekRange) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fr.startDay > fr.endDay {
		bm.resetBits()
		return
	}
	if fr.startDay <= time.Sunday && fr.endDay >= time.Saturday {
		return
	}

	timestamps := bs.getTimestamps()
	bm.forEachSetBit(func(idx int) bool {
		return fr.matchTimestampValue(timestamps[idx])
	})
}
