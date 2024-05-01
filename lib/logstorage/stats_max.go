package logstorage

import (
	"fmt"
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
			f := c.getMaxValue(br)
			if f > smp.max || math.IsNaN(smp.max) {
				smp.max = f
			}
		}
		return 0
	}

	// Find the maximum value across the requested columns
	for _, field := range smp.sm.fields {
		c := br.getColumnByName(field)
		f := c.getMaxValue(br)
		if f > smp.max || math.IsNaN(smp.max) {
			smp.max = f
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
		return 0
	}

	// Find the maximum value across the requested fields for the given row
	for _, field := range smp.sm.fields {
		c := br.getColumnByName(field)
		f := c.getFloatValueAtRow(rowIdx)
		if f > smp.max || math.IsNaN(smp.max) {
			smp.max = f
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
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'max' args: %w", err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("'max' must contain at least one arg")
	}
	sm := &statsMax{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return sm, nil
}
