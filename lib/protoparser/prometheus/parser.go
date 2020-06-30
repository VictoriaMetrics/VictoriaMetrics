package prometheus

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows contains parsed Prometheus rows.
type Rows struct {
	Rows []Row

	tagsPool []Tag
}

// Reset resets rs.
func (rs *Rows) Reset() {
	// Reset items, so they can be GC'ed

	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	for i := range rs.tagsPool {
		rs.tagsPool[i].reset()
	}
	rs.tagsPool = rs.tagsPool[:0]
}

// Unmarshal unmarshals Prometheus exposition text rows from s.
//
// See https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-format-details
//
// s must be unchanged until rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.UnmarshalWithErrLogger(s, stdErrLogger)
}

func stdErrLogger(s string) {
	logger.ErrorfSkipframes(1, "%s", s)
}

// UnmarshalWithErrLogger unmarshal Prometheus exposition text rows from s.
//
// It calls errLogger for logging parsing errors.
func (rs *Rows) UnmarshalWithErrLogger(s string, errLogger func(s string)) {
	noEscapes := strings.IndexByte(s, '\\') < 0
	rs.Rows, rs.tagsPool = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0], noEscapes, errLogger)
}

// Row is a single Prometheus row.
type Row struct {
	Metric    string
	Tags      []Tag
	Value     float64
	Timestamp int64
}

func (r *Row) reset() {
	r.Metric = ""
	r.Tags = nil
	r.Value = 0
	r.Timestamp = 0
}

func skipLeadingWhitespace(s string) string {
	// Prometheus treats ' ' and '\t' as whitespace
	// according to https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-format-details
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

func skipTrailingWhitespace(s string) string {
	// Prometheus treats ' ' and '\t' as whitespace
	// according to https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-format-details
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func nextWhitespace(s string) int {
	n := strings.IndexByte(s, ' ')
	if n < 0 {
		return strings.IndexByte(s, '\t')
	}
	n1 := strings.IndexByte(s, '\t')
	if n1 < 0 || n1 > n {
		return n
	}
	return n1
}

func (r *Row) unmarshal(s string, tagsPool []Tag, noEscapes bool) ([]Tag, error) {
	r.reset()
	s = skipLeadingWhitespace(s)
	n := strings.IndexByte(s, '{')
	if n >= 0 {
		// Tags found. Parse them.
		r.Metric = skipTrailingWhitespace(s[:n])
		s = s[n+1:]
		tagsStart := len(tagsPool)
		var err error
		s, tagsPool, err = unmarshalTags(tagsPool, s, noEscapes)
		if err != nil {
			return tagsPool, fmt.Errorf("cannot unmarshal tags: %w", err)
		}
		if len(s) > 0 && s[0] == ' ' {
			// Fast path - skip whitespace.
			s = s[1:]
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
	} else {
		// Tags weren't found. Search for value after whitespace
		n = nextWhitespace(s)
		if n < 0 {
			return tagsPool, fmt.Errorf("missing value")
		}
		r.Metric = s[:n]
		s = s[n+1:]
	}
	if len(r.Metric) == 0 {
		return tagsPool, fmt.Errorf("metric cannot be empty")
	}
	s = skipLeadingWhitespace(s)
	if len(s) == 0 {
		return tagsPool, fmt.Errorf("value cannot be empty")
	}
	n = nextWhitespace(s)
	if n < 0 {
		// There is no timestamp.
		r.Value = fastfloat.ParseBestEffort(s)
		return tagsPool, nil
	}
	// There is timestamp.
	r.Value = fastfloat.ParseBestEffort(s[:n])
	s = skipLeadingWhitespace(s[n+1:])
	r.Timestamp = fastfloat.ParseInt64BestEffort(s)
	return tagsPool, nil
}

func unmarshalRows(dst []Row, s string, tagsPool []Tag, noEscapes bool, errLogger func(s string)) ([]Row, []Tag) {
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			return unmarshalRow(dst, s, tagsPool, noEscapes, errLogger)
		}
		dst, tagsPool = unmarshalRow(dst, s[:n], tagsPool, noEscapes, errLogger)
		s = s[n+1:]
	}
	return dst, tagsPool
}

func unmarshalRow(dst []Row, s string, tagsPool []Tag, noEscapes bool, errLogger func(s string)) ([]Row, []Tag) {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	s = skipLeadingWhitespace(s)
	if len(s) == 0 {
		// Skip empty line
		return dst, tagsPool
	}
	if s[0] == '#' {
		// Skip comment
		return dst, tagsPool
	}
	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Row{})
	}
	r := &dst[len(dst)-1]
	var err error
	tagsPool, err = r.unmarshal(s, tagsPool, noEscapes)
	if err != nil {
		dst = dst[:len(dst)-1]
		msg := fmt.Sprintf("cannot unmarshal Prometheus line %q: %s", s, err)
		errLogger(msg)
		invalidLines.Inc()
	}
	return dst, tagsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="prometheus"}`)

func unmarshalTags(dst []Tag, s string, noEscapes bool) (string, []Tag, error) {
	for {
		s = skipLeadingWhitespace(s)
		if len(s) > 0 && s[0] == '}' {
			// End of tags found.
			return s[1:], dst, nil
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return s, dst, fmt.Errorf("missing value for tag %q", s)
		}
		key := skipTrailingWhitespace(s[:n])
		s = skipLeadingWhitespace(s[n+1:])
		if len(s) == 0 || s[0] != '"' {
			return s, dst, fmt.Errorf("expecting quoted value for tag %q; got %q", key, s)
		}
		value := s[1:]
		if noEscapes {
			// Fast path - the line has no escape chars
			n = strings.IndexByte(value, '"')
			if n < 0 {
				return s, dst, fmt.Errorf("missing closing quote for tag value %q", s)
			}
			s = value[n+1:]
			value = value[:n]
		} else {
			// Slow path - the line contains escape chars
			n = findClosingQuote(s)
			if n < 0 {
				return s, dst, fmt.Errorf("missing closing quote for tag value %q", s)
			}
			var err error
			value, err = unescapeValue(s[:n+1])
			if err != nil {
				return s, dst, fmt.Errorf("cannot unescape value %q for tag %q: %w", s[:n+1], key, err)
			}
			s = s[n+1:]
		}
		if len(key) > 0 {
			// Allow empty values (len(value)==0) - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/453
			if cap(dst) > len(dst) {
				dst = dst[:len(dst)+1]
			} else {
				dst = append(dst, Tag{})
			}
			tag := &dst[len(dst)-1]
			tag.Key = key
			tag.Value = value
		}
		s = skipLeadingWhitespace(s)
		if len(s) > 0 && s[0] == '}' {
			// End of tags found.
			return s[1:], dst, nil
		}
		if len(s) == 0 || s[0] != ',' {
			return s, dst, fmt.Errorf("missing comma after tag %s=%q", key, value)
		}
		s = s[1:]
	}
}

// Tag is a Prometheus tag.
type Tag struct {
	Key   string
	Value string
}

func (t *Tag) reset() {
	t.Key = ""
	t.Value = ""
}

func findClosingQuote(s string) int {
	if len(s) == 0 || s[0] != '"' {
		return -1
	}
	off := 1
	s = s[1:]
	for {
		n := strings.IndexByte(s, '"')
		if n < 0 {
			return -1
		}
		if prevBackslashesCount(s[:n])%2 == 0 {
			return off + n
		}
		off += n + 1
		s = s[n+1:]
	}
}

func unescapeValue(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("unexpected tag value: %q", s)
	}
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		// Fast path - nothing to unescape
		return s[1 : len(s)-1], nil
	}
	return strconv.Unquote(s)
}

func prevBackslashesCount(s string) int {
	n := 0
	for len(s) > 0 && s[len(s)-1] == '\\' {
		n++
		s = s[:len(s)-1]
	}
	return n
}
