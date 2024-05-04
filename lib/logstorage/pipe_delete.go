package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeDelete implements '| delete ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#transformations
type pipeDelete struct {
	// fields contains a list of fields to delete
	fields []string
}

func (pd *pipeDelete) String() string {
	if len(pd.fields) == 0 {
		logger.Panicf("BUG: pipeDelete must contain at least a single field")
	}

	return "delete " + fieldNamesString(pd.fields)
}

func (pd *pipeDelete) getNeededFields() ([]string, map[string][]string) {
	return []string{"*"}, nil
}

func (pd *pipeDelete) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeDeleteProcessor{
		pd:     pd,
		ppBase: ppBase,
	}
}

type pipeDeleteProcessor struct {
	pd     *pipeDelete
	ppBase pipeProcessor
}

func (pdp *pipeDeleteProcessor) writeBlock(workerID uint, br *blockResult) {
	br.deleteColumns(pdp.pd.fields)
	pdp.ppBase.writeBlock(workerID, br)
}

func (pdp *pipeDeleteProcessor) flush() error {
	return nil
}

func parsePipeDelete(lex *lexer) (*pipeDelete, error) {
	if !lex.isKeyword("delete") {
		return nil, fmt.Errorf("expecting 'delete'; got %q", lex.token)
	}

	var fields []string
	for {
		lex.nextToken()
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}

		fields = append(fields, field)

		switch {
		case lex.isKeyword("|", ")", ""):
			pd := &pipeDelete{
				fields: fields,
			}
			return pd, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',', '|' or ')'", lex.token)
		}
	}
}
