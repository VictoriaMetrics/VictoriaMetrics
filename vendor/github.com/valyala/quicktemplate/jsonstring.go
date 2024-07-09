package quicktemplate

import (
	"bytes"
	"fmt"
	"strings"
)

func hasSpecialChars(s string) bool {
	if strings.IndexByte(s, '"') >= 0 || strings.IndexByte(s, '\\') >= 0 || strings.IndexByte(s, '<') >= 0 || strings.IndexByte(s, '\'') >= 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 {
			return true
		}
	}
	return false
}

// AppendJSONString appends json-encoded string s to dst and returns the result.
//
// If addQuotes is true, then the appended json string is wrapped into double quotes.
func AppendJSONString(dst []byte, s string, addQuotes bool) []byte {
	if !hasSpecialChars(s) {
		// Fast path - nothing to escape.
		if !addQuotes {
			return append(dst, s...)
		}
		dst = append(dst, '"')
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}

	// Slow path - there are chars to escape.
	if addQuotes {
		dst = append(dst, '"')
	}
	dst = jsonReplacer.AppendReplace(dst, s)
	if addQuotes {
		dst = append(dst, '"')
	}
	return dst
}

var jsonReplacer = newByteReplacer(func() ([]byte, []string) {
	oldChars := []byte("\n\r\t\b\f\"\\<'")
	newStrings := []string{`\n`, `\r`, `\t`, `\b`, `\f`, `\"`, `\\`, `\u003c`, `\u0027`}
	for i := 0; i < 0x20; i++ {
		c := byte(i)
		if n := bytes.IndexByte(oldChars, c); n >= 0 {
			continue
		}
		oldChars = append(oldChars, byte(i))
		newStrings = append(newStrings, fmt.Sprintf(`\u%04x`, i))
	}
	return oldChars, newStrings
}())

type byteReplacer struct {
	m   [256]byte
	newStrings []string
}

func newByteReplacer(oldChars []byte, newStrings []string) *byteReplacer {
	if len(oldChars) != len(newStrings) {
		panic(fmt.Errorf("len(oldChars)=%d must be equal to len(newStrings)=%d", len(oldChars), len(newStrings)))
	}
	if len(oldChars) >= 255 {
		panic(fmt.Errorf("len(oldChars)=%d must be smaller than 255", len(oldChars)))
	}

	var m [256]byte
	for i := range m[:] {
		m[i] = 255
	}
	for i, c := range oldChars {
		m[c] = byte(i)
	}
	return &byteReplacer{
		m:   m,
		newStrings: newStrings,
	}
}

func (br *byteReplacer) AppendReplace(dst []byte, s string) []byte {
	m := br.m
	newStrings := br.newStrings
	for i := 0; i < len(s); i++ {
		c := s[i]
		n := m[c]
		if n == 255 {
			dst = append(dst, c)
		} else {
			dst = append(dst, newStrings[n]...)
		}
	}
	return dst
}
