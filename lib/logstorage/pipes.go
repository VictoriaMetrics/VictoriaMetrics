package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type pipe interface {
	String() string
}

func parsePipes(lex *lexer) ([]pipe, error) {
	var pipes []pipe
	for !lex.isEnd() {
		if !lex.isKeyword("|") {
			return nil, fmt.Errorf("expecting '|'")
		}
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing token after '|'")
		}
		switch {
		case lex.isKeyword("fields"):
			fp, err := parseFieldsPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'fields' pipe: %w", err)
			}
			pipes = append(pipes, fp)
		case lex.isKeyword("stats"):
			sp, err := parseStatsPipe(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'stats' pipe: %w", err)
			}
			pipes = append(pipes, sp)
		default:
			return nil, fmt.Errorf("unexpected pipe %q", lex.token)
		}
	}
	return pipes, nil
}

type fieldsPipe struct {
	// fields contains list of fields to fetch
	fields []string
}

func (fp *fieldsPipe) String() string {
	if len(fp.fields) == 0 {
		logger.Panicf("BUG: fieldsPipe must contain at least a single field")
	}
	return "fields " + fieldNamesString(fp.fields)
}

func parseFieldsPipe(lex *lexer) (*fieldsPipe, error) {
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name")
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected ','; expecting field name")
		}
		field := parseFieldName(lex)
		fields = append(fields, field)
		switch {
		case lex.isKeyword("|", ""):
			fp := &fieldsPipe{
				fields: fields,
			}
			return fp, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',' or '|'", lex.token)
		}
	}
}

type statsPipe struct {
	byFields []string
	funcs    []statsFunc
}

type statsFunc interface {
	// String returns string representation of statsFunc
	String() string

	// neededFields returns the needed fields for calculating the given stats
	neededFields() []string
}

func (sp *statsPipe) String() string {
	s := "stats "
	if len(sp.byFields) > 0 {
		s += "by (" + fieldNamesString(sp.byFields) + ") "
	}

	if len(sp.funcs) == 0 {
		logger.Panicf("BUG: statsPipe must contain at least a single statsFunc")
	}
	a := make([]string, len(sp.funcs))
	for i, f := range sp.funcs {
		a[i] = f.String()
	}
	s += strings.Join(a, ", ")
	return s
}

func (sp *statsPipe) neededFields() []string {
	var neededFields []string
	m := make(map[string]struct{})
	updateNeededFields := func(fields []string) {
		for _, field := range fields {
			if _, ok := m[field]; !ok {
				m[field] = struct{}{}
				neededFields = append(neededFields, field)
			}
		}
	}

	updateNeededFields(sp.byFields)

	for _, f := range sp.funcs {
		fields := f.neededFields()
		updateNeededFields(fields)
	}

	return neededFields
}

func parseStatsPipe(lex *lexer) (*statsPipe, error) {
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing stats config")
	}

	var sp statsPipe
	if lex.isKeyword("by") {
		lex.nextToken()
		fields, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by': %w", err)
		}
		sp.byFields = fields
	}

	var funcs []statsFunc
	for {
		sf, err := parseStatsFunc(lex)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, sf)
		if lex.isKeyword("|", "") {
			sp.funcs = funcs
			return &sp, nil
		}
		if !lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected token %q; want ',' or '|'", lex.token)
		}
		lex.nextToken()
	}
}

func parseStatsFunc(lex *lexer) (statsFunc, error) {
	switch {
	case lex.isKeyword("count"):
		sfc, err := parseStatsFuncCount(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'count' func: %w", err)
		}
		return sfc, nil
	default:
		return nil, fmt.Errorf("unknown stats func %q", lex.token)
	}
}

type statsFuncCount struct {
	fields     []string
	resultName string
}

func (sfc *statsFuncCount) String() string {
	fields := getFieldsIgnoreStar(sfc.fields)
	return "count(" + fieldNamesString(fields) + ") as " + quoteTokenIfNeeded(sfc.resultName)
}

func (sfc *statsFuncCount) neededFields() []string {
	return getFieldsIgnoreStar(sfc.fields)
}

func parseStatsFuncCount(lex *lexer) (*statsFuncCount, error) {
	lex.nextToken()
	fields, err := parseFieldNamesInParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'count' args: %w", err)
	}

	if !lex.isKeyword("as") {
		return nil, fmt.Errorf("missing 'as' keyword")
	}
	if !lex.mustNextToken() {
		return nil, fmt.Errorf("missing token after 'as' keyword")
	}
	resultName := parseFieldName(lex)

	sfc := &statsFuncCount{
		fields:     fields,
		resultName: resultName,
	}
	return sfc, nil
}

func parseFieldNamesInParens(lex *lexer) ([]string, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing `(`")
	}
	var fields []string
	for {
		if !lex.mustNextToken() {
			return nil, fmt.Errorf("missing field name or ')'")
		}
		if lex.isKeyword(")") {
			lex.nextToken()
			return fields, nil
		}
		if lex.isKeyword(",") {
			return nil, fmt.Errorf("unexpected `,`")
		}
		field := parseFieldName(lex)
		fields = append(fields, field)
		switch {
		case lex.isKeyword(")"):
			lex.nextToken()
			return fields, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',' or ')'", lex.token)
		}
	}
}

func parseFieldName(lex *lexer) string {
	s := lex.token
	lex.nextToken()
	for !lex.isSkippedSpace && !lex.isKeyword(",", "|", ")", "") {
		s += lex.rawToken
		lex.nextToken()
	}
	return s
}

func fieldNamesString(fields []string) string {
	a := make([]string, len(fields))
	for i, f := range fields {
		if f != "*" {
			f = quoteTokenIfNeeded(f)
		}
		a[i] = f
	}
	return strings.Join(a, ", ")
}

func getFieldsIgnoreStar(fields []string) []string {
	var result []string
	for _, f := range fields {
		if f != "*" {
			result = append(result, f)
		}
	}
	return result
}
