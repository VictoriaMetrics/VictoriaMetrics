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

type statsRowMin struct {
	srcField string

	fieldFilters []string
}

func (sm *statsRowMin) String() string {
	s := "row_min(" + quoteTokenIfNeeded(sm.srcField)
	if !prefixfilter.MatchAll(sm.fieldFilters) {
		s += ", " + fieldNamesString(sm.fieldFilters)
	}
	s += ")"
	return s
}

func (sm *statsRowMin) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.fieldFilters)
	pf.AddAllowFilter(sm.srcField)
}

func (sm *statsRowMin) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsRowMinProcessor()
}

type statsRowMinProcessor struct {
	min string

	fields []Field
}

func (smp *statsRowMinProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sm := sf.(*statsRowMin)
	stateSizeIncrease := 0

	c := br.getColumnByName(sm.srcField)
	if c.isConst {
		v := c.valuesEncoded[0]
		stateSizeIncrease += smp.updateState(sm, v, br, 0)
		return stateSizeIncrease
	}
	if c.isTime {
		timestamp, ok := TryParseTimestampRFC3339Nano(smp.min)
		if !ok {
			timestamp = (1 << 63) - 1
		}
		minTimestamp := br.getMinTimestamp(timestamp)
		if minTimestamp >= timestamp {
			return stateSizeIncrease
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], minTimestamp)
		v := bytesutil.ToUnsafeString(bb.B)
		stateSizeIncrease += smp.updateState(sm, v, br, 0)
		bbPool.Put(bb)
		return stateSizeIncrease
	}

	needUpdateState := false
	switch c.valueType {
	case valueTypeString:
		needUpdateState = true
	case valueTypeDict:
		c.forEachDictValue(br, func(v string) {
			if !needUpdateState && smp.needUpdateStateString(v) {
				needUpdateState = true
			}
		})
	case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64:
		bb := bbPool.Get()
		bb.B = marshalUint64String(bb.B[:0], c.minValue)
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeInt64:
		bb := bbPool.Get()
		bb.B = marshalInt64String(bb.B[:0], int64(c.minValue))
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.minValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.minValue))
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.minValue))
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}

	if needUpdateState {
		values := c.getValues(br)
		for i, v := range values {
			stateSizeIncrease += smp.updateState(sm, v, br, i)
		}
	}

	return stateSizeIncrease
}

func (smp *statsRowMinProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sm := sf.(*statsRowMin)
	stateSizeIncrease := 0

	c := br.getColumnByName(sm.srcField)
	if c.isConst {
		v := c.valuesEncoded[0]
		stateSizeIncrease += smp.updateState(sm, v, br, rowIdx)
		return stateSizeIncrease
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], timestamps[rowIdx])
		v := bytesutil.ToUnsafeString(bb.B)
		stateSizeIncrease += smp.updateState(sm, v, br, rowIdx)
		bbPool.Put(bb)
		return stateSizeIncrease
	}

	v := c.getValueAtRow(br, rowIdx)
	stateSizeIncrease += smp.updateState(sm, v, br, rowIdx)

	return stateSizeIncrease
}

func (smp *statsRowMinProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsRowMinProcessor)
	if smp.needUpdateStateString(src.min) {
		smp.min = src.min
		smp.fields = src.fields
	}
}

func (smp *statsRowMinProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(smp.min))
	dst = marshalFields(dst, smp.fields)
	return dst
}

func (smp *statsRowMinProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	minValue, n := encoding.UnmarshalBytes(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal minValue")
	}
	src = src[n:]
	smp.min = string(minValue)

	fields, tail, err := unmarshalFields(nil, src)
	if err != nil {
		return 0, fmt.Errorf("cannot unmarshal fields: %w", err)
	}
	if len(tail) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail; len(tail)=%d", len(tail))
	}
	smp.fields = fields

	stateSize := len(smp.min) + fieldsStateSize(smp.fields)

	return stateSize, nil
}

func (smp *statsRowMinProcessor) needUpdateStateBytes(b []byte) bool {
	v := bytesutil.ToUnsafeString(b)
	return smp.needUpdateStateString(v)
}

func (smp *statsRowMinProcessor) needUpdateStateString(v string) bool {
	if v == "" {
		return false
	}
	return smp.min == "" || lessString(v, smp.min)
}

func (smp *statsRowMinProcessor) updateState(sm *statsRowMin, v string, br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0

	if !smp.needUpdateStateString(v) {
		// There is no need in updating state
		return stateSizeIncrease
	}

	stateSizeIncrease -= len(smp.min)
	stateSizeIncrease += len(v)
	smp.min = strings.Clone(v)

	fields := smp.fields
	for _, f := range fields {
		stateSizeIncrease -= len(f.Name) + len(f.Value)
	}

	clear(fields)
	fields = fields[:0]

	mc := getMatchingColumns(br, sm.fieldFilters)
	for _, c := range mc.cs {
		v := c.getValueAtRow(br, rowIdx)
		fields = append(fields, Field{
			Name:  strings.Clone(c.name),
			Value: strings.Clone(v),
		})
		stateSizeIncrease += len(c.name) + len(v)
	}
	putMatchingColumns(mc)

	smp.fields = fields

	return stateSizeIncrease
}

func (smp *statsRowMinProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return MarshalFieldsToJSON(dst, smp.fields)
}

func parseStatsRowMin(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "row_min")
	if err != nil {
		return nil, err
	}

	if len(fieldFilters) == 0 {
		return nil, fmt.Errorf("missing source field for 'row_min' func")
	}

	srcField := fieldFilters[0]
	if prefixfilter.IsWildcardFilter(srcField) {
		return nil, fmt.Errorf("the source field %q cannot be wildcard", srcField)
	}

	fieldFilters = fieldFilters[1:]
	if len(fieldFilters) == 0 {
		fieldFilters = []string{"*"}
	}

	sm := &statsRowMin{
		srcField:     srcField,
		fieldFilters: fieldFilters,
	}
	return sm, nil
}
