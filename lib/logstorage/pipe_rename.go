package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeRename implements '| rename ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#rename-pipe
type pipeRename struct {
	// srcFields contains a list of source fields to rename
	srcFields []string

	// dstFields contains a list of destination fields
	dstFields []string
}

func (pr *pipeRename) String() string {
	if len(pr.srcFields) == 0 {
		logger.Panicf("BUG: pipeRename must contain at least a single srcField")
	}

	a := make([]string, len(pr.srcFields))
	for i, srcField := range pr.srcFields {
		dstField := pr.dstFields[i]
		a[i] = quoteTokenIfNeeded(srcField) + " as " + quoteTokenIfNeeded(dstField)
	}
	return "rename " + strings.Join(a, ", ")
}

func (pr *pipeRename) canLiveTail() bool {
	return true
}

func (pr *pipeRename) updateNeededFields(neededFields, unneededFields fieldsSet) {
	for i := len(pr.srcFields) - 1; i >= 0; i-- {
		srcField := pr.srcFields[i]
		dstField := pr.dstFields[i]

		if neededFields.contains("*") {
			if unneededFields.contains(dstField) {
				unneededFields.add(srcField)
			} else {
				unneededFields.add(dstField)
				unneededFields.remove(srcField)
			}
		} else {
			if neededFields.contains(dstField) {
				neededFields.remove(dstField)
				neededFields.add(srcField)
			} else {
				neededFields.remove(srcField)
			}
		}
	}
}

func (pr *pipeRename) optimize() {
	// nothing to do
}

func (pr *pipeRename) hasFilterInWithQuery() bool {
	return false
}

func (pr *pipeRename) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pr, nil
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

	br.renameColumns(prp.pr.srcFields, prp.pr.dstFields)
	prp.ppNext.writeBlock(workerID, br)
}

func (prp *pipeRenameProcessor) flush() error {
	return nil
}

func parsePipeRename(lex *lexer) (*pipeRename, error) {
	if !lex.isKeyword("rename", "mv") {
		return nil, fmt.Errorf("expecting 'rename' or 'mv'; got %q", lex.token)
	}

	var srcFields []string
	var dstFields []string
	for {
		lex.nextToken()
		srcField, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse src field name: %w", err)
		}
		if lex.isKeyword("as") {
			lex.nextToken()
		}
		dstField, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse dst field name: %w", err)
		}

		srcFields = append(srcFields, srcField)
		dstFields = append(dstFields, dstField)

		switch {
		case lex.isKeyword("|", ")", ""):
			pr := &pipeRename{
				srcFields: srcFields,
				dstFields: dstFields,
			}
			return pr, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
