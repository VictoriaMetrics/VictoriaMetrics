package logstorage

import (
	"fmt"
	"sync/atomic"
)

// pipeSkip implements '| skip ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limiters
type pipeSkip struct {
	n uint64
}

func (ps *pipeSkip) String() string {
	return fmt.Sprintf("skip %d", ps.n)
}

func (ps *pipeSkip) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeSkipProcessor{
		ps:     ps,
		ppBase: ppBase,
	}
}

type pipeSkipProcessor struct {
	ps     *pipeSkip
	ppBase pipeProcessor

	rowsProcessed atomic.Uint64
}

func (psp *pipeSkipProcessor) writeBlock(workerID uint, br *blockResult) {
	rowsProcessed := psp.rowsProcessed.Add(uint64(len(br.timestamps)))
	if rowsProcessed <= psp.ps.n {
		return
	}

	rowsProcessed -= uint64(len(br.timestamps))
	if rowsProcessed >= psp.ps.n {
		psp.ppBase.writeBlock(workerID, br)
		return
	}

	rowsSkip := psp.ps.n - rowsProcessed
	br.skipRows(int(rowsSkip))
	psp.ppBase.writeBlock(workerID, br)
}

func (psp *pipeSkipProcessor) flush() error {
	return nil
}

func parsePipeSkip(lex *lexer) (*pipeSkip, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing the number of rows to skip")
	}
	n, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the number of rows to skip %q: %w", lex.token, err)
	}
	lex.nextToken()
	ps := &pipeSkip{
		n: n,
	}
	return ps, nil
}
