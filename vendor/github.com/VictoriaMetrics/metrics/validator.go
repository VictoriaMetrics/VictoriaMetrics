package metrics

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateMetric validates provided string
// to be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
func ValidateMetric(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("metric cannot be empty")
	}
	if strings.IndexByte(s, '\n') >= 0 {
		return fmt.Errorf("metric cannot contain line breaks")
	}
	n := strings.IndexByte(s, '{')
	if n < 0 {
		return validateIdent(s)
	}
	ident := s[:n]
	s = s[n+1:]
	if err := validateIdent(ident); err != nil {
		return err
	}
	if len(s) == 0 || s[len(s)-1] != '}' {
		return fmt.Errorf("missing closing curly brace at the end of %q", ident)
	}
	return validateTags(s[:len(s)-1])
}

func validateTags(s string) error {
	if len(s) == 0 {
		return nil
	}
	for {
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return fmt.Errorf("missing `=` after %q", s)
		}
		ident := s[:n]
		s = s[n+1:]
		if err := validateIdent(ident); err != nil {
			return err
		}
		if len(s) == 0 || s[0] != '"' {
			return fmt.Errorf("missing starting `\"` for %q value; tail=%q", ident, s)
		}
		s = s[1:]
	again:
		n = strings.IndexByte(s, '"')
		if n < 0 {
			return fmt.Errorf("missing trailing `\"` for %q value; tail=%q", ident, s)
		}
		m := n
		for m > 0 && s[m-1] == '\\' {
			m--
		}
		if (n-m)%2 == 1 {
			s = s[n+1:]
			goto again
		}
		s = s[n+1:]
		if len(s) == 0 {
			return nil
		}
		if !strings.HasPrefix(s, ",") {
			return fmt.Errorf("missing `,` after %q value; tail=%q", ident, s)
		}
		s = skipSpace(s[1:])
	}
}

func skipSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

func validateIdent(s string) error {
	if !identRegexp.MatchString(s) {
		return fmt.Errorf("invalid identifier %q", s)
	}
	return nil
}

var identRegexp = regexp.MustCompile("^[a-zA-Z_:.][a-zA-Z0-9_:.]*$")
