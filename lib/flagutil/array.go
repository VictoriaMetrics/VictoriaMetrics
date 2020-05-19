package flagutil

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// NewArray returns new Array with the given name and description.
func NewArray(name, description string) *Array {
	var a Array
	flag.Var(&a, name, description)
	return &a
}

// Array is a flag that holds an array of values.
//
// It may be set either by specifying multiple flags with the given name
// passed to NewArray or by joining flag values by comma.
//
// The following example sets equivalent flag array with two items (value1, value2):
//
//     -foo=value1 -foo=value2
//     -foo=value1,value2
//
// Flag values may be quoted. For instance, the following arg creates an array of ("a", "b, c") items:
//
//    -foo='a,"b, c"'
//
type Array []string

// String implements flag.Value interface
func (a *Array) String() string {
	aEscaped := make([]string, len(*a))
	for i, v := range *a {
		if strings.ContainsAny(v, `", `+"\n") {
			v = fmt.Sprintf("%q", v)
		}
		aEscaped[i] = v
	}
	return strings.Join(aEscaped, ",")
}

// Set implements flag.Value interface
func (a *Array) Set(value string) error {
	values := parseArrayValues(value)
	*a = append(*a, values...)
	return nil
}

func parseArrayValues(s string) []string {
	if len(s) == 0 {
		return nil
	}
	var values []string
	for {
		v, tail := getNextArrayValue(s)
		values = append(values, v)
		if len(tail) == 0 {
			return values
		}
		if tail[0] == ',' {
			tail = tail[1:]
		}
		s = tail
	}
}

func getNextArrayValue(s string) (string, string) {
	if len(s) == 0 {
		return "", ""
	}
	if s[0] != '"' {
		// Fast path - unquoted string
		n := strings.IndexByte(s, ',')
		if n < 0 {
			// The last item
			return s, ""
		}
		return s[:n], s[n:]
	}

	// Find the end of quoted string
	end := 1
	ss := s[1:]
	for {
		n := strings.IndexByte(ss, '"')
		if n < 0 {
			// Cannot find trailing quote. Return the whole string till the end.
			return s, ""
		}
		end += n + 1
		// Verify whether the trailing quote is escaped with backslash.
		backslashes := 0
		for n > backslashes && ss[n-backslashes-1] == '\\' {
			backslashes++
		}
		if backslashes&1 == 0 {
			// The trailing quote isn't escaped.
			break
		}
		// The trailing quote is escaped. Continue searching for the next quote.
		ss = ss[n+1:]
	}
	v := s[:end]
	vUnquoted, err := strconv.Unquote(v)
	if err == nil {
		v = vUnquoted
	}
	return v, s[end:]
}

// GetOptionalArg returns optional arg under the given argIdx.
func (a *Array) GetOptionalArg(argIdx int) string {
	x := *a
	if argIdx >= len(x) {
		if len(x) == 1 {
			return x[0]
		}
		return ""
	}
	return x[argIdx]
}
