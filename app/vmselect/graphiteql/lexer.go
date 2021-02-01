package graphiteql

import (
	"fmt"
	"strings"
)

type lexer struct {
	// Token contains the currently parsed token.
	// An empty token means EOF.
	Token string

	sOrig string
	sTail string

	err error
}

func (lex *lexer) Context() string {
	return fmt.Sprintf("%s%s", lex.Token, lex.sTail)
}

func (lex *lexer) Init(s string) {
	lex.Token = ""

	lex.sOrig = s
	lex.sTail = s

	lex.err = nil
}

func (lex *lexer) Next() error {
	if lex.err != nil {
		return lex.err
	}
	token, err := lex.next()
	if err != nil {
		lex.err = err
		return err
	}
	lex.Token = token
	return nil
}

func (lex *lexer) next() (string, error) {
	// Skip whitespace
	s := lex.sTail
	i := 0
	for i < len(s) && isSpaceChar(s[i]) {
		i++
	}
	s = s[i:]
	lex.sTail = s

	if len(s) == 0 {
		return "", nil
	}

	var token string
	var err error
	switch s[0] {
	case '(', ')', ',', '|', '=', '+', '-':
		token = s[:1]
		goto tokenFoundLabel
	}
	if isStringPrefix(s) {
		token, err = scanString(s)
		if err != nil {
			return "", err
		}
		goto tokenFoundLabel
	}
	if isPositiveNumberPrefix(s) {
		token, err = scanPositiveNumber(s)
		if err != nil {
			return "", err
		}
		goto tokenFoundLabel
	}
	token, err = scanIdent(s)
	if err != nil {
		return "", err
	}

tokenFoundLabel:
	lex.sTail = s[len(token):]
	return token, nil
}

func scanString(s string) (string, error) {
	if len(s) < 2 {
		return "", fmt.Errorf("cannot find end of string in %q", s)
	}

	quote := s[0]
	i := 1
	for {
		n := strings.IndexByte(s[i:], quote)
		if n < 0 {
			return "", fmt.Errorf("cannot find closing quote %c for the string %q", quote, s)
		}
		i += n
		bs := 0
		for bs < i && s[i-bs-1] == '\\' {
			bs++
		}
		if bs%2 == 0 {
			token := s[:i+1]
			return token, nil
		}
		i++
	}
}

func scanPositiveNumber(s string) (string, error) {
	// Scan integer part. It may be empty if fractional part exists.
	i := 0
	skipChars, isHex := scanSpecialIntegerPrefix(s)
	i += skipChars
	if isHex {
		// Scan integer hex number
		for i < len(s) && isHexChar(s[i]) {
			i++
		}
		if i == skipChars {
			return "", fmt.Errorf("number cannot be empty")
		}
		return s[:i], nil
	}
	for i < len(s) && isDecimalChar(s[i]) {
		i++
	}

	if i == len(s) {
		if i == skipChars {
			return "", fmt.Errorf("number cannot be empty")
		}
		return s, nil
	}
	if s[i] != '.' && s[i] != 'e' && s[i] != 'E' {
		return s[:i], nil
	}

	if s[i] == '.' {
		// Scan fractional part. It cannot be empty.
		i++
		j := i
		for j < len(s) && isDecimalChar(s[j]) {
			j++
		}
		if j == i {
			return "", fmt.Errorf("missing fractional part in %q", s)
		}
		i = j
		if i == len(s) {
			return s, nil
		}
	}

	if s[i] != 'e' && s[i] != 'E' {
		return s[:i], nil
	}
	i++

	// Scan exponent part.
	if i == len(s) {
		return "", fmt.Errorf("missing exponent part in %q", s)
	}
	if s[i] == '-' || s[i] == '+' {
		i++
	}
	j := i
	for j < len(s) && isDecimalChar(s[j]) {
		j++
	}
	if j == i {
		return "", fmt.Errorf("missing exponent part in %q", s)
	}
	return s[:j], nil
}

func scanIdent(s string) (string, error) {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '\\':
			// Skip the next char, since it is escaped
			i += 2
			if i > len(s) {
				return "", fmt.Errorf("missing escaped char in the end of %q", s)
			}
		case '[':
			n := strings.IndexByte(s[i+1:], ']')
			if n < 0 {
				return "", fmt.Errorf("missing `]` char in %q", s)
			}
			i += n + 2
		case '{':
			n := strings.IndexByte(s[i+1:], '}')
			if n < 0 {
				return "", fmt.Errorf("missing '}' char in %q", s)
			}
			i += n + 2
		case '*', '.':
			i++
		default:
			if !isIdentChar(s[i]) {
				goto end
			}
			i++
		}
	}
end:
	if i == 0 {
		return "", fmt.Errorf("cannot find a single ident char in %q", s)
	}
	return s[:i], nil
}

func unescapeIdent(s string) string {
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}
	dst := make([]byte, 0, len(s))
	for {
		dst = append(dst, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			return string(dst)
		}
		if s[0] == 'x' && len(s) >= 3 {
			h1 := fromHex(s[1])
			h2 := fromHex(s[2])
			if h1 >= 0 && h2 >= 0 {
				dst = append(dst, byte((h1<<4)|h2))
				s = s[3:]
			} else {
				dst = append(dst, s[0])
				s = s[1:]
			}
		} else {
			dst = append(dst, s[0])
			s = s[1:]
		}
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			dst = append(dst, s...)
			return string(dst)
		}
	}
}

func fromHex(ch byte) int {
	if ch >= '0' && ch <= '9' {
		return int(ch - '0')
	}
	if ch >= 'a' && ch <= 'f' {
		return int((ch - 'a') + 10)
	}
	if ch >= 'A' && ch <= 'F' {
		return int((ch - 'A') + 10)
	}
	return -1
}

func toHex(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
}

func isMetricExprChar(ch byte) bool {
	switch ch {
	case '.', '*', '[', ']', '{', '}', ',':
		return true
	}
	return false
}

func appendEscapedIdent(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if isIdentChar(ch) || isMetricExprChar(ch) {
			if i == 0 && !isFirstIdentChar(ch) {
				// hex-encode the first char
				dst = append(dst, '\\', 'x', toHex(ch>>4), toHex(ch&0xf))
			} else {
				dst = append(dst, ch)
			}
		} else if ch >= 0x20 && ch < 0x7f {
			// Leave ASCII printable chars as is
			dst = append(dst, '\\', ch)
		} else {
			// hex-encode non-printable chars
			dst = append(dst, '\\', 'x', toHex(ch>>4), toHex(ch&0xf))
		}
	}
	return dst
}

func isEOF(s string) bool {
	return len(s) == 0
}

func isBool(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "false"
}

func isStringPrefix(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch s[0] {
	case '"', '\'':
		return true
	default:
		return false
	}
}

func isPositiveNumberPrefix(s string) bool {
	if len(s) == 0 {
		return false
	}
	if isDecimalChar(s[0]) {
		return true
	}

	// Check for .234 numbers
	if s[0] != '.' || len(s) < 2 {
		return false
	}
	return isDecimalChar(s[1])
}

func isSpecialIntegerPrefix(s string) bool {
	skipChars, _ := scanSpecialIntegerPrefix(s)
	return skipChars > 0
}

func scanSpecialIntegerPrefix(s string) (skipChars int, isHex bool) {
	if len(s) < 1 || s[0] != '0' {
		return 0, false
	}
	s = strings.ToLower(s[1:])
	if len(s) == 0 {
		return 0, false
	}
	if isDecimalChar(s[0]) {
		// octal number: 0123
		return 1, false
	}
	if s[0] == 'x' {
		// 0x
		return 2, true
	}
	if s[0] == 'o' || s[0] == 'b' {
		// 0x, 0o or 0b prefix
		return 2, false
	}
	return 0, false
}

func isDecimalChar(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexChar(ch byte) bool {
	return isDecimalChar(ch) || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}

func isIdentPrefix(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '\\' {
		// Assume this is an escape char for the next char.
		return true
	}
	return isFirstIdentChar(s[0])
}

func isFirstIdentChar(ch byte) bool {
	if !isIdentChar(ch) {
		return false
	}
	if isDecimalChar(ch) {
		return false
	}
	return true
}

func isIdentChar(ch byte) bool {
	if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
		return true
	}
	if isDecimalChar(ch) {
		return true
	}
	switch ch {
	case '-', '_', '$', ':', '*', '{', '[':
		return true
	}
	return false
}

func isSpaceChar(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}
