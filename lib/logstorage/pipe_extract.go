package logstorage

import (
	"fmt"
	"unsafe"
)

// pipeExtract processes '| extract from <field> <pattern>' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe
type pipeExtract struct {
	fromField string
	steps     []patternStep

	pattern string

	// iff is an optional filter for skipping the extract func
	iff             filter
	iffNeededFields []string
}

func (pe *pipeExtract) String() string {
	s := "extract"
	if !isMsgFieldName(pe.fromField) {
		s += " from " + quoteTokenIfNeeded(pe.fromField)
	}
	s += " " + quoteTokenIfNeeded(pe.pattern)
	if pe.iff != nil {
		s += " if (" + pe.iff.String() + ")"
	}
	return s
}

func (pe *pipeExtract) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		needFromField := false
		for _, step := range pe.steps {
			if step.field != "" {
				if !unneededFieldsOrig.contains(step.field) {
					needFromField = true
				}
				unneededFields.add(step.field)
			}
		}
		if needFromField {
			unneededFields.removeFields(pe.iffNeededFields)
			unneededFields.remove(pe.fromField)
		} else {
			unneededFields.add(pe.fromField)
		}
	} else {
		neededFieldsOrig := neededFields.clone()
		needFromField := false
		for _, step := range pe.steps {
			if step.field != "" && neededFieldsOrig.contains(step.field) {
				needFromField = true
				neededFields.remove(step.field)
			}
		}
		if needFromField {
			neededFields.addFields(pe.iffNeededFields)
			neededFields.add(pe.fromField)
		}
	}
}

func (pe *pipeExtract) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeExtractProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeExtractProcessorShard{
			pipeExtractProcessorShardNopad: pipeExtractProcessorShardNopad{
				ef: newPattern(pe.steps),
			},
		}
	}

	pep := &pipeExtractProcessor{
		pe:     pe,
		ppBase: ppBase,

		shards: shards,
	}
	return pep
}

type pipeExtractProcessor struct {
	pe     *pipeExtract
	ppBase pipeProcessor

	shards []pipeExtractProcessorShard
}

type pipeExtractProcessorShard struct {
	pipeExtractProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeExtractProcessorShardNopad{})%128]byte
}

type pipeExtractProcessorShardNopad struct {
	ef *pattern

	bm bitmap

	uctx fieldsUnpackerContext
	wctx pipeUnpackWriteContext
}

func (pep *pipeExtractProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pep.shards[workerID]
	shard.wctx.init(br, pep.ppBase)
	ef := shard.ef

	bm := &shard.bm
	bm.init(len(br.timestamps))
	bm.setBits()
	if iff := pep.pe.iff; iff != nil {
		iff.applyToBlockResult(br, bm)
		if bm.isZero() {
			// Fast path - nothing to extract.
			pep.ppBase.writeBlock(workerID, br)
			return
		}
	}

	c := br.getColumnByName(pep.pe.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		ef.apply(v)
		for i := range br.timestamps {
			shard.uctx.resetFields()
			if bm.isSetBit(i) {
				for _, f := range ef.fields {
					shard.uctx.addField(f.name, *f.value, "")
				}
			}
			shard.wctx.writeRow(i, shard.uctx.fields)

		}
	} else {
		values := c.getValues(br)
		vPrevApplied := ""
		for i, v := range values {
			shard.uctx.resetFields()
			if bm.isSetBit(i) {
				if vPrevApplied != v {
					ef.apply(v)
					vPrevApplied = v
				}
				for _, f := range ef.fields {
					shard.uctx.addField(f.name, *f.value, "")
				}
			}
			shard.wctx.writeRow(i, shard.uctx.fields)
		}
	}

	shard.wctx.flush()
	shard.uctx.reset()
}

func (pep *pipeExtractProcessor) flush() error {
	return nil
}

func parsePipeExtract(lex *lexer) (*pipeExtract, error) {
	if !lex.isKeyword("extract") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "extract")
	}
	lex.nextToken()

	fromField := "_msg"
	if lex.isKeyword("from") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field name: %w", err)
		}
		fromField = f
	}

	// parse pattern
	pattern, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'pattern': %w", err)
	}
	steps, err := parsePatternSteps(pattern)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", pattern, err)
	}

	pe := &pipeExtract{
		fromField: fromField,
		steps:     steps,
		pattern:   pattern,
	}

	// parse optional if (...)
	if lex.isKeyword("if") {
		iff, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		pe.iff = iff

		neededFields := newFieldsSet()
		iff.updateNeededFields(neededFields)
		pe.iffNeededFields = neededFields.getAll()
	}

	return pe, nil
}
