package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeFields implements '| fields ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limiters
type pipeFields struct {
	// fields contains list of fields to fetch
	fields []string

	// whether fields contains star
	containsStar bool
}

func (pf *pipeFields) String() string {
	if len(pf.fields) == 0 {
		logger.Panicf("BUG: pipeFields must contain at least a single field")
	}
	return "fields " + fieldNamesString(pf.fields)
}

func (pf *pipeFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeFieldsProcessor{
		pf:     pf,
		ppBase: ppBase,
	}
}

type pipeFieldsProcessor struct {
	pf     *pipeFields
	ppBase pipeProcessor
}

func (pfp *pipeFieldsProcessor) writeBlock(workerID uint, br *blockResult) {
	if !pfp.pf.containsStar {
		br.updateColumns(pfp.pf.fields)
	}
	pfp.ppBase.writeBlock(workerID, br)
}

func (pfp *pipeFieldsProcessor) flush() error {
	return nil
}

func parsePipeFields(lex *lexer) (*pipeFields, error) {
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name")
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ")", ""):
			pf := &pipeFields{
				fields:       fields,
				containsStar: slices.Contains(fields, "*"),
			}
			return pf, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
