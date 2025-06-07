package logstorage

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeSample implements '| sample ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#limit-sample
type pipeSample struct {
	// sample shows how many rows on average must be skipped during sampling
	sample uint64
}

func (ps *pipeSample) String() string {
	return fmt.Sprintf("sample %d", ps.sample)
}

func (ps *pipeSample) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return ps, nil
}

func (ps *pipeSample) canLiveTail() bool {
	return true
}

func (ps *pipeSample) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (ps *pipeSample) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeSample) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return ps, nil
}

func (ps *pipeSample) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (ps *pipeSample) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	psp := &pipeSampleProcessor{}
	psp.shards.Init = func(shard *pipeSampleProcessorShard) {
		shard.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
		shard.d = float64(ps.sample) - 1
		shard.ppNext = ppNext

		shard.rowNext = shard.nextStep() - 1
	}
	return psp
}

type pipeSampleProcessor struct {
	shards atomicutil.Slice[pipeSampleProcessorShard]
}

type pipeSampleProcessorShard struct {
	rng    *rand.Rand
	d      float64
	ppNext pipeProcessor

	rowsProcessed uint64
	rowNext       uint64

	rcs []resultColumn
	br  blockResult
}

func (shard *pipeSampleProcessorShard) nextStep() uint64 {
	return 1 + uint64(math.Round(shard.d*shard.rng.ExpFloat64()))
}

func (psp *pipeSampleProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := psp.shards.Get(workerID)

	for {
		if shard.rowNext < shard.rowsProcessed {
			logger.Panicf("BUG: rowNext=%d cannot be smaller than rowsProcessed=%d", shard.rowNext, shard.rowsProcessed)
		}

		rowIdx := shard.rowNext - shard.rowsProcessed
		if rowIdx >= uint64(br.rowsLen) {
			shard.rowsProcessed += uint64(br.rowsLen)
			return
		}

		shard.writeRow(workerID, br, rowIdx)
		shard.rowNext += shard.nextStep()
	}
}

func (shard *pipeSampleProcessorShard) writeRow(workerID uint, br *blockResult, rowIdx uint64) {
	cs := br.getColumns()
	rcs := slicesutil.SetLength(shard.rcs, len(cs))
	for i, c := range cs {
		values := c.getValues(br)

		rcs[i] = resultColumn{
			name:   c.name,
			values: values[rowIdx : rowIdx+1],
		}
	}
	shard.br.setResultColumns(rcs, 1)
	shard.ppNext.writeBlock(workerID, &shard.br)

	clear(rcs)
	shard.rcs = rcs
	shard.br.reset()
}

func (psp *pipeSampleProcessor) flush() error {
	return nil
}

func parsePipeSample(lex *lexer) (pipe, error) {
	if !lex.isKeyword("sample") {
		return nil, fmt.Errorf("expecting 'sample'; got %q", lex.token)
	}
	lex.nextToken()

	sample, err := parseUint(lex.token)
	if err != nil {
		return nil, fmt.Errorf("cannot parse sample from %q: %w", lex.token, err)
	}
	lex.nextToken()
	if sample <= 0 {
		return nil, fmt.Errorf("unexpected sample=%d; it must be bigger than 0", sample)
	}

	ps := &pipeSample{
		sample: sample,
	}
	return ps, nil
}
