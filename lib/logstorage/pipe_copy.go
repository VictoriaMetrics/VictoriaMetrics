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

func (pc *pipeCopy) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededSrcFields := make([]bool, len(pc.srcFields))
	for i, dstField := range pc.dstFields {
		if neededFields.contains(dstField) && !unneededFields.contains(dstField) {
			neededSrcFields[i] = true
		}
	}
	if neededFields.contains("*") {
		// update only unneeded fields
		unneededFields.addFields(pc.dstFields)
		for i, srcField := range pc.srcFields {
			if neededSrcFields[i] {
				unneededFields.remove(srcField)
			}
		}
	} else {
		// update only needed fields and reset unneeded fields
		neededFields.removeFields(pc.dstFields)
		for i, srcField := range pc.srcFields {
			if neededSrcFields[i] {
				neededFields.add(srcField)
			}
		}
		unneededFields.reset()
	}
}

func (pc *pipeCopy) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeCopyProcessor{
		pc:     pc,
		ppBase: ppBase,
	}
}

type pipeCopyProcessor struct {
	pc     *pipeCopy
	ppBase pipeProcessor
}

func (pcp *pipeCopyProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	br.copyColumns(pcp.pc.srcFields, pcp.pc.dstFields)
	pcp.ppBase.writeBlock(workerID, br)
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
