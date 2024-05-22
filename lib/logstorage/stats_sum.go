package logstorage

import (
	"math"
	"strconv"
	"unsafe"
)

type statsSum struct {
	fields []string
}

func (ss *statsSum) String() string {
	return "sum(" + statsFuncFieldsToString(ss.fields) + ")"
}

func (ss *statsSum) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, ss.fields)
}

func (ss *statsSum) newStatsProcessor() (statsProcessor, int) {
	ssp := &statsSumProcessor{
		ss:  ss,
		sum: nan,
	}
	return ssp, int(unsafe.Sizeof(*ssp))
}

type statsSumProcessor struct {
	ss *statsSum

	sum float64
}

func (ssp *statsSumProcessor) updateStatsForAllRows(br *blockResult) int {
	fields := ssp.ss.fields
	if len(fields) == 0 {
		// Sum all the columns
		for _, c := range br.getColumns() {
			ssp.updateStateForColumn(br, c)
		}
	} else {
		// Sum the requested columns
		for _, field := range fields {
			c := br.getColumnByName(field)
			ssp.updateStateForColumn(br, c)
		}
	}
	return 0
}

func (ssp *statsSumProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	fields := ssp.ss.fields
	if len(fields) == 0 {
		// Sum all the fields for the given row
		for _, c := range br.getColumns() {
			f, ok := c.getFloatValueAtRow(br, rowIdx)
			if ok {
				ssp.updateState(f)
			}
		}
	} else {
		// Sum only the given fields for the given row
		for _, field := range fields {
			c := br.getColumnByName(field)
			f, ok := c.getFloatValueAtRow(br, rowIdx)
			if ok {
				ssp.updateState(f)
			}
		}
	}
	return 0
}

func (ssp *statsSumProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	f, count := c.sumValues(br)
	if count > 0 {
		ssp.updateState(f)
	}
}

func (ssp *statsSumProcessor) updateState(f float64) {
	if math.IsNaN(ssp.sum) {
		ssp.sum = f
	} else {
		ssp.sum += f
	}
}

func (ssp *statsSumProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsSumProcessor)
	if !math.IsNaN(src.sum) {
		ssp.updateState(src.sum)
	}
}

func (ssp *statsSumProcessor) finalizeStats() string {
	return strconv.FormatFloat(ssp.sum, 'f', -1, 64)
}

func parseStatsSum(lex *lexer) (*statsSum, error) {
	fields, err := parseStatsFuncFields(lex, "sum")
	if err != nil {
		return nil, err
	}
	ss := &statsSum{
		fields: fields,
	}
	return ss, nil
}
