package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
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

func (pf *pipeFilter) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pf, nil
}

func (pf *pipeFilter) canLiveTail() bool {
	return true
}

func (pf *pipeFilter) updateNeededFields(f *prefixfilter.Filter) {
	pf.f.updateNeededFields(f)
}

func (pf *pipeFilter) hasFilterInWithQuery() bool {
	return hasFilterInWithQueryForFilter(pf.f)
}

func (pf *pipeFilter) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	fNew, err := initFilterInValuesForFilter(cache, pf.f, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	pfNew := *pf
	pfNew.f = fNew
	return &pfNew, nil
}

func (pf *pipeFilter) visitSubqueries(visitFunc func(q *Query)) {
	visitSubqueriesInFilter(pf.f, visitFunc)
}

func (pf *pipeFilter) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pfp := &pipeFilterProcessor{
		pf:     pf,
		ppNext: ppNext,
	}
	return pfp
}

type pipeFilterProcessor struct {
	pf     *pipeFilter
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeFilterProcessorShard]
}

type pipeFilterProcessorShard struct {
	br blockResult
	bm bitmap
}

func (pfp *pipeFilterProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pfp.shards.Get(workerID)

	bm := &shard.bm
	bm.init(br.rowsLen)
	bm.setBits()
	pfp.pf.f.applyToBlockResult(br, bm)
	if bm.areAllBitsSet() {
		// Fast path - the filter didn't filter out anything - send br to the next pipe as is.
		pfp.ppNext.writeBlock(workerID, br)
		return
	}
	if bm.isZero() {
		// Nothing to send
		return
	}

	// Slow path - copy the remaining rows from br to shard.br before sending them to the next pipe.
	shard.br.initFromFilterAllColumns(br, bm)
	pfp.ppNext.writeBlock(workerID, &shard.br)
}

func (pfp *pipeFilterProcessor) flush() error {
	return nil
}

func parsePipeFilter(lex *lexer, needFilterKeyword bool) (pipe, error) {
	if needFilterKeyword {
		if !lex.isKeyword("filter", "where") {
			return nil, fmt.Errorf("expecting 'filter' or 'where'; got %q", lex.token)
		}
		lex.nextToken()
	}

	f, err := parseFilter(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'filter': %w", err)
	}

	pf := &pipeFilter{
		f: f,
	}
	return pf, nil
}
