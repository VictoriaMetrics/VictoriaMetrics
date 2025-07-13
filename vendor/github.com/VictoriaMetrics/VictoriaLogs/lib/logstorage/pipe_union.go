package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeUnion processes '| union ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#union-pipe
type pipeUnion struct {
	// q is a query for obtaining results to add after all the input results
	q *Query

	// runUnionQuery must be initialized by the caller via initUnionQuery before query execution
	runUnionQuery runUnionQueryFunc
}

func (pu *pipeUnion) initUnionQuery(runUnionQuery runUnionQueryFunc) pipe {
	puNew := *pu
	puNew.runUnionQuery = runUnionQuery
	return &puNew
}

func (pu *pipeUnion) String() string {
	return fmt.Sprintf("union (%s)", pu.q.String())
}

func (pu *pipeUnion) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return nil, []pipe{pu}
}

func (pu *pipeUnion) canLiveTail() bool {
	return false
}

func (pu *pipeUnion) hasFilterInWithQuery() bool {
	// The pu.q query with possible in(...) filters is processed independently at pu.flush(), so return false here.
	return false
}

func (pu *pipeUnion) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	// The values for in(..) filters at pu.q query are obtained independently at pu.flush().
	return pu, nil
}

func (pu *pipeUnion) visitSubqueries(visitFunc func(q *Query)) {
	pu.q.visitSubqueries(visitFunc)
}

func (pu *pipeUnion) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (pu *pipeUnion) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUnionProcessor{
		pu:     pu,
		stopCh: stopCh,
		ppNext: ppNext,
	}
}

type pipeUnionProcessor struct {
	pu     *pipeUnion
	stopCh <-chan struct{}
	ppNext pipeProcessor
}

func (pup *pipeUnionProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}
	pup.ppNext.writeBlock(workerID, br)
}

func (pup *pipeUnionProcessor) flush() error {
	// execute the query to union
	ctxWithCancel, cancel := contextutil.NewStopChanContext(pup.stopCh)
	defer cancel()

	return pup.pu.runUnionQuery(ctxWithCancel, pup.pu.q, pup.ppNext.writeBlock)
}

func parsePipeUnion(lex *lexer) (pipe, error) {
	if !lex.isKeyword("union") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "union")
	}
	lex.nextToken()

	q, err := parseQueryInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse union(...): %w", err)
	}

	pu := &pipeUnion{
		q: q,
	}
	return pu, nil
}
