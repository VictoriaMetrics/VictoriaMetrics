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

type statsMax struct {
	fieldFilters []string
}

func (sm *statsMax) String() string {
	return "max(" + fieldNamesString(sm.fieldFilters) + ")"
}

func (sm *statsMax) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.fieldFilters)
}

func (sm *statsMax) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsMaxProcessor()
}

type statsMaxProcessor struct {
	max      string
	hasItems bool
}

func (smp *statsMaxProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sm := sf.(*statsMax)

	maxLen := len(smp.max)

	mc := getMatchingColumns(br, sm.fieldFilters)
	for _, c := range mc.cs {
		smp.updateStateForColumn(br, c)
	}
	putMatchingColumns(mc)

	return len(smp.max) - maxLen
}

func (smp *statsMaxProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sm := sf.(*statsMax)

	maxLen := len(smp.max)

	mc := getMatchingColumns(br, sm.fieldFilters)
	for _, c := range mc.cs {
		v := c.getValueAtRow(br, rowIdx)
		smp.updateStateString(v)
	}
	putMatchingColumns(mc)

	return maxLen - len(smp.max)
}

func (smp *statsMaxProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsMaxProcessor)
	if src.hasItems {
		smp.updateStateString(src.max)
	}
}

func (smp *statsMaxProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	if !smp.hasItems {
		dst = append(dst, 0)
		return dst
	}

	dst = append(dst, 1)
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(smp.max))
	return dst
}

func (smp *statsMaxProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) == 0 {
		return 0, fmt.Errorf("missing `hasItems`")
	}
	smp.hasItems = (src[0] == 1)
	src = src[1:]

	if smp.hasItems {
		maxValue, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal max value")
		}
		smp.max = string(maxValue)
		src = src[n:]
	} else {
		smp.max = ""
	}

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected tail left after decoding max value; len(tail)=%d", len(src))
	}

	return len(smp.max), nil
}

func (smp *statsMaxProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if c.isTime {
		timestamp, ok := TryParseTimestampRFC3339Nano(smp.max)
		if !ok {
			timestamp = -1 << 63
		}
		maxTimestamp := br.getMaxTimestamp(timestamp)
		if maxTimestamp <= timestamp {
			return
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], maxTimestamp)
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
		bb.B = marshalUint64String(bb.B[:0], c.maxValue)
		smp.updateStateWithUpperBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeInt64:
		bb := bbPool.Get()
		bb.B = marshalInt64String(bb.B[:0], int64(c.maxValue))
		smp.updateStateWithUpperBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.maxValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		smp.updateStateWithUpperBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.maxValue))
		smp.updateStateWithUpperBound(br, c, bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.maxValue))
		smp.updateStateWithUpperBound(br, c, bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}
}

func (smp *statsMaxProcessor) updateStateWithUpperBound(br *blockResult, c *blockResultColumn, upperBound []byte) {
	upperBoundStr := bytesutil.ToUnsafeString(upperBound)
	if !smp.needsUpdateState(upperBoundStr) {
		return
	}
	if br.isFull() {
		smp.setState(upperBoundStr)
	} else {
		for _, v := range c.getValues(br) {
			smp.updateStateString(v)
		}
	}
}

func (smp *statsMaxProcessor) updateStateBytes(b []byte) {
	v := bytesutil.ToUnsafeString(b)
	smp.updateStateString(v)
}

func (smp *statsMaxProcessor) updateStateString(v string) {
	if smp.needsUpdateState(v) {
		smp.setState(v)
	}
}

func (smp *statsMaxProcessor) setState(v string) {
	smp.max = strings.Clone(v)
	if !smp.hasItems {
		smp.hasItems = true
	}
}

func (smp *statsMaxProcessor) needsUpdateState(v string) bool {
	return !smp.hasItems || lessString(smp.max, v)
}

func (smp *statsMaxProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return append(dst, smp.max...)
}

func parseStatsMax(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "max")
	if err != nil {
		return nil, err
	}
	sm := &statsMax{
		fieldFilters: fieldFilters,
	}
	return sm, nil
}
