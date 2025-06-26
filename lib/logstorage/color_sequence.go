package logstorage

import (
	"strings"
)

// hasColorSequences returns true if s contains ANSI color escape sequences.
func hasColorSequences(s string) bool {
	return strings.Contains(s, "\x1b[")
}

// dropColorSequences removes ANSI escape sequences from src, appends the result to dst and returns the result
//
// See https://en.wikipedia.org/wiki/ANSI_escape_code
func dropColorSequences(dst []byte, src string) []byte {
	for {
		n := strings.Index(src, "\x1b[")
		if n < 0 {
			return append(dst, src...)
		}
		dst = append(dst, src[:n]...)
		src = src[n+2:]

		src = skipANSISequence(src)
	}
}

// skipANSISequence skips non-ansi escape sequence at the beginning of s and returns the position of the first byte after it.
func skipANSISequence(s string) string {
	n := 0

	// Skip optional parameter bytes after CSI (control sequence introducer).
	// See https://gist.github.com/ConnerWill/d4b6c776b509add763e17f9f113fd25b
	for n < len(s) {
		ch := s[n]
		if ch < 0x30 || ch > 0x3f {
			break
		}
		n++
	}

	// Scan ansi escape sequence according to the chapter 13.1
	// at https://www.ecma-international.org/wp-content/uploads/ECMA-35_6th_edition_december_1994.pdf

	// skip optional intermediate bytes
	for n < len(s) {
		ch := s[n]
		if ch < 0x20 || ch > 0x2f {
			break
		}
		n++
	}

	// skip the final byte
	if n < len(s) {
		ch := s[n]
		if ch >= 0x30 && ch <= 0x7e {
			n++
		}
	}

	return s[n:]
}
