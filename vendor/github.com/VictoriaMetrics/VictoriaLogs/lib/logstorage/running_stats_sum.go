package logstorage

import (
	"math"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsSum struct {
	fieldFilters []string
}

func (ss *runningStatsSum) String() string {
	return "sum(" + fieldNamesString(ss.fieldFilters) + ")"
}

func (ss *runningStatsSum) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(ss.fieldFilters)
}

func (ss *runningStatsSum) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsSumProcessor{
		sum: nan,
	}
}

type runningStatsSumProcessor struct {
	sum float64
}

func (ssp *runningStatsSumProcessor) updateRunningStats(sf runningStatsFunc, row []Field) {
	sm := sf.(*runningStatsSum)

	forEachMatchingField(row, sm.fieldFilters, func(v string) {
		f, ok := tryParseFloat64(v)
		if !ok {
			return
		}

		if math.IsNaN(ssp.sum) {
			ssp.sum = f
		} else {
			ssp.sum += f
		}
	})
}

func (ssp *runningStatsSumProcessor) getRunningStats() string {
	return strconv.FormatFloat(ssp.sum, 'f', -1, 64)
}

func parseRunningStatsSum(lex *lexer) (runningStatsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "sum")
	if err != nil {
		return nil, err
	}
	ss := &runningStatsSum{
		fieldFilters: fieldFilters,
	}
	return ss, nil
}
