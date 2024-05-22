package logstorage

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeFormat processes '| format ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#format-pipe
type pipeFormat struct {
	formatStr string
	steps     []patternStep

	resultField string

	// iff is an optional filter for skipping the format func
	iff *ifFilter
}

func (pf *pipeFormat) String() string {
	s := "format" + quoteTokenIfNeeded(pf.formatStr)
	if pf.iff != nil {
		s += " " + pf.iff.String()
	}
	s += " as " + quoteTokenIfNeeded(pf.resultField)
	return s
}

func (pf *pipeFormat) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(pf.resultField) {
			unneededFields.add(pf.resultField)
			for _, step := range pf.steps {
				if step.field != "" {
					unneededFields.remove(step.field)
				}
			}
		}
	} else {
		if neededFields.contains(pf.resultField) {
			neededFields.remove(pf.resultField)
			for _, step := range pf.steps {
				if step.field != "" {
					neededFields.add(step.field)
				}
			}
		}
	}
}

func (pf *pipeFormat) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return &pipeFormatProcessor{
		pf:     pf,
		ppBase: ppBase,

		shards: make([]pipeFormatProcessorShard, workersCount),
	}
}

type pipeFormatProcessor struct {
	pf     *pipeFormat
	ppBase pipeProcessor

	shards []pipeFormatProcessorShard
}

type pipeFormatProcessorShard struct {
	pipeFormatProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeFormatProcessorShardNopad{})%128]byte
}

type pipeFormatProcessorShardNopad struct {
	bm bitmap

	uctx fieldsUnpackerContext
	wctx pipeUnpackWriteContext
}

func (pfp *pipeFormatProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pfp.shards[workerID]
	shard.wctx.init(workerID, pfp.ppBase, br)
	shard.uctx.init(workerID, "")

	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	if iff := pfp.pf.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pfp.ppBase.writeBlock(workerID, br)
			return
		}
	}

	for rowIdx := range br.timestamps {
		if bm.isSetBit(rowIdx) {
			shard.formatRow(pfp.pf, br, rowIdx)
			shard.wctx.writeRow(rowIdx, shard.uctx.fields)
		} else {
			shard.wctx.writeRow(rowIdx, nil)
		}
	}

	shard.wctx.flush()
	shard.wctx.reset()
	shard.uctx.reset()
}

func (pfp *pipeFormatProcessor) flush() error {
	return nil
}

func (shard *pipeFormatProcessorShard) formatRow(pf *pipeFormat, br *blockResult, rowIdx int) {
	bb := bbPool.Get()
	b := bb.B
	for _, step := range pf.steps {
		b = append(b, step.prefix...)
		if step.field != "" {
			c := br.getColumnByName(step.field)
			v := c.getValueAtRow(br, rowIdx)
			b = append(b, v...)
		}
	}
	bb.B = b

	s := bytesutil.ToUnsafeString(b)
	shard.uctx.resetFields()
	shard.uctx.addField(pf.resultField, s)
	bbPool.Put(bb)
}

func parsePipeFormat(lex *lexer) (*pipeFormat, error) {
	if !lex.isKeyword("format") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "format")
	}
	lex.nextToken()

	// parse format
	formatStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'format': %w", err)
	}
	steps, err := parsePatternSteps(formatStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", formatStr, err)
	}

	if !lex.isKeyword("as") {
		return nil, fmt.Errorf("missing 'as' keyword after 'format %q'", formatStr)
	}
	lex.nextToken()

	resultField, err := parseFieldName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result field after 'format %q as': %w", formatStr, err)
	}

	pf := &pipeFormat{
		formatStr:   formatStr,
		steps:       steps,
		resultField: resultField,
	}

	// parse optional if (...)
	if lex.isKeyword("if") {
		iff, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		pf.iff = iff
	}

	return pf, nil
}
