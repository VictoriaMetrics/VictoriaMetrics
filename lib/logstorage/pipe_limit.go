package logstorage

import (
	"fmt"
	"sync/atomic"
)

// pipeLimit implements '| limit ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limit-pipe
type pipeLimit struct {
	limit uint64
}

func (pl *pipeLimit) String() string {
	return fmt.Sprintf("limit %d", pl.limit)
}

func (pl *pipeLimit) updateNeededFields(_, _ fieldsSet) {
}

func (pl *pipeLimit) newPipeProcessor(_ int, _ <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	if pl.limit == 0 {
		// Special case - notify the caller to stop writing data to the returned pipeLimitProcessor
		cancel()
	}
	return &pipeLimitProcessor{
		pl:     pl,
		cancel: cancel,
		ppBase: ppBase,
	}
}

type pipeLimitProcessor struct {
	pl     *pipeLimit
	cancel func()
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (plp *pipeLimitProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	rowsProcessed := plp.rowsProcessed.Add(uint64(len(br.timestamps)))
	if rowsProcessed <= plp.pl.limit {
		// Fast path - write all the rows to ppBase.
		plp.ppBase.writeBlock(workerID, br)
		return
	}

	// Slow path - overflow. Write the remaining rows if needed.
	rowsProcessed -= uint64(len(br.timestamps))
	if rowsProcessed >= plp.pl.limit {
		// Nothing to write. There is no need in cancel() call, since it has been called by another goroutine.
		return
	}

	// Write remaining rows.
	keepRows := plp.pl.limit - rowsProcessed
	br.truncateRows(int(keepRows))
	plp.ppBase.writeBlock(workerID, br)

	// Notify the caller that it should stop passing more data to writeBlock().
	plp.cancel()
}

func (plp *pipeLimitProcessor) flush() error {
	return nil
}

func parsePipeLimit(lex *lexer) (*pipeLimit, error) {
	if !lex.isKeyword("limit", "head") {
		return nil, fmt.Errorf("expecting 'limit' or 'head'; got %q", lex.token)
	}

	lex.nextToken()
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse rows limit from %q: %w", lex.token, err)
	}
	lex.nextToken()
	pl := &pipeLimit{
		limit: n,
	}
	return pl, nil
}
