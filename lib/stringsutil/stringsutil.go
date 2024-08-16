package stringsutil

import (
	"unicode"
	"unicode/utf8"
)

// LimitStringLen limits the length of s with maxLen.
//
// If len(s) > maxLen, then s is replaced with "s_prefix..s_suffix",
// so the total length of the returned string doesn't exceed maxLen.
func LimitStringLen(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	n := (maxLen / 2) - 1
	return s[:n] + ".." + s[len(s)-n:]
}

// AppendLowercase appends lowercase s to dst and returns the result.
//
// It is faster alternative to strings.ToLower.
func AppendLowercase(dst []byte, s string) []byte {
	dstLen := len(dst)

	// Try fast path at first by assuming that s contains only ASCII chars.
	hasUnicodeChars := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			hasUnicodeChars = true
			break
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	if hasUnicodeChars {
		// Slow path - s contains non-ASCII chars. Use Unicode encoding.
		dst = dst[:dstLen]
		for _, r := range s {
			r = unicode.ToLower(r)
			dst = utf8.AppendRune(dst, r)
		}
	}
	return dst
}
