package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeCopy implements '| copy ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#copy-pipe
type pipeCopy struct {
	// srcFields contains a list of source fields to copy
	srcFields []string

	// dstFields contains a list of destination fields
	dstFields []string
}

func (pc *pipeCopy) String() string {
	if len(pc.srcFields) == 0 {
		logger.Panicf("BUG: pipeCopy must contain at least a single srcField")
	}

	a := make([]string, len(pc.srcFields))
	for i, srcField := range pc.srcFields {
		dstField := pc.dstFields[i]
		a[i] = quoteTokenIfNeeded(srcField) + " as " + quoteTokenIfNeeded(dstField)
	}
	return "copy " + strings.Join(a, ", ")
}

func (pc *pipeCopy) canLiveTail() bool {
	return true
}

func (pc *pipeCopy) updateNeededFields(neededFields, unneededFields fieldsSet) {
	for i := len(pc.srcFields) - 1; i >= 0; i-- {
		srcField := pc.srcFields[i]
		dstField := pc.dstFields[i]

		if neededFields.contains("*") {
			if !unneededFields.contains(dstField) {
				unneededFields.add(dstField)
				unneededFields.remove(srcField)
			}
		} else {
			if neededFields.contains(dstField) {
				neededFields.remove(dstField)
				neededFields.add(srcField)
			}
		}
	}
}

func (pc *pipeCopy) optimize() {
	// Nothing to do
}

func (pc *pipeCopy) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeCopy) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pc, nil
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

	br.copyColumns(pcp.pc.srcFields, pcp.pc.dstFields)
	pcp.ppNext.writeBlock(workerID, br)
}

func (pcp *pipeCopyProcessor) flush() error {
	return nil
}

func parsePipeCopy(lex *lexer) (*pipeCopy, error) {
	if !lex.isKeyword("copy", "cp") {
		return nil, fmt.Errorf("expecting 'copy' or 'cp'; got %q", lex.token)
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
			pc := &pipeCopy{
				srcFields: srcFields,
				dstFields: dstFields,
			}
			return pc, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
