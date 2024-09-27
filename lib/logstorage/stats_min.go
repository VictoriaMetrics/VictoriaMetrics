package logstorage

import (
	"math"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsMin struct {
	fields []string
}

func (sm *statsMin) String() string {
	return "min(" + statsFuncFieldsToString(sm.fields) + ")"
}

func (sm *statsMin) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, sm.fields)
}

func (sm *statsMin) newStatsProcessor() (statsProcessor, int) {
	smp := &statsMinProcessor{
		sm: sm,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsMinProcessor struct {
	sm *statsMin

	min string
}

func (smp *statsMinProcessor) updateStatsForAllRows(br *blockResult) int {
	minLen := len(smp.min)

	fields := smp.sm.fields
	if len(fields) == 0 {
		// Find the minimum value across all the columns
		for _, c := range br.getColumns() {
			smp.updateStateForColumn(br, c)
		}
	} else {
		// Find the minimum value across the requested columns
		for _, field := range fields {
			c := br.getColumnByName(field)
			smp.updateStateForColumn(br, c)
		}
	}

	return len(smp.min) - minLen
}

func (smp *statsMinProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	minLen := len(smp.min)

	fields := smp.sm.fields
	if len(fields) == 0 {
		// Find the minimum value across all the fields for the given row
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	} else {
		// Find the minimum value across the requested fields for the given row
		for _, field := range fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	}

	return minLen - len(smp.min)
}

func (smp *statsMinProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMinProcessor)
	smp.updateStateString(src.min)
}

func (smp *statsMinProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if br.rowsLen == 0 {
		return
	}

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
		for _, v := range c.dictValues {
			smp.updateStateString(v)
		}
	case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64:
		bb := bbPool.Get()
		bb.B = marshalUint64String(bb.B[:0], c.minValue)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.minValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.minValue))
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.minValue))
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}
}

func (smp *statsMinProcessor) updateStateBytes(b []byte) {
	v := bytesutil.ToUnsafeString(b)
	smp.updateStateString(v)
}

func (smp *statsMinProcessor) updateStateString(v string) {
	if v == "" {
		// Skip empty strings
		return
	}
	if smp.min != "" && !lessString(v, smp.min) {
		return
	}
	smp.min = strings.Clone(v)
}

func (smp *statsMinProcessor) finalizeStats() string {
	return smp.min
}

func parseStatsMin(lex *lexer) (*statsMin, error) {
	fields, err := parseStatsFuncFields(lex, "min")
	if err != nil {
		return nil, err
	}
	sm := &statsMin{
		fields: fields,
	}
	return sm, nil
}
