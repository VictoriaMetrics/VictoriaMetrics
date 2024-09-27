package logstorage

import (
	"fmt"
	"sync/atomic"
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

func (po *pipeOffset) canLiveTail() bool {
	return false
}

func (po *pipeOffset) updateNeededFields(_, _ fieldsSet) {
	// nothing to do
}

func (po *pipeOffset) optimize() {
	// nothing to do
}

func (po *pipeOffset) hasFilterInWithQuery() bool {
	return false
}

func (po *pipeOffset) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return po, nil
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

func parsePipeOffset(lex *lexer) (*pipeOffset, error) {
	if !lex.isKeyword("offset", "skip") {
		return nil, fmt.Errorf("expecting 'offset' or 'skip'; got %q", lex.token)
	}

	lex.nextToken()
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of rows to skip from %q: %w", lex.token, err)
	}
	lex.nextToken()
	po := &pipeOffset{
		offset: n,
	}
	return po, nil
}
