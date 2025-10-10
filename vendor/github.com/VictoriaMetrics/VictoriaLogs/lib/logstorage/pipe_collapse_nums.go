package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeCollapseNums processes '| collapse_nums ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#collapse_nums-pipe
type pipeCollapseNums struct {
	// the field to collapse nums at
	field string

	// if isPrettify is set, then collapsed nums are prettified with common placeholders
	isPrettify bool

	// iff is an optional filter for skipping the collapse_nums operation
	iff *ifFilter
}

func (pc *pipeCollapseNums) String() string {
	s := "collapse_nums"
	if pc.iff != nil {
		s += " " + pc.iff.String()
	}
	if pc.field != "_msg" {
		s += " at " + quoteTokenIfNeeded(pc.field)
	}
	if pc.isPrettify {
		s += " prettify"
	}
	return s
}

func (pc *pipeCollapseNums) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pc, nil
}

func (pc *pipeCollapseNums) canLiveTail() bool {
	return true
}

func (pc *pipeCollapseNums) canReturnLastNResults() bool {
	return true
}

func (pc *pipeCollapseNums) updateNeededFields(pf *prefixfilter.Filter) {
	updateNeededFieldsForUpdatePipe(pf, pc.field, pc.iff)
}

func (pc *pipeCollapseNums) hasFilterInWithQuery() bool {
	return pc.iff.hasFilterInWithQuery()
}

func (pc *pipeCollapseNums) visitSubqueries(visitFunc func(q *Query)) {
	pc.iff.visitSubqueries(visitFunc)
}

func (pc *pipeCollapseNums) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (pipe, error) {
	iffNew, err := pc.iff.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}
	pcNew := *pc
	pcNew.iff = iffNew
	return &pcNew, nil
}

func (pc *pipeCollapseNums) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	updateFunc := func(a *arena, v string) string {
		bLen := len(a.b)
		a.b = appendCollapseNums(a.b, v)
		if pc.isPrettify {
			a.b = appendPrettifyCollapsedNums(a.b[:bLen], a.b[bLen:])
		}
		return bytesutil.ToUnsafeString(a.b[bLen:])
	}

	return newPipeUpdateProcessor(updateFunc, ppNext, pc.field, pc.iff)
}

func parsePipeCollapseNums(lex *lexer) (pipe, error) {
	if !lex.isKeyword("collapse_nums") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "collapse_nums")
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

	field := "_msg"
	if lex.isKeyword("at") {
		lex.nextToken()
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'at' field after 'collapse_nums': %w", err)
		}
		field = f
	}

	isPrettify := false
	if lex.isKeyword("prettify") {
		lex.nextToken()
		isPrettify = true
	}

	pc := &pipeCollapseNums{
		field:      field,
		isPrettify: isPrettify,
		iff:        iff,
	}

	return pc, nil
}

func appendCollapseNums(dst []byte, s string) []byte {
	offset := 0
	for offset < len(s) {
		numStart := indexNumStart(s, offset)
		if numStart < 0 {
			return append(dst, s[offset:]...)
		}
		dst = append(dst, s[offset:numStart]...)

		numEnd := indexNumEnd(s, numStart)
		if !isValidNum(s, numStart, numEnd) {
			dst = append(dst, s[numStart:numEnd]...)
		} else {
			dst = append(dst, "<N>"...)
		}
		offset = numEnd
	}
	return dst
}

func indexNumStart(s string, offset int) int {
	// It is safe iterating by chars instead of Unicode runes, since decimal and hex chars are ASCII
	// and they cannot clash with utf-8 encoded Unicode runes.
	n := offset
	for n < len(s) {
		if !isDecimalOrHexChar(s[n]) {
			n++
			continue
		}
		if n == 0 {
			return 0
		}
		if !isTokenChar(s[n-1]) || isSpecialNumStart(s[n-1]) {
			return n
		}
		n++
	}
	return -1
}

func indexNumEnd(s string, offset int) int {
	// It is safe iterating by chars instead of Unicode runes, since decimal and hex chars are ASCII
	// and they cannot clash with utf-8 encoded Unicode runes.
	n := offset
	for n < len(s) && isDecimalOrHexChar(s[n]) {
		n++
	}
	return n
}

func isValidNum(s string, start, end int) bool {
	if end < len(s) && isTokenChar(s[end]) && !isSpecialNumEnd(s[end]) {
		return false
	}
	return canBeTreatedAsNum(s[start:end])
}

func appendPrettifyCollapsedNums(dst, src []byte) []byte {
	dstLen := len(dst)
	dst = appendReplaceWithSkipTail(dst, src, "<N>-<N>-<N>-<N>-<N>", "<UUID>", nil)

	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<N>.<N>.<N>.<N>", "<IP4>", nil)
	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<N>:<N>:<N>", "<TIME>", skipTrailingSubsecs)
	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<N>-<N>-<N>", "<DATE>", nil)
	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<N>/<N>/<N>", "<DATE>", nil)
	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<DATE>T<TIME>", "<DATETIME>", skipTrailingTimezone)
	dst = appendReplaceWithSkipTail(dst[:dstLen], dst[dstLen:], "<DATE> <TIME>", "<DATETIME>", skipTrailingTimezone)

	return dst
}

func appendReplaceWithSkipTail(dst, src []byte, old, replacement string, skipTail func(s string) int) []byte {
	if len(replacement) > len(old) {
		panic(fmt.Errorf("BUG: len(replacement)=%d cannot exceed len(old)=%d", len(replacement), len(old)))
	}
	s := bytesutil.ToUnsafeString(src)
	offset := 0
	for offset < len(s) {
		n := strings.Index(s[offset:], old)
		if n < 0 {
			break
		}
		dst = append(dst, s[offset:offset+n]...)
		dst = append(dst, replacement...)
		offset += n + len(old)
		if skipTail != nil {
			offset += skipTail(s[offset:])
		}
	}
	return append(dst, s[offset:]...)
}

func skipTrailingSubsecs(s string) int {
	if strings.HasPrefix(s, ".<N>") || strings.HasPrefix(s, ",<N>") {
		return len(".<N>")
	}
	return 0
}

func skipTrailingTimezone(s string) int {
	if strings.HasPrefix(s, "Z") {
		return 1
	}
	if strings.HasPrefix(s, "-<N>:<N>") || strings.HasPrefix(s, "+<N>:<N>") {
		return len("-<N>:<N>")
	}
	return 0
}

func isDecimalOrHexChar(ch byte) bool {
	if ch <= '9' && ch >= '0' {
		return true
	}
	return isHexChar(ch)
}

func isHexChar(ch byte) bool {
	return ch <= 'f' && ch >= 'a' || ch <= 'F' && ch >= 'A'
}

func canBeTreatedAsNum(s string) bool {
	if s == "" {
		return false
	}
	if !hasHexChars(s) {
		// Decimal number can contain any number of chars
		return true
	}

	// The most of hex nums contain 4 and more chars, and the number of chars are usually even.
	// This prevents from incorrect detection of hex numbers such as "be", "ad", "foo", "abc", etc.
	if len(s) < 4 || len(s)%2 == 1 {
		return false
	}
	return true
}

func hasHexChars(s string) bool {
	for i := 0; i < len(s); i++ {
		if isHexChar(s[i]) {
			return true
		}
	}
	return false
}

func isSpecialNumStart(ch byte) bool {
	return ch == '_' || ch == 'T' || ch == 'X' || ch == 'x' || ch == 'v' || ch == 's' || ch == 'h' || ch == 'm'
}

func isSpecialNumEnd(ch byte) bool {
	return ch == '_' || ch == 'T' || ch == 'Z' || ch == 's' || ch == 'm' || ch == 'h' || ch == 'u' || ch == 'n'
}
