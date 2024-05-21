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

func (po *pipeOffset) updateNeededFields(_, _ fieldsSet) {
}

func (po *pipeOffset) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeOffsetProcessor{
		po:     po,
		ppBase: ppBase,
	}
}

type pipeOffsetProcessor struct {
	po     *pipeOffset
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (pop *pipeOffsetProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	rowsProcessed := pop.rowsProcessed.Add(uint64(len(br.timestamps)))
	if rowsProcessed <= pop.po.offset {
		return
	}

	rowsProcessed -= uint64(len(br.timestamps))
	if rowsProcessed >= pop.po.offset {
		pop.ppBase.writeBlock(workerID, br)
		return
	}

	rowsSkip := pop.po.offset - rowsProcessed
	br.skipRows(int(rowsSkip))
	pop.ppBase.writeBlock(workerID, br)
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
