package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsMax struct {
	fieldFilters []string
}

func (sm *runningStatsMax) String() string {
	return "max(" + fieldNamesString(sm.fieldFilters) + ")"
}

func (sm *runningStatsMax) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sm.fieldFilters)
}

func (sm *runningStatsMax) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsMaxProcessor{}
}

type runningStatsMaxProcessor struct {
	max      string
	hasItems bool
}

func (smp *runningStatsMaxProcessor) updateRunningStats(sf runningStatsFunc, row []Field) {
	sm := sf.(*runningStatsMax)

	forEachMatchingField(row, sm.fieldFilters, func(v string) {
		if !smp.hasItems {
			smp.max = v
			smp.hasItems = true
			return
		}

		if lessString(smp.max, v) {
			smp.max = v
		}
	})
}

func (smp *runningStatsMaxProcessor) getRunningStats() string {
	return smp.max
}

func parseRunningStatsMax(lex *lexer) (runningStatsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "max")
	if err != nil {
		return nil, err
	}
	sm := &runningStatsMax{
		fieldFilters: fieldFilters,
	}
	return sm, nil
}
