package logstorage

import (
	"fmt"
	"slices"
)

// pipeUnpackLogfmt processes '| unpack_logfmt ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe
type pipeUnpackLogfmt struct {
	// fromField is the field to unpack logfmt fields from
	fromField string

	// fields is an optional list of fields to extract from logfmt.
	//
	// if it is empty, then all the fields are extracted.
	fields []string

	// resultPrefix is prefix to add to unpacked field names
	resultPrefix string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping unpacking logfmt
	iff *ifFilter
}

func (pu *pipeUnpackLogfmt) String() string {
	s := "unpack_logfmt"
	if pu.iff != nil {
		s += " " + pu.iff.String()
	}
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if len(pu.fields) > 0 {
		s += " fields (" + fieldsToString(pu.fields) + ")"
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	if pu.keepOriginalFields {
		s += " keep_original_fields"
	}
	if pu.skipEmptyResults {
		s += " skip_empty_results"
	}
	return s
}

func (pu *pipeUnpackLogfmt) canLiveTail() bool {
	return true
}

func (pu *pipeUnpackLogfmt) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForUnpackPipe(pu.fromField, pu.fields, pu.keepOriginalFields, pu.skipEmptyResults, pu.iff, neededFields, unneededFields)
}

func (pu *pipeUnpackLogfmt) optimize() {
	pu.iff.optimizeFilterIn()
}

func (pu *pipeUnpackLogfmt) hasFilterInWithQuery() bool {
	return pu.iff.hasFilterInWithQuery()
}

func (pu *pipeUnpackLogfmt) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pu.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	puNew := *pu
	puNew.iff = iffNew
	return &puNew, nil
}

func (pu *pipeUnpackLogfmt) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	unpackLogfmt := func(uctx *fieldsUnpackerContext, s string) {
		p := getLogfmtParser()

		p.parse(s)
		if len(pu.fields) == 0 {
			for _, f := range p.fields {
				uctx.addField(f.Name, f.Value)
			}
		} else {
			for _, fieldName := range pu.fields {
				addedField := false
				for _, f := range p.fields {
					if f.Name == fieldName {
						uctx.addField(f.Name, f.Value)
						addedField = true
						break
					}
				}
				if !addedField {
					uctx.addField(fieldName, "")
				}
			}
		}

		putLogfmtParser(p)
	}

	return newPipeUnpackProcessor(workersCount, unpackLogfmt, ppNext, pu.fromField, pu.resultPrefix, pu.keepOriginalFields, pu.skipEmptyResults, pu.iff)
}

func parsePipeUnpackLogfmt(lex *lexer) (*pipeUnpackLogfmt, error) {
	if !lex.isKeyword("unpack_logfmt") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_logfmt")
	}
	lex.nextToken()

	var iff *ifFilter
	if lex.isKeyword("if") {
		f, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		iff = f
	}

	fromField := "_msg"
	if lex.isKeyword("from") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'from' field name: %w", err)
		}
		fromField = f
	}

	var fields []string
	if lex.isKeyword("fields") {
		lex.nextToken()
		fs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'fields': %w", err)
		}
		fields = fs
		if slices.Contains(fields, "*") {
			fields = nil
		}
	}

	resultPrefix := ""
	if lex.isKeyword("result_prefix") {
		lex.nextToken()
		p, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'result_prefix': %w", err)
		}
		resultPrefix = p
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

	pu := &pipeUnpackLogfmt{
		fromField:          fromField,
		fields:             fields,
		resultPrefix:       resultPrefix,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pu, nil
}
