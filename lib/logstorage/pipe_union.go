package logstorage

import (
	"context"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
)

// pipeUnion processes '| union ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#union-pipe
type pipeUnion struct {
	// q is a query for obtaining results to add after all the input results
	q *Query

	// runUnionQuery must be initialized by the caller via initUnionQuery before query execution
	runUnionQuery func(ctx context.Context, q *Query, writeBlock func(workerID uint, br *blockResult)) error
}

func (pu *pipeUnion) initUnionQuery(runUnionQuery func(ctx context.Context, q *Query, writeblock func(workerID uint, br *blockResult)) error) pipe {
	puNew := *pu
	puNew.runUnionQuery = runUnionQuery
	return &puNew
}

func (pu *pipeUnion) String() string {
	return fmt.Sprintf("union (%s)", pu.q.String())
}

func (pu *pipeUnion) canLiveTail() bool {
	return false
}

func (pu *pipeUnion) hasFilterInWithQuery() bool {
	// The pu.q query with possible in(...) filters is processed independently at pu.flush(),
	// so return false here.
	return false
}

func (pu *pipeUnion) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pu, nil
}

func (pu *pipeUnion) updateNeededFields(_, _ fieldsSet) {
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

	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '(' before the union query")
	}
	lex.nextToken()

	q, err := parseQuery(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse query inside union(...): %w", err)
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after the union query")
	}
	lex.nextToken()

	pu := &pipeUnion{
		q: q,
	}
	return pu, nil
}
