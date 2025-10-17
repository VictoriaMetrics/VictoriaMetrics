package logstorage

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeOffset implements '| offset ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#offset-pipe
type pipeOffset struct {
	offset uint64
}

func (po *pipeOffset) String() string {
	return fmt.Sprintf("offset %d", po.offset)
}

func (po *pipeOffset) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	if po.offset == 0 {
		// Special case - `offset 0` is safe to push to the remote side.
		return po, nil
	}
	return nil, []pipe{po}
}

func (po *pipeOffset) canLiveTail() bool {
	return false
}

func (po *pipeOffset) canReturnLastNResults() bool {
	return false
}

func (po *pipeOffset) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (po *pipeOffset) hasFilterInWithQuery() bool {
	return false
}

func (po *pipeOffset) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return po, nil
}

func (po *pipeOffset) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (po *pipeOffset) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeOffsetProcessor{
		po:     po,
		ppNext: ppNext,
	}
}

type pipeOffsetProcessor struct {
	po     *pipeOffset
	ppNext pipeProcessor

	rowsProcessed atomic.Uint64
}

func (pop *pipeOffsetProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	rowsProcessed := pop.rowsProcessed.Add(uint64(br.rowsLen))
	if rowsProcessed <= pop.po.offset {
		return
	}

	rowsProcessed -= uint64(br.rowsLen)
	if rowsProcessed >= pop.po.offset {
		pop.ppNext.writeBlock(workerID, br)
		return
	}

	rowsSkip := pop.po.offset - rowsProcessed
	br.skipRows(int(rowsSkip))
	pop.ppNext.writeBlock(workerID, br)
}

func (pop *pipeOffsetProcessor) flush() error {
	return nil
}

func parsePipeOffset(lex *lexer) (pipe, error) {
	if !lex.isKeyword("offset", "skip") {
		return nil, fmt.Errorf("expecting 'offset' or 'skip'; got %q", lex.token)
	}
	op := lex.token
	lex.nextToken()

	token, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot parse '%s': %w", op, err)
	}
	n, err := parseUint(token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse '%s %s': %w", op, token, err)
	}
	po := &pipeOffset{
		offset: n,
	}
	return po, nil
}
