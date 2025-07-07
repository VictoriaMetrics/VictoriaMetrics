package logstorage

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsSumLen struct {
	fieldFilters []string
}

func (ss *statsSumLen) String() string {
	return "sum_len(" + fieldNamesString(ss.fieldFilters) + ")"
}

func (ss *statsSumLen) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(ss.fieldFilters)
}

func (ss *statsSumLen) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsSumLenProcessor()
}

type statsSumLenProcessor struct {
	sumLen uint64
}

func (ssp *statsSumLenProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	ss := sf.(*statsSumLen)

	mc := getMatchingColumns(br, ss.fieldFilters)
	for _, c := range mc.cs {
		ssp.sumLen += c.sumLenValues(br)
	}
	putMatchingColumns(mc)

	return 0
}

func (ssp *statsSumLenProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	ss := sf.(*statsSumLen)

	mc := getMatchingColumns(br, ss.fieldFilters)
	for _, c := range mc.cs {
		v := c.getValueAtRow(br, rowIdx)
		ssp.sumLen += uint64(len(v))
	}
	putMatchingColumns(mc)

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
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "sum_len")
	if err != nil {
		return nil, err
	}
	ss := &statsSumLen{
		fieldFilters: fieldFilters,
	}
	return ss, nil
}
