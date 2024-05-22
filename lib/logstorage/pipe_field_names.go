package logstorage

import (
	"fmt"
	"strings"
	"unsafe"
)

// pipeFieldNames processes '| field_names' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#field-names-pipe
type pipeFieldNames struct {
	// resultName is the name of the column to write results to.
	resultName string

	// isFirstPipe is set to true if '| field_names' pipe is the first in the query.
	//
	// This allows skipping loading of _time column.
	isFirstPipe bool
}

func (pf *pipeFieldNames) String() string {
	return "field_names as " + quoteTokenIfNeeded(pf.resultName)
}

func (pf *pipeFieldNames) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.add("*")
	unneededFields.reset()

	if pf.isFirstPipe {
		unneededFields.add("_time")
	}
}

func (pf *pipeFieldNames) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeFieldNamesProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeFieldNamesProcessorShard{
			pipeFieldNamesProcessorShardNopad: pipeFieldNamesProcessorShardNopad{
				m: make(map[string]struct{}),
			},
		}
	}

	pfp := &pipeFieldNamesProcessor{
		pf:     pf,
		stopCh: stopCh,
		ppBase: ppBase,

		shards: shards,
	}
	return pfp
}

type pipeFieldNamesProcessor struct {
	pf     *pipeFieldNames
	stopCh <-chan struct{}
	ppBase pipeProcessor

	shards []pipeFieldNamesProcessorShard
}

type pipeFieldNamesProcessorShard struct {
	pipeFieldNamesProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFieldNamesProcessorShardNopad{})%128]byte
}

type pipeFieldNamesProcessorShardNopad struct {
	// m holds unique field names.
	m map[string]struct{}
}

func (pfp *pipeFieldNamesProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pfp.shards[workerID]
	cs := br.getColumns()
	for _, c := range cs {
		if _, ok := shard.m[c.name]; !ok {
			nameCopy := strings.Clone(c.name)
			shard.m[nameCopy] = struct{}{}
		}
	}
}

func (pfp *pipeFieldNamesProcessor) flush() error {
	if needStop(pfp.stopCh) {
		return nil
	}

	// merge state across shards
	shards := pfp.shards
	m := shards[0].m
	shards = shards[1:]
	for i := range shards {
		for k := range shards[i].m {
			m[k] = struct{}{}
		}
	}
	if pfp.pf.isFirstPipe {
		m["_time"] = struct{}{}
	}

	// write result
	wctx := &pipeFieldNamesWriteContext{
		pfp: pfp,
	}
	wctx.rcs[0].name = pfp.pf.resultName
	for k := range m {
		wctx.writeRow(k)
	}
	wctx.flush()

	return nil
}

type pipeFieldNamesWriteContext struct {
	pfp *pipeFieldNamesProcessor
	rcs [1]resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeFieldNamesWriteContext) writeRow(v string) {
	wctx.rcs[0].addValue(v)
	wctx.valuesLen += len(v)
	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeFieldNamesWriteContext) flush() {
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppBase
	br.setResultColumns(wctx.rcs[:1], wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.pfp.ppBase.writeBlock(0, br)
	br.reset()
	wctx.rcs[0].resetValues()
}

func parsePipeFieldNames(lex *lexer) (*pipeFieldNames, error) {
	if !lex.isKeyword("field_names") {
		return nil, fmt.Errorf("expecting 'field_names'; got %q", lex.token)
	}
	lex.nextToken()

	if lex.isKeyword("as") {
		lex.nextToken()
	}
	resultName, err := parseFieldName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name for 'field_names': %w", err)
	}

	pf := &pipeFieldNames{
		resultName: resultName,
	}
	return pf, nil
}
