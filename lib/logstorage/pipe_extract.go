package logstorage

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

type extractFormat struct {
	steps []*extractFormatStep

	results []string
}

type extractFormatStep struct {
	prefix string
	field     string
}

func (ef *extractFormat) apply(s string) {
	clear(ef.results)

	steps := ef.steps

	if prefix := steps[0].prefix; prefix != "" {
		n := strings.Index(s, prefix)
		if n < 0 {
			// Mismatch
			return
		}
		s = s[n+len(prefix):]
	}

	results := ef.results
	for i, step := range steps[1:] {
		prefix := step.prefix

		if steps[i].field != "" {
			us, nOffset, ok := tryUnquoteString(s)
			if ok {
				results[i] = us
				s = s[nOffset:]
				if !strings.HasPrefix(s, prefix) {
					// Mismatch
					return
				}
				s = s[len(prefix):]
				continue
			}
		}

		n := strings.Index(s, prefix)
		if n < 0 {
			// Mismatch
			return
		}
		results[i] = s[:n]
		s = s[n+len(prefix):]
	}

	if steps[len(steps)-1].field != "" {
		us, _, ok := tryUnquoteString(s)
		if ok {
			s = us
		}
	}
	results[len(steps)-1] = s
}

func tryUnquoteString(s string) (string, int, bool) {
	if len(s) == 0 {
		return s, 0, false
	}
	if s[0] != '"' && s[0] != '`' {
		return s, 0, false
	}
	qp, err := strconv.QuotedPrefix(s)
	if err != nil {
		return s, 0, false
	}
	us, err := strconv.Unquote(qp)
	if err != nil {
		return s, 0, false
	}
	return us, len(qp), true
}

func parseExtractFormat(s string) (*extractFormat, error) {
	steps, err := parseExtractFormatSteps(s)
	if err != nil {
		return nil, err
	}
	ef := &extractFormat{
		steps: steps,

		results: make([]string, len(steps)),
	}
	return ef, nil
}

func (efs *extractFormatStep) String() string {
	return fmt.Sprintf("[prefix=%q, field=%q]", efs.prefix, efs.field)
}

func parseExtractFormatSteps(s string) ([]*extractFormatStep, error) {
	var steps []*extractFormatStep

	n := strings.IndexByte(s, '<')
	if n < 0 {
		return nil, fmt.Errorf("missing <...> fields")
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
		steps = append(steps, &extractFormatStep{
			prefix: prefix,
			field:     field,
		})
		if len(s) == 0 {
			break
		}

		n = strings.IndexByte(s, '<')
		if n < 0 {
			steps = append(steps, &extractFormatStep{
				prefix: s,
			})
			break
		}
		if n == 0 {
			return nil, fmt.Errorf("missing delimiter after <%s>", field)
		}
		prefix = s[:n]
		s = s[n+1:]
	}

	for _, step := range steps {
		step.prefix = html.UnescapeString(step.prefix)
	}

	return steps, nil
}
