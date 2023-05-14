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

	// isLastColumn is set to true when the last column at the given Line is processed
	isLastColumn bool

	s string
}

// Init initializes sc with s
func (sc *scanner) Init(s string) {
	sc.Line = ""
	sc.Column = ""
	sc.Error = nil
	sc.isLastColumn = false
	sc.s = s
}

// NextLine advances csv scanner to the next line and sets cs.Line to it.
//
// It clears sc.Error.
//
// false is returned if no more lines left in sc.s
func (sc *scanner) NextLine() bool {
	s := sc.s
	sc.Line = ""
	sc.Error = nil
	sc.isLastColumn = false
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		var line string
		if n >= 0 {
			line = trimTrailingCR(s[:n])
			s = s[n+1:]
		} else {
			line = trimTrailingCR(s)
			s = ""
		}
		if len(line) > 0 {
			sc.Line = line
			sc.s = s
			return true
		}
	}
	sc.s = ""
	return false
}

// NextColumn advances sc.Line to the next Column and sets sc.Column to it.
//
// false is returned if no more columns left in sc.Line or if any error occurs.
// sc.Error is set to error in the case of error.
func (sc *scanner) NextColumn() bool {
	if sc.isLastColumn || sc.Error != nil {
		return false
	}
	s := sc.Line
	if strings.HasPrefix(s, `"`) || strings.HasPrefix(s, "'") {
		field, tail, err := readQuotedField(s)
		if err != nil {
			sc.Error = err
			return false
		}
		sc.Column = field
		if len(tail) == 0 {
			sc.isLastColumn = true
		} else {
			if tail[0] != ',' {
				sc.Error = fmt.Errorf("missing comma after quoted field in %q", s)
				return false
			}
			tail = tail[1:]
		}
		sc.Line = tail
		return true
	}
	n := strings.IndexByte(s, ',')
	if n >= 0 {
		sc.Column = s[:n]
		sc.Line = s[n+1:]
	} else {
		sc.Column = s
		sc.Line = ""
		sc.isLastColumn = true
	}
	return true
}

func trimTrailingCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func readQuotedField(s string) (string, string, error) {
	quote := s[0]
	offset := 1
	n := strings.IndexByte(s[offset:], quote)
	if n < 0 {
		return "", s, fmt.Errorf("missing closing quote for %q", s)
	}
	offset += n + 1
	if offset >= len(s) || s[offset] != quote {
		// Fast path - the quoted string doesn't contain escaped quotes
		return s[1 : offset-1], s[offset:], nil
	}
	// Slow path - the quoted string contains escaped quote
	buf := make([]byte, 0, len(s)-2)
	buf = append(buf, s[1:offset]...)
	for {
		offset++
		n := strings.IndexByte(s[offset:], quote)
		if n < 0 {
			return "", s, fmt.Errorf("missing closing quote for %q", s)
		}
		buf = append(buf, s[offset:offset+n]...)
		offset += n + 1
		if offset < len(s) && s[offset] == quote {
			buf = append(buf, quote)
			continue
		}
		return string(buf), s[offset:], nil
	}
}
