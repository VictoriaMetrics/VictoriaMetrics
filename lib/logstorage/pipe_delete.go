package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pipeDelete implements '| delete ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#delete-pipe
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

func (pd *pipeDelete) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pd, nil
}

func (pd *pipeDelete) canLiveTail() bool {
	return true
}

func (pd *pipeDelete) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.addFields(pd.fields)
	} else {
		neededFields.removeFields(pd.fields)
	}
}

func (pd *pipeDelete) hasFilterInWithQuery() bool {
	return false
}

func (pd *pipeDelete) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pd, nil
}

func (pd *pipeDelete) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pd *pipeDelete) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeDeleteProcessor{
		pd:     pd,
		ppNext: ppNext,
	}
}

type pipeDeleteProcessor struct {
	pd     *pipeDelete
	ppNext pipeProcessor
}

func (pdp *pipeDeleteProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	br.deleteColumns(pdp.pd.fields)
	pdp.ppNext.writeBlock(workerID, br)
}

func (pdp *pipeDeleteProcessor) flush() error {
	return nil
}

func parsePipeDelete(lex *lexer) (pipe, error) {
	if !lex.isKeyword("delete", "del", "rm", "drop") {
		return nil, fmt.Errorf("expecting 'delete', 'del', 'rm' or 'drop'; got %q", lex.token)
	}
	lex.nextToken()

	fields, err := parseCommaSeparatedFields(lex)
	if err != nil {
		return nil, err
	}
	pd := &pipeDelete{
		fields: fields,
	}
	return pd, nil
}
