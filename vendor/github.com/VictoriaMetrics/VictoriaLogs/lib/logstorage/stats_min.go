package logstorage

import (
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsMin struct {
	fieldFilters []string
}

func (sm *statsMin) String() string {
	return "min(" + fieldNamesString(sm.fieldFilters) + ")"
}

func (sm *statsMin) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.fieldFilters)
}

func (sm *statsMin) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsMinProcessor()
}

type statsMinProcessor struct {
	min      string
	hasItems bool
}

func (smp *statsMinProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sm := sf.(*statsMin)

	minLen := len(smp.min)

	mc := getMatchingColumns(br, sm.fieldFilters)
	for _, c := range mc.cs {
		smp.updateStateForColumn(br, c)
	}
	putMatchingColumns(mc)

	return len(smp.min) - minLen
}

func (smp *statsMinProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sm := sf.(*statsMin)

	minLen := len(smp.min)

	mc := getMatchingColumns(br, sm.fieldFilters)
	for _, c := range mc.cs {
		v := c.getValueAtRow(br, rowIdx)
		smp.updateStateString(v)
	}
	putMatchingColumns(mc)

	return minLen - len(smp.min)
}

func (smp *statsMinProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsMinProcessor)
	if src.hasItems {
		smp.updateStateString(src.min)
	}
}

func (smp *statsMinProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	if !smp.hasItems {
		dst = append(dst, 0)
		return dst
	}

	dst = append(dst, 1)
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(smp.min))
	return dst
}

func (smp *statsMinProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) == 0 {
		return 0, fmt.Errorf("missing `hasItems`")
	}
	smp.hasItems = (src[0] == 1)
	src = src[1:]

	if smp.hasItems {
		minValue, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal min value")
		}
		smp.min = string(minValue)
		src = src[n:]
	} else {
		smp.min = ""
	}

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left after decoding min value; len(tail)=%d", len(src))
	}

	return len(smp.min), nil
}

func (smp *statsMinProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if c.isTime {
		timestamp, ok := TryParseTimestampRFC3339Nano(smp.min)
		if !ok {
			timestamp = (1 << 63) - 1
		}
		minTimestamp := br.getMinTimestamp(timestamp)
		if minTimestamp >= timestamp {
			return
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], minTimestamp)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)

		return
	}
	if c.isConst {
		// Special case for const column
		v := c.valuesEncoded[0]
		smp.updateStateString(v)
		return
	}

	switch c.valueType {
	case valueTypeString:
		for _, v := range c.getValuesEncoded(br) {
			smp.updateStateString(v)
		}
	case valueTypeDict:
		c.forEachDictValue(br, func(v string) {
			smp.updateStateString(v)
		})
	case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64:
		bb := bbPool.Get()
		bb.B = marshalUint64String(bb.B[:0], c.minValue)
		smp.updateStateWithLowerBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeInt64:
		bb := bbPool.Get()
		bb.B = marshalInt64String(bb.B[:0], int64(c.minValue))
		smp.updateStateWithLowerBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.minValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		smp.updateStateWithLowerBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.minValue))
		smp.updateStateWithLowerBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.minValue))
		smp.updateStateWithLowerBound(br, c, bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}
}

func (smp *statsMinProcessor) updateStateWithLowerBound(br *blockResult, c *blockResultColumn, lowerBound []byte) {
	lowerBoundStr := bytesutil.ToUnsafeString(lowerBound)
	if !smp.needsUpdateState(lowerBoundStr) {
		return
	}
	if br.isFull() {
		smp.setState(lowerBoundStr)
	} else {
		for _, v := range c.getValues(br) {
			smp.updateStateString(v)
		}
	}
}

func (smp *statsMinProcessor) updateStateBytes(b []byte) {
	v := bytesutil.ToUnsafeString(b)
	smp.updateStateString(v)
}

func (smp *statsMinProcessor) updateStateString(v string) {
	if smp.needsUpdateState(v) {
		smp.setState(v)
	}
}

func (smp *statsMinProcessor) setState(v string) {
	smp.min = strings.Clone(v)
	if !smp.hasItems {
		smp.hasItems = true
	}
}

func (smp *statsMinProcessor) needsUpdateState(v string) bool {
	return !smp.hasItems || lessString(v, smp.min)
}

func (smp *statsMinProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return append(dst, smp.min...)
}

func parseStatsMin(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "min")
	if err != nil {
		return nil, err
	}
	sm := &statsMin{
		fieldFilters: fieldFilters,
	}
	return sm, nil
}
