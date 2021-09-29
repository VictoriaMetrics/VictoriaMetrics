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
// s shouldn't be modified while rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.UnmarshalWithErrLogger(s, stdErrLogger)
}

func stdErrLogger(s string) {
	logger.ErrorfSkipframes(1, "%s", s)
}

// UnmarshalWithErrLogger unmarshal Prometheus exposition text rows from s.
//
// It calls errLogger for logging parsing errors.
//
// s shouldn't be modified while rs is in use.
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

func skipTrailingComment(s string) string {
	n := strings.IndexByte(s, '#')
	if n < 0 {
		return s
	}
	return s[:n]
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
	s = skipTrailingComment(s)
	if len(s) == 0 {
		return tagsPool, fmt.Errorf("value cannot be empty")
	}
	n = nextWhitespace(s)
	if n < 0 {
		// There is no timestamp.
		v, err := fastfloat.Parse(s)
		if err != nil {
			return tagsPool, fmt.Errorf("cannot parse value %q: %w", s, err)
		}
		r.Value = v
		return tagsPool, nil
	}
	// There is a timestamp.
	v, err := fastfloat.Parse(s[:n])
	if err != nil {
		return tagsPool, fmt.Errorf("cannot parse value %q: %w", s[:n], err)
	}
	r.Value = v
	s = skipLeadingWhitespace(s[n+1:])
	if len(s) == 0 {
		// There is no timestamp - just a whitespace after the value.
		return tagsPool, nil
	}
	// There are some whitespaces after timestamp
	s = skipTrailingWhitespace(s)
	ts, err := fastfloat.Parse(s)
	if err != nil {
		return tagsPool, fmt.Errorf("cannot parse timestamp %q: %w", s, err)
	}
	if ts >= -1<<31 && ts < 1<<31 {
		// This looks like OpenMetrics timestamp in Unix seconds.
		// Convert it to milliseconds.
		//
		// See https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md#timestamps
		ts *= 1000
	}
	r.Timestamp = int64(ts)
	return tagsPool, nil
}

var rowsReadScrape = metrics.NewCounter(`vm_protoparser_rows_read_total{type="promscrape"}`)

func unmarshalRows(dst []Row, s string, tagsPool []Tag, noEscapes bool, errLogger func(s string)) ([]Row, []Tag) {
	dstLen := len(dst)
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			dst, tagsPool = unmarshalRow(dst, s, tagsPool, noEscapes, errLogger)
			break
		}
		dst, tagsPool = unmarshalRow(dst, s[:n], tagsPool, noEscapes, errLogger)
		s = s[n+1:]
	}
	rowsReadScrape.Add(len(dst) - dstLen)
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
			value = unescapeValue(s[1:n])
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

func unescapeValue(s string) string {
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		// Fast path - nothing to unescape
		return s
	}
	b := make([]byte, 0, len(s))
	for {
		b = append(b, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			b = append(b, '\\')
			break
		}
		// label_value can be any sequence of UTF-8 characters, but the backslash (\), double-quote ("),
		// and line feed (\n) characters have to be escaped as \\, \", and \n, respectively.
		// See https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md
		switch s[0] {
		case '\\':
			b = append(b, '\\')
		case '"':
			b = append(b, '"')
		case 'n':
			b = append(b, '\n')
		default:
			b = append(b, '\\', s[0])
		}
		s = s[1:]
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			b = append(b, s...)
			break
		}
	}
	return string(b)
}

func prevBackslashesCount(s string) int {
	n := 0
	for len(s) > 0 && s[len(s)-1] == '\\' {
		n++
		s = s[:len(s)-1]
	}
	return n
}

// GetRowsDiff returns rows from s1, which are missing in s2.
//
// The returned rows have default value 0 and have no timestamps.
func GetRowsDiff(s1, s2 string) string {
	var r1, r2 Rows
	r1.Unmarshal(s1)
	r2.Unmarshal(s2)
	rows1 := r1.Rows
	rows2 := r2.Rows
	m := make(map[string]bool, len(rows2))
	for i := range rows2 {
		r := &rows2[i]
		key := marshalMetricNameWithTags(r)
		m[key] = true
	}
	var diff []byte
	for i := range rows1 {
		r := &rows1[i]
		key := marshalMetricNameWithTags(r)
		if !m[key] {
			diff = append(diff, key...)
			diff = append(diff, " 0\n"...)
		}
	}
	return string(diff)
}

func marshalMetricNameWithTags(r *Row) string {
	if len(r.Tags) == 0 {
		return r.Metric
	}
	var b []byte
	b = append(b, r.Metric...)
	b = append(b, '{')
	for i, t := range r.Tags {
		b = append(b, t.Key...)
		b = append(b, '=')
		b = strconv.AppendQuote(b, t.Value)
		if i+1 < len(r.Tags) {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b)
}

// AreIdenticalSeriesFast returns true if s1 and s2 contains identical Prometheus series with possible different values.
//
// This function is optimized for speed.
func AreIdenticalSeriesFast(s1, s2 string) bool {
	for {
		if len(s1) == 0 {
			// The last byte on the line reached.
			return len(s2) == 0
		}
		if len(s2) == 0 {
			// The last byte on s2 reached, while s1 has non-empty contents.
			return false
		}

		// Extract the next pair of lines from s1 and s2.
		var x1, x2 string
		n1 := strings.IndexByte(s1, '\n')
		if n1 < 0 {
			x1 = s1
			s1 = ""
		} else {
			x1 = s1[:n1]
			s1 = s1[n1+1:]
		}
		if n := strings.IndexByte(x1, '#'); n >= 0 {
			// Drop comment.
			x1 = x1[:n]
		}
		n2 := strings.IndexByte(s2, '\n')
		if n2 < 0 {
			if n1 >= 0 {
				return false
			}
			x2 = s2
			s2 = ""
		} else {
			if n1 < 0 {
				return false
			}
			x2 = s2[:n2]
			s2 = s2[n2+1:]
		}
		if n := strings.IndexByte(x2, '#'); n >= 0 {
			// Drop comment.
			x2 = x2[:n]
		}

		// Skip whitespaces in front of lines
		for len(x1) > 0 && x1[0] == ' ' {
			if len(x2) == 0 || x2[0] != ' ' {
				return false
			}
			x1 = x1[1:]
			x2 = x2[1:]
		}
		if len(x1) == 0 {
			// The last byte on x1 reached.
			if len(x2) != 0 {
				return false
			}
			continue
		}
		if len(x2) == 0 {
			// The last byte on x2 reached, while x1 has non-empty contents.
			return false
		}
		// Compare metric names
		n := strings.IndexByte(x1, ' ')
		if n < 0 {
			// Invalid Prometheus line - it must contain at least a single space between metric name and value
			// Compare it in full with x2.
			n = len(x1) - 1
		}
		n++
		if n > len(x2) || x1[:n] != x2[:n] {
			// Metric names mismatch
			return false
		}
		x1 = x1[n:]
		x2 = x2[n:]

		// The space could belong to metric name in the following cases:
		//   foo {bar="baz"} 1
		//   foo{ bar="baz"} 2
		//   foo{bar="baz", aa="b"} 3
		//   foo{bar="b az"} 4
		//   foo   5
		// Continue comparing the remaining parts until space or newline.
		for {
			n1 := strings.IndexByte(x1, ' ')
			if n1 < 0 {
				// Fast path.
				// Treat x1 as a value.
				// Skip values at x1 and x2.
				n2 := strings.IndexByte(x2, ' ')
				if n2 >= 0 {
					// x2 contains additional parts.
					return false
				}
				break
			}
			n1++
			// Slow path.
			// The x1[:n1] can be either a part of metric name or a value if timestamp is present:
			//   foo 12 34
			if isNumeric(x1[:n1-1]) {
				// Skip numeric part (most likely a value before timestamp) in x1 and x2
				n2 := strings.IndexByte(x2, ' ')
				if n2 < 0 {
					// x2 contains less parts than x1
					return false
				}
				n2++
				if !isNumeric(x2[:n2-1]) {
					// x1 contains numeric part, while x2 contains non-numeric part
					return false
				}
				x1 = x1[n1:]
				x2 = x2[n2:]
			} else {
				// The non-numeric part from x1 must match the corresponding part from x2.
				if n1 > len(x2) || x1[:n1] != x2[:n1] {
					// Parts mismatch
					return false
				}
				x1 = x1[n1:]
				x2 = x2[n1:]
			}
		}
	}
}

func isNumeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if numericChars[s[i]] {
			continue
		}
		if i == 0 && s == "NaN" || s == "nan" || s == "Inf" || s == "inf" {
			return true
		}
		if i == 1 && (s[0] == '-' || s[0] == '+') && (s[1:] == "Inf" || s[1:] == "inf") {
			return true
		}
		return false
	}
	return true
}

var numericChars = [256]bool{
	'0': true,
	'1': true,
	'2': true,
	'3': true,
	'4': true,
	'5': true,
	'6': true,
	'7': true,
	'8': true,
	'9': true,
	'-': true,
	'+': true,
	'e': true,
	'E': true,
	'.': true,
}
