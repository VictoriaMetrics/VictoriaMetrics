package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeUnpackJSON processes '| unpack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe
type pipeUnpackJSON struct {
	// fromField is the field to unpack json fields from
	fromField string

	// resultPrefix is prefix to add to unpacked field names
	resultPrefix string

	// iff is an optional filter for skipping unpacking json
	iff *ifFilter
}

func (pu *pipeUnpackJSON) String() string {
	s := "unpack_json"
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	if pu.iff != nil {
		s += " " + pu.iff.String()
	}
	return s
}

func (pu *pipeUnpackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.remove(pu.fromField)
		if pu.iff != nil {
			unneededFields.removeFields(pu.iff.neededFields)
		}
	} else {
		neededFields.add(pu.fromField)
		if pu.iff != nil {
			neededFields.addFields(pu.iff.neededFields)
		}
	}
}

func (pu *pipeUnpackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return newPipeUnpackProcessor(workersCount, unpackJSON, ppBase, pu.fromField, pu.resultPrefix, pu.iff)
}

func unpackJSON(uctx *fieldsUnpackerContext, s string) {
	if len(s) == 0 || s[0] != '{' {
		// This isn't a JSON object
		return
	}
	p := GetJSONParser()
	if err := p.ParseLogMessage(bytesutil.ToUnsafeBytes(s)); err == nil {
		for _, f := range p.Fields {
			uctx.addField(f.Name, f.Value)
		}
	}
	PutJSONParser(p)
}

func parsePipeUnpackJSON(lex *lexer) (*pipeUnpackJSON, error) {
	if !lex.isKeyword("unpack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_json")
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

	resultPrefix := ""
	if lex.isKeyword("result_prefix") {
		lex.nextToken()
		p, err := getCompoundToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'result_prefix': %w", err)
		}
		resultPrefix = p
	}

	pu := &pipeUnpackJSON{
		fromField:    fromField,
		resultPrefix: resultPrefix,
	}

	if lex.isKeyword("if") {
		iff, err := parseIfFilter(lex)
		if err != nil {
			return nil, err
		}
		pu.iff = iff
	}

	return pu, nil
}
