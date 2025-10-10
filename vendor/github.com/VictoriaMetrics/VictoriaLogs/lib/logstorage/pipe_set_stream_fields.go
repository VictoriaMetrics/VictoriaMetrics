package logstorage

import (
	"fmt"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeSetStreamFields processes '| set_stream_fields ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#set_stream_fields-pipe
type pipeSetStreamFields struct {
	streamFieldFilters []string

	// iff is an optional filter for skipping setting stream fields
	iff *ifFilter
}

func (ps *pipeSetStreamFields) String() string {
	s := "set_stream_fields"
	if ps.iff != nil {
		s += " " + ps.iff.String()
	}
	s += " " + fieldNamesString(ps.streamFieldFilters)
	return s
}

func (ps *pipeSetStreamFields) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return ps, nil
}

func (ps *pipeSetStreamFields) canLiveTail() bool {
	return true
}

func (ps *pipeSetStreamFields) canReturnLastNResults() bool {
	return true
}

func (ps *pipeSetStreamFields) updateNeededFields(f *prefixfilter.Filter) {
	if !f.MatchString("_stream") {
		return
	}

	if ps.iff != nil {
		f.AddAllowFilters(ps.iff.allowFilters)
	} else {
		f.AddDenyFilter("_stream")
	}
	f.AddAllowFilters(ps.streamFieldFilters)
}

func (ps *pipeSetStreamFields) hasFilterInWithQuery() bool {
	return ps.iff.hasFilterInWithQuery()
}

func (ps *pipeSetStreamFields) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	iffNew, err := ps.iff.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	psNew := *ps
	psNew.iff = iffNew
	return &psNew, nil
}

func (ps *pipeSetStreamFields) visitSubqueries(visitFunc func(q *Query)) {
	ps.iff.visitSubqueries(visitFunc)
}

func (ps *pipeSetStreamFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeSetStreamFieldsProcessor{
		ps:     ps,
		ppNext: ppNext,
	}
}

type pipeSetStreamFieldsProcessor struct {
	ps     *pipeSetStreamFields
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeSetStreamFieldsProcessorShard]
}

type pipeSetStreamFieldsProcessorShard struct {
	bm bitmap

	a   arena
	rcs [2]resultColumn
}

func (psp *pipeSetStreamFieldsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := psp.shards.Get(workerID)
	ps := psp.ps

	bm := &shard.bm
	if iff := ps.iff; iff != nil {
		bm.init(br.rowsLen)
		bm.setBits()
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			psp.ppNext.writeBlock(workerID, br)
			return
		}
	}

	shard.rcs[0].name = "_stream"
	shard.rcs[1].name = "_stream_id"

	streamColumn := br.getColumnByName("_stream")
	streamIDColumn := br.getColumnByName("_stream_id")
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		stream := ""
		streamID := ""
		if ps.iff == nil || bm.isSetBit(rowIdx) {
			stream = shard.setLogStreamFields(ps, br, rowIdx)
		} else {
			stream = streamColumn.getValueAtRow(br, rowIdx)
			streamID = streamIDColumn.getValueAtRow(br, rowIdx)
		}
		shard.rcs[0].addValue(stream)
		shard.rcs[1].addValue(streamID)
	}

	br.addResultColumn(shard.rcs[0])
	br.addResultColumn(shard.rcs[1])
	psp.ppNext.writeBlock(workerID, br)

	shard.a.reset()
	shard.rcs[0].reset()
	shard.rcs[1].reset()
}

func (psp *pipeSetStreamFieldsProcessor) flush() error {
	return nil
}

func (shard *pipeSetStreamFieldsProcessorShard) setLogStreamFields(ps *pipeSetStreamFields, br *blockResult, rowIdx int) string {
	st := GetStreamTags()

	cs := br.getColumns()
	for _, c := range cs {
		if !prefixfilter.MatchFilters(ps.streamFieldFilters, c.name) {
			continue
		}

		v := c.getValueAtRow(br, rowIdx)
		st.Add(c.name, v)
	}

	bLen := len(shard.a.b)
	sort.Sort(st)
	shard.a.b = st.marshalString(shard.a.b)
	PutStreamTags(st)

	return bytesutil.ToUnsafeString(shard.a.b[bLen:])
}

func parsePipeSetStreamFields(lex *lexer) (pipe, error) {
	if !lex.isKeyword("set_stream_fields") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "set_stream_fields")
	}
	lex.nextToken()

	// parse optional if (...)
	var iff *ifFilter
	if lex.isKeyword("if") {
		f, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		iff = f
	}

	// Parse stream fields
	streamFieldFilters, err := parseCommaSeparatedFields(lex)
	if err != nil {
		return nil, err
	}

	ps := &pipeSetStreamFields{
		streamFieldFilters: streamFieldFilters,

		iff: iff,
	}

	return ps, nil
}
