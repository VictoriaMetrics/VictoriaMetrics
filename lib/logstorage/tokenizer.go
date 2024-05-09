package logstorage

import (
	"sync"
	"unicode"
)

// tokenizeStrings extracts word tokens from a, appends them to dst and returns the result.
//
// the order of returned tokens is unspecified.
func tokenizeStrings(dst, a []string) []string {
	t := getTokenizer()
	for i, s := range a {
		if i > 0 && s == a[i-1] {
			// This string has been already tokenized
			continue
		}
		dst = t.tokenizeString(dst, s)
	}
	putTokenizer(t)

	return dst
}

type tokenizer struct {
	m map[string]struct{}
}

func (t *tokenizer) reset() {
	clear(t.m)
}

func (t *tokenizer) tokenizeString(dst []string, s string) []string {
	m := t.m
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
			if _, ok := m[token]; ok {
				m[token] = struct{}{}
				dst = append(dst, token)
			}
		}
		s = s[nextIdx:]
	}
	return dst
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

type tokensBuf struct {
	A []string
}

func (tb *tokensBuf) reset() {
	clear(tb.A)
	tb.A = tb.A[:0]
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
