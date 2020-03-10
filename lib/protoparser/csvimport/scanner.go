package csvimport

import (
	"fmt"
	"strings"
)

// scanner is csv scanner
type scanner struct {
	// The line value read after the call to NextLine()
	Line string

	// The column value read after the call to NextColumn()
	Column string

	// Error may be set only on NextColumn call.
	// It is cleared on NextLine call.
	Error error

	s string
}

// Init initializes sc with s
func (sc *scanner) Init(s string) {
	sc.Line = ""
	sc.Column = ""
	sc.Error = nil
	sc.s = s
}

// NextLine advances csv scanner to the next line and sets cs.Line to it.
//
// It clears sc.Error.
//
// false is returned if no more lines left in sc.s
func (sc *scanner) NextLine() bool {
	s := sc.s
	sc.Error = nil
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		var line string
		if n >= 0 {
			line = trimTrailingSpace(s[:n])
			s = s[n+1:]
		} else {
			line = trimTrailingSpace(s)
			s = ""
		}
		sc.Line = line
		sc.s = s
		if len(line) > 0 {
			return true
		}
	}
	return false
}

// NextColumn advances sc.Line to the next Column and sets sc.Column to it.
//
// false is returned if no more columns left in sc.Line or if any error occurs.
// sc.Error is set to error in the case of error.
func (sc *scanner) NextColumn() bool {
	s := sc.Line
	if len(s) == 0 {
		return false
	}
	if sc.Error != nil {
		return false
	}
	if s[0] == '"' {
		sc.Column, sc.Line, sc.Error = readQuotedField(s)
		return sc.Error == nil
	}
	n := strings.IndexByte(s, ',')
	if n >= 0 {
		sc.Column = s[:n]
		sc.Line = s[n+1:]
	} else {
		sc.Column = s
		sc.Line = ""
	}
	return true
}

func trimTrailingSpace(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func readQuotedField(s string) (string, string, error) {
	sOrig := s
	if len(s) == 0 || s[0] != '"' {
		return "", sOrig, fmt.Errorf("missing opening quote for %q", sOrig)
	}
	s = s[1:]
	hasEscapedQuote := false
	for {
		n := strings.IndexByte(s, '"')
		if n < 0 {
			return "", sOrig, fmt.Errorf("missing closing quote for %q", sOrig)
		}
		s = s[n+1:]
		if len(s) == 0 {
			// The end of string found
			return unquote(sOrig[1:len(sOrig)-1], hasEscapedQuote), "", nil
		}
		if s[0] == '"' {
			// Take into account escaped quote
			s = s[1:]
			hasEscapedQuote = true
			continue
		}
		if s[0] != ',' {
			return "", sOrig, fmt.Errorf("missing comma after quoted field in %q", sOrig)
		}
		return unquote(sOrig[1:len(sOrig)-len(s)-1], hasEscapedQuote), s[1:], nil
	}
}

func unquote(s string, hasEscapedQuote bool) string {
	if !hasEscapedQuote {
		return s
	}
	return strings.ReplaceAll(s, `""`, `"`)
}
