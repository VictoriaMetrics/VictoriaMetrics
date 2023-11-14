package ini

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	runesTrue  = []rune("true")
	runesFalse = []rune("false")
)

// isCaselessLitValue is a caseless value comparison, assumes want is already lower-cased for efficiency.
func isCaselessLitValue(want, have []rune) bool {
	if len(have) < len(want) {
		return false
	}

	for i := 0; i < len(want); i++ {
		if want[i] != unicode.ToLower(have[i]) {
			return false
		}
	}

	return true
}

func isValid(b []rune) (bool, int, error) {
	if len(b) == 0 {
		// TODO: should probably return an error
		return false, 0, nil
	}

	return isValidRune(b[0]), 1, nil
}

func isValidRune(r rune) bool {
	return r != ':' && r != '=' && r != '[' && r != ']' && r != ' ' && r != '\n'
}

// ValueType is an enum that will signify what type
// the Value is
type ValueType int

func (v ValueType) String() string {
	switch v {
	case NoneType:
		return "NONE"
	case StringType:
		return "STRING"
	}

	return ""
}

// ValueType enums
const (
	NoneType = ValueType(iota)
	StringType
	QuotedStringType
)

// Value is a union container
type Value struct {
	Type ValueType
	raw  []rune

	str string
	mp  map[string]string
}

func newValue(t ValueType, base int, raw []rune) (Value, error) {
	v := Value{
		Type: t,
		raw:  raw,
	}

	switch t {
	case StringType:
		v.str = string(raw)
		if isSubProperty(raw) {
			v.mp = v.MapValue()
		}
	case QuotedStringType:
		v.str = string(raw[1 : len(raw)-1])
	}

	return v, nil
}

// NewStringValue returns a Value type generated using a string input.
func NewStringValue(str string) (Value, error) {
	return newValue(StringType, 10, []rune(str))
}

func (v Value) String() string {
	switch v.Type {
	case StringType:
		return fmt.Sprintf("string: %s", string(v.raw))
	case QuotedStringType:
		return fmt.Sprintf("quoted string: %s", string(v.raw))
	default:
		return "union not set"
	}
}

func newLitToken(b []rune) (Token, int, error) {
	n := 0
	var err error

	token := Token{}
	if b[0] == '"' {
		n, err = getStringValue(b)
		if err != nil {
			return token, n, err
		}
		token = newToken(TokenLit, b[:n], QuotedStringType)
	} else if isSubProperty(b) {
		offset := 0
		end, err := getSubProperty(b, offset)
		if err != nil {
			return token, n, err
		}
		token = newToken(TokenLit, b[offset:end], StringType)
		n = end
	} else {
		n, err = getValue(b)
		token = newToken(TokenLit, b[:n], StringType)
	}

	return token, n, err
}

// replace with slices.Contains when Go 1.21
// is min supported Go version in the SDK
func containsRune(runes []rune, val rune) bool {
	for i := range runes {
		if val == runes[i] {
			return true
		}
	}
	return false
}

func isSubProperty(runes []rune) bool {
	// needs at least
	// (1) newline (2) whitespace (3) literal
	if len(runes) < 3 {
		return false
	}

	// must have an equal expression
	if !containsRune(runes, '=') && !containsRune(runes, ':') {
		return false
	}

	// must start with a new line
	if !isNewline(runes) {
		return false
	}
	_, n, err := newNewlineToken(runes)
	if err != nil {
		return false
	}
	// whitespace must follow newline
	return isWhitespace(runes[n])
}

// getSubProperty pulls all subproperties and terminates when
// it hits a newline that is not the start of another subproperty.
// offset allows for removal of leading newline and whitespace
// characters
func getSubProperty(runes []rune, offset int) (int, error) {
	for idx, val := range runes[offset:] {
		if val == '\n' && !isSubProperty(runes[offset+idx:]) {
			return offset + idx, nil
		}
	}
	return offset + len(runes), nil
}

// MapValue returns a map value for sub properties
func (v Value) MapValue() map[string]string {
	newlineParts := strings.Split(string(v.raw), "\n")
	mp := make(map[string]string)
	for _, part := range newlineParts {
		operandParts := strings.Split(part, "=")
		if len(operandParts) < 2 {
			continue
		}
		key := strings.TrimSpace(operandParts[0])
		val := strings.TrimSpace(operandParts[1])
		mp[key] = val
	}
	return mp
}

// IntValue returns an integer value
func (v Value) IntValue() (int64, bool) {
	i, err := strconv.ParseInt(string(v.raw), 0, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}

// FloatValue returns a float value
func (v Value) FloatValue() (float64, bool) {
	f, err := strconv.ParseFloat(string(v.raw), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// BoolValue returns a bool value
func (v Value) BoolValue() (bool, bool) {
	// we don't use ParseBool as it recognizes more than what we've
	// historically supported
	if isCaselessLitValue(runesTrue, v.raw) {
		return true, true
	} else if isCaselessLitValue(runesFalse, v.raw) {
		return false, true
	}
	return false, false
}

func isTrimmable(r rune) bool {
	switch r {
	case '\n', ' ':
		return true
	}
	return false
}

// StringValue returns the string value
func (v Value) StringValue() string {
	switch v.Type {

	case StringType:
		return strings.TrimFunc(string(v.raw), isTrimmable)
	case QuotedStringType:
		// preserve all characters in the quotes
		return string(removeEscapedCharacters(v.raw[1 : len(v.raw)-1]))
	default:
		return strings.TrimFunc(string(v.raw), isTrimmable)
	}
}

func contains(runes []rune, c rune) bool {
	for i := 0; i < len(runes); i++ {
		if runes[i] == c {
			return true
		}
	}

	return false
}

func runeCompare(v1 []rune, v2 []rune) bool {
	if len(v1) != len(v2) {
		return false
	}

	for i := 0; i < len(v1); i++ {
		if v1[i] != v2[i] {
			return false
		}
	}

	return true
}
