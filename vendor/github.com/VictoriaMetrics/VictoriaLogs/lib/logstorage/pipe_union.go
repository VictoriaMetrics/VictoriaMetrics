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
	// q is a query for obtaining results to add after all the input results are processed.
	//
	// q is nil if rows is non-nil.
	q *Query

	// rows contains rows to add after processing all the input results.
	//
	// rows are obtained either by executing q at initUnionQuery
	// or they can be put inline in the union pipe via the following syntax:
	//
	//     union rows({row1}, ... {rowN})
	//
	rows [][]Field

	// runQuery must be initialized by the caller via initUnionQuery before query execution
	runQuery runUnionQueryFunc
}

func (pu *pipeUnion) initUnionQuery(qctx *QueryContext, runQuery runUnionQueryFunc, eagerExecute bool) (pipe, error) {
	rows := pu.rows
	if eagerExecute && rows == nil {
		qctxLocal := qctx.WithQuery(pu.q)

		var err error
		rows, err = getRows(qctxLocal, func(qctx *QueryContext, writeBlock writeBlockResultFunc) error {
			return runQuery(qctx.Context, qctx.Query, writeBlock)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot execute query at pipe [%s]: %w", pu, err)
		}
	}

	puNew := *pu
	if rows != nil {
		puNew.q = nil
	}
	puNew.rows = rows
	puNew.runQuery = runQuery

	return &puNew, nil
}

func (pu *pipeUnion) String() string {
	var dst []byte
	dst = append(dst, "union "...)

	if pu.rows != nil {
		dst = marshalRows(dst, pu.rows)
	} else {
		dst = append(dst, '(')
		dst = append(dst, pu.q.String()...)
		dst = append(dst, ')')
	}

	return string(dst)
}

func (pu *pipeUnion) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return nil, []pipe{pu}
}

func (pu *pipeUnion) canLiveTail() bool {
	return false
}

func (pu *pipeUnion) canReturnLastNResults() bool {
	return false
}

func (pu *pipeUnion) isFixedOutputFieldsOrder() bool {
	return false
}

func (pu *pipeUnion) hasFilterInWithQuery() bool {
	// The pu.q query with possible in(...) filters is processed independently at pu.flush(), so return false here.
	return false
}

func (pu *pipeUnion) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
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

	if pup.pu.rows != nil {
		var br blockResult
		br.mustInitFromRows(pup.pu.rows)
		pup.ppNext.writeBlock(0, &br)
		return nil
	}

	return pup.pu.runQuery(ctxWithCancel, pup.pu.q, pup.ppNext.writeBlock)
}

func parsePipeUnion(lex *lexer) (pipe, error) {
	if !lex.isKeyword("union") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "union")
	}
	lex.nextToken()

	var q *Query
	var rows [][]Field
	var err error
	if lex.isKeyword("rows") {
		rows, err = parseRows(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse rows inside 'union': %w", err)
		}
	} else {
		q, err = parseQueryInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse subquery inside 'union': %w", err)
		}
	}

	pu := &pipeUnion{
		q:    q,
		rows: rows,
	}
	return pu, nil
}
