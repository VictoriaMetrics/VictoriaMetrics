package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterIPv4Range matches the given ipv4 range [minValue..maxValue].
//
// Example LogsQL: `fieldName:ipv4_range(127.0.0.1, 127.0.0.255)`
type filterIPv4Range struct {
	fieldName string
	minValue  uint32
	maxValue  uint32
}

func (fr *filterIPv4Range) String() string {
	minValue := marshalIPv4String(nil, fr.minValue)
	maxValue := marshalIPv4String(nil, fr.maxValue)
	return fmt.Sprintf("%sipv4_range(%s, %s)", quoteFieldNameIfNeeded(fr.fieldName), minValue, maxValue)
}

func (fr *filterIPv4Range) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fr.fieldName)
}

func (fr *filterIPv4Range) applyToBlockResult(br *blockResult, bm *bitmap) {
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fr.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchIPv4Range(v, minValue, maxValue) {
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
			return matchIPv4Range(v, minValue, maxValue)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchIPv4Range(v, minValue, maxValue) {
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
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			ip := unmarshalIPv4(valuesEncoded[idx])
			return ip >= minValue && ip <= maxValue
		})
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fr *filterIPv4Range) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	csh := bs.getColumnsHeader()
	v := csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchIPv4Range(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByIPv4Range(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByIPv4Range(bs, ch, bm, minValue, maxValue)
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
		matchIPv4ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchValuesDictByIPv4Range(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue uint32) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchIPv4Range(v, minValue, maxValue) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByIPv4Range(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue uint32) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchIPv4Range(v, minValue, maxValue)
	})
}

func matchIPv4Range(s string, minValue, maxValue uint32) bool {
	n, ok := tryParseIPv4(s)
	if !ok {
		return false
	}
	return n >= minValue && n <= maxValue
}

func matchIPv4ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue uint32) {
	if ch.minValue > uint64(maxValue) || ch.maxValue < uint64(minValue) {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 4 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of IPv4: got %d; want 4", bs.partPath(), len(v))
		}
		n := unmarshalIPv4(v)
		return n >= minValue && n <= maxValue
	})
}
