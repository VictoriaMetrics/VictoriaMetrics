package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeUnpackJSON processes '| unpack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe
type pipeUnpackJSON struct {
	// fromField is the field to unpack json fields from
	fromField string

	// fields is an optional list of fields to extract from json.
	//
	// if it is empty, then all the fields are extracted.
	fields []string

	// resultPrefix is prefix to add to unpacked field names
	resultPrefix string

	keepOriginalFields bool
	skipEmptyResults   bool

	// iff is an optional filter for skipping unpacking json
	iff *ifFilter
}

func (pu *pipeUnpackJSON) String() string {
	s := "unpack_json"
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

func (pu *pipeUnpackJSON) canLiveTail() bool {
	return true
}

func (pu *pipeUnpackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForUnpackPipe(pu.fromField, pu.fields, pu.keepOriginalFields, pu.skipEmptyResults, pu.iff, neededFields, unneededFields)
}

func (pu *pipeUnpackJSON) optimize() {
	pu.iff.optimizeFilterIn()
}

func (pu *pipeUnpackJSON) hasFilterInWithQuery() bool {
	return pu.iff.hasFilterInWithQuery()
}

func (pu *pipeUnpackJSON) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pu.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	puNew := *pu
	puNew.iff = iffNew
	return &puNew, nil
}

func (pu *pipeUnpackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	unpackJSON := func(uctx *fieldsUnpackerContext, s string) {
		if len(s) == 0 || s[0] != '{' {
			// This isn't a JSON object
			return
		}
		p := GetJSONParser()
		err := p.ParseLogMessage(bytesutil.ToUnsafeBytes(s))
		if err != nil {
			for _, fieldName := range pu.fields {
				uctx.addField(fieldName, "")
			}
		} else {
			if len(pu.fields) == 0 {
				for _, f := range p.Fields {
					uctx.addField(f.Name, f.Value)
				}
			} else {
				for _, fieldName := range pu.fields {
					addedField := false
					for _, f := range p.Fields {
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
		}
		PutJSONParser(p)
	}
	return newPipeUnpackProcessor(workersCount, unpackJSON, ppNext, pu.fromField, pu.resultPrefix, pu.keepOriginalFields, pu.skipEmptyResults, pu.iff)
}

func parsePipeUnpackJSON(lex *lexer) (*pipeUnpackJSON, error) {
	if !lex.isKeyword("unpack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_json")
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

	pu := &pipeUnpackJSON{
		fromField:          fromField,
		fields:             fields,
		resultPrefix:       resultPrefix,
		keepOriginalFields: keepOriginalFields,
		skipEmptyResults:   skipEmptyResults,
		iff:                iff,
	}

	return pu, nil
}
