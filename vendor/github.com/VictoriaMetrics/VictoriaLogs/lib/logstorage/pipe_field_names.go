package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeFieldNames processes '| field_names' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#field_names-pipe
type pipeFieldNames struct {
	// resultName is an optional name of the column to write results to.
	// By default results are written into 'name' column.
	resultName string

	// if isFirstPipe is set, then there is no need in loading columnsHeader in writeBlock().
	isFirstPipe bool
}

func (pf *pipeFieldNames) String() string {
	s := "field_names"
	if pf.resultName != "name" {
		s += " as " + quoteTokenIfNeeded(pf.resultName)
	}
	return s
}

func (pf *pipeFieldNames) splitToRemoteAndLocal(timestamp int64) (pipe, []pipe) {
	pStr := fmt.Sprintf("stats by (%s) sum(hits) hits", quoteTokenIfNeeded(pf.resultName))
	pLocal := mustParsePipe(pStr, timestamp)

	return pf, []pipe{pLocal}
}

func (pf *pipeFieldNames) canLiveTail() bool {
	return false
}

func (pf *pipeFieldNames) updateNeededFields(f *prefixfilter.Filter) {
	if pf.isFirstPipe {
		f.Reset()
	} else {
		f.AddAllowFilter("*")
	}
}

func (pf *pipeFieldNames) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldNames) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pf, nil
}

func (pf *pipeFieldNames) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pf *pipeFieldNames) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pfp := &pipeFieldNamesProcessor{
		pf:     pf,
		stopCh: stopCh,
		ppNext: ppNext,
	}
	return pfp
}

type pipeFieldNamesProcessor struct {
	pf     *pipeFieldNames
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeFieldNamesProcessorShard]
}

type pipeFieldNamesProcessorShard struct {
	// m holds hits per each field name
	m map[string]*uint64

	// a is used for reducing memory allocations when collecting the stats over big number of log fields
	a chunkedAllocator
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

	// Assume that the column is set for all the rows in the block.
	// This is much faster than reading all the column values and counting non-empty rows.
	hits := uint64(br.rowsLen)

	shard := pfp.shards.Get(workerID)
	if !pfp.pf.isFirstPipe || br.bs == nil || br.bs.partFormatVersion() < 1 {
		cs := br.getColumns()
		for _, c := range cs {
			shard.updateColumnHits(c.name, hits)
		}
	} else {
		cshIndex := br.bs.getColumnsHeaderIndex()
		shard.updateHits(cshIndex.columnHeadersRefs, br, hits)
		shard.updateHits(cshIndex.constColumnsRefs, br, hits)
		shard.updateColumnHits("_time", hits)
		shard.updateColumnHits("_stream", hits)
		shard.updateColumnHits("_stream_id", hits)
	}
}

func (shard *pipeFieldNamesProcessorShard) updateHits(refs []columnHeaderRef, br *blockResult, hits uint64) {
	for _, cr := range refs {
		columnName := br.bs.getColumnNameByID(cr.columnNameID)
		shard.updateColumnHits(columnName, hits)
	}
}

func (shard *pipeFieldNamesProcessorShard) updateColumnHits(columnName string, hits uint64) {
	if columnName == "" {
		columnName = "_msg"
	}
	m := shard.getM()
	pHits := m[columnName]
	if pHits == nil {
		nameCopy := shard.a.cloneString(columnName)
		pHits = shard.a.newUint64()
		m[nameCopy] = pHits
	}
	*pHits += hits
}

func (pfp *pipeFieldNamesProcessor) flush() error {
	if needStop(pfp.stopCh) {
		return nil
	}

	// merge state across shards
	shards := pfp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	m := shards[0].getM()
	shards = shards[1:]
	for _, shard := range shards {
		for name, pHitsSrc := range shard.getM() {
			pHits := m[name]
			if pHits == nil {
				m[name] = pHitsSrc
			} else {
				*pHits += *pHitsSrc
			}
		}
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

func parsePipeFieldNames(lex *lexer) (pipe, error) {
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
