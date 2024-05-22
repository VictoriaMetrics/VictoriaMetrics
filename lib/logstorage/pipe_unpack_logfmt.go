package logstorage

import (
	"fmt"
	"slices"
	"strings"
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
	return s
}

func (pu *pipeUnpackLogfmt) updateNeededFields(neededFields, unneededFields fieldsSet) {
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

func (pu *pipeUnpackLogfmt) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppBase pipeProcessor) pipeProcessor {
	addField := func(uctx *fieldsUnpackerContext, name, value string) {
		if len(pu.fields) == 0 || slices.Contains(pu.fields, name) {
			uctx.addField(name, value)
		}
	}

	unpackLogfmt := func(uctx *fieldsUnpackerContext, s string) {
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
				addField(uctx, name, "")
			}

			// Search for field value
			value, nOffset := tryUnquoteString(s)
			if nOffset >= 0 {
				addField(uctx, name, value)
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
					addField(uctx, name, s)
					return
				}
				addField(uctx, name, s[:n])
				s = s[n+1:]
			}
		}
	}

	return newPipeUnpackProcessor(workersCount, unpackLogfmt, ppBase, pu.fromField, pu.resultPrefix, pu.iff)

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

	pu := &pipeUnpackLogfmt{
		fromField:    fromField,
		fields:       fields,
		resultPrefix: resultPrefix,
		iff:          iff,
	}

	return pu, nil
}
