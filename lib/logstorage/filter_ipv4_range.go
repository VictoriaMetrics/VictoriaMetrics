package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
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
	minValue := string(encoding.MarshalUint32(nil, fr.minValue))
	maxValue := string(encoding.MarshalUint32(nil, fr.maxValue))
	return fmt.Sprintf("%sipv4_range(%s, %s)", quoteFieldNameIfNeeded(fr.fieldName), toIPv4String(nil, minValue), toIPv4String(nil, maxValue))
}

func (fr *filterIPv4Range) apply(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchIPv4Range(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
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
	for i, v := range ch.valuesDict.values {
		if matchIPv4Range(v, minValue, maxValue) {
			bb.B = append(bb.B, byte(i))
		}
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
		b := bytesutil.ToUnsafeBytes(v)
		n := encoding.UnmarshalUint32(b)
		return n >= minValue && n <= maxValue
	})
}
