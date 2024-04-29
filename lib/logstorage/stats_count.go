package logstorage

import (
	"fmt"
	"slices"
	"strconv"
	"unsafe"
)

type statsCount struct {
	fields       []string
	containsStar bool

	resultName string
}

func (sc *statsCount) String() string {
	return "count(" + fieldNamesString(sc.fields) + ") as " + quoteTokenIfNeeded(sc.resultName)
}

func (sc *statsCount) neededFields() []string {
	return getFieldsIgnoreStar(sc.fields)
}

func (sc *statsCount) newStatsProcessor() (statsProcessor, int) {
	scp := &statsCountProcessor{
		sc: sc,
	}
	return scp, int(unsafe.Sizeof(*scp))
}

type statsCountProcessor struct {
	sc *statsCount

	rowsCount uint64
}

func (scp *statsCountProcessor) updateStatsForAllRows(timestamps []int64, columns []BlockColumn) int {
	fields := scp.sc.fields
	if len(fields) == 0 || scp.sc.containsStar {
		// Fast path - count all the columns.
		scp.rowsCount += uint64(len(timestamps))
		return 0
	}

	// Slow path - count rows containing at least a single non-empty value for the fields enumerated inside count().
	bm := getBitmap(len(timestamps))
	defer putBitmap(bm)

	bm.setBits()
	for _, f := range fields {
		if idx := getBlockColumnIndex(columns, f); idx >= 0 {
			values := columns[idx].Values
			bm.forEachSetBit(func(i int) bool {
				return values[i] == ""
			})
		}
	}

	emptyValues := 0
	bm.forEachSetBit(func(i int) bool {
		emptyValues++
		return true
	})

	scp.rowsCount += uint64(len(timestamps) - emptyValues)
	return 0
}

func (scp *statsCountProcessor) updateStatsForRow(_ []int64, columns []BlockColumn, rowIdx int) int {
	fields := scp.sc.fields
	if len(fields) == 0 || scp.sc.containsStar {
		// Fast path - count the given column
		scp.rowsCount++
		return 0
	}

	// Slow path - count the row at rowIdx if at least a single field enumerated inside count() is non-empty
	for _, f := range fields {
		if idx := getBlockColumnIndex(columns, f); idx >= 0 && columns[idx].Values[rowIdx] != "" {
			scp.rowsCount++
			return 0
		}
	}
	return 0
}

func (scp *statsCountProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsCountProcessor)
	scp.rowsCount += src.rowsCount
}

func (scp *statsCountProcessor) finalizeStats() (string, string) {
	value := strconv.FormatUint(scp.rowsCount, 10)
	return scp.sc.resultName, value
}

func parseStatsCount(lex *lexer) (*statsCount, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'count' args: %w", err)
	}
	resultName, err := parseResultName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}
	sc := &statsCount{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
		resultName:   resultName,
	}
	return sc, nil
}

func getFieldsIgnoreStar(fields []string) []string {
	var result []string
	for _, f := range fields {
		if f != "*" {
			result = append(result, f)
		}
	}
	return result
}
