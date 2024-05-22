package logstorage

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// pattern represents text pattern in the form 'some_text<some_field>other_text...'
type pattern struct {
	// steps contains steps for extracting fields from string
	steps []patternStep

	// matches contains matches for every step in steps
	matches []string

	// fields contains matches for non-empty fields
	fields []patternField
}

type patternField struct {
	name  string
	value *string
}

type patternStep struct {
	prefix string
	field  string
}

func (ptn *pattern) clone() *pattern {
	steps := ptn.steps
	fields, matches := newFieldsAndMatchesFromPatternSteps(steps)
	if len(fields) == 0 {
		logger.Panicf("BUG: fields cannot be empty for steps=%v", steps)
	}
	return &pattern{
		steps:   steps,
		matches: matches,
		fields:  fields,
	}
}

func parsePattern(s string) (*pattern, error) {
	steps, err := parsePatternSteps(s)
	if err != nil {
		return nil, err
	}

	// Verify that prefixes are non-empty between fields. The first prefix may be empty.
	for i := 1; i < len(steps); i++ {
		if steps[i].prefix == "" {
			return nil, fmt.Errorf("missing delimiter between <%s> and <%s>", steps[i-1].field, steps[i].field)
		}
	}

	// Build pattern struct
	fields, matches := newFieldsAndMatchesFromPatternSteps(steps)
	if len(fields) == 0 {
		return nil, fmt.Errorf("pattern %q must contain at least a single named field in the form <field_name>", s)
	}

	ptn := &pattern{
		steps:   steps,
		matches: matches,
		fields:  fields,
	}
	return ptn, nil
}

func newFieldsAndMatchesFromPatternSteps(steps []patternStep) ([]patternField, []string) {
	matches := make([]string, len(steps))

	var fields []patternField
	for i, step := range steps {
		if step.field != "" {
			fields = append(fields, patternField{
				name:  step.field,
				value: &matches[i],
			})
		}
	}

	return fields, matches
}

func (ptn *pattern) apply(s string) {
	clear(ptn.matches)

	steps := ptn.steps

	if prefix := steps[0].prefix; prefix != "" {
		n := strings.Index(s, prefix)
		if n < 0 {
			// Mismatch
			return
		}
		s = s[n+len(prefix):]
	}

	matches := ptn.matches
	for i := range steps {
		nextPrefix := ""
		if i+1 < len(steps) {
			nextPrefix = steps[i+1].prefix
		}

		us, nOffset := tryUnquoteString(s)
		if nOffset >= 0 {
			// Matched quoted string
			matches[i] = us
			s = s[nOffset:]
			if !strings.HasPrefix(s, nextPrefix) {
				// Mismatch
				return
			}
			s = s[len(nextPrefix):]
		} else {
			// Match unquoted string until the nextPrefix
			if nextPrefix == "" {
				matches[i] = s
				return
			}
			n := strings.Index(s, nextPrefix)
			if n < 0 {
				// Mismatch
				return
			}
			matches[i] = s[:n]
			s = s[n+len(nextPrefix):]
		}
	}
}

func tryUnquoteString(s string) (string, int) {
	if len(s) == 0 {
		return s, -1
	}
	if s[0] != '"' && s[0] != '`' {
		return s, -1
	}
	qp, err := strconv.QuotedPrefix(s)
	if err != nil {
		return s, -1
	}
	us, err := strconv.Unquote(qp)
	if err != nil {
		return s, -1
	}
	return us, len(qp)
}

func parsePatternSteps(s string) ([]patternStep, error) {
	if len(s) == 0 {
		return nil, nil
	}

	var steps []patternStep

	n := strings.IndexByte(s, '<')
	if n < 0 {
		steps = append(steps, patternStep{
			prefix: html.UnescapeString(s),
		})
		return steps, nil
	}
	prefix := s[:n]
	s = s[n+1:]
	for {
		n := strings.IndexByte(s, '>')
		if n < 0 {
			return nil, fmt.Errorf("missing '>' for <%s", s)
		}
		field := s[:n]
		s = s[n+1:]

		if field == "_" || field == "*" {
			field = ""
		}
		steps = append(steps, patternStep{
			prefix: prefix,
			field:  field,
		})
		if len(s) == 0 {
			break
		}

		n = strings.IndexByte(s, '<')
		if n < 0 {
			steps = append(steps, patternStep{
				prefix: s,
			})
			break
		}
		prefix = s[:n]
		s = s[n+1:]
	}

	for i := range steps {
		step := &steps[i]
		step.prefix = html.UnescapeString(step.prefix)
	}

	return steps, nil
}
