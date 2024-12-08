package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeCollapseNums processes '| collapse_nums ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#collapse_nums-pipe
type pipeCollapseNums struct {
	field string

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
	return s
}

func (pc *pipeCollapseNums) canLiveTail() bool {
	return true
}

func (pc *pipeCollapseNums) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForUpdatePipe(neededFields, unneededFields, pc.field, pc.iff)
}

func (pc *pipeCollapseNums) hasFilterInWithQuery() bool {
	return pc.iff.hasFilterInWithQuery()
}

func (pc *pipeCollapseNums) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (pipe, error) {
	iffNew, err := pc.iff.initFilterInValues(cache, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}
	pcNew := *pc
	pcNew.iff = iffNew
	return &pcNew, nil
}

func (pc *pipeCollapseNums) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	updateFunc := func(a *arena, v string) string {
		bLen := len(a.b)
		a.b = appendCollapseNums(a.b, v)
		return bytesutil.ToUnsafeString(a.b[bLen:])
	}

	return newPipeUpdateProcessor(workersCount, updateFunc, ppNext, pc.field, pc.iff)
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

	pc := &pipeCollapseNums{
		field: field,
		iff:   iff,
	}

	return pc, nil
}

func appendCollapseNums(dst []byte, s string) []byte {
	if !isASCII(s) {
		return appendCollapseNumsUnicode(dst, s)
	}

	start := 0
	numStart := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isDecimalOrHexRune(rune(c)) {
			if numStart < 0 && (i == 0 || isSpecialStartNumRune(rune(s[i-1])) || !isTokenChar(s[i-1])) {
				numStart = i
			}
			continue
		}
		if numStart < 0 {
			continue
		}

		dst = append(dst, s[start:numStart]...)
		if (!isSpecialEndNumRune(rune(c)) && isTokenChar(c)) || !canBeTreatedAsNum(s[numStart:i]) {
			dst = append(dst, s[numStart:i]...)
		} else {
			dst = append(dst, "<N>"...)
		}
		start = i
		numStart = -1
	}
	if numStart >= 0 && canBeTreatedAsNum(s[numStart:]) {
		dst = append(dst, s[start:numStart]...)
		dst = append(dst, "<N>"...)
	} else {
		dst = append(dst, s[start:]...)
	}
	return dst
}

func appendCollapseNumsUnicode(dst []byte, s string) []byte {
	start := 0
	numStart := -1
	rPrev := rune(0)
	for i, r := range s {
		if isDecimalOrHexRune(r) {
			if numStart < 0 && (isSpecialStartNumRune(rPrev) || !isTokenRune(rPrev)) {
				numStart = i
			}
			rPrev = r
			continue
		}
		if numStart < 0 {
			rPrev = r
			continue
		}

		dst = append(dst, s[start:numStart]...)
		if (!isSpecialEndNumRune(r) && isTokenRune(r)) || !canBeTreatedAsNum(s[numStart:i]) {
			dst = append(dst, s[numStart:i]...)
		} else {
			dst = append(dst, "<N>"...)
		}
		start = i
		numStart = -1
		rPrev = r
	}
	if numStart >= 0 && canBeTreatedAsNum(s[numStart:]) {
		dst = append(dst, s[start:numStart]...)
		dst = append(dst, "<N>"...)
	} else {
		dst = append(dst, s[start:]...)
	}
	return dst
}

func isDecimalOrHexRune(r rune) bool {
	if r <= '9' && r >= '0' {
		return true
	}
	return isHexRune(r)
}

func isHexRune(r rune) bool {
	return r <= 'f' && r >= 'a' || r <= 'F' && r >= 'A'
}

func canBeTreatedAsNum(s string) bool {
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
		if isHexRune(rune(s[i])) {
			return true
		}
	}
	return false
}

func isSpecialStartNumRune(r rune) bool {
	return r == 'T' || r == 'v' || r == 's' || r == 'h' || r == 'm'
}

func isSpecialEndNumRune(r rune) bool {
	return r == 'T' || r == 'Z' || r == 's' || r == 'm' || r == 'h' || r == 'μ' || r == 'u' || r == 'n'
}
