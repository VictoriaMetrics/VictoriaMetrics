package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsMin struct {
	fieldFilters []string
}

func (sm *runningStatsMin) String() string {
	return "min(" + fieldNamesString(sm.fieldFilters) + ")"
}

func (sm *runningStatsMin) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.fieldFilters)
}

func (sm *runningStatsMin) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsMinProcessor{}
}

type runningStatsMinProcessor struct {
	min      string
	hasItems bool
}

func (smp *runningStatsMinProcessor) updateRunningStats(sf runningStatsFunc, row []Field) {
	sm := sf.(*runningStatsMin)

	forEachMatchingField(row, sm.fieldFilters, func(v string) {
		if !smp.hasItems {
			smp.min = v
			smp.hasItems = true
			return
		}

		if lessString(v, smp.min) {
			smp.min = v
		}
	})
}

func (smp *runningStatsMinProcessor) getRunningStats() string {
	return smp.min
}

func parseRunningStatsMin(lex *lexer) (runningStatsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "min")
	if err != nil {
		return nil, err
	}
	sm := &runningStatsMin{
		fieldFilters: fieldFilters,
	}
	return sm, nil
}
