package logstorage

import (
	"fmt"
	"unsafe"
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

func (pc *pipeBlocksCount) canLiveTail() bool {
	return false
}

func (pc *pipeBlocksCount) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.reset()
	unneededFields.reset()
}

func (pc *pipeBlocksCount) optimize() {
	// nothing to do
}

func (pc *pipeBlocksCount) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeBlocksCount) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pc, nil
}

func (pc *pipeBlocksCount) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	shards := make([]pipeBlocksCountProcessorShard, workersCount)

	pcp := &pipeBlocksCountProcessor{
		pc:     pc,
		stopCh: stopCh,
		ppNext: ppNext,

		shards: shards,
	}
	return pcp
}

type pipeBlocksCountProcessor struct {
	pc     *pipeBlocksCount
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards []pipeBlocksCountProcessorShard
}

type pipeBlocksCountProcessorShard struct {
	pipeBlocksCountProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeBlocksCountProcessorShardNopad{})%128]byte
}

type pipeBlocksCountProcessorShardNopad struct {
	blocksCount uint64
}

func (pcp *pipeBlocksCountProcessor) writeBlock(workerID uint, _ *blockResult) {
	shard := &pcp.shards[workerID]
	shard.blocksCount++
}

func (pcp *pipeBlocksCountProcessor) flush() error {
	if needStop(pcp.stopCh) {
		return nil
	}

	// merge state across shards
	shards := pcp.shards
	blocksCount := shards[0].blocksCount
	shards = shards[1:]
	for i := range shards {
		blocksCount += shards[i].blocksCount
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

func parsePipeBlocksCount(lex *lexer) (*pipeBlocksCount, error) {
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
