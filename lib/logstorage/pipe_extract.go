package logstorage

import (
	"fmt"
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

func (pe *pipeExtract) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFieldsOrig := unneededFields.clone()
		needFromField := false
		for _, step := range pe.ptn.steps {
			if step.field != "" {
				if !unneededFieldsOrig.contains(step.field) {
					needFromField = true
				}
				if !pe.keepOriginalFields && !pe.skipEmptyResults {
					unneededFields.add(step.field)
				}
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
			if step.field != "" && neededFieldsOrig.contains(step.field) {
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

func (pe *pipeExtract) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	patterns := make([]*pattern, workersCount)
	for i := range patterns {
		patterns[i] = pe.ptn.clone()
	}

	unpackFunc := func(uctx *fieldsUnpackerContext, s string) {
		ptn := patterns[uctx.workerID]
		ptn.apply(s)
		for _, f := range ptn.fields {
			uctx.addField(f.name, *f.value)
		}
	}

	return newPipeUnpackProcessor(workersCount, unpackFunc, ppBase, pe.fromField, "", pe.keepOriginalFields, pe.skipEmptyResults, pe.iff)
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
