package logstorage

import (
	"fmt"
	"unsafe"
)

// pipeFilter processes '| filter ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#filter-pipe
type pipeFilter struct {
	// f is a filter to apply to the written rows.
	f filter
}

func (pf *pipeFilter) String() string {
	return "filter " + pf.f.String()
}

func (pf *pipeFilter) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		fs := newFieldsSet()
		pf.f.updateNeededFields(fs)
		for f := range fs {
			unneededFields.remove(f)
		}
	} else {
		pf.f.updateNeededFields(neededFields)
	}
}

func (pf *pipeFilter) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeFilterProcessorShard, workersCount)

	pfp := &pipeFilterProcessor{
		pf:     pf,
		ppBase: ppBase,

		shards: shards,
	}
	return pfp
}

type pipeFilterProcessor struct {
	pf     *pipeFilter
	ppBase pipeProcessor

	shards []pipeFilterProcessorShard
}

type pipeFilterProcessorShard struct {
	pipeFilterProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFilterProcessorShardNopad{})%128]byte
}

type pipeFilterProcessorShardNopad struct {
	br blockResult
	bm bitmap
}

func (pfp *pipeFilterProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pfp.shards[workerID]

	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	pfp.pf.f.applyToBlockResult(br, bm)
	if bm.areAllBitsSet() {
		// Fast path - the filter didn't filter out anything - send br to the base pipe as is.
		pfp.ppBase.writeBlock(workerID, br)
		return
	}
	if bm.isZero() {
		// Nothing to send
		return
	}

	// Slow path - copy the remaining rows from br to shard.br before sending them to base pipe.
	shard.br.initFromFilterAllColumns(br, bm)
	pfp.ppBase.writeBlock(workerID, &shard.br)
}

func (pfp *pipeFilterProcessor) flush() error {
	return nil
}

func parsePipeFilter(lex *lexer) (*pipeFilter, error) {
	if !lex.isKeyword("filter") {
		return nil, fmt.Errorf("expecting 'filter'; got %q", lex.token)
	}
	lex.nextToken()

	f, err := parseFilter(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'filter': %w", err)
	}

	pf := &pipeFilter{
		f: f,
	}
	return pf, nil
}
