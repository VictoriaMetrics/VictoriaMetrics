package logstorage

import (
	"math"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsMax struct {
	fields []string
}

func (sm *statsMax) String() string {
	return "max(" + statsFuncFieldsToString(sm.fields) + ")"
}

func (sm *statsMax) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sm.fields)
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

	if len(sm.fields) == 0 {
		// Find the minimum value across all the columns
		for _, c := range br.getColumns() {
			smp.updateStateForColumn(br, c)
		}
	} else {
		// Find the minimum value across the requested columns
		for _, field := range sm.fields {
			c := br.getColumnByName(field)
			smp.updateStateForColumn(br, c)
		}
	}

	return len(smp.max) - maxLen
}

func (smp *statsMaxProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sm := sf.(*statsMax)
	maxLen := len(smp.max)

	if len(sm.fields) == 0 {
		// Find the minimum value across all the fields for the given row
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	} else {
		// Find the minimum value across the requested fields for the given row
		for _, field := range sm.fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	}

	return maxLen - len(smp.max)
}

func (smp *statsMaxProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsMaxProcessor)
	if src.hasItems {
		smp.updateStateString(src.max)
	}
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
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeInt64:
		bb := bbPool.Get()
		bb.B = marshalInt64String(bb.B[:0], int64(c.maxValue))
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.maxValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.maxValue))
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.maxValue))
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}
}

func (smp *statsMaxProcessor) updateStateBytes(b []byte) {
	v := bytesutil.ToUnsafeString(b)
	smp.updateStateString(v)
}

func (smp *statsMaxProcessor) updateStateString(v string) {
	if !smp.hasItems {
		smp.max = strings.Clone(v)
		smp.hasItems = true
		return
	}
	if lessString(smp.max, v) {
		smp.max = strings.Clone(v)
	}
}

func (smp *statsMaxProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return append(dst, smp.max...)
}

func parseStatsMax(lex *lexer) (*statsMax, error) {
	fields, err := parseStatsFuncFields(lex, "max")
	if err != nil {
		return nil, err
	}
	sm := &statsMax{
		fields: fields,
	}
	return sm, nil
}
