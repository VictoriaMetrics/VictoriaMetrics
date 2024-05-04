package logstorage

import (
	"fmt"
	"sync/atomic"
)

// pipeHead implements '| head ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limiters
type pipeHead struct {
	n uint64
}

func (ph *pipeHead) String() string {
	return fmt.Sprintf("head %d", ph.n)
}

func (ph *pipeHead) getNeededFields() ([]string, map[string][]string) {
	return []string{"*"}, nil
}

func (ph *pipeHead) newPipeProcessor(_ int, _ <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	if ph.n == 0 {
		// Special case - notify the caller to stop writing data to the returned pipeHeadProcessor
		cancel()
	}
	return &pipeHeadProcessor{
		ph:     ph,
		cancel: cancel,
		ppBase: ppBase,
	}
}

type pipeHeadProcessor struct {
	ph     *pipeHead
	cancel func()
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (php *pipeHeadProcessor) writeBlock(workerID uint, br *blockResult) {
	rowsProcessed := php.rowsProcessed.Add(uint64(len(br.timestamps)))
	if rowsProcessed <= php.ph.n {
		// Fast path - write all the rows to ppBase.
		php.ppBase.writeBlock(workerID, br)
		return
	}

	// Slow path - overflow. Write the remaining rows if needed.
	rowsProcessed -= uint64(len(br.timestamps))
	if rowsProcessed >= php.ph.n {
		// Nothing to write. There is no need in cancel() call, since it has been called by another goroutine.
		return
	}

	// Write remaining rows.
	keepRows := php.ph.n - rowsProcessed
	br.truncateRows(int(keepRows))
	php.ppBase.writeBlock(workerID, br)

	// Notify the caller that it should stop passing more data to writeBlock().
	php.cancel()
}

func (php *pipeHeadProcessor) flush() error {
	return nil
}

func parsePipeHead(lex *lexer) (*pipeHead, error) {
	if !lex.isKeyword("head") {
		return nil, fmt.Errorf("expecting 'head'; got %q", lex.token)
	}

	lex.nextToken()
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of head rows to return from %q: %w", lex.token, err)
	}
	lex.nextToken()
	ph := &pipeHead{
		n: n,
	}
	return ph, nil
}
