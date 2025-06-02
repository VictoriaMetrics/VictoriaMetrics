package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipeRename implements '| rename ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#rename-pipe
type pipeRename struct {
	// srcFieldFilters contains a list of source fields to rename
	srcFieldFilters []string

	// dstFieldFilters contains a list of destination fields
	dstFieldFilters []string
}

func (pr *pipeRename) String() string {
	if len(pr.srcFieldFilters) == 0 {
		logger.Panicf("BUG: pipeRename must contain at least a single srcFieldFilter")
	}

	a := make([]string, len(pr.srcFieldFilters))
	for i, srcFieldFilter := range pr.srcFieldFilters {
		dstFieldFilter := pr.dstFieldFilters[i]
		a[i] = quoteFieldFilterIfNeeded(srcFieldFilter) + " as " + quoteFieldFilterIfNeeded(dstFieldFilter)
	}
	return "rename " + strings.Join(a, ", ")
}

func (pr *pipeRename) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pr, nil
}

func (pr *pipeRename) canLiveTail() bool {
	return true
}

func (pr *pipeRename) updateNeededFields(pf *prefixfilter.Filter) {
	for i := len(pr.srcFieldFilters) - 1; i >= 0; i-- {
		srcFieldFilter := pr.srcFieldFilters[i]
		dstFieldFilter := pr.dstFieldFilters[i]

		needSrcField := pf.MatchStringOrWildcard(dstFieldFilter)
		pf.AddDenyFilter(dstFieldFilter)
		if needSrcField {
			pf.AddAllowFilter(srcFieldFilter)
		} else {
			pf.AddDenyFilter(srcFieldFilter)
		}
	}
}

func (pr *pipeRename) hasFilterInWithQuery() bool {
	return false
}

func (pr *pipeRename) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pr, nil
}

func (pr *pipeRename) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pr *pipeRename) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeRenameProcessor{
		pr:     pr,
		ppNext: ppNext,
	}
}

type pipeRenameProcessor struct {
	pr     *pipeRename
	ppNext pipeProcessor
}

func (prp *pipeRenameProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	br.renameColumnsByFilters(prp.pr.srcFieldFilters, prp.pr.dstFieldFilters)
	prp.ppNext.writeBlock(workerID, br)
}

func (prp *pipeRenameProcessor) flush() error {
	return nil
}

func parsePipeRename(lex *lexer) (pipe, error) {
	if !lex.isKeyword("rename", "mv") {
		return nil, fmt.Errorf("expecting 'rename' or 'mv'; got %q", lex.token)
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
			pr := &pipeRename{
				srcFieldFilters: srcFieldFilters,
				dstFieldFilters: dstFieldFilters,
			}
			return pr, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
