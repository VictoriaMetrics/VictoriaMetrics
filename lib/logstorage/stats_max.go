package logstorage

import (
	"slices"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsMax struct {
	fields       []string
	containsStar bool
}

func (sm *statsMax) String() string {
	return "max(" + fieldNamesString(sm.fields) + ")"
}

func (sm *statsMax) neededFields() []string {
	return sm.fields
}

func (sm *statsMax) newStatsProcessor() (statsProcessor, int) {
	smp := &statsMaxProcessor{
		sm: sm,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsMaxProcessor struct {
	sm *statsMax

	max    string
	hasMax bool
}

func (smp *statsMaxProcessor) updateStatsForAllRows(br *blockResult) int {
	maxLen := len(smp.max)

	if smp.sm.containsStar {
		// Find the minimum value across all the columns
		for _, c := range br.getColumns() {
			smp.updateStateForColumn(br, c)
		}
	} else {
		// Find the minimum value across the requested columns
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			smp.updateStateForColumn(br, c)
		}
	}

	return len(smp.max) - maxLen
}

func (smp *statsMaxProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	maxLen := len(smp.max)

	if smp.sm.containsStar {
		// Find the minimum value across all the fields for the given row
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			smp.updateState(v)
		}
	} else {
		// Find the minimum value across the requested fields for the given row
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			smp.updateState(v)
		}
	}

	return maxLen - len(smp.max)
}

func (smp *statsMaxProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMaxProcessor)
	if src.hasMax {
		smp.updateState(src.max)
	}
}

func (smp *statsMaxProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if c.isTime {
		// Special case for time column
		timestamps := br.timestamps
		if len(timestamps) == 0 {
			return
		}
		maxTimestamp := timestamps[len(timestamps)-1]
		for _, timestamp := range timestamps[:len(timestamps)-1] {
			if timestamp > maxTimestamp {
				maxTimestamp = timestamp
			}
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339Nano(bb.B[:0], maxTimestamp)
		v := bytesutil.ToUnsafeString(bb.B)
		smp.updateState(v)
		bbPool.Put(bb)

		return
	}
	if c.isConst {
		// Special case for const column
		v := c.encodedValues[0]
		smp.updateState(v)
		return
	}

	for _, v := range c.getValues(br) {
		smp.updateState(v)
	}
}

func (smp *statsMaxProcessor) updateState(v string) {
	if smp.hasMax && !lessString(smp.max, v) {
		return
	}
	smp.max = strings.Clone(v)
	smp.hasMax = true
}

func (smp *statsMaxProcessor) finalizeStats() string {
	if !smp.hasMax {
		return "NaN"
	}
	return smp.max
}

func parseStatsMax(lex *lexer) (*statsMax, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "max")
	if err != nil {
		return nil, err
	}
	sm := &statsMax{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return sm, nil
}
