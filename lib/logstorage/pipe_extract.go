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
}

func (pe *pipeExtract) String() string {
	s := "extract"
	if !isMsgFieldName(pe.fromField) {
		s += " from " + quoteTokenIfNeeded(pe.fromField)
	}
	s += " " + quoteTokenIfNeeded(pe.pattern)
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
			unneededFields.remove(pe.fromField)
		} else {
			unneededFields.add(pe.fromField)
		}
	} else {
		needFromField := false
		for _, step := range pe.steps {
			if step.field != "" && neededFields.contains(step.field) {
				needFromField = true
				neededFields.remove(step.field)
			}
		}
		if needFromField {
			neededFields.add(pe.fromField)
		}
	}
}

func (pe *pipeExtract) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	shards := make([]pipeExtractProcessorShard, workersCount)
	for i := range shards {
		ef := newPattern(pe.steps)
		rcs := make([]resultColumn, len(ef.fields))
		for j := range rcs {
			rcs[j].name = ef.fields[j].name
		}
		shards[i] = pipeExtractProcessorShard{
			pipeExtractProcessorShardNopad: pipeExtractProcessorShardNopad{
				ef:  ef,
				rcs: rcs,
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

	rcs []resultColumn
}

func (pep *pipeExtractProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pep.shards[workerID]
	ef := shard.ef
	rcs := shard.rcs

	c := br.getColumnByName(pep.pe.fromField)
	if c.isConst {
		v := c.valuesEncoded[0]
		ef.apply(v)
		for i, f := range ef.fields {
			fieldValue := *f.value
			rc := &rcs[i]
			for range br.timestamps {
				rc.addValue(fieldValue)
			}
		}
	} else {
		values := c.getValues(br)
		for i, v := range values {
			if i == 0 || values[i-1] != v {
				ef.apply(v)
			}
			for j, f := range ef.fields {
				rcs[j].addValue(*f.value)
			}
		}
	}

	br.addResultColumns(rcs)
	pep.ppBase.writeBlock(workerID, br)

	for i := range rcs {
		rcs[i].resetValues()
	}
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
	return pe, nil
}
