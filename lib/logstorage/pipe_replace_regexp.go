package logstorage

import (
	"fmt"
	"regexp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipeReplaceRegexp processes '| replace_regexp ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#replace_regexp-pipe
type pipeReplaceRegexp struct {
	field       string
	re          *regexp.Regexp
	replacement string

	// limit limits the number of replacements, which can be performed
	limit uint64

	// iff is an optional filter for skipping the replace_regexp operation
	iff *ifFilter
}

func (pr *pipeReplaceRegexp) String() string {
	s := "replace_regexp"
	if pr.iff != nil {
		s += " " + pr.iff.String()
	}
	s += fmt.Sprintf(" (%s, %s)", quoteTokenIfNeeded(pr.re.String()), quoteTokenIfNeeded(pr.replacement))
	if pr.field != "_msg" {
		s += " at " + quoteTokenIfNeeded(pr.field)
	}
	if pr.limit > 0 {
		s += fmt.Sprintf(" limit %d", pr.limit)
	}
	return s
}

func (pr *pipeReplaceRegexp) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pr, nil
}

func (pr *pipeReplaceRegexp) canLiveTail() bool {
	return true
}

func (pr *pipeReplaceRegexp) updateNeededFields(pf *prefixfilter.Filter) {
	updateNeededFieldsForUpdatePipe(pf, pr.field, pr.iff)
}

func (pr *pipeReplaceRegexp) hasFilterInWithQuery() bool {
	return pr.iff.hasFilterInWithQuery()
}

func (pr *pipeReplaceRegexp) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	iffNew, err := pr.iff.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	peNew := *pr
	peNew.iff = iffNew
	return &peNew, nil
}

func (pr *pipeReplaceRegexp) visitSubqueries(visitFunc func(q *Query)) {
	pr.iff.visitSubqueries(visitFunc)
}

func (pr *pipeReplaceRegexp) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	updateFunc := func(a *arena, v string) string {
		bLen := len(a.b)
		a.b = appendReplaceRegexp(a.b, v, pr.re, pr.replacement, pr.limit)
		return bytesutil.ToUnsafeString(a.b[bLen:])
	}

	return newPipeUpdateProcessor(updateFunc, ppNext, pr.field, pr.iff)

}

func parsePipeReplaceRegexp(lex *lexer) (pipe, error) {
	if !lex.isKeyword("replace_regexp") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "replace_regexp")
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
		return nil, fmt.Errorf("missing '(' after 'replace_regexp'")
	}
	lex.nextToken()

	reStr, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse reStr in 'replace_regexp': %w", err)
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse regexp %q in 'replace_regexp': %w", reStr, err)
	}
	if !lex.isKeyword(",") {
		return nil, fmt.Errorf("missing ',' after 'replace_regexp(%q'", reStr)
	}
	lex.nextToken()

	replacement, err := getCompoundToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse replacement in 'replace_regexp(%q': %w", reStr, err)
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')' after 'replace_regexp(%q, %q'", reStr, replacement)
	}
	lex.nextToken()

	field := "_msg"
	if lex.isKeyword("at") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'at' field after 'replace_regexp(%q, %q)': %w", reStr, replacement, err)
		}
		field = f
	}

	limit := uint64(0)
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s' in 'replace_regexp'", lex.token)
		}
		lex.nextToken()
		limit = n
	}

	pr := &pipeReplaceRegexp{
		field:       field,
		re:          re,
		replacement: replacement,
		limit:       limit,
		iff:         iff,
	}

	return pr, nil
}

func appendReplaceRegexp(dst []byte, s string, re *regexp.Regexp, replacement string, limit uint64) []byte {
	if len(s) == 0 {
		return dst
	}

	var matches [][]int
	if limit > 0 {
		matches = re.FindAllStringSubmatchIndex(s, int(limit))
	} else {
		matches = re.FindAllStringSubmatchIndex(s, -1)
	}

	if len(matches) == 0 {
		return append(dst, s...)
	}

	prevEnd := 0
	for _, locs := range matches {
		start := locs[0]
		end := locs[1]

		dst = append(dst, s[prevEnd:start]...)
		dst = re.ExpandString(dst, replacement, s, locs)

		prevEnd = end
	}

	// Append the rest of the string after the last match
	return append(dst, s[prevEnd:]...)
}
