package logstorage

import (
	"fmt"
	"strings"
	"unsafe"
)

// pipeFieldNames processes '| field_names' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#field_names-pipe
type pipeFieldNames struct {
	// resultName is an optional name of the column to write results to.
	// By default results are written into 'name' column.
	resultName string

	// isFirstPipe is set to true if '| field_names' pipe is the first in the query.
	//
	// This allows skipping loading of _time column.
	isFirstPipe bool
}

func (pf *pipeFieldNames) String() string {
	s := "field_names"
	if pf.resultName != "name" {
		s += " as " + quoteTokenIfNeeded(pf.resultName)
	}
	return s
}

func (pf *pipeFieldNames) canLiveTail() bool {
	return false
}

func (pf *pipeFieldNames) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.add("*")
	unneededFields.reset()

	if pf.isFirstPipe {
		unneededFields.add("_time")
	}
}

func (pf *pipeFieldNames) optimize() {
	// nothing to do
}

func (pf *pipeFieldNames) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldNames) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pf, nil
}

func (pf *pipeFieldNames) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	shards := make([]pipeFieldNamesProcessorShard, workersCount)

	pfp := &pipeFieldNamesProcessor{
		pf:     pf,
		stopCh: stopCh,
		ppNext: ppNext,

		shards: shards,
	}
	return pfp
}

type pipeFieldNamesProcessor struct {
	pf     *pipeFieldNames
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards []pipeFieldNamesProcessorShard
}

type pipeFieldNamesProcessorShard struct {
	pipeFieldNamesProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFieldNamesProcessorShardNopad{})%128]byte
}

type pipeFieldNamesProcessorShardNopad struct {
	// m holds hits per each field name
	m map[string]*uint64
}

func (shard *pipeFieldNamesProcessorShard) getM() map[string]*uint64 {
	if shard.m == nil {
		shard.m = make(map[string]*uint64)
	}
	return shard.m
}

func (pfp *pipeFieldNamesProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &pfp.shards[workerID]
	m := shard.getM()

	cs := br.getColumns()
	for _, c := range cs {
		pHits, ok := m[c.name]
		if !ok {
			nameCopy := strings.Clone(c.name)
			hits := uint64(0)
			pHits = &hits
			m[nameCopy] = pHits
		}

		// Assume that the column is set for all the rows in the block.
		// This is much faster than reading all the column values and counting non-empty rows.
		*pHits += uint64(br.rowsLen)
	}
}

func (pfp *pipeFieldNamesProcessor) flush() error {
	if needStop(pfp.stopCh) {
		return nil
	}

	// merge state across shards
	shards := pfp.shards
	m := shards[0].getM()
	shards = shards[1:]
	for i := range shards {
		for name, pHitsSrc := range shards[i].getM() {
			pHits, ok := m[name]
			if !ok {
				m[name] = pHitsSrc
			} else {
				*pHits += *pHitsSrc
			}
		}
	}
	if pfp.pf.isFirstPipe {
		pHits := m["_stream"]
		if pHits == nil {
			hits := uint64(0)
			pHits = &hits
		}
		m["_time"] = pHits
	}

	// write result
	wctx := &pipeFieldNamesWriteContext{
		pfp: pfp,
	}
	wctx.rcs[0].name = pfp.pf.resultName
	wctx.rcs[1].name = "hits"

	for name, pHits := range m {
		hits := string(marshalUint64String(nil, *pHits))
		wctx.writeRow(name, hits)
	}
	wctx.flush()

	return nil
}

type pipeFieldNamesWriteContext struct {
	pfp *pipeFieldNamesProcessor
	rcs [2]resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeFieldNamesWriteContext) writeRow(name, hits string) {
	wctx.rcs[0].addValue(name)
	wctx.rcs[1].addValue(hits)
	wctx.valuesLen += len(name) + len(hits)
	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeFieldNamesWriteContext) flush() {
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(wctx.rcs[:], wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.pfp.ppNext.writeBlock(0, br)
	br.reset()
	wctx.rcs[0].resetValues()
	wctx.rcs[1].resetValues()
}

func parsePipeFieldNames(lex *lexer) (*pipeFieldNames, error) {
	if !lex.isKeyword("field_names") {
		return nil, fmt.Errorf("expecting 'field_names'; got %q", lex.token)
	}
	lex.nextToken()

	resultName := "name"
	if lex.isKeyword("as") {
		lex.nextToken()
		name, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result name for 'field_names': %w", err)
		}
		resultName = name
	} else if !lex.isKeyword("", "|") {
		name, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result name for 'field_names': %w", err)
		}
		resultName = name
	}

	pf := &pipeFieldNames{
		resultName: resultName,
	}
	return pf, nil
}
