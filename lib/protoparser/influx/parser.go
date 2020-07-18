package influx

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

// Rows contains parsed influx rows.
type Rows struct {
	Rows []Row

	tagsPool   []Tag
	fieldsPool []Field
}

// Reset resets rs.
func (rs *Rows) Reset() {
	// Reset rows, tags and fields in order to remove references to old data,
	// so GC could collect it.

	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]

	for i := range rs.tagsPool {
		rs.tagsPool[i].reset()
	}
	rs.tagsPool = rs.tagsPool[:0]

	for i := range rs.fieldsPool {
		rs.fieldsPool[i].reset()
	}
	rs.fieldsPool = rs.fieldsPool[:0]
}

// Unmarshal unmarshals influx line protocol rows from s.
//
// See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/
//
// s must be unchanged until rs is in use.
func (rs *Rows) Unmarshal(s string) {
	rs.Rows, rs.tagsPool, rs.fieldsPool = unmarshalRows(rs.Rows[:0], s, rs.tagsPool[:0], rs.fieldsPool[:0])
}

// Row is a single influx row.
type Row struct {
	Measurement string
	Tags        []Tag
	Fields      []Field
	Timestamp   int64
}

func (r *Row) reset() {
	r.Measurement = ""
	r.Tags = nil
	r.Fields = nil
	r.Timestamp = 0
}

func (r *Row) unmarshal(s string, tagsPool []Tag, fieldsPool []Field, noEscapeChars bool) ([]Tag, []Field, error) {
	r.reset()
	n := nextUnescapedChar(s, ' ', noEscapeChars)
	if n < 0 {
		return tagsPool, fieldsPool, fmt.Errorf("cannot find Whitespace I in %q", s)
	}
	measurementTags := s[:n]
	s = s[n+1:]

	// Parse measurement and tags
	var err error
	n = nextUnescapedChar(measurementTags, ',', noEscapeChars)
	if n >= 0 {
		tagsStart := len(tagsPool)
		tagsPool, err = unmarshalTags(tagsPool, measurementTags[n+1:], noEscapeChars)
		if err != nil {
			return tagsPool, fieldsPool, err
		}
		tags := tagsPool[tagsStart:]
		r.Tags = tags[:len(tags):len(tags)]
		measurementTags = measurementTags[:n]
	}
	r.Measurement = unescapeTagValue(measurementTags, noEscapeChars)
	// Allow empty r.Measurement. In this case metric name is constructed directly from field keys.

	// Parse fields
	fieldsStart := len(fieldsPool)
	hasQuotedFields := nextUnescapedChar(s, '"', noEscapeChars) >= 0
	n = nextUnquotedChar(s, ' ', noEscapeChars, hasQuotedFields)
	if n < 0 {
		// No timestamp.
		fieldsPool, err = unmarshalInfluxFields(fieldsPool, s, noEscapeChars, hasQuotedFields)
		if err != nil {
			return tagsPool, fieldsPool, err
		}
		fields := fieldsPool[fieldsStart:]
		r.Fields = fields[:len(fields):len(fields)]
		return tagsPool, fieldsPool, nil
	}
	fieldsPool, err = unmarshalInfluxFields(fieldsPool, s[:n], noEscapeChars, hasQuotedFields)
	if err != nil {
		return tagsPool, fieldsPool, err
	}
	r.Fields = fieldsPool[fieldsStart:]
	s = s[n+1:]

	// Parse timestamp
	timestamp := fastfloat.ParseInt64BestEffort(s)
	if timestamp == 0 && s != "0" {
		return tagsPool, fieldsPool, fmt.Errorf("cannot parse timestamp %q", s)
	}
	r.Timestamp = timestamp
	return tagsPool, fieldsPool, nil
}

// Tag represents influx tag.
type Tag struct {
	Key   string
	Value string
}

func (tag *Tag) reset() {
	tag.Key = ""
	tag.Value = ""
}

func (tag *Tag) unmarshal(s string, noEscapeChars bool) error {
	tag.reset()
	n := nextUnescapedChar(s, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", s)
	}
	tag.Key = unescapeTagValue(s[:n], noEscapeChars)
	tag.Value = unescapeTagValue(s[n+1:], noEscapeChars)
	return nil
}

// Field represents influx field.
type Field struct {
	Key   string
	Value float64
}

func (f *Field) reset() {
	f.Key = ""
	f.Value = 0
}

func (f *Field) unmarshal(s string, noEscapeChars, hasQuotedFields bool) error {
	f.reset()
	n := nextUnescapedChar(s, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing field value for %q", s)
	}
	f.Key = unescapeTagValue(s[:n], noEscapeChars)
	if len(f.Key) == 0 {
		return fmt.Errorf("field key cannot be empty")
	}
	v, err := parseFieldValue(s[n+1:], hasQuotedFields)
	if err != nil {
		return fmt.Errorf("cannot parse field value for %q: %w", f.Key, err)
	}
	f.Value = v
	return nil
}

func unmarshalRows(dst []Row, s string, tagsPool []Tag, fieldsPool []Field) ([]Row, []Tag, []Field) {
	noEscapeChars := strings.IndexByte(s, '\\') < 0
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			return unmarshalRow(dst, s, tagsPool, fieldsPool, noEscapeChars)
		}
		dst, tagsPool, fieldsPool = unmarshalRow(dst, s[:n], tagsPool, fieldsPool, noEscapeChars)
		s = s[n+1:]
	}
	return dst, tagsPool, fieldsPool
}

func unmarshalRow(dst []Row, s string, tagsPool []Tag, fieldsPool []Field, noEscapeChars bool) ([]Row, []Tag, []Field) {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		// Skip empty line
		return dst, tagsPool, fieldsPool
	}
	if s[0] == '#' {
		// Skip comment
		return dst, tagsPool, fieldsPool
	}

	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Row{})
	}
	r := &dst[len(dst)-1]
	var err error
	tagsPool, fieldsPool, err = r.unmarshal(s, tagsPool, fieldsPool, noEscapeChars)
	if err != nil {
		dst = dst[:len(dst)-1]
		logger.Errorf("cannot unmarshal Influx line %q: %s; skipping it", s, err)
		invalidLines.Inc()
	}
	return dst, tagsPool, fieldsPool
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="influx"}`)

func unmarshalTags(dst []Tag, s string, noEscapeChars bool) ([]Tag, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Tag{})
		}
		tag := &dst[len(dst)-1]
		n := nextUnescapedChar(s, ',', noEscapeChars)
		if n < 0 {
			if err := tag.unmarshal(s, noEscapeChars); err != nil {
				return dst[:len(dst)-1], err
			}
			if len(tag.Key) == 0 || len(tag.Value) == 0 {
				// Skip empty tag
				dst = dst[:len(dst)-1]
			}
			return dst, nil
		}
		if err := tag.unmarshal(s[:n], noEscapeChars); err != nil {
			return dst[:len(dst)-1], err
		}
		s = s[n+1:]
		if len(tag.Key) == 0 || len(tag.Value) == 0 {
			// Skip empty tag
			dst = dst[:len(dst)-1]
		}
	}
}

func unmarshalInfluxFields(dst []Field, s string, noEscapeChars, hasQuotedFields bool) ([]Field, error) {
	for {
		if cap(dst) > len(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Field{})
		}
		f := &dst[len(dst)-1]
		n := nextUnquotedChar(s, ',', noEscapeChars, hasQuotedFields)
		if n < 0 {
			if err := f.unmarshal(s, noEscapeChars, hasQuotedFields); err != nil {
				return dst, err
			}
			return dst, nil
		}
		if err := f.unmarshal(s[:n], noEscapeChars, hasQuotedFields); err != nil {
			return dst, err
		}
		s = s[n+1:]
	}
}

func unescapeTagValue(s string, noEscapeChars bool) string {
	if noEscapeChars {
		// Fast path - no escape chars.
		return s
	}
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	// Slow path. Remove escape chars.
	dst := make([]byte, 0, len(s))
	for {
		dst = append(dst, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			return string(append(dst, '\\'))
		}
		ch := s[0]
		if ch != ' ' && ch != ',' && ch != '=' && ch != '\\' {
			dst = append(dst, '\\')
		}
		dst = append(dst, ch)
		s = s[1:]
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			return string(append(dst, s...))
		}
	}
}

func parseFieldValue(s string, hasQuotedFields bool) (float64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("field value cannot be empty")
	}
	if hasQuotedFields && s[0] == '"' {
		if len(s) < 2 || s[len(s)-1] != '"' {
			return 0, fmt.Errorf("missing closing quote for quoted field value %s", s)
		}
		// Try converting quoted string to number, since sometimes Influx agents
		// send numbers as strings.
		s = s[1 : len(s)-1]
		return fastfloat.ParseBestEffort(s), nil
	}
	ch := s[len(s)-1]
	if ch == 'i' {
		// Integer value
		ss := s[:len(s)-1]
		n := fastfloat.ParseInt64BestEffort(ss)
		return float64(n), nil
	}
	if ch == 'u' {
		// Unsigned integer value
		ss := s[:len(s)-1]
		n := fastfloat.ParseUint64BestEffort(ss)
		return float64(n), nil
	}
	if s == "t" || s == "T" || s == "true" || s == "True" || s == "TRUE" {
		return 1, nil
	}
	if s == "f" || s == "F" || s == "false" || s == "False" || s == "FALSE" {
		return 0, nil
	}
	return fastfloat.ParseBestEffort(s), nil
}

func nextUnescapedChar(s string, ch byte, noEscapeChars bool) int {
	if noEscapeChars {
		// Fast path: just search for ch in s, since s has no escape chars.
		return strings.IndexByte(s, ch)
	}

	sOrig := s
again:
	n := strings.IndexByte(s, ch)
	if n < 0 {
		return -1
	}
	if n == 0 {
		return len(sOrig) - len(s) + n
	}
	if s[n-1] != '\\' {
		return len(sOrig) - len(s) + n
	}
	nOrig := n
	slashes := 0
	for n > 0 && s[n-1] == '\\' {
		slashes++
		n--
	}
	if slashes&1 == 0 {
		return len(sOrig) - len(s) + nOrig
	}
	s = s[nOrig+1:]
	goto again
}

func nextUnquotedChar(s string, ch byte, noEscapeChars, hasQuotedFields bool) int {
	if !hasQuotedFields {
		return nextUnescapedChar(s, ch, noEscapeChars)
	}
	sOrig := s
	for {
		n := nextUnescapedChar(s, ch, noEscapeChars)
		if n < 0 {
			return -1
		}
		if !isInQuote(s[:n], noEscapeChars) {
			return n + len(sOrig) - len(s)
		}
		s = s[n+1:]
		n = nextUnescapedChar(s, '"', noEscapeChars)
		if n < 0 {
			return -1
		}
		s = s[n+1:]
	}
}

func isInQuote(s string, noEscapeChars bool) bool {
	isQuote := false
	for {
		n := nextUnescapedChar(s, '"', noEscapeChars)
		if n < 0 {
			return isQuote
		}
		isQuote = !isQuote
		s = s[n+1:]
	}
}
