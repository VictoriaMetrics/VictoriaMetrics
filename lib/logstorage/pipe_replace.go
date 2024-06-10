package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeReplace processes '| replace ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#replace-pipe
type pipeReplace struct {
	field     string
	oldSubstr string
	newSubstr string

	// limit limits the number of replacements, which can be performed
	limit uint64

	// iff is an optional filter for skipping the replace operation
	iff *ifFilter
}

func (pr *pipeReplace) String() string {
	s := "replace"
	if pr.iff != nil {
		s += " " + pr.iff.String()
	}
	s += fmt.Sprintf(" (%s, %s)", quoteTokenIfNeeded(pr.oldSubstr), quoteTokenIfNeeded(pr.newSubstr))
	if pr.field != "_msg" {
		s += " at " + quoteTokenIfNeeded(pr.field)
	}
	if pr.limit > 0 {
		s += fmt.Sprintf(" limit %d", pr.limit)
	}
	return s
}

func (pr *pipeReplace) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForUpdatePipe(neededFields, unneededFields, pr.field, pr.iff)
}

func (pr *pipeReplace) optimize() {
	pr.iff.optimizeFilterIn()
}

func (pr *pipeReplace) hasFilterInWithQuery() bool {
	return pr.iff.hasFilterInWithQuery()
}

func (pr *pipeReplace) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pr.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	peNew := *pr
	peNew.iff = iffNew
	return &peNew, nil
}

func (pr *pipeReplace) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	updateFunc := func(a *arena, v string) string {
		bLen := len(a.b)
		a.b = appendReplace(a.b, v, pr.oldSubstr, pr.newSubstr, pr.limit)
		return bytesutil.ToUnsafeString(a.b[bLen:])
	}

	return newPipeUpdateProcessor(workersCount, updateFunc, ppNext, pr.field, pr.iff)
}

func parsePipeReplace(lex *lexer) (*pipeReplace, error) {
	if !lex.isKeyword("replace") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "replace")
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

	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '(' after 'replace'")
	}
	lex.nextToken()

	oldSubstr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse oldSubstr in 'replace': %w", err)
	}
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("missing ',' after 'replace(%q'", oldSubstr)
	}
	lex.nextToken()

	newSubstr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse newSubstr in 'replace(%q': %w", oldSubstr, err)
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after 'replace(%q, %q'", oldSubstr, newSubstr)
	}
	lex.nextToken()

	field := "_msg"
	if lex.isKeyword("at") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'at' field after 'replace(%q, %q)': %w", oldSubstr, newSubstr, err)
		}
		field = f
	}

	limit := uint64(0)
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' in 'replace'", lex.token)
		}
		lex.nextToken()
		limit = n
	}

	pr := &pipeReplace{
		field:     field,
		oldSubstr: oldSubstr,
		newSubstr: newSubstr,
		limit:     limit,
		iff:       iff,
	}

	return pr, nil
}

func appendReplace(dst []byte, s, oldSubstr, newSubstr string, limit uint64) []byte {
	if len(s) == 0 {
		return dst
	}
	if len(oldSubstr) == 0 {
		return append(dst, s...)
	}

	replacements := uint64(0)
	for {
		n := strings.Index(s, oldSubstr)
		if n < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:n]...)
		dst = append(dst, newSubstr...)
		s = s[n+len(oldSubstr):]
		replacements++
		if limit > 0 && replacements >= limit {
			return append(dst, s...)
		}
	}
}
