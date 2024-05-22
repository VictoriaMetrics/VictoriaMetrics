package logstorage

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsFieldsMax struct {
	srcField string

	fetchFields []string
}

func (sm *statsFieldsMax) String() string {
	s := "fields_max(" + quoteTokenIfNeeded(sm.srcField)
	if len(sm.fetchFields) > 0 {
		s += ", " + fieldNamesString(sm.fetchFields)
	}
	s += ")"
	return s
}

func (sm *statsFieldsMax) updateNeededFields(neededFields fieldsSet) {
	if len(sm.fetchFields) == 0 {
		neededFields.add("*")
	} else {
		neededFields.addFields(sm.fetchFields)
	}
	neededFields.add(sm.srcField)
}

func (sm *statsFieldsMax) newStatsProcessor() (statsProcessor, int) {
	smp := &statsFieldsMaxProcessor{
		sm: sm,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsFieldsMaxProcessor struct {
	sm *statsFieldsMax

	max string

	fields []Field
}

func (smp *statsFieldsMaxProcessor) updateStatsForAllRows(br *blockResult) int {
	stateSizeIncrease := 0

	c := br.getColumnByName(smp.sm.srcField)
	if c.isConst {
		v := c.valuesEncoded[0]
		stateSizeIncrease += smp.updateState(v, br, 0)
		return stateSizeIncrease
	}
	if c.isTime {
		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], br.timestamps[0])
		v := bytesutil.ToUnsafeString(bb.B)
		stateSizeIncrease += smp.updateState(v, br, 0)
		bbPool.Put(bb)
		return stateSizeIncrease
	}

	needUpdateState := false
	switch c.valueType {
	case valueTypeString:
		needUpdateState = true
	case valueTypeDict:
		for _, v := range c.dictValues {
			if smp.needUpdateStateString(v) {
				needUpdateState = true
				break
			}
		}
	case valueTypeUint8, valueTypeUint16, valueTypeUint32, valueTypeUint64:
		bb := bbPool.Get()
		bb.B = marshalUint64String(bb.B[:0], c.maxValue)
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		f := math.Float64frombits(c.maxValue)
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], f)
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], uint32(c.maxValue))
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], int64(c.maxValue))
		needUpdateState = smp.needUpdateStateBytes(bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}

	if needUpdateState {
		values := c.getValues(br)
		for i, v := range values {
			stateSizeIncrease += smp.updateState(v, br, i)
		}
	}

	return stateSizeIncrease
}

func (smp *statsFieldsMaxProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0

	c := br.getColumnByName(smp.sm.srcField)
	if c.isConst {
		v := c.valuesEncoded[0]
		stateSizeIncrease += smp.updateState(v, br, rowIdx)
		return stateSizeIncrease
	}
	if c.isTime {
		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], br.timestamps[rowIdx])
		v := bytesutil.ToUnsafeString(bb.B)
		stateSizeIncrease += smp.updateState(v, br, rowIdx)
		bbPool.Put(bb)
		return stateSizeIncrease
	}

	v := c.getValueAtRow(br, rowIdx)
	stateSizeIncrease += smp.updateState(v, br, rowIdx)

	return stateSizeIncrease
}

func (smp *statsFieldsMaxProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsFieldsMaxProcessor)
	if smp.needUpdateStateString(src.max) {
		smp.max = src.max
		smp.fields = src.fields
	}
}

func (smp *statsFieldsMaxProcessor) needUpdateStateBytes(b []byte) bool {
	v := bytesutil.ToUnsafeString(b)
	return smp.needUpdateStateString(v)
}

func (smp *statsFieldsMaxProcessor) needUpdateStateString(v string) bool {
	if v == "" {
		return false
	}
	return smp.max == "" || lessString(smp.max, v)
}

func (smp *statsFieldsMaxProcessor) updateState(v string, br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0

	if !smp.needUpdateStateString(v) {
		// There is no need in updating state
		return stateSizeIncrease
	}

	stateSizeIncrease -= len(smp.max)
	stateSizeIncrease += len(v)
	smp.max = strings.Clone(v)

	fields := smp.fields
	for _, f := range fields {
		stateSizeIncrease -= len(f.Name) + len(f.Value)
	}

	clear(fields)
	fields = fields[:0]
	fetchFields := smp.sm.fetchFields
	if len(fetchFields) == 0 {
		cs := br.getColumns()
		for _, c := range cs {
			v := c.getValueAtRow(br, rowIdx)
			fields = append(fields, Field{
				Name:  strings.Clone(c.name),
				Value: strings.Clone(v),
			})
			stateSizeIncrease += len(c.name) + len(v)
		}
	} else {
		for _, field := range fetchFields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			fields = append(fields, Field{
				Name:  strings.Clone(c.name),
				Value: strings.Clone(v),
			})
			stateSizeIncrease += len(c.name) + len(v)
		}
	}
	smp.fields = fields

	return stateSizeIncrease
}

func (smp *statsFieldsMaxProcessor) finalizeStats() string {
	bb := bbPool.Get()
	bb.B = marshalFieldsToJSON(bb.B, smp.fields)
	result := string(bb.B)
	bbPool.Put(bb)

	return result
}

func parseStatsFieldsMax(lex *lexer) (*statsFieldsMax, error) {
	if !lex.isKeyword("fields_max") {
		return nil, fmt.Errorf("unexpected func; got %q; want 'fields_max'", lex.token)
	}
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'fields_max' args: %w", err)
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("missing first arg for 'fields_max' func - source field")
	}

	srcField := fields[0]
	fetchFields := fields[1:]
	if slices.Contains(fetchFields, "*") {
		fetchFields = nil
	}

	sm := &statsFieldsMax{
		srcField:    srcField,
		fetchFields: fetchFields,
	}
	return sm, nil
}
