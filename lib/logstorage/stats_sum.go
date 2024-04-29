package logstorage

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"unsafe"
)

type statsSum struct {
	fields       []string
	containsStar bool
	resultName   string
}

func (ss *statsSum) String() string {
	return "sum(" + fieldNamesString(ss.fields) + ") as " + quoteTokenIfNeeded(ss.resultName)
}

func (ss *statsSum) neededFields() []string {
	return ss.fields
}

func (ss *statsSum) newStatsProcessor() (statsProcessor, int) {
	ssp := &statsSumProcessor{
		ss: ss,
	}
	return ssp, int(unsafe.Sizeof(*ssp))
}

type statsSumProcessor struct {
	ss *statsSum

	sum float64
}

func (ssp *statsSumProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int {
	if ssp.ss.containsStar {
		// Sum all the columns
		for _, c := range columns {
			ssp.sum += sumValues(c.Values)
		}
		return 0
	}

	// Sum the requested columns
	for _, field := range ssp.ss.fields {
		if idx := getBlockColumnIndex(columns, field); idx >= 0 {
			ssp.sum += sumValues(columns[idx].Values)
		}
	}
	return 0
}

func sumValues(values []string) float64 {
	sum := float64(0)
	f := float64(0)
	for i, v := range values {
		if i == 0 || values[i-1] != v {
			f, _ = tryParseFloat64(v)
			if math.IsNaN(f) {
				// Ignore NaN values, since this is the expected behaviour by most users.
				f = 0
			}
		}
		sum += f
	}
	return sum
}

func (ssp *statsSumProcessor) updateStatsForRow(_ []int64, columns []BlockColumn, rowIdx int) int {
	if ssp.ss.containsStar {
		// Sum all the fields for the given row
		for _, c := range columns {
			v := c.Values[rowIdx]
			f, _ := tryParseFloat64(v)
			if !math.IsNaN(f) {
				ssp.sum += f
			}
		}
		return 0
	}

	// Sum only the given fields for the given row
	for _, field := range ssp.ss.fields {
		if idx := getBlockColumnIndex(columns, field); idx >= 0 {
			v := columns[idx].Values[rowIdx]
			f, _ := tryParseFloat64(v)
			if !math.IsNaN(f) {
				ssp.sum += f
			}
		}
	}
	return 0
}

func (ssp *statsSumProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsSumProcessor)
	ssp.sum += src.sum
}

func (ssp *statsSumProcessor) finalizeStats() (string, string) {
	value := strconv.FormatFloat(ssp.sum, 'g', -1, 64)
	return ssp.ss.resultName, value
}

func parseStatsSum(lex *lexer) (*statsSum, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'sum' args: %w", err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("'sum' must contain at least one arg")
	}
	resultName, err := parseResultName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}
	ss := &statsSum{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
		resultName:   resultName,
	}
	return ss, nil
}
