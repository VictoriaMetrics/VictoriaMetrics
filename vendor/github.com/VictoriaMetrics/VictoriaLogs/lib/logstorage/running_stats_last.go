package logstorage

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type runningStatsLast struct {
	fieldName string
	offset    int
}

func (sl *runningStatsLast) String() string {
	s := "last(" + quoteTokenIfNeeded(sl.fieldName) + ")"
	if sl.offset > 0 {
		s += fmt.Sprintf(" offset %d", sl.offset)
	}
	return s
}

func (sl *runningStatsLast) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(sl.fieldName)
}

func (sl *runningStatsLast) newRunningStatsProcessor() runningStatsProcessor {
	return &runningStatsLastProcessor{
		sl: sl,
	}
}

type runningStatsLastProcessor struct {
	sl     *runningStatsLast
	values []string
}

func (slp *runningStatsLastProcessor) updateRunningStats(_ runningStatsFunc, row []Field) {
	sl := slp.sl

	value := ""
	for i := range row {
		f := &row[i]
		if f.Name == sl.fieldName {
			value = strings.Clone(f.Value)
			break
		}
	}

	slp.values = append(slp.values, value)
	if len(slp.values) > sl.offset+1 {
		slp.values = slp.values[len(slp.values)-sl.offset-1:]
	}
}

func (slp *runningStatsLastProcessor) getRunningStats() string {
	if len(slp.values) <= slp.sl.offset {
		return ""
	}
	return slp.values[len(slp.values)-slp.sl.offset-1]
}

func parseRunningStatsLast(lex *lexer) (runningStatsFunc, error) {
	args, err := parseStatsFuncArgs(lex, "last")
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpeccted number of args for the last() function; got %d; want 1; args: %q", len(args), args)
	}

	fieldName := args[0]

	offset := 0
	if lex.isKeyword("offset") {
		lex.nextToken()
		offsetStr := lex.token
		lex.nextToken()
		n, err := strconv.Atoi(offsetStr)
		if err != nil {
			return nil, fmt.Errorf("cannot parse offset=%q at last(%q): %w", offsetStr, fieldName, err)
		}
		if n < 0 {
			return nil, fmt.Errorf("offset=%d cannot be negative at last(%q)", n, fieldName)
		}
		offset = n
	}

	sf := &runningStatsLast{
		fieldName: fieldName,
		offset:    offset,
	}
	return sf, nil
}
