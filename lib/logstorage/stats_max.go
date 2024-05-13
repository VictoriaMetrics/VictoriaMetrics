package logstorage

import (
	"math"
	"slices"
	"strconv"
	"unsafe"
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
		sm:  sm,
		max: nan,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsMaxProcessor struct {
	sm *statsMax

	max float64
}

func (smp *statsMaxProcessor) updateStatsForAllRows(br *blockResult) int {
	if smp.sm.containsStar {
		// Find the maximum value across all the columns
		for _, c := range br.getColumns() {
			f := c.getMaxValue()
			if f > smp.max || math.IsNaN(smp.max) {
				smp.max = f
			}
		}
	} else {
		// Find the maximum value across the requested columns
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			f := c.getMaxValue()
			if f > smp.max || math.IsNaN(smp.max) {
				smp.max = f
			}
		}
	}
	return 0
}

func (smp *statsMaxProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if smp.sm.containsStar {
		// Find the maximum value across all the fields for the given row
		for _, c := range br.getColumns() {
			f := c.getFloatValueAtRow(rowIdx)
			if f > smp.max || math.IsNaN(smp.max) {
				smp.max = f
			}
		}
	} else {
		// Find the maximum value across the requested fields for the given row
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			f := c.getFloatValueAtRow(rowIdx)
			if f > smp.max || math.IsNaN(smp.max) {
				smp.max = f
			}
		}
	}
	return 0
}

func (smp *statsMaxProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMaxProcessor)
	if src.max > smp.max {
		smp.max = src.max
	}
}

func (smp *statsMaxProcessor) finalizeStats() string {
	return strconv.FormatFloat(smp.max, 'g', -1, 64)
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
