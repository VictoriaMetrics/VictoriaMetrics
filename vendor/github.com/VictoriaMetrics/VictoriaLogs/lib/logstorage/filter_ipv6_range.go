package logstorage

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterIPv6Range matches the given ipv6 range [minValue..maxValue].
//
// Example LogsQL: `fieldName:ipv6_range(::1, ::2)`
type filterIPv6Range struct {
	fieldName string
	minValue  [16]byte
	maxValue  [16]byte

	minMaxIPv4ValuesOnce sync.Once
	minIPv4Value         uint32
	maxIPv4Value         uint32
	isIPv4               bool
}

func (fr *filterIPv6Range) String() string {
	minValue := netip.AddrFrom16(fr.minValue).String()
	maxValue := netip.AddrFrom16(fr.maxValue).String()
	return fmt.Sprintf("%sipv6_range(%s, %s)", quoteFieldNameIfNeeded(fr.fieldName), minValue, maxValue)
}

func (fr *filterIPv6Range) getMinMaxIPv4Values() (uint32, uint32, bool) {
	fr.minMaxIPv4ValuesOnce.Do(fr.initMinMaxIPv4Values)
	return fr.minIPv4Value, fr.maxIPv4Value, fr.isIPv4
}

func (fr *filterIPv6Range) initMinMaxIPv4Values() {
	minValue6 := fr.minValue
	if ipv6Less(minValue6, minIPv6ForIPv4Value) {
		minValue6 = minIPv6ForIPv4Value
	}
	minValue4, okMin := getIPv4ValueFrom16(minValue6)

	maxValue6 := fr.maxValue
	if ipv6Less(maxIPv6ForIPv4Value, maxValue6) {
		maxValue6 = maxIPv6ForIPv4Value
	}
	maxValue4, okMax := getIPv4ValueFrom16(maxValue6)

	if okMin && okMax {
		fr.minIPv4Value = minValue4
		fr.maxIPv4Value = maxValue4
		fr.isIPv4 = true
	}
}

var (
	minIPv6ForIPv4Value = [16]byte{10: 255, 11: 255}
	maxIPv6ForIPv4Value = [16]byte{10: 255, 11: 255, 12: 255, 13: 255, 14: 255, 15: 255}
)

func getIPv4ValueFrom16(a [16]byte) (uint32, bool) {
	addr := netip.AddrFrom16(a).Unmap()
	if !addr.Is4() {
		return 0, false
	}
	ip4 := addr.As4()
	return encoding.UnmarshalUint32(ip4[:]), true
}

func (fr *filterIPv6Range) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fr.fieldName)
}

func (fr *filterIPv6Range) matchRow(fields []Field) bool {
	v := getFieldValueByName(fields, fr.fieldName)
	return matchIPv6Range(v, fr.minValue, fr.maxValue)
}

func (fr *filterIPv6Range) applyToBlockResult(br *blockResult, bm *bitmap) {
	minValue := fr.minValue
	maxValue := fr.maxValue

	if ipv6Less(maxValue, minValue) {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fr.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchIPv6Range(v, minValue, maxValue) {
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
			return matchIPv6Range(v, minValue, maxValue)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchIPv6Range(v, minValue, maxValue) {
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
		minValue4, maxValue4, ok := fr.getMinMaxIPv4Values()
		if !ok {
			bm.resetBits()
		} else {
			valuesEncoded := c.getValuesEncoded(br)
			bm.forEachSetBit(func(idx int) bool {
				ip := unmarshalIPv4(valuesEncoded[idx])
				return ip >= minValue4 && ip <= maxValue4
			})
		}
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fr *filterIPv6Range) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minValue := fr.minValue
	maxValue := fr.maxValue

	if ipv6Less(maxValue, minValue) {
		bm.resetBits()
		return
	}

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchIPv6Range(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByIPv6Range(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByIPv6Range(bs, ch, bm, minValue, maxValue)
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
		minValue4, maxValue4, ok := fr.getMinMaxIPv4Values()
		if !ok {
			bm.resetBits()
		} else {
			matchIPv4ByRange(bs, ch, bm, minValue4, maxValue4)
		}
	case valueTypeTimestampISO8601:
		bm.resetBits()
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchValuesDictByIPv6Range(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue [16]byte) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchIPv6Range(v, minValue, maxValue) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByIPv6Range(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue [16]byte) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchIPv6Range(v, minValue, maxValue)
	})
}

func matchIPv6Range(s string, minValue, maxValue [16]byte) bool {
	ip, ok := tryParseIPv6(s)
	if !ok {
		return false
	}
	if ipv6Less(ip, minValue) || ipv6Less(maxValue, ip) {
		return false
	}
	return true
}

func ipv6Less(a, b [16]byte) bool {
	for i := range 16 {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return false
}
