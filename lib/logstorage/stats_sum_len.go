package logstorage

import (
	"strconv"
	"unsafe"
)

type statsSumLen struct {
	fields []string
}

func (ss *statsSumLen) String() string {
	return "sum_len(" + statsFuncFieldsToString(ss.fields) + ")"
}

func (ss *statsSumLen) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, ss.fields)
}

func (ss *statsSumLen) newStatsProcessor() (statsProcessor, int) {
	ssp := &statsSumLenProcessor{
		ss:     ss,
		sumLen: 0,
	}
	return ssp, int(unsafe.Sizeof(*ssp))
}

type statsSumLenProcessor struct {
	ss *statsSumLen

	sumLen uint64
}

func (ssp *statsSumLenProcessor) updateStatsForAllRows(br *blockResult) int {
	fields := ssp.ss.fields
	if len(fields) == 0 {
		// Sum all the columns
		for _, c := range br.getColumns() {
			ssp.sumLen += c.sumLenValues(br)
		}
	} else {
		// Sum the requested columns
		for _, field := range fields {
			c := br.getColumnByName(field)
			ssp.sumLen += c.sumLenValues(br)
		}
	}
	return 0
}

func (ssp *statsSumLenProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	fields := ssp.ss.fields
	if len(fields) == 0 {
		// Sum all the fields for the given row
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			ssp.sumLen += uint64(len(v))
		}
	} else {
		// Sum only the given fields for the given row
		for _, field := range fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			ssp.sumLen += uint64(len(v))
		}
	}
	return 0
}

func (ssp *statsSumLenProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsSumLenProcessor)
	ssp.sumLen += src.sumLen
}

func (ssp *statsSumLenProcessor) finalizeStats() string {
	return strconv.FormatUint(ssp.sumLen, 10)
}

func parseStatsSumLen(lex *lexer) (*statsSumLen, error) {
	fields, err := parseStatsFuncFields(lex, "sum_len")
	if err != nil {
		return nil, err
	}
	ss := &statsSumLen{
		fields: fields,
	}
	return ss, nil
}
