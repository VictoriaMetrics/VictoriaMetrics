package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeCopy implements '| copy ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#copy-pipe
type pipeCopy struct {
	// srcFieldFilters contains a list of source field filters to copy
	srcFieldFilters []string

	// dstFieldFilters contains a list of destination field filters
	dstFieldFilters []string
}

func (pc *pipeCopy) String() string {
	if len(pc.srcFieldFilters) == 0 {
		logger.Panicf("BUG: pipeCopy must contain at least a single srcFieldFilter")
	}

	a := make([]string, len(pc.srcFieldFilters))
	for i, srcFieldFilter := range pc.srcFieldFilters {
		dstFieldFilter := pc.dstFieldFilters[i]
		a[i] = quoteFieldFilterIfNeeded(srcFieldFilter) + " as " + quoteFieldFilterIfNeeded(dstFieldFilter)
	}
	return "copy " + strings.Join(a, ", ")
}

func (pc *pipeCopy) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pc, nil
}

func (pc *pipeCopy) canLiveTail() bool {
	return true
}

func (pc *pipeCopy) updateNeededFields(f *prefixfilter.Filter) {
	for i := len(pc.srcFieldFilters) - 1; i >= 0; i-- {
		srcFieldFilter := pc.srcFieldFilters[i]
		dstFieldFilter := pc.dstFieldFilters[i]

		needSrcField := f.MatchStringOrWildcard(dstFieldFilter)
		f.AddDenyFilter(dstFieldFilter)
		if needSrcField {
			f.AddAllowFilter(srcFieldFilter)
		}
	}
}

func (pc *pipeCopy) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeCopy) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pc, nil
}

func (pc *pipeCopy) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pc *pipeCopy) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeCopyProcessor{
		pc:     pc,
		ppNext: ppNext,
	}
}

type pipeCopyProcessor struct {
	pc     *pipeCopy
	ppNext pipeProcessor
}

func (pcp *pipeCopyProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	br.copyColumnsByFilters(pcp.pc.srcFieldFilters, pcp.pc.dstFieldFilters)
	pcp.ppNext.writeBlock(workerID, br)
}

func (pcp *pipeCopyProcessor) flush() error {
	return nil
}

func parsePipeCopy(lex *lexer) (pipe, error) {
	if !lex.isKeyword("copy", "cp") {
		return nil, fmt.Errorf("expecting 'copy' or 'cp'; got %q", lex.token)
	}

	var srcFieldFilters []string
	var dstFieldFilters []string
	for {
		lex.nextToken()
		srcFieldFilter, err := parseFieldFilter(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse src field name: %w", err)
		}
		if lex.isKeyword("as") {
			lex.nextToken()
		}
		dstFieldFilter, err := parseFieldFilter(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse dst field name: %w", err)
		}

		srcFieldFilters = append(srcFieldFilters, srcFieldFilter)
		dstFieldFilters = append(dstFieldFilters, dstFieldFilter)

		switch {
		case lex.isKeyword("|", ")", ""):
			pc := &pipeCopy{
				srcFieldFilters: srcFieldFilters,
				dstFieldFilters: dstFieldFilters,
			}
			return pc, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
