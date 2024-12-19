package logstorage

import (
	"fmt"
)

// pipeFirst processes '| first ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#first-pipe
type pipeFirst struct {
	ps *pipeSort
}

func (pf *pipeFirst) String() string {
	return pipeLastFirstString(pf.ps)
}

func (pf *pipeFirst) canLiveTail() bool {
	return false
}

func (pf *pipeFirst) updateNeededFields(neededFields, unneededFields fieldsSet) {
	pf.ps.updateNeededFields(neededFields, unneededFields)
}

func (pf *pipeFirst) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFirst) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pf, nil
}

func (pf *pipeFirst) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	return newPipeTopkProcessor(pf.ps, workersCount, stopCh, cancel, ppNext)
}

func (pf *pipeFirst) addPartitionByTime(step int64) {
	pf.ps.addPartitionByTime(step)
}

func parsePipeFirst(lex *lexer) (pipe, error) {
	if !lex.isKeyword("first") {
		return nil, fmt.Errorf("expecting 'first'; got %q", lex.token)
	}
	lex.nextToken()

	ps, err := parsePipeLastFirst(lex)
	if err != nil {
		return nil, err
	}
	pf := &pipeFirst{
		ps: ps,
	}
	return pf, nil
}
