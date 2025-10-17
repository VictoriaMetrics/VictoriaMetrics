package logstorage

import (
	"strconv"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsCount struct {
	fieldFilters []string
}

func (sc *runningStatsCount) String() string {
	return "count(" + fieldNamesString(sc.fieldFilters) + ")"
}

func (sc *runningStatsCount) updateNeededFields(pf *prefixfilter.Filter) {
	if prefixfilter.MatchAll(sc.fieldFilters) {
		// Special case for count() - it doesn't need loading any additional fields
		return
	}

	pf.AddAllowFilters(sc.fieldFilters)
}

func (sc *runningStatsCount) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsCountProcessor{}
}

type runningStatsCountProcessor struct {
	rowsCount uint64
}

func (scp *runningStatsCountProcessor) updateRunningStats(sf runningStatsFunc, row []Field) {
	sc := sf.(*runningStatsCount)

	if prefixfilter.MatchAll(sc.fieldFilters) {
		scp.rowsCount++
		return
	}

	match := false
	forEachMatchingField(row, sc.fieldFilters, func(v string) {
		if v != "" {
			match = true
		}
	})
	if match {
		scp.rowsCount++
	}
}

func (scp *runningStatsCountProcessor) getRunningStats() string {
	return strconv.FormatUint(scp.rowsCount, 10)
}

func parseRunningStatsCount(lex *lexer) (runningStatsFunc, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "count")
	if err != nil {
		return nil, err
	}
	sc := &runningStatsCount{
		fieldFilters: fieldFilters,
	}
	return sc, nil
}

func forEachMatchingField(fields []Field, fieldFilters []string, callback func(v string)) {
	if isSingleField(fieldFilters) {
		// Fast path for single field
		found := false
		fieldName := fieldFilters[0]
		for i := range fields {
			f := &fields[i]
			if f.Name == fieldName {
				callback(f.Value)
				found = true
			}
		}
		if !found {
			callback("")
		}
		return
	}

	for i := range fields {
		f := &fields[i]
		if prefixfilter.MatchFilters(fieldFilters, f.Name) {
			callback(f.Value)
		}
	}
}
