package logstorage

import (
	"fmt"
	"regexp"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeExtractRegexp processes '| extract_regexp ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#extract_regexp-pipe
type pipeExtractRegexp struct {
	fromField string

	re       *regexp.Regexp
	reFields []string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping the extract func
	iff *ifFilter
}

func (pe *pipeExtractRegexp) String() string {
	s := "extract_regexp"
	if pe.iff != nil {
		s += " " + pe.iff.String()
	}
	reStr := pe.re.String()
	s += " " + quoteTokenIfNeeded(reStr)
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

func (pe *pipeExtractRegexp) canLiveTail() bool {
	return true
}

func (pe *pipeExtractRegexp) optimize() {
	pe.iff.optimizeFilterIn()
}

func (pe *pipeExtractRegexp) hasFilterInWithQuery() bool {
	return pe.iff.hasFilterInWithQuery()
}

func (pe *pipeExtractRegexp) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pe.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	peNew := *pe
	peNew.iff = iffNew
	return &peNew, nil
}

func (pe *pipeExtractRegexp) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		if pe.iff != nil {
			neededFields.addFields(pe.iff.neededFields)
		}
		return
	}

	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		needFromField := false
		for _, f := range pe.reFields {
			if f == "" {
				continue
			}
			if !unneededFieldsOrig.contains(f) {
				needFromField = true
			}
			if !pe.keepOriginalFields && !pe.skipEmptyResults {
				unneededFields.add(f)
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
		for _, f := range pe.reFields {
			if f == "" {
				continue
			}
			if neededFieldsOrig.contains(f) {
				needFromField = true
				if !pe.keepOriginalFields && !pe.skipEmptyResults {
					neededFields.remove(f)
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

func (pe *pipeExtractRegexp) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeExtractRegexpProcessor{
		pe:     pe,
		ppNext: ppNext,

		shards: make([]pipeExtractRegexpProcessorShard, workersCount),
	}
}

type pipeExtractRegexpProcessor struct {
	pe     *pipeExtractRegexp
	ppNext pipeProcessor

	shards []pipeExtractRegexpProcessorShard
}

type pipeExtractRegexpProcessorShard struct {
	pipeExtractRegexpProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeExtractRegexpProcessorShardNopad{})%128]byte
}

func (shard *pipeExtractRegexpProcessorShard) apply(re *regexp.Regexp, v string) {
	shard.fields = slicesutil.SetLength(shard.fields, len(shard.rcs))
	fields := shard.fields
	clear(fields)

	locs := re.FindStringSubmatchIndex(v)
	if locs == nil {
		return
	}

	for i := range fields {
		start := locs[2*i]
		if start < 0 {
			// mismatch
			continue
		}
		end := locs[2*i+1]
		fields[i] = v[start:end]
	}
}

type pipeExtractRegexpProcessorShardNopad struct {
	bm bitmap

	resultColumns []*blockResultColumn
	resultValues  []string

	rcs []resultColumn
	a   arena

	fields []string
}

func (pep *pipeExtractRegexpProcessor) writeBlock(workerID uint, br *blockResult) {
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

	reFields := pe.reFields

	shard.rcs = slicesutil.SetLength(shard.rcs, len(reFields))
	rcs := shard.rcs
	for i := range reFields {
		rcs[i].name = reFields[i]
	}

	c := br.getColumnByName(pe.fromField)
	values := c.getValues(br)

	shard.resultColumns = slicesutil.SetLength(shard.resultColumns, len(rcs))
	resultColumns := shard.resultColumns
	for i := range resultColumns {
		if reFields[i] != "" {
			resultColumns[i] = br.getColumnByName(rcs[i].name)
		}
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

				shard.apply(pe.re, v)

				for i, v := range shard.fields {
					if reFields[i] == "" {
						continue
					}
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
				if reFields[i] != "" {
					resultValues[i] = c.getValueAtRow(br, rowIdx)
				}
			}
		}

		for i, v := range resultValues {
			if reFields[i] != "" {
				rcs[i].addValue(v)
			}
		}
	}

	for i := range rcs {
		if reFields[i] != "" {
			br.addResultColumn(&rcs[i])
		}
	}
	pep.ppNext.writeBlock(workerID, br)

	for i := range rcs {
		rcs[i].reset()
	}
	shard.a.reset()
}

func (pep *pipeExtractRegexpProcessor) flush() error {
	return nil
}

func parsePipeExtractRegexp(lex *lexer) (*pipeExtractRegexp, error) {
	if !lex.isKeyword("extract_regexp") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "extract_regexp")
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
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", patternStr, err)
	}
	reFields := re.SubexpNames()

	hasNamedFields := false
	for _, f := range reFields {
		if f != "" {
			hasNamedFields = true
			break
		}
	}
	if !hasNamedFields {
		return nil, fmt.Errorf("the 'pattern' %q must contain at least a single named group in the form (?P<group_name>...)", patternStr)
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

	pe := &pipeExtractRegexp{
		fromField:          fromField,
		re:                 re,
		reFields:           reFields,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pe, nil
}
