package logstorage

import (
	"fmt"
	"math"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsStddev struct {
	fieldFilters []string
}

func (ss *statsStddev) String() string {
	return "stddev(" + fieldNamesString(ss.fieldFilters) + ")"
}

func (ss *statsStddev) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(ss.fieldFilters)
}

func (ss *statsStddev) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsStddevProcessor()
}

// statsStddevProcessor contains the state needed for calculating stddev over a stream of values.
//
// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
type statsStddevProcessor struct {
	avg   float64
	q     float64
	count float64
}

func (ssp *statsStddevProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	ss := sf.(*statsStddev)

	mc := getMatchingColumns(br, ss.fieldFilters)
	for _, c := range mc.cs {
		for rowIdx := range br.rowsLen {
			f, ok := c.getFloatValueAtRow(br, rowIdx)
			if ok {
				ssp.updateState(f)
			}
		}
	}
	putMatchingColumns(mc)

	return 0
}

func (ssp *statsStddevProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	ss := sf.(*statsStddev)

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

func (ssp *statsStddevProcessor) updateState(f float64) {
	// See https://github.com/ClickHouse/ClickHouse/blob/016a9d5691d6486dd92bf4f4084abdb45baf8a45/src/AggregateFunctions/AggregateFunctionStatistics.cpp#L60
	delta := f - ssp.avg
	countNew := ssp.count + 1
	avgNew := ssp.avg + delta/countNew
	qNew := ssp.q + delta*(f-avgNew)

	ssp.avg = avgNew
	ssp.q = qNew
	ssp.count = countNew
}

func (ssp *statsStddevProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsStddevProcessor)

	// See https://github.com/ClickHouse/ClickHouse/blob/016a9d5691d6486dd92bf4f4084abdb45baf8a45/src/AggregateFunctions/AggregateFunctionStatistics.cpp#L70
	delta := src.avg - ssp.avg
	countNew := src.count + ssp.count
	if countNew == 0 {
		// Nothing to merge.
		return
	}

	avgNew := (ssp.count*ssp.avg + src.count*src.avg) / countNew
	qNew := ssp.q + src.q + delta*delta*(ssp.count*src.count)/countNew

	ssp.avg = avgNew
	ssp.q = qNew
	ssp.count = countNew
}

func (ssp *statsStddevProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	dst = marshalFloat64(dst, ssp.avg)
	dst = marshalFloat64(dst, ssp.q)
	dst = marshalFloat64(dst, ssp.count)
	return dst
}

func (ssp *statsStddevProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) != 24 {
		return 0, fmt.Errorf("cannot unmarshal stddev from %d bytes; need 24 bytes", len(src))
	}

	ssp.avg = unmarshalFloat64(bytesutil.ToUnsafeString(src))
	src = src[8:]

	ssp.q = unmarshalFloat64(bytesutil.ToUnsafeString(src))
	src = src[8:]

	ssp.count = unmarshalFloat64(bytesutil.ToUnsafeString(src))

	return 0, nil
}

func (ssp *statsStddevProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
	stddev := math.Sqrt(ssp.q / ssp.count)
	return strconv.AppendFloat(dst, stddev, 'f', -1, 64)
}

func parseStatsStddev(lex *lexer) (statsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "stddev")
	if err != nil {
		return nil, err
	}
	sa := &statsStddev{
		fieldFilters: fieldFilters,
	}
	return sa, nil
}
