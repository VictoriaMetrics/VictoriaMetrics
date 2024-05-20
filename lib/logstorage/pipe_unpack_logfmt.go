package logstorage

import (
	"fmt"
	"strings"
)

// pipeUnpackLogfmt processes '| unpack_logfmt ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_logfmt-pipe
type pipeUnpackLogfmt struct {
	fromField string

	resultPrefix string
}

func (pu *pipeUnpackLogfmt) String() string {
	s := "unpack_logfmt"
	if !isMsgFieldName(pu.fromField) {
		s += " from " + quoteTokenIfNeeded(pu.fromField)
	}
	if pu.resultPrefix != "" {
		s += " result_prefix " + quoteTokenIfNeeded(pu.resultPrefix)
	}
	return s
}

func (pu *pipeUnpackLogfmt) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.remove(pu.fromField)
	} else {
		neededFields.add(pu.fromField)
	}
}

func (pu *pipeUnpackLogfmt) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	return newPipeUnpackProcessor(workersCount, unpackLogfmt, ppBase, pu.fromField, pu.resultPrefix)
}

func unpackLogfmt(uctx *fieldsUnpackerContext, s, fieldPrefix string) {
	for {
		// Search for field name
		n := strings.IndexByte(s, '=')
		if n < 0 {
			// field name couldn't be read
			return
		}

		name := strings.TrimSpace(s[:n])
		s = s[n+1:]
		if len(s) == 0 {
			uctx.addField(name, "", fieldPrefix)
		}

		// Search for field value
		value, nOffset := tryUnquoteString(s)
		if nOffset >= 0 {
			uctx.addField(name, value, fieldPrefix)
			s = s[nOffset:]
			if len(s) == 0 {
				return
			}
			if s[0] != ' ' {
				return
			}
			s = s[1:]
		} else {
			n := strings.IndexByte(s, ' ')
			if n < 0 {
				uctx.addField(name, s, fieldPrefix)
				return
			}
			uctx.addField(name, s[:n], fieldPrefix)
			s = s[n+1:]
		}
	}
}

func parsePipeUnpackLogfmt(lex *lexer) (*pipeUnpackLogfmt, error) {
	if !lex.isKeyword("unpack_logfmt") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_logfmt")
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

	pu := &pipeUnpackLogfmt{
		fromField:    fromField,
		resultPrefix: resultPrefix,
	}
	return pu, nil
}
