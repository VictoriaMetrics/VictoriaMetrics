package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
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

func (pf *pipeFirst) splitToRemoteAndLocal(timestamp int64) (pipe, []pipe) {
	return pf.ps.splitToRemoteAndLocal(timestamp)
}

func (pf *pipeFirst) canLiveTail() bool {
	return false
}

func (pf *pipeFirst) updateNeededFields(f *prefixfilter.Filter) {
	pf.ps.updateNeededFields(f)
}

func (pf *pipeFirst) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFirst) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pf, nil
}

func (pf *pipeFirst) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pf *pipeFirst) newPipeProcessor(_ int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	return newPipeTopkProcessor(pf.ps, stopCh, cancel, ppNext)
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
