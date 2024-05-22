package logstorage

import (
	"fmt"
)

// pipeExtract processes '| extract from <field> <pattern>' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#extract-pipe
type pipeExtract struct {
	fromField string
	ptn       *pattern

	patternStr string

	// iff is an optional filter for skipping the extract func
	iff *ifFilter
}

func (pe *pipeExtract) String() string {
	s := "extract"
	if !isMsgFieldName(pe.fromField) {
		s += " from " + quoteTokenIfNeeded(pe.fromField)
	}
	s += " " + quoteTokenIfNeeded(pe.patternStr)
	if pe.iff != nil {
		s += " " + pe.iff.String()
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
				unneededFields.add(step.field)
			}
		}
		if needFromField {
			if pe.iff != nil {
				unneededFields.removeFields(pe.iff.neededFields)
			}
			unneededFields.remove(pe.fromField)
		} else {
			unneededFields.add(pe.fromField)
		}
	} else {
		neededFieldsOrig := neededFields.clone()
		needFromField := false
		for _, step := range pe.ptn.steps {
			if step.field != "" && neededFieldsOrig.contains(step.field) {
				needFromField = true
				neededFields.remove(step.field)
			}
		}
		if needFromField {
			if pe.iff != nil {
				neededFields.addFields(pe.iff.neededFields)
			}
			neededFields.add(pe.fromField)
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

	return newPipeUnpackProcessor(workersCount, unpackFunc, ppBase, pe.fromField, "", pe.iff)
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
	patternStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot read 'pattern': %w", err)
	}
	ptn, err := parsePattern(patternStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'pattern' %q: %w", patternStr, err)
	}

	pe := &pipeExtract{
		fromField:  fromField,
		ptn:        ptn,
		patternStr: patternStr,
	}

	// parse optional if (...)
	if lex.isKeyword("if") {
		iff, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		pe.iff = iff
	}

	return pe, nil
}
