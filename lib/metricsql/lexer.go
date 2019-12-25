package metricsql

import (
	"fmt"
	"strconv"
	"strings"
)

type lexer struct {
	// Token contains the currently parsed token.
	// An empty token means EOF.
	Token string

	prevTokens []string
	nextTokens []string

	sOrig string
	sTail string

	err error
}

func (lex *lexer) Context() string {
	return fmt.Sprintf("%s%s", lex.Token, lex.sTail)
}

func (lex *lexer) Init(s string) {
	lex.Token = ""
	lex.prevTokens = nil
	lex.nextTokens = nil
	lex.err = nil

	lex.sOrig = s
	lex.sTail = s
}

func (lex *lexer) Next() error {
	if lex.err != nil {
		return lex.err
	}
	lex.prevTokens = append(lex.prevTokens, lex.Token)
	if len(lex.nextTokens) > 0 {
		lex.Token = lex.nextTokens[len(lex.nextTokens)-1]
		lex.nextTokens = lex.nextTokens[:len(lex.nextTokens)-1]
		return nil
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
again:
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
	case '#':
		// Skip comment till the end of string
		s = s[1:]
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			return "", nil
		}
		lex.sTail = s[n+1:]
		goto again
	case '{', '}', '[', ']', '(', ')', ',':
		token = s[:1]
		goto tokenFoundLabel
	}
	if isIdentPrefix(s) {
		token = scanIdent(s)
		goto tokenFoundLabel
	}
	if isStringPrefix(s) {
		token, err = scanString(s)
		if err != nil {
			return "", err
		}
		goto tokenFoundLabel
	}
	if n := scanBinaryOpPrefix(s); n > 0 {
		token = s[:n]
		goto tokenFoundLabel
	}
	if n := scanTagFilterOpPrefix(s); n > 0 {
		token = s[:n]
		goto tokenFoundLabel
	}
	if n := scanDuration(s, false); n > 0 {
		token = s[:n]
		goto tokenFoundLabel
	}
	if isPositiveNumberPrefix(s) {
		token, err = scanPositiveNumber(s)
		if err != nil {
			return "", err
		}
		goto tokenFoundLabel
	}
	return "", fmt.Errorf("cannot recognize %q", s)

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
			return "", fmt.Errorf("cannot find closing quote %ch for the string %q", quote, s)
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
	for i < len(s) && isDecimalChar(s[i]) {
		i++
	}

	if i == len(s) {
		if i == 0 {
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

func scanIdent(s string) string {
	i := 0
	for i < len(s) {
		if isIdentChar(s[i]) {
			i++
			continue
		}
		if s[i] != '\\' {
			break
		}

		// Do not verify the next char, since it is escaped.
		i += 2
		if i > len(s) {
			i--
			break
		}
	}
	if i == 0 {
		panic("BUG: scanIdent couldn't find a single ident char; make sure isIdentPrefix called before scanIdent")
	}
	return s[:i]
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

func appendEscapedIdent(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if isIdentChar(ch) {
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

func (lex *lexer) Prev() {
	lex.nextTokens = append(lex.nextTokens, lex.Token)
	lex.Token = lex.prevTokens[len(lex.prevTokens)-1]
	lex.prevTokens = lex.prevTokens[:len(lex.prevTokens)-1]
}

func isEOF(s string) bool {
	return len(s) == 0
}

func scanTagFilterOpPrefix(s string) int {
	if len(s) >= 2 {
		switch s[:2] {
		case "=~", "!~", "!=":
			return 2
		}
	}
	if len(s) >= 1 {
		if s[0] == '=' {
			return 1
		}
	}
	return -1
}

func isInfOrNaN(s string) bool {
	if len(s) != 3 {
		return false
	}
	s = strings.ToLower(s)
	return s == "inf" || s == "nan"
}

func isOffset(s string) bool {
	s = strings.ToLower(s)
	return s == "offset"
}

func isStringPrefix(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch s[0] {
	// See https://prometheus.io/docs/prometheus/latest/querying/basics/#string-literals
	case '"', '\'', '`':
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

func isPositiveDuration(s string) bool {
	n := scanDuration(s, false)
	return n == len(s)
}

// PositiveDurationValue returns the duration in milliseconds for the given s
// and the given step.
func PositiveDurationValue(s string, step int64) (int64, error) {
	d, err := DurationValue(s, step)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("duration cannot be negative; got %q", s)
	}
	return d, nil
}

// DurationValue returns the duration in milliseconds for the given s
// and the given step.
//
// The returned duration value can be negative.
func DurationValue(s string, step int64) (int64, error) {
	n := scanDuration(s, true)
	if n != len(s) {
		return 0, fmt.Errorf("cannot parse duration %q", s)
	}

	f, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse duration %q: %s", s, err)
	}

	var mp float64
	switch s[len(s)-1] {
	case 's':
		mp = 1
	case 'm':
		mp = 60
	case 'h':
		mp = 60 * 60
	case 'd':
		mp = 24 * 60 * 60
	case 'w':
		mp = 7 * 24 * 60 * 60
	case 'y':
		mp = 365 * 24 * 60 * 60
	case 'i':
		mp = float64(step) / 1e3
	default:
		return 0, fmt.Errorf("invalid duration suffix in %q", s)
	}
	return int64(mp * f * 1e3), nil
}

func scanDuration(s string, canBeNegative bool) int {
	if len(s) == 0 {
		return -1
	}
	i := 0
	if s[0] == '-' && canBeNegative {
		i++
	}
	for i < len(s) && isDecimalChar(s[i]) {
		i++
	}
	if i == 0 || i == len(s) {
		return -1
	}
	if s[i] == '.' {
		j := i
		i++
		for i < len(s) && isDecimalChar(s[i]) {
			i++
		}
		if i == j || i == len(s) {
			return -1
		}
	}
	switch s[i] {
	case 's', 'm', 'h', 'd', 'w', 'y', 'i':
		return i + 1
	default:
		return -1
	}
}

func isDecimalChar(ch byte) bool {
	return ch >= '0' && ch <= '9'
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
	if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
		return true
	}
	return ch == '_' || ch == ':'
}

func isIdentChar(ch byte) bool {
	if isFirstIdentChar(ch) {
		return true
	}
	return isDecimalChar(ch) || ch == '.'
}

func isSpaceChar(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}
