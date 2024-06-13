package logstorage

import (
	"fmt"
	"slices"
	"unsafe"

	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeUnroll processes '| unroll ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unroll-pipe
type pipeUnroll struct {
	// fields to unroll
	fields []string

	// iff is an optional filter for skipping the unroll
	iff *ifFilter
}

func (pu *pipeUnroll) String() string {
	s := "unroll"
	if pu.iff != nil {
		s += " " + pu.iff.String()
	}
	s += " by (" + fieldNamesString(pu.fields) + ")"
	return s
}

func (pu *pipeUnroll) optimize() {
	pu.iff.optimizeFilterIn()
}

func (pu *pipeUnroll) hasFilterInWithQuery() bool {
	return pu.iff.hasFilterInWithQuery()
}

func (pu *pipeUnroll) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pu.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	puNew := *pu
	puNew.iff = iffNew
	return &puNew, nil
}

func (pu *pipeUnroll) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if pu.iff != nil {
			unneededFields.removeFields(pu.iff.neededFields)
		}
		unneededFields.removeFields(pu.fields)
	} else {
		if pu.iff != nil {
			neededFields.addFields(pu.iff.neededFields)
		}
		neededFields.addFields(pu.fields)
	}
}

func (pu *pipeUnroll) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUnrollProcessor{
		pu:     pu,
		stopCh: stopCh,
		ppNext: ppNext,

		shards: make([]pipeUnrollProcessorShard, workersCount),
	}
}

type pipeUnrollProcessor struct {
	pu     *pipeUnroll
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards []pipeUnrollProcessorShard
}

type pipeUnrollProcessorShard struct {
	pipeUnrollProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnrollProcessorShardNopad{})%128]byte
}

type pipeUnrollProcessorShardNopad struct {
	bm bitmap

	wctx pipeUnpackWriteContext
	a    arena

	columnValues   [][]string
	unrolledValues [][]string
	valuesBuf      []string
	fields         []Field
}

func (pup *pipeUnrollProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	pu := pup.pu
	shard := &pup.shards[workerID]
	shard.wctx.init(workerID, pup.ppNext, false, false, br)

	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	if iff := pu.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pup.ppNext.writeBlock(workerID, br)
			return
		}
	}

	shard.columnValues = slicesutil.SetLength(shard.columnValues, len(pu.fields))
	columnValues := shard.columnValues
	for i, f := range pu.fields {
		c := br.getColumnByName(f)
		columnValues[i] = c.getValues(br)
	}

	fields := shard.fields
	for rowIdx := range br.timestamps {
		if bm.isSetBit(rowIdx) {
			if needStop(pup.stopCh) {
				return
			}
			shard.writeUnrolledFields(pu.fields, columnValues, rowIdx)
		} else {
			fields = fields[:0]
			for i, f := range pu.fields {
				v := columnValues[i][rowIdx]
				fields = append(fields, Field{
					Name:  f,
					Value: v,
				})
			}
			shard.wctx.writeRow(rowIdx, fields)
		}
	}

	shard.wctx.flush()
	shard.wctx.reset()
	shard.a.reset()
}

func (shard *pipeUnrollProcessorShard) writeUnrolledFields(fieldNames []string, columnValues [][]string, rowIdx int) {
	// unroll values at rowIdx row

	shard.unrolledValues = slicesutil.SetLength(shard.unrolledValues, len(columnValues))
	unrolledValues := shard.unrolledValues

	valuesBuf := shard.valuesBuf[:0]
	for i, values := range columnValues {
		v := values[rowIdx]
		valuesBufLen := len(valuesBuf)
		valuesBuf = unpackJSONArray(valuesBuf, &shard.a, v)
		unrolledValues[i] = valuesBuf[valuesBufLen:]
	}
	shard.valuesBuf = valuesBuf

	// find the number of rows across unrolled values
	rows := len(unrolledValues[0])
	for _, values := range unrolledValues[1:] {
		if len(values) > rows {
			rows = len(values)
		}
	}
	if rows == 0 {
		// Unroll too a single row with empty unrolled values.
		rows = 1
	}

	// write unrolled values to the next pipe.
	fields := shard.fields
	for unrollIdx := 0; unrollIdx < rows; unrollIdx++ {
		fields = fields[:0]
		for i, values := range unrolledValues {
			v := ""
			if unrollIdx < len(values) {
				v = values[unrollIdx]
			}
			fields = append(fields, Field{
				Name:  fieldNames[i],
				Value: v,
			})
		}
		shard.wctx.writeRow(rowIdx, fields)
	}
	shard.fields = fields
}

func (pup *pipeUnrollProcessor) flush() error {
	return nil
}

func parsePipeUnroll(lex *lexer) (*pipeUnroll, error) {
	if !lex.isKeyword("unroll") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unroll")
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

	// parse by (...)
	if lex.isKeyword("by") {
		lex.nextToken()
	}

	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'by(...)' at 'unroll': %w", err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("'by(...)' at 'unroll' must contain at least a single field")
	}
	if slices.Contains(fields, "*") {
		return nil, fmt.Errorf("unroll by '*' isn't supported")
	}

	pu := &pipeUnroll{
		fields: fields,
		iff:    iff,
	}

	return pu, nil
}

func unpackJSONArray(dst []string, a *arena, s string) []string {
	if s == "" || s[0] != '[' {
		return dst
	}

	p := jspp.Get()
	defer jspp.Put(p)

	jsv, err := p.Parse(s)
	if err != nil {
		return dst
	}
	jsa, err := jsv.Array()
	if err != nil {
		return dst
	}
	for _, jsv := range jsa {
		if jsv.Type() == fastjson.TypeString {
			sb, err := jsv.StringBytes()
			if err != nil {
				logger.Panicf("BUG: unexpected error returned from StringBytes(): %s", err)
			}
			v := a.copyBytesToString(sb)
			dst = append(dst, v)
		} else {
			bLen := len(a.b)
			a.b = jsv.MarshalTo(a.b)
			v := bytesutil.ToUnsafeString(a.b[bLen:])
			dst = append(dst, v)
		}
	}
	return dst
}

var jspp fastjson.ParserPool
