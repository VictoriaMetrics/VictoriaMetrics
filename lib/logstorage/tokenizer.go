package logstorage

import (
	"sync"
	"unicode"
	"unicode/utf8"
)

// tokenizeStrings extracts word tokens from a, appends them to dst and returns the result.
//
// The order of returned tokens equals the order of tokens seen in a.
func tokenizeStrings(dst, a []string) []string {
	t := getTokenizer()
	for i, s := range a {
		if i > 0 && s == a[i-1] {
			// This string has been already tokenized
			continue
		}
		dst = t.tokenizeString(dst, s, false)
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

func (t *tokenizer) tokenizeString(dst []string, s string, keepDuplicateTokens bool) []string {
	if !isASCII(s) {
		// Slow path - s contains unicode chars
		return t.tokenizeStringUnicode(dst, s, keepDuplicateTokens)
	}

	// Fast path for ASCII s
	m := t.m
	i := 0
	for i < len(s) {
		// Search for the next token.
		start := len(s)
		for i < len(s) {
			if !isTokenChar(s[i]) {
				i++
				continue
			}
			start = i
			i++
			break
		}
		// Search for the end of the token.
		end := len(s)
		for i < len(s) {
			if isTokenChar(s[i]) {
				i++
				continue
			}
			end = i
			i++
			break
		}
		if end <= start {
			break
		}

		// Register the token.
		token := s[start:end]
		if keepDuplicateTokens {
			dst = append(dst, token)
		} else if _, ok := m[token]; !ok {
			m[token] = struct{}{}
			dst = append(dst, token)
		}
	}
	return dst
}

func (t *tokenizer) tokenizeStringUnicode(dst []string, s string, keepDuplicateTokens bool) []string {
	m := t.m
	for len(s) > 0 {
		// Search for the next token.
		n := len(s)
		for offset, r := range s {
			if isTokenRune(r) {
				n = offset
				break
			}
		}
		s = s[n:]
		// Search for the end of the token.
		n = len(s)
		for offset, r := range s {
			if !isTokenRune(r) {
				n = offset
				break
			}
		}
		if n == 0 {
			break
		}

		// Register the token
		token := s[:n]
		s = s[n:]
		if keepDuplicateTokens {
			dst = append(dst, token)
		} else if _, ok := m[token]; !ok {
			m[token] = struct{}{}
			dst = append(dst, token)
		}
	}
	return dst
}

func isASCII(s string) bool {
	for i := range s {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func isTokenChar(c byte) bool {
	return tokenCharTable[c] != 0
}

var tokenCharTable = func() *[256]byte {
	var a [256]byte
	for c := uint(0); c < 256; c++ {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			a[c] = 1
		}
	}
	return &a
}()

func isTokenRune(c rune) bool {
	if c < utf8.RuneSelf {
		// Fast path - the char is ASCII
		return isTokenChar(byte(c))
	}
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
