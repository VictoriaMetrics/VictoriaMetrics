package logstorage

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsFirst struct {
	fieldName string
	offset    int
}

func (sf *runningStatsFirst) String() string {
	s := "first(" + quoteTokenIfNeeded(sf.fieldName) + ")"
	if sf.offset > 0 {
		s += fmt.Sprintf(" offset %d", sf.offset)
	}
	return s
}

func (sf *runningStatsFirst) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(sf.fieldName)
}

func (sf *runningStatsFirst) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsFirstProcessor{}
}

type runningStatsFirstProcessor struct {
	value    string
	rowsSeen int
}

func (sfp *runningStatsFirstProcessor) updateRunningStats(rsf runningStatsFunc, row []Field) {
	sf := rsf.(*runningStatsFirst)

	if sfp.rowsSeen == sf.offset {
		for i := range row {
			f := &row[i]
			if f.Name == sf.fieldName {
				sfp.value = strings.Clone(f.Value)
				break
			}
		}
	}
	sfp.rowsSeen++
}

func (sfp *runningStatsFirstProcessor) getRunningStats() string {
	return sfp.value
}

func parseRunningStatsFirst(lex *lexer) (runningStatsFunc, error) {
	args, err := parseStatsFuncArgs(lex, "first")
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args for the first() function; got %d; want 1; args: %q", len(args), args)
	}

	fieldName := args[0]

	offset := 0
	if lex.isKeyword("offset") {
		lex.nextToken()
		offsetStr := lex.token
		lex.nextToken()
		n, err := strconv.Atoi(offsetStr)
		if err != nil {
			return nil, fmt.Errorf("cannot parse offset=%q at first(%q): %w", offsetStr, fieldName, err)
		}
		if n < 0 {
			return nil, fmt.Errorf("offset=%d cannot be negative at first(%q)", n, fieldName)
		}
		offset = n
	}

	sf := &runningStatsFirst{
		fieldName: fieldName,
		offset:    offset,
	}
	return sf, nil
}
