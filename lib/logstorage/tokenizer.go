package logstorage

import (
	"sort"
	"sync"
	"unicode"
)

// tokenizeStrings extracts word tokens from a, appends them to dst and returns the result.
func tokenizeStrings(dst, a []string) []string {
	t := getTokenizer()
	m := t.m
	for i, s := range a {
		if i > 0 && s == a[i-1] {
			// This string has been already tokenized
			continue
		}
		tokenizeString(m, s)
	}
	dstLen := len(dst)
	for k := range t.m {
		dst = append(dst, k)
	}
	putTokenizer(t)

	// Sort tokens with zero memory allocations
	ss := getStringsSorter(dst[dstLen:])
	sort.Sort(ss)
	putStringsSorter(ss)

	return dst
}

type tokenizer struct {
	m map[string]struct{}
}

func (t *tokenizer) reset() {
	m := t.m
	for k := range m {
		delete(m, k)
	}
}

func tokenizeString(dst map[string]struct{}, s string) {
	for len(s) > 0 {
		// Search for the next token.
		nextIdx := len(s)
		for i, c := range s {
			if isTokenRune(c) {
				nextIdx = i
				break
			}
		}
		s = s[nextIdx:]
		// Search for the end of the token
		nextIdx = len(s)
		for i, c := range s {
			if !isTokenRune(c) {
				nextIdx = i
				break
			}
		}
		token := s[:nextIdx]
		if len(token) > 0 {
			dst[token] = struct{}{}
		}
		s = s[nextIdx:]
	}
}

func isTokenRune(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'
}

func getTokenizer() *tokenizer {
	v := tokenizerPool.Get()
	if v == nil {
		return &tokenizer{
			m: make(map[string]struct{}),
		}
	}
	return v.(*tokenizer)
}

func putTokenizer(t *tokenizer) {
	t.reset()
	tokenizerPool.Put(t)
}

var tokenizerPool sync.Pool

type stringsSorter struct {
	a []string
}

func (ss *stringsSorter) Len() int {
	return len(ss.a)
}
func (ss *stringsSorter) Swap(i, j int) {
	a := ss.a
	a[i], a[j] = a[j], a[i]
}
func (ss *stringsSorter) Less(i, j int) bool {
	a := ss.a
	return a[i] < a[j]
}

func getStringsSorter(a []string) *stringsSorter {
	v := stringsSorterPool.Get()
	if v == nil {
		return &stringsSorter{
			a: a,
		}
	}
	ss := v.(*stringsSorter)
	ss.a = a
	return ss
}

func putStringsSorter(ss *stringsSorter) {
	ss.a = nil
	stringsSorterPool.Put(ss)
}

var stringsSorterPool sync.Pool

type tokensBuf struct {
	A []string
}

func (tb *tokensBuf) reset() {
	a := tb.A
	for i := range a {
		a[i] = ""
	}
	tb.A = a[:0]
}

func getTokensBuf() *tokensBuf {
	v := tokensBufPool.Get()
	if v == nil {
		return &tokensBuf{}
	}
	return v.(*tokensBuf)
}

func putTokensBuf(tb *tokensBuf) {
	tb.reset()
	tokensBufPool.Put(tb)
}

var tokensBufPool sync.Pool
