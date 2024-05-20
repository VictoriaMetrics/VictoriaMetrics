package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeUnpackJSON processes '| unpack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_json-pipe
type pipeUnpackJSON struct {
	fromField string

	resultPrefix string
}

func (pu *pipeUnpackJSON) String() string {
	s := "unpack_json"
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	return s
}

func (pu *pipeUnpackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.remove(pu.fromField)
	} else {
		neededFields.add(pu.fromField)
	}
}

func (pu *pipeUnpackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return newPipeUnpackProcessor(workersCount, unpackJSON, ppBase, pu.fromField, pu.resultPrefix)
}

func unpackJSON(uctx *fieldsUnpackerContext, s, fieldPrefix string) {
	if len(s) == 0 || s[0] != '{' {
		// This isn't a JSON object
		return
	}
	p := GetJSONParser()
	if err := p.ParseLogMessage(bytesutil.ToUnsafeBytes(s)); err == nil {
		for _, f := range p.Fields {
			uctx.addField(f.Name, f.Value, fieldPrefix)
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
	return pu, nil
}
