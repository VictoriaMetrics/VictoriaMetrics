package logstorage

import (
	"fmt"
	"sync/atomic"
)

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

func (spp *pipeSkipProcessor) writeBlock(workerID uint, timestamps []int64, columns []BlockColumn) {
	rowsProcessed := spp.rowsProcessed.Add(uint64(len(timestamps)))
	if rowsProcessed <= spp.ps.n {
		return
	}

	rowsProcessed -= uint64(len(timestamps))
	if rowsProcessed >= spp.ps.n {
		spp.ppBase.writeBlock(workerID, timestamps, columns)
		return
	}

	rowsRemaining := spp.ps.n - rowsProcessed
	cs := make([]BlockColumn, len(columns))
	for i, c := range columns {
		cDst := &cs[i]
		cDst.Name = c.Name
		cDst.Values = c.Values[rowsRemaining:]
	}
	timestamps = timestamps[rowsRemaining:]
	spp.ppBase.writeBlock(workerID, timestamps, cs)
}

func (spp *pipeSkipProcessor) flush() error {
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
