package logstorage

import (
	"fmt"
	"math"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsSum struct {
	fieldFilters []string
}

func (ss *statsSum) String() string {
	return "sum(" + fieldNamesString(ss.fieldFilters) + ")"
}

func (ss *statsSum) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(ss.fieldFilters)
}

func (ss *statsSum) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	ssp := a.newStatsSumProcessor()
	ssp.sum = nan
	return ssp
}

type statsSumProcessor struct {
	sum float64
}

func (ssp *statsSumProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	ss := sf.(*statsSum)

	mc := getMatchingColumns(br, ss.fieldFilters)
	for _, c := range mc.cs {
		ssp.updateStateForColumn(br, c)
	}
	putMatchingColumns(mc)

	return 0
}

func (ssp *statsSumProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	ss := sf.(*statsSum)

	mc := getMatchingColumns(br, ss.fieldFilters)
	for _, c := range mc.cs {
		f, ok := c.getFloatValueAtRow(br, rowIdx)
		if ok {
			ssp.updateState(f)
		}
	}
	putMatchingColumns(mc)

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

func (ssp *statsSumProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsSumProcessor)
	if !math.IsNaN(src.sum) {
		ssp.updateState(src.sum)
	}
}

func (ssp *statsSumProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	return marshalFloat64(dst, ssp.sum)
}

func (ssp *statsSumProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) != 8 {
		return 0, fmt.Errorf("unexpected state length; got %d bytes; want 8 bytes", len(src))
	}
	ssp.sum = unmarshalFloat64(bytesutil.ToUnsafeString(src))
	return 0, nil
}

func (ssp *statsSumProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	return strconv.AppendFloat(dst, ssp.sum, 'f', -1, 64)
}

func parseStatsSum(lex *lexer) (*statsSum, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "sum")
	if err != nil {
		return nil, err
	}
	ss := &statsSum{
		fieldFilters: fieldFilters,
	}
	return ss, nil
}
