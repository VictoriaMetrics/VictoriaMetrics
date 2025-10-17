package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeBlocksCount processes '| blocks_count' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#blocks_count-pipe
type pipeBlocksCount struct {
	// resultName is an optional name of the column to write results to.
	// By default results are written into 'blocks_count' column.
	resultName string
}

func (pc *pipeBlocksCount) String() string {
	s := "blocks_count"
	if pc.resultName != "blocks_count" {
		s += " as " + quoteTokenIfNeeded(pc.resultName)
	}
	return s
}

func (pc *pipeBlocksCount) splitToRemoteAndLocal(timestamp int64) (pipe, []pipe) {
	resultNameQuoted := quoteTokenIfNeeded(pc.resultName)

	pStr := fmt.Sprintf("stats sum(%s) as %s", resultNameQuoted, resultNameQuoted)
	pLocal := mustParsePipe(pStr, timestamp)

	return pc, []pipe{pLocal}
}

func (pc *pipeBlocksCount) canLiveTail() bool {
	return false
}

func (pc *pipeBlocksCount) canReturnLastNResults() bool {
	return false
}

func (pc *pipeBlocksCount) updateNeededFields(pf *prefixfilter.Filter) {
	pf.Reset()
}

func (pc *pipeBlocksCount) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeBlocksCount) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pc, nil
}

func (pc *pipeBlocksCount) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pc *pipeBlocksCount) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pcp := &pipeBlocksCountProcessor{
		pc:     pc,
		stopCh: stopCh,
		ppNext: ppNext,
	}
	return pcp
}

type pipeBlocksCountProcessor struct {
	pc     *pipeBlocksCount
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeBlocksCountProcessorShard]
}

type pipeBlocksCountProcessorShard struct {
	blocksCount uint64
}

func (pcp *pipeBlocksCountProcessor) writeBlock(workerID uint, _ *blockResult) {
	shard := pcp.shards.Get(workerID)
	shard.blocksCount++
}

func (pcp *pipeBlocksCountProcessor) flush() error {
	if needStop(pcp.stopCh) {
		return nil
	}

	// merge state across shards
	shards := pcp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	blocksCount := shards[0].blocksCount
	shards = shards[1:]
	for _, shard := range shards {
		blocksCount += shard.blocksCount
	}

	// write result
	rowsCountStr := string(marshalUint64String(nil, blocksCount))

	rcs := [1]resultColumn{}
	rcs[0].name = pcp.pc.resultName
	rcs[0].addValue(rowsCountStr)

	var br blockResult
	br.setResultColumns(rcs[:], 1)
	pcp.ppNext.writeBlock(0, &br)

	return nil
}

func parsePipeBlocksCount(lex *lexer) (pipe, error) {
	if !lex.isKeyword("blocks_count") {
		return nil, fmt.Errorf("expecting 'blocks_count'; got %q", lex.token)
	}
	lex.nextToken()

	resultName := "blocks_count"
	if lex.isKeyword("as") {
		lex.nextToken()
		name, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result name for 'blocks_count': %w", err)
		}
		resultName = name
	} else if !lex.isKeyword("", "|") {
		name, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result name for 'blocks_count': %w", err)
		}
		resultName = name
	}

	pc := &pipeBlocksCount{
		resultName: resultName,
	}
	return pc, nil
}
