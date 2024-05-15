package logstorage

import (
	"slices"
	"strconv"
	"unsafe"
)

type statsAvg struct {
	fields       []string
	containsStar bool
}

func (sa *statsAvg) String() string {
	return "avg(" + fieldNamesString(sa.fields) + ")"
}

func (sa *statsAvg) neededFields() []string {
	return sa.fields
}

func (sa *statsAvg) newStatsProcessor() (statsProcessor, int) {
	sap := &statsAvgProcessor{
		sa: sa,
	}
	return sap, int(unsafe.Sizeof(*sap))
}

type statsAvgProcessor struct {
	sa *statsAvg

	sum   float64
	count uint64
}

func (sap *statsAvgProcessor) updateStatsForAllRows(br *blockResult) int {
	if sap.sa.containsStar {
		// Scan all the columns
		for _, c := range br.getColumns() {
			f, count := c.sumValues(br)
			sap.sum += f
			sap.count += uint64(count)
		}
	} else {
		// Scan the requested columns
		for _, field := range sap.sa.fields {
			c := br.getColumnByName(field)
			f, count := c.sumValues(br)
			sap.sum += f
			sap.count += uint64(count)
		}
	}
	return 0
}

func (sap *statsAvgProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if sap.sa.containsStar {
		// Scan all the fields for the given row
		for _, c := range br.getColumns() {
			f, ok := c.getFloatValueAtRow(rowIdx)
			if ok {
				sap.sum += f
				sap.count++
			}
		}
	} else {
		// Scan only the given fields for the given row
		for _, field := range sap.sa.fields {
			c := br.getColumnByName(field)
			f, ok := c.getFloatValueAtRow(rowIdx)
			if ok {
				sap.sum += f
				sap.count++
			}
		}
	}
	return 0
}

func (sap *statsAvgProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsAvgProcessor)
	sap.sum += src.sum
	sap.count += src.count
}

func (sap *statsAvgProcessor) finalizeStats() string {
	avg := sap.sum / float64(sap.count)
	return strconv.FormatFloat(avg, 'f', -1, 64)
}

func parseStatsAvg(lex *lexer) (*statsAvg, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "avg")
	if err != nil {
		return nil, err
	}
	sa := &statsAvg{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return sa, nil
}
