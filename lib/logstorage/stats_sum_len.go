package logstorage

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
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

func (ss *statsSumLen) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsSumLenProcessor()
}

type statsSumLenProcessor struct {
	sumLen uint64
}

func (ssp *statsSumLenProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	ss := sf.(*statsSumLen)
	fields := ss.fields
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

func (ssp *statsSumLenProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	ss := sf.(*statsSumLen)
	fields := ss.fields
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

func (ssp *statsSumLenProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsSumLenProcessor)
	ssp.sumLen += src.sumLen
}

func (ssp *statsSumLenProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return encoding.MarshalVarUint64(dst, ssp.sumLen)
}

func (ssp *statsSumLenProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	sumLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal sumLen")
	}
	src = src[n:]
	ssp.sumLen = sumLen

	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}

	return 0, nil
}

func (ssp *statsSumLenProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return strconv.AppendUint(dst, ssp.sumLen, 10)
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
