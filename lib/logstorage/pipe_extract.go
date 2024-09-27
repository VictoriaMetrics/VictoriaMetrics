package logstorage

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeExtract processes '| extract ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe
type pipeExtract struct {
	fromField string

	ptn        *pattern
	patternStr string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping the extract func
	iff *ifFilter
}

func (pe *pipeExtract) String() string {
	s := "extract"
	if pe.iff != nil {
		s += " " + pe.iff.String()
	}
	s += " " + quoteTokenIfNeeded(pe.patternStr)
	if !isMsgFieldName(pe.fromField) {
		s += " from " + quoteTokenIfNeeded(pe.fromField)
	}
	if pe.keepOriginalFields {
		s += " keep_original_fields"
	}
	if pe.skipEmptyResults {
		s += " skip_empty_results"
	}
	return s
}

func (pe *pipeExtract) canLiveTail() bool {
	return true
}

func (pe *pipeExtract) optimize() {
	pe.iff.optimizeFilterIn()
}

func (pe *pipeExtract) hasFilterInWithQuery() bool {
	return pe.iff.hasFilterInWithQuery()
}

func (pe *pipeExtract) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pe.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	peNew := *pe
	peNew.iff = iffNew
	return &peNew, nil
}

func (pe *pipeExtract) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		if pe.iff != nil {
			neededFields.addFields(pe.iff.neededFields)
		}
		return
	}

	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		needFromField := false
		for _, step := range pe.ptn.steps {
			if step.field == "" {
				continue
			}
			if !unneededFieldsOrig.contains(step.field) {
				needFromField = true
			}
			if !pe.keepOriginalFields && !pe.skipEmptyResults {
				unneededFields.add(step.field)
			}
		}
		if needFromField {
			unneededFields.remove(pe.fromField)
			if pe.iff != nil {
				unneededFields.removeFields(pe.iff.neededFields)
			}
		} else {
			unneededFields.add(pe.fromField)
		}
	} else {
		neededFieldsOrig := neededFields.clone()
		needFromField := false
		for _, step := range pe.ptn.steps {
			if step.field == "" {
				continue
			}
			if neededFieldsOrig.contains(step.field) {
				needFromField = true
				if !pe.keepOriginalFields && !pe.skipEmptyResults {
					neededFields.remove(step.field)
				}
			}
		}
		if needFromField {
			neededFields.add(pe.fromField)
			if pe.iff != nil {
				neededFields.addFields(pe.iff.neededFields)
			}
		}
	}
}

func (pe *pipeExtract) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeExtractProcessor{
		pe:     pe,
		ppNext: ppNext,

		shards: make([]pipeExtractProcessorShard, workersCount),
	}
}

type pipeExtractProcessor struct {
	pe     *pipeExtract
	ppNext pipeProcessor

	shards []pipeExtractProcessorShard
}

type pipeExtractProcessorShard struct {
	pipeExtractProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeExtractProcessorShardNopad{})%128]byte
}

type pipeExtractProcessorShardNopad struct {
	bm  bitmap
	ptn *pattern

	resultColumns []*blockResultColumn
	resultValues  []string

	rcs []resultColumn
	a   arena
}

func (pep *pipeExtractProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	pe := pep.pe
	shard := &pep.shards[workerID]

	bm := &shard.bm
	bm.init(br.rowsLen)
	bm.setBits()
	if iff := pe.iff; iff != nil {
		iff.f.applyToBlockResult(br, bm)
		if bm.isZero() {
			pep.ppNext.writeBlock(workerID, br)
			return
		}
	}

	if shard.ptn == nil {
		shard.ptn = pe.ptn.clone()
	}
	ptn := shard.ptn

	shard.rcs = slicesutil.SetLength(shard.rcs, len(ptn.fields))
	rcs := shard.rcs
	for i := range ptn.fields {
		rcs[i].name = ptn.fields[i].name
	}

	c := br.getColumnByName(pe.fromField)
	values := c.getValues(br)

	shard.resultColumns = slicesutil.SetLength(shard.resultColumns, len(rcs))
	resultColumns := shard.resultColumns
	for i := range resultColumns {
		resultColumns[i] = br.getColumnByName(rcs[i].name)
	}

	shard.resultValues = slicesutil.SetLength(shard.resultValues, len(rcs))
	resultValues := shard.resultValues

	hadUpdates := false
	vPrev := ""
	for rowIdx, v := range values {
		if bm.isSetBit(rowIdx) {
			if !hadUpdates || vPrev != v {
				vPrev = v
				hadUpdates = true

				ptn.apply(v)

				for i, f := range ptn.fields {
					v := *f.value
					if v == "" && pe.skipEmptyResults || pe.keepOriginalFields {
						c := resultColumns[i]
						if vOrig := c.getValueAtRow(br, rowIdx); vOrig != "" {
							v = vOrig
						}
					} else {
						v = shard.a.copyString(v)
					}
					resultValues[i] = v
				}
			}
		} else {
			for i, c := range resultColumns {
				resultValues[i] = c.getValueAtRow(br, rowIdx)
			}
		}

		for i, v := range resultValues {
			rcs[i].addValue(v)
		}
	}

	for i := range rcs {
		br.addResultColumn(&rcs[i])
	}
	pep.ppNext.writeBlock(workerID, br)

	for i := range rcs {
		rcs[i].reset()
	}
	shard.a.reset()
}

func (pep *pipeExtractProcessor) flush() error {
	return nil
}

func parsePipeExtract(lex *lexer) (*pipeExtract, error) {
	if !lex.isKeyword("extract") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "extract")
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

	// parse pattern
	patternStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'pattern': %w", err)
	}
	ptn, err := parsePattern(patternStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", patternStr, err)
	}

	// parse optional 'from ...' part
	fromField := "_msg"
	if lex.isKeyword("from") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field name: %w", err)
		}
		fromField = f
	}

	keepOriginalFields := false
	skipEmptyResults := false
	switch {
	case lex.isKeyword("keep_original_fields"):
		lex.nextToken()
		keepOriginalFields = true
	case lex.isKeyword("skip_empty_results"):
		lex.nextToken()
		skipEmptyResults = true
	}

	pe := &pipeExtract{
		fromField:          fromField,
		ptn:                ptn,
		patternStr:         patternStr,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pe, nil
}
