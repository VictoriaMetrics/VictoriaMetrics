package logstorage

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeBlockStats processes '| block_stats ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#block_stats-pipe
type pipeBlockStats struct {
}

func (ps *pipeBlockStats) String() string {
	return "block_stats"
}

func (ps *pipeBlockStats) canLiveTail() bool {
	return false
}

func (ps *pipeBlockStats) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeBlockStats) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return ps, nil
}

func (ps *pipeBlockStats) updateNeededFields(neededFields, unneededFields fieldsSet) {
	unneededFields.reset()
	neededFields.add("*")
}

func (ps *pipeBlockStats) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeBlockStatsProcessor{
		ppNext: ppNext,

		shards: make([]pipeBlockStatsProcessorShard, workersCount),
	}
}

type pipeBlockStatsProcessor struct {
	ppNext pipeProcessor

	shards []pipeBlockStatsProcessorShard
}

type pipeBlockStatsProcessorShard struct {
	pipeBlockStatsProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeBlockStatsProcessorShardNopad{})%128]byte
}

type pipeBlockStatsProcessorShardNopad struct {
	wctx pipeBlockStatsWriteContext
}

func (psp *pipeBlockStatsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &psp.shards[workerID]
	shard.wctx.init(workerID, psp.ppNext, br.rowsLen)

	cs := br.getColumns()
	for _, c := range cs {
		if c.isConst {
			shard.wctx.writeRow(c.name, "const", uint64(len(c.valuesEncoded[0])), 0, 0, 0)
			continue
		}
		if c.isTime {
			var blockSize uint64
			if br.bs != nil {
				blockSize = br.bs.bsw.bh.timestampsHeader.blockSize
			}
			shard.wctx.writeRow(c.name, "time", blockSize, 0, 0, 0)
			continue
		}
		if br.bs == nil {
			shard.wctx.writeRow(c.name, "inmemory", 0, 0, 0, 0)
			continue
		}

		typ := c.valueType.String()
		ch := br.bs.getColumnHeader(c.name)
		dictSize := 0
		dictItemsCount := len(ch.valuesDict.values)
		if c.valueType == valueTypeDict {
			for _, v := range ch.valuesDict.values {
				dictSize += len(v)
			}
		}
		shard.wctx.writeRow(c.name, typ, ch.valuesSize, ch.bloomFilterSize, uint64(dictItemsCount), uint64(dictSize))
	}

	shard.wctx.flush()
	shard.wctx.reset()
}

func (psp *pipeBlockStatsProcessor) flush() error {
	return nil
}

func parsePipeBlockStats(lex *lexer) (pipe, error) {
	if !lex.isKeyword("block_stats") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "block_stats")
	}
	lex.nextToken()

	ps := &pipeBlockStats{}

	return ps, nil
}

type pipeBlockStatsWriteContext struct {
	workerID uint
	ppNext   pipeProcessor

	a       arena
	rowsLen int
	tmpBuf  []byte

	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int
}

func (wctx *pipeBlockStatsWriteContext) reset() {
	wctx.workerID = 0
	wctx.ppNext = nil

	wctx.a.reset()
	wctx.rowsLen = 0
	wctx.tmpBuf = wctx.tmpBuf[:0]

	rcs := wctx.rcs
	for i := range rcs {
		rcs[i].reset()
	}
	wctx.rcs = rcs[:0]

	wctx.rowsCount = 0
}

func (wctx *pipeBlockStatsWriteContext) init(workerID uint, ppNext pipeProcessor, rowsLen int) {
	wctx.reset()

	wctx.workerID = workerID
	wctx.ppNext = ppNext

	wctx.rowsLen = rowsLen
}

func (wctx *pipeBlockStatsWriteContext) writeRow(columnName, columnType string, valuesSize, bloomSize, dictItems, dictSize uint64) {
	rcs := wctx.rcs
	if len(rcs) == 0 {
		wctx.rcs = slicesutil.SetLength(wctx.rcs, 7)
		rcs = wctx.rcs

		rcs[0].name = "field"
		rcs[1].name = "type"
		rcs[2].name = "values_bytes"
		rcs[3].name = "bloom_bytes"
		rcs[4].name = "dict_items"
		rcs[5].name = "dict_bytes"
		rcs[6].name = "rows"
	}

	wctx.addValue(&rcs[0], columnName)
	wctx.addValue(&rcs[1], columnType)
	wctx.addUint64Value(&rcs[2], valuesSize)
	wctx.addUint64Value(&rcs[3], bloomSize)
	wctx.addUint64Value(&rcs[4], dictItems)
	wctx.addUint64Value(&rcs[5], dictSize)
	wctx.addUint64Value(&rcs[6], uint64(wctx.rowsLen))

	wctx.rowsCount++
	if len(wctx.a.b) >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeBlockStatsWriteContext) addUint64Value(rc *resultColumn, n uint64) {
	wctx.tmpBuf = marshalUint64String(wctx.tmpBuf[:0], n)
	wctx.addValue(rc, bytesutil.ToUnsafeString(wctx.tmpBuf))
}

func (wctx *pipeBlockStatsWriteContext) addValue(rc *resultColumn, v string) {
	vCopy := wctx.a.copyString(v)
	rc.addValue(vCopy)
}

func (wctx *pipeBlockStatsWriteContext) flush() {
	rcs := wctx.rcs

	// Flush rcs to ppNext
	br := &wctx.br
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.ppNext.writeBlock(wctx.workerID, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
	wctx.a.reset()
}
