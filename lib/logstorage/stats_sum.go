package logstorage

import (
	"math"
	"slices"
	"strconv"
	"unsafe"
)

type statsSum struct {
	fields       []string
	containsStar bool
}

func (ss *statsSum) String() string {
	return "sum(" + fieldNamesString(ss.fields) + ")"
}

func (ss *statsSum) neededFields() []string {
	return ss.fields
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
	if ssp.ss.containsStar {
		// Sum all the columns
		for _, c := range br.getColumns() {
			f, count := c.sumValues(br)
			if count > 0 {
				if math.IsNaN(ssp.sum) {
					ssp.sum = f
				} else {
					ssp.sum += f
				}
			}
		}
	} else {
		// Sum the requested columns
		for _, field := range ssp.ss.fields {
			c := br.getColumnByName(field)
			f, count := c.sumValues(br)
			if count > 0 {
				if math.IsNaN(ssp.sum) {
					ssp.sum = f
				} else {
					ssp.sum += f
				}
			}
		}
	}
	return 0
}

func (ssp *statsSumProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	if ssp.ss.containsStar {
		// Sum all the fields for the given row
		for _, c := range br.getColumns() {
			f := c.getFloatValueAtRow(rowIdx)
			if !math.IsNaN(f) {
				if math.IsNaN(ssp.sum) {
					ssp.sum = f
				} else {
					ssp.sum += f
				}
			}
		}
	} else {
		// Sum only the given fields for the given row
		for _, field := range ssp.ss.fields {
			c := br.getColumnByName(field)
			f := c.getFloatValueAtRow(rowIdx)
			if !math.IsNaN(f) {
				if math.IsNaN(ssp.sum) {
					ssp.sum = f
				} else {
					ssp.sum += f
				}
			}
		}
	}
	return 0
}

func (ssp *statsSumProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsSumProcessor)
	ssp.sum += src.sum
}

func (ssp *statsSumProcessor) finalizeStats() string {
	return strconv.FormatFloat(ssp.sum, 'f', -1, 64)
}

func parseStatsSum(lex *lexer) (*statsSum, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "sum")
	if err != nil {
		return nil, err
	}
	ss := &statsSum{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return ss, nil
}
