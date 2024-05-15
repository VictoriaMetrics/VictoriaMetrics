package logstorage

import (
	"slices"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type statsMin struct {
	fields       []string
	containsStar bool
}

func (sm *statsMin) String() string {
	return "min(" + fieldNamesString(sm.fields) + ")"
}

func (sm *statsMin) neededFields() []string {
	return sm.fields
}

func (sm *statsMin) newStatsProcessor() (statsProcessor, int) {
	smp := &statsMinProcessor{
		sm: sm,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsMinProcessor struct {
	sm *statsMin

	min    string
	hasMin bool
}

func (smp *statsMinProcessor) updateStatsForAllRows(br *blockResult) int {
	minLen := len(smp.min)

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

	return len(smp.min) - minLen
}

func (smp *statsMinProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	minLen := len(smp.min)

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

	return minLen - len(smp.min)
}

func (smp *statsMinProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMinProcessor)
	if src.hasMin {
		smp.updateState(src.min)
	}
}

func (smp *statsMinProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if c.isTime {
		// Special case for time column
		timestamps := br.timestamps
		if len(timestamps) == 0 {
			return
		}
		minTimestamp := timestamps[0]
		for _, timestamp := range timestamps[1:] {
			if timestamp < minTimestamp {
				minTimestamp = timestamp
			}
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339Nano(bb.B[:0], minTimestamp)
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

func (smp *statsMinProcessor) updateState(v string) {
	if smp.hasMin && !lessString(v, smp.min) {
		return
	}
	smp.min = strings.Clone(v)
	smp.hasMin = true
}

func (smp *statsMinProcessor) finalizeStats() string {
	if !smp.hasMin {
		return "NaN"
	}
	return smp.min
}

func parseStatsMin(lex *lexer) (*statsMin, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "min")
	if err != nil {
		return nil, err
	}
	sm := &statsMin{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return sm, nil
}
